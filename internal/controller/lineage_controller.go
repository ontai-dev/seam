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
	lineagepkg "github.com/ontai-dev/seam-core/pkg/lineage"
)

// GovernanceAnnotationPrefix is the reserved governance sub-prefix under which
// the LineageController writes annotations on root declaration CRs
// and on LineageRecord instances. Individual Seam Operators are
// prohibited from writing under this sub-prefix. seam-core-schema.md §7 Declaration 4.
const GovernanceAnnotationPrefix = "governance.infrastructure.ontai.dev"

// GovernanceAnnotationLineageIndexRef is written on root declaration CRs to record
// the name of the LineageRecord created for that declaration.
const GovernanceAnnotationLineageIndexRef = GovernanceAnnotationPrefix + "/lineage-index-ref"

// GovernanceAnnotationControllerAuthored is written on LineageRecord
// instances to assert controller-authorship per CLAUDE.md Decision 3.
const GovernanceAnnotationControllerAuthored = GovernanceAnnotationPrefix + "/controller-authored"

// ReasonLineageIndexCreated is the reason set on the LineageSynced condition when
// the LineageController successfully creates a LineageRecord for the root declaration.
const ReasonLineageIndexCreated = "LineageIndexCreated"

// InfrastructureDomainRef is the canonical domainRef value for all Seam infrastructure
// LineageRecords. CLAUDE.md Decision 2.
const InfrastructureDomainRef = "infrastructure.core.ontai.dev"

// RootDeclarationGVK names all root-declaration CRD GroupVersionKinds that the
// LineageController watches. One LineageRecord is created per observed instance
// of any of these kinds.
//
// CLAUDE.md Decision 4 -- one record per root declaration across all operators.
var RootDeclarationGVKs = []schema.GroupVersionKind{
	// Platform operator -- seam.ontai.dev (MIGRATION-3.1)
	{Group: "seam.ontai.dev", Version: "v1alpha1", Kind: "TalosCluster"},

	// Platform operator -- platform.ontai.dev (operational root declarations)
	{Group: "platform.ontai.dev", Version: "v1alpha1", Kind: "UpgradePolicy"},
	{Group: "platform.ontai.dev", Version: "v1alpha1", Kind: "NodeMaintenance"},
	{Group: "platform.ontai.dev", Version: "v1alpha1", Kind: "ClusterMaintenance"},
	{Group: "platform.ontai.dev", Version: "v1alpha1", Kind: "PKIRotation"},
	{Group: "platform.ontai.dev", Version: "v1alpha1", Kind: "ClusterReset"},
	{Group: "platform.ontai.dev", Version: "v1alpha1", Kind: "NodeOperation"},
	{Group: "platform.ontai.dev", Version: "v1alpha1", Kind: "EtcdMaintenance"},
	{Group: "platform.ontai.dev", Version: "v1alpha1", Kind: "TalosMachineConfigBackup"},
	{Group: "platform.ontai.dev", Version: "v1alpha1", Kind: "TalosMachineConfigRestore"},
	{Group: "platform.ontai.dev", Version: "v1alpha1", Kind: "HardeningProfile"},

	// Platform CAPI provider -- infrastructure.ontai.dev
	{Group: "infrastructure.ontai.dev", Version: "v1alpha1", Kind: "SeamInfrastructureCluster"},
	{Group: "infrastructure.ontai.dev", Version: "v1alpha1", Kind: "SeamInfrastructureMachine"},

	// Dispatcher operator -- seam.ontai.dev (MIGRATION-3.3-3.7)
	{Group: "seam.ontai.dev", Version: "v1alpha1", Kind: "PackDelivery"},
	{Group: "seam.ontai.dev", Version: "v1alpha1", Kind: "PackExecution"},
	{Group: "seam.ontai.dev", Version: "v1alpha1", Kind: "PackInstalled"},

	// Guardian operator -- guardian.ontai.dev (constitutional refactor 2026-05-12)
	{Group: "guardian.ontai.dev", Version: "v1alpha1", Kind: "RBACPolicy"},
	{Group: "guardian.ontai.dev", Version: "v1alpha1", Kind: "RBACProfile"},
	{Group: "guardian.ontai.dev", Version: "v1alpha1", Kind: "IdentityBinding"},
	{Group: "guardian.ontai.dev", Version: "v1alpha1", Kind: "IdentityProvider"},
	{Group: "guardian.ontai.dev", Version: "v1alpha1", Kind: "PermissionSet"},
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

	// Step A1 -- Check the lineage-root CRD label. Only GVKs whose CRD carries
	// infrastructure.ontai.dev/lineage-root="true" produce a new LineageRecord.
	// All other GVKs are derived objects; they are forwarded to handleDerivedObject
	// which resolves the nearest root via ownerReferences.
	isRoot, err := IsRootDeclaration(ctx, r.Client, r.GVK)
	if err != nil {
		logger.Error(err, "CRD lineage-root label check failed — treating as non-root; requeuing")
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}
	if !isRoot {
		return r.handleDerivedObject(ctx, root)
	}

	// Step B — Check idempotency guard: governance annotation already set means
	// this root declaration has already been processed. Verify ILI exists and return.
	iliName := lineageIndexName(r.GVK.Kind, root.GetName())
	if existing, ok := root.GetAnnotations()[GovernanceAnnotationLineageIndexRef]; ok && existing == iliName {
		// Idempotency guard: annotation already set with the correct ILI name.
		// Verify the ILI still exists; if not, re-create it.
		ili := &seamv1alpha1.LineageRecord{}
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
	ili := &seamv1alpha1.LineageRecord{}
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
		// Re-fetch the ILI so downstream steps have a populated ResourceVersion.
		if err := r.Client.Get(ctx, iliKey, ili); err != nil {
			if apierrors.IsNotFound(err) {
				// Very unlikely: just created or a concurrent delete. Requeue.
				return ctrl.Result{Requeue: true}, nil
			}
			return ctrl.Result{}, fmt.Errorf("re-fetch InfrastructureLineageIndex %s: %w", iliName, err)
		}
	}

	// Step Cf — Prune stale descendant entries from the DescendantRegistry.
	// An entry is pruned when the referenced object is confirmed not-found AND
	// the entry's CreatedAt timestamp is older than the retention window.
	// conductor-schema.md (retention enforcement).
	if err := r.pruneStaleDescendants(ctx, ili); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to prune stale descendants: %w", err)
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

	// Step Eg — If deleteWithRoot=true, ensure ownerReference is set on the ILI
	// pointing to the root declaration so Kubernetes GC cascades deletion.
	if err := r.ensureOwnerReferenceIfDeleteWithRoot(ctx, root, ili); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to ensure ownerReference: %w", err)
	}

	return ctrl.Result{}, nil
}

