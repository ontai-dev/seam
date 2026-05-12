package controller

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
)

// OutcomeReconciler watches derived object GVKs and appends OutcomeEntry records
// to the governing InfrastructureLineageIndex when a terminal condition is observed.
//
// One OutcomeReconciler instance is registered per derived-object GVK in
// DescendantReconciler.DerivedObjectGVKs. All share the same reconcile logic.
//
// Reconcile loop:
//  1. Fetch derived object (unstructured). Not found -> no-op.
//  2. Check if the object carries a root-ili label. Absent -> no-op.
//  3. Resolve the InfrastructureLineageIndex from the label.
//  4. Extract the object's UID. Look for an existing outcomeRegistry entry for this UID.
//     If found -> no-op (idempotent, never modify existing entries).
//  5. Classify the derived object's terminal condition. If not terminal -> no-op.
//  6. Append an OutcomeEntry to spec.outcomeRegistry via MergePatch.
//
// seam-core-schema.md §7 Declaration 6.
type OutcomeReconciler struct {
	Client client.Client
	Scheme *runtime.Scheme
	GVK    schema.GroupVersionKind
}

// Reconcile is the reconcile loop for a single derived-object GVK.
func (r *OutcomeReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithValues("gvk", r.GVK.String())

	// Step 1 -- Fetch the derived object.
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(r.GVK)
	if err := r.Client.Get(ctx, req.NamespacedName, obj); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("get %s %s: %w", r.GVK.Kind, req.NamespacedName, err)
	}

	// Step 2 -- Require root-ili label.
	labels := obj.GetLabels()
	iliName := labels[LabelRootILI]
	if iliName == "" {
		return ctrl.Result{}, nil
	}

	// Step 3 -- Resolve ILI.
	iliNamespace := labels[LabelRootILINamespace]
	if iliNamespace == "" {
		iliNamespace = obj.GetNamespace()
	}
	ili := &seamv1alpha1.LineageRecord{}
	iliKey := client.ObjectKey{Name: iliName, Namespace: iliNamespace}
	if err := r.Client.Get(ctx, iliKey, ili); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("get ILI %s: %w", iliName, err)
	}

	// Step 4 -- Idempotency: skip if an outcome entry already exists for this UID.
	uid := obj.GetUID()
	for _, entry := range ili.Spec.OutcomeRegistry {
		if entry.DerivedObjectUID == uid {
			return ctrl.Result{}, nil
		}
	}

	// Step 5 -- Classify terminal condition.
	outcomeType, reason, message := classifyTerminalOutcome(obj)
	if outcomeType == "" {
		return ctrl.Result{}, nil // not yet terminal
	}

	// Step 6 -- Append OutcomeEntry.
	now := metav1.Now()
	outcome := seamv1alpha1.OutcomeEntry{
		DerivedObjectUID: uid,
		OutcomeType:      outcomeType,
		OutcomeTimestamp: now,
		OutcomeRef:       reason,
		OutcomeDetail:    message,
	}

	patch := client.MergeFrom(ili.DeepCopy())
	ili.Spec.OutcomeRegistry = append(ili.Spec.OutcomeRegistry, outcome)
	if err := r.Client.Patch(ctx, ili, patch); err != nil {
		return ctrl.Result{}, fmt.Errorf("patch outcomeRegistry on ILI %s: %w", iliName, err)
	}

	logger.Info("appended OutcomeEntry to ILI",
		"ili", iliName, "kind", r.GVK.Kind, "name", obj.GetName(),
		"uid", uid, "outcomeType", outcomeType)
	return ctrl.Result{}, nil
}

// classifyTerminalOutcome inspects the conditions of a derived object and returns
// the terminal OutcomeType if a terminal condition is present. Returns empty string
// if no terminal condition is observed.
//
// Classification rules applied in order:
//  1. Any condition of type "Ready": status=True -> Succeeded; status=False reason
//     containing "drift" or "drifted" -> Drifted; reason containing "superseded" ->
//     Superseded; otherwise -> Failed.
//  2. Any condition of type "Drifted" with status=True -> Drifted.
//  3. Any condition of type "Succeeded" with status=True -> Succeeded.
//  4. Any condition of type "Failed" with status=True -> Failed.
func classifyTerminalOutcome(obj *unstructured.Unstructured) (outcomeType seamv1alpha1.OutcomeType, reason, message string) {
	conditions, _, _ := unstructured.NestedSlice(obj.Object, "status", "conditions")
	for _, raw := range conditions {
		cond, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		condType, _ := cond["type"].(string)
		condStatus, _ := cond["status"].(string)
		condReason, _ := cond["reason"].(string)
		condMessage, _ := cond["message"].(string)

		switch condType {
		case "Ready":
			if condStatus != string(metav1.ConditionTrue) && condStatus != string(metav1.ConditionFalse) {
				continue
			}
			if condStatus == string(metav1.ConditionTrue) {
				return seamv1alpha1.OutcomeTypeSucceeded, condReason, condMessage
			}
			// status=False: classify by reason.
			reasonLower := strings.ToLower(condReason)
			switch {
			case strings.Contains(reasonLower, "drift"):
				return seamv1alpha1.OutcomeTypeDrifted, condReason, condMessage
			case strings.Contains(reasonLower, "superseded"):
				return seamv1alpha1.OutcomeTypeSuperseded, condReason, condMessage
			default:
				return seamv1alpha1.OutcomeTypeFailed, condReason, condMessage
			}
		case "Drifted":
			if condStatus == string(metav1.ConditionTrue) {
				return seamv1alpha1.OutcomeTypeDrifted, condReason, condMessage
			}
		case "Succeeded":
			if condStatus == string(metav1.ConditionTrue) {
				return seamv1alpha1.OutcomeTypeSucceeded, condReason, condMessage
			}
		case "Failed":
			if condStatus == string(metav1.ConditionTrue) {
				return seamv1alpha1.OutcomeTypeFailed, condReason, condMessage
			}
		}
	}
	return "", "", ""
}

// SetupWithManager registers the OutcomeReconciler as a controller for the GVK
// stored in r.GVK.
func (r *OutcomeReconciler) SetupWithManager(mgr ctrl.Manager) error {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(r.GVK)
	return ctrl.NewControllerManagedBy(mgr).
		Named("outcome-" + strings.ToLower(r.GVK.Kind)).
		For(u).
		Complete(r)
}
