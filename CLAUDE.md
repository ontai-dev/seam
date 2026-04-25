## seam-core: Operational Constraints
> Read ~/ontai/CLAUDE.md first. The constraints below extend the root constitutional document.

### Schema authority
Primary: docs/seam-core-schema.md
Supporting: ~/ontai/domain-core/docs/domain-core-schema.md (DomainLineageIndex schema owner)

### Invariants
SC-INV-001 -- seam-core owns all cross-operator CRD definitions under infrastructure.ontai.dev. The complete set of seam-core-owned types is: InfrastructureLineageIndex, InfrastructureRunnerConfig, InfrastructurePackReceipt, InfrastructureClusterPack, InfrastructurePackExecution, InfrastructurePackInstance, InfrastructurePackBuild, InfrastructureTalosCluster, DriftSignal, InfrastructurePolicy, InfrastructureProfile. Reconcilers for these CRDs live in the operator repos that own the domain logic, not in seam-core.
SC-INV-002 -- RunnerConfig and all cross-operator CRD type migrations to seam-core are complete as of Phase 2B (2026-04-25). No further governed migration sessions are required. The old API groups runner.ontai.dev and infra.ontai.dev are superseded. All new CRD work goes directly into seam-core under infrastructure.ontai.dev.
SC-INV-003 -- seam-core CRD manifests are installed before all operators. No operator reaches Running state on a cluster that has not applied the seam-core CRD bundle first.

### Session protocol additions
Step 4a -- Read docs/seam-core-schema.md in full before any CRD or shared library change.
Step 4b -- Any change to the creation rationale vocabulary (the Go constant enumeration owned by seam-core) requires a PR and Platform Governor review. No operator may extend the enumeration unilaterally. (root Section 14 Decision 5)
