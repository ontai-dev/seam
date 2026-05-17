// Package conditions_test verifies the Seam platform condition vocabulary package.
// Gap 31 — pkg/conditions is the canonical source for all condition type and reason
// string constants across guardian, platform, wrapper, conductor, and seam-core.
package conditions_test

import (
	"strings"
	"testing"

	"github.com/ontai-dev/seam/pkg/conditions"
)

// TestValidateCondition_AcceptsKnownPairs verifies that every declared constant
// pairing in the vocabulary is accepted by ValidateCondition.
func TestValidateCondition_AcceptsKnownPairs(t *testing.T) {
	t.Parallel()

	knownPairs := []struct {
		condType string
		reason   string
	}{
		// Cross-operator
		{conditions.ConditionTypeLineageSynced, conditions.ReasonLineageControllerAbsent},

		// Guardian
		{conditions.ConditionTypeBootstrapLabelAbsent, conditions.ReasonLabelAbsent},
		{conditions.ConditionTypeBootstrapLabelAbsent, conditions.ReasonLabelPresent},
		{conditions.ConditionTypeIdentityBindingTrustAnchorResolved, conditions.ReasonTrustAnchorResolved},
		{conditions.ConditionTypeIdentityBindingTrustAnchorResolved, conditions.ReasonTrustAnchorNotFound},
		{conditions.ConditionTypeIdentityBindingTrustAnchorResolved, conditions.ReasonTrustAnchorInvalid},
		{conditions.ConditionTypeIdentityBindingTrustAnchorResolved, conditions.ReasonTrustAnchorTypeMismatch},
		{conditions.ConditionTypeIdentityBindingTrustAnchorResolved, conditions.ReasonTrustMethodMismatch},
		{conditions.ConditionTypeIdentityBindingValid, conditions.ReasonIdentityBindingValid},
		{conditions.ConditionTypeIdentityBindingValid, conditions.ReasonIdentityBindingInvalid},
		{conditions.ConditionTypeIdentityBindingValid, conditions.ReasonPermissionSetMissing},
		{conditions.ConditionTypeIdentityBindingValid, conditions.ReasonPermissionSetNotFound},
		{conditions.ConditionTypeIdentityBindingValid, conditions.ReasonTokenTTLExceeded},
		{conditions.ConditionTypeIdentityProviderReachable, conditions.ReasonIdentityProviderReachable},
		{conditions.ConditionTypeIdentityProviderReachable, conditions.ReasonIdentityProviderUnreachable},
		{conditions.ConditionTypeIdentityProviderReachable, conditions.ReasonIdentityProviderPending},
		{conditions.ConditionTypeIdentityProviderValid, conditions.ReasonIdentityProviderValid},
		{conditions.ConditionTypeIdentityProviderValid, conditions.ReasonIdentityProviderInvalid},
		{conditions.ConditionTypePermissionSetValid, conditions.ReasonPermissionSetValid},
		{conditions.ConditionTypePermissionSetValid, conditions.ReasonPermissionSetInvalid},
		{conditions.ConditionTypePermissionSetValid, conditions.ReasonPermissionSetNotFound},
		{conditions.ConditionTypeRBACPolicyDegraded, conditions.ReasonValidationFailed},
		{conditions.ConditionTypeRBACPolicyDegraded, conditions.ReasonPolicyViolation},
		{conditions.ConditionTypeRBACPolicyDegraded, conditions.ReasonStructureInvalid},
		{conditions.ConditionTypeRBACPolicyDegraded, conditions.ReasonPolicyNotFound},
		{conditions.ConditionTypeRBACPolicyDegraded, conditions.ReasonEPGPending},
		{conditions.ConditionTypeRBACPolicyValid, conditions.ReasonValidationPassed},
		{conditions.ConditionTypeRBACPolicyValid, conditions.ReasonValidationFailed},
		{conditions.ConditionTypeRBACProfilePolicyCompliant, conditions.ReasonBootstrapProfilesReady},
		{conditions.ConditionTypeRBACProfilePolicyCompliant, conditions.ReasonBootstrapProfilesPending},
		{conditions.ConditionTypeRBACProfileProvisioned, conditions.ReasonProvisioningComplete},
		{conditions.ConditionTypeRBACProfileProvisioned, conditions.ReasonProvisioningFailed},
		{conditions.ConditionTypeRBACProfileValidated, conditions.ReasonValidationPassed},
		{conditions.ConditionTypeRBACProfileValidated, conditions.ReasonValidationFailed},
		{conditions.ConditionTypeWebhookRegistered, conditions.ReasonWebhookRegistered},

		// Platform — TalosCluster
		{conditions.ConditionTypeReady, conditions.ReasonClusterReady},
		{conditions.ConditionTypeReady, conditions.ReasonJobComplete},
		{conditions.ConditionTypeReady, conditions.ReasonPackReceiptReady},
		{conditions.ConditionTypeDegraded, conditions.ReasonBootstrapJobFailed},
		{conditions.ConditionTypeDegraded, conditions.ReasonJobFailed},
		{conditions.ConditionTypeDegraded, conditions.ReasonCapabilityUnknown},
		{conditions.ConditionTypeDegraded, conditions.ReasonReconcilerNotImplemented},
		{conditions.ConditionTypeRunning, conditions.ReasonJobSubmitted},
		{conditions.ConditionTypeRunning, conditions.ReasonJobComplete},
		{conditions.ConditionTypePending, conditions.ReasonPending},
		{conditions.ConditionTypePending, conditions.ReasonGatesClearing},
		{conditions.ConditionTypePending, conditions.ReasonAwaitingSignature},
		{conditions.ConditionTypePending, conditions.ReasonAwaitingConductorReady},
		{conditions.ConditionTypeBootstrapping, conditions.ReasonBootstrapJobSubmitted},
		{conditions.ConditionTypeBootstrapping, conditions.ReasonBootstrapJobComplete},
		{conditions.ConditionTypeBootstrapping, conditions.ReasonBootstrapJobFailed},
		{conditions.ConditionTypeBootstrapping, conditions.ReasonCAPIObjectsCreated},
		{conditions.ConditionTypeBootstrapping, conditions.ReasonCAPIClusterRunning},
		{conditions.ConditionTypeImporting, conditions.ReasonImportComplete},
		{conditions.ConditionTypeCiliumPending, conditions.ReasonCiliumPackPending},
		{conditions.ConditionTypeCiliumPending, conditions.ReasonCiliumPackReady},
		{conditions.ConditionTypeControlPlaneUnreachable, conditions.ReasonControlPlaneNodeUnreachable},
		{conditions.ConditionTypePartialWorkerAvailability, conditions.ReasonWorkerNodeUnreachable},
		{conditions.ConditionTypeConductorReady, conditions.ReasonConductorBootstrapComplete},
		{conditions.ConditionTypeConductorReady, conditions.ReasonConductorBootstrapPending},

		// Platform — ClusterMaintenance
		{conditions.ConditionTypeClusterMaintenancePaused, conditions.ReasonCAPIPaused},
		{conditions.ConditionTypeClusterMaintenancePaused, conditions.ReasonCAPIResumed},
		{conditions.ConditionTypeClusterMaintenanceWindowActive, conditions.ReasonMaintenanceWindowOpen},
		{conditions.ConditionTypeClusterMaintenanceWindowActive, conditions.ReasonMaintenanceWindowClosed},

		// Platform — Day-2 specific
		{conditions.ConditionTypeNodeOperationCAPIDelegated, conditions.ReasonNodeOpCAPIDelegated},
		{conditions.ConditionTypeResetPendingApproval, conditions.ReasonApprovalRequired},
		{conditions.EtcdBackupDestinationAbsent, conditions.ReasonS3DestinationAbsent},

		// Platform — CAPI Infrastructure Provider
		{conditions.ConditionTypeInfrastructureReady, conditions.ReasonAllControlPlaneMachinesReady},
		{conditions.ConditionTypeInfrastructureReady, conditions.ReasonControlPlaneMachinesNotReady},
		{conditions.ConditionTypeInfrastructureReady, conditions.ReasonControlPlaneMachinesPending},
		{conditions.ConditionTypeMachineReady, conditions.ReasonMachineReady},
		{conditions.ConditionTypeMachineReady, conditions.ReasonMachineConfigApplied},
		{conditions.ConditionTypeMachineReady, conditions.ReasonMachineConfigFailed},
		{conditions.ConditionTypeMachineReady, conditions.ReasonBootstrapDataNotReady},
		{conditions.ConditionTypeMachineReady, conditions.ReasonCAPIMachineNotBound},
		{conditions.ConditionTypeMachineReady, conditions.ReasonMachineOutOfMaintenance},
		{conditions.ConditionTypePortReachable, conditions.ReasonPortUnreachable},

		// Wrapper — ClusterPack
		{conditions.ConditionTypeClusterPackAvailable, conditions.ReasonPackAvailable},
		{conditions.ConditionTypeClusterPackAvailable, conditions.ReasonPackSignaturePending},
		{conditions.ConditionTypeClusterPackImmutabilityViolation, conditions.ReasonImmutabilityViolation},
		{conditions.ConditionTypeClusterPackRevoked, conditions.ReasonPackRevoked},
		{conditions.ConditionTypeClusterPackSignaturePending, conditions.ReasonPackSignaturePending},
		{conditions.ConditionTypeClusterPackSignaturePending, conditions.ReasonPackSigned},

		// Wrapper — PackExecution
		{conditions.ConditionTypePackExecutionWaiting, conditions.ReasonAwaitingConductorReady},
		{conditions.ConditionTypePackSignaturePending, conditions.ReasonAwaitingSignature},
		{conditions.ConditionTypePackSignaturePending, conditions.ReasonPackSigned},
		{conditions.ConditionTypePackRevoked, conditions.ReasonClusterPackRevoked},
		{conditions.ConditionTypePermissionSnapshotOutOfSync, conditions.ReasonSnapshotOutOfSync},
		{conditions.ConditionTypeRBACProfileNotProvisioned, conditions.ReasonRBACProfileNotReady},
		{conditions.ConditionTypePackExecutionFailed, conditions.ReasonJobFailed},
		{conditions.ConditionTypePackExecutionFailed, conditions.ReasonOperationResultNotFound},
		{conditions.ConditionTypePackExecutionSucceeded, conditions.ReasonJobSucceeded},

		// Wrapper — PackInstance
		{conditions.ConditionTypePackInstanceDependencyBlocked, conditions.ReasonDependencyDrifted},
		{conditions.ConditionTypePackInstanceDrifted, conditions.ReasonDriftDetected},
		{conditions.ConditionTypePackInstanceDrifted, conditions.ReasonNoDrift},
		{conditions.ConditionTypePackInstanceProgressing, conditions.ReasonPackDelivered},
		{conditions.ConditionTypePackInstanceProgressing, conditions.ReasonPackReceiptNotFound},
		{conditions.ConditionTypePackInstanceSecurityViolation, conditions.ReasonSignatureVerifyFailed},
		{conditions.ConditionTypePackInstanceSecurityViolation, conditions.ReasonSecurityViolationCleared},
	}

	for _, tc := range knownPairs {
		tc := tc
		t.Run(tc.condType+"/"+tc.reason, func(t *testing.T) {
			t.Parallel()
			if err := conditions.ValidateCondition(tc.condType, tc.reason); err != nil {
				t.Errorf("ValidateCondition(%q, %q) returned unexpected error: %v", tc.condType, tc.reason, err)
			}
		})
	}
}

