// Package dns_test verifies the ZoneFile and ConfigMapZoneWriter types in
// internal/dns. These are the zone management primitives used by DSNSReconciler.
// seam-core-schema.md §8 Decision 2.
package dns_test

import (
	"context"
	"regexp"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	idns "github.com/ontai-dev/seam/internal/dns"
)

// ── helpers ───────────────────────────────────────────────────────────────────

func newScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	s := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(s); err != nil {
		t.Fatalf("clientgoscheme: %v", err)
	}
	return s
}

func newFakeClient(t *testing.T, objs ...client.Object) client.Client {
	t.Helper()
	return fake.NewClientBuilder().WithScheme(newScheme(t)).WithObjects(objs...).Build()
}

// ── ZoneFile tests ────────────────────────────────────────────────────────────

// TestZoneFile_Render_IncludesSOAAndNS verifies that the rendered zone always
// contains a valid SOA record and zone-level NS record regardless of managed records.
func TestZoneFile_Render_IncludesSOAAndNS(t *testing.T) {
	t.Parallel()
	z := idns.NewZoneFile()
	out := z.Render()

	if !strings.Contains(out, "$ORIGIN seam.ontave.dev.") {
		t.Errorf("zone render missing $ORIGIN directive:\n%s", out)
	}
	if !strings.Contains(out, "IN SOA") {
		t.Errorf("zone render missing SOA record:\n%s", out)
	}
	if !strings.Contains(out, "IN NS") {
		t.Errorf("zone render missing NS record:\n%s", out)
	}
}

// TestZoneFile_Render_SOASerialIsUnixTimestamp verifies that the SOA serial is a
// 10-digit decimal number matching the current Unix timestamp magnitude.
func TestZoneFile_Render_SOASerialIsUnixTimestamp(t *testing.T) {
	t.Parallel()
	z := idns.NewZoneFile()
	out := z.Render()

	// SOA line contains the serial as the 5th whitespace-separated token after "IN SOA".
	// Regex: matches a 10-digit integer in the rendered output (current epoch seconds).
	re := regexp.MustCompile(`\b1[0-9]{9}\b`)
	if !re.MatchString(out) {
		t.Errorf("SOA serial does not look like a Unix timestamp (10-digit epoch) in:\n%s", out)
	}
}

// TestZoneFile_Render_MultipleRecordTypes verifies A, TXT, and NS records all
// appear correctly in the rendered zone file with proper RFC 1035 formatting.
func TestZoneFile_Render_MultipleRecordTypes(t *testing.T) {
	t.Parallel()
	z := idns.NewZoneFile()
	z.AddRecord(idns.Record{Name: "cluster1", Type: idns.RecordTypeA, Value: "10.20.0.10"})
	z.AddRecord(idns.Record{Name: "api.cluster1", Type: idns.RecordTypeA, Value: "10.20.0.10"})
	z.AddRecord(idns.Record{Name: "role.cluster1", Type: idns.RecordTypeTXT, Value: "management"})
	z.AddRecord(idns.Record{Name: "sovereign", Type: idns.RecordTypeNS, Value: "ns.sovereign.seam.ontave.dev"})

	out := z.Render()

	wantLines := []string{
		"cluster1 300 IN A 10.20.0.10",
		"api.cluster1 300 IN A 10.20.0.10",
		`role.cluster1 300 IN TXT "management"`,
		"sovereign 300 IN NS ns.sovereign.seam.ontave.dev.",
	}
	for _, want := range wantLines {
		if !strings.Contains(out, want) {
			t.Errorf("rendered zone missing expected line %q:\n%s", want, out)
		}
	}
}

// TestZoneFile_AddRecord_OverwritesSameTypeAndName verifies that adding a record
// with the same (type, name) as an existing record replaces it.
func TestZoneFile_AddRecord_OverwritesSameTypeAndName(t *testing.T) {
	t.Parallel()
	z := idns.NewZoneFile()
	z.AddRecord(idns.Record{Name: "cluster1", Type: idns.RecordTypeA, Value: "10.20.0.10"})
	z.AddRecord(idns.Record{Name: "cluster1", Type: idns.RecordTypeA, Value: "10.20.0.20"})

	out := z.Render()
	if strings.Contains(out, "10.20.0.10") {
		t.Errorf("old A record value still present after overwrite:\n%s", out)
	}
	if !strings.Contains(out, "cluster1 300 IN A 10.20.0.20") {
		t.Errorf("new A record value not found:\n%s", out)
	}
}

// TestZoneFile_RemoveRecord removes a record and verifies it is absent from output.
func TestZoneFile_RemoveRecord(t *testing.T) {
	t.Parallel()
	z := idns.NewZoneFile()
	z.AddRecord(idns.Record{Name: "cluster1", Type: idns.RecordTypeA, Value: "10.20.0.10"})
	z.AddRecord(idns.Record{Name: "role.cluster1", Type: idns.RecordTypeTXT, Value: "management"})
	z.RemoveRecord(idns.RecordTypeA, "cluster1")

	out := z.Render()
	if strings.Contains(out, "IN A 10.20.0.10") {
		t.Errorf("removed A record still appears in rendered zone:\n%s", out)
	}
	if !strings.Contains(out, `role.cluster1`) {
		t.Errorf("unrelated TXT record was incorrectly removed:\n%s", out)
	}
}

