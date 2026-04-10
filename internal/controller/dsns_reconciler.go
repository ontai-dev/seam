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

// categoryForKind maps a GVK kind to a DSNS record category constant.
func categoryForKind(kind string) string {
	switch kind {
	case "TalosCluster", "SeamInfrastructureCluster", "SeamInfrastructureMachine":
		return idns.RecordCategoryClusterTopology
	case "IdentityBinding", "IdentityProvider":
		return idns.RecordCategoryIdentityPlane
	case "PackInstance", "ClusterPack", "PackExecution":
		return idns.RecordCategoryPackLineage
	case "RunnerConfig":
		return idns.RecordCategoryExecutionAuthority
	default:
		return idns.RecordCategoryClusterTopology
	}
}

// clusterContextForNamespace derives a cluster context string from a namespace.
// Strips the seam-tenant- prefix; returns "management" for non-tenant namespaces.
func clusterContextForNamespace(ns string) string {
	if c := clusterFromNamespace(ns); c != "" {
		return c
	}
	return "management"
}

// recordStrings converts a slice of dns.Record to display strings for DSNSEvent.DerivedRecords.
func recordStrings(records []idns.Record) []string {
	if len(records) == 0 {
		return nil
	}
	ss := make([]string, 0, len(records))
	for _, r := range records {
		ttl := r.TTL
		if ttl == 0 {
			ttl = idns.DefaultTTL
		}
		ss = append(ss, fmt.Sprintf("%s %d IN %s %s", r.Name, ttl, r.Type, r.Value))
	}
	return ss
}

// severityForObject returns the DSNSEvent severity for an object based on its
// current condition state. Degraded condition → warning; all other states → informational.
func severityForObject(obj *unstructured.Unstructured, kind string) string {
	if kind == "RunnerConfig" && hasConditionTrue(obj, "Degraded") {
		return idns.SeverityWarning
	}
	return idns.SeverityInformational
}

// deletionEvent constructs a DSNSEvent for a record-removal operation.
func deletionEvent(gvk schema.GroupVersionKind, req ctrl.Request) idns.DSNSEvent {
	return idns.DSNSEvent{
		RecordCategory: categoryForKind(gvk.Kind),
		Operation:      idns.OperationDeleted,
		SourceRef: idns.SourceRef{
			Group:     gvk.Group,
			Version:   gvk.Version,
			Kind:      gvk.Kind,
			Name:      req.Name,
			Namespace: req.Namespace,
		},
		ClusterContext: clusterContextForNamespace(req.Namespace),
		Severity:       idns.SeverityInformational,
	}
}

