# seam-schema

API Group: `seam.ontai.dev`
Repository: `seam`
All agents read this document in full before touching any CRD or shared library type in this repository.

---

## 1. Domain boundary

seam is the cross-operator CRD registry for the ONT platform. It declares the four types that span operator boundaries and the shared library packages consumed by every operator. No reconciliation logic lives in this repository. No capability engine lives here.

seam.ontai.dev is not the group for operator-specific types. The division of ownership is:

- `seam.ontai.dev` -- LineageRecord, RunnerConfig, DriftSignal, SeamMembership (this repo)
- `platform.ontai.dev` -- TalosCluster, ClusterLog, and all day-2 operation CRDs (platform repo)
- `seam.ontai.dev` (dispatcher-owned) -- PackDelivery, PackExecution, PackInstalled, PackReceipt, PackLog (dispatcher repo)
- `guardian.ontai.dev` -- all guardian RBAC plane types (guardian repo)

SC-INV-001 -- seam owns all cross-operator CRD definitions under seam.ontai.dev. Reconcilers live in the operator repo.
SC-INV-002 -- All new CRD work goes directly into seam under seam.ontai.dev.
SC-INV-003 -- seam CRD manifests are installed before all operators.

---

## 2. Master GVK reference

All types below are under `seam.ontai.dev / v1alpha1`.

| Kind | Resource (plural) | Short name | Scope | Authoring principal | Reconciling operator |
|---|---|---|---|---|---|
| LineageRecord | lineagerecords | lr | Namespaced | LineageController only | LineageController (seam-owned, deferred) |
| RunnerConfig | runnerconfigs | rc | Namespaced | platform operator | conductor agent mode / exec mode |
| DriftSignal | driftsignals | ds | Namespaced | conductor role=tenant | conductor role=management |
| SeamMembership | seammemberships | sm | Namespaced | human / GitOps | guardian |

---

## 3. LineageRecord

### 3.1 Purpose

LineageRecord is the sealed causal chain index for a root declaration in the ONT platform. One LineageRecord is created per root declaration (currently: TalosCluster, PackDelivery) by the LineageController. All derived objects record derivation back to their root's LineageRecord. They do not each get their own record.

The record is controller-authored exclusively (root Decision 3). No human operator and no automation pipeline may create or modify a LineageRecord. The admission webhook enforces this.

`spec.descendantRegistry` grows monotonically. Entries are appended, never modified or removed (except by the retention enforcement loop after a staleness threshold is exceeded).

### 3.2 Naming convention

LineageRecord names are computed as `strings.ToLower(rootKind) + "-" + rootName`.

Examples: `taloscluster-ccs-mgmt`, `packdelivery-cilium-v1`

The helper `lineage.IndexName(kind, name)` from `pkg/lineage` produces this value. Operators that need to reference the LineageRecord for a given root use this function.

### 3.3 spec fields

**spec.rootBinding** (LineageRecordRootBinding, required, immutable after admission)

Records the root declaration that anchors this record. All subfields are immutable.

| Field | Type | Description |
|---|---|---|
| rootKind | string | Kind of the root declaration (e.g., TalosCluster, PackDelivery) |
| rootName | string | Name of the root declaration |
| rootNamespace | string | Namespace of the root declaration |
| rootUID | types.UID | UID of the root declaration at record creation time |
| rootObservedGeneration | int64 | metadata.generation of the root declaration at record creation |
| declaringPrincipal | string | Identity of the human or automation principal that applied the root CR. Stamped at CREATE by the admission webhook from the annotation `infrastructure.ontai.dev/declaring-principal`. Optional. |

**spec.domainRef** (string, optional)

References the DomainLineageIndex at `core.ontai.dev` that this LineageRecord instantiates.

**spec.descendantRegistry** ([]DescendantEntry, optional)

Append-only list of all objects derived from the root declaration.

Each DescendantEntry has:

