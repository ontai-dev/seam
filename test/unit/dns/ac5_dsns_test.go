package dns_test

// AC-5: DSNS lineage tracking in seam.ontave.dev acceptance contract.
//
// AC-5: When a root declaration CR is reconciled by DSNSReconciler, the
// seam.ontave.dev zone (projected via dsns-zone ConfigMap in ont-system) must
// contain a correctly typed DNS record whose category matches the root declaration's
// domain role:
//
//   - TalosCluster Ready        -> cluster-topology record (A or TXT with IN records)
//   - PackInstance Succeeded    -> pack-lineage record (TXT)
//   - IdentityBinding resolved  -> identity-plane record (TXT)
//   - RunnerConfig completed    -> execution-authority record (TXT)
//   - Root deletion             -> record removed from zone
//
// The zone's $ORIGIN must be seam.ontave.dev. and contain SOA + NS records
// regardless of managed state. DSNSGVKs must include all five required GVKs.
//
// seam-core-schema.md §8 Decisions 1 and 4.

import (
	"context"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/ontai-dev/seam/internal/controller"
	idns "github.com/ontai-dev/seam/internal/dns"
)

// reconcileAC5 reconciles obj and fails the test on error.
func reconcileAC5(t *testing.T, r *controller.DSNSReconciler, obj *unstructured.Unstructured) {
	t.Helper()
	req := ctrl.Request{NamespacedName: types.NamespacedName{Name: obj.GetName(), Namespace: obj.GetNamespace()}}
	if _, err := r.Reconcile(context.Background(), req); err != nil {
		t.Fatalf("AC-5: Reconcile: %v", err)
	}
}

// zoneAC5 returns the rendered dsns-zone zone content for assertion.
func zoneAC5(t *testing.T, fc client.Client) string {
	t.Helper()
	return zoneContent(t, fc)
}

// ---------------------------------------------------------------------------
// Test 1 -- TalosCluster Ready -> cluster-topology records in seam.ontave.dev
// ---------------------------------------------------------------------------

// TestAC5_DSNS_TalosCluster_Ready_ProducesClusterTopologyRecord verifies that a
// Ready TalosCluster produces DNS records in the seam.ontave.dev zone. The zone
// must contain at least one A or TXT record for the cluster name.
// AC-5 gate: cluster-topology projection contract. seam-core-schema.md §8.
func TestAC5_DSNS_TalosCluster_Ready_ProducesClusterTopologyRecord(t *testing.T) {
	t.Parallel()
	tc := newUnstructured(schema.GroupVersionKind{Group: "seam.ontai.dev", Version: "v1alpha1", Kind: "TalosCluster"},
		"ccs-mgmt", "ont-system")
	setField(tc, "10.10.0.1", "spec", "clusterEndpoint")
	setField(tc, "bootstrapped", "status", "origin")
	setCondition(tc, "Ready", "True", "ClusterReady", "2026-04-20T00:00:00Z")

	fc := newFakeClient(t, tc)
	state := idns.NewDSNSState(fc)
	r := newDSNSReconciler(fc, tc.GroupVersionKind(), state)

	reconcileAC5(t, r, tc)

	zone := zoneAC5(t, fc)
	if !strings.Contains(zone, "ccs-mgmt") {
		t.Errorf("AC-5: seam.ontave.dev zone must contain ccs-mgmt record after TalosCluster Ready; zone:\n%s", zone)
	}
	if !strings.Contains(zone, "$ORIGIN seam.ontave.dev.") {
		t.Errorf("AC-5: zone missing $ORIGIN seam.ontave.dev.; zone:\n%s", zone)
	}
}

// ---------------------------------------------------------------------------
// Test 2 -- PackInstance Succeeded -> pack-lineage record in seam.ontave.dev
// ---------------------------------------------------------------------------

// TestAC5_DSNS_PackInstance_Succeeded_ProducesPackLineageRecord verifies that a
// Succeeded PackInstance produces a pack-lineage TXT record in the zone.
// AC-5 gate: pack-lineage projection contract. seam-core-schema.md §8.
func TestAC5_DSNS_PackInstance_Succeeded_ProducesPackLineageRecord(t *testing.T) {
	t.Parallel()
	pi := newUnstructured(schema.GroupVersionKind{Group: "seam.ontai.dev", Version: "v1alpha1", Kind: "PackInstalled"},
		"cilium-v1.0.0", "seam-tenant-ccs-dev")
	setField(pi, "cilium", "spec", "clusterPackRef")
	setField(pi, "ccs-dev", "spec", "targetClusterRef")
	setField(pi, "sha256:cafebabe", "status", "receiptDigest")
	setCondition(pi, "Ready", "True", "PackReceiptReady", "2026-04-20T00:00:00Z")

	fc := newFakeClient(t, pi)
	state := idns.NewDSNSState(fc)
	r := newDSNSReconciler(fc, pi.GroupVersionKind(), state)

	reconcileAC5(t, r, pi)

	zone := zoneAC5(t, fc)
	if !strings.Contains(zone, "pack.cilium") {
		t.Errorf("AC-5: zone must contain pack.cilium pack-lineage record; zone:\n%s", zone)
	}
}

// ---------------------------------------------------------------------------
// Test 3 -- IdentityBinding resolved -> identity-plane record in seam.ontave.dev
// ---------------------------------------------------------------------------

