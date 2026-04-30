// Package conditions defines the platform-wide condition type and reason string
// vocabulary for all Seam operators. It is the canonical source of truth for every
// condition type and reason constant used across guardian, platform, wrapper,
// conductor, and seam-core.
//
// Consolidation rationale: each operator previously declared its own condition
// and reason constants locally, creating drift risk. This package centralises the
// vocabulary so that ValidateCondition can assert correctness at test time and
// operators can reference a shared, typed constant instead of a raw string.
//
// seam-core-schema.md §7 Declaration 5 (LineageSynced reserved cross-operator).
// Gap 31.
package conditions

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SetCondition finds an existing condition of the given type in the slice and
// updates it in-place, or appends a new condition if none exists.
// LastTransitionTime is updated only when the Status value changes.
func SetCondition(
	conditions *[]metav1.Condition,
	conditionType string,
	status metav1.ConditionStatus,
	reason string,
	message string,
	observedGeneration int64,
) {
	now := metav1.Now()
	existing := FindCondition(*conditions, conditionType)
	if existing == nil {
		*conditions = append(*conditions, metav1.Condition{
			Type:               conditionType,
			Status:             status,
			Reason:             reason,
			Message:            message,
			ObservedGeneration: observedGeneration,
			LastTransitionTime: now,
		})
		return
	}
	if existing.Status != status {
		existing.LastTransitionTime = now
	}
	existing.Status = status
	existing.Reason = reason
	existing.Message = message
	existing.ObservedGeneration = observedGeneration
}

// FindCondition returns a pointer to the first condition with the given Type,
// or nil if no such condition exists.
func FindCondition(conditions []metav1.Condition, conditionType string) *metav1.Condition {
	for i := range conditions {
		if conditions[i].Type == conditionType {
			return &conditions[i]
		}
	}
	return nil
}

// ─── Cross-operator conditions ───────────────────────────────────────────────
// These condition types and reasons are used by all five Seam operators.

const (
	// ConditionTypeLineageSynced is the reserved condition type set on every root
	// declaration CR across all Seam operators. This is the canonical source —
	// supersedes the duplicate declarations in:
	//   - seam-core/api/v1alpha1/infrastructurelineageindex_types.go
	//   - guardian/api/v1alpha1/lineage_conditions.go
	//   - platform/api/v1alpha1/lineage_conditions.go
	//   - wrapper/api/v1alpha1/packexecution_types.go (ReasonLineageControllerAbsent only)
	//   - platform/api/infrastructure/v1alpha1/lineage_conditions.go
	//
	// Lifecycle protocol (seam-core-schema.md §7 Declaration 5):
	//  1. On first observation the responsible reconciler sets this to False with
	//     reason ReasonLineageControllerAbsent. One-time write; never written again.
	//  2. InfrastructureLineageController takes ownership on deployment and sets True.
	//  3. If InfrastructureLineageController is absent, remains False indefinitely.
	//     This is expected steady-state during the stub phase.
	//
	// Terminal state: True (set by InfrastructureLineageController, not by operator).
	// Operators: guardian, platform, wrapper, seam-core.
	ConditionTypeLineageSynced = "LineageSynced"

	// ReasonLineageControllerAbsent is set when a reconciler initialises
	// LineageSynced to False. Indicates InfrastructureLineageController is not yet
	// deployed. seam-core-schema.md §7 Declaration 5.
	ReasonLineageControllerAbsent = "LineageControllerAbsent"
)

// ─── Guardian conditions ──────────────────────────────────────────────────────
// Operator: guardian (security.ontai.dev).

const (
	// ConditionTypeBootstrapLabelAbsent is set on the Guardian singleton when
	// the seam-system namespace does not carry the required webhook-mode label.
	// Terminal state: False (label present → webhook can register).
	// Operators: guardian.
	ConditionTypeBootstrapLabelAbsent = "BootstrapLabelAbsent"

	// ReasonLabelAbsent is set on BootstrapLabelAbsent=True when the
	// seam.ontai.dev/webhook-mode label is missing from seam-system.
	ReasonLabelAbsent = "LabelAbsent"

	// ReasonLabelPresent is set on BootstrapLabelAbsent=False when the label is present.
	ReasonLabelPresent = "LabelPresent"
)

const (
	// ConditionTypeIdentityBindingTrustAnchorResolved indicates whether the
	// TrustAnchor referenced by an IdentityBinding has been resolved.
	// Terminal state: True (trust anchor resolved and validated).
	// Operators: guardian.
	ConditionTypeIdentityBindingTrustAnchorResolved = "TrustAnchorResolved"

	// ReasonTrustAnchorResolved is set on TrustAnchorResolved=True.
	ReasonTrustAnchorResolved = "TrustAnchorResolved"

	// ReasonTrustAnchorNotFound is set when the referenced TrustAnchor CR does not exist.
	ReasonTrustAnchorNotFound = "TrustAnchorNotFound"

	// ReasonTrustAnchorInvalid is set when the TrustAnchor CR exists but is invalid.
	ReasonTrustAnchorInvalid = "TrustAnchorInvalid"

	// ReasonTrustAnchorTypeMismatch is set when the TrustAnchor type does not match
	// the IdentityBinding's expected trust method.
	ReasonTrustAnchorTypeMismatch = "TrustAnchorTypeMismatch"

	// ReasonTrustMethodMismatch is set when the IdentityBinding and TrustAnchor
	// disagree on the trust method.
	ReasonTrustMethodMismatch = "TrustMethodMismatch"
)

