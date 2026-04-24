package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/ontai-dev/seam-core/pkg/lineage"
)

// InfrastructurePackExecutionSpec defines the desired state of an InfrastructurePackExecution.
// wrapper-schema.md §3.
type InfrastructurePackExecutionSpec struct {
	// ClusterPackRef is the name of the ClusterPack CR to execute.
	ClusterPackRef string `json:"clusterPackRef"`

	// TargetClusterRef is the name of the target cluster to deliver the pack to.
	TargetClusterRef string `json:"targetClusterRef"`

	// AdmissionProfileRef is the name of the RBACProfile governing this execution.
	// +optional
	AdmissionProfileRef string `json:"admissionProfileRef,omitempty"`

	// ChartVersion is the Helm chart version for this execution. Carried from ClusterPack.
	// +optional
	ChartVersion string `json:"chartVersion,omitempty"`

	// ChartURL is the Helm chart repository URL. Carried from ClusterPack.
	// +optional
	ChartURL string `json:"chartURL,omitempty"`

	// ChartName is the Helm chart name. Carried from ClusterPack.
	// +optional
	ChartName string `json:"chartName,omitempty"`

	// HelmVersion is the Helm SDK version used to render the pack. Carried from ClusterPack.
	// +optional
	HelmVersion string `json:"helmVersion,omitempty"`

	// Lineage is the sealed causal chain record for this root declaration. Immutable after creation.
	// +optional
	Lineage *lineage.SealedCausalChain `json:"lineage,omitempty"`
}

// InfrastructurePackExecutionStatus is the observed state of an InfrastructurePackExecution.
type InfrastructurePackExecutionStatus struct {
	// ObservedGeneration is the generation most recently reconciled.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Conditions is the list of status conditions for this PackExecution.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,shortName=ipe
// +kubebuilder:printcolumn:name="Pack",type=string,JSONPath=".spec.clusterPackRef"
// +kubebuilder:printcolumn:name="Target",type=string,JSONPath=".spec.targetClusterRef"
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=".metadata.creationTimestamp"

// InfrastructurePackExecution is the seam-core CRD for a runtime pack delivery request.
// wrapper-schema.md §3.
type InfrastructurePackExecution struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   InfrastructurePackExecutionSpec   `json:"spec,omitempty"`
	Status InfrastructurePackExecutionStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// InfrastructurePackExecutionList contains a list of InfrastructurePackExecution.
type InfrastructurePackExecutionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []InfrastructurePackExecution `json:"items"`
}

func init() {
	SchemeBuilder.Register(&InfrastructurePackExecution{}, &InfrastructurePackExecutionList{})
}
