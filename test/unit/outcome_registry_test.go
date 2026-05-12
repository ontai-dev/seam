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

func newPackInstanceWithCondition(name, namespace, iliName, condType, condStatus, condReason string) *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(packInstanceGVK)
	u.SetName(name)
	u.SetNamespace(namespace)
	u.SetUID(types.UID("uid-pi-" + name))
	u.SetGeneration(1)
	u.SetLabels(map[string]string{
		controller.LabelRootILI:           iliName,
		controller.LabelSeamOperator:      "wrapper",
		controller.LabelCreationRationale: string(lineage.PackExecution),
	})
	conditions := []interface{}{
		map[string]interface{}{
			"type":               condType,
			"status":             condStatus,
			"reason":             condReason,
			"message":            "test message for " + condReason,
			"lastTransitionTime": "2026-04-21T00:00:00Z",
		},
	}
	_ = unstructured.SetNestedSlice(u.Object, conditions, "status", "conditions")
	return u
}

func newILIForOutcome(name, namespace string) *seamv1alpha1.LineageRecord {
	return &seamv1alpha1.LineageRecord{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: seamv1alpha1.LineageRecordSpec{
			RootBinding: seamv1alpha1.LineageRecordRootBinding{
				RootKind:      "PackExecution",
				RootName:      "exec-001",
				RootNamespace: namespace,
			},
		},
	}
}

// TestOutcomeRegistry_SucceededConditionAppendsEntry verifies that a Ready=True
// condition on a derived object causes an OutcomeEntry with Succeeded to be appended.
func TestOutcomeRegistry_SucceededConditionAppendsEntry(t *testing.T) {
	c := fake.NewClientBuilder().WithScheme(newTestScheme(t)).Build()
	ili := newILIForOutcome("packexecution-exec-001", "seam-system")
	if err := c.Create(context.Background(), ili); err != nil {
		t.Fatalf("create ILI: %v", err)
	}

	pi := newPackInstanceWithCondition("pi-001", "seam-system", "packexecution-exec-001",
		"Ready", string(metav1.ConditionTrue), "Succeeded")
	if err := c.Create(context.Background(), pi); err != nil {
		t.Fatalf("create PackInstance: %v", err)
	}

	r := &controller.OutcomeReconciler{
		Client: c,
		Scheme: newTestScheme(t),
		GVK:    packInstanceGVK,
	}
	if _, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "pi-001", Namespace: "seam-system"},
	}); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	updated := &seamv1alpha1.LineageRecord{}
	if err := c.Get(context.Background(), client.ObjectKey{
		Name: "packexecution-exec-001", Namespace: "seam-system",
	}, updated); err != nil {
		t.Fatalf("get ILI: %v", err)
	}

	if len(updated.Spec.OutcomeRegistry) != 1 {
		t.Fatalf("OutcomeRegistry len = %d, want 1", len(updated.Spec.OutcomeRegistry))
	}
	entry := updated.Spec.OutcomeRegistry[0]
	if entry.OutcomeType != seamv1alpha1.OutcomeTypeSucceeded {
		t.Errorf("OutcomeType = %q, want %q", entry.OutcomeType, seamv1alpha1.OutcomeTypeSucceeded)
	}
	if entry.DerivedObjectUID != pi.GetUID() {
		t.Errorf("DerivedObjectUID = %q, want %q", entry.DerivedObjectUID, pi.GetUID())
	}
}

// TestOutcomeRegistry_FailedConditionAppendsFailedEntry verifies that a Ready=False
// condition without a drift/superseded reason appends a Failed OutcomeEntry.
func TestOutcomeRegistry_FailedConditionAppendsFailedEntry(t *testing.T) {
	c := fake.NewClientBuilder().WithScheme(newTestScheme(t)).Build()
	ili := newILIForOutcome("packexecution-exec-002", "seam-system")
	if err := c.Create(context.Background(), ili); err != nil {
		t.Fatalf("create ILI: %v", err)
	}

	pi := newPackInstanceWithCondition("pi-002", "seam-system", "packexecution-exec-002",
		"Ready", string(metav1.ConditionFalse), "ImagePullFailed")
	if err := c.Create(context.Background(), pi); err != nil {
		t.Fatalf("create PackInstance: %v", err)
	}

	r := &controller.OutcomeReconciler{
		Client: c,
		Scheme: newTestScheme(t),
		GVK:    packInstanceGVK,
	}
	if _, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "pi-002", Namespace: "seam-system"},
	}); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	updated := &seamv1alpha1.LineageRecord{}
	if err := c.Get(context.Background(), client.ObjectKey{
		Name: "packexecution-exec-002", Namespace: "seam-system",
	}, updated); err != nil {
		t.Fatalf("get ILI: %v", err)
	}

	if len(updated.Spec.OutcomeRegistry) != 1 {
		t.Fatalf("OutcomeRegistry len = %d, want 1", len(updated.Spec.OutcomeRegistry))
	}
	if got := updated.Spec.OutcomeRegistry[0].OutcomeType; got != seamv1alpha1.OutcomeTypeFailed {
		t.Errorf("OutcomeType = %q, want %q", got, seamv1alpha1.OutcomeTypeFailed)
	}
}

