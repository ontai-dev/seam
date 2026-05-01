# seam-core-schema
> API Group: infrastructure.ontai.dev
> Repository: seam-core
> Layer: Seam Core - infrastructure domain instantiation of core.ontai.dev
> All agents read this before touching any seam-core CRD or shared library type.

---

## 1. Domain Boundary

Seam Core is the CRD registry for the Seam platform. It owns cross-operator CRD
definitions that no single operator owns. No reconciliation logic lives here.
No capability engine lives here. Seam Core installs CRD definitions and runs
three controllers: LineageReconciler, DescendantReconciler, and DSNSReconciler.

**SC-INV-001** - seam-core owns all cross-operator CRD definitions. Reconcilers live in the operator repo.
**SC-INV-002** - Phase 2B migration complete (2026-04-25). All types migrated to infrastructure.ontai.dev. Old groups runner.ontai.dev and infra.ontai.dev are superseded.
**SC-INV-003** - seam-core installs before all operators.

---

## 2. Master GVK Reference

All types below are under `infrastructure.ontai.dev / v1alpha1`.

| Kind                        | Resource (plural)                   | Short | Scope      | Authoring operator  | Reconciling operator | Migrated from              |
|-----------------------------|-------------------------------------|-------|------------|---------------------|----------------------|----------------------------|
| InfrastructureLineageIndex  | infrastructurelineageindices        | ili   | Namespaced | seam-core (LC)      | seam-core (LC)       | --                         |
| InfrastructureRunnerConfig  | infrastructurerunnerconfigs         | irc   | Namespaced | platform            | conductor (agent/exec)| runner.ontai.dev/RunnerConfig |
| InfrastructureTalosCluster  | infrastructuretalosclusters         | itc   | Namespaced | human / GitOps      | platform             | platform.ontai.dev/TalosCluster |
| InfrastructureClusterPack   | infrastructureclusterpacks          | icp   | Namespaced | human / GitOps      | wrapper              | infra.ontai.dev/ClusterPack   |
| InfrastructurePackExecution | infrastructurepackexecutions        | ipe   | Namespaced | human / GitOps      | wrapper              | infra.ontai.dev/PackExecution |
| InfrastructurePackInstance  | infrastructurepackinstances         | ipi   | Namespaced | wrapper             | wrapper              | infra.ontai.dev/PackInstance  |
| InfrastructurePackBuild     | infrastructurepackbuilds            | ipb   | Namespaced | human / GitOps      | compiler (offline)   | infra.ontai.dev/PackBuild     |
| InfrastructurePackReceipt   | infrastructurepackreceipts          | ipr   | Namespaced | conductor (agent)   | conductor (agent)    | runner.ontai.dev/PackReceipt  |
| PackOperationResult         | packoperationresults                | por   | Namespaced | conductor (execute) | wrapper (reader)     | --                           |
| DriftSignal                 | driftsignals                        | ds    | Namespaced | conductor (tenant)  | conductor (mgmt)     | --                           |
| SeamMembership              | seammemberships                     | sm    | Namespaced | human / GitOps      | guardian             | --                           |

**ILI naming convention:** `strings.ToLower(kind) + "-" + name`
Examples: `infrastructuretaloscluster-ccs-mgmt`, `infrastructurepackexecution-exec-001`

**CAPI types** (separate API group, not owned by seam-core):

| Kind                            | Group                              | Reconciling operator |
|---------------------------------|------------------------------------|----------------------|
| SeamInfrastructureCluster       | infrastructure.cluster.x-k8s.io   | platform (CAPI)      |
| SeamInfrastructureMachine       | infrastructure.cluster.x-k8s.io   | platform (CAPI)      |
| SeamInfrastructureMachineTemplate | infrastructure.cluster.x-k8s.io | platform (CAPI)      |

**Seam-core controller GVK watch lists** (determines which GVKs cause reconciler invocations):

- `RootDeclarationGVKs` (LineageReconciler): InfrastructureTalosCluster, InfrastructureClusterPack, InfrastructurePackExecution, InfrastructurePackInstance, RBACPolicy, RBACProfile, IdentityBinding, IdentityProvider, PermissionSet
- `DerivedObjectGVKs` (DescendantReconciler): InfrastructureRunnerConfig, InfrastructurePackInstance
- `DSNSGVKs` (DSNSReconciler): InfrastructureTalosCluster, InfrastructurePackInstance, IdentityBinding, IdentityProvider, InfrastructureRunnerConfig

---

## 3. InfrastructureLineageIndex

### 3.1 Purpose

`InfrastructureLineageIndex` is the concrete sealed causal chain index for all
objects managed by the Seam platform in the infrastructure domain. One instance
is created per root declaration (TalosCluster, PackExecution, etc.) by the
LineageReconciler. All derived objects carry a reference to their root
declaration's `InfrastructureLineageIndex`. They do not carry their own index
instances.

The index grows monotonically as new derived objects are created. Entries in
`spec.descendantRegistry` are never modified or removed.

### 3.2 CRD Stub