const (
	// ConditionTypeIdentityBindingValid indicates whether an IdentityBinding has
	// passed structural and semantic validation.
	// Terminal state: True (binding valid) or False (binding structurally invalid,
	// requires human correction).
	// Operators: guardian.
	ConditionTypeIdentityBindingValid = "IdentityBindingValid"

	// ReasonIdentityBindingValid is set on IdentityBindingValid=True.
	ReasonIdentityBindingValid = "Valid"

	// ReasonIdentityBindingInvalid is set on IdentityBindingValid=False.
	ReasonIdentityBindingInvalid = "Invalid"

	// ReasonPermissionSetMissing is set when a required PermissionSet is absent.
	ReasonPermissionSetMissing = "PermissionSetMissing"

	// ReasonPermissionSetNotFound is set when the referenced PermissionSet CR cannot
	// be found.
	ReasonPermissionSetNotFound = "PermissionSetNotFound"

	// ReasonTokenTTLExceeded is set when an identity token has exceeded its TTL.
	ReasonTokenTTLExceeded = "TokenTTLExceeded"
)

const (
	// ConditionTypeIdentityProviderReachable indicates whether the configured
	// IdentityProvider endpoint is reachable.
	// Terminal state: neither (polled continuously).
	// Operators: guardian.
	ConditionTypeIdentityProviderReachable = "Reachable"

	// ReasonIdentityProviderReachable is set on Reachable=True.
	ReasonIdentityProviderReachable = "Reachable"

	// ReasonIdentityProviderUnreachable is set on Reachable=False.
	ReasonIdentityProviderUnreachable = "Unreachable"

	// ReasonIdentityProviderPending is set while the reachability check is pending.
	ReasonIdentityProviderPending = "Pending"
)

const (
	// ConditionTypeIdentityProviderValid indicates whether an IdentityProvider CR
	// has passed structural validation.
	// Terminal state: True (provider valid) or False (invalid, requires correction).
	// Operators: guardian.
	ConditionTypeIdentityProviderValid = "Valid"

	// ReasonIdentityProviderValid is set on Valid=True.
	ReasonIdentityProviderValid = "Valid"

	// ReasonIdentityProviderInvalid is set on Valid=False.
	ReasonIdentityProviderInvalid = "Invalid"
)

const (
	// ConditionTypePermissionSetValid indicates whether a PermissionSet CR has
	// passed structural and reference validation.
	// Terminal state: True (valid) or False (invalid).
	// Operators: guardian.
	ConditionTypePermissionSetValid = "PermissionSetValid"

	// ReasonPermissionSetValid is set on PermissionSetValid=True.
	ReasonPermissionSetValid = "Valid"

	// ReasonPermissionSetInvalid is set on PermissionSetValid=False.
	ReasonPermissionSetInvalid = "Invalid"
)

const (
	// ConditionTypeRBACPolicyDegraded indicates that an RBACPolicy CR has entered a
	// degraded state due to a policy evaluation or validation failure.
	// Terminal state: True (degraded, requires intervention).
	// Operators: guardian.
	ConditionTypeRBACPolicyDegraded = "RBACPolicyDegraded"

	// ReasonValidationFailed is set when policy validation fails.
	ReasonValidationFailed = "ValidationFailed"

	// ReasonPolicyViolation is set when an evaluated policy produces a violation.
	ReasonPolicyViolation = "PolicyViolation"

	// ReasonStructureInvalid is set when the policy structure is malformed.
	ReasonStructureInvalid = "StructureInvalid"

	// ReasonPolicyNotFound is set when a referenced policy cannot be found.
	ReasonPolicyNotFound = "PolicyNotFound"

	// ReasonEPGPending is set when an endpoint group is not yet ready.
	ReasonEPGPending = "EPGPending"
)

const (
	// ConditionTypeRBACPolicyValid indicates whether an RBACPolicy CR has passed
	// validation.
	// Terminal state: True (valid) or False (invalid).
	// Operators: guardian.
	ConditionTypeRBACPolicyValid = "RBACPolicyValid"

	// ReasonValidationPassed is set on RBACPolicyValid=True.
	ReasonValidationPassed = "ValidationPassed"
)

const (
	// ConditionTypeRBACProfilePolicyCompliant indicates whether an RBACProfile has
	// passed bootstrap profile readiness checks.
	// Terminal state: True (compliant).
	// Operators: guardian.
	ConditionTypeRBACProfilePolicyCompliant = "PolicyCompliant"

	// ReasonBootstrapProfilesReady is set when all bootstrap RBACProfile CRs are ready.
	ReasonBootstrapProfilesReady = "BootstrapProfilesReady"

	// ReasonBootstrapProfilesPending is set while bootstrap RBACProfile CRs are pending.
	ReasonBootstrapProfilesPending = "BootstrapProfilesPending"
)

const (
	// ConditionTypeRBACProfileProvisioned indicates whether an RBACProfile has been
	// fully provisioned. All operators gate execution on this condition being True.
	// Terminal state: True (provisioned).
	// Operators: guardian (writes); platform, wrapper (reads).
	ConditionTypeRBACProfileProvisioned = "Provisioned"

	// ReasonProvisioningComplete is set on Provisioned=True.
	ReasonProvisioningComplete = "ProvisioningComplete"

	// ReasonProvisioningFailed is set on Provisioned=False after a provisioning failure.
	ReasonProvisioningFailed = "ProvisioningFailed"
)

const (
	// ConditionTypeRBACProfileValidated indicates whether an RBACProfile has passed
	// structural validation.
	// Terminal state: True (validated).
	// Operators: guardian.
	ConditionTypeRBACProfileValidated = "ProfileValidated"
)

const (
	// ConditionTypeWebhookRegistered indicates whether the Guardian admission webhook
	// has been registered with the API server.
	// Terminal state: True (registered).
	// Operators: guardian.
	ConditionTypeWebhookRegistered = "WebhookRegistered"

	// ReasonWebhookRegistered is set on WebhookRegistered=True.
	ReasonWebhookRegistered = "WebhookRegistered"
)

