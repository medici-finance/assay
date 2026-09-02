# Assay tooling contracts — schema-first, three-part

Assay's load-bearing tooling seams are held by a pinned binary and prose. The pin proves
*which* build runs; on its own it proves nothing about whether that build and its consumer
still agree on the *contract* between them. When the two drift — a validator grows a field
the spec never documents, a consumer wires a key the tool no longer reads — nothing fails
until someone notices by hand, later.

This document is the pattern that turns "the maintainer discovers drift eventually" into
"CI fails the day it drifts." It is the Brokk/Anvil OpenAPI idea — a published,
machine-readable contract with a conformance check on both sides — applied to Assay's own
seams. It is written once over the seam that already has it (the brief-v1 frontmatter
contract) and parameterized so the second seam (a consumer repo's install contract) can
follow the same shape.

## The three parts

Every Assay tooling contract has the same three parts. A contract missing any one of them
is not a contract — it is a convention with a version number.

1. **A versioned, machine-readable artifact.** The contract is a file, checked into the
   source repo, that a machine can read: a JSON Schema, a pin-file grammar, a manifest.
   It carries its own version in a stable identifier so a consumer can tell which contract
   it is reading, and so a deliberate contract change is a visible version bump rather than
   a silent shape change.

2. **A source-side coverage gate.** A test in the source repo's own CI derives the
   contract's surface *from the reference implementation's own tables* and asserts the
   committed artifact covers them exactly — plus a negative case proving the gate reddens
   on a doctored artifact. This is the layer that makes the artifact and the code unable to
   drift apart without the source repo's CI failing at author time. Without it the artifact
   is hand-maintained documentation that rots at the first change nobody remembers to
   mirror.

3. **A consumer-side conformance run.** A read-only, three-state check the consumer runs in
   its *own* CI — shipped in the pinned binary and picked up on the consumer's next routine
   pin bump, so no per-consumer wiring is needed after the first. It fails on the consumer's
   side of the seam (a file the consumer owns no longer satisfies the contract) with a
   different signal, in a different repo, than the source-side gate. This is the layer that
   turns drift into a red check in the place drift actually happens.

The three parts are independent by construction — a different check, in a different
component, tripping on a different signal — which is what makes them defense-in-depth rather
than one check written three times. The source-side gate catches the maintainer editing the
validator without the artifact; the consumer-side run catches a consumer editing its own
files away from the artifact; the versioned identifier catches a deliberate migration and
reports it *as* a migration, not as a field error.

Each part reports three-state, never two: `checked-clean` / `checked-failed` /
`could-not-check`, per [`three-state-instrument-rule.md`](three-state-instrument-rule.md). A
check that could not run has cleared nothing, and must never render the same result as one
that ran and found nothing.

## Instance #1 — the brief-v1 frontmatter contract (built)

The seam is `statusgen` ↔ the `schema: brief-v1` frontmatter every brief carries. The
reference implementation validates a brief's frontmatter shape — required keys, field types,
closed value sets — but until this contract shipped, that shape lived only in the validator's
Go tables and a prose spec that had already drifted eleven releases behind it.

| Part | Realization |
|------|-------------|
| Versioned machine-readable artifact | [`../schemas/brief-v1.json`](../schemas/brief-v1.json) — JSON Schema (draft 2020-12) describing brief-v1 frontmatter exactly as the reference validator enforces it: required keys, the four `risk` booleans, and the closed value sets (`effort`, `gate`, `exec-tier`, `domain`, `blocked-by`, `measures`, …). Its `$id` carries the schema version, so the artifact is self-identifying. It is **descriptive of the validator's current behaviour first** — the on-file brief corpus is the de-facto contract, and a schema the corpus fails is a wrong schema, not a wrong corpus. |
| Source-side coverage gate | `statusgen/`'s `TestBriefV1SchemaCoverage` (with its `_RejectsDrift` negative case, `TestValidatorCoversSchemaKeywords`, and `TestEmbeddedSchemaMatchesCommitted`) derives the required-key and value sets from the validator's own tables and asserts the committed schema covers them, so validator and schema cannot drift without the assay repo's CI going red. |
| Consumer-side conformance run | `statusgen conform --root <repo>` validates every `schema: brief-v1` file under a consumer's tree against the schema embedded in the pinned binary, and reports three-state (`checked-clean` / `checked-failed` naming file and field / `could-not-check`, fail-closed), exit 1 on any failure. It is distinct from `--lint`: lint enforces methodology rules, `conform` enforces the schema contract. `statusgen conform --emit-schema` prints the embedded schema to stdout, so the artifact is reproducible from any pinned build. A file whose `schema:` marker is newer than the binary's embedded contract is reported as a **version mismatch**, not a field error — the deliberate-migration signal on a pin bump, not a false failure across every consumer at once. |

The consumer-side run rides the existing distribution lane, not a new one: it ships in a
`statusgen` release, a consumer picks it up when it bumps its [`.assay-versions`](distribution.md)
pin, and it is invoked from the shared lint action after the existing `--lint` step. Wiring
the run into that action is gated on the pinned binary actually carrying the `conform`
subcommand — a call to a verb a not-yet-bumped pinned binary lacks would redden every
consumer at once, so the wiring is sequenced *after* the release that ships `conform` and the
pin bump that adopts it, never before.

## Instance #2 — the consumer install contract (pattern ready, not yet built)

The second seam is `desk-tools` ↔ a consumer repo: what a repo must wire to run the desk
tooling. Today that contract is prose plus example configs plus a pinned binary, with one
partial machine-readable piece already in place — `deskpins`, which validates the
`.assay-versions` pin-file grammar against the contract in
[`distribution.md`](distribution.md) and is the proto-pattern this document generalizes
(a published contract plus a read-only, three-state validator). The remaining surfaces of
the seam have no artifact and no consumer-side conformance run.

Instantiating the pattern here means naming each surface and giving it the same three parts:

| Surface of the seam | Versioned artifact | Source-side coverage gate | Consumer-side conformance run |
|---------------------|--------------------|---------------------------|-------------------------------|
| The pin file (`.assay-versions`) | the pin-file grammar (already the `deskpins` contract in `distribution.md`) | a test deriving the grammar the validator accepts from its own tables | `deskpins --check` (already exists) |
| The roster known-set (the `ASSAY_*` configuration keys a consumer may set) | a machine-readable list of the known keys, versioned, emitted from the loader's own registry | a test asserting the emitted list covers the loader's registry exactly, so a new key cannot be added without the artifact moving | a consumer-side run that rejects an unknown `ASSAY_*` key against the published set (the loader already fails closed on one; the contract makes the *set itself* a published, checkable artifact) |
| The hook set (the pre-push and related client-side guards a consumer installs) | a versioned manifest of the expected hooks and their identifying content | a test asserting the manifest matches the hooks the installer actually ships | a consumer-side run reporting three-state whether the installed hooks match the manifest |
| The binary manifest (the installed tool set and its checksums) | the checksum manifest already published per release | a test asserting the manifest lists exactly the shipped assets | a consumer-side run verifying installed binaries against the manifest |
| The CI wiring (the lint action a consumer references) | the versioned action contract — the steps a consumer is expected to run | a test asserting the action runs the expected steps | the consumer's own CI *is* the run — a green lint job is the conformance signal |

Building instance #2 is a follow-up: each row above is a unit of work, and the pattern's
value is that each unit is the *same shape* as instance #1 — a descriptive-first artifact, a
source-side gate that derives its surface from the implementation's own tables, and a
consumer-side three-state run that rides the pin/release lane the consumer already follows.
The rule that keeps every instance honest is the one instance #1 states first: the artifact
is descriptive of current behaviour, and the on-file corpus is the arbiter. A contract the
existing consumers fail is a wrong contract, not a wrong fleet.

## What the pattern is not

- **Not a second linter.** `conform` and `--lint` are deliberately distinct verbs: the schema
  contract is per-file frontmatter *shape*; methodology rules (id-versus-filename, wave-versus-
  README, dependency resolution, the verifier floor, the risk/gate coupling) need the corpus or
  the README tables and stay in `--lint`. Merging them would make a reviewer unable to tell a
  shape violation from a methodology violation.
- **Not a prescriptive redesign.** The artifact never leads the implementation. It is written
  from the reference implementation's behaviour and held there by the source-side gate. A
  contract authored from the *spec* rather than the *validator* produces a false-positive storm
  on the first real corpus run — which is exactly the failure the descriptive-first rule and the
  pre-merge corpus run exist to prevent.
- **Not a replacement for the pin.** The pin still proves which build runs; this pattern adds
  the orthogonal proof that the build and the consumer still agree on the contract. Both layers
  are load-bearing, and neither subsumes the other.
