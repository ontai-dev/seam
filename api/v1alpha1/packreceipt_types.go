package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// InfrastructurePackReceiptSpec defines the desired state of an InfrastructurePackReceipt.
// Written by conductor agent on the tenant cluster after verifying the signed PackInstance.
// INV-026. conductor-schema.md.
type InfrastructurePackReceiptSpec struct {
	// ClusterPackRef is the name of the ClusterPack CR this receipt acknowledges.
	ClusterPackRef string `json:"clusterPackRef"`

	// TargetClusterRef is the name of the cluster this receipt was generated on.
	TargetClusterRef string `json:"targetClusterRef"`

	// PackSignature is the base64-encoded Ed25519 signature from the management cluster conductor.
	// INV-026.
	// +optional
	PackSignature string `json:"packSignature,omitempty"`

	// SignatureVerified indicates whether the conductor agent verified the pack signature.
	// +optional
	SignatureVerified bool `json:"signatureVerified,omitempty"`

	// RBACDigest is the OCI digest of the RBAC layer. Carried from ClusterPack for audit.
	// +optional
	RBACDigest string `json:"rbacDigest,omitempty"`

	// WorkloadDigest is the OCI digest of the workload layer. Carried from ClusterPack.
	// +optional
	WorkloadDigest string `json:"workloadDigest,omitempty"`

	// ChartVersion is the Helm chart version. Carried from ClusterPack.
	// +optional
	ChartVersion string `json:"chartVersion,omitempty"`

	// ChartURL is the Helm chart repository URL. Carried from ClusterPack.
	// +optional
	ChartURL string `json:"chartURL,omitempty"`

	// ChartName is the Helm chart name. Carried from ClusterPack.
	// +optional
	ChartName string `json:"chartName,omitempty"`

	// HelmVersion is the Helm SDK version. Carried from ClusterPack.
	// +optional
	HelmVersion string `json:"helmVersion,omitempty"`
}

// InfrastructurePackReceiptStatus is the observed state of an InfrastructurePackReceipt.
type InfrastructurePackReceiptStatus struct {
	// ObservedGeneration is the generation most recently reconciled.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Conditions is the list of status conditions for this PackReceipt.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,shortName=ipr
// +kubebuilder:printcolumn:name="Pack",type=string,JSONPath=".spec.clusterPackRef"
// +kubebuilder:printcolumn:name="Verified",type=boolean,JSONPath=".spec.signatureVerified"
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=".metadata.creationTimestamp"

// InfrastructurePackReceipt is the seam-core CRD for pack delivery acknowledgement on a tenant cluster.
// Written by conductor agent after signature verification. INV-026.
type InfrastructurePackReceipt struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   InfrastructurePackReceiptSpec   `json:"spec,omitempty"`
	Status InfrastructurePackReceiptStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// InfrastructurePackReceiptList contains a list of InfrastructurePackReceipt.
type InfrastructurePackReceiptList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []InfrastructurePackReceipt `json:"items"`
}

func init() {
	SchemeBuilder.Register(&InfrastructurePackReceipt{}, &InfrastructurePackReceiptList{})
}