const (
	// ConditionTypePermissionSnapshotFresh indicates whether a PermissionSnapshot
	// is fresh relative to its spec.freshnessWindowSeconds. The reconciler sets
	// this to True on first observation and re-evaluates on every requeue.
	// Terminal state: False (stale) when snapshot age exceeds freshnessWindowSeconds.
	// Operators: guardian (PermissionSnapshotReconciler, both roles).
	ConditionTypePermissionSnapshotFresh = "Fresh"

	// ReasonSnapshotFresh is set on Fresh=True when the snapshot age is within
	// spec.freshnessWindowSeconds from SnapshotTimestamp.
	ReasonSnapshotFresh = "SnapshotFresh"

	// ReasonSnapshotStale is set on Fresh=False when the snapshot age has
	// exceeded spec.freshnessWindowSeconds.
	ReasonSnapshotStale = "SnapshotStale"
)

const (
	// ConditionTypePermissionSnapshotCompliant indicates whether the observed RBAC
	// surface on the target cluster matches the snapshot content.
	// Terminal state: False (non-compliant) when drift is detected by Guardian's
	// drift detection loop on the management cluster.
	// Operators: guardian (management cluster drift detection).
	ConditionTypePermissionSnapshotCompliant = "Compliant"

	// ReasonDriftAbsent is set on Compliant=True when no RBAC drift is detected
	// between the snapshot content and the observed cluster RBAC surface.
	ReasonDriftAbsent = "DriftAbsent"
	// ReasonDriftDetected (already declared under PackInstance) is reused on
	// Compliant=False when drift is detected. String value: "DriftDetected".
)

// ─── Platform — TalosCluster conditions ──────────────────────────────────────
// Operator: platform (platform.ontai.dev), TalosCluster CR.

const (
	// ConditionTypeReady indicates a resource is fully operational and all checks
	// have passed. Used by TalosCluster, all day-2 operation CRDs, SIC, and
	// PackInstance. The terminal True state signals that the operation completed.
	// Terminal state: True.
	// Operators: platform (TalosCluster, EtcdMaintenance, NodeMaintenance,
	//   PKIRotation, ClusterReset, UpgradePolicy, NodeOperation, MaintenanceBundle,
	//   SeamInfrastructureCluster), wrapper (PackInstance).
	//
	// Aliases in platform: ConditionTypeEtcdMaintenanceReady, ConditionTypeNodeMaintenanceReady,
	//   ConditionTypePKIRotationReady, ConditionTypeResetReady, ConditionTypeUpgradePolicyReady,
	//   ConditionTypeNodeOperationReady, ConditionTypeMaintenanceBundleReady.
	ConditionTypeReady = "Ready"

	// Aliases — same string value; retained for migration compatibility.
	ConditionTypeEtcdMaintenanceReady      = ConditionTypeReady
	ConditionTypeNodeMaintenanceReady      = ConditionTypeReady
	ConditionTypePKIRotationReady          = ConditionTypeReady
	ConditionTypeResetReady                = ConditionTypeReady
	ConditionTypeUpgradePolicyReady        = ConditionTypeReady
	ConditionTypeNodeOperationReady        = ConditionTypeReady
	ConditionTypeMaintenanceBundleReady    = ConditionTypeReady

	// ReasonClusterReady is set on TalosCluster Ready=True.
	ReasonClusterReady = "ClusterReady"

	// ReasonCAPIClusterRunning is set on TalosCluster Ready when CAPI cluster Running.
	ReasonCAPIClusterRunning = "CAPIClusterRunning"

	// ReasonJobComplete is set on day-2 operation Ready=True when the RunnerConfig
	// or Job has completed successfully. Shared by EtcdMaintenance, NodeMaintenance,
	// PKIRotation, ClusterReset, UpgradePolicy, NodeOperation, MaintenanceBundle.
	// Also used as a terminal reason on Running=False and Pending=False.
	ReasonJobComplete = "JobComplete"

	// Aliases for ReasonJobComplete — retained for migration compatibility.
	ReasonEtcdJobComplete            = ReasonJobComplete
	ReasonNodeJobComplete            = ReasonJobComplete
	ReasonPKIJobComplete             = ReasonJobComplete
	ReasonResetJobComplete           = ReasonJobComplete
	ReasonUpgradeJobComplete         = ReasonJobComplete
	ReasonNodeOpJobComplete          = ReasonJobComplete
	ReasonMaintenanceBundleJobComplete = ReasonJobComplete

	// ReasonResetComplete is set on ClusterReset Ready=True.
	ReasonResetComplete = "ResetComplete"

	// ReasonAllControlPlaneMachinesReady is set on SIC InfrastructureReady=True.
	ReasonAllControlPlaneMachinesReady = "AllControlPlaneMachinesReady"

	// ReasonPackReceiptReady is set on PackInstance Ready=True.
	ReasonPackReceiptReady = "PackReceiptReady"
)

