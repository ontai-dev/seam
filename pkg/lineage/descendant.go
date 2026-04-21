package lineage

import (
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// LabelRootILINamespace is the optional label key that carries the namespace
// of the InfrastructureLineageIndex when it differs from the derived object's
// namespace. DescendantReconciler reads this label and uses it for the ILI
// fetch, defaulting to the derived object's own namespace when absent.
// Required when the derived object lives in a different namespace than its
// root declaration (e.g. RunnerConfig in ont-system, TalosCluster ILI in seam-system).
const LabelRootILINamespace = "infrastructure.ontai.dev/root-ili-namespace"

// LabelActorRef is the label key that carries the declaring principal identity
// propagated from the root declaration's declaring-principal annotation. Set by
// SetDescendantLabels at derived object creation time. Read by DescendantReconciler
// when populating DescendantEntry.ActorRef. seam-core-schema.md §7 Declaration 6.
const LabelActorRef = "infrastructure.ontai.dev/actor-ref"

// AnnotationDeclaringPrincipal is the annotation key stamped by the admission
// webhook at CREATE time on root declaration CRDs. Operators read this annotation
// from the root object and pass it as actorRef to SetDescendantLabels.
const AnnotationDeclaringPrincipal = "infrastructure.ontai.dev/declaring-principal"

// IndexName returns the deterministic InfrastructureLineageIndex name for a
// given root declaration kind and instance name. Format: {lowercasekind}-{name}.
// This mirrors the private lineageIndexName function in the LineageController so
// operators can compute the correct ILI reference without importing internal packages.
// seam-core-schema.md §3.
func IndexName(kind, name string) string {
	return strings.ToLower(kind) + "-" + name
}

// SetDescendantLabels writes the five label keys required by the
// DescendantReconciler onto a derived object. Operators call this at derived
// object creation time so the LineageController can append the object to the
// referenced ILI's DescendantRegistry.
//
// iliName is the name of the InfrastructureLineageIndex (e.g., "taloscluster-prod").
// iliNamespace is the namespace containing the ILI. When the derived object and
// the ILI share a namespace this equals the derived object's namespace. When they
// differ (e.g. RunnerConfig in ont-system, ILI in seam-system) pass the ILI
// namespace explicitly so DescendantReconciler can resolve the cross-namespace ref.
// operator is the canonical Seam Operator name (e.g., "platform", "wrapper").
// rationale is drawn from the CreationRationale controlled vocabulary.
// actorRef is the declaring principal propagated from the root declaration's
// AnnotationDeclaringPrincipal annotation. Pass empty string when the annotation
// is absent (pre-amendment objects or bootstrap window). seam-core-schema.md §7 Declaration 6.
func SetDescendantLabels(obj metav1.Object, iliName, iliNamespace, operator string, rationale CreationRationale, actorRef string) {
	labels := obj.GetLabels()
	if labels == nil {
		labels = make(map[string]string)
	}
	labels["infrastructure.ontai.dev/root-ili"] = iliName
	labels[LabelRootILINamespace] = iliNamespace
	labels["infrastructure.ontai.dev/seam-operator"] = operator
	labels["infrastructure.ontai.dev/creation-rationale"] = string(rationale)
	labels[LabelActorRef] = actorRef
	obj.SetLabels(labels)
}
