package unit_test

import (
	"context"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	seamv1alpha1 "github.com/ontai-dev/seam/api/v1alpha1"
	"github.com/ontai-dev/seam/internal/controller"
)

// rootGVKs used in tests -- covers one from each operator family.
var (
	talosClusterGVK  = schema.GroupVersionKind{Group: "seam.ontai.dev", Version: "v1alpha1", Kind: "TalosCluster"}
	clusterPackGVK   = schema.GroupVersionKind{Group: "seam.ontai.dev", Version: "v1alpha1", Kind: "PackDelivery"}
	rbacPolicyGVK    = schema.GroupVersionKind{Group: "guardian.ontai.dev", Version: "v1alpha1", Kind: "RBACPolicy"}
	packExecutionGVK = schema.GroupVersionKind{Group: "seam.ontai.dev", Version: "v1alpha1", Kind: "PackExecution"}
	identityBindGVK  = schema.GroupVersionKind{Group: "guardian.ontai.dev", Version: "v1alpha1", Kind: "IdentityBinding"}
	packInstanceGVK  = schema.GroupVersionKind{Group: "seam.ontai.dev", Version: "v1alpha1", Kind: "PackInstalled"}
)

func newTestScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	s := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(s); err != nil {
		t.Fatalf("clientgoscheme: %v", err)
	}
	if err := seamv1alpha1.AddToScheme(s); err != nil {
		t.Fatalf("seamv1alpha1: %v", err)
	}
	return s
}

func newRootDeclaration(gvk schema.GroupVersionKind, name, namespace string) *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(gvk)
	u.SetName(name)
	u.SetNamespace(namespace)
	u.SetUID(types.UID("uid-" + name))
	u.SetGeneration(1)
	// Initialize LineageSynced=False/LineageControllerAbsent as operators do.
	_ = unstructured.SetNestedSlice(u.Object, []interface{}{
		map[string]interface{}{
			"type":               "LineageSynced",
			"status":             "False",
			"reason":             "LineageControllerAbsent",
			"message":            "LineageController is not yet deployed.",
			"lastTransitionTime": metav1.Now().UTC().Format("2006-01-02T15:04:05Z"),
		},
	}, "status", "conditions")
	return u
}

func reconcileRoot(t *testing.T, r *controller.LineageReconciler, name, namespace string) ctrl.Result {
	t.Helper()
	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: name, Namespace: namespace},
	})
	if err != nil {
		t.Fatalf("Reconcile returned unexpected error: %v", err)
	}
	return result
}

// TestLineageReconciler_CreatesILIForTalosCluster verifies that reconciling a
// TalosCluster root declaration creates a LineageRecord with
// the correct rootBinding.
func TestLineageReconciler_CreatesILIForTalosCluster(t *testing.T) {
	s := newTestScheme(t)
	root := newRootDeclaration(talosClusterGVK, "prod-cluster", "ont-system")

	fakeClient := fake.NewClientBuilder().WithScheme(s).
		WithObjects(root, newCRDObject(talosClusterGVK, true)).
		WithStatusSubresource(root, &seamv1alpha1.LineageRecord{}).
		Build()

	r := &controller.LineageReconciler{
		Client: fakeClient,
		Scheme: s,
		GVK:    talosClusterGVK,
	}

	result := reconcileRoot(t, r, "prod-cluster", "ont-system")
	if result.RequeueAfter != 0 {
		t.Errorf("expected no requeue, got %+v", result)
	}

	// Verify LineageRecord was created.
	ili := &seamv1alpha1.LineageRecord{}
	iliKey := client.ObjectKey{Name: "taloscluster-prod-cluster", Namespace: "ont-system"}
	if err := fakeClient.Get(context.Background(), iliKey, ili); err != nil {
		t.Fatalf("expected LineageRecord to exist: %v", err)
	}

	// Verify rootBinding.
	rb := ili.Spec.RootBinding
	if rb.RootKind != "TalosCluster" {
		t.Errorf("expected RootKind=TalosCluster, got %q", rb.RootKind)
	}
	if rb.RootName != "prod-cluster" {
		t.Errorf("expected RootName=prod-cluster, got %q", rb.RootName)
	}
	if rb.RootNamespace != "ont-system" {
		t.Errorf("expected RootNamespace=ont-system, got %q", rb.RootNamespace)
	}
	if rb.RootUID != types.UID("uid-prod-cluster") {
		t.Errorf("expected RootUID=uid-prod-cluster, got %q", rb.RootUID)
	}
	if rb.RootObservedGeneration != 1 {
		t.Errorf("expected RootObservedGeneration=1, got %d", rb.RootObservedGeneration)
	}
}