const (
	// ConditionTypeDegraded indicates that a resource has entered a degraded state
	// requiring operator attention. Used by TalosCluster and all day-2 CRDs.
	// Terminal state: True (terminal failure, requeue not issued).
	// Operators: platform.
	//
	// Aliases in platform: ConditionTypeEtcdMaintenanceDegraded, ConditionTypeNodeMaintenanceDegraded,
	//   ConditionTypePKIRotationDegraded, ConditionTypeResetDegraded, ConditionTypeUpgradePolicyDegraded,
	//   ConditionTypeNodeOperationDegraded, ConditionTypeMaintenanceBundleDegraded.
	ConditionTypeDegraded = "Degraded"

	// Aliases — same string value; retained for migration compatibility.
	ConditionTypeEtcdMaintenanceDegraded   = ConditionTypeDegraded
	ConditionTypeNodeMaintenanceDegraded   = ConditionTypeDegraded
	ConditionTypePKIRotationDegraded       = ConditionTypeDegraded
	ConditionTypeResetDegraded             = ConditionTypeDegraded
	ConditionTypeUpgradePolicyDegraded     = ConditionTypeDegraded
	ConditionTypeNodeOperationDegraded     = ConditionTypeDegraded
	ConditionTypeMaintenanceBundleDegraded = ConditionTypeDegraded

	// ReasonDegraded is a generic degraded reason for TalosCluster.
	ReasonDegraded = "Degraded"

	// ReasonBootstrapJobFailed is set on TalosCluster Degraded=True when the
	// management cluster bootstrap Job fails.
	ReasonBootstrapJobFailed = "BootstrapJobFailed"

	// ReasonConductorJobGateBlocked is set on Degraded=True or Paused=True when
	// the ConductorJobGate blocks execution.
	ReasonConductorJobGateBlocked = "ConductorJobGateBlocked"

	// ReasonJobFailed is set on Degraded=True when a day-2 RunnerConfig or Job fails.
	// Shared by EtcdMaintenance, NodeMaintenance, PKIRotation, ClusterReset,
	// UpgradePolicy, NodeOperation, MaintenanceBundle.
	ReasonJobFailed = "JobFailed"

	// Aliases for ReasonJobFailed — retained for migration compatibility.
	ReasonEtcdJobFailed            = ReasonJobFailed
	ReasonNodeJobFailed            = ReasonJobFailed
	ReasonPKIJobFailed             = ReasonJobFailed
	ReasonResetJobFailed           = ReasonJobFailed
	ReasonUpgradeJobFailed         = ReasonJobFailed
	ReasonNodeOpJobFailed          = ReasonJobFailed
	ReasonMaintenanceBundleJobFailed = ReasonJobFailed

	// ReasonCapabilityUnknown is set when the requested Conductor capability is
	// not declared in the capability registry.
	ReasonCapabilityUnknown = "CapabilityUnknown"

	// ReasonMaintenanceBundleCapabilityUnknown is an alias for ReasonCapabilityUnknown.
	ReasonMaintenanceBundleCapabilityUnknown = ReasonCapabilityUnknown

	// ReasonReconcilerNotImplemented is set when a reconciler stub has not yet
	// been implemented (deferred implementation milestone).
	ReasonReconcilerNotImplemented = "ReconcilerNotImplemented"

	// ReasonMaintenanceBundleReconcilerNotImplemented is an alias for
	// ReasonReconcilerNotImplemented.
	ReasonMaintenanceBundleReconcilerNotImplemented = ReasonReconcilerNotImplemented

	// ReasonS3DestinationAbsent is set on EtcdBackupDestinationAbsent=True when
	// no S3 backup destination is configured.
	ReasonS3DestinationAbsent = "S3DestinationAbsent"

	// ReasonEtcdBackupDestinationAbsent is an alias for ReasonS3DestinationAbsent.
	ReasonEtcdBackupDestinationAbsent = ReasonS3DestinationAbsent
)

const (
	// ConditionTypeRunning indicates that an operation (RunnerConfig or Job) has been
	// submitted and is in progress. Used by EtcdMaintenance (platform) and PackExecution (wrapper).
	// Terminal state: False (transitions to Degraded or Ready terminal states).
	// Operators: platform (EtcdMaintenance), wrapper (PackExecution).
	//
	// Alias in platform: ConditionTypeEtcdMaintenanceRunning.
	// Alias in wrapper: ConditionTypePackExecutionRunning.
	ConditionTypeRunning = "Running"

	// Aliases — same string value; retained for migration compatibility.
	ConditionTypeEtcdMaintenanceRunning = ConditionTypeRunning
	ConditionTypePackExecutionRunning   = ConditionTypeRunning

	// ReasonJobSubmitted is set on Running=True when a Job or RunnerConfig is submitted.
	// Shared by EtcdMaintenance and PackExecution.
	ReasonJobSubmitted = "JobSubmitted"

	// Aliases for ReasonJobSubmitted.
	ReasonEtcdJobSubmitted             = ReasonJobSubmitted
	ReasonMaintenanceBundleJobSubmitted = ReasonJobSubmitted
	ReasonNodeJobSubmitted             = ReasonJobSubmitted
	ReasonPKIJobSubmitted              = ReasonJobSubmitted
	ReasonResetJobSubmitted            = ReasonJobSubmitted
	ReasonUpgradeJobSubmitted          = ReasonJobSubmitted
	ReasonNodeOpJobSubmitted           = ReasonJobSubmitted

	// ReasonJobSucceeded is set on PackExecution Running=False / Succeeded=True.
	ReasonJobSucceeded = "JobSucceeded"
)

const (
	// ConditionTypePending indicates a resource is waiting for prerequisites. Used by
	// MaintenanceBundle (platform) and PackExecution (wrapper).
	// Terminal state: False (transitions to Ready or Degraded).
	// Operators: platform (MaintenanceBundle), wrapper (PackExecution).
	//
	// Alias in platform: ConditionTypeMaintenanceBundlePending.
	// Alias in wrapper: ConditionTypePackExecutionPending.
	ConditionTypePending = "Pending"

	// Aliases — same string value; retained for migration compatibility.
	ConditionTypeMaintenanceBundlePending = ConditionTypePending
	ConditionTypePackExecutionPending     = ConditionTypePending

	// ReasonPending is set on Pending=True as a generic "in progress" reason.
	ReasonPending = "Pending"

	// Aliases for ReasonPending.
	ReasonEtcdOperationPending      = ReasonPending
	ReasonMaintenanceBundlePending  = ReasonPending
	ReasonNodeOperationPending      = ReasonPending
	ReasonNodeOpPending             = ReasonPending
	ReasonPKIOperationPending       = ReasonPending
	ReasonUpgradeOperationPending   = ReasonPending

	// ReasonGatesClearing is set on PackExecution Pending=True while gates are being
	// evaluated (gate 0–4).
	ReasonGatesClearing = "GatesClearing"

	// ReasonAwaitingSignature is set on PackExecution Pending=True and
	// PackSignaturePending=True while waiting for conductor signature.
	ReasonAwaitingSignature = "AwaitingSignature"

	// ReasonAwaitingConductorReady is set on PackExecution Pending=True and
	// Waiting=True while the target cluster's Conductor Deployment is not yet Available.
	// platform-schema.md §12. Gap 27.
	ReasonAwaitingConductorReady = "AwaitingConductorReady"
)