```
# STUB - infrastructure.ontai.dev/v1alpha1 InfrastructureLineageIndex
# Seam Core infrastructure domain instantiation of core.ontai.dev DomainLineageIndex.
# controller-gen not yet wired for seam-core. Hand-authored stub pending wiring.
---
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: infrastructurelineageindices.infrastructure.ontai.dev
  annotations:
    ontai.dev/layer: "seam-core"
    ontai.dev/status: "stub"
    ontai.dev/instantiates: "core.ontai.dev/DomainLineageIndex"
spec:
  group: infrastructure.ontai.dev
  names:
    kind: InfrastructureLineageIndex
    listKind: InfrastructureLineageIndexList
    plural: infrastructurelineageindices
    singular: infrastructurelineageindex
    shortNames:
      - ili
  scope: Namespaced
  versions:
    - name: v1alpha1
      served: true
      storage: true
      subresources:
        status: {}
      schema:
        openAPIV3Schema:
          type: object
          properties:
            spec:
              type: object
              required: [rootBinding]
              properties:
                rootBinding:
                  type: object
                  description: >
                    Identifies the root declaration that anchors this lineage index.
                    Immutable after creation. Admission webhook rejects any update
                    that modifies a field in this section.
                  required: [rootKind, rootName, rootNamespace, rootUID, rootObservedGeneration]
                  properties:
                    rootKind:
                      type: string
                    rootName:
                      type: string
                    rootNamespace:
                      type: string
                    rootUID:
                      type: string
                    rootObservedGeneration:
                      type: integer
                      format: int64
                    declaringPrincipal:
                      type: string
                      description: >
                        Identity of the human operator or automation principal that
                        applied the root declaration CR. Stamped by the admission
                        webhook via annotation infrastructure.ontai.dev/declaring-principal
                        at object creation time. Read by LineageController when creating
                        the ILI. Immutable after rootBinding is sealed.
                descendantRegistry:
                  type: array
                  description: >
                    Registry of all objects derived from the root declaration.
                    Appended monotonically. Entries are never modified or removed.
                  items:
                    type: object
                    required: [kind, name, namespace, uid, seamOperator, creationRationale, rootGenerationAtCreation]
                    properties:
                      kind:
                        type: string
                      name:
                        type: string
                      namespace:
                        type: string
                      uid:
                        type: string
                      seamOperator:
                        type: string
                        description: Name of the Seam Operator that created this derived object.
                      creationRationale:
                        type: string
                        description: >
                          Reason this derived object was created. Constrained to
                          the pkg/lineage.CreationRationale enumeration.
                        enum:
                          - ClusterProvision
                          - ClusterDecommission
                          - SecurityEnforcement
                          - PackExecution
                          - VirtualizationFulfillment
                          - ConductorAssignment
                          - VortexBinding
                      rootGenerationAtCreation:
                        type: integer
                        format: int64
                      createdAt:
                        type: string
                        format: date-time
                        description: >
                          Timestamp when DescendantReconciler appended this entry.
                          Set to the reconciler's current time at append. Immutable.
                      actorRef:
                        type: string
                        description: >
                          Identity propagated from rootBinding.declaringPrincipal.
                          Every derived object entry carries the initiating human
                          principal from the root of its causal chain. Immutable.
                policyBindingStatus:
                  type: object
                  description: >
                    Records the InfrastructurePolicy and InfrastructureProfile
                    bound to the root declaration at last evaluation.
                  properties:
                    domainPolicyRef:
                      type: string
                      description: Name of the InfrastructurePolicy bound to the root declaration.
                    domainProfileRef:
                      type: string
                      description: Name of the InfrastructureProfile bound to the root declaration.
                    policyGenerationAtLastEvaluation:
                      type: integer
                      format: int64
                      description: Generation of the InfrastructurePolicy at last evaluation.
                    driftDetected:
                      type: boolean
                      description: True if drift detected between expected and observed state.
                outcomeRegistry:
                  type: array
                  description: >
                    Append-only registry of terminal outcomes for derived objects
                    tracked in descendantRegistry. Entries are appended by
                    LineageController when a terminal condition is observed on a
                    tracked derived object. Entries are never modified or removed.
                  items:
                    type: object
                    required: [derivedObjectUID, outcomeType, outcomeTimestamp]
                    properties:
                      derivedObjectUID:
                        type: string
                        description: UID matching a derivedObject entry in descendantRegistry.
                      outcomeType:
                        type: string
                        enum: [Succeeded, Failed, Drifted, Superseded]
                        description: Terminal classification of the derived object lifecycle.
                      outcomeTimestamp:
                        type: string
                        format: date-time
                        description: Time when the terminal condition was observed.
                      outcomeRef:
                        type: string
                        description: >
                          Name of the OperationResult ConfigMap or terminal condition
                          reason that produced this outcome classification. Optional.
                      outcomeDetail:
                        type: string
                        description: >
                          Brief human-readable summary of the outcome. Written by
                          LineageController from the terminal condition message.
                          Optional.
            status:
              type: object
              properties:
                conditions:
                  type: array
                  items:
                    type: object
                observedGeneration:
                  type: integer
                  format: int64
```

### 3.3 Field Reference

#### spec.rootBinding (immutable after admission)

| Field                  | Type   | Required | Description                                                                           |
|------------------------|--------|----------|---------------------------------------------------------------------------------------|
| rootKind               | string | yes      | Kind of the root declaration                                                          |
| rootName               | string | yes      | Name of the root declaration                                                          |
| rootNamespace          | string | yes      | Namespace of the root declaration                                                     |
| rootUID                | string | yes      | UID of the root declaration at index creation time                                    |
| rootObservedGeneration | int64  | yes      | Root declaration generation when this index was created                               |
| declaringPrincipal     | string | no       | Identity of the principal that applied the root declaration CR. Stamped at CREATE time. Immutable. |

#### spec.descendantRegistry[]

| Field                    | Type      | Required | Description                                                            |
|--------------------------|-----------|----------|------------------------------------------------------------------------|
| group                    | string    | yes      | API group of the derived object                                        |
| version                  | string    | yes      | API version of the derived object                                      |
| kind                     | string    | yes      | Kind of the derived object                                             |
| name                     | string    | yes      | Name of the derived object                                             |
| namespace                | string    | yes      | Namespace of the derived object                                        |
| uid                      | string    | yes      | UID of the derived object                                              |
| seamOperator             | string    | yes      | Seam Operator that created this derived object                         |
| creationRationale        | string    | yes      | Value from `pkg/lineage.CreationRationale` enum                        |
| rootGenerationAtCreation | int64     | yes      | Root declaration generation when derived object was created            |
| createdAt                | date-time | no       | Timestamp when DescendantReconciler appended this entry. Immutable.    |
| actorRef                 | string    | no       | Identity propagated from rootBinding.declaringPrincipal. Immutable.    |

#### spec.policyBindingStatus

| Field                            | Type    | Required | Description                                                          |
|----------------------------------|---------|----------|----------------------------------------------------------------------|
| domainPolicyRef                  | string  | no       | Name of the bound InfrastructurePolicy                               |
| domainProfileRef                 | string  | no       | Name of the bound InfrastructureProfile                              |
| policyGenerationAtLastEvaluation | int64   | no       | InfrastructurePolicy generation at last evaluation                   |
| driftDetected                    | boolean | no       | True if drift detected at last evaluation                            |

#### spec.outcomeRegistry[]

| Field            | Type      | Required | Description                                                                          |
|------------------|-----------|----------|--------------------------------------------------------------------------------------|
| derivedObjectUID | string    | yes      | UID matching a derivedObject entry in descendantRegistry                             |
| outcomeType      | string    | yes      | Terminal classification: Succeeded, Failed, Drifted, or Superseded                  |
| outcomeTimestamp | date-time | yes      | Time when the terminal condition was observed                                        |
| outcomeRef       | string    | no       | Name of the OperationResult ConfigMap or terminal condition reason. Optional.        |
| outcomeDetail    | string    | no       | Brief human-readable summary written by LineageController. Optional.                 |

