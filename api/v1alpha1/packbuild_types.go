package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// InfrastructurePackBuildCategory is the compilation category for a pack.
// +kubebuilder:validation:Enum=helm;kustomize;raw
type InfrastructurePackBuildCategory string

const (
	InfrastructurePackBuildCategoryHelm      InfrastructurePackBuildCategory = "helm"
	InfrastructurePackBuildCategoryKustomize InfrastructurePackBuildCategory = "kustomize"
	InfrastructurePackBuildCategoryRaw       InfrastructurePackBuildCategory = "raw"
)

// InfrastructurePackHelmSource describes a Helm chart source for pack compilation.
type InfrastructurePackHelmSource struct {
	// URL is the full URL to the Helm chart tarball (.tgz).
	URL string `json:"url"`

	// Chart is the chart name used for rendering context.
	Chart string `json:"chart"`

	// Version is the chart version string.
	Version string `json:"version"`

	// ValuesFile is the path to a YAML values file. Optional.
	// +optional
	ValuesFile string `json:"valuesFile,omitempty"`
}

// InfrastructurePackKustomizeSource describes a Kustomize source for pack compilation.
type InfrastructurePackKustomizeSource struct {
	// Path is the path to the kustomization root directory.
	Path string `json:"path"`
}

// InfrastructurePackRawSource describes a raw manifest source for pack compilation.
type InfrastructurePackRawSource struct {
	// Path is the path to the directory containing raw YAML manifests.
	Path string `json:"path"`
}

// InfrastructurePackBuildSpec defines the desired state of an InfrastructurePackBuild.
// Compiler input specification. Read by the Compiler at compile time; never applied to a cluster as a CR.
// conductor-schema.md §7.
type InfrastructurePackBuildSpec struct {
	// ComponentName is the name of the component being compiled.
	ComponentName string `json:"componentName"`

	// Category declares the compilation category. Must be one of: helm, kustomize, raw.
	// +kubebuilder:validation:Enum=helm;kustomize;raw
	Category InfrastructurePackBuildCategory `json:"category"`

	// HelmSource describes the Helm chart source. Required when category=helm.
	// +optional
	HelmSource *InfrastructurePackHelmSource `json:"helmSource,omitempty"`

	// KustomizeSource describes the Kustomize source. Required when category=kustomize.
	// +optional
	KustomizeSource *InfrastructurePackKustomizeSource `json:"kustomizeSource,omitempty"`

	// RawSource describes the raw manifest source. Required when category=raw.
	// +optional
	RawSource *InfrastructurePackRawSource `json:"rawSource,omitempty"`

	// TargetClusters is the list of cluster names to which the compiled pack should be delivered.
	// +optional
	TargetClusters []string `json:"targetClusters,omitempty"`
}

// InfrastructurePackBuildStatus is the observed state of an InfrastructurePackBuild.
type InfrastructurePackBuildStatus struct {
	// ObservedGeneration is the generation most recently reconciled.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Conditions is the list of status conditions for this PackBuild.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,shortName=ipb
// +kubebuilder:printcolumn:name="Category",type=string,JSONPath=".spec.category"
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=".metadata.creationTimestamp"

// InfrastructurePackBuild is the seam-core CRD for compiler input specification.
// Compiler reads this at compile time; never applied to a cluster as a live CR.
// conductor-schema.md §7.
type InfrastructurePackBuild struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   InfrastructurePackBuildSpec   `json:"spec,omitempty"`
	Status InfrastructurePackBuildStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// InfrastructurePackBuildList contains a list of InfrastructurePackBuild.
type InfrastructurePackBuildList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []InfrastructurePackBuild `json:"items"`
}

func init() {
	SchemeBuilder.Register(&InfrastructurePackBuild{}, &InfrastructurePackBuildList{})
}
