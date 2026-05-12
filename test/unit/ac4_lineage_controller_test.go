package unit_test

// AC-4: LineageController manifest tracking acceptance contract.
//
// AC-4: When a root declaration CR is created, the InfrastructureLineageController
// must:
//   - Create exactly one InfrastructureLineageIndex with the deterministic name
//     {lowercasekind}-{name} in the same namespace as the root declaration.
//   - Set governance.infrastructure.ontai.dev/lineage-index-ref annotation on the root.
//   - Transition LineageSynced from False/LineageControllerAbsent to True/LineageIndexCreated.
//   - Initialize DescendantRegistry as nil (empty at creation).
//   - Be idempotent: a second reconcile must not create a duplicate ILI.
//   - Cover all 9 root declaration GVKs registered in RootDeclarationGVKs.
//
// These tests constitute the acceptance contract gate for AC-4.
// seam-core-schema.md §3, CLAUDE.md §14 Decisions 3 and 4.

import (
	"context"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	seamv1alpha1 "github.com/ontai-dev/seam-core/api/v1alpha1"
	"github.com/ontai-dev/seam-core/internal/controller"
	seamconditions "github.com/ontai-dev/seam-core/pkg/conditions"
)

// buildAC4ReconcilerWithClient returns a reconciler and the fake client.
func buildAC4ReconcilerWithClient(t *testing.T, gvk schema.GroupVersionKind, root client.Object) (*controller.LineageReconciler, client.Client) {
	t.Helper()
	s := newTestScheme(t)
	c := fake.NewClientBuilder().
		WithScheme(s).
		WithObjects(root, newCRDObject(gvk, true)).
		WithStatusSubresource(root, &seamv1alpha1.LineageRecord{}).
		Build()
	return &controller.LineageReconciler{
		Client: c,
		Scheme: s,
		GVK:    gvk,
	}, c
}

// TestAC4_LineageReconciler_CreatesILIWithDeterministicName verifies that reconciling
// a root declaration creates an InfrastructureLineageIndex named {lowercasekind}-{name}
// in the same namespace, with rootBinding populated correctly.
// AC-4 gate: ILI creation and naming contract. CLAUDE.md §14 Decision 4.
func TestAC4_LineageReconciler_CreatesILIWithDeterministicName(t *testing.T) {
	root := newRootDeclaration(talosClusterGVK, "prod-cluster", "seam-system")
	r, c := buildAC4ReconcilerWithClient(t, talosClusterGVK, root)

	reconcileRoot(t, r, "prod-cluster", "seam-system")

	ili := &seamv1alpha1.LineageRecord{}
	if err := c.Get(context.Background(), client.ObjectKey{
		Name:      "taloscluster-prod-cluster",
		Namespace: "seam-system",
	}, ili); err != nil {
		t.Fatalf("AC-4: InfrastructureLineageIndex taloscluster-prod-cluster not found: %v", err)
	}
	if ili.Spec.RootBinding.RootKind != "TalosCluster" {
		t.Errorf("AC-4: ILI rootBinding.rootKind = %q, want TalosCluster", ili.Spec.RootBinding.RootKind)
	}
	if ili.Spec.RootBinding.RootName != "prod-cluster" {
		t.Errorf("AC-4: ILI rootBinding.rootName = %q, want prod-cluster", ili.Spec.RootBinding.RootName)
	}
	if ili.Spec.RootBinding.RootNamespace != "seam-system" {
		t.Errorf("AC-4: ILI rootBinding.rootNamespace = %q, want seam-system", ili.Spec.RootBinding.RootNamespace)
	}
	if ili.Spec.DescendantRegistry != nil {
		t.Errorf("AC-4: ILI descendantRegistry must be nil at creation, got %v", ili.Spec.DescendantRegistry)
	}
}

