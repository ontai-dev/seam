// Package lineage_test contains integration tests for the seam-core
// InfrastructureLineageController, verifying that all 9 registered root declaration
// GVKs produce a correctly structured InfrastructureLineageIndex with the right
// rootBinding, governance annotation, and LineageSynced=True condition.
//
// Tests use controller-runtime's fake client — no live cluster or envtest required.
// The table-driven approach runs the full reconcile path for every GVK registered
// in controller.RootDeclarationGVKs, providing a regression guard against accidental
// GVK removal or ILI naming drift.
//
// seam-core-schema.md §7. CLAUDE.md Decisions 1-6. Root invariant: 9 GVKs.
package lineage_test

import (
	"context"
	"fmt"
	"strings"
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

	seamv1alpha1 "github.com/ontai-dev/seam/api/v1alpha1"
	"github.com/ontai-dev/seam/internal/controller"
)

// ── helpers ──────────────────────────────────────────────────────────────────

func newGVKScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	s := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(s); err != nil {
		t.Fatalf("clientgo scheme: %v", err)
	}
	if err := seamv1alpha1.AddToScheme(s); err != nil {
		t.Fatalf("seamv1alpha1 scheme: %v", err)
	}
	return s
}

// buildRootDeclaration builds an unstructured root declaration for the given GVK
// with a pre-populated LineageSynced=False condition as operators write on first
// reconcile. This matches the pre-condition for the LineageController.
func buildRootDeclaration(gvk schema.GroupVersionKind, name, namespace string) *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(gvk)
	u.SetName(name)
	u.SetNamespace(namespace)
	u.SetUID(types.UID("uid-" + name))
	u.SetGeneration(1)
	_ = unstructured.SetNestedSlice(u.Object, []interface{}{
		map[string]interface{}{
			"type":               seamv1alpha1.ConditionTypeLineageSynced,
			"status":             "False",
			"reason":             "LineageControllerAbsent",
			"message":            "InfrastructureLineageController is not yet deployed.",
			"lastTransitionTime": metav1.Now().UTC().Format("2006-01-02T15:04:05Z"),
		},
	}, "status", "conditions")
	return u
}

// expectedILIName returns the deterministic ILI name for a given GVK kind and root name.
// Format: {lowercasekind}-{name}. seam-core-schema.md §7.
func expectedILIName(kind, name string) string {
	return strings.ToLower(kind) + "-" + name
}

// buildRootLabeledCRD returns an unstructured CRD object for gvk carrying the
// lineage-root label. Added to fake clients so IsRootDeclaration returns true.
func buildRootLabeledCRD(gvk schema.GroupVersionKind) *unstructured.Unstructured {
	crd := &unstructured.Unstructured{}
	crd.SetGroupVersionKind(schema.GroupVersionKind{
		Group: "apiextensions.k8s.io", Version: "v1", Kind: "CustomResourceDefinition",
	})
	crd.SetName(controller.CRDNameForGVK(gvk))
	crd.SetLabels(map[string]string{"infrastructure.ontai.dev/lineage-root": "true"})
	return crd
}

// reconcileGVK runs the LineageReconciler for the given GVK and root object.
func reconcileGVK(t *testing.T, r *controller.LineageReconciler, name, namespace string) ctrl.Result {
	t.Helper()
	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: name, Namespace: namespace},
	})
	if err != nil {
		t.Fatalf("Reconcile error for %s/%s: %v", namespace, name, err)
	}
	return result
}

// ── WS7 scenario: all 9 GVKs produce correct ILI ────────────────────────────

// TestLineageController_AllGVKs_ProduceILIWithCorrectRootBinding is a table-driven
// integration test that exercises the LineageReconciler for every GVK in
// controller.RootDeclarationGVKs. For each GVK it verifies:
//  1. ILI is created with name = {lowercasekind}-root-{n}
//  2. ILI.Spec.RootBinding.RootKind matches the GVK Kind
//  3. ILI.Spec.RootBinding.RootName matches the root object name
//  4. ILI.Spec.RootBinding.RootUID matches the root object UID
//  5. governance annotation controller-authored=true is present on the ILI
//
// seam-core-schema.md §7. Decision 3: LineageIndex instances are controller-authored.
func TestLineageController_AllGVKs_ProduceILIWithCorrectRootBinding(t *testing.T) {
	const ns = "seam-system"

	for i, gvk := range controller.RootDeclarationGVKs {
		gvk := gvk // capture for parallel safety if needed
		t.Run(fmt.Sprintf("%s/%s", gvk.Group, gvk.Kind), func(t *testing.T) {
			s := newGVKScheme(t)
			rootName := fmt.Sprintf("root-%d", i)
			root := buildRootDeclaration(gvk, rootName, ns)

			c := fake.NewClientBuilder().
				WithScheme(s).
				WithObjects(root, buildRootLabeledCRD(gvk)).
				WithStatusSubresource(root, &seamv1alpha1.LineageRecord{}).
				Build()
			r := &controller.LineageReconciler{Client: c, Scheme: s, GVK: gvk}

			result := reconcileGVK(t, r, rootName, ns)
			if result.RequeueAfter != 0 {
				t.Errorf("expected no requeue, got %+v", result)
			}

			iliName := expectedILIName(gvk.Kind, rootName)
			ili := &seamv1alpha1.LineageRecord{}
			if err := c.Get(context.Background(), client.ObjectKey{Name: iliName, Namespace: ns}, ili); err != nil {
				t.Fatalf("ILI %s not created: %v", iliName, err)
			}

			rb := ili.Spec.RootBinding
			if rb.RootKind != gvk.Kind {
				t.Errorf("RootKind = %q, want %q", rb.RootKind, gvk.Kind)
			}
			if rb.RootName != rootName {
				t.Errorf("RootName = %q, want %q", rb.RootName, rootName)
			}
			if rb.RootUID != types.UID("uid-"+rootName) {
				t.Errorf("RootUID = %q, want uid-%s", rb.RootUID, rootName)
			}
			if rb.RootObservedGeneration != 1 {
				t.Errorf("RootObservedGeneration = %d, want 1", rb.RootObservedGeneration)
			}

			// Governance annotation: controller-authored. Decision 3.
			annotations := ili.GetAnnotations()
			if annotations[controller.GovernanceAnnotationControllerAuthored] != "true" {
				t.Errorf("controller-authored annotation = %q, want true",
					annotations[controller.GovernanceAnnotationControllerAuthored])
			}
		})
	}
}