| Field | Type | Description |
|---|---|---|
| group | string | API group of the derived object |
| version | string | API version of the derived object |
| kind | string | Kind of the derived object |
| name | string | Name of the derived object |
| namespace | string | Namespace of the derived object |
| uid | types.UID | UID of the derived object |
| seamOperator | string | Name of the Seam Operator that created this derived object |
| creationRationale | CreationRationale | Reason this derived object was created (see section 6) |
| rootGenerationAtCreation | int64 | metadata.generation of the root declaration at derived object creation time |
| createdAt | *metav1.Time | Optional. Time this entry was appended |
| actorRef | string | Optional. Identity propagated from rootBinding.declaringPrincipal |

**spec.policyBindingStatus** (*InfrastructurePolicyBindingStatus, optional)

Records the InfrastructurePolicy and InfrastructureProfile bound to the root declaration at last evaluation.

| Field | Type | Description |
|---|---|---|
| domainPolicyRef | string | Name of the InfrastructurePolicy bound at last evaluation |
| domainProfileRef | string | Name of the InfrastructureProfile bound at last evaluation |
| policyGenerationAtLastEvaluation | int64 | metadata.generation of the bound policy at last evaluation |
| driftDetected | bool | True if drift was detected at the last evaluation cycle |

**spec.outcomeRegistry** ([]OutcomeEntry, optional)

Append-only registry of terminal outcomes for derived objects. Entries are written by LineageController when a terminal condition is observed. Never modified or removed.

Each OutcomeEntry has:

| Field | Type | Description |
|---|---|---|
| derivedObjectUID | types.UID | UID matching a DescendantEntry in descendantRegistry |
| outcomeType | OutcomeType | Terminal classification: Succeeded, Failed, Drifted, or Superseded |
| outcomeTimestamp | metav1.Time | Time when the terminal condition was observed |
| outcomeRef | string | Optional. Name of the OperationResult or terminal condition reason |
| outcomeDetail | string | Optional. Brief human-readable summary of the outcome |

**spec.retentionPolicy** (*LineageRetentionPolicy, optional)

Declares garbage collection behavior.

| Field | Type | Default | Description |
|---|---|---|---|
| descendantRetentionDays | int32 | 30 | Days a stale descendant entry is retained after its object is confirmed not-found. Minimum 1. |
| deleteWithRoot | bool | true | When true, this LineageRecord is garbage collected when its root declaration is deleted |

### 3.4 status fields

| Field | Type | Description |
|---|---|---|
| observedGeneration | int64 | Last generation processed by the controller |
| conditions | []metav1.Condition | Standard Kubernetes condition array |

Condition types:

- `LineageSynced` -- set by the responsible reconciler to False with reason `LineageControllerAbsent` on first observation. LineageController sets it to True when it takes ownership. See section 7 Declaration 5.

### 3.5 Labels used by DescendantReconciler

Operators call `lineage.SetDescendantLabels` at derived object creation time to mark that object for the DescendantReconciler. The function writes five labels:

| Label key | Value |
|---|---|
| `infrastructure.ontai.dev/root-ili` | LineageRecord name (e.g., taloscluster-ccs-mgmt) |
| `infrastructure.ontai.dev/root-ili-namespace` | Namespace of the LineageRecord (may differ from derived object namespace) |
| `infrastructure.ontai.dev/seam-operator` | Canonical operator name (e.g., platform, dispatcher) |
| `infrastructure.ontai.dev/creation-rationale` | CreationRationale value |
| `infrastructure.ontai.dev/actor-ref` | Declaring principal, propagated from root annotation |

The cross-namespace case: when a derived object lives in a different namespace than its root (e.g., RunnerConfig in `ont-system`, LineageRecord in `seam-system`), the `root-ili-namespace` label carries the LineageRecord's namespace so the DescendantReconciler can resolve the reference correctly.

---

## 4. RunnerConfig