// TestValidateCondition_RejectsUnknownType verifies that ValidateCondition returns
// an error for a condition type not in the vocabulary.
func TestValidateCondition_RejectsUnknownType(t *testing.T) {
	t.Parallel()

	err := conditions.ValidateCondition("UnknownConditionType", "SomeReason")
	if err == nil {
		t.Fatal("expected error for unknown condition type, got nil")
	}
	if !strings.Contains(err.Error(), "unknown condition type") {
		t.Errorf("expected 'unknown condition type' in error message, got: %v", err)
	}
}

// TestValidateCondition_RejectsUnknownReason verifies that ValidateCondition returns
// an error when the reason is not valid for a known condition type.
func TestValidateCondition_RejectsUnknownReason(t *testing.T) {
	t.Parallel()

	err := conditions.ValidateCondition(conditions.ConditionTypeLineageSynced, "BogusReason")
	if err == nil {
		t.Fatal("expected error for unknown reason, got nil")
	}
	if !strings.Contains(err.Error(), "BogusReason") {
		t.Errorf("expected reason name in error message, got: %v", err)
	}
}

// TestValidateCondition_RejectsEmptyInputs verifies that empty conditionType or
// reason produce errors.
func TestValidateCondition_RejectsEmptyInputs(t *testing.T) {
	t.Parallel()

	if err := conditions.ValidateCondition("", "SomeReason"); err == nil {
		t.Error("expected error for empty conditionType, got nil")
	}
	if err := conditions.ValidateCondition(conditions.ConditionTypeReady, ""); err == nil {
		t.Error("expected error for empty reason, got nil")
	}
}