// TestAC5_DSNS_IdentityBinding_Resolved_ProducesIdentityPlaneRecord verifies that
// a resolved IdentityBinding produces an identity-plane TXT record in the zone.
// AC-5 gate: identity-plane projection contract. seam-core-schema.md §8.
func TestAC5_DSNS_IdentityBinding_Resolved_ProducesIdentityPlaneRecord(t *testing.T) {
	t.Parallel()
	ib := newUnstructured(schema.GroupVersionKind{Group: "guardian.ontai.dev", Version: "v1alpha1", Kind: "IdentityBinding"},
		"alice-binding", "seam-tenant-ccs-dev")
	setField(ib, "alice@example.com", "spec", "subject")
	setField(ib, "admin-profile", "spec", "rbacProfileRef", "name")
	setField(ib, "okta-provider", "spec", "identityProviderRef", "name")
	setCondition(ib, "TrustAnchorResolved", "True", "TrustAnchorResolved", "2026-04-20T00:00:00Z")

	fc := newFakeClient(t, ib)
	state := idns.NewDSNSState(fc)
	r := newDSNSReconciler(fc, ib.GroupVersionKind(), state)

	reconcileAC5(t, r, ib)

	zone := zoneAC5(t, fc)
	if !strings.Contains(zone, "identity.") || !strings.Contains(zone, "guardian.ccs-dev") {
		t.Errorf("AC-5: zone must contain identity-plane record with guardian.ccs-dev suffix; zone:\n%s", zone)
	}
}

// ---------------------------------------------------------------------------
// Test 4 -- RunnerConfig completed -> execution-authority record
// ---------------------------------------------------------------------------

// TestAC5_DSNS_RunnerConfig_Completed_ProducesExecutionAuthorityRecord verifies
// that a completed RunnerConfig produces an execution-authority TXT record in zone.
// AC-5 gate: execution-authority projection contract. seam-core-schema.md §8.
func TestAC5_DSNS_RunnerConfig_Completed_ProducesExecutionAuthorityRecord(t *testing.T) {
	t.Parallel()
	rc := newUnstructured(schema.GroupVersionKind{Group: "seam.ontai.dev", Version: "v1alpha1", Kind: "RunnerConfig"},
		"ccs-dev", "ont-system")
	setField(rc, []interface{}{"talos-apply", "helm-install"}, "spec", "capabilities")
	setCondition(rc, "Ready", "True", "CapabilitiesPublished", "2026-04-20T00:00:00Z")

	fc := newFakeClient(t, rc)
	state := idns.NewDSNSState(fc)
	r := newDSNSReconciler(fc, rc.GroupVersionKind(), state)

	reconcileAC5(t, r, rc)

	zone := zoneAC5(t, fc)
	if !strings.Contains(zone, "ccs-dev") {
		t.Errorf("AC-5: zone must contain ccs-dev execution-authority record; zone:\n%s", zone)
	}
}

// ---------------------------------------------------------------------------
// Test 5 -- Zone always contains SOA and NS regardless of managed records
// ---------------------------------------------------------------------------

// TestAC5_DSNS_Zone_AlwaysContainsSOAandNS verifies that the seam.ontave.dev zone
// contains an SOA record and a zone-level NS record even when no managed records
// exist. This is required for the zone to be a valid authoritative zone file.
// AC-5 gate: zone integrity contract. seam-core-schema.md §8 Decision 2.
func TestAC5_DSNS_Zone_AlwaysContainsSOAandNS(t *testing.T) {
	t.Parallel()
	fc := newFakeClient(t)
	state := idns.NewDSNSState(fc)
	// Apply an empty event to force the zone ConfigMap to be written with SOA+NS.
	if err := state.Apply(context.Background()); err != nil {
		t.Fatalf("AC-5: DSNSState.Apply: %v", err)
	}

	zone := zoneAC5(t, fc)
	if !strings.Contains(zone, "IN SOA") {
		t.Errorf("AC-5: zone missing SOA record; zone:\n%s", zone)
	}
	if !strings.Contains(zone, "IN NS") {
		t.Errorf("AC-5: zone missing NS record; zone:\n%s", zone)
	}
}

// ---------------------------------------------------------------------------
// Test 6 -- DSNSGVKs contains all five required GVKs
// ---------------------------------------------------------------------------

// TestAC5_DSNSGVKs_ContainsAllRequiredGVKs verifies that DSNSGVKs includes all
// five root declaration GVKs that project to seam.ontave.dev records. A missing
// GVK means that CRD family produces no DNS projection.
// AC-5 gate: GVK coverage contract. seam-core-schema.md §8 Decision 4.
func TestAC5_DSNSGVKs_ContainsAllRequiredGVKs(t *testing.T) {
	required := []schema.GroupVersionKind{
		{Group: "seam.ontai.dev", Version: "v1alpha1", Kind: "TalosCluster"},
		{Group: "seam.ontai.dev", Version: "v1alpha1", Kind: "PackInstalled"},
		{Group: "guardian.ontai.dev", Version: "v1alpha1", Kind: "IdentityBinding"},
		{Group: "guardian.ontai.dev", Version: "v1alpha1", Kind: "IdentityProvider"},
		{Group: "seam.ontai.dev", Version: "v1alpha1", Kind: "RunnerConfig"},
	}

	registered := make(map[schema.GroupVersionKind]bool, len(controller.DSNSGVKs))
	for _, gvk := range controller.DSNSGVKs {
		registered[gvk] = true
	}

	for _, want := range required {
		if !registered[want] {
			t.Errorf("AC-5: GVK %v not in DSNSGVKs — no DNS projection for this CRD family", want)
		}
	}
}
