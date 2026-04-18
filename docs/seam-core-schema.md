# seam-core-schema
> API Group: infrastructure.ontai.dev
> Repository: seam-core
> Layer: Seam Core — infrastructure domain instantiation of core.ontai.dev
> All agents read this before touching any seam-core CRD or shared library type.

---

## 1. Domain Boundary

Seam Core is the CRD registry for the Seam platform. It owns cross-operator CRD
definitions that no single operator owns. No reconciliation logic lives here.
No capability engine lives here. Seam Core installs CRD definitions and runs
a schema controller that validates CRD schema versions.

**Seam Core owns:**
- `InfrastructureLineageIndex` — the infrastructure domain instantiation of
  `DomainLineageIndex` from `core.ontai.dev`. Anchors the sealed causal chain
  for every root declaration managed by the Seam platform.
- `RunnerConfig` — produced by Platform and Wrapper, reconciled by Conductor.
  Transfer from conductor shared library is a governed migration (SC-INV-002).
- `InfrastructurePolicy` — produced by humans/guardian, reconciled by Guardian.
- `InfrastructureProfile` — reconciled by Guardian.
- The creation rationale enumeration (`pkg/lineage`) — a compile-time typed
  constant set imported by all Seam Operators.
- The `SealedCausalChain` field type (`pkg/lineage`) — embedded by every
  Seam-managed CRD in its spec.

**What Seam Core does NOT own:**
- Reconciliation loops for any CRD.
- Operator-specific admission logic.
- Runtime or compile-mode execution.

**SC-INV-001** — seam-core owns CRD definitions. Reconcilers live in the operator.
**SC-INV-002** — RunnerConfig CRD transfer is a governed migration.
**SC-INV-003** — seam-core installs before all operators.

---

## 2. Derivation from Domain Core

`InfrastructureLineageIndex` instantiates `DomainLineageIndex` from `core.ontai.dev`
per the domain-core-schema.md instantiation contract (§3). The instantiation rules
applied in this domain are:

| Constraint                        | Domain Core (abstract)                  | Seam Core (infrastructure instantiation)               |
|-----------------------------------|-----------------------------------------|--------------------------------------------------------|
| API group                         | core.ontai.dev                          | infrastructure.ontai.dev                               |
| creationRationale constraint      | unconstrained string                    | enum — `pkg/lineage.CreationRationale` values          |
| domainPolicyRef                   | string (abstract)                       | Name of an `InfrastructurePolicy` CR                   |
| domainProfileRef                  | string (abstract)                       | Name of an `InfrastructureProfile` CR                  |
| rootBinding fields                | as defined — unmodified                 | as defined — unmodified                                |
| Lineage Index Pattern             | one index per root declaration          | one index per root declaration — unchanged             |
| Authorship rule                   | controller-authored exclusively         | controller-authored exclusively — unchanged            |
| Immutability rule                 | rootBinding sealed at admission         | rootBinding sealed at admission — unchanged            |

---

## 3. InfrastructureLineageIndex

### 3.1 Purpose

`InfrastructureLineageIndex` is the concrete sealed causal chain index for all
objects managed by the Seam platform in the infrastructure domain. One instance
is created per root declaration (TalosCluster, PackExecution, etc.) by the
controller responsible for that root declaration type.

All derived objects (RunnerConfig, Job, OperationResult, PermissionSnapshot, etc.)
carry a reference to their root declaration's `InfrastructureLineageIndex`. They
do not carry their own index instances.

The index grows monotonically as new derived objects are created. Entries in
`spec.descendantRegistry` are never modified or removed.

### 3.2 CRD Stub

