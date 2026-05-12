package dns_test

import (
	"context"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/ontai-dev/seam-core/internal/controller"
	idns "github.com/ontai-dev/seam-core/internal/dns"
)

// ── test helpers ──────────────────────────────────────────────────────────────

var (
	talosClusterGVK   = schema.GroupVersionKind{Group: "seam.ontai.dev", Version: "v1alpha1", Kind: "TalosCluster"}
	identityBindGVK   = schema.GroupVersionKind{Group: "guardian.ontai.dev", Version: "v1alpha1", Kind: "IdentityBinding"}
	identityProvGVK   = schema.GroupVersionKind{Group: "guardian.ontai.dev", Version: "v1alpha1", Kind: "IdentityProvider"}
	packInstanceGVK   = schema.GroupVersionKind{Group: "seam.ontai.dev", Version: "v1alpha1", Kind: "PackInstalled"}
	runnerConfigGVK   = schema.GroupVersionKind{Group: "seam.ontai.dev", Version: "v1alpha1", Kind: "RunnerConfig"}
)

func newUnstructured(gvk schema.GroupVersionKind, name, ns string) *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(gvk)
	u.SetName(name)
	u.SetNamespace(ns)
	u.SetUID(types.UID("uid-" + name))
	u.SetGeneration(1)
	return u
}

func setCondition(u *unstructured.Unstructured, condType, status, reason, lastTime string) {
	existing, _, _ := unstructured.NestedSlice(u.Object, "status", "conditions")
	cond := map[string]interface{}{
		"type":               condType,
		"status":             status,
		"reason":             reason,
		"message":            "",
		"lastTransitionTime": lastTime,
	}
	updated := make([]interface{}, 0, len(existing)+1)
	for _, c := range existing {
		cm, ok := c.(map[string]interface{})
		if ok && cm["type"] == condType {
			continue // replace
		}
		updated = append(updated, c)
	}
	updated = append(updated, cond)
	_ = unstructured.SetNestedSlice(u.Object, updated, "status", "conditions")
}

func setField(u *unstructured.Unstructured, value interface{}, fields ...string) {
	_ = unstructured.SetNestedField(u.Object, value, fields...)
}

func newDSNSReconciler(fc client.Client, gvk schema.GroupVersionKind, state *idns.DSNSState) *controller.DSNSReconciler {
	return &controller.DSNSReconciler{
		Client: fc,
		GVK:    gvk,
		State:  state,
	}
}

func reconcile(t *testing.T, r *controller.DSNSReconciler, obj *unstructured.Unstructured) {
	t.Helper()
	req := ctrl.Request{NamespacedName: types.NamespacedName{Name: obj.GetName(), Namespace: obj.GetNamespace()}}
	if _, err := r.Reconcile(context.Background(), req); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
}

func zoneContent(t *testing.T, fc client.Client) string {
	t.Helper()
	var cm unstructured.Unstructured
	cm.SetAPIVersion("v1")
	cm.SetKind("ConfigMap")
	if err := fc.Get(context.Background(), client.ObjectKey{
		Name:      idns.ZoneConfigMapName,
		Namespace: idns.ZoneConfigMapNamespace,
	}, &cm); err != nil {
		t.Fatalf("get dsns-zone ConfigMap: %v", err)
	}
	data, _, _ := unstructured.NestedStringMap(cm.Object, "data")
	return data[idns.ZoneDataKey]
}

// ── TalosCluster tests ────────────────────────────────────────────────────────