const (
	// ConditionTypeBootstrapping indicates a TalosCluster bootstrap operation is
	// in progress.
	// Terminal state: neither (transitions to Ready=True or Degraded=True).
	// Operators: platform (TalosCluster).
	ConditionTypeBootstrapping = "Bootstrapping"

	// ReasonBootstrapJobSubmitted is set on Bootstrapping=True when the bootstrap
	// Conductor Job has been submitted.
	ReasonBootstrapJobSubmitted = "BootstrapJobSubmitted"

	// ReasonBootstrapJobComplete is set on Bootstrapping=True when the bootstrap
	// Job has completed successfully.
	ReasonBootstrapJobComplete = "BootstrapJobComplete"

	// ReasonCAPIObjectsCreated is set on Bootstrapping=True when CAPI objects have
	// been created for a target cluster (capi.enabled=true).
	ReasonCAPIObjectsCreated = "CAPIObjectsCreated"
)

const (
	// ConditionTypeImporting indicates a TalosCluster import operation is in progress.
	// Terminal state: True (import complete).
	// Operators: platform (TalosCluster).
	ConditionTypeImporting = "Importing"

	// ReasonImportComplete is set on Importing=True when the import completes.
	ReasonImportComplete = "ImportComplete"
)

const (
	// ConditionTypeCiliumPending indicates the CAPI cluster is Running but the
	// Cilium PackInstance has not yet reached Ready. Nodes are NotReady during
	// this window. platform-schema.md §5, CP-INV-013.
	// Terminal state: False (Cilium Ready).
	// Operators: platform (TalosCluster).
	ConditionTypeCiliumPending = "CiliumPending"

	// ReasonCiliumPackPending is set on CiliumPending=True.
	ReasonCiliumPackPending = "CiliumPackPending"

	// ReasonCiliumPackReady is set on CiliumPending=False when Cilium is Ready.
	ReasonCiliumPackReady = "CiliumPackReady"
)

const (
	// ConditionTypeControlPlaneUnreachable is set when control plane
	// SeamInfrastructureMachine nodes cannot be reached on port 50000 after the
	// retry threshold. Reconciliation halts until cleared.
	// Terminal state: True (halts reconciliation; clears on next reconcile when reachable).
	// Operators: platform (TalosCluster).
	ConditionTypeControlPlaneUnreachable = "ControlPlaneUnreachable"

	// ReasonControlPlaneNodeUnreachable is set on ControlPlaneUnreachable=True.
	ReasonControlPlaneNodeUnreachable = "ControlPlaneNodeUnreachable"
)

const (
	// ConditionTypePartialWorkerAvailability is set when one or more worker
	// SeamInfrastructureMachine nodes cannot be reached. Reconciliation continues
	// with available workers.
	// Terminal state: neither (clears automatically on next successful reconcile).
	// Operators: platform (TalosCluster).
	ConditionTypePartialWorkerAvailability = "PartialWorkerAvailability"

	// ReasonWorkerNodeUnreachable is set on PartialWorkerAvailability=True.
	ReasonWorkerNodeUnreachable = "WorkerNodeUnreachable"
)

const (
	// ConditionTypeConductorReady gates cluster readiness on the conductor bootstrap
	// window being complete. True when ont-system namespace, conductor ServiceAccount,
	// ClusterRole/Binding, and InfrastructureTalosCluster copy are all established on
	// the tenant cluster. The cluster does not transition to Ready until ConductorReady=True.
	// Platform never deploys the conductor Deployment -- that is admin-controlled via
	// the enable bundle. platform-schema.md §12.
	// Operators: platform (TalosCluster, writes); wrapper (PackExecutionReconciler, reads).
	ConditionTypeConductorReady = "ConductorReady"

	// ReasonConductorBootstrapComplete is set on ConductorReady=True when the bootstrap
	// window items (namespace, SA, RBAC, InfrastructureTalosCluster copy) are all done.
	ReasonConductorBootstrapComplete = "ConductorBootstrapComplete"

	// ReasonConductorBootstrapPending is set on ConductorReady=False while the bootstrap
	// window setup is still in progress (kubeconfig not yet available, or items pending).
	ReasonConductorBootstrapPending = "ConductorBootstrapPending"
)

const (
	// ConditionTypeBootstrapped indicates whether the bootstrap or import phase has
	// completed. True means the operation succeeded. False means in progress or failed.
	// Replaces the confusing Bootstrapping=False-means-complete pattern.
	// Operators: platform (TalosCluster).
	ConditionTypeBootstrapped = "Bootstrapped"

	// ConditionTypeScreenProviderNotImplemented is set when
	// spec.infrastructureProvider=screen is observed. Screen is a future operator
	// (INV-021). The reconciler halts without further reconciliation until Screen is
	// implemented. Clears when Screen support is added.
	// Operators: platform (TalosCluster).
	ConditionTypeScreenProviderNotImplemented = "ScreenProviderNotImplemented"

	// ConditionTypePhaseFailed is set when a required field is missing or invalid
	// and reconciliation cannot proceed. The reason encodes the specific cause.
	// Operators: platform (TalosCluster).
	ConditionTypePhaseFailed = "PhaseFailed"

	// ConditionTypeKubeconfigUnavailable is set on the import path when a kubeconfig
	// Secret cannot be generated because a prerequisite resource is absent.
	// Clears when the kubeconfig Secret is successfully written to seam-system.
	// Operators: platform (TalosCluster).
	ConditionTypeKubeconfigUnavailable = "KubeconfigUnavailable"

	// ReasonScreenNotImplemented is set on ScreenProviderNotImplemented=True when
	// spec.infrastructureProvider=screen is observed. INV-021.
	ReasonScreenNotImplemented = "ScreenNotImplemented"

	// ReasonTalosVersionRequired is set on PhaseFailed=True when spec.talosVersion is
	// empty and a RunnerConfig cannot be created without a version-tagged conductor image.
	// conductor-schema.md §3, INV-012.
	ReasonTalosVersionRequired = "TalosVersionRequired"

	// ReasonTalosConfigSecretAbsent is set on KubeconfigUnavailable=True when the
	// talosconfig Secret is not found in seam-system. Clears when the Secret appears.
	ReasonTalosConfigSecretAbsent = "TalosConfigSecretAbsent"
)