// TestAC4_LineageReconciler_TransitionsLineageSyncedToTrue verifies that reconciling
// a root declaration transitions its LineageSynced condition from
// False/LineageControllerAbsent to True/LineageIndexCreated.
// AC-4 gate: LineageSynced handoff contract. seam-core-schema.md §7 Declaration 5.
func TestAC4_LineageReconciler_TransitionsLineageSyncedToTrue(t *testing.T) {
	root := newRootDeclaration(clusterPackGVK, "base-pack", "infra-system")
	r, c := buildAC4ReconcilerWithClient(t, clusterPackGVK, root)

	reconcileRoot(t, r, "base-pack", "infra-system")

	// Re-fetch the root as unstructured to read status.conditions.
	updatedRoot := &unstructured.Unstructured{}
	updatedRoot.SetGroupVersionKind(clusterPackGVK)
	if err := c.Get(context.Background(), client.ObjectKey{Name: "base-pack", Namespace: "infra-system"}, updatedRoot); err != nil {
		t.Fatalf("AC-4: re-fetch root declaration: %v", err)
	}

	conditions, found, _ := unstructured.NestedSlice(updatedRoot.Object, "status", "conditions")
	if !found || len(conditions) == 0 {
		t.Fatalf("AC-4: LineageSynced condition not found on root declaration after reconcile")
	}

	var lineageSyncedCond map[string]interface{}
	for _, rawCond := range conditions {
		cond, ok := rawCond.(map[string]interface{})
		if !ok {
			continue
		}
		if cond["type"] == seamconditions.ConditionTypeLineageSynced {
			lineageSyncedCond = cond
			break
		}
	}
	if lineageSyncedCond == nil {
		t.Fatalf("AC-4: LineageSynced condition not found in status.conditions")
	}
	if status, _ := lineageSyncedCond["status"].(string); status != "True" {
		t.Errorf("AC-4: LineageSynced status = %q, want True", status)
	}
	if reason, _ := lineageSyncedCond["reason"].(string); reason != controller.ReasonLineageIndexCreated {
		t.Errorf("AC-4: LineageSynced reason = %q, want %q", reason, controller.ReasonLineageIndexCreated)
	}
}

// TestAC4_LineageReconciler_GovernanceAnnotationOnRoot verifies that the
// governance.infrastructure.ontai.dev/lineage-index-ref annotation is written
// onto the root declaration after reconcile. This annotation is the controller's
// idempotency guard and the cross-object reference to the ILI.
// AC-4 gate: governance annotation contract. CLAUDE.md §14 Decision 3.
func TestAC4_LineageReconciler_GovernanceAnnotationOnRoot(t *testing.T) {
	root := newRootDeclaration(rbacPolicyGVK, "platform-policy", "seam-system")
	r, c := buildAC4ReconcilerWithClient(t, rbacPolicyGVK, root)

	reconcileRoot(t, r, "platform-policy", "seam-system")

	updated := newRootDeclaration(rbacPolicyGVK, "platform-policy", "seam-system")
	if err := c.Get(context.Background(), client.ObjectKeyFromObject(root), updated); err != nil {
		t.Fatalf("AC-4: re-fetch root declaration: %v", err)
	}
	got := updated.GetAnnotations()[controller.GovernanceAnnotationLineageIndexRef]
	want := "rbacpolicy-platform-policy"
	if got != want {
		t.Errorf("AC-4: governance annotation = %q, want %q", got, want)
	}
}

// TestAC4_LineageReconciler_Idempotent verifies that a second reconcile does not
// create a duplicate ILI. The total ILI count in the namespace must be exactly 1
// after two reconcile calls. CLAUDE.md §14 Decision 4.
// AC-4 gate: idempotency contract.
func TestAC4_LineageReconciler_Idempotent(t *testing.T) {
	root := newRootDeclaration(packExecutionGVK, "exec-001", "infra-system")
	r, c := buildAC4ReconcilerWithClient(t, packExecutionGVK, root)

	reconcileRoot(t, r, "exec-001", "infra-system")
	reconcileRoot(t, r, "exec-001", "infra-system")

	iliList := &seamv1alpha1.LineageRecordList{}
	if err := c.List(context.Background(), iliList, client.InNamespace("infra-system")); err != nil {
		t.Fatalf("AC-4: list ILIs: %v", err)
	}
	if len(iliList.Items) != 1 {
		t.Errorf("AC-4: want exactly 1 ILI after second reconcile, got %d", len(iliList.Items))
	}
}