// buildILI constructs a new InfrastructureLineageIndex from the root declaration.
// It reads the infrastructure.ontai.dev/declaring-principal annotation from the
// root declaration and populates rootBinding.declaringPrincipal. If the annotation
// is absent (bootstrap window or pre-amendment object), declaringPrincipal is set
// to "system:unknown". seam-core-schema.md §7 Declaration 6.
func (r *LineageReconciler) buildILI(root *unstructured.Unstructured, iliName string) *seamv1alpha1.LineageRecord {
	declaringPrincipal := root.GetAnnotations()["infrastructure.ontai.dev/declaring-principal"]
	if declaringPrincipal == "" {
		declaringPrincipal = "system:unknown"
	}

	return &seamv1alpha1.LineageRecord{
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
		Spec: seamv1alpha1.LineageRecordSpec{
			RootBinding: seamv1alpha1.LineageRecordRootBinding{
				RootKind:               r.GVK.Kind,
				RootName:               root.GetName(),
				RootNamespace:          root.GetNamespace(),
				RootUID:                root.GetUID(),
				RootObservedGeneration: root.GetGeneration(),
				DeclaringPrincipal:     declaringPrincipal,
			},
			// DomainRef is the canonical traceability link from this infrastructure
			// ILI to the abstract DomainLineageIndex at core.ontai.dev. All Seam
			// infrastructure ILIs trace to this single domain root. CLAUDE.md §14 Decision 2.
			DomainRef:           InfrastructureDomainRef,
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
// A fresh GET is issued before building the patch to avoid overwriting conditions
// set by other reconcilers (e.g. RBACPolicyValid) between the initial reconcile
// Get and this status write. Without the re-fetch, a JSON Merge Patch on a stale
// conditions array would silently discard conditions written by concurrent reconcilers.
func (r *LineageReconciler) ensureLineageSyncedTrue(ctx context.Context, root *unstructured.Unstructured, iliName string) error {
	// Re-fetch the root declaration to obtain the latest conditions. Other
	// reconcilers (e.g. RBACPolicyReconciler) may have written conditions after
	// the initial Get in Reconcile(). Using stale data here would cause those
	// conditions to be lost when we patch the conditions array.
	fresh := &unstructured.Unstructured{}
	fresh.SetGroupVersionKind(root.GroupVersionKind())
	if err := r.Client.Get(ctx, client.ObjectKey{Name: root.GetName(), Namespace: root.GetNamespace()}, fresh); err != nil {
		return fmt.Errorf("re-fetch root for LineageSynced patch: %w", err)
	}

	// Read current conditions from status.
	rawConditions, _, _ := unstructured.NestedSlice(fresh.Object, "status", "conditions")

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
		"observedGeneration": fresh.GetGeneration(),
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

	patchBase := fresh.DeepCopyObject().(client.Object)
	if err := unstructured.SetNestedSlice(fresh.Object, updated, "status", "conditions"); err != nil {
		return fmt.Errorf("failed to set conditions in unstructured: %w", err)
	}
	return r.Client.Status().Patch(ctx, fresh, client.MergeFrom(patchBase))
}

// defaultDescendantRetentionDays is the default number of days a stale descendant
// entry is retained after the referenced object is confirmed deleted.
const defaultDescendantRetentionDays = 30

// pruneStaleDescendants inspects each entry in the ILI DescendantRegistry. If the
// entry's referenced object is not-found in the API server AND the entry's CreatedAt
// timestamp is older than the effective retention window, the entry is removed from
// the registry. The registry is patched only if at least one entry was pruned.
//
// A nil CreatedAt timestamp means the entry predates retention tracking — the entry
// is never pruned to preserve backward compatibility.
func (r *LineageReconciler) pruneStaleDescendants(ctx context.Context, ili *seamv1alpha1.LineageRecord) error {
	if len(ili.Spec.DescendantRegistry) == 0 {
		return nil
	}

	retentionDays := int32(defaultDescendantRetentionDays)
	if ili.Spec.RetentionPolicy != nil && ili.Spec.RetentionPolicy.DescendantRetentionDays > 0 {
		retentionDays = ili.Spec.RetentionPolicy.DescendantRetentionDays
	}
	retentionWindow := time.Duration(retentionDays) * 24 * time.Hour

	logger := log.FromContext(ctx).WithValues("ili", ili.Name, "namespace", ili.Namespace)

	var kept []seamv1alpha1.DescendantEntry
	pruned := false

	for _, entry := range ili.Spec.DescendantRegistry {
		// Entries without CreatedAt predate retention tracking — always keep.
		if entry.CreatedAt == nil {
			kept = append(kept, entry)
			continue
		}
		// Keep entries within the retention window regardless of object existence.
		if time.Since(entry.CreatedAt.Time) < retentionWindow {
			kept = append(kept, entry)
			continue
		}
		// Retention window elapsed — do a non-blocking existence check.
		obj := &unstructured.Unstructured{}
		obj.SetGroupVersionKind(schema.GroupVersionKind{
			Group:   entry.Group,
			Version: entry.Version,
			Kind:    entry.Kind,
		})
		err := r.Client.Get(ctx, client.ObjectKey{Name: entry.Name, Namespace: entry.Namespace}, obj)
		if err == nil {
			// Object still exists — keep the entry.
			kept = append(kept, entry)
			continue
		}
		if !apierrors.IsNotFound(err) {
			// API server error — keep conservatively, log the issue.
			logger.Error(err, "existence check failed for descendant — keeping entry",
				"kind", entry.Kind, "name", entry.Name, "namespace", entry.Namespace)
			kept = append(kept, entry)
			continue
		}
		// Object is not-found and retention window elapsed — prune.
		logger.Info("pruning stale descendant entry",
			"kind", entry.Kind, "name", entry.Name, "namespace", entry.Namespace,
			"createdAt", entry.CreatedAt, "retentionDays", retentionDays)
		pruned = true
	}

	if !pruned {
		return nil
	}

	// Apply the pruned registry as a patch.
	patch := client.MergeFrom(ili.DeepCopy())
	ili.Spec.DescendantRegistry = kept
	return r.Client.Patch(ctx, ili, patch)
}

// ensureOwnerReferenceIfDeleteWithRoot adds an ownerReference from the ILI to the
// root declaration when the effective RetentionPolicy.DeleteWithRoot is true. This
// causes Kubernetes garbage collection to cascade deletion of the ILI when the root
// declaration is deleted.
//
// The ownerReference is idempotent — if it is already set, no patch is issued.
func (r *LineageReconciler) ensureOwnerReferenceIfDeleteWithRoot(ctx context.Context, root *unstructured.Unstructured, ili *seamv1alpha1.LineageRecord) error {
	deleteWithRoot := true // default per RetentionPolicy
	if ili.Spec.RetentionPolicy != nil {
		deleteWithRoot = ili.Spec.RetentionPolicy.DeleteWithRoot
	}
	if !deleteWithRoot {
		return nil
	}

	// Check if ownerReference already points to this root declaration.
	rootUID := root.GetUID()
	for _, ref := range ili.GetOwnerReferences() {
		if ref.UID == rootUID {
			return nil // already set
		}
	}

	// Construct the ownerReference. The root declaration is in the same namespace as
	// the ILI — owner references are valid for same-namespace resources.
	blockOwnerDeletion := true
	ownerRef := metav1.OwnerReference{
		APIVersion:         r.GVK.GroupVersion().String(),
		Kind:               r.GVK.Kind,
		Name:               root.GetName(),
		UID:                rootUID,
		BlockOwnerDeletion: &blockOwnerDeletion,
	}

	patch := client.MergeFrom(ili.DeepCopy())
	ili.SetOwnerReferences(append(ili.GetOwnerReferences(), ownerRef))
	return r.Client.Patch(ctx, ili, patch)
}

// CRDNameForGVK returns the CRD object name for a GVK using the standard plural convention:
//   - lowercase kind
//   - trailing 'y' replaced with 'ies'
//   - 's' appended otherwise
//   - suffixed with "." + group
func CRDNameForGVK(gvk schema.GroupVersionKind) string {
	kind := strings.ToLower(gvk.Kind)
	var plural string
	switch {
	case strings.HasSuffix(kind, "y"):
		plural = kind[:len(kind)-1] + "ies"
	case strings.HasSuffix(kind, "s"):
		plural = kind + "es"
	default:
		plural = kind + "s"
	}
	return plural + "." + gvk.Group
}

// IsRootDeclaration reports whether the CRD for gvk carries the lineage-root label
// (infrastructure.ontai.dev/lineage-root="true"). Returns false (not an error) when
// the CRD object is not found in the API server.
func IsRootDeclaration(ctx context.Context, c client.Client, gvk schema.GroupVersionKind) (bool, error) {
	crdName := CRDNameForGVK(gvk)
	crd := &unstructured.Unstructured{}
	crd.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "apiextensions.k8s.io",
		Version: "v1",
		Kind:    "CustomResourceDefinition",
	})
	if err := c.Get(ctx, client.ObjectKey{Name: crdName}, crd); err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("get CRD %s: %w", crdName, err)
	}
	labels, _, _ := unstructured.NestedStringMap(crd.Object, "metadata", "labels")
	return labels[lineagepkg.LabelLineageRoot] == "true", nil
}