// TestLineageReconciler_GovernanceAnnotationSet verifies that the governance
// annotation is written on the root declaration after ILI creation.
func TestLineageReconciler_GovernanceAnnotationSet(t *testing.T) {
	s := newTestScheme(t)
	root := newRootDeclaration(clusterPackGVK, "my-pack", "infra-system")

	fakeClient := fake.NewClientBuilder().WithScheme(s).
		WithObjects(root, newCRDObject(clusterPackGVK, true)).
		WithStatusSubresource(root, &seamv1alpha1.LineageRecord{}).
		Build()

	r := &controller.LineageReconciler{
		Client: fakeClient,
		Scheme: s,
		GVK:    clusterPackGVK,
	}

	reconcileRoot(t, r, "my-pack", "infra-system")

	// Re-fetch root to check governance annotation.
	updatedRoot := &unstructured.Unstructured{}
	updatedRoot.SetGroupVersionKind(clusterPackGVK)
	if err := fakeClient.Get(context.Background(),
		client.ObjectKey{Name: "my-pack", Namespace: "infra-system"}, updatedRoot); err != nil {
		t.Fatalf("get updated root: %v", err)
	}

	annotations := updatedRoot.GetAnnotations()
	iliRef, ok := annotations[controller.GovernanceAnnotationLineageIndexRef]
	if !ok {
		t.Fatalf("expected governance annotation %q to be set", controller.GovernanceAnnotationLineageIndexRef)
	}
	expectedILIName := "packdelivery-my-pack"
	if iliRef != expectedILIName {
		t.Errorf("expected annotation value %q, got %q", expectedILIName, iliRef)
	}
}

// TestLineageReconciler_LineageSyncedTransitionedToTrue verifies that the
// LineageSynced condition is set to True after ILI creation.
func TestLineageReconciler_LineageSyncedTransitionedToTrue(t *testing.T) {
	s := newTestScheme(t)
	root := newRootDeclaration(rbacPolicyGVK, "platform-policy", "security-system")

	fakeClient := fake.NewClientBuilder().WithScheme(s).
		WithObjects(root, newCRDObject(rbacPolicyGVK, true)).
		WithStatusSubresource(root, &seamv1alpha1.LineageRecord{}).
		Build()

	r := &controller.LineageReconciler{
		Client: fakeClient,
		Scheme: s,
		GVK:    rbacPolicyGVK,
	}

	reconcileRoot(t, r, "platform-policy", "security-system")

	// Re-fetch root to check LineageSynced condition.
	updatedRoot := &unstructured.Unstructured{}
	updatedRoot.SetGroupVersionKind(rbacPolicyGVK)
	if err := fakeClient.Get(context.Background(),
		client.ObjectKey{Name: "platform-policy", Namespace: "security-system"}, updatedRoot); err != nil {
		t.Fatalf("get updated root: %v", err)
	}

	conditions, found, _ := unstructured.NestedSlice(updatedRoot.Object, "status", "conditions")
	if !found || len(conditions) == 0 {
		t.Fatal("expected status.conditions to be set")
	}

	var lineageSyncedFound bool
	for _, rawCond := range conditions {
		cond, ok := rawCond.(map[string]interface{})
		if !ok {
			continue
		}
		if cond["type"] == "LineageSynced" {
			lineageSyncedFound = true
			if cond["status"] != "True" {
				t.Errorf("expected LineageSynced=True, got %v", cond["status"])
			}
			if cond["reason"] != controller.ReasonLineageIndexCreated {
				t.Errorf("expected reason=%q, got %v", controller.ReasonLineageIndexCreated, cond["reason"])
			}
		}
	}
	if !lineageSyncedFound {
		t.Error("LineageSynced condition not found in status.conditions")
	}
}

