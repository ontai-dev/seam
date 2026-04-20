package controller

// DescendantReconciler watches a single derived-object GVK. When a derived object
// carries the label infrastructure.ontai.dev/root-ili, the reconciler appends a
// DescendantEntry to the named InfrastructureLineageIndex in the same namespace.
//
// This is the append path for ILI.Spec.DescendantRegistry. Operators set the
// required labels on derived objects (RunnerConfig, etc.) at creation time.
// The LineageController prunes stale entries; this reconciler only appends.
//
// Required labels on derived objects:
//   infrastructure.ontai.dev/root-ili           -- ILI name (e.g., "taloscluster-prod")
//   infrastructure.ontai.dev/seam-operator       -- operator name (e.g., "platform")
//   infrastructure.ontai.dev/creation-rationale  -- CreationRationale value
//
// seam-core-schema.md §3. CLAUDE.md §14 Decision 4.

import (
	"context"
	"fmt"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	seamv1alpha1 "github.com/ontai-dev/seam-core/api/v1alpha1"
	"github.com/ontai-dev/seam-core/pkg/lineage"
)

// Label keys operators must set on derived objects to trigger descendant tracking.
const (
	LabelRootILI           = "infrastructure.ontai.dev/root-ili"
	LabelSeamOperator      = "infrastructure.ontai.dev/seam-operator"
	LabelCreationRationale = "infrastructure.ontai.dev/creation-rationale"
)

// DerivedObjectGVKs lists the derived-object GVKs that the DescendantReconciler
// watches. Operators set LabelRootILI on objects of these kinds at creation time.
// One DescendantReconciler instance is registered per GVK in main.go.
var DerivedObjectGVKs = []schema.GroupVersionKind{
	{Group: "runner.ontai.dev", Version: "v1alpha1", Kind: "RunnerConfig"},
	{Group: "infra.ontai.dev", Version: "v1alpha1", Kind: "PackInstance"},
}

// DescendantReconciler watches a single derived-object GVK and appends
// DescendantEntry records to the ILI named by LabelRootILI on each object.
type DescendantReconciler struct {
	Client client.Client
	Scheme *runtime.Scheme
	GVK    schema.GroupVersionKind
}

// Reconcile is the reconcile loop for a single derived-object GVK.
func (r *DescendantReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithValues("gvk", r.GVK.String())

	// Fetch the derived object as unstructured.
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(r.GVK)
	if err := r.Client.Get(ctx, req.NamespacedName, obj); err != nil {
		if apierrors.IsNotFound(err) {
			// INV-006: no action on the delete path.
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("get %s %s: %w", r.GVK.Kind, req.NamespacedName, err)
	}

	labels := obj.GetLabels()
	iliName, ok := labels[LabelRootILI]
	if !ok || iliName == "" {
		// No lineage label -- this derived object is not tracked by any ILI.
		return ctrl.Result{}, nil
	}

	// Fetch the referenced ILI.
	ili := &seamv1alpha1.InfrastructureLineageIndex{}
	iliKey := client.ObjectKey{Name: iliName, Namespace: obj.GetNamespace()}
	if err := r.Client.Get(ctx, iliKey, ili); err != nil {
		if apierrors.IsNotFound(err) {
			// ILI not yet created -- requeue and wait for the LineageReconciler.
			logger.Info("ILI not found, requeuing",
				"ili", iliName, "namespace", obj.GetNamespace())
			return ctrl.Result{Requeue: true}, nil
		}
		return ctrl.Result{}, fmt.Errorf("get InfrastructureLineageIndex %s: %w", iliName, err)
	}

	uid := obj.GetUID()

	// Idempotency guard: check if this object is already registered.
	for _, entry := range ili.Spec.DescendantRegistry {
		if entry.UID == uid {
			return ctrl.Result{}, nil
		}
	}

	// Build the DescendantEntry from object metadata and labels.
	now := metav1.Now()
	entry := seamv1alpha1.DescendantEntry{
		Group:                    r.GVK.Group,
		Version:                  r.GVK.Version,
		Kind:                     r.GVK.Kind,
		Name:                     obj.GetName(),
		Namespace:                obj.GetNamespace(),
		UID:                      uid,
		SeamOperator:             labels[LabelSeamOperator],
		CreationRationale:        lineage.CreationRationale(labels[LabelCreationRationale]),
		RootGenerationAtCreation: obj.GetGeneration(),
		RecordedAt:               &now,
	}

	// Append to DescendantRegistry via patch.
	patch := client.MergeFrom(ili.DeepCopy())
	ili.Spec.DescendantRegistry = append(ili.Spec.DescendantRegistry, entry)
	if err := r.Client.Patch(ctx, ili, patch); err != nil {
		return ctrl.Result{}, fmt.Errorf("patch DescendantRegistry on ILI %s: %w", iliName, err)
	}

	logger.Info("appended DescendantEntry to ILI",
		"ili", iliName, "kind", r.GVK.Kind, "name", obj.GetName(),
		"uid", uid)
	return ctrl.Result{}, nil
}

// SetupWithManager registers the DescendantReconciler as a controller for the GVK
// stored in r.GVK. The controller watches unstructured objects of that GVK.
func (r *DescendantReconciler) SetupWithManager(mgr ctrl.Manager) error {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(r.GVK)
	return ctrl.NewControllerManagedBy(mgr).
		Named("descendant-" + strings.ToLower(r.GVK.Kind)).
		For(u).
		Complete(r)
}