// ResolveRootOwner walks ownerReferences from obj up to maxHops to find an owner
// whose GVK CRD carries the lineage-root label. Returns nil, nil when no root
// owner is found within maxHops.
func ResolveRootOwner(ctx context.Context, c client.Client, obj *unstructured.Unstructured, maxHops int) (*unstructured.Unstructured, error) {
	current := obj
	for hop := 0; hop < maxHops; hop++ {
		refs := current.GetOwnerReferences()
		if len(refs) == 0 {
			return nil, nil
		}
		// Prefer the controller ownerRef; fall back to the first ref.
		ref := refs[0]
		for _, r := range refs {
			if r.Controller != nil && *r.Controller {
				ref = r
				break
			}
		}
		gv, err := schema.ParseGroupVersion(ref.APIVersion)
		if err != nil {
			return nil, fmt.Errorf("parse ownerRef APIVersion %q: %w", ref.APIVersion, err)
		}
		ownerGVK := gv.WithKind(ref.Kind)

		isRoot, err := IsRootDeclaration(ctx, c, ownerGVK)
		if err != nil {
			return nil, fmt.Errorf("CRD check for owner %v: %w", ownerGVK, err)
		}

		owner := &unstructured.Unstructured{}
		owner.SetGroupVersionKind(ownerGVK)
		if err := c.Get(ctx, client.ObjectKey{Name: ref.Name, Namespace: current.GetNamespace()}, owner); err != nil {
			if apierrors.IsNotFound(err) {
				return nil, nil
			}
			return nil, fmt.Errorf("get owner %s %s: %w", ref.Kind, ref.Name, err)
		}

		if isRoot {
			return owner, nil
		}
		current = owner
	}
	return nil, nil
}

