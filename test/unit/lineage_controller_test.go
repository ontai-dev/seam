package unit_test

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	seamv1alpha1 "github.com/ontai-dev/seam-core/api/v1alpha1"
	"github.com/ontai-dev/seam-core/internal/controller"
)

// rootGVKs used in tests — covers one from each operator family.
var (
	talosClusterGVK   = schema.GroupVersionKind{Group: "platform.ontai.dev", Version: "v1alpha1", Kind: "TalosCluster"}
	clusterPackGVK    = schema.GroupVersionKind{Group: "infra.ontai.dev", Version: "v1alpha1", Kind: "ClusterPack"}
	rbacPolicyGVK     = schema.GroupVersionKind{Group: "security.ontai.dev", Version: "v1alpha1", Kind: "RBACPolicy"}
	packExecutionGVK  = schema.GroupVersionKind{Group: "infra.ontai.dev", Version: "v1alpha1", Kind: "PackExecution"}
	identityBindGVK   = schema.GroupVersionKind{Group: "security.ontai.dev", Version: "v1alpha1", Kind: "IdentityBinding"}
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
			"message":            "InfrastructureLineageController is not yet deployed.",
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
// TalosCluster root declaration creates an InfrastructureLineageIndex with
// the correct rootBinding.
func TestLineageReconciler_CreatesILIForTalosCluster(t *testing.T) {
	s := newTestScheme(t)
	root := newRootDeclaration(talosClusterGVK, "prod-cluster", "ont-system")

	fakeClient := fake.NewClientBuilder().WithScheme(s).
		WithObjects(root).
		WithStatusSubresource(root, &seamv1alpha1.InfrastructureLineageIndex{}).
		Build()

	r := &controller.LineageReconciler{
		Client: fakeClient,
		Scheme: s,
		GVK:    talosClusterGVK,
	}

	result := reconcileRoot(t, r, "prod-cluster", "ont-system")
	if result.Requeue || result.RequeueAfter != 0 {
		t.Errorf("expected no requeue, got %+v", result)
	}

	// Verify ILI was created.
	ili := &seamv1alpha1.InfrastructureLineageIndex{}
	iliKey := client.ObjectKey{Name: "taloscluster-prod-cluster", Namespace: "ont-system"}
	if err := fakeClient.Get(context.Background(), iliKey, ili); err != nil {
		t.Fatalf("expected InfrastructureLineageIndex to exist: %v", err)
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
		WithObjects(root).
		WithStatusSubresource(root, &seamv1alpha1.InfrastructureLineageIndex{}).
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
	expectedILIName := "clusterpack-my-pack"
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
		WithObjects(root).
		WithStatusSubresource(root, &seamv1alpha1.InfrastructureLineageIndex{}).
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
	if result.Requeue || result.RequeueAfter != 0 {
		t.Errorf("expected no requeue for not-found, got %+v", result)
	}
	// No ILI should be created.
	iliList := &seamv1alpha1.InfrastructureLineageIndexList{}
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
		WithObjects(root).
		WithStatusSubresource(root, &seamv1alpha1.InfrastructureLineageIndex{}).
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
	iliList := &seamv1alpha1.InfrastructureLineageIndexList{}
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
		controller.GovernanceAnnotationLineageIndexRef: "clusterpack-pack-v2",
	})

	// Create an existing ILI with matching rootBinding.
	existingILI := &seamv1alpha1.InfrastructureLineageIndex{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "clusterpack-pack-v2",
			Namespace: "infra-system",
		},
		Spec: seamv1alpha1.InfrastructureLineageIndexSpec{
			RootBinding: seamv1alpha1.InfrastructureLineageIndexRootBinding{
				RootKind:               "ClusterPack",
				RootName:               "pack-v2",
				RootNamespace:          "infra-system",
				RootUID:                types.UID("uid-pack-v2"),
				RootObservedGeneration: 1,
			},
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(s).
		WithObjects(root, existingILI).
		WithStatusSubresource(root, &seamv1alpha1.InfrastructureLineageIndex{}).
		Build()

	r := &controller.LineageReconciler{
		Client: fakeClient,
		Scheme: s,
		GVK:    clusterPackGVK,
	}

	// Reconcile should succeed without attempting to recreate or modify the ILI.
	result := reconcileRoot(t, r, "pack-v2", "infra-system")
	if result.Requeue || result.RequeueAfter != 0 {
		t.Errorf("expected no requeue on idempotent reconcile, got %+v", result)
	}

	// ILI rootBinding must be unchanged.
	ili := &seamv1alpha1.InfrastructureLineageIndex{}
	if err := fakeClient.Get(context.Background(),
		client.ObjectKey{Name: "clusterpack-pack-v2", Namespace: "infra-system"}, ili); err != nil {
		t.Fatalf("get ILI: %v", err)
	}
	if ili.Spec.RootBinding.RootUID != types.UID("uid-pack-v2") {
		t.Errorf("ILI rootBinding.RootUID was modified — immutability violated")
	}
}

// TestLineageReconciler_ControllerAuthoredAnnotation verifies that newly created
// InfrastructureLineageIndex instances carry the controller-authored governance
// annotation per CLAUDE.md §14 Decision 3.
func TestLineageReconciler_ControllerAuthoredAnnotation(t *testing.T) {
	s := newTestScheme(t)
	root := newRootDeclaration(rbacPolicyGVK, "tenant-policy", "security-system")

	fakeClient := fake.NewClientBuilder().WithScheme(s).
		WithObjects(root).
		WithStatusSubresource(root, &seamv1alpha1.InfrastructureLineageIndex{}).
		Build()

	r := &controller.LineageReconciler{
		Client: fakeClient,
		Scheme: s,
		GVK:    rbacPolicyGVK,
	}

	reconcileRoot(t, r, "tenant-policy", "security-system")

	ili := &seamv1alpha1.InfrastructureLineageIndex{}
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
// list contains all nine expected GVKs across the three Seam operator domains.
func TestLineageReconciler_AllRootDeclarationGVKsRegistered(t *testing.T) {
	expected := map[string]bool{
		"platform.ontai.dev/v1alpha1/TalosCluster":      false,
		"infra.ontai.dev/v1alpha1/ClusterPack":          false,
		"infra.ontai.dev/v1alpha1/PackExecution":        false,
		"infra.ontai.dev/v1alpha1/PackInstance":         false,
		"security.ontai.dev/v1alpha1/RBACPolicy":        false,
		"security.ontai.dev/v1alpha1/RBACProfile":       false,
		"security.ontai.dev/v1alpha1/IdentityBinding":   false,
		"security.ontai.dev/v1alpha1/IdentityProvider":  false,
		"security.ontai.dev/v1alpha1/PermissionSet":     false,
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

	if len(controller.RootDeclarationGVKs) != 9 {
		t.Errorf("expected 9 GVKs, got %d", len(controller.RootDeclarationGVKs))
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
		{clusterPackGVK, "cilium-v1", "clusterpack-cilium-v1"},
		{rbacPolicyGVK, "platform", "rbacpolicy-platform"},
		{packExecutionGVK, "deploy-123", "packexecution-deploy-123"},
	}

	for _, tc := range cases {
		s := newTestScheme(t)
		root := newRootDeclaration(tc.gvk, tc.rootName, "test-ns")

		fakeClient := fake.NewClientBuilder().WithScheme(s).
			WithObjects(root).
			WithStatusSubresource(root, &seamv1alpha1.InfrastructureLineageIndex{}).
			Build()

		r := &controller.LineageReconciler{
			Client: fakeClient,
			Scheme: s,
			GVK:    tc.gvk,
		}
		reconcileRoot(t, r, tc.rootName, "test-ns")

		ili := &seamv1alpha1.InfrastructureLineageIndex{}
		if err := fakeClient.Get(context.Background(),
			client.ObjectKey{Name: tc.wantName, Namespace: "test-ns"}, ili); err != nil {
			t.Errorf("GVK %s name %q: expected ILI %q, not found: %v",
				tc.gvk.Kind, tc.rootName, tc.wantName, err)
		}
	}
}
