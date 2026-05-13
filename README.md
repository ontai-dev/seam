# seam

API group: `seam.ontai.dev`

seam owns the cross-operator CRD type definitions and shared library packages for the ONT platform. No operator binary is deployed from this repository. seam installs its CRD manifests before all operators reach Running state on any cluster (SC-INV-003).

---

## CRD types

All four types are under `seam.ontai.dev/v1alpha1`.

| Kind | Short name | Scope | Authoring principal |
|---|---|---|---|
| LineageRecord | lr | Namespaced | LineageController (controller-authored exclusively) |
| RunnerConfig | rc | Namespaced | platform operator (operator-generated, never human-authored) |
| DriftSignal | ds | Namespaced | conductor role=tenant |
| SeamMembership | sm | Namespaced | human / GitOps |

Schema reference: [docs/seam-schema.md](docs/seam-schema.md)

---

## Shared packages

### `pkg/lineage`

Defines the `SealedCausalChain` type embedded in every Seam-managed CRD spec. Defines `CreationRationale`, the compile-time controlled vocabulary for why a derived object was created. Provides `SetDescendantLabels` for operators to mark derived objects at creation time. No operator extends this vocabulary unilaterally -- new values require a seam PR and Platform Governor review.

Import path: `github.com/ontai-dev/seam/pkg/lineage`

### `pkg/conditions`

Defines condition type and reason string constants for all ONT operators. This package is the single source of truth for every condition type and reason used across guardian, platform, dispatcher, and conductor. Operators import these constants rather than declaring their own strings.

Import path: `github.com/ontai-dev/seam/pkg/conditions`

---

## Building

Compile all packages:

```
make build
```

Generate deep-copy methods and CRD manifests:

```
make generate
```

Requires `controller-gen` on PATH or at `~/go/bin/controller-gen`.

---

## Testing

Unit tests:

```
make test-unit
```

End-to-end tests require `MGMT_KUBECONFIG` to be set. All e2e specs skip automatically when the variable is absent:

```
make e2e
```

---

## Status

Alpha. API group `seam.ontai.dev/v1alpha1`. No type in this group has graduated to beta.

Issues: https://github.com/ontai-dev/seam/issues

---

seam - Seam Cross-Operator CRD Definitions and Shared Library / Apache License, Version 2.0
