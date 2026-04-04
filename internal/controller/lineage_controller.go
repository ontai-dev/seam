// Package controller implements the InfrastructureLineageController.
//
// The InfrastructureLineageController is the concrete realization of the abstract
// lineage aggregation ODC defined in domain-core-schema.md §4. It watches all
// root-declaration CRDs across the Seam operator family and creates one
// InfrastructureLineageIndex per root declaration, following the Lineage Index
// Pattern — seam-core-schema.md §3, CLAUDE.md §14 Decision 4.
//
// Authorship enforcement:
// The InfrastructureLineageController ServiceAccount is the only principal
// permitted to create or update InfrastructureLineageIndex instances.
// This is enforced at the admission webhook layer (deferred implementation).
// The controller itself annotates each ILI with the governance sub-prefix
// per seam-core-schema.md §7 Declaration 4.
package controller

import (
	"context"
	"fmt"
	"strings"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	seamv1alpha1 "github.com/ontai-dev/seam-core/api/v1alpha1"
)

// GovernanceAnnotationPrefix is the reserved governance sub-prefix under which
// the InfrastructureLineageController writes annotations on root declaration CRs
// and on InfrastructureLineageIndex instances. Individual Seam Operators are
// prohibited from writing under this sub-prefix. seam-core-schema.md §7 Declaration 4.
const GovernanceAnnotationPrefix = "governance.infrastructure.ontai.dev"

// GovernanceAnnotationLineageIndexRef is written on root declaration CRs to record
// the name of the InfrastructureLineageIndex created for that declaration.
const GovernanceAnnotationLineageIndexRef = GovernanceAnnotationPrefix + "/lineage-index-ref"

// GovernanceAnnotationControllerAuthored is written on InfrastructureLineageIndex
// instances to assert controller-authorship per CLAUDE.md §14 Decision 3.
const GovernanceAnnotationControllerAuthored = GovernanceAnnotationPrefix + "/controller-authored"

// ReasonLineageIndexCreated is the reason set on the LineageSynced condition when
// the InfrastructureLineageController successfully creates an ILI for the root declaration.
const ReasonLineageIndexCreated = "LineageIndexCreated"

// RootDeclarationGVK names all root-declaration CRD GroupVersionKinds that the
// InfrastructureLineageController watches. One InfrastructureLineageIndex is created
// per observed instance of any of these kinds.
//
// CLAUDE.md §14 Decision 4 — one index per root declaration across all operators.
var RootDeclarationGVKs = []schema.GroupVersionKind{
	// Platform operator — platform.ontai.dev
	{Group: "platform.ontai.dev", Version: "v1alpha1", Kind: "TalosCluster"},

	// Wrapper operator — infra.ontai.dev
	{Group: "infra.ontai.dev", Version: "v1alpha1", Kind: "ClusterPack"},
	{Group: "infra.ontai.dev", Version: "v1alpha1", Kind: "PackExecution"},
	{Group: "infra.ontai.dev", Version: "v1alpha1", Kind: "PackInstance"},

	// Guardian operator — security.ontai.dev
	{Group: "security.ontai.dev", Version: "v1alpha1", Kind: "RBACPolicy"},
	{Group: "security.ontai.dev", Version: "v1alpha1", Kind: "RBACProfile"},
	{Group: "security.ontai.dev", Version: "v1alpha1", Kind: "IdentityBinding"},
	{Group: "security.ontai.dev", Version: "v1alpha1", Kind: "IdentityProvider"},
	{Group: "security.ontai.dev", Version: "v1alpha1", Kind: "PermissionSet"},
}

// LineageReconciler watches a single root-declaration GVK and reconciles
// InfrastructureLineageIndex instances for each observed root declaration.
//
// One LineageReconciler instance is registered per GVK in RootDeclarationGVKs.
// All instances share the same reconcile logic — only the GVK field differs.
//
// Reconcile loop:
//  1. Fetch root declaration (unstructured). Not found → no-op (INV-006).
//  2. Check if governance annotation already set → if so, verify ILI exists.
//  3. Compute deterministic ILI name: lineageIndexName(kind, name).
//  4. Get ILI — if not found, create it with rootBinding from the root declaration.
//  5. Write governance annotation on root declaration metadata.
//  6. Transition LineageSynced condition to True on root declaration status.
type LineageReconciler struct {
	Client client.Client
	Scheme *runtime.Scheme
	GVK    schema.GroupVersionKind
}