// ─── Platform — ClusterMaintenance conditions ────────────────────────────────

const (
	// ConditionTypeClusterMaintenancePaused indicates the cluster has been paused
	// for a maintenance window.
	// Terminal state: neither (toggles with maintenance window state).
	// Operators: platform (ClusterMaintenance).
	ConditionTypeClusterMaintenancePaused = "Paused"

	// ReasonCAPIPaused is set on Paused=True when CAPI reconciliation is paused.
	ReasonCAPIPaused = "CAPIPaused"

	// ReasonCAPIResumed is set on Paused=False when CAPI reconciliation resumes.
	ReasonCAPIResumed = "CAPIResumed"
)

const (
	// ConditionTypeClusterMaintenanceWindowActive indicates whether a maintenance
	// window is currently active.
	// Terminal state: neither.
	// Operators: platform (ClusterMaintenance).
	ConditionTypeClusterMaintenanceWindowActive = "WindowActive"

	// ReasonMaintenanceWindowOpen is set on WindowActive=True.
	ReasonMaintenanceWindowOpen = "MaintenanceWindowOpen"

	// ReasonMaintenanceWindowClosed is set on WindowActive=False.
	ReasonMaintenanceWindowClosed = "MaintenanceWindowClosed"
)

// ─── Platform — Day-2 specific conditions ────────────────────────────────────

const (
	// ConditionTypeNodeOperationCAPIDelegated indicates the NodeOperation has been
	// delegated to CAPI for execution.
	// Terminal state: True (delegated).
	// Aliases in platform: ConditionTypeUpgradePolicyCAPIDelegated.
	// Operators: platform (NodeOperation, UpgradePolicy).
	ConditionTypeNodeOperationCAPIDelegated = "CAPIDelegated"

	// Alias.
	ConditionTypeUpgradePolicyCAPIDelegated = ConditionTypeNodeOperationCAPIDelegated

	// ReasonNodeOpCAPIDelegated is set on CAPIDelegated=True.
	ReasonNodeOpCAPIDelegated = "CAPIDelegated"

	// ReasonUpgradeCAPIDelegated is an alias for ReasonNodeOpCAPIDelegated.
	ReasonUpgradeCAPIDelegated = ReasonNodeOpCAPIDelegated
)

const (
	// ConditionTypeResetPendingApproval indicates a ClusterReset is awaiting
	// human approval (ontai.dev/reset-approved=true annotation).
	// Terminal state: False (approved, proceeds).
	// Operators: platform (ClusterReset).
	ConditionTypeResetPendingApproval = "PendingApproval"

	// ReasonApprovalRequired is set on PendingApproval=True.
	ReasonApprovalRequired = "ApprovalRequired"
)

const (
	// EtcdBackupDestinationAbsent is the condition type set on EtcdMaintenance when
	// no S3 backup destination is configured. Note: this constant is not prefixed
	// with ConditionType for historical compatibility with the platform operator.
	// Terminal state: True (halts until S3 config is added).
	// Operators: platform (EtcdMaintenance).
	EtcdBackupDestinationAbsent = "EtcdBackupDestinationAbsent"

	// ReasonCAPIClusterDeleting is set when the CAPI cluster is being deleted.
	ReasonCAPIClusterDeleting = "CAPIClusterDeleting"

	// ReasonCAPIClusterDrained is set when the CAPI cluster has been drained.
	ReasonCAPIClusterDrained = "CAPIClusterDrained"
)

// ─── Platform — CAPI Infrastructure Provider conditions ─────────────────────
// Operator: platform (infrastructure.cluster.x-k8s.io), SeamInfrastructureCluster
// and SeamInfrastructureMachine.

const (
	// ConditionTypeInfrastructureReady indicates that the SeamInfrastructureCluster
	// has all required control plane machines ready.
	// Terminal state: True.
	// Operators: platform (SeamInfrastructureCluster).
	ConditionTypeInfrastructureReady = "InfrastructureReady"

	// ReasonControlPlaneMachinesNotReady is set on InfrastructureReady=False when
	// one or more control plane machines are not ready.
	ReasonControlPlaneMachinesNotReady = "ControlPlaneMachinesNotReady"

	// ReasonControlPlaneMachinesPending is set on InfrastructureReady=False while
	// control plane machines are being provisioned.
	ReasonControlPlaneMachinesPending = "ControlPlaneMachinesPending"
)

const (
	// ConditionTypeMachineReady indicates whether a SeamInfrastructureMachine has
	// completed the six-step machineconfig delivery flow.
	// Terminal state: True (machine ready and in cluster).
	// Operators: platform (SeamInfrastructureMachine).
	ConditionTypeMachineReady = "MachineReady"

	// ReasonMachineReady is set on MachineReady=True.
	ReasonMachineReady = "MachineReady"

	// ReasonMachineConfigApplied is set on MachineReady=False after machineconfig
	// delivery — node is rebooting.
	ReasonMachineConfigApplied = "MachineConfigApplied"

	// ReasonMachineConfigFailed is set on MachineReady=False when machineconfig
	// delivery failed.
	ReasonMachineConfigFailed = "MachineConfigFailed"

	// ReasonBootstrapDataNotReady is set on MachineReady=False when bootstrap data
	// has not yet been generated by CABPT.
	ReasonBootstrapDataNotReady = "BootstrapDataNotReady"

	// ReasonCAPIMachineNotBound is set on MachineReady=False when no CAPI Machine
	// has been bound to this SeamInfrastructureMachine.
	ReasonCAPIMachineNotBound = "CAPIMachineNotBound"

	// ReasonMachineOutOfMaintenance is set on MachineReady=False when the node has
	// exited maintenance mode and is transitioning to cluster membership.
	ReasonMachineOutOfMaintenance = "MachineOutOfMaintenance"
)

