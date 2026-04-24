package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// RunnerPhaseConfig carries per-phase parameters for the runner's execution context.
type RunnerPhaseConfig struct {
	// Name identifies the phase.
	Name string `json:"name"`

	// Parameters holds phase-specific key-value configuration.
	// +optional
	Parameters map[string]string `json:"parameters,omitempty"`
}

// RunnerConfigStep declares one step in a multi-step operation intent.
type RunnerConfigStep struct {
	// Name is the unique identifier for this step within the RunnerConfig.
	Name string `json:"name"`

	// Capability is the named Conductor capability to invoke for this step.
	Capability string `json:"capability"`

	// Parameters is the input parameter map passed to the capability at Job materialisation time.
	// +optional
	Parameters map[string]string `json:"parameters,omitempty"`

	// DependsOn is the name of a prior step that must complete before this step begins.
	// +optional
	DependsOn string `json:"dependsOn,omitempty"`

	// HaltOnFailure controls sequencer behaviour when this step fails.
	// When true, failure terminates the RunnerConfig with no further steps executing.
	// +optional
	HaltOnFailure bool `json:"haltOnFailure,omitempty"`
}

// RunnerOperationalHistoryEntry is a single append-only audit record describing one
// configuration change applied to this RunnerConfig. Never truncated.
type RunnerOperationalHistoryEntry struct {
	// AppliedAt is the time this change was applied.
	AppliedAt metav1.Time `json:"appliedAt"`

	// Concern identifies what aspect of configuration changed.
	Concern string `json:"concern"`

	// PreviousValue is the value before the change. Empty for initial entries.
	// +optional
	PreviousValue string `json:"previousValue,omitempty"`

	// NewValue is the value after the change.
	NewValue string `json:"newValue"`

	// AppliedBy identifies who applied the change.
	AppliedBy string `json:"appliedBy"`
}

// RunnerCapabilityEntry is one capability declared by the Conductor agent on startup.
type RunnerCapabilityEntry struct {
	// Name is the capability name (e.g., pack-deploy, talos-upgrade).
	Name string `json:"name"`

	// Version is the capability version declared by the agent.
	Version string `json:"version"`

	// Description is a human-readable description of what this capability does.
	// +optional
	Description string `json:"description,omitempty"`
}

// RunnerStepResultPhase is the lifecycle phase of a RunnerConfig step result.
// +kubebuilder:validation:Enum=Succeeded;Failed;Skipped
type RunnerStepResultPhase string

const (
	RunnerStepSucceeded RunnerStepResultPhase = "Succeeded"
	RunnerStepFailed    RunnerStepResultPhase = "Failed"
	RunnerStepSkipped   RunnerStepResultPhase = "Skipped"
)

// RunnerConfigStepResult is the status record for one step.
type RunnerConfigStepResult struct {
	// Name matches the Name field of the corresponding RunnerConfigStep in spec.
	Name string `json:"name"`

	// Status is the terminal status of this step execution.
	// +kubebuilder:validation:Enum=Succeeded;Failed;Skipped
	Status RunnerStepResultPhase `json:"status"`

	// StartedAt is the time this step began execution.
	// +optional
	StartedAt *metav1.Time `json:"startedAt,omitempty"`

	// CompletedAt is the time this step finished execution.
	// +optional
	CompletedAt *metav1.Time `json:"completedAt,omitempty"`

	// Message is additional context about the step outcome.
	// +optional
	Message string `json:"message,omitempty"`
}

// InfrastructureRunnerConfigSpec is the operator-generated operational contract for a
// specific cluster. Generated at runtime by platform using the runner shared library.
// Never human-authored. INV-009, INV-010. conductor-schema.md.
type InfrastructureRunnerConfigSpec struct {
	// ClusterRef is the name of the TalosCluster this RunnerConfig is authoritative for.
	ClusterRef string `json:"clusterRef"`

	// RunnerImage is the fully qualified container image reference for the Conductor agent.
	// Tag convention: v{talosVersion}-r{revision} stable, dev/dev-rc{N} development. INV-011.
	RunnerImage string `json:"runnerImage"`

	// Phases is the ordered list of operational phases for this cluster's Conductor lifecycle.
	// +optional
	Phases []RunnerPhaseConfig `json:"phases,omitempty"`

	// Steps is the ordered list of execution steps across all phases.
	// +optional
	Steps []RunnerConfigStep `json:"steps,omitempty"`

	// OperationalHistory is an append-only record of completed RunnerConfig executions.
	// +optional
	OperationalHistory []RunnerOperationalHistoryEntry `json:"operationalHistory,omitempty"`

	// MaintenanceTargetNodes is the list of node names that are the subject of the operation.
	// +optional
	MaintenanceTargetNodes []string `json:"maintenanceTargetNodes,omitempty"`

	// OperatorLeaderNode is the node hosting the leader pod of the initiating operator.
	// +optional
	OperatorLeaderNode string `json:"operatorLeaderNode,omitempty"`

	// SelfOperation is true when the Job's execution cluster and the target cluster are the same.
	// +optional
	SelfOperation bool `json:"selfOperation,omitempty"`
}

// InfrastructureRunnerConfigStatus is written exclusively by the Conductor agent leader.
// CR-INV-006.
type InfrastructureRunnerConfigStatus struct {
	// Capabilities is the self-declared capability manifest emitted by the Conductor agent on startup.
	// CR-INV-005.
	// +optional
	Capabilities []RunnerCapabilityEntry `json:"capabilities,omitempty"`

	// AgentVersion is the version string of the Conductor agent binary currently running.
	// +optional
	AgentVersion string `json:"agentVersion,omitempty"`

	// AgentLeader is the pod name of the current Conductor agent leader.
	// +optional
	AgentLeader string `json:"agentLeader,omitempty"`

	// StepResults is the ordered list of step result records written by Conductor execute mode.
	// +optional
	StepResults []RunnerConfigStepResult `json:"stepResults,omitempty"`

	// Conditions is the standard Kubernetes condition list for this RunnerConfig.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,shortName=irc
// +kubebuilder:printcolumn:name="Cluster",type=string,JSONPath=".spec.clusterRef"
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=".metadata.creationTimestamp"

// InfrastructureRunnerConfig is the seam-core CRD for Conductor agent runtime configuration.
// Owned by seam-core; authored exclusively by the platform operator. INV-009.
// conductor-schema.md.
type InfrastructureRunnerConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   InfrastructureRunnerConfigSpec   `json:"spec,omitempty"`
	Status InfrastructureRunnerConfigStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// InfrastructureRunnerConfigList contains a list of InfrastructureRunnerConfig.
type InfrastructureRunnerConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []InfrastructureRunnerConfig `json:"items"`
}

func init() {
	SchemeBuilder.Register(&InfrastructureRunnerConfig{}, &InfrastructureRunnerConfigList{})
}
