package unit_test

// Tests for the lineage-root CRD label filter and the 3-hop ownerReference resolver
// introduced in the LineageController admission filter (MIGRATION-3.8 Part 3).
//
// IsRootDeclaration reads the CRD object's metadata.labels to determine whether a
// GVK is a lineage root (infrastructure.ontai.dev/lineage-root="true").
//
// ResolveRootOwner walks ownerReferences up to 3 hops to find the nearest root.

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/ontai-dev/seam/internal/controller"
	"github.com/ontai-dev/seam/pkg/lineage"
)

// crdGVK is the GVK for CustomResourceDefinition objects.
var crdGVK = schema.GroupVersionKind{
	Group:   "apiextensions.k8s.io",
	Version: "v1",
	Kind:    "CustomResourceDefinition",
}

// newCRDObject builds a minimal unstructured CRD object for a given GVK, optionally
// carrying the lineage-root label.
func newCRDObject(gvk schema.GroupVersionKind, isRoot bool) *unstructured.Unstructured {
	crd := &unstructured.Unstructured{}
	crd.SetGroupVersionKind(crdGVK)
	crd.SetName(controller.CRDNameForGVK(gvk))
	if isRoot {
		crd.SetLabels(map[string]string{
			lineage.LabelLineageRoot: "true",
		})
	}
	return crd
}

// newUnstructuredObj builds a minimal unstructured object of the given GVK with
// optional ownerReferences.
func newUnstructuredObj(gvk schema.GroupVersionKind, name, ns string, ownerRefs []metav1.OwnerReference) *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(gvk)
	u.SetName(name)
	u.SetNamespace(ns)
	u.SetUID(types.UID("uid-" + name))
	u.SetGeneration(1)
	if len(ownerRefs) > 0 {
		u.SetOwnerReferences(ownerRefs)
	}
	return u
}

// ownerRef builds an OwnerReference with Controller=true.
func ownerRef(apiVersion, kind, name string, uid types.UID) metav1.OwnerReference {
	isController := true
	return metav1.OwnerReference{
		APIVersion: apiVersion,
		Kind:       kind,
		Name:       name,
		UID:        uid,
		Controller: &isController,
	}
}

func buildFakeClientForFilter(t *testing.T, objs ...client.Object) client.Client {
	t.Helper()
	s := newTestScheme(t)
	return fake.NewClientBuilder().WithScheme(s).WithObjects(objs...).Build()
}

// ── CRDNameForGVK tests ───────────────────────────────────────────────────────

func TestCRDNameForGVK_StandardKind(t *testing.T) {
	gvk := schema.GroupVersionKind{Group: "seam.ontai.dev", Version: "v1alpha1", Kind: "LineageRecord"}
	got := controller.CRDNameForGVK(gvk)
	if got != "lineagerecords.seam.ontai.dev" {
		t.Errorf("CRDNameForGVK = %q, want lineagerecords.seam.ontai.dev", got)
	}
}

func TestCRDNameForGVK_KindEndingInY(t *testing.T) {
	gvk := schema.GroupVersionKind{Group: "seam.ontai.dev", Version: "v1alpha1", Kind: "PackDelivery"}
	got := controller.CRDNameForGVK(gvk)
	if got != "packdeliveries.seam.ontai.dev" {
		t.Errorf("CRDNameForGVK = %q, want packdeliveries.seam.ontai.dev", got)
	}
}

func TestCRDNameForGVK_KindEndingInS(t *testing.T) {
	gvk := schema.GroupVersionKind{Group: "platform.ontai.dev", Version: "v1alpha1", Kind: "ClusterMaintenance"}
	got := controller.CRDNameForGVK(gvk)
	if got != "clustermaintenances.platform.ontai.dev" {
		t.Errorf("CRDNameForGVK = %q, want clustermaintenances.platform.ontai.dev", got)
	}
}

func TestCRDNameForGVK_TalosCluster(t *testing.T) {
	gvk := schema.GroupVersionKind{Group: "seam.ontai.dev", Version: "v1alpha1", Kind: "TalosCluster"}
	got := controller.CRDNameForGVK(gvk)
	if got != "talosclusters.seam.ontai.dev" {
		t.Errorf("CRDNameForGVK = %q, want talosclusters.seam.ontai.dev", got)
	}
}

// ── IsRootDeclaration tests ───────────────────────────────────────────────────

// TestIsRootDeclaration_CRDWithLabel verifies that a CRD carrying the lineage-root
// label causes IsRootDeclaration to return true.
func TestIsRootDeclaration_CRDWithLabel(t *testing.T) {
	gvk := schema.GroupVersionKind{Group: "seam.ontai.dev", Version: "v1alpha1", Kind: "TalosCluster"}
	crd := newCRDObject(gvk, true)

	c := buildFakeClientForFilter(t, crd)
	got, err := controller.IsRootDeclaration(context.Background(), c, gvk)
	if err != nil {
		t.Fatalf("IsRootDeclaration: %v", err)
	}
	if !got {
		t.Error("expected IsRootDeclaration=true for CRD with lineage-root label")
	}
}

