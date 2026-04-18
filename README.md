# seam-core

**Seam Core — Cross-operator CRD definitions and shared library**
**API Group:** `infrastructure.ontai.dev`

---

## What this repository is

`seam-core` declares the cross-operator CRD types shared across the Seam platform
and owns the shared library packages imported by all operators and both Compiler
and Conductor binaries.

No operator or binary in the Seam stack is deployed from this repository.
`seam-core` is a schema controller and library repository.

---

## CRDs

| Kind | API Group | Role |
|---|---|---|
| `InfrastructureLineageIndex` | `infrastructure.ontai.dev` | Sealed causal chain index for infrastructure declarations |
| `SeamMembership` | `infrastructure.ontai.dev` | Domain membership record linking a principal to a target cluster |

---

## Shared packages

| Package | Role |
|---|---|
| `pkg/lineage` | `CreationRationale` enumeration and lineage record construction helpers |
| `pkg/conditions` | Shared condition type constants used across all Seam CRDs |
| `pkg/e2e` | End-to-end test helpers (not imported in production code) |

---

## InfrastructureLineageIndex

`InfrastructureLineageIndex` is the concrete instantiation of the
`DomainLineageIndex` abstract pattern from `domain-core`. One instance is created
per root declaration, never one per derived object. This is the **Lineage Index
Pattern**.

Key properties:
- `spec.rootBinding` is immutable after creation. The admission webhook rejects any
  UPDATE that modifies this section.
- `spec.descendantRegistry` is monotonically growing. Entries are appended, never
  modified or removed.
- Controller-authored exclusively. Writes from any principal other than the
  designated controller service account are rejected at admission.

---

## SeamMembership

`SeamMembership` records that a principal (identified by a Kubernetes service account
reference) has been admitted to a target cluster domain. The reconciler in `guardian`
owns this record.

Admission requires:
1. The referenced `RBACProfile` exists and has `provisioned=true`.
2. The `domainIdentityRef` in the `RBACProfile` matches the `principalRef`.

---

## CreationRationale vocabulary

The `pkg/lineage` package exports the `CreationRationale` enumeration. This is the
compile-time controlled vocabulary for why a root declaration was created. All
operators and both binaries import this enumeration. New values require a Pull
Request to this repository and a Platform Governor review.

Current values:
- `ClusterProvision`
- `ClusterDecommission`
- `SecurityEnforcement`
- `PackExecution`
- `VirtualizationFulfillment`
- `ConductorAssignment`
- `VortexBinding`

---

## Building

```sh
go build ./...
```

There is no deployable binary in this repository. The build target confirms that
all packages compile cleanly.

---

## Testing

```sh
go test ./...
```

---

## Schema reference

- `docs/seam-core-schema.md` - Full API contract and field definitions

---

*seam-core - Seam Core Schema Controller and Shared Library*
*Apache License, Version 2.0*