// TestDSNSReconciler_TalosCluster_ReadyState verifies that a Ready TalosCluster
// produces an A record for the cluster endpoint, an A record for the api endpoint,
// and a TXT role record carrying status.origin.
// Bug fix: tests updated to use spec.clusterEndpoint (not spec.vip/status.apiEndpoint)
// and status.origin (not spec.infrastructure.provider) per platform commit 02132d1.
func TestDSNSReconciler_TalosCluster_ReadyState(t *testing.T) {
	t.Parallel()

	tc := newUnstructured(talosClusterGVK, "cluster1", "ont-system")
	setField(tc, "10.20.0.10", "spec", "clusterEndpoint")
	setField(tc, "bootstrapped", "status", "origin")
	setCondition(tc, "Ready", "True", "ClusterReady", "2026-04-06T00:00:00Z")

	fc := newFakeClient(t, tc)
	state := idns.NewDSNSState(fc)
	r := newDSNSReconciler(fc, talosClusterGVK, state)
	reconcile(t, r, tc)

	zone := zoneContent(t, fc)
	wantLines := []string{
		"cluster1 300 IN A 10.20.0.10",
		"api.cluster1 300 IN A 10.20.0.10",
		`role.cluster1 300 IN TXT "bootstrapped"`,
	}
	for _, want := range wantLines {
		if !strings.Contains(zone, want) {
			t.Errorf("zone missing %q:\n%s", want, zone)
		}
	}
}

// TestDSNSReconciler_TalosCluster_NotReady verifies that a non-Ready TalosCluster
// produces no DNS records.
func TestDSNSReconciler_TalosCluster_NotReady(t *testing.T) {
	t.Parallel()
	tc := newUnstructured(talosClusterGVK, "cluster-nr", "ont-system")
	setField(tc, "10.20.0.10", "spec", "clusterEndpoint")
	// No Ready=True condition.

	fc := newFakeClient(t, tc)
	state := idns.NewDSNSState(fc)
	r := newDSNSReconciler(fc, talosClusterGVK, state)
	reconcile(t, r, tc)

	zone := zoneContent(t, fc)
	if strings.Contains(zone, "cluster-nr") {
		t.Errorf("zone should not contain records for non-Ready cluster:\n%s", zone)
	}
}

// TestDSNSReconciler_TalosCluster_SovereignProvider verifies that a TalosCluster
// with infrastructureProvider="screen" emits an NS delegation record instead of a
// TXT role record. Bug fix: field path updated to spec.infrastructureProvider
// (json tag) from stale spec.infrastructure.provider (nested struct).
func TestDSNSReconciler_TalosCluster_SovereignProvider(t *testing.T) {
	t.Parallel()
	tc := newUnstructured(talosClusterGVK, "sovereign1", "ont-system")
	setField(tc, "10.20.0.30", "spec", "clusterEndpoint")
	setField(tc, "screen", "spec", "infrastructureProvider")
	setCondition(tc, "Ready", "True", "ClusterReady", "2026-04-06T00:00:00Z")

	fc := newFakeClient(t, tc)
	state := idns.NewDSNSState(fc)
	r := newDSNSReconciler(fc, talosClusterGVK, state)
	reconcile(t, r, tc)

	zone := zoneContent(t, fc)
	if !strings.Contains(zone, "sovereign1 300 IN NS") {
		t.Errorf("zone missing NS delegation for sovereign cluster:\n%s", zone)
	}
	if strings.Contains(zone, `role.sovereign1 300 IN TXT`) {
		t.Errorf("zone should not have TXT role record for sovereign cluster:\n%s", zone)
	}
	// Glue A record for ns.sovereign1
	if !strings.Contains(zone, "ns.sovereign1 300 IN A 10.20.0.30") {
		t.Errorf("zone missing glue A record for NS delegation:\n%s", zone)
	}
}