// TestOutcomeRegistry_IdempotentOnSecondReconcile verifies that a second reconcile
// after an OutcomeEntry is already present for the same UID does not append a second entry.
func TestOutcomeRegistry_IdempotentOnSecondReconcile(t *testing.T) {
	c := fake.NewClientBuilder().WithScheme(newTestScheme(t)).Build()
	pi := newPackInstanceWithCondition("pi-003", "seam-system", "packexecution-exec-003",
		"Ready", string(metav1.ConditionTrue), "Succeeded")

	ili := newILIForOutcome("packexecution-exec-003", "seam-system")
	// Pre-populate the outcomeRegistry with an entry for this UID.
	ili.Spec.OutcomeRegistry = []seamv1alpha1.OutcomeEntry{
		{
			DerivedObjectUID: pi.GetUID(),
			OutcomeType:      seamv1alpha1.OutcomeTypeSucceeded,
			OutcomeTimestamp: metav1.Now(),
		},
	}
	if err := c.Create(context.Background(), ili); err != nil {
		t.Fatalf("create ILI: %v", err)
	}
	if err := c.Create(context.Background(), pi); err != nil {
		t.Fatalf("create PackInstance: %v", err)
	}

	r := &controller.OutcomeReconciler{
		Client: c,
		Scheme: newTestScheme(t),
		GVK:    packInstanceGVK,
	}
	if _, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "pi-003", Namespace: "seam-system"},
	}); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	updated := &seamv1alpha1.LineageRecord{}
	if err := c.Get(context.Background(), client.ObjectKey{
		Name: "packexecution-exec-003", Namespace: "seam-system",
	}, updated); err != nil {
		t.Fatalf("get ILI: %v", err)
	}

	if len(updated.Spec.OutcomeRegistry) != 1 {
		t.Errorf("OutcomeRegistry len = %d, want 1 (idempotent)", len(updated.Spec.OutcomeRegistry))
	}
}

// TestOutcomeRegistry_DoesNotModifyDescendantRegistry verifies that appending an
// OutcomeEntry does not modify any DescendantRegistry entries.
func TestOutcomeRegistry_DoesNotModifyDescendantRegistry(t *testing.T) {
	c := fake.NewClientBuilder().WithScheme(newTestScheme(t)).Build()
	now := metav1.Now()
	existingEntry := seamv1alpha1.DescendantEntry{
		Kind:              "PackInstance",
		Name:              "pi-004",
		Namespace:         "seam-system",
		UID:               "uid-pi-pi-004",
		SeamOperator:      "wrapper",
		CreationRationale: lineage.PackExecution,
		CreatedAt:         &now,
	}
	ili := newILIForOutcome("packexecution-exec-004", "seam-system")
	ili.Spec.DescendantRegistry = []seamv1alpha1.DescendantEntry{existingEntry}
	if err := c.Create(context.Background(), ili); err != nil {
		t.Fatalf("create ILI: %v", err)
	}

	pi := newPackInstanceWithCondition("pi-004", "seam-system", "packexecution-exec-004",
		"Ready", string(metav1.ConditionTrue), "Succeeded")
	pi.SetUID(existingEntry.UID)
	if err := c.Create(context.Background(), pi); err != nil {
		t.Fatalf("create PackInstance: %v", err)
	}

	r := &controller.OutcomeReconciler{
		Client: c,
		Scheme: newTestScheme(t),
		GVK:    packInstanceGVK,
	}
	if _, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "pi-004", Namespace: "seam-system"},
	}); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	updated := &seamv1alpha1.LineageRecord{}
	if err := c.Get(context.Background(), client.ObjectKey{
		Name: "packexecution-exec-004", Namespace: "seam-system",
	}, updated); err != nil {
		t.Fatalf("get ILI: %v", err)
	}

	if len(updated.Spec.OutcomeRegistry) != 1 {
		t.Fatalf("OutcomeRegistry len = %d, want 1", len(updated.Spec.OutcomeRegistry))
	}
	if len(updated.Spec.DescendantRegistry) != 1 {
		t.Fatalf("DescendantRegistry len = %d, want 1 (unmodified)", len(updated.Spec.DescendantRegistry))
	}
	// Verify descendant entry is unchanged.
	if updated.Spec.DescendantRegistry[0].UID != existingEntry.UID {
		t.Errorf("DescendantRegistry[0].UID modified: got %q, want %q",
			updated.Spec.DescendantRegistry[0].UID, existingEntry.UID)
	}
}
