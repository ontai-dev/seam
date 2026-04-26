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

// TalosClusterOperationRecord is a single day-2 operation record within one
// talosVersion revision. Multiple records accumulate in the parent TCOR as
// operations are performed against the cluster.
type TalosClusterOperationRecord struct {
	// Capability is the conductor capability that produced this record.
	Capability string `json:"capability"`

	// JobRef is the Kubernetes Job name that produced this record.
	// The platform reconciler uses this to correlate the record with the Job it submitted.
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

// InfrastructureTalosClusterOperationResultSpec is the accumulated day-2 operation
// history for one cluster, scoped to the current talosVersion revision.
//
// One CR per cluster. Created by the platform operator when the cluster tenant
// namespace is provisioned. Named by the cluster name. Lives in seam-tenant-{clusterRef}.
//
// When the cluster talosVersion is upgraded, the current revision is archived to
// the GraphQuery DB and a new revision begins: Revision increments, TalosVersion
// is updated, and Operations is cleared.
//
// conductor-schema.md §8, seam-core-schema.md §TCOR.
type InfrastructureTalosClusterOperationResultSpec struct {
	// ClusterRef is the name of the InfrastructureTalosCluster this result accumulates.
	ClusterRef string `json:"clusterRef"`

	// TalosVersion is the cluster talosVersion for the current active revision.
	// Matches InfrastructureTalosCluster.spec.talosVersion at the time this revision began.
	TalosVersion string `json:"talosVersion"`

	// Revision is the monotonic revision counter. Starts at 1. Increments on each
	// talosVersion upgrade. Each revision holds the operations performed during that
	// version epoch. Archived revisions are stored in the GraphQuery DB.
	// +kubebuilder:default=1
	Revision int64 `json:"revision"`

	// Operations is the map of day-2 operation records for the current revision,
	// keyed by Kubernetes Job name (OPERATION_RESULT_CR). Map keying enables
	// O(1) lookup by the platform reconciler and clean serialization when
	// archiving the revision to the GraphQuery DB.
	// +optional
	Operations map[string]TalosClusterOperationRecord `json:"operations,omitempty"`
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
// +kubebuilder:printcolumn:name="Cluster",type=string,JSONPath=`.spec.clusterRef`
// +kubebuilder:printcolumn:name="TalosVersion",type=string,JSONPath=`.spec.talosVersion`
// +kubebuilder:printcolumn:name="Revision",type=integer,JSONPath=`.spec.revision`
// +kubebuilder:printcolumn:name="Ops",type=integer,JSONPath=`.spec.operations`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// InfrastructureTalosClusterOperationResult accumulates the day-2 operation history
// for one cluster. One CR per cluster, created when the platform operator provisions
// the cluster tenant namespace. Operations are appended by the Conductor execute-mode
// Job. On talosVersion upgrade, the current revision is archived to the GraphQuery DB
// and a new revision epoch begins.
//
// Named by the cluster name. Lives in seam-tenant-{clusterRef}.
// conductor-schema.md §8, seam-core-schema.md §TCOR.
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
