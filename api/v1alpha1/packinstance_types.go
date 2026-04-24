package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// InfrastructurePackInstanceSpec defines the desired state of an InfrastructurePackInstance.
// wrapper-schema.md §3.
type InfrastructurePackInstanceSpec struct {
	// ClusterPackRef is the name of the ClusterPack CR this instance tracks.
	ClusterPackRef string `json:"clusterPackRef"`

	// Version is the pack version delivered to the target cluster.
	Version string `json:"version"`

	// TargetClusterRef is the name of the target cluster this instance is installed on.
	TargetClusterRef string `json:"targetClusterRef"`

	// DependsOn is the list of pack base names that must be Delivered before this instance.
	// +optional
	DependsOn []string `json:"dependsOn,omitempty"`

	// DependencyPolicy controls how dependency failures affect this instance.
	// +optional
	DependencyPolicy string `json:"dependencyPolicy,omitempty"`

	// ChartVersion is the Helm chart version for this instance. Carried from ClusterPack.
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
}

// InfrastructurePackInstanceStatus is the observed state of an InfrastructurePackInstance.
type InfrastructurePackInstanceStatus struct {
	// ObservedGeneration is the generation most recently reconciled.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Conditions is the list of status conditions for this PackInstance.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,shortName=ipi
// +kubebuilder:printcolumn:name="Pack",type=string,JSONPath=".spec.clusterPackRef"
// +kubebuilder:printcolumn:name="Target",type=string,JSONPath=".spec.targetClusterRef"
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=".metadata.creationTimestamp"

// InfrastructurePackInstance is the seam-core CRD recording the delivered state of a pack on a cluster.
// wrapper-schema.md §3.
type InfrastructurePackInstance struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   InfrastructurePackInstanceSpec   `json:"spec,omitempty"`
	Status InfrastructurePackInstanceStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// InfrastructurePackInstanceList contains a list of InfrastructurePackInstance.
type InfrastructurePackInstanceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []InfrastructurePackInstance `json:"items"`
}

func init() {
	SchemeBuilder.Register(&InfrastructurePackInstance{}, &InfrastructurePackInstanceList{})
}