// TestIsRootDeclaration_CRDWithoutLabel verifies that a CRD without the lineage-root
// label causes IsRootDeclaration to return false.
func TestIsRootDeclaration_CRDWithoutLabel(t *testing.T) {
	gvk := schema.GroupVersionKind{Group: "guardian.ontai.dev", Version: "v1alpha1", Kind: "RBACPolicy"}
	crd := newCRDObject(gvk, false)

	c := buildFakeClientForFilter(t, crd)
	got, err := controller.IsRootDeclaration(context.Background(), c, gvk)
	if err != nil {
		t.Fatalf("IsRootDeclaration: %v", err)
	}
	if got {
		t.Error("expected IsRootDeclaration=false for CRD without lineage-root label")
	}
}

// TestIsRootDeclaration_CRDNotFound verifies that when the CRD does not exist,
// IsRootDeclaration returns false without an error (NotFound is not an error).
func TestIsRootDeclaration_CRDNotFound(t *testing.T) {
	gvk := schema.GroupVersionKind{Group: "seam.ontai.dev", Version: "v1alpha1", Kind: "PackExecution"}
	// Do not add any CRD object to the fake client.
	c := buildFakeClientForFilter(t)

	got, err := controller.IsRootDeclaration(context.Background(), c, gvk)
	if err != nil {
		t.Fatalf("IsRootDeclaration returned error for missing CRD: %v", err)
	}
	if got {
		t.Error("expected IsRootDeclaration=false when CRD not found")
	}
}

// ── ResolveRootOwner tests ────────────────────────────────────────────────────

// TestResolveRootOwner_DirectOwnerIsRoot verifies that a 1-hop ownerRef pointing
// to a root-labelled GVK returns the owner on the first hop.
func TestResolveRootOwner_DirectOwnerIsRoot(t *testing.T) {
	const ns = "seam-system"
	rootGVK := schema.GroupVersionKind{Group: "seam.ontai.dev", Version: "v1alpha1", Kind: "TalosCluster"}
	derivedGVK := schema.GroupVersionKind{Group: "seam.ontai.dev", Version: "v1alpha1", Kind: "PackInstalled"}

	rootCRD := newCRDObject(rootGVK, true)
	rootObj := newUnstructuredObj(rootGVK, "prod-cluster", ns, nil)
	derived := newUnstructuredObj(derivedGVK, "rc-001", ns, []metav1.OwnerReference{
		ownerRef("seam.ontai.dev/v1alpha1", "TalosCluster", "prod-cluster", rootObj.GetUID()),
	})

	c := buildFakeClientForFilter(t, rootCRD, rootObj, derived)

	found, err := controller.ResolveRootOwner(context.Background(), c, derived, 3)
	if err != nil {
		t.Fatalf("ResolveRootOwner: %v", err)
	}
	if found == nil {
		t.Fatal("expected root to be found, got nil")
	}
	if found.GetName() != "prod-cluster" {
		t.Errorf("found.Name = %q, want prod-cluster", found.GetName())
	}
}

// TestResolveRootOwner_TwoHopsToRoot verifies that a 2-hop chain (derived → intermediate → root)
// resolves the root at hop 2.
func TestResolveRootOwner_TwoHopsToRoot(t *testing.T) {
	const ns = "seam-system"
	rootGVK := schema.GroupVersionKind{Group: "seam.ontai.dev", Version: "v1alpha1", Kind: "TalosCluster"}
	midGVK := schema.GroupVersionKind{Group: "seam.ontai.dev", Version: "v1alpha1", Kind: "PackDelivery"}
	leafGVK := schema.GroupVersionKind{Group: "seam.ontai.dev", Version: "v1alpha1", Kind: "PackInstalled"}

	rootCRD := newCRDObject(rootGVK, true)
	midCRD := newCRDObject(midGVK, false)
	rootObj := newUnstructuredObj(rootGVK, "root-cluster", ns, nil)
	midObj := newUnstructuredObj(midGVK, "pack-del-001", ns, []metav1.OwnerReference{
		ownerRef("seam.ontai.dev/v1alpha1", "TalosCluster", "root-cluster", rootObj.GetUID()),
	})
	leafObj := newUnstructuredObj(leafGVK, "pack-inst-001", ns, []metav1.OwnerReference{
		ownerRef("seam.ontai.dev/v1alpha1", "PackDelivery", "pack-del-001", midObj.GetUID()),
	})

	c := buildFakeClientForFilter(t, rootCRD, midCRD, rootObj, midObj, leafObj)

	found, err := controller.ResolveRootOwner(context.Background(), c, leafObj, 3)
	if err != nil {
		t.Fatalf("ResolveRootOwner: %v", err)
	}
	if found == nil {
		t.Fatal("expected root to be found at hop 2, got nil")
	}
	if found.GetName() != "root-cluster" {
		t.Errorf("found.Name = %q, want root-cluster", found.GetName())
	}
}