// updateEvent constructs a DSNSEvent for a record-write operation.
func updateEvent(gvk schema.GroupVersionKind, req ctrl.Request, obj *unstructured.Unstructured, records []idns.Record) idns.DSNSEvent {
	return idns.DSNSEvent{
		RecordCategory: categoryForKind(gvk.Kind),
		Operation:      idns.OperationUpdated,
		SourceRef: idns.SourceRef{
			Group:     gvk.Group,
			Version:   gvk.Version,
			Kind:      gvk.Kind,
			Name:      req.Name,
			Namespace: req.Namespace,
		},
		ClusterContext: clusterContextForNamespace(req.Namespace),
		DerivedRecords: recordStrings(records),
		Severity:       severityForObject(obj, gvk.Kind),
	}
}

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
			return ctrl.Result{}, r.State.Apply(ctx, deletionEvent(r.GVK, req))
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
		if err := r.State.Apply(ctx, deletionEvent(r.GVK, req)); err != nil {
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
	// TalosCluster: log the cluster name and Ready status before derivation to aid
	// diagnosis when the cluster is Ready but records are not appearing in the zone.
	// The reconciler watches all namespaces and requires Ready=True to emit records.
	if r.GVK.Kind == "TalosCluster" {
		ready := hasConditionTrue(obj, "Ready")
		logger.V(1).Info("reconciling TalosCluster DNS records",
			"cluster", obj.GetName(), "namespace", obj.GetNamespace(), "ready", ready)
	}
	records := r.deriveRecords(obj)
	r.State.UpdateRecords(ownerID, records)
	if err := r.State.Apply(ctx, updateEvent(r.GVK, req, obj, records)); err != nil {
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
		Named("dsns-" + strings.ToLower(r.GVK.Kind)).
		For(u).
		Complete(r)
}

// ── record derivation — one function per GVK ─────────────────────────────────

// deriveTalosClusterRecords emits:
//   - A record at {cluster-name} → spec.clusterEndpoint
//   - A record at api.{cluster-name} → spec.clusterEndpoint
//   - TXT record at role.{cluster-name} → status.origin (or spec.mode fallback)
//     OR sovereign NS delegation if spec.infrastructureProvider == "screen".
//
// Returns nil when Ready=False or spec.clusterEndpoint is empty.
//
// Bug fix: prior code read spec.vip and status.apiEndpoint — neither field exists
// in TalosClusterSpec. The correct unstructured key is "clusterEndpoint" (json tag
// from ClusterEndpoint string `json:"clusterEndpoint,omitempty"`).
// Bug fix: screen provider check used stale path spec.infrastructure.provider —
// corrected to spec.infrastructureProvider (json tag: infrastructureProvider).
// Bug fix: role TXT used spec.infrastructure.provider — replaced with status.origin
// (bootstrapped/imported) falling back to spec.mode (bootstrap/import).
// seam-core-schema.md §8 Decision 4 — Platform records.
func deriveTalosClusterRecords(obj *unstructured.Unstructured) []idns.Record {
	if !hasConditionTrue(obj, "Ready") {
		return nil
	}
	name := obj.GetName()

	// spec.clusterEndpoint is the json tag for ClusterEndpoint string on
	// TalosClusterSpec. platform commit 02132d1 added this field.
	clusterEndpoint, _, _ := unstructured.NestedString(obj.Object, "spec", "clusterEndpoint")
	if clusterEndpoint == "" {
		return nil
	}

	// spec.infrastructureProvider is the json tag for InfrastructureProvider on
	// TalosClusterSpec. Stale path was spec.infrastructure.provider (nested struct
	// that does not exist). platform-schema.md §5.
	provider, _, _ := unstructured.NestedString(obj.Object, "spec", "infrastructureProvider")

	records := []idns.Record{
		{Name: name, Type: idns.RecordTypeA, Value: clusterEndpoint},
		{Name: "api." + name, Type: idns.RecordTypeA, Value: clusterEndpoint},
	}

	if provider == "screen" {
		// Sovereign cluster delegation.
		// seam-core-schema.md §8 Decision 4 — sovereign NS delegation.
		nsFQDN := "ns." + name + "." + idns.Zone
		records = append(records,
			idns.Record{Name: name, Type: idns.RecordTypeNS, Value: nsFQDN},
			// Glue A record for the sovereign NS nameserver.
			idns.Record{Name: "ns." + name, Type: idns.RecordTypeA, Value: clusterEndpoint},
		)
	} else {
		// Role TXT: prefer status.origin (bootstrapped/imported), fall back to
		// spec.mode (bootstrap/import). Stale code read spec.infrastructure.provider.
		roleVal, _, _ := unstructured.NestedString(obj.Object, "status", "origin")
		if roleVal == "" {
			roleVal, _, _ = unstructured.NestedString(obj.Object, "spec", "mode")
		}
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
// carrying status.receiptDigest (or "delivered" when the digest is absent).
// Emitted whenever Ready=True — PackReceipt is written by the conductor agent
// on target clusters only, so receiptDigest is not required for record emission.
//
// spec.clusterPackRef is a flat string (pack name). Version is not available on
// PackInstance spec; "unknown" is used as a fallback per seam-core-schema.md §8.
// spec.targetClusterRef is read directly instead of deriving from namespace.
//
// seam-core-schema.md §8 Decision 4 — Wrapper records.
func derivePackInstanceRecords(obj *unstructured.Unstructured) []idns.Record {
	if !hasConditionTrue(obj, "Ready") {
		return nil
	}
	packName, _, _ := unstructured.NestedString(obj.Object, "spec", "clusterPackRef")
	clusterName, _, _ := unstructured.NestedString(obj.Object, "spec", "targetClusterRef")
	receiptDigest, _, _ := unstructured.NestedString(obj.Object, "status", "receiptDigest")

	if packName == "" {
		packName = obj.GetName()
	}
	// Version is not present on PackInstance spec; use "unknown" as a stable fallback.
	packVersion := "unknown"
	if clusterName == "" {
		// Fall back to namespace derivation for objects predating targetClusterRef.
		clusterName = clusterFromNamespace(obj.GetNamespace())
	}
	if clusterName == "" {
		return nil
	}
	value := receiptDigest
	if value == "" {
		// PackReceipt is written by the conductor agent on target clusters.
		// On the management cluster the digest may be absent; emit anyway.
		value = "delivered"
	}

	recName := fmt.Sprintf("pack.%s.%s.wrapper.%s", packName, packVersion, clusterName)
	return []idns.Record{
		{Name: recName, Type: idns.RecordTypeTXT, Value: value},
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