// TestAC4_AllRootDeclarationGVKsAreRegistered verifies that all root declaration GVKs
// named in the architecture are registered in RootDeclarationGVKs.
// A missing registration means the LineageController is blind to that CRD family.
// AC-4 gate: GVK coverage contract. seam-core-schema.md §3.
func TestAC4_AllNineRootDeclarationGVKsAreRegistered(t *testing.T) {
	required := []schema.GroupVersionKind{
		// Platform -- seam.ontai.dev (MIGRATION-3.1)
		{Group: "seam.ontai.dev", Version: "v1alpha1", Kind: "TalosCluster"},
		// Platform operational -- platform.ontai.dev
		{Group: "platform.ontai.dev", Version: "v1alpha1", Kind: "UpgradePolicy"},
		{Group: "platform.ontai.dev", Version: "v1alpha1", Kind: "NodeMaintenance"},
		{Group: "platform.ontai.dev", Version: "v1alpha1", Kind: "ClusterMaintenance"},
		{Group: "platform.ontai.dev", Version: "v1alpha1", Kind: "PKIRotation"},
		{Group: "platform.ontai.dev", Version: "v1alpha1", Kind: "ClusterReset"},
		{Group: "platform.ontai.dev", Version: "v1alpha1", Kind: "NodeOperation"},
		{Group: "platform.ontai.dev", Version: "v1alpha1", Kind: "EtcdMaintenance"},
		{Group: "platform.ontai.dev", Version: "v1alpha1", Kind: "TalosMachineConfigBackup"},
		{Group: "platform.ontai.dev", Version: "v1alpha1", Kind: "TalosMachineConfigRestore"},
		{Group: "platform.ontai.dev", Version: "v1alpha1", Kind: "HardeningProfile"},
		// Platform CAPI provider -- infrastructure.ontai.dev
		{Group: "infrastructure.ontai.dev", Version: "v1alpha1", Kind: "SeamInfrastructureCluster"},
		{Group: "infrastructure.ontai.dev", Version: "v1alpha1", Kind: "SeamInfrastructureMachine"},
		// Dispatcher -- seam.ontai.dev (MIGRATION-3.3-3.7)
		{Group: "seam.ontai.dev", Version: "v1alpha1", Kind: "PackDelivery"},
		{Group: "seam.ontai.dev", Version: "v1alpha1", Kind: "PackExecution"},
		{Group: "seam.ontai.dev", Version: "v1alpha1", Kind: "PackInstalled"},
		// Guardian -- guardian.ontai.dev (constitutional refactor 2026-05-12)
		{Group: "guardian.ontai.dev", Version: "v1alpha1", Kind: "RBACPolicy"},
		{Group: "guardian.ontai.dev", Version: "v1alpha1", Kind: "RBACProfile"},
		{Group: "guardian.ontai.dev", Version: "v1alpha1", Kind: "IdentityBinding"},
		{Group: "guardian.ontai.dev", Version: "v1alpha1", Kind: "IdentityProvider"},
		{Group: "guardian.ontai.dev", Version: "v1alpha1", Kind: "PermissionSet"},
	}

	if len(controller.RootDeclarationGVKs) != len(required) {
		t.Errorf("AC-4: RootDeclarationGVKs has %d entries, want %d",
			len(controller.RootDeclarationGVKs), len(required))
	}

	registered := make(map[schema.GroupVersionKind]bool, len(controller.RootDeclarationGVKs))
	for _, gvk := range controller.RootDeclarationGVKs {
		registered[gvk] = true
	}

	for _, want := range required {
		if !registered[want] {
			t.Errorf("AC-4: GVK %v not registered in RootDeclarationGVKs", want)
		}
	}
}