// TestResolveRootOwner_NoOwnerRefs verifies that an object with no ownerReferences
// returns nil without error.
func TestResolveRootOwner_NoOwnerRefs(t *testing.T) {
	const ns = "seam-system"
	leafGVK := schema.GroupVersionKind{Group: "seam.ontai.dev", Version: "v1alpha1", Kind: "PackExecution"}
	leafObj := newUnstructuredObj(leafGVK, "exec-001", ns, nil) // no ownerRefs

	c := buildFakeClientForFilter(t, leafObj)

	found, err := controller.ResolveRootOwner(context.Background(), c, leafObj, 3)
	if err != nil {
		t.Fatalf("ResolveRootOwner: %v", err)
	}
	if found != nil {
		t.Errorf("expected nil when no ownerRefs present, got %s", found.GetName())
	}
}

// TestResolveRootOwner_ExceedsMaxHops verifies that a chain deeper than maxHops
// returns nil without error (walk stops at the hop limit).
func TestResolveRootOwner_ExceedsMaxHops(t *testing.T) {
	const ns = "seam-system"
	gvk := schema.GroupVersionKind{Group: "seam.ontai.dev", Version: "v1alpha1", Kind: "PackInstalled"}
	crd := newCRDObject(gvk, false) // none of these are roots

	// Build a chain with 4 non-root hops before the root: A -> B -> C -> X -> D (root).
	// With maxHops=3, the walker examines owners at hop 1 (B), hop 2 (C), hop 3 (X)
	// and stops before reaching D (hop 4). Returns nil.
	rootGVK := schema.GroupVersionKind{Group: "seam.ontai.dev", Version: "v1alpha1", Kind: "TalosCluster"}
	rootCRD := newCRDObject(rootGVK, true)
	dObj := newUnstructuredObj(rootGVK, "deep-root", ns, nil)
	xObj := newUnstructuredObj(gvk, "hop-x", ns, []metav1.OwnerReference{
		ownerRef("seam.ontai.dev/v1alpha1", "TalosCluster", "deep-root", dObj.GetUID()),
	})
	cObj := newUnstructuredObj(gvk, "hop-c", ns, []metav1.OwnerReference{
		ownerRef("seam.ontai.dev/v1alpha1", "PackInstalled", "hop-x", xObj.GetUID()),
	})
	bObj := newUnstructuredObj(gvk, "hop-b", ns, []metav1.OwnerReference{
		ownerRef("seam.ontai.dev/v1alpha1", "PackInstalled", "hop-c", cObj.GetUID()),
	})
	aObj := newUnstructuredObj(gvk, "hop-a", ns, []metav1.OwnerReference{
		ownerRef("seam.ontai.dev/v1alpha1", "PackInstalled", "hop-b", bObj.GetUID()),
	})

	c := buildFakeClientForFilter(t, crd, rootCRD, dObj, xObj, cObj, bObj, aObj)

	found, err := controller.ResolveRootOwner(context.Background(), c, aObj, 3)
	if err != nil {
		t.Fatalf("ResolveRootOwner: %v", err)
	}
	if found != nil {
		t.Errorf("expected nil when root is beyond maxHops, got %s", found.GetName())
	}
}

// TestResolveRootOwner_ThreeHopsExact verifies that a 3-hop chain where the root
// is at exactly hop 3 is resolved successfully (boundary condition).
func TestResolveRootOwner_ThreeHopsExact(t *testing.T) {
	const ns = "seam-system"
	rootGVK := schema.GroupVersionKind{Group: "seam.ontai.dev", Version: "v1alpha1", Kind: "TalosCluster"}
	midGVK := schema.GroupVersionKind{Group: "seam.ontai.dev", Version: "v1alpha1", Kind: "PackDelivery"}
	leafGVK := schema.GroupVersionKind{Group: "seam.ontai.dev", Version: "v1alpha1", Kind: "PackInstalled"}

	rootCRD := newCRDObject(rootGVK, true)
	midCRD := newCRDObject(midGVK, false)

	rootObj := newUnstructuredObj(rootGVK, "exact-root", ns, nil)
	mid1 := newUnstructuredObj(midGVK, "mid1", ns, []metav1.OwnerReference{
		ownerRef("seam.ontai.dev/v1alpha1", "TalosCluster", "exact-root", rootObj.GetUID()),
	})
	mid2 := newUnstructuredObj(midGVK, "mid2", ns, []metav1.OwnerReference{
		ownerRef("seam.ontai.dev/v1alpha1", "PackDelivery", "mid1", mid1.GetUID()),
	})
	leafObj := newUnstructuredObj(leafGVK, "leaf", ns, []metav1.OwnerReference{
		ownerRef("seam.ontai.dev/v1alpha1", "PackDelivery", "mid2", mid2.GetUID()),
	})

	c := buildFakeClientForFilter(t, rootCRD, midCRD, rootObj, mid1, mid2, leafObj)

	found, err := controller.ResolveRootOwner(context.Background(), c, leafObj, 3)
	if err != nil {
		t.Fatalf("ResolveRootOwner: %v", err)
	}
	if found == nil {
		t.Fatal("expected root to be found at exactly 3 hops, got nil")
	}
	if found.GetName() != "exact-root" {
		t.Errorf("found.Name = %q, want exact-root", found.GetName())
	}
}