### 4.1 Purpose

RunnerConfig is the operator-generated operational contract for a Conductor agent on a specific cluster. It is created by the platform operator at runtime. It is never human-authored (INV-009). The platform operator is the sole author of RunnerConfig specs. The Conductor agent leader is the sole author of RunnerConfig status.

### 4.2 spec fields

| Field | Type | Description |
|---|---|---|
| clusterRef | string | Name of the TalosCluster this RunnerConfig is authoritative for |
| runnerImage | string | Fully qualified container image reference for the Conductor agent. Tag convention: `v{talosVersion}-r{revision}` for stable, `dev` or `dev-rc{N}` for development (INV-011, INV-012) |
| phases | []RunnerPhaseConfig | Optional. Ordered list of operational phases for this cluster's Conductor lifecycle |
| steps | []RunnerConfigStep | Optional. Ordered list of execution steps across all phases |
| operationalHistory | []RunnerOperationalHistoryEntry | Optional. Append-only record of completed RunnerConfig executions. Never truncated |
| maintenanceTargetNodes | []string | Optional. Node names that are the subject of the operation |
| operatorLeaderNode | string | Optional. Node hosting the leader pod of the initiating operator |
| selfOperation | bool | Optional. True when the Job's execution cluster and the target cluster are the same |

**RunnerPhaseConfig** fields:

| Field | Type | Description |
|---|---|---|
| name | string | Phase identifier |
| parameters | map[string]string | Optional. Phase-specific key-value configuration |

**RunnerConfigStep** fields:

| Field | Type | Description |
|---|---|---|
| name | string | Unique identifier for this step within the RunnerConfig |
| capability | string | Named Conductor capability to invoke for this step |
| parameters | map[string]string | Optional. Input parameter map passed to the capability at Job materialisation time |
| dependsOn | string | Optional. Name of a prior step that must complete before this step begins |
| haltOnFailure | bool | Optional. When true, failure of this step terminates the RunnerConfig with no further steps executing |

**RunnerOperationalHistoryEntry** fields:

| Field | Type | Description |
|---|---|---|
| appliedAt | metav1.Time | Time this change was applied |
| concern | string | What aspect of configuration changed |
| previousValue | string | Optional. Value before the change |
| newValue | string | Value after the change |
| appliedBy | string | Identity of who applied the change |

### 4.3 status fields

Status is written exclusively by the Conductor agent leader.

| Field | Type | Description |
|---|---|---|
| capabilities | []RunnerCapabilityEntry | Optional. Self-declared capability manifest emitted by the Conductor agent on startup |
| agentVersion | string | Optional. Version string of the Conductor agent binary currently running |
| agentLeader | string | Optional. Pod name of the current Conductor agent leader |
| phase | string | Optional. Terminal execution phase written by Conductor exec mode. "Completed" means all steps succeeded. "Failed" means at least one step failed. Empty means execution is in progress |
| failedStep | string | Optional. Name of the first step that reached the Failed phase. Present only when phase="Failed" |
| stepResults | []RunnerConfigStepResult | Optional. Ordered list of step result records written by Conductor exec mode |
| conditions | []metav1.Condition | Optional. Standard Kubernetes condition list |

**RunnerCapabilityEntry** fields:

| Field | Type | Description |
|---|---|---|
| name | string | Capability name (e.g., pack-deploy, talos-upgrade) |
| version | string | Capability version declared by the agent |
| description | string | Optional. Human-readable description |

**RunnerConfigStepResult** fields:

| Field | Type | Description |
|---|---|---|
| name | string | Matches Name in the corresponding RunnerConfigStep |
| status | RunnerStepResultPhase | Terminal status: Succeeded, Failed, or Skipped |
| startedAt | *metav1.Time | Optional. Time this step began execution |
| completedAt | *metav1.Time | Optional. Time this step finished execution |
| message | string | Optional. Additional context about the step outcome |

