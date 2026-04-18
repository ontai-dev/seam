package conditions

import (
	"fmt"
	"sort"
	"strings"
)

// vocabulary maps each condition type string to the set of valid reason strings
// for that type. Built from the constants declared in conditions.go; covers all
// five Seam operators. Used by ValidateCondition.
//
// For condition types that share a common string value across multiple CRDs
// (e.g. "Degraded", "Ready", "Running", "Pending"), the set is the union of all
// valid reasons across every operator that uses that string.
var vocabulary = map[string]map[string]struct{}{
	// ── Cross-operator ──────────────────────────────────────────────────────
	ConditionTypeLineageSynced: reasons(
		ReasonLineageControllerAbsent,
	),

	// ── Guardian ─────────────────────────────────────────────────────────────
	ConditionTypeBootstrapLabelAbsent: reasons(
		ReasonLabelAbsent,
		ReasonLabelPresent,
	),
	ConditionTypeIdentityBindingTrustAnchorResolved: reasons(
		ReasonTrustAnchorResolved,
		ReasonTrustAnchorNotFound,
		ReasonTrustAnchorInvalid,
		ReasonTrustAnchorTypeMismatch,
		ReasonTrustMethodMismatch,
	),
	ConditionTypeIdentityBindingValid: reasons(
		ReasonIdentityBindingValid,
		ReasonIdentityBindingInvalid,
		ReasonPermissionSetMissing,
		ReasonPermissionSetNotFound,
		ReasonTokenTTLExceeded,
	),
	ConditionTypeIdentityProviderReachable: reasons(
		ReasonIdentityProviderReachable,
		ReasonIdentityProviderUnreachable,
		ReasonIdentityProviderPending,
	),
	ConditionTypeIdentityProviderValid: reasons(
		ReasonIdentityProviderValid,
		ReasonIdentityProviderInvalid,
	),
	ConditionTypePermissionSetValid: reasons(
		ReasonPermissionSetValid,
		ReasonPermissionSetInvalid,
		ReasonPermissionSetNotFound,
	),
	ConditionTypeRBACPolicyDegraded: reasons(
		ReasonValidationFailed,
		ReasonPolicyViolation,
		ReasonStructureInvalid,
		ReasonPolicyNotFound,
		ReasonEPGPending,
	),
	ConditionTypeRBACPolicyValid: reasons(
		ReasonValidationPassed,
		ReasonValidationFailed,
	),
	ConditionTypeRBACProfilePolicyCompliant: reasons(
		ReasonBootstrapProfilesReady,
		ReasonBootstrapProfilesPending,
	),
	ConditionTypeRBACProfileProvisioned: reasons(
		ReasonProvisioningComplete,
		ReasonProvisioningFailed,
	),
	ConditionTypeRBACProfileValidated: reasons(
		ReasonValidationPassed,
		ReasonValidationFailed,
	),
	ConditionTypeWebhookRegistered: reasons(
		ReasonWebhookRegistered,
	),

	// ── Guardian — PermissionSnapshot ─────────────────────────────────────────
	ConditionTypePermissionSnapshotFresh: reasons(
		ReasonSnapshotFresh,
		ReasonSnapshotStale,
	),
	ConditionTypePermissionSnapshotCompliant: reasons(
		ReasonDriftAbsent,
		ReasonDriftDetected,
	),

	// ── Platform — TalosCluster and shared day-2 operation types ────────────
	//
	// "Ready" is a union of all operators and CRDs that use this type string.
	ConditionTypeReady: reasons(
		ReasonClusterReady,
		ReasonCAPIClusterRunning,
		ReasonJobComplete,
		ReasonResetComplete,
		ReasonAllControlPlaneMachinesReady,
		// PackInstance (wrapper)
		ReasonPackReceiptReady,
		ReasonPackReceiptNotFound,
		ReasonSignatureVerifyFailed,
		ReasonDependencyDrifted,
		ReasonDriftDetected,
	),
	// "Degraded" is a union of all platform CRDs that use this type string.
	ConditionTypeDegraded: reasons(
		ReasonDegraded,
		ReasonBootstrapJobFailed,
		ReasonConductorJobGateBlocked,
		ReasonJobFailed,
		ReasonCapabilityUnknown,
		ReasonReconcilerNotImplemented,
		ReasonS3DestinationAbsent,
	),
	// "Running" is used by EtcdMaintenance (platform) and PackExecution (wrapper).
	ConditionTypeRunning: reasons(
		ReasonJobSubmitted,
		ReasonJobFailed,
		ReasonJobComplete,
		ReasonJobSucceeded,
	),
	// "Pending" is used by MaintenanceBundle (platform) and PackExecution (wrapper).
	ConditionTypePending: reasons(
		ReasonPending,
		ReasonJobSubmitted,
		ReasonJobFailed,
		ReasonJobComplete,
		ReasonCapabilityUnknown,
		ReasonReconcilerNotImplemented,
		ReasonGatesClearing,
		ReasonAwaitingSignature,
		ReasonAwaitingConductorReady,
	),
	ConditionTypeBootstrapping: reasons(
		ReasonBootstrapJobSubmitted,
		ReasonBootstrapJobComplete,
		ReasonBootstrapJobFailed,
		ReasonCAPIObjectsCreated,
		ReasonCAPIClusterRunning,
	),
	ConditionTypeImporting: reasons(
		ReasonImportComplete,
	),
	ConditionTypeCiliumPending: reasons(
		ReasonCiliumPackPending,
		ReasonCiliumPackReady,
	),
	ConditionTypeControlPlaneUnreachable: reasons(
		ReasonControlPlaneNodeUnreachable,
	),
	ConditionTypePartialWorkerAvailability: reasons(
		ReasonWorkerNodeUnreachable,
	),
	ConditionTypeConductorReady: reasons(
		ReasonConductorDeploymentAvailable,
		ReasonConductorDeploymentUnavailable,
	),

	// ── Platform — ClusterMaintenance ────────────────────────────────────────
	ConditionTypeClusterMaintenancePaused: reasons(
		ReasonCAPIPaused,
		ReasonCAPIResumed,
		ReasonConductorJobGateBlocked,
	),
	ConditionTypeClusterMaintenanceWindowActive: reasons(
		ReasonMaintenanceWindowOpen,
		ReasonMaintenanceWindowClosed,
	),

	// ── Platform — Day-2 specific ─────────────────────────────────────────────
	ConditionTypeNodeOperationCAPIDelegated: reasons(
		ReasonNodeOpCAPIDelegated,
	),
	ConditionTypeResetPendingApproval: reasons(
		ReasonApprovalRequired,
	),
	EtcdBackupDestinationAbsent: reasons(
		ReasonS3DestinationAbsent,
	),

	// ── Platform — CAPI Infrastructure Provider ───────────────────────────────
	ConditionTypeInfrastructureReady: reasons(
		ReasonAllControlPlaneMachinesReady,
		ReasonControlPlaneMachinesNotReady,
		ReasonControlPlaneMachinesPending,
	),
	ConditionTypeMachineReady: reasons(
		ReasonMachineReady,
		ReasonMachineConfigApplied,
		ReasonMachineConfigFailed,
		ReasonBootstrapDataNotReady,
		ReasonCAPIMachineNotBound,
		ReasonMachineOutOfMaintenance,
	),
	ConditionTypePortReachable: reasons(
		ReasonPortUnreachable,
	),

	// ── Wrapper — ClusterPack ────────────────────────────────────────────────
	ConditionTypeClusterPackAvailable: reasons(
		ReasonPackAvailable,
		ReasonPackSignaturePending,
		ReasonPackSigned,
	),
	ConditionTypeClusterPackImmutabilityViolation: reasons(
		ReasonImmutabilityViolation,
	),
	ConditionTypeClusterPackRevoked: reasons(
		ReasonPackRevoked,
	),
	ConditionTypeClusterPackSignaturePending: reasons(
		ReasonPackSignaturePending,
		ReasonPackSigned,
	),

	// ── Wrapper — PackExecution ───────────────────────────────────────────────
	ConditionTypePackExecutionWaiting: reasons(
		ReasonAwaitingConductorReady,
	),
	ConditionTypePackSignaturePending: reasons(
		ReasonAwaitingSignature,
		ReasonPackSigned,
	),
	ConditionTypePackRevoked: reasons(
		ReasonClusterPackRevoked,
	),
	ConditionTypePermissionSnapshotOutOfSync: reasons(
		ReasonSnapshotOutOfSync,
	),
	ConditionTypeRBACProfileNotProvisioned: reasons(
		ReasonRBACProfileNotReady,
	),
	ConditionTypePackExecutionFailed: reasons(
		ReasonJobFailed,
		ReasonOperationResultNotFound,
	),
	ConditionTypePackExecutionSucceeded: reasons(
		ReasonJobSucceeded,
	),

	// ── Wrapper — PackInstance ────────────────────────────────────────────────
	ConditionTypePackInstanceDependencyBlocked: reasons(
		ReasonDependencyDrifted,
	),
	ConditionTypePackInstanceDrifted: reasons(
		ReasonDriftDetected,
		ReasonNoDrift,
	),
	ConditionTypePackInstanceProgressing: reasons(
		ReasonPackDelivered,
		ReasonPackReceiptNotFound,
	),
	ConditionTypePackInstanceSecurityViolation: reasons(
		ReasonSignatureVerifyFailed,
		ReasonSecurityViolationCleared,
	),
}