// TestDSNSReconciler_TalosCluster_Deletion verifies that deleting a TalosCluster
// removes its DNS records from the zone.
func TestDSNSReconciler_TalosCluster_Deletion(t *testing.T) {
	t.Parallel()
	tc := newUnstructured(talosClusterGVK, "cluster-del", "ont-system")
	setField(tc, "10.20.0.10", "spec", "clusterEndpoint")
	setField(tc, "bootstrapped", "status", "origin")
	setCondition(tc, "Ready", "True", "ClusterReady", "2026-04-06T00:00:00Z")

	fc := newFakeClient(t, tc)
	state := idns.NewDSNSState(fc)
	r := newDSNSReconciler(fc, talosClusterGVK, state)

	// First reconcile — adds finalizer and records.
	reconcile(t, r, tc)
	zone := zoneContent(t, fc)
	if !strings.Contains(zone, "cluster-del 300 IN A") {
		t.Fatalf("expected A record after first reconcile:\n%s", zone)
	}

	// Simulate deletion: fetch updated object (has finalizer), call Delete.
	var fresh unstructured.Unstructured
	fresh.SetGroupVersionKind(talosClusterGVK)
	if err := fc.Get(context.Background(), client.ObjectKey{Name: "cluster-del", Namespace: "ont-system"}, &fresh); err != nil {
		t.Fatalf("re-fetch after reconcile: %v", err)
	}
	if err := fc.Delete(context.Background(), &fresh); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	// Object should still exist (has finalizer) with DeletionTimestamp set.
	var deleted unstructured.Unstructured
	deleted.SetGroupVersionKind(talosClusterGVK)
	if err := fc.Get(context.Background(), client.ObjectKey{Name: "cluster-del", Namespace: "ont-system"}, &deleted); err != nil {
		t.Fatalf("Get after Delete: %v", err)
	}

	// Reconcile the deleting object.
	reconcile(t, r, &deleted)

	zone = zoneContent(t, fc)
	if strings.Contains(zone, "cluster-del") {
		t.Errorf("zone still contains records for deleted cluster:\n%s", zone)
	}
}

// ── IdentityBinding tests ─────────────────────────────────────────────────────

// TestDSNSReconciler_IdentityBinding_Resolved verifies that a resolved
// IdentityBinding emits a TXT identity record with the sha256-based name.
func TestDSNSReconciler_IdentityBinding_Resolved(t *testing.T) {
	t.Parallel()
	ib := newUnstructured(identityBindGVK, "alice-binding", "seam-tenant-cluster1")
	setField(ib, "alice@example.com", "spec", "subject")
	setField(ib, "admin-profile", "spec", "rbacProfileRef", "name")
	setField(ib, "okta-provider", "spec", "identityProviderRef", "name")
	setCondition(ib, "TrustAnchorResolved", "True", "TrustAnchorResolved", "2026-04-06T00:00:00Z")

	fc := newFakeClient(t, ib)
	state := idns.NewDSNSState(fc)
	r := newDSNSReconciler(fc, identityBindGVK, state)
	reconcile(t, r, ib)

	zone := zoneContent(t, fc)
	// Check that the identity record appears with guardian.cluster1 suffix and correct value.
	if !strings.Contains(zone, ".guardian.cluster1") {
		t.Errorf("zone missing identity record with .guardian.cluster1 suffix:\n%s", zone)
	}
	if !strings.Contains(zone, `"admin-profile okta-provider"`) {
		t.Errorf("zone missing identity TXT value:\n%s", zone)
	}
	if !strings.Contains(zone, "identity.") {
		t.Errorf("zone missing identity. prefix in record name:\n%s", zone)
	}
}

// TestDSNSReconciler_IdentityBinding_NotResolved verifies no record is emitted
// for an IdentityBinding without TrustAnchorResolved=True.
func TestDSNSReconciler_IdentityBinding_NotResolved(t *testing.T) {
	t.Parallel()
	ib := newUnstructured(identityBindGVK, "pending-binding", "seam-tenant-cluster1")
	setField(ib, "bob@example.com", "spec", "subject")
	// No TrustAnchorResolved condition.

	fc := newFakeClient(t, ib)
	state := idns.NewDSNSState(fc)
	r := newDSNSReconciler(fc, identityBindGVK, state)
	reconcile(t, r, ib)

	zone := zoneContent(t, fc)
	if strings.Contains(zone, "identity.") {
		t.Errorf("zone should not contain identity record for unresolved binding:\n%s", zone)
	}
}