// handleDerivedObject is called when the reconciled GVK is not a lineage root.
// It walks ownerReferences to find the nearest root and appends a DescendantEntry
// to that root's LineageRecord. If no root is found within 3 hops, a warning is
// logged and the object is requeued.
func (r *LineageReconciler) handleDerivedObject(ctx context.Context, obj *unstructured.Unstructured) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithValues("gvk", r.GVK.String(), "name", obj.GetName())

	root, err := ResolveRootOwner(ctx, r.Client, obj, 3)
	if err != nil {
		logger.Error(err, "ownerRef walk failed — requeuing")
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}
	if root == nil {
		logger.Info("no lineage-root owner found within 3 ownerRef hops — requeuing",
			"namespace", obj.GetNamespace())
		return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
	}

	iliName := lineageIndexName(root.GroupVersionKind().Kind, root.GetName())
	ili := &seamv1alpha1.LineageRecord{}
	if err := r.Client.Get(ctx, client.ObjectKey{Name: iliName, Namespace: root.GetNamespace()}, ili); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
		}
		return ctrl.Result{}, fmt.Errorf("get LineageRecord %s: %w", iliName, err)
	}

	// Idempotency: skip if entry for this UID already exists.
	objUID := obj.GetUID()
	for _, entry := range ili.Spec.DescendantRegistry {
		if entry.UID == objUID {
			return ctrl.Result{}, nil
		}
	}

	labels := obj.GetLabels()
	seamOperator := labels["infrastructure.ontai.dev/seam-operator"]
	if seamOperator == "" {
		seamOperator = "unknown"
	}
	rationaleStr := labels["infrastructure.ontai.dev/creation-rationale"]
	if rationaleStr == "" {
		rationaleStr = string(lineagepkg.ClusterProvision)
	}
	now := metav1.Now()
	entry := seamv1alpha1.DescendantEntry{
		Group:                    r.GVK.Group,
		Version:                  r.GVK.Version,
		Kind:                     r.GVK.Kind,
		Name:                     obj.GetName(),
		Namespace:                obj.GetNamespace(),
		UID:                      objUID,
		SeamOperator:             seamOperator,
		CreationRationale:        lineagepkg.CreationRationale(rationaleStr),
		RootGenerationAtCreation: root.GetGeneration(),
		CreatedAt:                &now,
	}

	patch := client.MergeFrom(ili.DeepCopy())
	ili.Spec.DescendantRegistry = append(ili.Spec.DescendantRegistry, entry)
	logger.Info("appended DescendantEntry to LineageRecord",
		"iliName", iliName, "rootKind", root.GroupVersionKind().Kind, "rootName", root.GetName())
	return ctrl.Result{}, r.Client.Patch(ctx, ili, patch)
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
		Named("lineage-" + strings.ToLower(r.GVK.Kind)).
		For(u).
		Complete(r)
}