// TestLineageReconciler_NotFound_NoError verifies that a reconcile request for a
// deleted root declaration returns no error and no requeue.
func TestLineageReconciler_NotFound_NoError(t *testing.T) {
	s := newTestScheme(t)
	// No root declaration object in the fake client.
	fakeClient := fake.NewClientBuilder().WithScheme(s).Build()

	r := &controller.LineageReconciler{
		Client: fakeClient,
		Scheme: s,
		GVK:    packExecutionGVK,
	}

	result := reconcileRoot(t, r, "missing-exec", "infra-system")
	if result.RequeueAfter != 0 {
		t.Errorf("expected no requeue for not-found, got %+v", result)
	}
	// No ILI should be created.
	iliList := &seamv1alpha1.LineageRecordList{}
	if err := fakeClient.List(context.Background(), iliList); err != nil {
		t.Fatalf("list ILIs: %v", err)
	}
	if len(iliList.Items) != 0 {
		t.Errorf("expected no ILI created for not-found root, got %d", len(iliList.Items))
	}
}

// TestLineageReconciler_Idempotent verifies that a second reconcile on a root
// declaration that already has the governance annotation does not create a
// duplicate ILI and returns no error.
func TestLineageReconciler_Idempotent(t *testing.T) {
	s := newTestScheme(t)
	root := newRootDeclaration(identityBindGVK, "admin-binding", "security-system")

	fakeClient := fake.NewClientBuilder().WithScheme(s).
		WithObjects(root, newCRDObject(identityBindGVK, true)).
		WithStatusSubresource(root, &seamv1alpha1.LineageRecord{}).
		Build()

	r := &controller.LineageReconciler{
		Client: fakeClient,
		Scheme: s,
		GVK:    identityBindGVK,
	}

	// First reconcile — creates ILI.
	reconcileRoot(t, r, "admin-binding", "security-system")

	// Second reconcile — should be idempotent.
	reconcileRoot(t, r, "admin-binding", "security-system")

	// Only one ILI should exist.
	iliList := &seamv1alpha1.LineageRecordList{}
	if err := fakeClient.List(context.Background(), iliList,
		client.InNamespace("security-system")); err != nil {
		t.Fatalf("list ILIs: %v", err)
	}
	if len(iliList.Items) != 1 {
		t.Errorf("expected exactly 1 ILI after two reconciles, got %d", len(iliList.Items))
	}
}