const (
	// ConditionTypePortReachable indicates whether port 50000 on the node is
	// reachable for machineconfig delivery.
	// Terminal state: True (reachable, delivery proceeding).
	// Operators: platform (SeamInfrastructureMachine).
	ConditionTypePortReachable = "PortReachable"

	// ReasonPortUnreachable is set on PortReachable=False and also on PortReachable=True
	// (the reconciler reuses this reason for the clear after successful delivery).
	ReasonPortUnreachable = "PortUnreachable"
)

// ─── Wrapper — ClusterPack conditions ────────────────────────────────────────
// Operator: wrapper (infra.ontai.dev), ClusterPack CR.

const (
	// ConditionTypeClusterPackAvailable indicates whether a ClusterPack is signed
	// and available for deployment.
	// Terminal state: True (signed and available).
	// Operators: wrapper (ClusterPack).
	ConditionTypeClusterPackAvailable = "Available"

	// ReasonPackAvailable is set on Available=True when the ClusterPack is signed.
	ReasonPackAvailable = "PackAvailable"

	// ReasonPackSignaturePending is set on Available=False while awaiting signature.
	// Also used as reason on ConditionTypeClusterPackSignaturePending=True.
	ReasonPackSignaturePending = "SignaturePending"
)

const (
	// ConditionTypeClusterPackImmutabilityViolation is set when a ClusterPack spec
	// has been mutated after creation. CI-INV-002.
	// Terminal state: True (immutability violation; no requeue).
	// Operators: wrapper (ClusterPack).
	ConditionTypeClusterPackImmutabilityViolation = "ImmutabilityViolation"

	// ReasonImmutabilityViolation is set on ImmutabilityViolation=True.
	ReasonImmutabilityViolation = "ImmutabilityViolation"
)

const (
	// ConditionTypeClusterPackRevoked indicates whether a ClusterPack has been
	// revoked. Set by the conductor signing loop; read by wrapper reconcilers.
	// Terminal state: True (revoked; no requeue).
	// Operators: conductor (writes); wrapper (reads).
	ConditionTypeClusterPackRevoked = "Revoked"

	// ReasonPackRevoked is set on Revoked=True.
	ReasonPackRevoked = "PackRevoked"
)

const (
	// ConditionTypeClusterPackSignaturePending indicates that the ClusterPack is
	// awaiting a signature from the conductor signing loop.
	// Terminal state: False (signed).
	// Operators: wrapper (ClusterPack).
	ConditionTypeClusterPackSignaturePending = "SignaturePending"

	// ReasonPackSigned is set on SignaturePending=False when the pack is signed.
	// Also used on Available=True.
	ReasonPackSigned = "PackSigned"
)

// ─── Wrapper — PackExecution conditions ──────────────────────────────────────
// Operator: wrapper (infra.ontai.dev), PackExecution CR.

const (
	// ConditionTypePackExecutionWaiting is set while gate 0 (ConductorReady) is not
	// yet cleared. The PackExecution is waiting for a cluster-level prerequisite.
	// Terminal state: False (conductor ready, proceed to gate 1).
	// Operators: wrapper (PackExecution). Gap 27.
	ConditionTypePackExecutionWaiting = "Waiting"
)

const (
	// ConditionTypePackSignaturePending is set on PackExecution while the referenced
	// ClusterPack has not yet been signed. Gate 1 of the 5-gate check.
	// Terminal state: False (signature present).
	// Operators: wrapper (PackExecution).
	ConditionTypePackSignaturePending = "PackSignaturePending"
)

const (
	// ConditionTypePackRevoked is set on PackExecution when the referenced ClusterPack
	// has been revoked. No requeue — human intervention required.
	// Terminal state: True.
	// Operators: wrapper (PackExecution).
	ConditionTypePackRevoked = "PackRevoked"

	// ReasonClusterPackRevoked is set on PackRevoked=True.
	ReasonClusterPackRevoked = "ClusterPackRevoked"
)

const (
	// ConditionTypePermissionSnapshotOutOfSync is set when the PermissionSnapshot
	// for the target cluster is not current. Requeues with backoff.
	// Terminal state: False (snapshot current, proceed).
	// Operators: wrapper (PackExecution).
	ConditionTypePermissionSnapshotOutOfSync = "PermissionSnapshotOutOfSync"

	// ReasonSnapshotOutOfSync is set on both True and False states of this condition.
	ReasonSnapshotOutOfSync = "SnapshotOutOfSync"
)

const (
	// ConditionTypeRBACProfileNotProvisioned is set when the RBACProfile for the
	// target cluster has not reached provisioned=true. Requeues with backoff.
	// Terminal state: False (profile provisioned, proceed).
	// Operators: wrapper (PackExecution).
	ConditionTypeRBACProfileNotProvisioned = "RBACProfileNotProvisioned"

	// ReasonRBACProfileNotReady is set on both True and False states of this condition.
	ReasonRBACProfileNotReady = "RBACProfileNotProvisioned"
)

const (
	// ConditionTypeWrapperRunnerRBACNotReady is set when the wrapper-runner
	// ServiceAccount in the target cluster namespace lacks a required RBAC
	// permission. Gate 5 of the PackExecution gate check — runs via
	// SubjectAccessReview before the Kueue Job is submitted.
	// Terminal state: False (all required verbs allowed, proceed).
	// Operators: wrapper (PackExecution).
	ConditionTypeWrapperRunnerRBACNotReady = "WrapperRunnerRBACNotReady"

	// ReasonWrapperRunnerRBACNotReady is set on both True and False states.
	ReasonWrapperRunnerRBACNotReady = "WrapperRunnerRBACNotReady"
)