```
# STUB — infrastructure.ontai.dev/v1alpha1 InfrastructureLineageIndex
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

| Field                  | Type   | Required | Description                                                             |
|------------------------|--------|----------|-------------------------------------------------------------------------|
| rootKind               | string | yes      | Kind of the root declaration                                            |
| rootName               | string | yes      | Name of the root declaration                                            |
| rootNamespace          | string | yes      | Namespace of the root declaration                                       |
| rootUID                | string | yes      | UID of the root declaration at index creation time                      |
| rootObservedGeneration | int64  | yes      | Root declaration generation when this index was created                 |

#### spec.descendantRegistry[]

| Field                    | Type   | Required | Description                                                            |
|--------------------------|--------|----------|------------------------------------------------------------------------|
| kind                     | string | yes      | Kind of the derived object                                             |
| name                     | string | yes      | Name of the derived object                                             |
| namespace                | string | yes      | Namespace of the derived object                                        |
| uid                      | string | yes      | UID of the derived object                                              |
| seamOperator             | string | yes      | Seam Operator that created this derived object                         |
| creationRationale        | string | yes      | Value from `pkg/lineage.CreationRationale` enum                        |
| rootGenerationAtCreation | int64  | yes      | Root declaration generation when derived object was created            |

#### spec.policyBindingStatus

| Field                            | Type    | Required | Description                                                          |
|----------------------------------|---------|----------|----------------------------------------------------------------------|
| domainPolicyRef                  | string  | no       | Name of the bound InfrastructurePolicy                               |
| domainProfileRef                 | string  | no       | Name of the bound InfrastructureProfile                              |
| policyGenerationAtLastEvaluation | int64   | no       | InfrastructurePolicy generation at last evaluation                   |
| driftDetected                    | boolean | no       | True if drift detected at last evaluation                            |

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

## 6. Deferred Implementation

The following are out of scope for the stub phase and must not be acted on
without explicit Governor scheduling:

- **LineageController** — the controller that manages `InfrastructureLineageIndex`
  CR lifecycle (create, append descendant entries, evaluate policy binding status).
  Requires a dedicated Controller Engineer session.
- **Admission webhook immutability gate** — the webhook handler that rejects
  updates modifying `spec.rootBinding` or `SealedCausalChain` fields.
  Requires a Guardian Controller Engineer session.
- **RunnerConfig CRD ownership transfer** — from conductor shared library to
  seam-core. SC-INV-002. Requires Governor-scheduled migration session.
- **controller-gen wiring for seam-core** — currently no code generation.
  The InfrastructureLineageIndex CRD YAML above is a hand-authored stub.

---

## 7. Lineage Provision Standards

### Declaration 1 — core.ontai.dev is a contract and pattern layer exclusively

The `core.ontai.dev` API group is a contract and pattern layer exclusively. It
defines abstract types and structural contracts that domain layers instantiate. It
never runs controllers against downstream domain CRs. It never watches, lists, or
reconciles objects from any domain below it. Seam Core inherits this as a permanent,
locked boundary: no Seam Core component may implement or instantiate a controller
at the `core.ontai.dev` layer, and no component at that layer may reference
`infrastructure.ontai.dev` types. This boundary has no exceptions and requires a
Platform Governor constitutional amendment to change.

---

### Declaration 2 — infrastructure.ontai.dev is the aggregation boundary for the Seam operator family

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

**Semantic data flow constraint — permanent:**
Semantic lineage data produced by Seam operators flows downward only. It originates
at the operator layer, registers in `InfrastructureLineageIndex` at Seam Core, and
is evaluated against `InfrastructurePolicy` and `InfrastructureProfile` at Seam Core.
Semantic data never travels upward to `core.ontai.dev`. The core layer has no
knowledge of which Seam Operators exist, which clusters they manage, or what
operational history they have produced. This is a permanent, locked constraint.

---

### Declaration 3 — Community domain pattern: contract and pattern from core, runtime from domain

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

### Declaration 4 — Seam Core annotation namespace structure

All annotations placed on Seam-managed CRs by Seam Operators follow a two-tier
namespace structure under the `infrastructure.ontai.dev` prefix.

**Tier 1 — operator-authored annotations:**
Individual Seam Operators author annotations under the `infrastructure.ontai.dev`
prefix for their own operational keys. Each operator retains full authorship
authority over its own keys within this prefix. No cross-operator coordination is
required for operator-specific annotation keys.

**Tier 2 — governance sub-prefix (reserved):**
The `governance.infrastructure.ontai.dev` sub-prefix is reserved exclusively for
cross-cutting annotations written by controllers governed by Seam Core — specifically
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

### Declaration 5 — Reserved LineageSynced condition type

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

## 8. Domain Semantic Name Service

> **Locked Governor Decision — 2026-04-06**
> All six decisions in this section are locked. A Platform Governor constitutional
> amendment is required to change any of them.

---

### Decision 1 — DSNS is a controller within seam-core, not a separate binary or deployment

DSNS (Domain Semantic Name Service) is implemented as a controller registered in
`cmd/seam-core/main.go` alongside `LineageController`. It shares the existing
informer cache. There is no separate DSNS binary, no separate DSNS Deployment, and
no separate DSNS service account beyond the seam-core controller manager's service
account. DSNS runs inside the seam-core controller manager process.

DSNS watches all nine root-declaration GVKs already watched by LineageController:

| GVK | Operator |
|-----|----------|
| TalosCluster | Platform |
| SeamInfrastructureCluster | Platform |
| SeamInfrastructureMachine | Platform |
| ClusterPack | Wrapper |
| PackExecution | Wrapper |
| PackInstance | Wrapper |
| RBACPolicy | Guardian |
| RBACProfile | Guardian |
| IdentityBinding | Guardian |

DSNS derives all DNS records from these CRDs without calling into any operator's
reconciler and without any operator importing a DSNS client package.

**The boundary is permanent and locked:** Operators write CRDs. DSNS projects
CRDs to DNS. No operator holds a dependency on DSNS. No operator calls DSNS APIs.
This separation is enforced at the package boundary — DSNS is a consumer of CRD
state, not a shared library imported by operators.

---

### Decision 2 — DNS backend is a ConfigMap named dsns-zone in ont-system

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

### Decision 3 — Authoritative zone is seam.ontave.dev; CoreDNS delivery via compiler enable phase 5

The authoritative zone served by DSNS is `seam.ontave.dev`.

The CoreDNS configuration is additive to the existing CoreDNS deployment — one new
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
(phases 1–2), all operators (phase 3), and Conductor (phase 4) being operational
before DNS records can be populated.

---

### Decision 4 — DNS record schema

All records are under the `seam.ontave.dev` zone. DSNSReconciler derives records
from the nine watched GVKs with no operator involvement.

**Platform records** — emitted when a `TalosCluster` reaches `Ready`:

| Record type | Name pattern | Value |
|-------------|--------------|-------|
| A | `{cluster-name}.seam.ontave.dev` | Cluster VIP |
| A | `api.{cluster-name}.seam.ontave.dev` | API server endpoint |
| TXT | `role.{cluster-name}.seam.ontave.dev` | Cluster role classification |

**Guardian records** — emitted on condition transitions:

| Record type | Name pattern | Value | Trigger |
|-------------|--------------|-------|---------|
| TXT | `identity.{subject-hash}.guardian.{cluster-name}.seam.ontave.dev` | RBACProfile name and IdentityProvider | IdentityBinding resolves |
| TXT | `idp.{provider-name}.guardian.seam.ontave.dev` | OIDC discovery endpoint | IdentityProvider reaches Valid condition |

**Wrapper records** — emitted on PackInstance completion:

| Record type | Name pattern | Value |
|-------------|--------------|-------|
| TXT | `pack.{pack-name}.{version}.wrapper.{cluster-name}.seam.ontave.dev` | PackReceipt digest |

**Conductor records** — emitted from RunnerConfig terminal state:

| Record type | Name pattern | Value |
|-------------|--------------|-------|
| TLSA-style TXT | `authority.conductor.seam.ontave.dev` | Management Conductor signing key fingerprint |
| TXT | `run.{runnerconfig-name}.conductor.{cluster-name}.seam.ontave.dev` | Validation result digest |

**Sovereign cluster delegation** — emitted when TalosCluster role classification is `sovereign`:

| Record type | Name pattern | Value |
|-------------|--------------|-------|
| NS | `{cluster-name}.seam.ontave.dev` | Sovereign cluster's CoreDNS endpoint |

The NS delegation record enables a sovereign cluster to serve its own sub-zone under
`{cluster-name}.seam.ontave.dev` via its own CoreDNS instance. DSNS writes the
delegation record; the sovereign cluster operator is responsible for configuring the
delegated zone.

---

### Decision 5 — Two runtime consumers with hard dependencies on DSNS availability

Exactly two runtime components have hard dependencies on DSNS availability:

**Conductor — tenant agent startup:**
At tenant cluster agent startup, Conductor queries `authority.conductor.seam.ontave.dev`
to retrieve the management Conductor signing key fingerprint. This DNS lookup
bootstraps signing key trust before the tenant Conductor establishes its federation
gRPC stream to the management cluster. If the record is absent or unreachable, the
tenant Conductor enters a degraded state and retries. The federation stream is not
established until the signing key fingerprint is confirmed.

**Compiler — cluster coordinate discovery:**
Compiler queries the `seam.ontave.dev` zone during cluster operations to discover
cluster coordinates (VIP, API server endpoint, role) without requiring hardcoded
kubeconfig paths or static configuration. Cluster coordinates are read from DNS at
compile time. This eliminates the need for hardcoded cluster addresses in Compiler
invocations after initial bootstrap.

No other Seam component holds a hard runtime dependency on DSNS. All other consumers
are optional or best-effort.

---

*seam-core-schema — Seam Core infrastructure domain*
*This document is authored and amended by the Platform Governor and Schema Engineer only.*