// TestLineageReconciler_ILIRootBindingImmutable verifies that if an ILI already
// exists with different rootBinding fields, the controller does not overwrite it.
// (The admission webhook enforces immutability; the controller must not attempt
// to mutate an existing rootBinding.)
func TestLineageReconciler_ILIRootBindingImmutable(t *testing.T) {
	s := newTestScheme(t)
	root := newRootDeclaration(clusterPackGVK, "pack-v2", "infra-system")
	// Pre-populate governance annotation to simulate prior reconcile.
	root.SetAnnotations(map[string]string{
		controller.GovernanceAnnotationLineageIndexRef: "packdelivery-pack-v2",
	})

	// Create an existing ILI with matching rootBinding.
	existingILI := &seamv1alpha1.LineageRecord{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "packdelivery-pack-v2",
			Namespace: "infra-system",
		},
		Spec: seamv1alpha1.LineageRecordSpec{
			RootBinding: seamv1alpha1.LineageRecordRootBinding{
				RootKind:               "PackDelivery",
				RootName:               "pack-v2",
				RootNamespace:          "infra-system",
				RootUID:                types.UID("uid-pack-v2"),
				RootObservedGeneration: 1,
			},
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(s).
		WithObjects(root, existingILI, newCRDObject(clusterPackGVK, true)).
		WithStatusSubresource(root, &seamv1alpha1.LineageRecord{}).
		Build()

	r := &controller.LineageReconciler{
		Client: fakeClient,
		Scheme: s,
		GVK:    clusterPackGVK,
	}

	// Reconcile should succeed without attempting to recreate or modify the ILI.
	result := reconcileRoot(t, r, "pack-v2", "infra-system")
	if result.RequeueAfter != 0 {
		t.Errorf("expected no requeue on idempotent reconcile, got %+v", result)
	}

	// ILI rootBinding must be unchanged.
	ili := &seamv1alpha1.LineageRecord{}
	if err := fakeClient.Get(context.Background(),
		client.ObjectKey{Name: "packdelivery-pack-v2", Namespace: "infra-system"}, ili); err != nil {
		t.Fatalf("get ILI: %v", err)
	}
	if ili.Spec.RootBinding.RootUID != types.UID("uid-pack-v2") {
		t.Errorf("ILI rootBinding.RootUID was modified — immutability violated")
	}
}

// TestLineageReconciler_ControllerAuthoredAnnotation verifies that newly created
// LineageRecord instances carry the controller-authored governance
// annotation per CLAUDE.md §14 Decision 3.
func TestLineageReconciler_ControllerAuthoredAnnotation(t *testing.T) {
	s := newTestScheme(t)
	root := newRootDeclaration(rbacPolicyGVK, "tenant-policy", "security-system")

	fakeClient := fake.NewClientBuilder().WithScheme(s).
		WithObjects(root, newCRDObject(rbacPolicyGVK, true)).
		WithStatusSubresource(root, &seamv1alpha1.LineageRecord{}).
		Build()

	r := &controller.LineageReconciler{
		Client: fakeClient,
		Scheme: s,
		GVK:    rbacPolicyGVK,
	}

	reconcileRoot(t, r, "tenant-policy", "security-system")

	ili := &seamv1alpha1.LineageRecord{}
	if err := fakeClient.Get(context.Background(),
		client.ObjectKey{Name: "rbacpolicy-tenant-policy", Namespace: "security-system"}, ili); err != nil {
		t.Fatalf("get ILI: %v", err)
	}
	annotations := ili.GetAnnotations()
	if annotations[controller.GovernanceAnnotationControllerAuthored] != "true" {
		t.Errorf("expected %q=true annotation on ILI, got %q",
			controller.GovernanceAnnotationControllerAuthored,
			annotations[controller.GovernanceAnnotationControllerAuthored])
	}
}

// TestLineageReconciler_AllRootDeclarationGVKsRegistered verifies the RootDeclarationGVKs
// list contains all expected GVKs across all Seam operator domains.
func TestLineageReconciler_AllRootDeclarationGVKsRegistered(t *testing.T) {
	expected := map[string]bool{
		// Platform infra
		"seam.ontai.dev/v1alpha1/TalosCluster": false,
		// Platform operational
		"platform.ontai.dev/v1alpha1/UpgradePolicy":              false,
		"platform.ontai.dev/v1alpha1/NodeMaintenance":            false,
		"platform.ontai.dev/v1alpha1/ClusterMaintenance":         false,
		"platform.ontai.dev/v1alpha1/PKIRotation":                false,
		"platform.ontai.dev/v1alpha1/ClusterReset":               false,
		"platform.ontai.dev/v1alpha1/NodeOperation":              false,
		"platform.ontai.dev/v1alpha1/EtcdMaintenance":            false,
		"platform.ontai.dev/v1alpha1/TalosMachineConfigBackup":   false,
		"platform.ontai.dev/v1alpha1/TalosMachineConfigRestore":  false,
		"platform.ontai.dev/v1alpha1/HardeningProfile":           false,
		// Platform CAPI provider
		"infrastructure.ontai.dev/v1alpha1/SeamInfrastructureCluster": false,
		"infrastructure.ontai.dev/v1alpha1/SeamInfrastructureMachine": false,
		// Dispatcher (MIGRATION-3.3-3.7)
		"seam.ontai.dev/v1alpha1/PackDelivery": false,
		"seam.ontai.dev/v1alpha1/PackExecution": false,
		"seam.ontai.dev/v1alpha1/PackInstalled": false,
		// Guardian (guardian.ontai.dev -- constitutional refactor 2026-05-12)
		"guardian.ontai.dev/v1alpha1/RBACPolicy":     false,
		"guardian.ontai.dev/v1alpha1/RBACProfile":    false,
		"guardian.ontai.dev/v1alpha1/IdentityBinding": false,
		"guardian.ontai.dev/v1alpha1/IdentityProvider": false,
		"guardian.ontai.dev/v1alpha1/PermissionSet":   false,
	}

	for _, gvk := range controller.RootDeclarationGVKs {
		key := gvk.Group + "/" + gvk.Version + "/" + gvk.Kind
		if _, ok := expected[key]; !ok {
			t.Errorf("unexpected GVK in RootDeclarationGVKs: %s", key)
			continue
		}
		expected[key] = true
	}

	for key, found := range expected {
		if !found {
			t.Errorf("expected GVK missing from RootDeclarationGVKs: %s", key)
		}
	}

	if len(controller.RootDeclarationGVKs) != 21 {
		t.Errorf("expected 21 GVKs, got %d", len(controller.RootDeclarationGVKs))
	}
}

// --- Retention policy tests ---

// newILIWithDescendants builds a LineageRecord with a pre-populated
// DescendantRegistry for retention tests. Each entry's CreatedAt is set to the
// provided age ago.
func newILIWithDescendants(t *testing.T, name, namespace string, entries []seamv1alpha1.DescendantEntry) *seamv1alpha1.LineageRecord {
	t.Helper()
	return &seamv1alpha1.LineageRecord{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Annotations: map[string]string{
				controller.GovernanceAnnotationControllerAuthored: "true",
			},
		},
		Spec: seamv1alpha1.LineageRecordSpec{
			RootBinding: seamv1alpha1.LineageRecordRootBinding{
				RootKind:      "TalosCluster",
				RootName:      name,
				RootNamespace: namespace,
			},
			DescendantRegistry: entries,
		},
	}
}