Entries are appended monotonically. No entry is ever updated or removed. An
outcomeRegistry entry for a given derivedObjectUID supersedes but does not replace
its corresponding descendantRegistry entry. Both records are permanent.

---

## 4. Creation Rationale Enumeration

Defined in `seam-core/pkg/lineage/rationale.go`. This is a compile-time
`CreationRationale string` type with a controlled vocabulary. All Seam Operators
import this package when populating `SealedCausalChain.CreationRationale`.

New values require a Pull Request to seam-core and Platform Governor review.
Operators do not extend this vocabulary unilaterally.

| Value                    | Operator(s)                  | Meaning                                                           |
|--------------------------|------------------------------|-------------------------------------------------------------------|
| ClusterProvision         | Platform                     | A cluster lifecycle root declaration was created                  |
| ClusterDecommission      | Platform                     | A cluster decommission root declaration was created               |
| SecurityEnforcement      | Guardian                     | A security plane declaration was created                          |
| PackExecution            | Wrapper                      | A pack delivery or execution root declaration was created         |
| VirtualizationFulfillment| Screen (future)              | A virtualization workload root declaration was created            |
| ConductorAssignment      | Conductor (agent mode)       | An operational assignment was created by the Conductor agent      |
| VortexBinding            | Vortex (future)              | A portal policy binding was created                               |

---

## 5. SealedCausalChain Field Type

Defined in `seam-core/pkg/lineage/chain.go`. This is the Go struct that every
Seam-managed CRD embeds in its spec. It is authored once at creation time and
sealed at admission. The admission webhook rejects any update request that
modifies this field after the object is created.

| Field                    | Type                         | Description                                                       |
|--------------------------|------------------------------|-------------------------------------------------------------------|
| rootKind                 | string                       | Kind of the root declaration that caused this object to exist     |
| rootName                 | string                       | Name of the root declaration                                      |
| rootNamespace            | string                       | Namespace of the root declaration                                 |
| rootUID                  | types.UID                    | UID of the root declaration at time of this object's creation     |
| creatingOperator         | OperatorIdentity             | Seam Operator name and version that created this object           |
| creationRationale        | lineage.CreationRationale    | Reason from the controlled vocabulary                             |
| rootGenerationAtCreation | int64                        | Root declaration generation at time this object was created       |

`OperatorIdentity` has two fields: `name` (string) and `version` (string).

---

## 6. Derivation from Domain Core

`InfrastructureLineageIndex` instantiates `DomainLineageIndex` from `core.ontai.dev`
per the domain-core-schema.md instantiation contract (§3). The instantiation rules
applied in this domain are:

| Constraint                        | Domain Core (abstract)                  | Seam Core (infrastructure instantiation)               |
|-----------------------------------|-----------------------------------------|--------------------------------------------------------|
| API group                         | core.ontai.dev                          | infrastructure.ontai.dev                               |
| creationRationale constraint      | unconstrained string                    | enum - `pkg/lineage.CreationRationale` values          |
| domainPolicyRef                   | string (abstract)                       | Name of an `InfrastructurePolicy` CR                   |
| domainProfileRef                  | string (abstract)                       | Name of an `InfrastructureProfile` CR                  |
| rootBinding fields                | as defined - unmodified                 | as defined - unmodified                                |
| Lineage Index Pattern             | one index per root declaration          | one index per root declaration - unchanged             |
| Authorship rule                   | controller-authored exclusively         | controller-authored exclusively - unchanged            |
| Immutability rule                 | rootBinding sealed at admission         | rootBinding sealed at admission - unchanged            |

---

## 7. Per-CRD Field Reference

### 7.1 InfrastructureRunnerConfig

**Short name:** irc | **Scope:** Namespaced | **Authored by:** platform | **Reconciled by:** conductor

Operator-generated operational contract for a specific cluster. Never human-authored.
INV-009, INV-010. Phase-2B migration from `runner.ontai.dev/RunnerConfig`.

#### spec

| Field                  | Type                      | Required | Description                                                               |
|------------------------|---------------------------|----------|---------------------------------------------------------------------------|
| clusterRef             | string                    | yes      | Name of the TalosCluster this RunnerConfig is authoritative for           |
| runnerImage            | string                    | yes      | Fully qualified container image reference for Conductor agent. INV-011.   |
| phases                 | RunnerPhaseConfig[]       | no       | Ordered list of operational phases for Conductor lifecycle                |
| steps                  | RunnerConfigStep[]        | no       | Ordered list of execution steps across all phases                         |
| operationalHistory     | RunnerOperationalHistoryEntry[] | no | Append-only record of completed RunnerConfig executions                  |
| maintenanceTargetNodes | string[]                  | no       | Node names that are the subject of the operation                          |
| operatorLeaderNode     | string                    | no       | Node hosting the leader pod of the initiating operator                    |
| selfOperation          | bool                      | no       | True when execution cluster and target cluster are the same               |

**RunnerConfigStep fields:** name, capability, parameters (map), dependsOn (string), haltOnFailure (bool)

**RunnerPhaseConfig fields:** name, parameters (map)

#### status (written by Conductor agent leader exclusively)

| Field          | Type                    | Description                                                                        |
|----------------|-------------------------|------------------------------------------------------------------------------------|
| capabilities   | RunnerCapabilityEntry[] | Self-declared capability manifest emitted by Conductor agent on startup            |
| agentVersion   | string                  | Version string of the running Conductor agent binary                               |
| agentLeader    | string                  | Pod name of the current Conductor agent leader                                     |
| phase          | string                  | Terminal execution phase: Completed or Failed. Empty when in progress.             |
| failedStep     | string                  | Name of the first step that reached Failed. Present only when phase=Failed.        |
| stepResults    | RunnerConfigStepResult[]| Ordered step result records written by Conductor execute mode                      |
| conditions     | metav1.Condition[]      | Standard Kubernetes condition list                                                 |

---

### 7.2 InfrastructureTalosCluster

**Short name:** itc | **Scope:** Namespaced | **Authored by:** human / GitOps | **Reconciled by:** platform

Root declaration for every cluster under Seam governance. One instance per cluster.
Phase-2B migration from `platform.ontai.dev/TalosCluster`. Decision H applies (deletion cascade order).

#### spec

