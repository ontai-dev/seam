package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TalosClusterResultStatus is the terminal status of a TalosCluster day-2 operation.
// +kubebuilder:validation:Enum=Succeeded;Failed
type TalosClusterResultStatus string

const (
	TalosClusterResultSucceeded TalosClusterResultStatus = "Succeeded"
	TalosClusterResultFailed    TalosClusterResultStatus = "Failed"
)

// TalosClusterOperationFailureReason is a structured failure description for
// a day-2 operation that reached a terminal Failed state.
type TalosClusterOperationFailureReason struct {
	// Category classifies the failure domain.
	// +kubebuilder:validation:Enum=ValidationFailure;CapabilityUnavailable;ExecutionFailure;ExternalDependencyFailure;InvariantViolation
	Category string `json:"category"`

	// Reason is a human-readable description of the failure.
	Reason string `json:"reason"`
}

// InfrastructureTalosClusterOperationResultSpec is the complete result
// written by a Conductor execute-mode Job after a day-2 TalosCluster operation.
// Immutable after creation. One CR per Job. conductor-schema.md §8.
type InfrastructureTalosClusterOperationResultSpec struct {
	// Capability is the conductor capability that produced this result.
	Capability string `json:"capability"`

	// ClusterRef is the name of the InfrastructureTalosCluster this operation targeted.
	ClusterRef string `json:"clusterRef"`

	// JobRef is the Kubernetes Job name that produced this result.
	JobRef string `json:"jobRef"`

	// Status is the terminal status of the capability execution.
	// +kubebuilder:validation:Enum=Succeeded;Failed
	Status TalosClusterResultStatus `json:"status"`

	// Message provides a human-readable summary of the outcome.
	// +optional
	Message string `json:"message,omitempty"`

	// StartedAt is the time the capability execution began.
	// +optional
	StartedAt *metav1.Time `json:"startedAt,omitempty"`

	// CompletedAt is the time the capability execution finished.
	// +optional
	CompletedAt *metav1.Time `json:"completedAt,omitempty"`

	// FailureReason is populated when Status is Failed. Nil on success.
	// +optional
	FailureReason *TalosClusterOperationFailureReason `json:"failureReason,omitempty"`
}

// InfrastructureTalosClusterOperationResultStatus is the observed state.
// Currently empty; reserved for future conditions.
type InfrastructureTalosClusterOperationResultStatus struct {
	// ObservedGeneration is the last generation observed by any consumer.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,shortName=tcor
// +kubebuilder:printcolumn:name="Capability",type=string,JSONPath=`.spec.capability`
// +kubebuilder:printcolumn:name="Status",type=string,JSONPath=`.spec.status`
// +kubebuilder:printcolumn:name="Cluster",type=string,JSONPath=`.spec.clusterRef`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// InfrastructureTalosClusterOperationResult is the immutable result record
// written by the Conductor execute-mode Job after a day-2 TalosCluster operation
// completes. One CR per Job, created in the Job's namespace (ont-system).
// Owned by the triggering platform day-2 CR via ownerReference for automatic GC.
// conductor-schema.md §8, seam-core-schema.md §TBD.
type InfrastructureTalosClusterOperationResult struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   InfrastructureTalosClusterOperationResultSpec   `json:"spec,omitempty"`
	Status InfrastructureTalosClusterOperationResultStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// InfrastructureTalosClusterOperationResultList contains a list of results.
type InfrastructureTalosClusterOperationResultList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []InfrastructureTalosClusterOperationResult `json:"items"`
}

func init() {
	SchemeBuilder.Register(
		&InfrastructureTalosClusterOperationResult{},
		&InfrastructureTalosClusterOperationResultList{},
	)
}