// TestLineageReconciler_RetentionPrunesStaleEntry verifies that a descendant entry
// whose referenced object is not-found AND whose CreatedAt is older than the retention
// window is pruned from the DescendantRegistry on the next reconcile cycle.
func TestLineageReconciler_RetentionPrunesStaleEntry(t *testing.T) {
	s := newTestScheme(t)
	const ns = "test-ns"
	root := newRootDeclaration(talosClusterGVK, "cluster-a", ns)

	// Build an ILI with one stale entry: CreatedAt 31 days ago, retention=30 days.
	// The referenced object (some-runnerconfig) does not exist in the fake client.
	staleTime := metav1.NewTime(time.Now().Add(-31 * 24 * time.Hour))
	ili := newILIWithDescendants(t, "taloscluster-cluster-a", ns, []seamv1alpha1.DescendantEntry{
		{
			Group:             "seam.ontai.dev",
			Version:           "v1alpha1",
			Kind:              "RunnerConfig",
			Name:              "some-runnerconfig",
			Namespace:         ns,
			UID:               "uid-rc-1",
			SeamOperator:      "conductor",
			CreatedAt:        &staleTime,
		},
	})
	ili.Spec.RetentionPolicy = &seamv1alpha1.LineageRetentionPolicy{
		DescendantRetentionDays: 30,
		DeleteWithRoot:          false,
	}

	fakeClient := fake.NewClientBuilder().WithScheme(s).
		WithObjects(root, ili, newCRDObject(talosClusterGVK, true)).
		WithStatusSubresource(root, ili).
		Build()

	r := &controller.LineageReconciler{
		Client: fakeClient,
		Scheme: s,
		GVK:    talosClusterGVK,
	}

	reconcileRoot(t, r, "cluster-a", ns)

	// After reconcile the stale entry must have been pruned.
	updatedILI := &seamv1alpha1.LineageRecord{}
	if err := fakeClient.Get(context.Background(), client.ObjectKey{Name: "taloscluster-cluster-a", Namespace: ns}, updatedILI); err != nil {
		t.Fatalf("get ILI: %v", err)
	}
	if len(updatedILI.Spec.DescendantRegistry) != 0 {
		t.Errorf("expected empty DescendantRegistry after pruning stale entry, got %d entries", len(updatedILI.Spec.DescendantRegistry))
	}
}

// TestLineageReconciler_RetentionKeepsEntryWithinWindow verifies that a descendant
// entry whose CreatedAt is within the retention window is NOT pruned even when the
// referenced object is not-found.
func TestLineageReconciler_RetentionKeepsEntryWithinWindow(t *testing.T) {
	s := newTestScheme(t)
	const ns = "test-ns"
	root := newRootDeclaration(talosClusterGVK, "cluster-b", ns)

	// CreatedAt 5 days ago, retention=30 days — entry is within window.
	recentTime := metav1.NewTime(time.Now().Add(-5 * 24 * time.Hour))
	ili := newILIWithDescendants(t, "taloscluster-cluster-b", ns, []seamv1alpha1.DescendantEntry{
		{
			Group:        "seam.ontai.dev",
			Version:      "v1alpha1",
			Kind:         "RunnerConfig",
			Name:         "rc-recent",
			Namespace:    ns,
			UID:          "uid-rc-2",
			SeamOperator: "conductor",
			CreatedAt:   &recentTime,
		},
	})
	ili.Spec.RetentionPolicy = &seamv1alpha1.LineageRetentionPolicy{
		DescendantRetentionDays: 30,
		DeleteWithRoot:          false,
	}

	fakeClient := fake.NewClientBuilder().WithScheme(s).
		WithObjects(root, ili, newCRDObject(talosClusterGVK, true)).
		WithStatusSubresource(root, ili).
		Build()

	r := &controller.LineageReconciler{
		Client: fakeClient,
		Scheme: s,
		GVK:    talosClusterGVK,
	}

	reconcileRoot(t, r, "cluster-b", ns)

	// Entry must still be present.
	updatedILI := &seamv1alpha1.LineageRecord{}
	if err := fakeClient.Get(context.Background(), client.ObjectKey{Name: "taloscluster-cluster-b", Namespace: ns}, updatedILI); err != nil {
		t.Fatalf("get ILI: %v", err)
	}
	if len(updatedILI.Spec.DescendantRegistry) != 1 {
		t.Errorf("expected 1 entry kept within retention window, got %d", len(updatedILI.Spec.DescendantRegistry))
	}
}