// TestZoneFile_RemoveRecord_NoOpWhenAbsent verifies Remove on an absent record
// does not panic or corrupt zone state.
func TestZoneFile_RemoveRecord_NoOpWhenAbsent(t *testing.T) {
	t.Parallel()
	z := idns.NewZoneFile()
	z.AddRecord(idns.Record{Name: "cluster1", Type: idns.RecordTypeA, Value: "10.20.0.10"})
	z.RemoveRecord(idns.RecordTypeTXT, "cluster1") // wrong type — no-op
	z.RemoveRecord(idns.RecordTypeA, "missing")    // absent — no-op

	out := z.Render()
	if !strings.Contains(out, "cluster1 300 IN A 10.20.0.10") {
		t.Errorf("existing A record was incorrectly removed:\n%s", out)
	}
}

// ── ConfigMapZoneWriter tests ─────────────────────────────────────────────────

// TestConfigMapZoneWriter_CreatePath verifies that ApplyContent creates the
// dsns-zone ConfigMap with the correct label and governance annotation when it
// does not yet exist.
func TestConfigMapZoneWriter_CreatePath(t *testing.T) {
	t.Parallel()
	fc := newFakeClient(t)
	w := idns.NewConfigMapZoneWriter(fc)

	const content = "$ORIGIN seam.ontave.dev.\n"
	if err := w.ApplyContent(context.Background(), content); err != nil {
		t.Fatalf("ApplyContent (create path): %v", err)
	}

	var cm corev1.ConfigMap
	if err := fc.Get(context.Background(), client.ObjectKey{
		Name:      idns.ZoneConfigMapName,
		Namespace: idns.ZoneConfigMapNamespace,
	}, &cm); err != nil {
		t.Fatalf("Get dsns-zone ConfigMap: %v", err)
	}

	if got := cm.Labels[idns.ZoneLabelKey]; got != idns.ZoneLabelValue {
		t.Errorf("label %s = %q, want %q", idns.ZoneLabelKey, got, idns.ZoneLabelValue)
	}
	if got := cm.Annotations[idns.ZoneOwnerAnnotationKey]; got != idns.ZoneOwnerAnnotationVal {
		t.Errorf("annotation %s = %q, want %q", idns.ZoneOwnerAnnotationKey, got, idns.ZoneOwnerAnnotationVal)
	}
	if got := cm.Data[idns.ZoneDataKey]; got != content {
		t.Errorf("zone.db = %q, want %q", got, content)
	}
}

// TestConfigMapZoneWriter_PatchPath verifies that ApplyContent updates the
// zone.db key of an existing dsns-zone ConfigMap via MergeFrom patch.
func TestConfigMapZoneWriter_PatchPath(t *testing.T) {
	t.Parallel()

	// Pre-create the ConfigMap
	existing := &corev1.ConfigMap{}
	existing.Name = idns.ZoneConfigMapName
	existing.Namespace = idns.ZoneConfigMapNamespace
	existing.Data = map[string]string{idns.ZoneDataKey: "old content"}
	fc := newFakeClient(t, existing)

	w := idns.NewConfigMapZoneWriter(fc)
	const newContent = "$ORIGIN seam.ontave.dev.\nnew content\n"
	if err := w.ApplyContent(context.Background(), newContent); err != nil {
		t.Fatalf("ApplyContent (patch path): %v", err)
	}

	var cm corev1.ConfigMap
	if err := fc.Get(context.Background(), client.ObjectKey{
		Name:      idns.ZoneConfigMapName,
		Namespace: idns.ZoneConfigMapNamespace,
	}, &cm); err != nil {
		t.Fatalf("Get after patch: %v", err)
	}
	if got := cm.Data[idns.ZoneDataKey]; got != newContent {
		t.Errorf("zone.db after patch = %q, want %q", got, newContent)
	}
}

// TestConfigMapZoneWriter_Apply_UsesZoneFileRender verifies that Apply calls
// Render on the provided ZoneFile and writes the output to zone.db.
func TestConfigMapZoneWriter_Apply_UsesZoneFileRender(t *testing.T) {
	t.Parallel()
	fc := newFakeClient(t)
	w := idns.NewConfigMapZoneWriter(fc)

	z := idns.NewZoneFile()
	z.AddRecord(idns.Record{Name: "cluster1", Type: idns.RecordTypeA, Value: "10.1.2.3"})

	if err := w.Apply(context.Background(), z); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	var cm corev1.ConfigMap
	if err := fc.Get(context.Background(), client.ObjectKey{
		Name:      idns.ZoneConfigMapName,
		Namespace: idns.ZoneConfigMapNamespace,
	}, &cm); err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !strings.Contains(cm.Data[idns.ZoneDataKey], "cluster1 300 IN A 10.1.2.3") {
		t.Errorf("zone.db missing expected A record:\n%s", cm.Data[idns.ZoneDataKey])
	}
}
