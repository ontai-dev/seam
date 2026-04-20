package lineage

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// SetDescendantLabels writes the three label keys required by the
// DescendantReconciler onto a derived object. Operators call this at derived
// object creation time so the LineageController can append the object to the
// referenced ILI's DescendantRegistry.
//
// iliName is the name of the InfrastructureLineageIndex in the same namespace as
// the derived object (e.g., "taloscluster-prod-cluster").
// operator is the canonical Seam Operator name (e.g., "platform", "wrapper").
// rationale is drawn from the CreationRationale controlled vocabulary.
func SetDescendantLabels(obj metav1.Object, iliName, operator string, rationale CreationRationale) {
	labels := obj.GetLabels()
	if labels == nil {
		labels = make(map[string]string)
	}
	labels["infrastructure.ontai.dev/root-ili"] = iliName
	labels["infrastructure.ontai.dev/seam-operator"] = operator
	labels["infrastructure.ontai.dev/creation-rationale"] = string(rationale)
	obj.SetLabels(labels)
}
