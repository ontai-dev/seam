// dsns_reconciler.go implements the Domain Semantic Name Service (DSNS) controller.
//
// DSNSReconciler watches root-declaration GVKs and projects their state to DNS
// records in the seam.ontave.dev zone via the dsns-zone ConfigMap in ont-system.
// One DSNSReconciler instance is registered per GVK in DSNSGVKs. All instances
// share a single *dns.DSNSState which holds the in-memory zone and ownership map.
//
// seam-core-schema.md §8 Decision 1 — DSNS controller, not a separate binary.
// seam-core-schema.md §8 Decision 4 — DNS record schema.
package controller

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	idns "github.com/ontai-dev/seam-core/internal/dns"
)

// DSNSFinalizer is added to every CRD watched by DSNSReconciler so that the
// controller can read all record-bearing fields before the object is fully deleted.
const DSNSFinalizer = "dsns.infrastructure.ontai.dev/cleanup"

// DSNSGVKs lists the GVKs watched by DSNSReconciler. These are the GVKs whose
// CRD state is projected to DNS records in seam.ontave.dev.
// seam-core-schema.md §8 Decision 4.
var DSNSGVKs = []schema.GroupVersionKind{
	// Platform operator — TalosCluster Ready state → cluster A, api A, role TXT
	// (or sovereign NS delegation for screen provider).
	{Group: "platform.ontai.dev", Version: "v1alpha1", Kind: "TalosCluster"},

	// Wrapper operator — PackInstance terminal Succeeded state → pack TXT.
	{Group: "infra.ontai.dev", Version: "v1alpha1", Kind: "PackInstance"},

	// Guardian operator — IdentityBinding resolved → identity TXT.
	{Group: "security.ontai.dev", Version: "v1alpha1", Kind: "IdentityBinding"},

	// Guardian operator — IdentityProvider Valid → idp TXT.
	{Group: "security.ontai.dev", Version: "v1alpha1", Kind: "IdentityProvider"},

	// Conductor — RunnerConfig terminal state → run TXT.
	{Group: "runner.ontai.dev", Version: "v1alpha1", Kind: "RunnerConfig"},
}

// DSNSReconciler watches a single GVK and projects CRD state to DNS records.
// All instances share the same *dns.DSNSState via the State field.
type DSNSReconciler struct {
	Client client.Client
	GVK    schema.GroupVersionKind
	State  *idns.DSNSState
}

// Reconcile is the reconcile loop entry point. It dispatches on r.GVK.Kind.
func (r *DSNSReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithValues("dsns-gvk", r.GVK.Kind, "name", req.Name, "ns", req.Namespace)

	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(r.GVK)

	ownerID := dsnsOwnerID(r.GVK.Kind, req.Namespace, req.Name)

	if err := r.Client.Get(ctx, req.NamespacedName, obj); err != nil {
		if apierrors.IsNotFound(err) {
			// Object fully deleted — perform best-effort record removal.
			// (This path is reached when the object had no finalizer or when
			// it was deleted and finalizer already removed in a prior cycle.)
			logger.Info("object not found — removing records")
			r.State.RemoveRecords(ownerID)
			return ctrl.Result{}, r.State.Apply(ctx)
		}
		return ctrl.Result{}, fmt.Errorf("get %s %s: %w", r.GVK.Kind, req.NamespacedName, err)
	}

	// Finalizer-gated deletion path: object exists but is being deleted.
	if !obj.GetDeletionTimestamp().IsZero() {
		if !containsDSNSFinalizer(obj) {
			return ctrl.Result{}, nil
		}
		logger.Info("object deleting — removing records and releasing finalizer")
		r.State.RemoveRecords(ownerID)
		if err := r.State.Apply(ctx); err != nil {
			return ctrl.Result{}, err
		}
		removeDSNSFinalizer(obj)
		return ctrl.Result{}, r.Client.Update(ctx, obj)
	}

	// Ensure our finalizer is present so we can clean up records on deletion.
	if !containsDSNSFinalizer(obj) {
		addDSNSFinalizer(obj)
		if err := r.Client.Update(ctx, obj); err != nil {
			return ctrl.Result{}, fmt.Errorf("add finalizer to %s %s: %w", r.GVK.Kind, req.NamespacedName, err)
		}
		// Re-fetch to get the latest ResourceVersion before deriving records.
		if err := r.Client.Get(ctx, req.NamespacedName, obj); err != nil {
			return ctrl.Result{}, fmt.Errorf("re-fetch after finalizer: %w", err)
		}
	}

	// Derive DNS records from the current object state.
	records := r.deriveRecords(obj)
	r.State.UpdateRecords(ownerID, records)
	if err := r.State.Apply(ctx); err != nil {
		return ctrl.Result{}, fmt.Errorf("apply zone to ConfigMap: %w", err)
	}

	logger.Info("zone updated", "ownerID", ownerID, "records", len(records))
	return ctrl.Result{}, nil
}