// ── IdentityProvider tests ────────────────────────────────────────────────────

// TestDSNSReconciler_IdentityProvider_Valid verifies that a Valid IdentityProvider
// emits a TXT idp record carrying the issuerURL.
func TestDSNSReconciler_IdentityProvider_Valid(t *testing.T) {
	t.Parallel()
	ip := newUnstructured(identityProvGVK, "okta", "seam-system")
	setField(ip, "https://accounts.example.com", "status", "issuerURL")
	setCondition(ip, "Valid", "True", "ProviderReachable", "2026-04-06T00:00:00Z")

	fc := newFakeClient(t, ip)
	state := idns.NewDSNSState(fc)
	r := newDSNSReconciler(fc, identityProvGVK, state)
	reconcile(t, r, ip)

	zone := zoneContent(t, fc)
	wantLines := []string{
		`idp.okta.guardian 300 IN TXT "https://accounts.example.com"`,
	}
	for _, want := range wantLines {
		if !strings.Contains(zone, want) {
			t.Errorf("zone missing %q:\n%s", want, zone)
		}
	}
}

// ── PackInstance tests ────────────────────────────────────────────────────────

// TestDSNSReconciler_PackInstance_Succeeded verifies that a Ready PackInstance
// emits a TXT pack record. spec.clusterPackRef is the flat string pack name;
// spec.targetClusterRef is the cluster; version uses "unknown" fallback since
// it is not present on PackInstance spec. seam-core-schema.md §8 Decision 4.
func TestDSNSReconciler_PackInstance_Succeeded(t *testing.T) {
	t.Parallel()
	pi := newUnstructured(packInstanceGVK, "nginx-instance", "seam-tenant-cluster1")
	setField(pi, "nginx", "spec", "clusterPackRef")
	setField(pi, "cluster1", "spec", "targetClusterRef")
	setField(pi, "sha256:abcdef1234567890", "status", "receiptDigest")
	setCondition(pi, "Ready", "True", "PackReceiptReady", "2026-04-06T00:00:00Z")

	fc := newFakeClient(t, pi)
	state := idns.NewDSNSState(fc)
	r := newDSNSReconciler(fc, packInstanceGVK, state)
	reconcile(t, r, pi)

	zone := zoneContent(t, fc)
	want := `pack.nginx.unknown.wrapper.cluster1 300 IN TXT "sha256:abcdef1234567890"`
	if !strings.Contains(zone, want) {
		t.Errorf("zone missing %q:\n%s", want, zone)
	}
}

// TestDSNSReconciler_PackInstance_ReadyNoReceipt verifies that a Ready PackInstance
// without a receiptDigest still emits a TXT record with value "delivered".
// PackReceipt is only written by the conductor agent on target clusters; the
// management cluster will never produce one in lab. seam-core-schema.md §8 Decision 4.
func TestDSNSReconciler_PackInstance_ReadyNoReceipt(t *testing.T) {
	t.Parallel()
	pi := newUnstructured(packInstanceGVK, "my-pack-instance", "seam-tenant-cluster1")
	setField(pi, "my-pack", "spec", "clusterPackRef")
	setField(pi, "cluster1", "spec", "targetClusterRef")
	// No receiptDigest set — simulates management cluster lab environment.
	setCondition(pi, "Ready", "True", "PackDelivered", "2026-04-10T00:00:00Z")

	fc := newFakeClient(t, pi)
	state := idns.NewDSNSState(fc)
	r := newDSNSReconciler(fc, packInstanceGVK, state)
	reconcile(t, r, pi)

	zone := zoneContent(t, fc)
	want := `pack.my-pack.unknown.wrapper.cluster1 300 IN TXT "delivered"`
	if !strings.Contains(zone, want) {
		t.Errorf("zone missing %q:\n%s", want, zone)
	}
}

// ── RunnerConfig tests ────────────────────────────────────────────────────────