| Field                  | Type                          | Required | Description                                                                 |
|------------------------|-------------------------------|----------|-----------------------------------------------------------------------------|
| mode                   | string (enum)                 | yes      | `bootstrap` or `import`. bootstrap = new cluster, import = existing cluster |
| role                   | string (enum)                 | no*      | `management` or `tenant`. Mandatory on mode=import.                         |
| talosVersion           | string                        | no       | Talos OS version. Used by Conductor for compatible runner image. INV-012.    |
| clusterEndpoint        | string                        | no       | Cluster VIP or primary API endpoint IP. Required on mode=import.            |
| nodeAddresses          | string[]                      | no       | Node IPs. Used by DSNSReconciler to populate A records.                     |
| capi                   | InfrastructureCAPIConfig      | no       | CAPI integration settings. Absent = direct bootstrap.                       |
| infrastructureProvider | string (enum)                 | no       | `native`, `capi`, or `screen` (reserved). Default: native.                  |
| kubeconfigSecretRef    | string                        | no       | Secret name containing kubeconfig. Required on mode=import.                 |
| talosconfigSecretRef   | string                        | no       | Secret name containing talosconfig.                                         |
| lineage                | lineage.SealedCausalChain     | no       | Sealed causal chain record. Immutable after creation.                       |

**InfrastructureCAPIConfig fields:** enabled (bool), talosVersion, kubernetesVersion, controlPlane (replicas), workers ([]InfrastructureCAPIWorkerPool), ciliumPackRef

**Deletion semantics (Decision H):** mode=bootstrap triggers permanent decommission. mode=import triggers severance only (cluster continues to exist unmanaged).

#### status

| Field           | Type                  | Description                                                              |
|-----------------|-----------------------|--------------------------------------------------------------------------|
| observedGeneration | int64              | Generation most recently reconciled                                      |
| origin          | string (enum)         | `bootstrapped` or `imported`                                             |
| capiClusterRef  | InfrastructureLocalObjectRef | Reference to the owned CAPI Cluster object. Only for CAPI clusters. |
| conditions      | metav1.Condition[]    | Status conditions including `Ready` and `LineageSynced`                  |

---

### 7.3 InfrastructureClusterPack

**Short name:** icp | **Scope:** Namespaced | **Authored by:** human / GitOps | **Reconciled by:** wrapper

Records a compiled OCI artifact that is ready for runtime delivery. Spec is immutable
after creation. Phase-2B migration from `infra.ontai.dev/ClusterPack`.

#### spec

| Field              | Type                              | Required | Description                                                          |
|--------------------|-----------------------------------|----------|----------------------------------------------------------------------|
| version            | string                            | yes      | Semantic version of this pack. Immutable.                            |
| registryRef        | InfrastructurePackRegistryRef     | yes      | OCI registry URL and digest. Immutable.                              |
| checksum           | string                            | no       | Content-addressed checksum of the full artifact manifest set         |
| rbacDigest         | string                            | no       | OCI digest of the RBAC layer (SA, Role, ClusterRole, Bindings)       |
| workloadDigest     | string                            | no       | OCI digest of the workload layer. Applied after RBACProfile ready.   |
| clusterScopedDigest| string                            | no       | OCI digest of cluster-scoped non-RBAC layer                          |
| sourceBuildRef     | string                            | no       | Opaque reference to the build that produced this pack                |
| executionOrder     | InfrastructurePackExecutionStage[]| no       | Ordered stages: rbac, storage, stateful, stateless                   |
| provenance         | InfrastructurePackProvenance      | no       | Build-time metadata: buildID, buildTimestamp, sourceRef              |
| basePackName       | string                            | no       | Logical pack name shared across versions (e.g., nginx-ingress)       |
| targetClusters     | string[]                          | no       | Cluster names to which this pack should be delivered                 |
| chartVersion       | string                            | no       | Helm chart version used to compile this pack                         |
| chartURL           | string                            | no       | Helm chart repository URL                                            |
| chartName          | string                            | no       | Helm chart name                                                      |
| helmVersion        | string                            | no       | Helm SDK version                                                     |
| valuesFile         | string                            | no       | Path to the values file used during pack compilation                 |
| lifecyclePolicies  | InfrastructureLifecyclePolicy     | no       | retainOnDeletion (bool, default true)                                |
| lineage            | lineage.SealedCausalChain         | no       | Sealed causal chain record. Immutable after creation.                |

#### status

| Field              | Type               | Description                                                              |
|--------------------|--------------------|--------------------------------------------------------------------------|
| signed             | bool               | Whether the conductor signing loop has signed this pack                  |
| packSignature      | string             | Base64-encoded Ed25519 signature from management cluster conductor       |
| observedGeneration | int64              | Generation most recently reconciled                                      |
| conditions         | metav1.Condition[] | Status conditions                                                        |

---

### 7.4 InfrastructurePackExecution

**Short name:** ipe | **Scope:** Namespaced | **Authored by:** human / GitOps | **Reconciled by:** wrapper

Runtime pack delivery request. Triggers wrapper to create a Kueue Job.
Phase-2B migration from `infra.ontai.dev/PackExecution`.

#### spec

| Field              | Type                           | Required | Description                                                      |
|--------------------|--------------------------------|----------|------------------------------------------------------------------|
| clusterPackRef     | InfrastructureClusterPackRef   | yes      | Name and version of the ClusterPack to deploy                    |
| targetClusterRef   | string                         | yes      | Name of the target cluster to deliver the pack to               |
| admissionProfileRef| string                         | no       | Name of the RBACProfile governing this execution                 |
| chartVersion       | string                         | no       | Carried from ClusterPack                                         |
| chartURL           | string                         | no       | Carried from ClusterPack                                         |
| chartName          | string                         | no       | Carried from ClusterPack                                         |
| helmVersion        | string                         | no       | Carried from ClusterPack                                         |
| lineage            | lineage.SealedCausalChain      | no       | Sealed causal chain record. Immutable after creation.            |

#### status

| Field              | Type               | Description                                                              |
|--------------------|--------------------|--------------------------------------------------------------------------|
| observedGeneration | int64              | Generation most recently reconciled                                      |
| jobName            | string             | Name of the pack-deploy Kueue Job submitted for this execution           |
| operationResultRef | string             | Name of the PackOperationResult CR written after successful completion   |
| conditions         | metav1.Condition[] | Status conditions                                                        |

---

### 7.5 InfrastructurePackInstance

**Short name:** ipi | **Scope:** Namespaced | **Authored by:** wrapper | **Reconciled by:** wrapper

Records the delivered state of a pack on a specific cluster. Created by wrapper
after pack-deploy Job completes. Phase-2B migration from `infra.ontai.dev/PackInstance`.

#### spec

