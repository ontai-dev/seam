package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// InfrastructureDriftPolicy controls how dependency drift is handled.
// wrapper-schema.md §3 PackInstance spec.dependencyPolicy.onDrift.
type InfrastructureDriftPolicy string

const (
	// InfrastructureDriftPolicyBlock stops all further pack ops on this cluster
	// when a dependency PackInstance reports Drifted=True.
	InfrastructureDriftPolicyBlock InfrastructureDriftPolicy = "Block"

	// InfrastructureDriftPolicyWarn emits a warning event when a dependency
	// PackInstance reports Drifted=True but does not block further pack ops.
	InfrastructureDriftPolicyWarn InfrastructureDriftPolicy = "Warn"

	// InfrastructureDriftPolicyIgnore takes no action when a dependency
	// PackInstance reports Drifted=True.
	InfrastructureDriftPolicyIgnore InfrastructureDriftPolicy = "Ignore"
)

// InfrastructureDependencyPolicy defines behavior when a dependency PackInstance reports drift.
// wrapper-schema.md §3 PackInstance spec.dependencyPolicy.
type InfrastructureDependencyPolicy struct {
	// OnDrift controls how this PackInstance responds when a declared dependency
	// PackInstance reports Drifted=True.
	// +kubebuilder:validation:Enum=Block;Warn;Ignore
	// +kubebuilder:default=Warn
	OnDrift InfrastructureDriftPolicy `json:"onDrift,omitempty"`
}

// InfrastructureDeployedResourceRef records a single Kubernetes resource applied
// by the pack-deploy job. Used by the PackInstance deletion handler to clean up
// deployed workload when the ClusterPack is deleted. wrapper-schema.md §3.
type InfrastructureDeployedResourceRef struct {
	// APIVersion is the Kubernetes apiVersion (e.g., apps/v1, v1).
	APIVersion string `json:"apiVersion"`

	// Kind is the Kubernetes resource Kind (e.g., Deployment, Namespace).
	Kind string `json:"kind"`

	// Namespace is the resource namespace. Empty for cluster-scoped resources.
	// +optional
	Namespace string `json:"namespace,omitempty"`

	// Name is the resource name.
	Name string `json:"name"`
}

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

	// DependencyPolicy defines behavior when a dependency reports drift.
	// +optional
	DependencyPolicy *InfrastructureDependencyPolicy `json:"dependencyPolicy,omitempty"`

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

	// DeliveredAt records when the pack was most recently confirmed delivered.
	// +optional
	DeliveredAt *metav1.Time `json:"deliveredAt,omitempty"`

	// DriftSummary is a human-readable summary of the current drift state.
	// +optional
	DriftSummary string `json:"driftSummary,omitempty"`

	// UpgradeDirection records the version transition direction for the last deployment.
	// Initial: first deployment. Upgrade: newer version. Rollback: older version. Redeploy: same version.
	// +optional
	// +kubebuilder:validation:Enum=Initial;Upgrade;Rollback;Redeploy
	UpgradeDirection string `json:"upgradeDirection,omitempty"`

	// DeployedResources is the list of Kubernetes resources applied by the pack-deploy job.
	// Used by the PackInstance deletion handler for cleanup.
	// +optional
	DeployedResources []InfrastructureDeployedResourceRef `json:"deployedResources,omitempty"`

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