// TestLineageReconciler_RetentionKeepsEntryWithNilCreatedAt verifies that an entry
// without a CreatedAt timestamp (pre-retention-tracking entry) is never pruned.
func TestLineageReconciler_RetentionKeepsEntryWithNilCreatedAt(t *testing.T) {
	s := newTestScheme(t)
	const ns = "test-ns"
	root := newRootDeclaration(talosClusterGVK, "cluster-c", ns)

	// CreatedAt is nil — entry predates retention tracking.
	ili := newILIWithDescendants(t, "taloscluster-cluster-c", ns, []seamv1alpha1.DescendantEntry{
		{
			Group:        "seam.ontai.dev",
			Version:      "v1alpha1",
			Kind:         "RunnerConfig",
			Name:         "rc-old",
			Namespace:    ns,
			UID:          "uid-rc-3",
			SeamOperator: "conductor",
			CreatedAt:   nil,
		},
	})
	ili.Spec.RetentionPolicy = &seamv1alpha1.LineageRetentionPolicy{
		DescendantRetentionDays: 1,
		DeleteWithRoot:          false,
	}

	fakeClient := fake.NewClientBuilder().WithScheme(s).
		WithObjects(root, ili, newCRDObject(talosClusterGVK, true)).
		WithStatusSubresource(root, ili).
		Build()

	r := &controller.LineageReconciler{
		Client: fakeClient,
		Scheme: s,
		GVK:    talosClusterGVK,
	}

	reconcileRoot(t, r, "cluster-c", ns)

	updatedILI := &seamv1alpha1.LineageRecord{}
	if err := fakeClient.Get(context.Background(), client.ObjectKey{Name: "taloscluster-cluster-c", Namespace: ns}, updatedILI); err != nil {
		t.Fatalf("get ILI: %v", err)
	}
	// Entry without CreatedAt must never be pruned.
	if len(updatedILI.Spec.DescendantRegistry) != 1 {
		t.Errorf("expected entry with nil CreatedAt to be kept, got %d entries", len(updatedILI.Spec.DescendantRegistry))
	}
}

// TestLineageReconciler_DeleteWithRoot_AddsOwnerReference verifies that when
// RetentionPolicy.DeleteWithRoot=true the ILI gains an ownerReference pointing
// to the root declaration after reconcile.
func TestLineageReconciler_DeleteWithRoot_AddsOwnerReference(t *testing.T) {
	s := newTestScheme(t)
	const ns = "test-ns"
	root := newRootDeclaration(talosClusterGVK, "cluster-d", ns)
	root.SetUID("uid-cluster-d")

	ili := newILIWithDescendants(t, "taloscluster-cluster-d", ns, nil)
	ili.Spec.RetentionPolicy = &seamv1alpha1.LineageRetentionPolicy{
		DescendantRetentionDays: 30,
		DeleteWithRoot:          true,
	}

	fakeClient := fake.NewClientBuilder().WithScheme(s).
		WithObjects(root, ili, newCRDObject(talosClusterGVK, true)).
		WithStatusSubresource(root, ili).
		Build()

	r := &controller.LineageReconciler{
		Client: fakeClient,
		Scheme: s,
		GVK:    talosClusterGVK,
	}

	reconcileRoot(t, r, "cluster-d", ns)

	updatedILI := &seamv1alpha1.LineageRecord{}
	if err := fakeClient.Get(context.Background(), client.ObjectKey{Name: "taloscluster-cluster-d", Namespace: ns}, updatedILI); err != nil {
		t.Fatalf("get ILI: %v", err)
	}
	refs := updatedILI.GetOwnerReferences()
	if len(refs) == 0 {
		t.Fatal("expected ownerReference to be set when DeleteWithRoot=true")
	}
	found := false
	for _, ref := range refs {
		if ref.UID == "uid-cluster-d" && ref.Kind == "TalosCluster" {
			found = true
			if ref.BlockOwnerDeletion == nil || !*ref.BlockOwnerDeletion {
				t.Error("expected BlockOwnerDeletion=true on ownerReference")
			}
		}
	}
	if !found {
		t.Errorf("ownerReference to root declaration (uid-cluster-d) not found in ILI ownerReferences: %v", refs)
	}
}

