package unit_test

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	ctrl "sigs.k8s.io/controller-runtime"

	seamv1alpha1 "github.com/ontai-dev/seam-core/api/v1alpha1"
	"github.com/ontai-dev/seam-core/internal/controller"
	"github.com/ontai-dev/seam-core/pkg/lineage"
)

var runnerConfigGVK = schema.GroupVersionKind{
	Group:   "runner.ontai.dev",
	Version: "v1alpha1",
	Kind:    "RunnerConfig",
}

// newRunnerConfig builds a minimal unstructured RunnerConfig with descendant labels.
func newRunnerConfig(name, namespace, iliName string) *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(runnerConfigGVK)
	u.SetName(name)
	u.SetNamespace(namespace)
	u.SetUID(types.UID("uid-" + name))
	u.SetGeneration(1)
	labels := map[string]string{
		controller.LabelRootILI:           iliName,
		controller.LabelSeamOperator:      "platform",
		controller.LabelCreationRationale: string(lineage.ClusterProvision),
	}
	u.SetLabels(labels)
	return u
}

// newRunnerConfigCrossNS builds a RunnerConfig with all four descendant labels,
// including root-ili-namespace pointing to a different namespace than the RC itself.
func newRunnerConfigCrossNS(name, rcNamespace, iliName, iliNamespace string) *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(runnerConfigGVK)
	u.SetName(name)
	u.SetNamespace(rcNamespace)
	u.SetUID(types.UID("uid-" + name))
	u.SetGeneration(1)
	labels := map[string]string{
		controller.LabelRootILI:           iliName,
		controller.LabelRootILINamespace:  iliNamespace,
		controller.LabelSeamOperator:      "platform",
		controller.LabelCreationRationale: string(lineage.ConductorAssignment),
	}
	u.SetLabels(labels)
	return u
}

// newILIForDescendant builds a pre-existing ILI with nil DescendantRegistry.
func newILIForDescendant(t *testing.T, name, namespace string) *seamv1alpha1.InfrastructureLineageIndex {
	t.Helper()
	return &seamv1alpha1.InfrastructureLineageIndex{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: seamv1alpha1.InfrastructureLineageIndexSpec{
			RootBinding: seamv1alpha1.InfrastructureLineageIndexRootBinding{
				RootKind:      "TalosCluster",
				RootName:      "prod-cluster",
				RootNamespace: namespace,
			},
		},
	}
}

func buildDescendantReconciler(t *testing.T, objects ...client.Object) (*controller.DescendantReconciler, client.Client) {
	t.Helper()
	s := newTestScheme(t)
	c := fake.NewClientBuilder().
		WithScheme(s).
		WithObjects(objects...).
		Build()
	return &controller.DescendantReconciler{
		Client: c,
		Scheme: s,
		GVK:    runnerConfigGVK,
	}, c
}

// TestDescendantReconciler_AppendsEntryToILI verifies that when a RunnerConfig
// carries infrastructure.ontai.dev/root-ili, the DescendantReconciler appends
// a DescendantEntry to the referenced ILI's DescendantRegistry.
func TestDescendantReconciler_AppendsEntryToILI(t *testing.T) {
	const ns = "seam-system"
	const iliName = "taloscluster-prod-cluster"

	ili := newILIForDescendant(t, iliName, ns)
	rc := newRunnerConfig("rc-etcd-001", ns, iliName)

	r, c := buildDescendantReconciler(t, ili, rc)

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "rc-etcd-001", Namespace: ns},
	})
	if err != nil {
		t.Fatalf("Reconcile error: %v", err)
	}
	if result.RequeueAfter != 0 {
		t.Errorf("expected no requeue")
	}

	got := &seamv1alpha1.InfrastructureLineageIndex{}
	if err := c.Get(context.Background(), client.ObjectKey{Name: iliName, Namespace: ns}, got); err != nil {
		t.Fatalf("get ILI: %v", err)
	}
	if len(got.Spec.DescendantRegistry) != 1 {
		t.Fatalf("DescendantRegistry: want 1 entry, got %d", len(got.Spec.DescendantRegistry))
	}

	entry := got.Spec.DescendantRegistry[0]
	if entry.Kind != "RunnerConfig" {
		t.Errorf("entry.Kind = %q, want RunnerConfig", entry.Kind)
	}
	if entry.Name != "rc-etcd-001" {
		t.Errorf("entry.Name = %q, want rc-etcd-001", entry.Name)
	}
	if entry.UID != types.UID("uid-rc-etcd-001") {
		t.Errorf("entry.UID = %q, want uid-rc-etcd-001", entry.UID)
	}
	if entry.SeamOperator != "platform" {
		t.Errorf("entry.SeamOperator = %q, want platform", entry.SeamOperator)
	}
	if entry.CreationRationale != lineage.ClusterProvision {
		t.Errorf("entry.CreationRationale = %q, want %q", entry.CreationRationale, lineage.ClusterProvision)
	}
	if entry.CreatedAt == nil {
		t.Errorf("entry.CreatedAt must be set")
	}
}