// deriveRecords dispatches on GVK kind and returns the DNS records this object
// should contribute to the zone. Returns nil when the object is not in a state
// that warrants DNS record emission (e.g. not Ready).
// seam-core-schema.md §8 Decision 4.
func (r *DSNSReconciler) deriveRecords(obj *unstructured.Unstructured) []idns.Record {
	switch r.GVK.Kind {
	case "TalosCluster":
		return deriveTalosClusterRecords(obj)
	case "IdentityBinding":
		return deriveIdentityBindingRecords(obj)
	case "IdentityProvider":
		return deriveIdentityProviderRecords(obj)
	case "PackInstance":
		return derivePackInstanceRecords(obj)
	case "RunnerConfig":
		return deriveRunnerConfigRecords(obj)
	default:
		return nil
	}
}

// SetupWithManager registers the DSNSReconciler as a controller for r.GVK.
func (r *DSNSReconciler) SetupWithManager(mgr ctrl.Manager) error {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(r.GVK)
	return ctrl.NewControllerManagedBy(mgr).
		For(u).
		Complete(r)
}

// ── record derivation — one function per GVK ─────────────────────────────────

// deriveTalosClusterRecords emits:
//   - A record at {cluster-name} → spec.vip
//   - A record at api.{cluster-name} → status.apiEndpoint
//   - TXT record at role.{cluster-name} → spec.infrastructure.provider
//     OR sovereign NS delegation if provider == "screen".
//
// seam-core-schema.md §8 Decision 4 — Platform records.
func deriveTalosClusterRecords(obj *unstructured.Unstructured) []idns.Record {
	if !hasConditionTrue(obj, "Ready") {
		return nil
	}
	name := obj.GetName()
	vip, _, _ := unstructured.NestedString(obj.Object, "spec", "vip")
	apiEndpoint, _, _ := unstructured.NestedString(obj.Object, "status", "apiEndpoint")
	provider, _, _ := unstructured.NestedString(obj.Object, "spec", "infrastructure", "provider")

	if vip == "" || apiEndpoint == "" {
		return nil
	}

	records := []idns.Record{
		{Name: name, Type: idns.RecordTypeA, Value: vip},
		{Name: "api." + name, Type: idns.RecordTypeA, Value: apiEndpoint},
	}

	if provider == "screen" {
		// Sovereign cluster delegation.
		// seam-core-schema.md §8 Decision 4 — sovereign NS delegation.
		nsFQDN := "ns." + name + "." + idns.Zone
		records = append(records,
			idns.Record{Name: name, Type: idns.RecordTypeNS, Value: nsFQDN},
			// Glue A record for the NS nameserver.
			idns.Record{Name: "ns." + name, Type: idns.RecordTypeA, Value: vip},
		)
	} else {
		roleVal := provider
		if roleVal == "" {
			roleVal = "general"
		}
		records = append(records, idns.Record{Name: "role." + name, Type: idns.RecordTypeTXT, Value: roleVal})
	}

	return records
}

// deriveIdentityBindingRecords emits a TXT record at:
//
//	identity.{sha256hex16}.guardian.{cluster-name}
//
// carrying "{rbacProfileName} {identityProviderName}". Cluster name is derived
// from the namespace (seam-tenant-{cluster}). Only emitted when TrustAnchorResolved=True.
//
// seam-core-schema.md §8 Decision 4 — Guardian records.
func deriveIdentityBindingRecords(obj *unstructured.Unstructured) []idns.Record {
	if !hasConditionTrue(obj, "TrustAnchorResolved") {
		return nil
	}
	clusterName := clusterFromNamespace(obj.GetNamespace())
	if clusterName == "" {
		return nil
	}
	subject, _, _ := unstructured.NestedString(obj.Object, "spec", "subject")
	if subject == "" {
		return nil
	}
	rbacProfile, _, _ := unstructured.NestedString(obj.Object, "spec", "rbacProfileRef", "name")
	idpName, _, _ := unstructured.NestedString(obj.Object, "spec", "identityProviderRef", "name")

	hash := subjectHash(subject)
	recName := fmt.Sprintf("identity.%s.guardian.%s", hash, clusterName)
	recValue := fmt.Sprintf("%s %s", rbacProfile, idpName)
	return []idns.Record{
		{Name: recName, Type: idns.RecordTypeTXT, Value: recValue},
	}
}

// deriveIdentityProviderRecords emits a TXT record at:
//
//	idp.{provider-name}.guardian
//
// carrying status.issuerURL. Only emitted when Valid=True.
//
// seam-core-schema.md §8 Decision 4 — Guardian records.
func deriveIdentityProviderRecords(obj *unstructured.Unstructured) []idns.Record {
	if !hasConditionTrue(obj, "Valid") {
		return nil
	}
	issuerURL, _, _ := unstructured.NestedString(obj.Object, "status", "issuerURL")
	if issuerURL == "" {
		return nil
	}
	recName := fmt.Sprintf("idp.%s.guardian", obj.GetName())
	return []idns.Record{
		{Name: recName, Type: idns.RecordTypeTXT, Value: issuerURL},
	}
}

