package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DriftSignalState is the three-state acknowledgement enum for the drift signal chain.
// Decision I. conductor-schema.md.
// +kubebuilder:validation:Enum=pending;delivered;queued;confirmed
type DriftSignalState string

const (
	// DriftSignalStatePending is the initial state. The signal has been written by conductor
	// role=tenant but has not yet been delivered to the management cluster.
	DriftSignalStatePending DriftSignalState = "pending"

	// DriftSignalStateDelivered indicates the federation channel has transmitted the signal
	// to the management cluster conductor. At-least-once delivery guarantee applies.
	DriftSignalStateDelivered DriftSignalState = "delivered"

	// DriftSignalStateQueued indicates the management cluster conductor has accepted the signal
	// and enqueued a corrective job. The job has not yet completed.
	DriftSignalStateQueued DriftSignalState = "queued"

	// DriftSignalStateConfirmed indicates the corrective job completed and the management cluster
	// conductor has acknowledged resolution. Terminal state.
	DriftSignalStateConfirmed DriftSignalState = "confirmed"
)

// DriftAffectedCRRef is a typed reference to the CR that exhibited drift.
type DriftAffectedCRRef struct {
	// Group is the API group of the drifted CR.
	Group string `json:"group"`

	// Kind is the Kind of the drifted CR.
	Kind string `json:"kind"`

	// Namespace is the namespace of the drifted CR. Empty for cluster-scoped resources.
	// +optional
	Namespace string `json:"namespace,omitempty"`

	// Name is the name of the drifted CR.
	Name string `json:"name"`
}

// DriftSignalSpec defines the observed drift event written by conductor role=tenant.
// Decision I. conductor-schema.md.
type DriftSignalSpec struct {
	// State is the current acknowledgement state of this drift signal. Decision I.
	// +kubebuilder:validation:Enum=pending;delivered;queued;confirmed
	State DriftSignalState `json:"state"`

	// CorrelationID is a unique identifier for this drift event, used to deduplicate
	// signals across federation retries. Format: UUID v4.
	CorrelationID string `json:"correlationID"`

	// ObservedAt is the time the drift was first observed by conductor role=tenant.
	ObservedAt metav1.Time `json:"observedAt"`

	// AffectedCRRef is a typed reference to the CR that exhibited drift.
	AffectedCRRef DriftAffectedCRRef `json:"affectedCRRef"`

	// DriftReason is a human-readable description of why drift was detected.
	DriftReason string `json:"driftReason"`

	// CorrectionJobRef is the name of the corrective Job created by the management cluster.
	// Populated when state transitions to queued.
	// +optional
	CorrectionJobRef string `json:"correctionJobRef,omitempty"`

	// EscalationCounter is the number of times this signal has been re-emitted without
	// acknowledgement. Incremented by conductor role=tenant on each re-emit cycle.
	// When this counter reaches the configurable escalation threshold, conductor writes a
	// type=TerminalDrift Condition on the affected CR and stops re-emitting. Decision I.
	// +optional
	EscalationCounter int32 `json:"escalationCounter,omitempty"`
}

// DriftSignalStatus is the observed state of a DriftSignal. Written by conductor role=management.
type DriftSignalStatus struct {
	// ObservedGeneration is the generation most recently reconciled.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Conditions is the list of status conditions for this DriftSignal.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,shortName=ds
// +kubebuilder:printcolumn:name="State",type=string,JSONPath=".spec.state"
// +kubebuilder:printcolumn:name="CorrelationID",type=string,JSONPath=".spec.correlationID"
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=".metadata.creationTimestamp"

// DriftSignal is the seam-core CRD for the three-state drift acknowledgement chain.
// Written by conductor role=tenant; acknowledged by conductor role=management.
// Decision I. At-least-once delivery. Human-at-Boundary invariant enforced via
// configurable escalation threshold and TerminalDrift Condition. conductor-schema.md.
type DriftSignal struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DriftSignalSpec   `json:"spec,omitempty"`
	Status DriftSignalStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// DriftSignalList contains a list of DriftSignal.
type DriftSignalList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DriftSignal `json:"items"`
}

func init() {
	SchemeBuilder.Register(&DriftSignal{}, &DriftSignalList{})
}