// ValidateCondition returns an error if reason is not a declared valid reason for
// conditionType. Both conditionType and reason must be non-empty.
//
// The vocabulary covers all condition types and reason strings used across the
// five Seam operators (guardian, platform, wrapper, conductor, seam-core).
// Operator unit tests call this function to assert that every condition emission
// uses a valid vocabulary pairing, preventing vocabulary drift over time. Gap 31.
func ValidateCondition(conditionType, reason string) error {
	if conditionType == "" {
		return fmt.Errorf("conditions.ValidateCondition: conditionType must not be empty")
	}
	if reason == "" {
		return fmt.Errorf("conditions.ValidateCondition: reason must not be empty for conditionType %q", conditionType)
	}
	validReasons, ok := vocabulary[conditionType]
	if !ok {
		return fmt.Errorf("conditions.ValidateCondition: unknown condition type %q; not declared in the Seam condition vocabulary", conditionType)
	}
	if _, valid := validReasons[reason]; !valid {
		sorted := sortedKeys(validReasons)
		return fmt.Errorf("conditions.ValidateCondition: reason %q is not valid for condition type %q; valid reasons: [%s]",
			reason, conditionType, strings.Join(sorted, ", "))
	}
	return nil
}

// KnownConditionTypes returns the sorted list of all condition type strings
// registered in the vocabulary. Useful for inspection and test assertions.
func KnownConditionTypes() []string {
	types := make([]string, 0, len(vocabulary))
	for t := range vocabulary {
		types = append(types, t)
	}
	sort.Strings(types)
	return types
}

// ValidReasonsFor returns the sorted list of valid reason strings for the given
// condition type, or nil if the type is not in the vocabulary.
func ValidReasonsFor(conditionType string) []string {
	validReasons, ok := vocabulary[conditionType]
	if !ok {
		return nil
	}
	return sortedKeys(validReasons)
}

// reasons constructs a set of reason strings for use in the vocabulary map.
func reasons(rs ...string) map[string]struct{} {
	m := make(map[string]struct{}, len(rs))
	for _, r := range rs {
		m[r] = struct{}{}
	}
	return m
}

func sortedKeys(m map[string]struct{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