Platform operators watch `status.phase` to detect terminal conditions without scanning `status.stepResults`.

---

## 5. DriftSignal

### 5.1 Purpose

DriftSignal is the three-state acknowledgement chain for drift events detected by Conductor role=tenant on a managed cluster. Conductor role=tenant writes the signal when it observes drift on a TalosCluster or PackDelivery CR. Conductor role=management reads the signal and orchestrates a corrective job. At-least-once delivery applies across federation retries.

Conductor detects drift only for TalosCluster and PackDelivery. It writes the drift reason to the CR status, signals via DriftSignal, and never remediates directly (root Decision 14).

### 5.2 State machine

| State | Set by | Meaning |
|---|---|---|
| pending | conductor role=tenant | Signal written; not yet delivered to management cluster |
| delivered | federation channel | Signal transmitted to management cluster. At-least-once delivery |
| queued | conductor role=management | Corrective job accepted and enqueued; not yet complete |
| confirmed | conductor role=management | Corrective job completed; resolution acknowledged. Terminal state |

### 5.3 Escalation protocol

`spec.escalationCounter` is incremented by conductor role=tenant on each re-emit cycle when no acknowledgement has been received. When the counter reaches the configurable escalation threshold, conductor writes a `type=TerminalDrift` condition on the affected CR and stops re-emitting. Human intervention is required at that point.

### 5.4 spec fields

| Field | Type | Description |
|---|---|---|
| state | DriftSignalState | Current acknowledgement state: pending, delivered, queued, or confirmed |
| correlationID | string | UUID v4. Unique identifier for this drift event; used to deduplicate signals across federation retries |
| observedAt | metav1.Time | Time the drift was first observed by conductor role=tenant |
| affectedCRRef | DriftAffectedCRRef | Typed reference to the CR that exhibited drift |
| driftReason | string | Human-readable description of why drift was detected |
| correctionJobRef | string | Optional. Name of the corrective Job created by the management cluster. Set when state transitions to queued |
| escalationCounter | int32 | Optional. Number of times this signal has been re-emitted without acknowledgement |

**DriftAffectedCRRef** fields:

| Field | Type | Description |
|---|---|---|
| group | string | API group of the drifted CR |
| kind | string | Kind of the drifted CR |
| namespace | string | Optional. Namespace of the drifted CR. Empty for cluster-scoped resources |
| name | string | Name of the drifted CR |

### 5.5 status fields

Status is written by conductor role=management.

| Field | Type | Description |
|---|---|---|
| observedGeneration | int64 | Optional. Generation most recently reconciled |
| conditions | []metav1.Condition | Optional. Standard Kubernetes condition list |

### 5.6 Print columns

`kubectl get ds` shows: State, CorrelationID, Age.

---

## 6. SeamMembership

### 6.1 Purpose

SeamMembership is the formal join declaration for an operator wishing to become a member of the ONT platform operator family. An operator applies a SeamMembership CR on startup. Guardian validates the membership by verifying the referenced RBACProfile exists and has reached `provisioned=true`, and that the `domainIdentityRef` matches. Operators that are not admitted members may not receive PermissionSnapshots.

### 6.2 spec fields

| Field | Type | Description |
|---|---|---|
| appIdentityRef | string | The operator's application-layer identity. For current operators this is the operator's service account name |
| domainIdentityRef | string | References the DomainIdentity at `core.ontai.dev`. Must match the `domainIdentityRef` on the operator's RBACProfile |
| principalRef | string | Kubernetes service account that this operator runs as. Format: `system:serviceaccount:{namespace}:{name}`. Must match the `principalRef` on the operator's RBACProfile |
| tier | string | Membership tier. Valid values: `infrastructure` (Seam family operators), `application` (application operators) |

### 6.3 status fields

Status is written by guardian.