// TestValidateCondition_ErrorMessageContainsValidReasons verifies that the error
// message for an invalid reason lists the valid alternatives.
func TestValidateCondition_ErrorMessageContainsValidReasons(t *testing.T) {
	t.Parallel()

	err := conditions.ValidateCondition(conditions.ConditionTypeConductorReady, "WrongReason")
	if err == nil {
		t.Fatal("expected error for invalid reason, got nil")
	}
	msg := err.Error()
	if !strings.Contains(msg, conditions.ReasonConductorBootstrapComplete) {
		t.Errorf("error message should list valid reasons including %q; got: %v",
			conditions.ReasonConductorBootstrapComplete, msg)
	}
	if !strings.Contains(msg, conditions.ReasonConductorBootstrapPending) {
		t.Errorf("error message should list valid reasons including %q; got: %v",
			conditions.ReasonConductorBootstrapPending, msg)
	}
}

// TestKnownConditionTypes verifies that KnownConditionTypes returns a non-empty
// sorted list containing at least the canonical cross-operator types.
func TestKnownConditionTypes(t *testing.T) {
	t.Parallel()

	types := conditions.KnownConditionTypes()
	if len(types) == 0 {
		t.Fatal("KnownConditionTypes returned empty slice")
	}
	// Verify sorted order.
	for i := 1; i < len(types); i++ {
		if types[i] < types[i-1] {
			t.Errorf("KnownConditionTypes is not sorted: %q comes before %q", types[i-1], types[i])
		}
	}
	// Verify key entries are present.
	found := make(map[string]bool)
	for _, ct := range types {
		found[ct] = true
	}
	required := []string{
		conditions.ConditionTypeLineageSynced,
		conditions.ConditionTypeReady,
		conditions.ConditionTypeDegraded,
		conditions.ConditionTypeConductorReady,
	}
	for _, r := range required {
		if !found[r] {
			t.Errorf("KnownConditionTypes missing expected entry %q", r)
		}
	}
}

