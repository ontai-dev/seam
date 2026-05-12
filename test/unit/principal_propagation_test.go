package unit_test

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	seamv1alpha1 "github.com/ontai-dev/seam/api/v1alpha1"
	"github.com/ontai-dev/seam/internal/controller"
	"github.com/ontai-dev/seam/pkg/lineage"
)

func newTalosCluster(name, namespace string, annotations map[string]string) *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(talosClusterGVK)
	u.SetName(name)
	u.SetNamespace(namespace)
	u.SetUID(types.UID("uid-tc-" + name))
	u.SetGeneration(1)
	if annotations != nil {
		u.SetAnnotations(annotations)
	}
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

func newLineageReconcilerForPrincipal(t *testing.T, c client.Client) *controller.LineageReconciler {
	t.Helper()
	return &controller.LineageReconciler{
		Client: c,
		Scheme: newTestScheme(t),
		GVK:    talosClusterGVK,
	}
}

// TestPrincipalPropagation_ILICreatedWithDeclaringPrincipal verifies that when the
// root declaration carries the declaring-principal annotation, the created ILI
// has rootBinding.declaringPrincipal set to that value.
func TestPrincipalPropagation_ILICreatedWithDeclaringPrincipal(t *testing.T) {
	tc := newTalosCluster("prod", "seam-system", map[string]string{
		lineage.AnnotationDeclaringPrincipal: "alice@example.com",
	})
	c := fake.NewClientBuilder().WithScheme(newTestScheme(t)).
		WithObjects(newCRDObject(talosClusterGVK, true)).
		WithStatusSubresource(tc, &seamv1alpha1.LineageRecord{}).Build()
	r := newLineageReconcilerForPrincipal(t, c)
	if err := c.Create(context.Background(), tc); err != nil {
		t.Fatalf("create TalosCluster: %v", err)
	}

	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "prod", Namespace: "seam-system"},
	})
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	ili := &seamv1alpha1.LineageRecord{}
	if err := c.Get(context.Background(), client.ObjectKey{
		Name:      "taloscluster-prod",
		Namespace: "seam-system",
	}, ili); err != nil {
		t.Fatalf("get ILI: %v", err)
	}

	if got := ili.Spec.RootBinding.DeclaringPrincipal; got != "alice@example.com" {
		t.Errorf("declaringPrincipal = %q, want %q", got, "alice@example.com")
	}
}

// TestPrincipalPropagation_AnnotationAbsentSetsSystemUnknown verifies that when the
// root declaration lacks the declaring-principal annotation, the ILI sets
// declaringPrincipal to "system:unknown".
func TestPrincipalPropagation_AnnotationAbsentSetsSystemUnknown(t *testing.T) {
	tc := newTalosCluster("dev", "seam-system", nil)
	c := fake.NewClientBuilder().WithScheme(newTestScheme(t)).
		WithObjects(newCRDObject(talosClusterGVK, true)).
		WithStatusSubresource(tc, &seamv1alpha1.LineageRecord{}).Build()
	r := newLineageReconcilerForPrincipal(t, c)
	if err := c.Create(context.Background(), tc); err != nil {
		t.Fatalf("create TalosCluster: %v", err)
	}

	if _, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "dev", Namespace: "seam-system"},
	}); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	ili := &seamv1alpha1.LineageRecord{}
	if err := c.Get(context.Background(), client.ObjectKey{
		Name: "taloscluster-dev", Namespace: "seam-system",
	}, ili); err != nil {
		t.Fatalf("get ILI: %v", err)
	}

	if got := ili.Spec.RootBinding.DeclaringPrincipal; got != "system:unknown" {
		t.Errorf("declaringPrincipal = %q, want %q", got, "system:unknown")
	}
}