| Field            | Type                              | Required | Description                                                          |
|------------------|-----------------------------------|----------|----------------------------------------------------------------------|
| clusterPackRef   | string                            | yes      | Name of the ClusterPack CR this instance tracks                      |
| version          | string                            | yes      | Pack version delivered to the target cluster                         |
| targetClusterRef | string                            | yes      | Name of the target cluster this instance is installed on             |
| dependsOn        | string[]                          | no       | Pack base names that must be Delivered before this instance          |
| dependencyPolicy | InfrastructureDependencyPolicy    | no       | onDrift: Block, Warn (default), or Ignore                            |
| chartVersion     | string                            | no       | Carried from ClusterPack                                             |
| chartURL         | string                            | no       | Carried from ClusterPack                                             |
| chartName        | string                            | no       | Carried from ClusterPack                                             |
| helmVersion      | string                            | no       | Carried from ClusterPack                                             |

#### status

| Field              | Type                                | Description                                                          |
|--------------------|-------------------------------------|----------------------------------------------------------------------|
| observedGeneration | int64                               | Generation most recently reconciled                                  |
| deliveredAt        | metav1.Time                         | When the pack was most recently confirmed delivered                  |
| driftSummary       | string                              | Human-readable summary of the current drift state                   |
| upgradeDirection   | string (enum)                       | Initial, Upgrade, Rollback, or Redeploy                              |
| deployedResources  | InfrastructureDeployedResourceRef[] | Resources applied by pack-deploy. Used for deletion cleanup.        |
| conditions         | metav1.Condition[]                  | Status conditions                                                    |

---

### 7.6 InfrastructurePackBuild

**Short name:** ipb | **Scope:** Namespaced | **Authored by:** human / GitOps | **Reconciled by:** compiler (offline)

Compiler input specification. Read by the Compiler at compile time. Never applied
to a cluster as a live CR with an active controller. Phase-2B migration from
`infra.ontai.dev/PackBuild`.

#### spec

| Field           | Type                                    | Required     | Description                                                     |
|-----------------|-----------------------------------------|--------------|-----------------------------------------------------------------|
| componentName   | string                                  | yes          | Name of the component being compiled                            |
| category        | string (enum)                           | yes          | `helm`, `kustomize`, or `raw`                                   |
| helmSource      | InfrastructurePackHelmSource            | if helm      | URL, chart, version, valuesFile                                 |
| kustomizeSource | InfrastructurePackKustomizeSource       | if kustomize | path to kustomization root                                      |
| rawSource       | InfrastructurePackRawSource             | if raw       | path to raw YAML manifest directory                             |
| targetClusters  | string[]                                | no           | Cluster names to which the compiled pack should be delivered    |

---

### 7.7 InfrastructurePackReceipt

**Short name:** ipr | **Scope:** Namespaced | **Authored by:** conductor (agent) | **Reconciled by:** conductor (agent)

Pack delivery acknowledgement written by the Conductor agent on the tenant cluster
after verifying the signed PackInstance. INV-026. Phase-2B migration from
`runner.ontai.dev/PackReceipt`.

#### spec

| Field             | Type   | Required | Description                                                                 |
|-------------------|--------|----------|-----------------------------------------------------------------------------|
| clusterPackRef    | string | yes      | Name of the ClusterPack CR this receipt acknowledges                        |
| targetClusterRef  | string | yes      | Name of the cluster this receipt was generated on                           |
| packSignature     | string | no       | Base64-encoded Ed25519 signature from management cluster conductor. INV-026. |
| signatureVerified | bool   | no       | Whether the conductor agent verified the pack signature                     |
| rbacDigest        | string | no       | OCI digest of the RBAC layer. Carried from ClusterPack for audit.           |
| workloadDigest    | string | no       | OCI digest of the workload layer. Carried from ClusterPack.                 |
| chartVersion      | string | no       | Carried from ClusterPack                                                    |
| chartURL          | string | no       | Carried from ClusterPack                                                    |
| chartName         | string | no       | Carried from ClusterPack                                                    |
| helmVersion       | string | no       | Carried from ClusterPack                                                    |

---

### 7.8 PackOperationResult

**Short name:** por | **Scope:** Namespaced | **Authored by:** conductor (execute) | **Read by:** wrapper

Immutable result record written by the Conductor execute-mode Job after a pack-deploy
capability completes. Replaces the ConfigMap output channel. One per PackExecution,
created in `seam-tenant-{clusterName}` namespace.

#### spec

| Field                         | Type                           | Required | Description                                                           |
|-------------------------------|--------------------------------|----------|-----------------------------------------------------------------------|
| packExecutionRef              | string                         | no       | Name of the PackExecution CR that triggered this operation            |
| clusterPackRef                | string                         | no       | Name of the ClusterPack CR that was deployed                          |
| targetClusterRef              | string                         | no       | Name of the target cluster this operation ran against                 |
| capability                    | string                         | yes      | Name of the Conductor capability that produced this result            |
| phase                         | string                         | no       | RunnerConfig phase this result belongs to                             |
| status                        | string (enum)                  | yes      | `Succeeded` or `Failed`                                               |
| startedAt                     | metav1.Time                    | no       | Time the capability execution began                                   |
| completedAt                   | metav1.Time                    | no       | Time the capability execution finished                                |
| failureReason                 | PackOperationFailureReason     | no       | Structured failure description. Nil on success.                       |
| deployedResources             | PackOperationDeployedResource[]| no       | Resources applied during execution. Used for PackInstance cleanup.    |
| artifacts                     | PackOperationArtifact[]        | no       | Artifacts produced by this execution                                  |
| steps                         | PackOperationStepResult[]      | no       | Step results for multi-step capabilities                              |
| revision                      | int64                          | no       | Monotonically increasing revision counter for this pack operation     |
| previousRevisionRef           | string                         | no       | Name of the PackOperationResult deleted when this revision was written |
| talosClusterOperationResultRef| string                         | no       | Reserved stub. Not populated by any current controller.               |
| clusterPackVersion            | string                         | no       | ClusterPack spec.version deployed in this operation. Rollback anchor. |
| rbacDigest                    | string                         | no       | OCI digest of the RBAC layer deployed. Rollback restoration anchor.   |
| workloadDigest                | string                         | no       | OCI digest of the workload layer deployed. Rollback restoration anchor.|
| previousClusterPackVersion    | string                         | no       | clusterPackVersion from the superseded revision. One-step rollback.   |
| previousRBACDigest            | string                         | no       | rbacDigest from the superseded revision. Copied before predecessor deletion. |
| previousWorkloadDigest        | string                         | no       | workloadDigest from the superseded revision. Copied before predecessor deletion. |

