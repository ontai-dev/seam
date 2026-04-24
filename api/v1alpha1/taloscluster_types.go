package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/ontai-dev/seam-core/pkg/lineage"
)

// InfrastructureTalosClusterMode declares whether the cluster is bootstrapped or imported.
// +kubebuilder:validation:Enum=bootstrap;import
type InfrastructureTalosClusterMode string

const (
	InfrastructureTalosClusterModeBootstrap InfrastructureTalosClusterMode = "bootstrap"
	InfrastructureTalosClusterModeImport    InfrastructureTalosClusterMode = "import"
)

// InfrastructureTalosClusterRole declares the role of the cluster in the Seam topology.
// Mandatory on mode=import.
// +kubebuilder:validation:Enum=management;tenant
type InfrastructureTalosClusterRole string

const (
	InfrastructureTalosClusterRoleManagement InfrastructureTalosClusterRole = "management"
	InfrastructureTalosClusterRoleTenant     InfrastructureTalosClusterRole = "tenant"
)

// InfrastructureCAPIConfig holds CAPI integration settings.
type InfrastructureCAPIConfig struct {
	// Enabled controls whether CAPI manages the cluster lifecycle.
	Enabled bool `json:"enabled"`
}

// InfrastructureTalosClusterSpec is the declared desired state of an InfrastructureTalosCluster.
// platform-schema.md §4.
type InfrastructureTalosClusterSpec struct {
	// Mode declares whether this cluster is bootstrapped from scratch or imported.
	// +kubebuilder:validation:Enum=bootstrap;import
	Mode InfrastructureTalosClusterMode `json:"mode"`

	// Role declares the cluster role in the Seam topology. Mandatory on mode=import.
	// +kubebuilder:validation:Enum=management;tenant
	// +optional
	Role InfrastructureTalosClusterRole `json:"role,omitempty"`

	// TalosVersion is the Talos OS version for this cluster. Used by Conductor to select
	// a compatible runner image. INV-012.
	// +optional
	TalosVersion string `json:"talosVersion,omitempty"`

	// CAPI holds CAPI integration settings. When absent, the cluster uses direct bootstrap.
	// +optional
	CAPI *InfrastructureCAPIConfig `json:"capi,omitempty"`

	// Endpoint is the API server endpoint for this cluster. Required on mode=import.
	// +optional
	Endpoint string `json:"endpoint,omitempty"`

	// KubeconfigSecretRef is the name of the Secret containing the kubeconfig for this cluster.
	// Required on mode=import.
	// +optional
	KubeconfigSecretRef string `json:"kubeconfigSecretRef,omitempty"`

	// TalosconfigSecretRef is the name of the Secret containing the talosconfig for this cluster.
	// +optional
	TalosconfigSecretRef string `json:"talosconfigSecretRef,omitempty"`

	// Lineage is the sealed causal chain record for this root declaration. Immutable after creation.
	// +optional
	Lineage *lineage.SealedCausalChain `json:"lineage,omitempty"`
}

// InfrastructureTalosClusterStatus is the observed state of an InfrastructureTalosCluster.
type InfrastructureTalosClusterStatus struct {
	// Origin records how this cluster came under Seam governance: "imported" or "bootstrapped".
	// +optional
	Origin string `json:"origin,omitempty"`

	// ObservedGeneration is the generation most recently reconciled.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Conditions is the list of status conditions for this TalosCluster.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,shortName=itc
// +kubebuilder:printcolumn:name="Mode",type=string,JSONPath=".spec.mode"
// +kubebuilder:printcolumn:name="Role",type=string,JSONPath=".spec.role"
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=".metadata.creationTimestamp"

// InfrastructureTalosCluster is the seam-core CRD for a Talos cluster under Seam governance.
// platform-schema.md §4. Decision H.
type InfrastructureTalosCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   InfrastructureTalosClusterSpec   `json:"spec,omitempty"`
	Status InfrastructureTalosClusterStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// InfrastructureTalosClusterList contains a list of InfrastructureTalosCluster.
type InfrastructureTalosClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []InfrastructureTalosCluster `json:"items"`
}

func init() {
	SchemeBuilder.Register(&InfrastructureTalosCluster{}, &InfrastructureTalosClusterList{})
}