const (
	// ConditionTypePackExecutionFailed indicates the pack-deploy Job failed.
	// Human intervention is required.
	// Terminal state: True.
	// Operators: wrapper (PackExecution).
	ConditionTypePackExecutionFailed = "Failed"

	// ReasonOperationResultNotFound is set on Failed=True when the Job succeeded but
	// the OperationResult ConfigMap was not written.
	ReasonOperationResultNotFound = "OperationResultNotFound"
)

const (
	// ConditionTypePackExecutionSucceeded indicates the pack-deploy Job completed
	// successfully and OperationResult was written.
	// Terminal state: True.
	// Operators: wrapper (PackExecution).
	ConditionTypePackExecutionSucceeded = "Succeeded"
)

// ─── Wrapper — PackInstance conditions ───────────────────────────────────────
// Operator: wrapper (infra.ontai.dev), PackInstance CR.

const (
	// ConditionTypePackInstanceReady indicates the pack has been delivered and
	// is in sync with the target cluster.
	// Terminal state: True (delivered and in sync).
	// Operators: wrapper (PackInstance).
	ConditionTypePackInstanceReady = "Ready"

	// ReasonAwaitingDelivery is set on Ready=False when no succeeded PackExecution
	// has been found for this pack and cluster yet.
	ReasonAwaitingDelivery = "AwaitingDelivery"
)

const (
	// ConditionTypePackInstanceDependencyBlocked is set when a dependency PackInstance
	// is drifted and the DependencyPolicy is Block.
	// Terminal state: True (blocked; clears when dependency drift resolves).
	// Operators: wrapper (PackInstance).
	ConditionTypePackInstanceDependencyBlocked = "DependencyBlocked"

	// ReasonDependencyDrifted is set on DependencyBlocked=True and also on
	// PackInstance Ready=False when blocked.
	ReasonDependencyDrifted = "DependencyDrifted"
)

const (
	// ConditionTypePackInstanceDrifted indicates drift between the expected pack
	// state and the observed state on the target cluster.
	// Terminal state: False (in sync).
	// Operators: wrapper (PackInstance).
	ConditionTypePackInstanceDrifted = "Drifted"

	// ReasonDriftDetected is set on Drifted=True.
	ReasonDriftDetected = "DriftDetected"

	// ReasonNoDrift is set on Drifted=False.
	ReasonNoDrift = "NoDrift"
)

const (
	// ConditionTypePackInstanceProgressing indicates that the PackInstance is
	// progressing toward a delivered state.
	// Terminal state: False (delivered).
	// Operators: wrapper (PackInstance).
	ConditionTypePackInstanceProgressing = "Progressing"

	// ReasonPackDelivered is set on Progressing=False when the pack is delivered.
	ReasonPackDelivered = "PackDelivered"

	// ReasonPackReceiptNotFound is set on Progressing=True and Ready=False while
	// waiting for the conductor to write the PackReceipt.
	ReasonPackReceiptNotFound = "PackReceiptNotFound"
)

const (
	// ConditionTypePackInstanceSecurityViolation is set when the PackReceipt reports
	// signatureVerified=false. All pack operations on the affected cluster are blocked.
	// Terminal state: True (cleared when violation resolves).
	// Operators: wrapper (PackInstance).
	ConditionTypePackInstanceSecurityViolation = "SecurityViolation"

	// ReasonSignatureVerifyFailed is set on SecurityViolation=True.
	// Also used on PackInstance Ready=False when blocked by a security violation.
	ReasonSignatureVerifyFailed = "SignatureVerifyFailed"

	// ReasonSecurityViolationCleared is set on SecurityViolation=False when
	// signature verification passes.
	ReasonSecurityViolationCleared = "SecurityViolationCleared"
)

// ─── Platform — Version upgrade and anti-regression conditions ──────────────
// Operator: platform (TalosClusterReconciler, UpgradePolicyReconciler).

const (
	// ConditionTypeVersionUpgradePending is set on InfrastructureTalosCluster when
	// spec.versionUpgrade=true is observed. Cleared after the generated UpgradePolicy
	// reaches Ready=True and status.observedTalosVersion is updated.
	// Terminal state: False (upgrade complete or not requested).
	// Operators: platform (TalosClusterReconciler).
	ConditionTypeVersionUpgradePending = "VersionUpgradePending"

	// ReasonVersionUpgradeRequested is set on VersionUpgradePending=True when
	// spec.versionUpgrade=true is first observed and validation passes.
	ReasonVersionUpgradeRequested = "VersionUpgradeRequested"

	// ReasonVersionUpgradeSubmitted is set on VersionUpgradePending=True after the
	// UpgradePolicy CR has been created by the TalosCluster reconciler.
	ReasonVersionUpgradeSubmitted = "VersionUpgradeSubmitted"

	// ReasonVersionUpgradeComplete is set on VersionUpgradePending=False after the
	// generated UpgradePolicy reaches Ready=True.
	ReasonVersionUpgradeComplete = "VersionUpgradeComplete"

	// ConditionTypeVersionRegressionBlocked is set on InfrastructureTalosCluster when
	// spec.talosVersion would regress the cluster below status.observedTalosVersion.
	// The reconciler does not downgrade and sets this condition until the spec is
	// corrected to match or exceed the current observed version.
	// Terminal state: False (spec corrected).
	// Operators: platform (TalosClusterReconciler).
	ConditionTypeVersionRegressionBlocked = "VersionRegressionBlocked"

	// ReasonVersionRegressionAttempted is set on VersionRegressionBlocked=True when
	// spec.talosVersion is lower than status.observedTalosVersion.
	ReasonVersionRegressionAttempted = "VersionRegressionAttempted"
)