// TestDSNSReconciler_RunnerConfig_Completed verifies that a completed RunnerConfig
// emits a TXT run record with phase=Completed.
func TestDSNSReconciler_RunnerConfig_Completed(t *testing.T) {
	t.Parallel()
	rc := newUnstructured(runnerConfigGVK, "node-drain-rc", "seam-tenant-cluster1")
	setCondition(rc, "Ready", "True", "AllStepsCompleted", "2026-04-06T12:00:00Z")

	fc := newFakeClient(t, rc)
	state := idns.NewDSNSState(fc)
	r := newDSNSReconciler(fc, runnerConfigGVK, state)
	reconcile(t, r, rc)

	zone := zoneContent(t, fc)
	if !strings.Contains(zone, "run.node-drain-rc.conductor.cluster1") {
		t.Errorf("zone missing run record:\n%s", zone)
	}
	if !strings.Contains(zone, `phase=Completed`) {
		t.Errorf("zone missing phase=Completed in run record:\n%s", zone)
	}
}

// TestDSNSReconciler_RunnerConfig_Failed verifies that a failed RunnerConfig
// emits a TXT run record with phase=Failed.
func TestDSNSReconciler_RunnerConfig_Failed(t *testing.T) {
	t.Parallel()
	rc := newUnstructured(runnerConfigGVK, "upgrade-rc", "seam-tenant-cluster1")
	setCondition(rc, "Degraded", "True", "StepFailed", "2026-04-06T12:00:00Z")

	fc := newFakeClient(t, rc)
	state := idns.NewDSNSState(fc)
	r := newDSNSReconciler(fc, runnerConfigGVK, state)
	reconcile(t, r, rc)

	zone := zoneContent(t, fc)
	if !strings.Contains(zone, "run.upgrade-rc.conductor.cluster1") {
		t.Errorf("zone missing run record for failed RunnerConfig:\n%s", zone)
	}
	if !strings.Contains(zone, `phase=Failed`) {
		t.Errorf("zone missing phase=Failed in run record:\n%s", zone)
	}
}

// ── Authority record test ─────────────────────────────────────────────────────

// TestDSNSReconciler_StaticAuthorityRecord verifies that a static authority.conductor
// TXT record set via DSNSState.SetStaticRecord appears in the zone.
// seam-core-schema.md §8 Decision 4 — Conductor authority record.
func TestDSNSReconciler_StaticAuthorityRecord(t *testing.T) {
	t.Parallel()
	fc := newFakeClient(t)
	state := idns.NewDSNSState(fc)

	const fingerprint = "SHA256:AAABBBCCC111222333"
	state.SetStaticRecord(idns.Record{
		Name:  "authority.conductor",
		Type:  idns.RecordTypeTXT,
		Value: fingerprint,
	})

	if err := state.Apply(context.Background()); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	zone := zoneContent(t, fc)
	want := `authority.conductor 300 IN TXT "SHA256:AAABBBCCC111222333"`
	if !strings.Contains(zone, want) {
		t.Errorf("zone missing authority.conductor record:\nwant: %q\ngot:\n%s", want, zone)
	}
}

// TestDSNSReconciler_TalosCluster_SpecModeFallback verifies that when status.origin
// is empty, the role TXT falls back to spec.mode.
func TestDSNSReconciler_TalosCluster_SpecModeFallback(t *testing.T) {
	t.Parallel()
	tc := newUnstructured(talosClusterGVK, "cluster-import", "ont-system")
	setField(tc, "10.20.0.50", "spec", "clusterEndpoint")
	setField(tc, "import", "spec", "mode") // status.origin absent — fall back to spec.mode
	setCondition(tc, "Ready", "True", "ClusterReady", "2026-04-06T00:00:00Z")

	fc := newFakeClient(t, tc)
	state := idns.NewDSNSState(fc)
	r := newDSNSReconciler(fc, talosClusterGVK, state)
	reconcile(t, r, tc)

	zone := zoneContent(t, fc)
	if !strings.Contains(zone, `role.cluster-import 300 IN TXT "import"`) {
		t.Errorf("zone missing spec.mode fallback TXT:\n%s", zone)
	}
}