// TestPrincipalPropagation_DescendantActorRefMatchesILI verifies that after the ILI
// is created with a declaringPrincipal, the DescendantReconciler populates ActorRef
// on the descendant entry from the ILI's rootBinding.declaringPrincipal.
func TestPrincipalPropagation_DescendantActorRefMatchesILI(t *testing.T) {
	c := fake.NewClientBuilder().WithScheme(newTestScheme(t)).Build()

	// Pre-create an ILI with a known declaringPrincipal.
	ili := &seamv1alpha1.LineageRecord{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "taloscluster-prod",
			Namespace: "seam-system",
		},
		Spec: seamv1alpha1.LineageRecordSpec{
			RootBinding: seamv1alpha1.LineageRecordRootBinding{
				RootKind:           "TalosCluster",
				RootName:           "prod",
				RootNamespace:      "seam-system",
				RootUID:            "uid-tc-prod",
				DeclaringPrincipal: "bob@company.org",
			},
		},
	}
	if err := c.Create(context.Background(), ili); err != nil {
		t.Fatalf("create ILI: %v", err)
	}

	// Create a RunnerConfig with the root-ili label pointing to this ILI.
	rc := newRunnerConfigWithActorRef("rc-001", "ont-system", "taloscluster-prod", "seam-system", "alice@example.com")
	if err := c.Create(context.Background(), rc); err != nil {
		t.Fatalf("create RunnerConfig: %v", err)
	}

	dr := &controller.DescendantReconciler{
		Client: c,
		Scheme: newTestScheme(t),
		GVK:    runnerConfigGVK,
	}
	if _, err := dr.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "rc-001", Namespace: "ont-system"},
	}); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	updated := &seamv1alpha1.LineageRecord{}
	if err := c.Get(context.Background(), client.ObjectKey{
		Name: "taloscluster-prod", Namespace: "seam-system",
	}, updated); err != nil {
		t.Fatalf("get ILI: %v", err)
	}

	if len(updated.Spec.DescendantRegistry) == 0 {
		t.Fatal("expected descendant entry")
	}
	// ActorRef must come from ILI.declaringPrincipal, not the label.
	if got := updated.Spec.DescendantRegistry[0].ActorRef; got != "bob@company.org" {
		t.Errorf("ActorRef = %q, want %q (from ILI declaringPrincipal)", got, "bob@company.org")
	}
}

// TestPrincipalPropagation_DescendantCreatedAtIsSet verifies that the createdAt
// timestamp is populated on the descendant entry when appended.
func TestPrincipalPropagation_DescendantCreatedAtIsSet(t *testing.T) {
	c := fake.NewClientBuilder().WithScheme(newTestScheme(t)).Build()

	ili := &seamv1alpha1.LineageRecord{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "taloscluster-prod",
			Namespace: "seam-system",
		},
		Spec: seamv1alpha1.LineageRecordSpec{
			RootBinding: seamv1alpha1.LineageRecordRootBinding{
				RootKind:      "TalosCluster",
				RootName:      "prod",
				RootNamespace: "seam-system",
			},
		},
	}
	if err := c.Create(context.Background(), ili); err != nil {
		t.Fatalf("create ILI: %v", err)
	}

	rc := newRunnerConfig("rc-002", "ont-system", "taloscluster-prod")
	rc.SetLabels(addLabel(rc.GetLabels(), controller.LabelRootILINamespace, "seam-system"))
	if err := c.Create(context.Background(), rc); err != nil {
		t.Fatalf("create RunnerConfig: %v", err)
	}

	dr := &controller.DescendantReconciler{
		Client: c,
		Scheme: newTestScheme(t),
		GVK:    runnerConfigGVK,
	}
	if _, err := dr.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "rc-002", Namespace: "ont-system"},
	}); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	updated := &seamv1alpha1.LineageRecord{}
	if err := c.Get(context.Background(), client.ObjectKey{
		Name: "taloscluster-prod", Namespace: "seam-system",
	}, updated); err != nil {
		t.Fatalf("get ILI: %v", err)
	}
	if len(updated.Spec.DescendantRegistry) == 0 {
		t.Fatal("expected descendant entry")
	}
	if updated.Spec.DescendantRegistry[0].CreatedAt == nil {
		t.Error("CreatedAt must be set on descendant entry")
	}
}

// newRunnerConfigWithActorRef builds a RunnerConfig with all descendant labels
// including the actor-ref label.
func newRunnerConfigWithActorRef(name, namespace, iliName, iliNamespace, actorRef string) *unstructured.Unstructured {
	u := newRunnerConfig(name, namespace, iliName)
	labels := u.GetLabels()
	labels[controller.LabelRootILINamespace] = iliNamespace
	labels[controller.LabelActorRef] = actorRef
	u.SetLabels(labels)
	return u
}

func addLabel(labels map[string]string, key, value string) map[string]string {
	if labels == nil {
		labels = make(map[string]string)
	}
	labels[key] = value
	return labels
}