**Rollback invariants:** Previous-state fields (previousClusterPackVersion, previousRBACDigest, previousWorkloadDigest) are copied from the predecessor POR by the writer before the predecessor is deleted. Empty on revision 1. These fields plus ClusterPackSpec.rollbackToRevision are the complete rollback contract: no additional CRs or history retention required. One-step rollback only (N to N-1) per invocation.

**Label:** `ontai.dev/cluster-pack={clusterPackRef}` is set on every POR to allow the wrapper rollback handler to find the current POR by ClusterPack name without knowing the PackExecution ref.

**PackOperationFailureReason fields:** category (enum: ValidationFailure, CapabilityUnavailable, ExecutionFailure, ExternalDependencyFailure, InvariantViolation, LicenseViolation, StorageUnavailable), reason (string), failedStep (string)

**PackOperationArtifact fields:** name, kind (enum: ConfigMap, Secret, OCIImage, S3Object), reference, checksum

---

### 7.9 DriftSignal

**Short name:** ds | **Scope:** Namespaced | **Authored by:** conductor (role=tenant) | **Reconciled by:** conductor (role=management)

Three-state drift acknowledgement chain. Written by conductor role=tenant when it
detects a delta between declared and actual state. Acknowledged by conductor
role=management. Decision H. At-least-once delivery.

#### spec

| Field             | Type                  | Required | Description                                                                                     |
|-------------------|-----------------------|----------|-------------------------------------------------------------------------------------------------|
| state             | string (enum)         | yes      | `pending`, `delivered`, `queued`, or `confirmed`                                                |
| correlationID     | string                | yes      | UUID v4. Used to deduplicate signals across federation retries.                                 |
| observedAt        | metav1.Time           | yes      | Time the drift was first observed by conductor role=tenant                                      |
| affectedCRRef     | DriftAffectedCRRef    | yes      | Typed reference to the CR that exhibited drift: group, kind, namespace, name                    |
| driftReason       | string                | yes      | Human-readable description of why drift was detected                                            |
| correctionJobRef  | string                | no       | Name of the corrective Job created by the management cluster. Set when state=queued.            |
| escalationCounter | int32                 | no       | Re-emit count without acknowledgement. At threshold: TerminalDrift Condition set, no more re-emits. |

**State lifecycle:** pending -> delivered -> queued -> confirmed. Transitions are one-way.

---

### 7.10 SeamMembership

**Short name:** sm | **Scope:** Namespaced | **Authored by:** human / GitOps | **Reconciled by:** guardian

Formal join declaration for an operator wishing to become a member of the Seam
infrastructure family. Guardian validates and admits the membership after verifying
the operator's RBACProfile. Operators that are not members may not be allocated
PermissionSnapshots.

#### spec

| Field             | Type          | Required | Description                                                                                          |
|-------------------|---------------|----------|------------------------------------------------------------------------------------------------------|
| appIdentityRef    | string        | yes      | Operator's application-layer identity (e.g., guardian, platform, wrapper, conductor)                 |
| domainIdentityRef | string        | yes      | DomainIdentity at core.ontai.dev this operator traces to. Must match the RBACProfile's domainIdentityRef. |
| principalRef      | string        | yes      | Kubernetes service account: `system:serviceaccount:{namespace}:{name}`. Must match RBACProfile.      |
| tier              | string (enum) | yes      | `infrastructure` (Seam family operators) or `application` (app operators)                            |

#### status

| Field                | Type               | Description                                                            |
|----------------------|--------------------|------------------------------------------------------------------------|
| admitted             | bool               | True when Guardian has validated and admitted this member              |
| admittedAt           | metav1.Time        | Timestamp when Guardian admitted this member                           |
| permissionSnapshotRef| string             | Name of the PermissionSnapshot Guardian resolved for this member       |
| conditions           | metav1.Condition[] | Admitted (bool), Validated (bool)                                      |

---

## 8. Domain Semantic Name Service

> **Locked Governor Decision - 2026-04-06**
> All six decisions in this section are locked. A Platform Governor constitutional
> amendment is required to change any of them.

---

### Decision 1 - DSNS is a controller within seam-core, not a separate binary or deployment

DSNS (Domain Semantic Name Service) is implemented as a controller registered in
`cmd/seam-core/main.go` alongside `LineageController`. It shares the existing
informer cache. There is no separate DSNS binary, no separate DSNS Deployment, and
no separate DSNS service account beyond the seam-core controller manager's service
account. DSNS runs inside the seam-core controller manager process.

DSNS watches all nine root-declaration GVKs already watched by LineageController:

| GVK | API group | Operator |
|-----|-----------|----------|
| InfrastructureTalosCluster | infrastructure.ontai.dev | Platform |
| SeamInfrastructureCluster | infrastructure.cluster.x-k8s.io | Platform |
| SeamInfrastructureMachine | infrastructure.cluster.x-k8s.io | Platform |
| InfrastructureClusterPack | infrastructure.ontai.dev | Wrapper |
| InfrastructurePackExecution | infrastructure.ontai.dev | Wrapper |
| InfrastructurePackInstance | infrastructure.ontai.dev | Wrapper |
| RBACPolicy | security.ontai.dev | Guardian |
| RBACProfile | security.ontai.dev | Guardian |
| IdentityBinding | security.ontai.dev | Guardian |

DSNS derives all DNS records from these CRDs without calling into any operator's
reconciler and without any operator importing a DSNS client package.

**The boundary is permanent and locked:** Operators write CRDs. DSNS projects
CRDs to DNS. No operator holds a dependency on DSNS. No operator calls DSNS APIs.
This separation is enforced at the package boundary - DSNS is a consumer of CRD
state, not a shared library imported by operators.

---

### Decision 2 - DNS backend is a ConfigMap named dsns-zone in ont-system

The DNS backend is a single ConfigMap named `dsns-zone` in `ont-system`. DSNS is
the sole writer of this ConfigMap. Write exclusivity is enforced by an admission
webhook using the same controller-authorship gate pattern as
`InfrastructureLineageIndex`: writes to the `dsns-zone` ConfigMap from any principal
other than the seam-core controller manager service account are rejected.

CoreDNS serves the zone using the `file` plugin, mounting the `dsns-zone` ConfigMap
at a path in the CoreDNS pod. The CoreDNS `reload` plugin propagates zone changes
within five seconds of the ConfigMap being updated.