// Reconcile is the reconcile loop for a single root-declaration GVK.
func (r *LineageReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithValues("gvk", r.GVK.String())

	// Step A — Fetch the root declaration as unstructured.
	root := &unstructured.Unstructured{}
	root.SetGroupVersionKind(r.GVK)
	if err := r.Client.Get(ctx, req.NamespacedName, root); err != nil {
		if apierrors.IsNotFound(err) {
			// Root declaration deleted. Lineage index is a permanent audit record —
			// we do not delete it on root deletion. INV-006: no Jobs on the delete path.
			logger.Info("root declaration not found — likely deleted, lineage index retained",
				"namespacedName", req.NamespacedName)
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("failed to get %s %s: %w", r.GVK.Kind, req.NamespacedName, err)
	}

	// Step B — Check idempotency guard: governance annotation already set means
	// this root declaration has already been processed. Verify ILI exists and return.
	iliName := lineageIndexName(r.GVK.Kind, root.GetName())
	if existing, ok := root.GetAnnotations()[GovernanceAnnotationLineageIndexRef]; ok && existing == iliName {
		// Idempotency guard: annotation already set with the correct ILI name.
		// Verify the ILI still exists; if not, re-create it.
		ili := &seamv1alpha1.InfrastructureLineageIndex{}
		err := r.Client.Get(ctx, client.ObjectKey{Name: iliName, Namespace: root.GetNamespace()}, ili)
		if err == nil {
			// ILI exists and annotation is set — ensure LineageSynced=True.
			return ctrl.Result{}, r.ensureLineageSyncedTrue(ctx, root, iliName)
		}
		if !apierrors.IsNotFound(err) {
			return ctrl.Result{}, fmt.Errorf("failed to get InfrastructureLineageIndex %s: %w", iliName, err)
		}
		// ILI was deleted — fall through to re-create.
		logger.Info("InfrastructureLineageIndex was deleted — re-creating",
			"iliName", iliName, "namespace", root.GetNamespace())
	}

	// Step C — Get or create the InfrastructureLineageIndex.
	ili := &seamv1alpha1.InfrastructureLineageIndex{}
	iliKey := client.ObjectKey{Name: iliName, Namespace: root.GetNamespace()}
	if err := r.Client.Get(ctx, iliKey, ili); err != nil {
		if !apierrors.IsNotFound(err) {
			return ctrl.Result{}, fmt.Errorf("failed to get InfrastructureLineageIndex %s: %w", iliName, err)
		}
		// Create the ILI. This is the authoritative creation path. CLAUDE.md §14 Decision 3.
		newILI := r.buildILI(root, iliName)
		if err := r.Client.Create(ctx, newILI); err != nil {
			if apierrors.IsAlreadyExists(err) {
				// Race between two reconcile calls — safe to proceed.
				logger.Info("InfrastructureLineageIndex already exists (race) — continuing",
					"iliName", iliName)
			} else {
				return ctrl.Result{}, fmt.Errorf("failed to create InfrastructureLineageIndex %s: %w", iliName, err)
			}
		} else {
			logger.Info("InfrastructureLineageIndex created",
				"iliName", iliName, "rootKind", r.GVK.Kind, "rootName", root.GetName())
		}
	}

	// Step D — Write governance annotation on root declaration metadata.
	// governance.infrastructure.ontai.dev/lineage-index-ref records the ILI name.
	// seam-core-schema.md §7 Declaration 4.
	if err := r.writeGovernanceAnnotation(ctx, root, iliName); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to write governance annotation: %w", err)
	}

	// Step E — Transition LineageSynced=True on root declaration status.
	if err := r.ensureLineageSyncedTrue(ctx, root, iliName); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to set LineageSynced=True: %w", err)
	}

	return ctrl.Result{}, nil
}

// buildILI constructs a new InfrastructureLineageIndex from the root declaration.
func (r *LineageReconciler) buildILI(root *unstructured.Unstructured, iliName string) *seamv1alpha1.InfrastructureLineageIndex {
	return &seamv1alpha1.InfrastructureLineageIndex{
		ObjectMeta: metav1.ObjectMeta{
			Name:      iliName,
			Namespace: root.GetNamespace(),
			Annotations: map[string]string{
				// Controller-authorship assertion per CLAUDE.md §14 Decision 3.
				GovernanceAnnotationControllerAuthored: "true",
				// Back-reference to the root declaration.
				GovernanceAnnotationPrefix + "/root-kind":      r.GVK.Kind,
				GovernanceAnnotationPrefix + "/root-name":      root.GetName(),
				GovernanceAnnotationPrefix + "/root-namespace": root.GetNamespace(),
			},
			Labels: map[string]string{
				"infrastructure.ontai.dev/root-kind": strings.ToLower(r.GVK.Kind),
				"infrastructure.ontai.dev/root-name": root.GetName(),
			},
		},
		Spec: seamv1alpha1.InfrastructureLineageIndexSpec{
			RootBinding: seamv1alpha1.InfrastructureLineageIndexRootBinding{
				RootKind:               r.GVK.Kind,
				RootName:               root.GetName(),
				RootNamespace:          root.GetNamespace(),
				RootUID:                root.GetUID(),
				RootObservedGeneration: root.GetGeneration(),
			},
			DescendantRegistry:  nil,
			PolicyBindingStatus: nil,
		},
	}
}