// TestDSNSReconciler_StaticNsGlueRecord verifies that a static ns A record set via
// DSNSState.SetStaticRecord (seeded from DSNS_SERVICE_IP in main.go) appears in the
// zone so that CoreDNS can resolve its own nameserver ns.seam.ontave.dev.
// Bug 2 fix: ns.seam.ontave.dev had no A record; SOA declared it as nameserver but
// it was unresolvable.
func TestDSNSReconciler_StaticNsGlueRecord(t *testing.T) {
	t.Parallel()
	fc := newFakeClient(t)
	state := idns.NewDSNSState(fc)

	const dsnsIP = "10.20.0.241"
	state.SetStaticRecord(idns.Record{
		Name:  "ns",
		Type:  idns.RecordTypeA,
		Value: dsnsIP,
	})

	if err := state.Apply(context.Background()); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	zone := zoneContent(t, fc)
	want := "ns 300 IN A 10.20.0.241"
	if !strings.Contains(zone, want) {
		t.Errorf("zone missing ns glue A record:\nwant: %q\ngot:\n%s", want, zone)
	}
}

// TestDSNSReconciler_Deletion_RemovesRecordsViaFinalizer verifies that the
// finalizer-gated deletion path removes all records owned by the object.
func TestDSNSReconciler_Deletion_RemovesRecordsViaFinalizer(t *testing.T) {
	t.Parallel()
	ip := newUnstructured(identityProvGVK, "test-idp", "seam-system")
	setField(ip, "https://idp.example.com", "status", "issuerURL")
	setCondition(ip, "Valid", "True", "ProviderReachable", "2026-04-06T00:00:00Z")

	fc := newFakeClient(t, ip)
	state := idns.NewDSNSState(fc)
	r := newDSNSReconciler(fc, identityProvGVK, state)

	// First reconcile: adds finalizer and idp record.
	reconcile(t, r, ip)
	zone := zoneContent(t, fc)
	if !strings.Contains(zone, "idp.test-idp.guardian") {
		t.Fatalf("expected idp record after reconcile:\n%s", zone)
	}

	// Fetch the updated object (now has finalizer).
	var fresh unstructured.Unstructured
	fresh.SetGroupVersionKind(identityProvGVK)
	if err := fc.Get(context.Background(), client.ObjectKey{Name: "test-idp", Namespace: "seam-system"}, &fresh); err != nil {
		t.Fatalf("re-fetch: %v", err)
	}
	if err := fc.Delete(context.Background(), &fresh); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	// Fetch the deleting object.
	var deleting unstructured.Unstructured
	deleting.SetGroupVersionKind(identityProvGVK)
	if err := fc.Get(context.Background(), client.ObjectKey{Name: "test-idp", Namespace: "seam-system"}, &deleting); err != nil {
		t.Fatalf("Get after Delete: %v", err)
	}
	if deleting.GetDeletionTimestamp() == nil {
		t.Fatal("expected DeletionTimestamp to be set after Delete")
	}

	// Reconcile the deleting object: removes records and finalizer.
	reconcile(t, r, &deleting)

	zone = zoneContent(t, fc)
	if strings.Contains(zone, "idp.test-idp.guardian") {
		t.Errorf("zone still contains idp record after deletion:\n%s", zone)
	}

	// Object should now be fully gone.
	var gone unstructured.Unstructured
	gone.SetGroupVersionKind(identityProvGVK)
	err := fc.Get(context.Background(), client.ObjectKey{Name: "test-idp", Namespace: "seam-system"}, &gone)
	if err == nil {
		t.Errorf("expected object to be fully deleted after finalizer removal, but Get succeeded")
	}
}