No etcd sidecar. No dedicated DNS backend. No new infrastructure beyond one
ConfigMap and one CoreDNS Corefile stanza. The zone file content in the ConfigMap
is a standard RFC 1035 zone file generated by DSNSReconciler.

---

### Decision 3 - Authoritative zone is seam.ontave.dev; CoreDNS delivery via compiler enable phase 5

The authoritative zone served by DSNS is `seam.ontave.dev`.

The CoreDNS configuration is additive to the existing CoreDNS deployment - one new
Corefile stanza is added for the `seam.ontave.dev` zone, pointing at the mounted
ConfigMap path. The existing CoreDNS stanzas are not modified.

CoreDNS is exposed externally via a LoadBalancer service on port 53 UDP and TCP
using the management cluster VIP (10.20.0.10 in the lab). This LoadBalancer service
is a new resource emitted by the enable phase; it does not modify the existing
CoreDNS ClusterIP service used for in-cluster DNS.

The following three resources are all emitted by `compiler enable` in phase 5
(`05-post-bootstrap`), in this order:

1. The `dsns-zone` ConfigMap (initially empty zone file; DSNSReconciler populates it)
2. The CoreDNS Corefile stanza (mounted as a ConfigMap update to the CoreDNS config)
3. The CoreDNS LoadBalancer service on port 53 UDP/TCP

Phase 5 is the correct phase because DSNS depends on CNPG (phase 0), Guardian
(phases 1-2), all operators (phase 3), and Conductor (phase 4) being operational
before DNS records can be populated.

---

### Decision 4 - DNS record schema

All records are under the `seam.ontave.dev` zone. DSNSReconciler derives records
from the nine watched GVKs with no operator involvement.

**Platform records** - emitted when an `InfrastructureTalosCluster` reaches `Ready`:

| Record type | Name pattern | Value |
|-------------|--------------|-------|
| A | `{cluster-name}.seam.ontave.dev` | Cluster VIP |
| A | `api.{cluster-name}.seam.ontave.dev` | API server endpoint |
| TXT | `role.{cluster-name}.seam.ontave.dev` | Cluster role classification |

**Guardian records** - emitted on condition transitions:

| Record type | Name pattern | Value | Trigger |
|-------------|--------------|-------|---------|
| TXT | `identity.{subject-hash}.guardian.{cluster-name}.seam.ontave.dev` | RBACProfile name and IdentityProvider | IdentityBinding resolves |
| TXT | `idp.{provider-name}.guardian.seam.ontave.dev` | OIDC discovery endpoint | IdentityProvider reaches Valid condition |

**Wrapper records** - emitted on PackInstance completion:

| Record type | Name pattern | Value |
|-------------|--------------|-------|
| TXT | `pack.{pack-name}.{version}.wrapper.{cluster-name}.seam.ontave.dev` | PackReceipt digest |

**Conductor records** - emitted from RunnerConfig terminal state:

| Record type | Name pattern | Value |
|-------------|--------------|-------|
| TLSA-style TXT | `authority.conductor.seam.ontave.dev` | Management Conductor signing key fingerprint |
| TXT | `run.{runnerconfig-name}.conductor.{cluster-name}.seam.ontave.dev` | Validation result digest |

**Sovereign cluster delegation** - emitted when InfrastructureTalosCluster role classification is `sovereign`:

| Record type | Name pattern | Value |
|-------------|--------------|-------|
| NS | `{cluster-name}.seam.ontave.dev` | Sovereign cluster's CoreDNS endpoint |

The NS delegation record enables a sovereign cluster to serve its own sub-zone under
`{cluster-name}.seam.ontave.dev` via its own CoreDNS instance. DSNS writes the
delegation record; the sovereign cluster operator is responsible for configuring the
delegated zone.

---

### Decision 5 - Two runtime consumers with hard dependencies on DSNS availability

Exactly two runtime components have hard dependencies on DSNS availability:

**Conductor - tenant agent startup:**
At tenant cluster agent startup, Conductor queries `authority.conductor.seam.ontave.dev`
to retrieve the management Conductor signing key fingerprint. This DNS lookup
bootstraps signing key trust before the tenant Conductor establishes its federation
gRPC stream to the management cluster. If the record is absent or unreachable, the
tenant Conductor enters a degraded state and retries. The federation stream is not
established until the signing key fingerprint is confirmed.

**Compiler - cluster coordinate discovery:**
Compiler queries the `seam.ontave.dev` zone during cluster operations to discover
cluster coordinates (VIP, API server endpoint, role) without requiring hardcoded
kubeconfig paths or static configuration. Cluster coordinates are read from DNS at
compile time. This eliminates the need for hardcoded cluster addresses in Compiler
invocations after initial bootstrap.

No other Seam component holds a hard runtime dependency on DSNS. All other consumers
are optional or best-effort.

---

## 9. Lineage Provision Standards

### Declaration 1 - core.ontai.dev is a contract and pattern layer exclusively

The `core.ontai.dev` API group is a contract and pattern layer exclusively. It
defines abstract types and structural contracts that domain layers instantiate. It
never runs controllers against downstream domain CRs. It never watches, lists, or
reconciles objects from any domain below it. Seam Core inherits this as a permanent,
locked boundary: no Seam Core component may implement or instantiate a controller
at the `core.ontai.dev` layer, and no component at that layer may reference
`infrastructure.ontai.dev` types. This boundary has no exceptions and requires a
Platform Governor constitutional amendment to change.

---

### Declaration 2 - infrastructure.ontai.dev is the aggregation boundary for the Seam operator family

Seam Core inherits `DomainLineageIndex` from `core.ontai.dev` and translates it
into `InfrastructureLineageIndex` under `infrastructure.ontai.dev`. This extended
type carries Seam-specific lineage fields beyond the abstract contract: cluster
topology classification, RunnerConfig provenance, and operator domain boundary
metadata. These extensions are defined here, at the domain instantiation layer,
not at `core.ontai.dev`.

Seam Core also instantiates the abstract aggregation ODC from `core.ontai.dev`
as the concrete `InfrastructureLineageController`. This controller manages
`InfrastructureLineageIndex` CR lifecycle within the Seam operator family. It is
the sole controller that appends to `spec.descendantRegistry`, evaluates
`spec.policyBindingStatus`, and reconciles lineage index state. No individual Seam
Operator implements its own lineage aggregation controller.

**Semantic data flow constraint - permanent:**
Semantic lineage data produced by Seam operators flows downward only. It originates
at the operator layer, registers in `InfrastructureLineageIndex` at Seam Core, and
is evaluated against `InfrastructurePolicy` and `InfrastructureProfile` at Seam Core.
Semantic data never travels upward to `core.ontai.dev`. The core layer has no
knowledge of which Seam Operators exist, which clusters they manage, or what
operational history they have produced. This is a permanent, locked constraint.