// TestLineageReconciler_DeleteWithRoot_False_NoOwnerReference verifies that when
// RetentionPolicy.DeleteWithRoot=false no ownerReference is added to the ILI.
func TestLineageReconciler_DeleteWithRoot_False_NoOwnerReference(t *testing.T) {
	s := newTestScheme(t)
	const ns = "test-ns"
	root := newRootDeclaration(talosClusterGVK, "cluster-e", ns)
	root.SetUID("uid-cluster-e")

	ili := newILIWithDescendants(t, "taloscluster-cluster-e", ns, nil)
	ili.Spec.RetentionPolicy = &seamv1alpha1.LineageRetentionPolicy{
		DescendantRetentionDays: 30,
		DeleteWithRoot:          false,
	}

	fakeClient := fake.NewClientBuilder().WithScheme(s).
		WithObjects(root, ili, newCRDObject(talosClusterGVK, true)).
		WithStatusSubresource(root, ili).
		Build()

	r := &controller.LineageReconciler{
		Client: fakeClient,
		Scheme: s,
		GVK:    talosClusterGVK,
	}

	reconcileRoot(t, r, "cluster-e", ns)

	updatedILI := &seamv1alpha1.LineageRecord{}
	if err := fakeClient.Get(context.Background(), client.ObjectKey{Name: "taloscluster-cluster-e", Namespace: ns}, updatedILI); err != nil {
		t.Fatalf("get ILI: %v", err)
	}
	// No ownerReference should be set.
	refs := updatedILI.GetOwnerReferences()
	for _, ref := range refs {
		if ref.UID == "uid-cluster-e" {
			t.Errorf("unexpected ownerReference to root declaration when DeleteWithRoot=false")
		}
	}
}

// TestLineageReconciler_ILIHasDomainRef verifies that a newly created
// LineageRecord carries spec.domainRef set to the canonical
// infrastructure domain reference. CLAUDE.md §14 Decision 2.
func TestLineageReconciler_ILIHasDomainRef(t *testing.T) {
	s := newTestScheme(t)
	root := newRootDeclaration(talosClusterGVK, "dc-cluster", "ont-system")

	fakeClient := fake.NewClientBuilder().WithScheme(s).
		WithObjects(root, newCRDObject(talosClusterGVK, true)).
		WithStatusSubresource(root, &seamv1alpha1.LineageRecord{}).
		Build()

	r := &controller.LineageReconciler{
		Client: fakeClient,
		Scheme: s,
		GVK:    talosClusterGVK,
	}

	reconcileRoot(t, r, "dc-cluster", "ont-system")

	ili := &seamv1alpha1.LineageRecord{}
	if err := fakeClient.Get(context.Background(),
		client.ObjectKey{Name: "taloscluster-dc-cluster", Namespace: "ont-system"}, ili); err != nil {
		t.Fatalf("get ILI: %v", err)
	}

	const want = "infrastructure.core.ontai.dev"
	if ili.Spec.DomainRef != want {
		t.Errorf("expected ILI spec.domainRef=%q, got %q", want, ili.Spec.DomainRef)
	}
}

// TestLineageReconciler_ILINameDerivation verifies the deterministic ILI naming
// convention for each GVK: lowercasekind-name.
func TestLineageReconciler_ILINameDerivation(t *testing.T) {
	cases := []struct {
		gvk      schema.GroupVersionKind
		rootName string
		wantName string
	}{
		{talosClusterGVK, "prod", "taloscluster-prod"},
		{clusterPackGVK, "cilium-v1", "packdelivery-cilium-v1"},
		{rbacPolicyGVK, "platform", "rbacpolicy-platform"},
		{packExecutionGVK, "deploy-123", "packexecution-deploy-123"},
	}

	for _, tc := range cases {
		s := newTestScheme(t)
		root := newRootDeclaration(tc.gvk, tc.rootName, "test-ns")

		fakeClient := fake.NewClientBuilder().WithScheme(s).
			WithObjects(root, newCRDObject(tc.gvk, true)).
			WithStatusSubresource(root, &seamv1alpha1.LineageRecord{}).
			Build()

		r := &controller.LineageReconciler{
			Client: fakeClient,
			Scheme: s,
			GVK:    tc.gvk,
		}
		reconcileRoot(t, r, tc.rootName, "test-ns")

		ili := &seamv1alpha1.LineageRecord{}
		if err := fakeClient.Get(context.Background(),
			client.ObjectKey{Name: tc.wantName, Namespace: "test-ns"}, ili); err != nil {
			t.Errorf("GVK %s name %q: expected ILI %q, not found: %v",
				tc.gvk.Kind, tc.rootName, tc.wantName, err)
		}
	}
}