| Field | Type | Description |
|---|---|---|
| admitted | bool | Optional. True when Guardian has validated and admitted this member |
| admittedAt | *metav1.Time | Optional. Timestamp when Guardian admitted this member |
| permissionSnapshotRef | string | Optional. Name of the PermissionSnapshot Guardian resolved for this member. Set after admission |
| conditions | []metav1.Condition | Optional. Standard Kubernetes condition list |

Condition types:

- `Admitted` -- True when Guardian has validated and admitted this member (all validation passes)
- `Validated` -- True when structural and cross-reference validation passes (domainIdentityRef matches RBACProfile, RBACProfile is provisioned)

Condition reasons:

| Reason | Condition | Description |
|---|---|---|
| MembershipAdmitted | Admitted=True | All validation passed |
| DomainIdentityMismatch | Admitted=False | spec.domainIdentityRef does not match the RBACProfile |
| PrincipalMismatch | Admitted=False | spec.principalRef does not match any RBACProfile principalRef |
| RBACProfileNotProvisioned | Admitted=False | Matching RBACProfile has not reached provisioned=true. Requeues |

### 6.4 Print columns

`kubectl get sm` shows: Admitted, Tier, Age.

---

## 7. CreationRationale enumeration

`pkg/lineage.CreationRationale` is the compile-time controlled vocabulary for why a derived object was created. Every `SealedCausalChain` carries exactly one value from this enumeration. It is not a free-text field and is not a per-operator registry.

New values require a PR to this repository and Platform Governor review. No operator may extend the enumeration unilaterally (root Decision 5).

| Value | Set by | Trigger |
|---|---|---|
| ClusterProvision | platform | A TalosCluster or related cluster lifecycle root declaration is created |
| ClusterDecommission | platform | A cluster decommission root declaration (e.g., ClusterReset) is created |
| SecurityEnforcement | guardian | A security plane root declaration (e.g., RBACPolicy, PermissionSet) causes a derived object |
| PackExecution | dispatcher | A pack delivery or execution root declaration (e.g., PackDelivery, PackExecution) is created |
| VirtualizationFulfillment | screen (future) | A virtualization workload root declaration is created. NOT IMPLEMENTED |
| ConductorAssignment | conductor agent mode | An operational assignment object is created by the management cluster Conductor |
| VortexBinding | vortex (future) | A portal policy binding root declaration is created. NOT IMPLEMENTED |

---

## 8. SealedCausalChain type

`pkg/lineage.SealedCausalChain` is the immutable causal chain field embedded in every Seam-managed CRD spec. All operators embed this type by reference rather than redefining its fields.

Immutability contract: this field is authored once at object creation time and is sealed permanently. The admission webhook rejects any update that modifies any field within SealedCausalChain after the object has been created. No controller, human, or pipeline may alter this field post-admission.

| Field | Type | Description |
|---|---|---|
| rootKind | string | Kind of the root declaration that caused this object to exist |
| rootName | string | Name of the root declaration |
| rootNamespace | string | Namespace of the root declaration |
| rootUID | types.UID | UID of the root declaration at creation time. Detects root replacement |
| creatingOperator | OperatorIdentity | Seam Operator that created this object |
| creationRationale | CreationRationale | Reason from the controlled vocabulary in section 7 |
| rootGenerationAtCreation | int64 | metadata.generation of the root declaration at creation time. Temporal anchor |

**OperatorIdentity** subtype:

| Field | Type | Description |
|---|---|---|
| name | string | Canonical operator name (e.g., platform, guardian, dispatcher, conductor) |
| version | string | Deployed version of the operator at creation time (e.g., v1.26.5-r3). Enables audit correlation |

---

## 9. pkg/conditions vocabulary

`pkg/conditions` is the single source of truth for all condition type and reason string constants used across guardian, platform, dispatcher, and conductor. Operators import these constants and do not declare local string aliases.

### Cross-operator conditions (all operators)