---

### Declaration 3 - Community domain pattern: contract and pattern from core, runtime from domain

The community domain pattern is the standard composition model for any operator
family adopting `DomainLineageIndex`. The `core.ontai.dev` layer provides two
things: the contract (the abstract type definition) and the pattern (the abstract
aggregation ODC defining how conforming implementations must behave). The domain
layer provides everything else: the concrete type instantiation, the concrete
controller implementation, the domain-specific rationale enumeration, and the
domain-specific policy and profile CRD bindings.

This is the community free pass: any operator family can participate in structured
lineage tracking by instantiating `DomainLineageIndex` in their own API group,
implementing the aggregation ODC contract in their own controller, and defining
their own rationale vocabulary. They do not need to contribute to `core.ontai.dev`
or `seam-core` to participate. The abstract contract is sufficient. Seam Core's
`InfrastructureLineageIndex` and `InfrastructureLineageController` are the first
community instantiation of this pattern. They are the reference implementation, not
the only allowed implementation.

---

### Declaration 4 - Seam Core annotation namespace structure

All annotations placed on Seam-managed CRs by Seam Operators follow a two-tier
namespace structure under the `infrastructure.ontai.dev` prefix.

**Tier 1 - operator-authored annotations:**
Individual Seam Operators author annotations under the `infrastructure.ontai.dev`
prefix for their own operational keys. Each operator retains full authorship
authority over its own keys within this prefix. No cross-operator coordination is
required for operator-specific annotation keys.

**Tier 2 - governance sub-prefix (reserved):**
The `governance.infrastructure.ontai.dev` sub-prefix is reserved exclusively for
cross-cutting annotations written by controllers governed by Seam Core - specifically
the `InfrastructureLineageController` and any future Seam Core governed controller.
Individual Seam Operators never write annotations under the
`governance.infrastructure.ontai.dev` sub-prefix on their own authority. Doing so
is an invariant violation. Any annotation key under the governance sub-prefix that
an individual operator needs to consume must be written by the Seam Core governed
controller and only read by the operator.

This structure ensures that annotations under the governance sub-prefix carry the
same authorship integrity guarantee as the `InfrastructureLineageIndex` itself:
they are controller-authored by a single, designated authority, not overwritten by
arbitrary operators.

---

### Declaration 5 - Reserved LineageSynced condition type

The condition type `LineageSynced` is reserved across all Seam Operator CRD status
condition sets. No Seam Operator may define a condition of a different type for
this purpose, and no Seam Operator may repurpose this condition type for a meaning
other than lineage synchronization status.

**Lifecycle protocol:**
1. On first observation of a root declaration CR, the responsible reconciler sets
   `LineageSynced = False` with reason `LineageControllerAbsent` and a message
   indicating that `InfrastructureLineageController` has not yet processed this object.
2. The reconciler that owns the root declaration type never updates `LineageSynced`
   again after this initial write. It is a one-time initialization. The reconciler
   writes it once; it does not poll or re-evaluate it.
3. Once `InfrastructureLineageController` is deployed and processes the root
   declaration, it takes ownership of the `LineageSynced` condition and updates it
   to `True` with appropriate reason and message. All subsequent updates to
   `LineageSynced` are made exclusively by `InfrastructureLineageController`.
4. If `InfrastructureLineageController` is not deployed (e.g., stub phase, pre-Seam
   Core installation), `LineageSynced` remains `False/LineageControllerAbsent`
   indefinitely. This is an expected and documented steady state during the stub
   phase. It is not an error condition requiring operator action.

This protocol ensures that `LineageSynced` is never in an undefined state: it is
either at its initialized value (written by the reconciler, ownership not yet
transferred) or at a Seam Core governed value (ownership held by
`InfrastructureLineageController`). The transition is a one-way ownership transfer.
It cannot be reversed without a Governor-scheduled migration session.

---

### Declaration 6 - outcomeRegistry is the terminal closure protocol

When LineageController observes a terminal condition on any tracked derived object,
it appends an outcomeRegistry entry before the next reconcile cycle begins. An
outcomeRegistry entry for a given derivedObjectUID supersedes but does not replace
its corresponding descendantRegistry entry. Both records are permanent.

Terminal condition observation is event-driven: LineageController watches all nine
root-declaration GVKs via its existing informer cache. On any status condition
transition to a terminal type (Succeeded, Failed, Drifted, or Superseded), the
controller appends the corresponding outcomeRegistry entry to the governing ILI
using an SSA patch. The patch is idempotent: if an entry already exists for the
given derivedObjectUID, the patch is a no-op. The append-only invariant is enforced
at admission: any write that modifies an existing outcomeRegistry entry is rejected.

This protocol closes the causal chain: every derived object has both its creation
recorded (descendantRegistry) and its terminal outcome recorded (outcomeRegistry)
within the same ILI. Vortex retrieval can reconstruct the full lifecycle of any
derived object without querying the derived object's namespace.

---

## 10. Deferred Implementation

The following are out of scope for the stub phase and must not be acted on
without explicit Governor scheduling:

- **LineageController admission webhook** - the webhook handler that rejects
  updates modifying `spec.rootBinding` or `SealedCausalChain` fields.
  Requires a Guardian Controller Engineer session.
- **controller-gen wiring for seam-core** - currently no code generation for
  InfrastructureLineageIndex. The CRD YAML in §3 is a hand-authored stub.
- **InfrastructurePolicy and InfrastructureProfile CRDs** - referenced by ILI
  spec.policyBindingStatus. Type files not yet in api/v1alpha1. Deferred pending
  Guardian policy engine design session.

---

*seam-core-schema - Seam Core infrastructure domain*
*This document is authored and amended by the Platform Governor and Schema Engineer only.*

2026-04-21 - Amended to inherit domain-core-schema.md amendments from session/12.
  declaringPrincipal added to spec.rootBinding. createdAt and actorRef added to
  spec.descendantRegistry entries. outcomeRegistry added. Declaration 6 added.
2026-04-25 - Phase 2B complete. §2 Master GVK Reference added. §7 Per-CRD Field
  Reference added covering all 11 seam-core owned types. Sections renumbered
  to accommodate new content. All GVK references updated to infrastructure.ontai.dev
  with Infrastructure-prefixed kind names. Old groups runner.ontai.dev and
  infra.ontai.dev removed from all tables.