// TestValidReasonsFor verifies that ValidReasonsFor returns the correct set and
// nil for unknown types.
func TestValidReasonsFor(t *testing.T) {
	t.Parallel()

	got := conditions.ValidReasonsFor(conditions.ConditionTypeLineageSynced)
	if len(got) != 1 || got[0] != conditions.ReasonLineageControllerAbsent {
		t.Errorf("ValidReasonsFor(LineageSynced) = %v; want [%s]",
			got, conditions.ReasonLineageControllerAbsent)
	}

	if conditions.ValidReasonsFor("NoSuchType") != nil {
		t.Error("ValidReasonsFor(unknown) should return nil")
	}
}

// TestAliasesProduceSameString verifies that all Go constant aliases that share
// a string value are actually equal. This catches copy-paste drift.
func TestAliasesProduceSameString(t *testing.T) {
	t.Parallel()

	degradedAliases := []string{
		conditions.ConditionTypeDegraded,
		conditions.ConditionTypeEtcdMaintenanceDegraded,
		conditions.ConditionTypeNodeMaintenanceDegraded,
		conditions.ConditionTypePKIRotationDegraded,
		conditions.ConditionTypeResetDegraded,
		conditions.ConditionTypeUpgradePolicyDegraded,
		conditions.ConditionTypeNodeOperationDegraded,
		conditions.ConditionTypeMaintenanceBundleDegraded,
	}
	for _, a := range degradedAliases {
		if a != "Degraded" {
			t.Errorf("expected Degraded alias to equal %q, got %q", "Degraded", a)
		}
	}

	readyAliases := []string{
		conditions.ConditionTypeReady,
		conditions.ConditionTypeEtcdMaintenanceReady,
		conditions.ConditionTypeNodeMaintenanceReady,
		conditions.ConditionTypePKIRotationReady,
		conditions.ConditionTypeResetReady,
		conditions.ConditionTypeUpgradePolicyReady,
		conditions.ConditionTypeNodeOperationReady,
		conditions.ConditionTypeMaintenanceBundleReady,
	}
	for _, a := range readyAliases {
		if a != "Ready" {
			t.Errorf("expected Ready alias to equal %q, got %q", "Ready", a)
		}
	}

	jobCompleteAliases := []string{
		conditions.ReasonJobComplete,
		conditions.ReasonEtcdJobComplete,
		conditions.ReasonNodeJobComplete,
		conditions.ReasonPKIJobComplete,
		conditions.ReasonResetJobComplete,
		conditions.ReasonUpgradeJobComplete,
		conditions.ReasonNodeOpJobComplete,
		conditions.ReasonMaintenanceBundleJobComplete,
	}
	for _, a := range jobCompleteAliases {
		if a != "JobComplete" {
			t.Errorf("expected JobComplete alias to equal %q, got %q", "JobComplete", a)
		}
	}
}