// writeGovernanceAnnotation patches the governance annotation onto the root
// declaration's metadata. This is an idempotent write — if the annotation is
// already correct, no patch is issued.
func (r *LineageReconciler) writeGovernanceAnnotation(ctx context.Context, root *unstructured.Unstructured, iliName string) error {
	existing := root.GetAnnotations()
	if val, ok := existing[GovernanceAnnotationLineageIndexRef]; ok && val == iliName {
		return nil // already set correctly
	}
	patch := client.MergeFrom(root.DeepCopyObject().(client.Object))
	annotations := root.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}
	annotations[GovernanceAnnotationLineageIndexRef] = iliName
	root.SetAnnotations(annotations)
	return r.Client.Patch(ctx, root, patch)
}

// ensureLineageSyncedTrue transitions the LineageSynced condition on the root
// declaration status from False/LineageControllerAbsent to True/LineageIndexCreated.
// This is the ownership-transfer write described in seam-core-schema.md §7 Declaration 5.
// It is idempotent — if LineageSynced is already True, no status patch is issued.
//
// root must be the current in-memory state of the root declaration, including any
// ResourceVersion updates from prior Patch() calls in the same reconcile cycle.
// writeGovernanceAnnotation updates root in-place, so root already has the latest
// ResourceVersion when this method is called — no re-fetch required.
func (r *LineageReconciler) ensureLineageSyncedTrue(ctx context.Context, root *unstructured.Unstructured, iliName string) error {
	// Read current conditions from status.
	rawConditions, _, _ := unstructured.NestedSlice(root.Object, "status", "conditions")

	// Check if LineageSynced is already True — idempotency guard.
	for _, rawCond := range rawConditions {
		cond, ok := rawCond.(map[string]interface{})
		if !ok {
			continue
		}
		if cond["type"] == seamv1alpha1.ConditionTypeLineageSynced &&
			cond["status"] == string(metav1.ConditionTrue) {
			return nil // already True — nothing to do
		}
	}

	// Build the updated conditions slice with LineageSynced=True.
	now := metav1.Now().UTC().Format(time.RFC3339)
	newCondition := map[string]interface{}{
		"type":               seamv1alpha1.ConditionTypeLineageSynced,
		"status":             string(metav1.ConditionTrue),
		"reason":             ReasonLineageIndexCreated,
		"message":            fmt.Sprintf("InfrastructureLineageIndex %q created by InfrastructureLineageController.", iliName),
		"lastTransitionTime": now,
		"observedGeneration": root.GetGeneration(),
	}

	// Replace existing LineageSynced entry or append new one.
	updated := make([]interface{}, 0, len(rawConditions)+1)
	replaced := false
	for _, rawCond := range rawConditions {
		cond, ok := rawCond.(map[string]interface{})
		if ok && cond["type"] == seamv1alpha1.ConditionTypeLineageSynced {
			updated = append(updated, newCondition)
			replaced = true
		} else {
			updated = append(updated, rawCond)
		}
	}
	if !replaced {
		updated = append(updated, newCondition)
	}

	// Capture patchBase before mutating root's status.
	// root.Patch() (called by writeGovernanceAnnotation) updates root in-place, so
	// root already has the latest ResourceVersion at this point.
	patchBase := root.DeepCopyObject().(client.Object)
	if err := unstructured.SetNestedSlice(root.Object, updated, "status", "conditions"); err != nil {
		return fmt.Errorf("failed to set conditions in unstructured: %w", err)
	}
	return r.Client.Status().Patch(ctx, root, client.MergeFrom(patchBase))
}

// lineageIndexName returns the deterministic InfrastructureLineageIndex name for
// a given root declaration kind and name. The format is:
// {lowercasekind}-{name}
// This is unique within a namespace because Kubernetes does not allow two objects
// of different GVKs to share the same ILI name within a namespace in practice.
func lineageIndexName(kind, name string) string {
	return strings.ToLower(kind) + "-" + name
}

// SetupWithManager registers the LineageReconciler as a controller for the GVK
// stored in r.GVK. The controller watches unstructured objects of that GVK.
func (r *LineageReconciler) SetupWithManager(mgr ctrl.Manager) error {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(r.GVK)
	return ctrl.NewControllerManagedBy(mgr).
		For(u).
		Complete(r)
}