| Constant | String value | Used by |
|---|---|---|
| ConditionTypeLineageSynced | `LineageSynced` | All root-declaration reconcilers |
| ReasonLineageControllerAbsent | `LineageControllerAbsent` | All root-declaration reconcilers |

LineageSynced lifecycle protocol (Declaration 5):

1. On first observation, the responsible reconciler sets LineageSynced to False with reason LineageControllerAbsent. One-time write.
2. LineageController takes ownership on deployment and sets LineageSynced to True.
3. If LineageController is absent, the condition remains False indefinitely. This is the expected steady state during the stub phase.

Terminal state: True (set by LineageController, not by the operator reconciler).

The full condition vocabulary is in `pkg/conditions/conditions.go`. Operator-specific sections cover guardian, platform, and dispatcher condition types.

---

## 10. Controllers

### LineageController

- Status: deferred implementation milestone (root Decision 6)
- Manages the lifecycle of LineageRecord CRs
- Is the sole principal permitted to create or update LineageRecord instances (root Decision 3)
- Watches root declaration GVKs (CRDs labelled `infrastructure.ontai.dev/lineage-root=true`)
- Appends DescendantEntry records as derived objects appear
- Writes OutcomeEntry records when terminal conditions are observed on derived objects
- Sets LineageSynced=True on root declarations it manages

### DescendantReconciler

- Watches derived objects that carry the `infrastructure.ontai.dev/root-ili` label
- Reads the label set written by `lineage.SetDescendantLabels` to locate the LineageRecord
- Appends the derived object to `spec.descendantRegistry` of the referenced LineageRecord
- Cross-namespace label: when the derived object is in a different namespace from the LineageRecord, uses `infrastructure.ontai.dev/root-ili-namespace` to resolve the reference

---

## 11. Lineage provision standards

These declarations define the platform-wide contract for how lineage fields are populated and who writes them. They apply to every operator in the ONT stack.

**Declaration 1 -- One LineageRecord per root declaration.**
TalosCluster or PackDelivery creation produces exactly one LineageRecord anchored to that root. All derived objects record derivation back to the anchor. Records are not replicated per derived object.

**Declaration 2 -- spec.lineage is immutable after admission.**
The SealedCausalChain embedded in each operator CRD spec is typed, immutable after the admission webhook processes the CREATE request, and carries creation context. Annotations are advisory. Spec fields are contractual.

**Declaration 3 -- LineageRecord instances are controller-authored exclusively.**
No human or pipeline writes a LineageRecord. Only the owning controller creates it. The admission webhook enforces this.

**Declaration 4 -- Lineage field introduced at stub phase.**
spec.lineage and the SealedCausalChain construction function are authored when types are first stubbed. Reconcilers proceed without LineageController being deployed. LineageController is a deferred milestone.

**Declaration 5 -- LineageSynced is the reserved cross-operator condition.**
Every root declaration reconciler sets LineageSynced=False with reason LineageControllerAbsent on first observation. LineageController sets it True when deployed. Steady state without LineageController is LineageSynced=False.

**Declaration 6 -- actorRef is propagated from the declaring-principal annotation.**
The admission webhook stamps `infrastructure.ontai.dev/declaring-principal` on root declaration CRDs at CREATE time. Operators read this annotation from the root object and pass it as `actorRef` to `lineage.SetDescendantLabels` at derived object creation. Pass empty string when the annotation is absent.

---

## 12. Deferred implementations

The following capabilities are designed and specified but not yet implemented:

- LineageController -- manages LineageRecord lifecycle, appends descendant entries, writes outcome entries. Declaration 4.
- VirtualizationFulfillment rationale -- reserved for the Screen operator. INV-021: no Screen implementation until Governor-approved ADR.
- VortexBinding rationale -- reserved for the Vortex operator (future).
- `spec.domainRef` on LineageRecord -- references a DomainLineageIndex at `core.ontai.dev`. The `core.ontai.dev` group is not yet implemented.