// TestLineageController_AllGVKs_LineageSyncedTransitionsToTrue verifies that after
// the LineageReconciler creates the ILI, it patches the root declaration's
// LineageSynced condition to True. This is the "one-time write" contract.
// seam-core-schema.md §7 Declaration 5.
func TestLineageController_AllGVKs_LineageSyncedTransitionsToTrue(t *testing.T) {
	const ns = "seam-system"

	for i, gvk := range controller.RootDeclarationGVKs {
		gvk := gvk
		t.Run(fmt.Sprintf("%s/%s", gvk.Group, gvk.Kind), func(t *testing.T) {
			s := newGVKScheme(t)
			rootName := fmt.Sprintf("synced-%d", i)
			root := buildRootDeclaration(gvk, rootName, ns)

			c := fake.NewClientBuilder().
				WithScheme(s).
				WithObjects(root, buildRootLabeledCRD(gvk)).
				WithStatusSubresource(root, &seamv1alpha1.LineageRecord{}).
				Build()
			r := &controller.LineageReconciler{Client: c, Scheme: s, GVK: gvk}

			reconcileGVK(t, r, rootName, ns)

			// Read updated root declaration and check LineageSynced=True.
			updated := &unstructured.Unstructured{}
			updated.SetGroupVersionKind(gvk)
			if err := c.Get(context.Background(), client.ObjectKey{Name: rootName, Namespace: ns}, updated); err != nil {
				t.Fatalf("get updated root: %v", err)
			}

			conditions, _, _ := unstructured.NestedSlice(updated.Object, "status", "conditions")
			var found bool
			for _, raw := range conditions {
				cond, ok := raw.(map[string]interface{})
				if !ok {
					continue
				}
				if cond["type"] == seamv1alpha1.ConditionTypeLineageSynced {
					found = true
					if cond["status"] != "True" {
						t.Errorf("LineageSynced.Status = %q, want True", cond["status"])
					}
					break
				}
			}
			if !found {
				t.Error("LineageSynced condition not found on root declaration after reconcile")
			}
		})
	}
}

// TestLineageController_AllGVKs_ILINameFormat verifies that the ILI name follows
// the deterministic {lowercasekind}-{name} format for every GVK.
// This guards against naming drift that would break cross-operator ILI lookups.
func TestLineageController_AllGVKs_ILINameFormat(t *testing.T) {
	const ns = "seam-system"

	for i, gvk := range controller.RootDeclarationGVKs {
		gvk := gvk
		t.Run(fmt.Sprintf("%s/%s", gvk.Group, gvk.Kind), func(t *testing.T) {
			s := newGVKScheme(t)
			rootName := fmt.Sprintf("name-fmt-%d", i)
			root := buildRootDeclaration(gvk, rootName, ns)

			c := fake.NewClientBuilder().
				WithScheme(s).
				WithObjects(root, buildRootLabeledCRD(gvk)).
				WithStatusSubresource(root, &seamv1alpha1.LineageRecord{}).
				Build()
			r := &controller.LineageReconciler{Client: c, Scheme: s, GVK: gvk}

			reconcileGVK(t, r, rootName, ns)

			wantName := strings.ToLower(gvk.Kind) + "-" + rootName
			ili := &seamv1alpha1.LineageRecord{}
			if err := c.Get(context.Background(), client.ObjectKey{Name: wantName, Namespace: ns}, ili); err != nil {
				t.Errorf("ILI with expected name %q not found: %v", wantName, err)
			}
		})
	}
}

// TestLineageController_GVKCount verifies the number of GVKs registered.
// Guards against silent additions or removals. seam-core-schema.md §7.
func TestLineageController_GVKCount(t *testing.T) {
	// 1 platform infra + 10 platform operational + 2 platform CAPI +
	// 3 wrapper + 5 guardian = 21
	const expected = 21
	if got := len(controller.RootDeclarationGVKs); got != expected {
		t.Errorf("RootDeclarationGVKs count = %d, want %d", got, expected)
	}
}