// derivePackInstanceRecords emits a TXT record at:
//
//	pack.{pack-name}.{pack-version}.wrapper.{cluster-name}
//
// carrying status.receiptDigest. Only emitted when the PackInstance is in
// terminal Succeeded state (Ready=True / PackReceiptReady).
//
// seam-core-schema.md §8 Decision 4 — Wrapper records.
func derivePackInstanceRecords(obj *unstructured.Unstructured) []idns.Record {
	if !hasConditionTrue(obj, "Ready") {
		return nil
	}
	clusterName := clusterFromNamespace(obj.GetNamespace())
	if clusterName == "" {
		return nil
	}
	packName, _, _ := unstructured.NestedString(obj.Object, "spec", "packRef", "name")
	packVersion, _, _ := unstructured.NestedString(obj.Object, "spec", "packRef", "version")
	receiptDigest, _, _ := unstructured.NestedString(obj.Object, "status", "receiptDigest")

	if packName == "" {
		packName = obj.GetName()
	}
	if packVersion == "" {
		packVersion = "unknown"
	}
	if receiptDigest == "" {
		return nil
	}

	recName := fmt.Sprintf("pack.%s.%s.wrapper.%s", packName, packVersion, clusterName)
	return []idns.Record{
		{Name: recName, Type: idns.RecordTypeTXT, Value: receiptDigest},
	}
}

// deriveRunnerConfigRecords emits a TXT record at:
//
//	run.{name}.conductor.{cluster-name}
//
// carrying "phase={phase} completed={lastTransitionTime}". Only emitted when
// the RunnerConfig has reached terminal state (Ready=True or Degraded=True).
//
// seam-core-schema.md §8 Decision 4 — Conductor records.
func deriveRunnerConfigRecords(obj *unstructured.Unstructured) []idns.Record {
	clusterName := clusterFromNamespace(obj.GetNamespace())
	if clusterName == "" {
		clusterName = "management"
	}

	var phase, completedAt string
	if hasConditionTrue(obj, "Ready") {
		phase = "Completed"
		completedAt = conditionLastTransitionTime(obj, "Ready")
	} else if hasConditionTrue(obj, "Degraded") {
		phase = "Failed"
		completedAt = conditionLastTransitionTime(obj, "Degraded")
	} else {
		return nil
	}

	recName := fmt.Sprintf("run.%s.conductor.%s", obj.GetName(), clusterName)
	recValue := fmt.Sprintf("phase=%s completed=%s", phase, completedAt)
	return []idns.Record{
		{Name: recName, Type: idns.RecordTypeTXT, Value: recValue},
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

// dsnsOwnerID returns the deduplication key for record ownership tracking.
func dsnsOwnerID(kind, namespace, name string) string {
	return kind + "/" + namespace + "/" + name
}

// hasConditionTrue returns true if the object has a condition of the given type
// with status "True" in status.conditions.
func hasConditionTrue(obj *unstructured.Unstructured, condType string) bool {
	conditions, _, _ := unstructured.NestedSlice(obj.Object, "status", "conditions")
	for _, raw := range conditions {
		cond, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		if cond["type"] == condType && cond["status"] == "True" {
			return true
		}
	}
	return false
}

// conditionLastTransitionTime returns the lastTransitionTime for the first
// condition of the given type, or an empty string if not found.
func conditionLastTransitionTime(obj *unstructured.Unstructured, condType string) string {
	conditions, _, _ := unstructured.NestedSlice(obj.Object, "status", "conditions")
	for _, raw := range conditions {
		cond, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		if cond["type"] == condType {
			if t, ok := cond["lastTransitionTime"].(string); ok {
				return t
			}
		}
	}
	return ""
}

// clusterFromNamespace derives a cluster name from a seam-tenant-{cluster}
// namespace. Returns an empty string for non-tenant namespaces.
func clusterFromNamespace(ns string) string {
	const prefix = "seam-tenant-"
	if !strings.HasPrefix(ns, prefix) {
		return ""
	}
	return strings.TrimPrefix(ns, prefix)
}

// subjectHash returns the first 16 hex characters of the sha256 hash of subject.
// This is used to construct the identity DNS record name.
func subjectHash(subject string) string {
	sum := sha256.Sum256([]byte(subject))
	return hex.EncodeToString(sum[:])[:16]
}

// containsDSNSFinalizer returns true if obj has the DSNS finalizer.
func containsDSNSFinalizer(obj *unstructured.Unstructured) bool {
	for _, f := range obj.GetFinalizers() {
		if f == DSNSFinalizer {
			return true
		}
	}
	return false
}

// addDSNSFinalizer appends the DSNS finalizer to obj's finalizer list.
func addDSNSFinalizer(obj *unstructured.Unstructured) {
	obj.SetFinalizers(append(obj.GetFinalizers(), DSNSFinalizer))
}

// removeDSNSFinalizer removes the DSNS finalizer from obj's finalizer list.
func removeDSNSFinalizer(obj *unstructured.Unstructured) {
	current := obj.GetFinalizers()
	updated := make([]string, 0, len(current))
	for _, f := range current {
		if f != DSNSFinalizer {
			updated = append(updated, f)
		}
	}
	obj.SetFinalizers(updated)
}