// TestDescendantReconciler_Idempotent verifies that a second reconcile of the same
// RunnerConfig does not create a duplicate DescendantEntry.
func TestDescendantReconciler_Idempotent(t *testing.T) {
	const ns = "seam-system"
	const iliName = "taloscluster-prod-cluster"

	ili := newILIForDescendant(t, iliName, ns)
	rc := newRunnerConfig("rc-pki-001", ns, iliName)

	r, c := buildDescendantReconciler(t, ili, rc)

	req := ctrl.Request{NamespacedName: types.NamespacedName{Name: "rc-pki-001", Namespace: ns}}

	if _, err := r.Reconcile(context.Background(), req); err != nil {
		t.Fatalf("first Reconcile error: %v", err)
	}
	if _, err := r.Reconcile(context.Background(), req); err != nil {
		t.Fatalf("second Reconcile error: %v", err)
	}

	got := &seamv1alpha1.InfrastructureLineageIndex{}
	if err := c.Get(context.Background(), client.ObjectKey{Name: iliName, Namespace: ns}, got); err != nil {
		t.Fatalf("get ILI: %v", err)
	}
	if len(got.Spec.DescendantRegistry) != 1 {
		t.Errorf("DescendantRegistry: want exactly 1 entry after two reconciles, got %d",
			len(got.Spec.DescendantRegistry))
	}
}

// TestDescendantReconciler_CrossNamespaceILI verifies that a derived object in
// ont-system carrying root-ili-namespace=seam-system correctly looks up the ILI
// in seam-system, not in ont-system. Resolves PLATFORM-BL-ILI-CROSS-NS.
func TestDescendantReconciler_CrossNamespaceILI(t *testing.T) {
	const rcNamespace = "ont-system"
	const iliNamespace = "seam-system"
	const iliName = "taloscluster-ccs-dev"

	// ILI lives in seam-system; RunnerConfig lives in ont-system.
	ili := newILIForDescendant(t, iliName, iliNamespace)
	rc := newRunnerConfigCrossNS("rc-bootstrap-ccs-dev", rcNamespace, iliName, iliNamespace)

	r, c := buildDescendantReconciler(t, ili, rc)

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "rc-bootstrap-ccs-dev", Namespace: rcNamespace},
	})
	if err != nil {
		t.Fatalf("Reconcile error: %v", err)
	}
	if result.RequeueAfter != 0 {
		t.Errorf("expected no requeue: ILI should be found via cross-namespace label, RequeueAfter=%s", result.RequeueAfter)
	}

	got := &seamv1alpha1.InfrastructureLineageIndex{}
	if err := c.Get(context.Background(),
		client.ObjectKey{Name: iliName, Namespace: iliNamespace}, got); err != nil {
		t.Fatalf("get ILI in seam-system: %v", err)
	}
	if len(got.Spec.DescendantRegistry) != 1 {
		t.Fatalf("DescendantRegistry: want 1 entry, got %d", len(got.Spec.DescendantRegistry))
	}
	entry := got.Spec.DescendantRegistry[0]
	if entry.Name != "rc-bootstrap-ccs-dev" {
		t.Errorf("entry.Name = %q, want rc-bootstrap-ccs-dev", entry.Name)
	}
	if entry.Namespace != rcNamespace {
		t.Errorf("entry.Namespace = %q, want %s", entry.Namespace, rcNamespace)
	}
	if entry.CreationRationale != lineage.ConductorAssignment {
		t.Errorf("entry.CreationRationale = %q, want ConductorAssignment", entry.CreationRationale)
	}
}

// TestDescendantReconciler_NoOpWhenLabelAbsent verifies that a RunnerConfig without
// the infrastructure.ontai.dev/root-ili label leaves the ILI unchanged.
func TestDescendantReconciler_NoOpWhenLabelAbsent(t *testing.T) {
	const ns = "seam-system"
	const iliName = "taloscluster-prod-cluster"

	ili := newILIForDescendant(t, iliName, ns)

	// RunnerConfig without any labels.
	rc := &unstructured.Unstructured{}
	rc.SetGroupVersionKind(runnerConfigGVK)
	rc.SetName("rc-unlabeled")
	rc.SetNamespace(ns)
	rc.SetUID("uid-rc-unlabeled")

	r, c := buildDescendantReconciler(t, ili, rc)

	if _, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "rc-unlabeled", Namespace: ns},
	}); err != nil {
		t.Fatalf("Reconcile error: %v", err)
	}

	got := &seamv1alpha1.InfrastructureLineageIndex{}
	if err := c.Get(context.Background(), client.ObjectKey{Name: iliName, Namespace: ns}, got); err != nil {
		t.Fatalf("get ILI: %v", err)
	}
	if len(got.Spec.DescendantRegistry) != 0 {
		t.Errorf("DescendantRegistry: want 0 entries, got %d", len(got.Spec.DescendantRegistry))
	}
}