// TestLineageReconciler_PackInstanceCreatesILIAndSetsLineageSynced verifies the
// full reconcile path for seam.ontai.dev/v1alpha1/PackInstalled: a LineageRecord is created
// and LineageSynced is transitioned to True.
//
// PackInstance was identified in SEAM-CORE-BL-LINEAGE as having LineageSynced=False
// in production. This test is the permanent regression guard confirming the reconciler
// exercises the complete path including Status().Patch() with WithStatusSubresource.
func TestLineageReconciler_PackInstanceCreatesILIAndSetsLineageSynced(t *testing.T) {
	s := newTestScheme(t)
	root := newRootDeclaration(packInstanceGVK, "cilium-v1-ccs-mgmt", "ont-system")

	// WithStatusSubresource must include the unstructured root with its GVK set.
	// The fake client matches status subresource eligibility by GVK — without this
	// the Status().Patch() call in ensureLineageSyncedTrue is a no-op and
	// LineageSynced remains False. seam-core session 22 investigation.
	fakeClient := fake.NewClientBuilder().WithScheme(s).
		WithObjects(root, newCRDObject(packInstanceGVK, true)).
		WithStatusSubresource(root, &seamv1alpha1.LineageRecord{}).
		Build()

	r := &controller.LineageReconciler{
		Client: fakeClient,
		Scheme: s,
		GVK:    packInstanceGVK,
	}

	result := reconcileRoot(t, r, "cilium-v1-ccs-mgmt", "ont-system")
	if result.RequeueAfter != 0 {
		t.Errorf("expected no requeue, got %+v", result)
	}

	// Verify LineageRecord was created with correct rootBinding.
	ili := &seamv1alpha1.LineageRecord{}
	iliKey := client.ObjectKey{Name: "packinstalled-cilium-v1-ccs-mgmt", Namespace: "ont-system"}
	if err := fakeClient.Get(context.Background(), iliKey, ili); err != nil {
		t.Fatalf("expected LineageRecord to exist: %v", err)
	}
	if ili.Spec.RootBinding.RootKind != "PackInstalled" {
		t.Errorf("expected RootKind=PackInstalled, got %q", ili.Spec.RootBinding.RootKind)
	}
	if ili.Spec.RootBinding.RootName != "cilium-v1-ccs-mgmt" {
		t.Errorf("expected RootName=cilium-v1-ccs-mgmt, got %q", ili.Spec.RootBinding.RootName)
	}

	// Verify LineageSynced was transitioned to True via Status().Patch().
	updatedRoot := &unstructured.Unstructured{}
	updatedRoot.SetGroupVersionKind(packInstanceGVK)
	if err := fakeClient.Get(context.Background(),
		client.ObjectKey{Name: "cilium-v1-ccs-mgmt", Namespace: "ont-system"}, updatedRoot); err != nil {
		t.Fatalf("get updated PackInstance: %v", err)
	}

	conditions, found, _ := unstructured.NestedSlice(updatedRoot.Object, "status", "conditions")
	if !found || len(conditions) == 0 {
		t.Fatal("expected status.conditions to be set on PackInstance after reconcile")
	}

	var lineageSyncedFound bool
	for _, rawCond := range conditions {
		cond, ok := rawCond.(map[string]interface{})
		if !ok {
			continue
		}
		if cond["type"] == "LineageSynced" {
			lineageSyncedFound = true
			if cond["status"] != string(metav1.ConditionTrue) {
				t.Errorf("LineageSynced on PackInstance: got status=%v; want True", cond["status"])
			}
			if cond["reason"] != controller.ReasonLineageIndexCreated {
				t.Errorf("LineageSynced on PackInstance: got reason=%v; want %q",
					cond["reason"], controller.ReasonLineageIndexCreated)
			}
		}
	}
	if !lineageSyncedFound {
		t.Error("LineageSynced condition not found on PackInstance after reconcile")
	}
}