// TestDSNSReconciler_MultipleOwners verifies that records from multiple objects
// coexist in the zone and removing one does not affect the other.
func TestDSNSReconciler_MultipleOwners(t *testing.T) {
	t.Parallel()

	ip1 := newUnstructured(identityProvGVK, "idp-one", "seam-system")
	setField(ip1, "https://one.example.com", "status", "issuerURL")
	setCondition(ip1, "Valid", "True", "OK", "2026-04-06T00:00:00Z")

	ip2 := newUnstructured(identityProvGVK, "idp-two", "seam-system")
	setField(ip2, "https://two.example.com", "status", "issuerURL")
	setCondition(ip2, "Valid", "True", "OK", "2026-04-06T00:00:00Z")

	fc := newFakeClient(t, ip1, ip2)
	state := idns.NewDSNSState(fc)
	r := newDSNSReconciler(fc, identityProvGVK, state)

	reconcile(t, r, ip1)
	reconcile(t, r, ip2)

	zone := zoneContent(t, fc)
	if !strings.Contains(zone, "idp.idp-one.guardian") {
		t.Errorf("zone missing idp-one record:\n%s", zone)
	}
	if !strings.Contains(zone, "idp.idp-two.guardian") {
		t.Errorf("zone missing idp-two record:\n%s", zone)
	}

	// Delete ip1 via finalizer path.
	var fresh unstructured.Unstructured
	fresh.SetGroupVersionKind(identityProvGVK)
	_ = fc.Get(context.Background(), client.ObjectKey{Name: "idp-one", Namespace: "seam-system"}, &fresh)
	_ = fc.Delete(context.Background(), &fresh)
	var deleting unstructured.Unstructured
	deleting.SetGroupVersionKind(identityProvGVK)
	_ = fc.Get(context.Background(), client.ObjectKey{Name: "idp-one", Namespace: "seam-system"}, &deleting)
	reconcile(t, r, &deleting)

	zone = zoneContent(t, fc)
	if strings.Contains(zone, "idp.idp-one.guardian") {
		t.Errorf("zone still contains removed record:\n%s", zone)
	}
	if !strings.Contains(zone, "idp.idp-two.guardian") {
		t.Errorf("zone lost idp-two record after removing idp-one:\n%s", zone)
	}
}

// TestDSNSReconciler_IsNotFound_BestEffortCleanup verifies that when an object
// is no longer found (already fully deleted), the reconciler removes any records
// that were previously owned by it without returning an error.
func TestDSNSReconciler_IsNotFound_BestEffortCleanup(t *testing.T) {
	t.Parallel()

	// Pre-populate the zone with a record associated with an owner that is now gone.
	fc := newFakeClient(t) // empty — object does not exist
	state := idns.NewDSNSState(fc)

	// Manually seed the state as if a prior reconcile had added records.
	state.UpdateRecords("IdentityProvider/seam-system/orphaned", []idns.Record{
		{Name: "idp.orphaned.guardian", Type: idns.RecordTypeTXT, Value: "https://gone.example.com"},
	})
	// Write once so the ConfigMap exists.
	if err := state.Apply(context.Background()); err != nil {
		t.Fatalf("initial Apply: %v", err)
	}
	zone := zoneContent(t, fc)
	if !strings.Contains(zone, "idp.orphaned.guardian") {
		t.Fatalf("expected seeded record before cleanup:\n%s", zone)
	}

	// Now reconcile for the orphaned object (IsNotFound path).
	r := newDSNSReconciler(fc, identityProvGVK, state)
	req := ctrl.Request{NamespacedName: types.NamespacedName{Name: "orphaned", Namespace: "seam-system"}}
	if _, err := r.Reconcile(context.Background(), req); err != nil {
		t.Fatalf("Reconcile (IsNotFound path): %v", err)
	}

	zone = zoneContent(t, fc)
	if strings.Contains(zone, "idp.orphaned.guardian") {
		t.Errorf("zone still contains orphaned record after IsNotFound reconcile:\n%s", zone)
	}
}
