## seam: Operational Constraints

Read `~/ontai/CLAUDE.md` first. The constraints below extend the root constitutional document.

---

### Schema authority

Primary reference: `docs/seam-schema.md`

`docs/seam-schema.md` is the authoritative field-level specification for every CRD type declared in this repository and for the shared library packages in `pkg/`. Read it in full before any CRD change or shared library change. Source files are the ground truth for field shapes; the schema doc reflects them and must be kept in sync.

---

### Invariants

SC-INV-001 -- seam owns all cross-operator CRD definitions under seam.ontai.dev. The complete set of seam-owned types is: LineageRecord, RunnerConfig, DriftSignal, SeamMembership. Reconcilers for these CRDs live in the operator repos that own the domain logic, not in seam.

SC-INV-002 -- All new CRD work goes directly into seam under seam.ontai.dev. No new type is declared outside this repository without an explicit Governor directive.

SC-INV-003 -- seam CRD manifests are installed before all operators. No operator reaches Running state on a cluster that has not applied the seam CRD bundle first.

---

### Session protocol additions

Step 4a -- Read `docs/seam-schema.md` in full before any CRD or shared library change.

Step 4b -- Any change to the `CreationRationale` vocabulary (the Go constant enumeration in `pkg/lineage/rationale.go`) requires a PR and Platform Governor review. No operator may extend the enumeration unilaterally. (root Section 14 Decision 5)
