# Contributing to seam-core

Thank you for your interest in contributing to the Seam platform.

---

## Before you begin

Read the Seam Platform Constitution (`CLAUDE.md` in the ontai root repository)
before opening a Pull Request. All contributions must respect the platform
invariants defined in that document.

Key invariants for this repository:

- `seam-core` is a schema controller and library repository. No operator or
  binary is deployed from this repository.
- `pkg/lineage` is the single source of the `CreationRationale` enumeration.
  All operators and both Compiler and Conductor binaries import it. Changes to
  this enumeration are breaking changes for every component in the platform.
- New `CreationRationale` values require Platform Governor review before merging.
- `InfrastructureLineageIndex` implements the Lineage Index Pattern: one instance
  per root declaration, never one per derived object.
- `InfrastructureLineageIndex.spec.rootBinding` is immutable after creation.
  The admission webhook must reject any UPDATE that modifies this section.

---

## Development setup

```sh
git clone https://github.com/ontai-dev/seam-core
cd seam-core
go build ./...
go test ./...
```

There is no deployable binary. The build target confirms all packages compile
cleanly against their declared dependencies.

---

## Adding a CreationRationale value

1. Add the constant to `pkg/lineage/rationale.go`.
2. Add the value to `docs/seam-core-schema.md` under the rationale table.
3. Open a Platform Governor review before merging.
4. After merge, open coordinated Pull Requests in all operator repositories
   that consume `pkg/lineage` to update their imports.

---

## Adding a new CRD type

New cross-operator CRD types in `api/v1alpha1/` must:

1. Include a `spec.lineage` field of the `SealedCausalChain` type (sourced from
   the `domain-core` abstract definition) when the type is a root declaration.
2. Be accompanied by a `docs/seam-core-schema.md` update in the same Pull Request.
3. Receive Platform Governor approval before merging.

---

## Schema changes

Changes to existing CRD field shapes in `api/v1alpha1/` are potentially breaking
for all operators. They require:

- A Platform Governor review before merging.
- A coordinated release with all operators that consume these types.
- An update to `docs/seam-core-schema.md`.

---

## Pull Request checklist

- [ ] `go build ./...` passes with no errors
- [ ] `go test ./...` passes
- [ ] No em dashes in any new documentation
- [ ] No shell scripts added (Go only, per INV-001)
- [ ] `docs/seam-core-schema.md` updated if CRD types or shared packages changed
- [ ] Platform Governor review requested for any `pkg/lineage` or CRD changes

---

## Reporting issues

Open an issue at: https://github.com/ontai-dev/seam-core/issues

For security vulnerabilities, contact the maintainers directly rather than
opening a public issue.

---

## License

By contributing, you agree that your contributions will be licensed under the
Apache License, Version 2.0. See `LICENSE` for the full text.

---

*seam-core - Seam Core Schema Controller and Shared Library*
