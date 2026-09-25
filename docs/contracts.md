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
| Versioned machine-readable artifact | [`../schemas/brief-v1.json`](../schemas/brief-v1.json) and [`../schemas/brief-v2.json`](../schemas/brief-v2.json) — one JSON Schema (draft 2020-12) **per brief-schema version**, each describing that version's frontmatter exactly as the reference validator enforces it: required keys, the four `risk` booleans, and the closed value sets (`effort`, `gate`, `exec-tier`, `domain`, `blocked-by`, `measures`, …), plus, for brief-v2, the hierarchical `brief:` id, the `version:` counter and the reserved `id` / `supersedes` / `gates` / `feathers` / `verify` keys. Each `$id` carries the schema version, so the artifact is self-identifying. Both are **descriptive of the validator's current behaviour first** — the on-file brief corpus is the de-facto contract, and a schema the corpus fails is a wrong schema, not a wrong corpus. A brief-schema version the validator recognizes but no artifact describes is the failure mode this seam is most exposed to: the consumer-side run below then reports could-not-check for every brief in a migrated tree, so `TestEveryRecognizedBriefSchemaHasAContract` holds the two sets equal. |
| Source-side coverage gate | `statusgen/`'s `TestBriefV1SchemaCoverage` and `TestBriefV2SchemaCoverage` (each with its `_RejectsDrift` negative case, plus `TestValidatorCoversSchemaKeywords`, `TestEmbeddedSchemaMatchesCommitted` and `TestEveryRecognizedBriefSchemaHasAContract`) derive the required-key and value sets from the validator's own tables and assert the committed schemas cover them, so validator and schemas cannot drift without the assay repo's CI going red. |
| Consumer-side conformance run | `statusgen conform --root <repo>` validates every brief file under a consumer's tree against the embedded contract that file's own `schema:` marker declares — so a tree mid-migration, holding brief-v1 and brief-v2 files side by side, has each validated against the version it actually declares. It reports three-state (`checked-clean` / `checked-failed` naming file and field / `could-not-check`, fail-closed), exit 1 on any failure. It is distinct from `--lint`: lint enforces methodology rules, `conform` enforces the schema contract. `statusgen conform --emit-schema [--schema brief-v2]` prints an embedded schema to stdout, so every artifact is reproducible from any pinned build. A file whose `schema:` marker is newer than every contract the binary embeds is reported as a **version mismatch**, not a field error — the deliberate-migration signal on a pin bump, not a false failure across every consumer at once. |

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

## Semantic owners — one meaning, one home

A shared meaning — readiness, delivery, a claim, an identity — should have exactly one module
that computes it, so every consumer asks the same question of the same code instead of each
recomputing its own answer. Without that record nobody can tell "one meaning, several call
sites" from "one meaning, several *owners* that can quietly disagree" until two of them
diverge and the fix becomes another classification rule bolted onto one of them. Readiness
alone has been computed this way about seven times across this tree. This section is the
record: it names today's owner for each seeded meaning, the duplicates that exist alongside
it, and — where one exists — the decision record that chose the owner.

A row here is an **ownership record**, not a seam contract: it usually has none of the three
parts above (a row that later grows a versioned artifact, a source-side gate and a
consumer-side run stops being a bare ownership record and becomes an Instance in this
document, the way Instance #1 and #2 above did). Recording an owner is a prerequisite for a
seam contract, not a substitute for one.

**Ownership versus enforcement.** A second *owner* of one meaning is a duplicate — two
modules independently deciding the same question, free to drift apart. A second
*enforcement point* checking the same fact at a **different trust boundary**, failing for a
**different reason**, is a legitimate defense-in-depth layer, not a duplicate — the same
distinction this document already draws between the source-side gate and the consumer-side
run. This section keeps the two in separate columns so a reviewer can tell them apart at a
glance rather than re-deriving it from the diff.

**The amendment rule.** Changing a row's owner or meaning amends its decision record in the
same PR. A local exception carved out in another module — "this one caller gets to keep its
own copy" — is not a silent edit to this table; it is a `design-fit` review finding, to be
argued and recorded, not assumed.

Every path below was verified against `origin/main` at `d1c130ce5` (2026-09-25). A cell that
found nothing names the search that found nothing, per the three-state instrument rule —
"not found" and "not checked" are never the same cell.

| id | meaning | owner | decision record | duplicates (path: note) | enforcement points | contract parts (artifact/source-gate/consumer-run) |
|---|---|---|---|---|---|---|
| S-eligibility | Whether a brief satisfies its `gates:`/`feathers:`/`depends:` edges — satisfied / unsatisfied / could-not-check — the one verdict every dispatcher, board and CLI reads. | `statusgen/eligibility.go` — its own header calls it "the SINGLE deterministic verdict every consumer reads: Next-up (`eligibleBase`, nextup.go), the drive frontier (`briefFrontierState`, drivefrontier.go) and the `--eligibility` CLI". | none (`ls docs/streams/decisions/` — no `DR-<slug>` scoped to this meaning) | none found (`git grep -n 'ligib' -- tools/desk/cmd/fanoutloop tools/desk/cmd/reviewloop tools/desk/cmd/scanloop tools/desk/cmd/verifyloop tools/desk/cmd/deskboard tools/desk/cmd/deskdispatch tools/desk/cmd/desksupervise`): every checked consumer shells to the pinned `statusgen` binary or reads its rendered output rather than re-deriving. Caution, not a duplicate: `tools/desk/internal/loopengine/reconcile.go` defines its own `EligibilityVerdict` type for claim-liveness reconciliation — same name, a different question. | none — one evaluator, no second trust boundary applies this check. | none |
| S-delivery | Whether an item's deliverable (a PR) has already landed for a given brief/dispatch id — the read that must agree everywhere, or a landed item gets re-dispatched. | none declared — three independent implementations answer this question today, none named canonical. | none | `tools/desk/cmd/fanoutloop/represented.go` (already-represented reconciliation feeding `plan`); `tools/desk/cmd/deskdispatch/phantom.go` (the phantom check, same reduction type, its own classification); `tools/desk/cmd/desksupervise/reconcile.go` (its own live-PR reader; by its own comment folds "merged" and "closed" into one state, a gap the other two do not share); `statusgen/reconcile.go` (a fourth, cross-module re-derivation — statusgen does not import `tools/desk`, so its "has this landed" read is wholly separate machinery). Partial shared plumbing: `represented.go` and `phantom.go` both reduce through `tools/desk/internal/deskkit/prrepr.go`, but each still owns its own classification/dispatch decision on top of it. | none distinct from the duplicates above — all four sit at the same boundary (a queue about to act on possibly-stale state), not independent trust boundaries. | none |
| S-identity | Which credential/App identity a session or commit resolves to (bot slug, numeric id, commit noreply address) — "who is this identity, and is it the right one." | `tools/desk/internal/deskkit/forgeidentity.go` — the per-forge identity grammar and commit-address shape. | none | none found (`git grep -n 'noreply.github.com\|CommitEmailSpec' -- tools/desk/cmd`) beyond call sites into forgeidentity.go. | `tools/desk/internal/deskkit/publishidentity.go` — a second, independent check at a different trust boundary: provisioning (`deskwt`, `deskdispatch`'s worktree-create) stamps identity when a worktree is CREATED; this file re-checks every commit's author/committer against the same rules at PUSH time, catching a worktree whose identity drifted after correct provisioning. Its own header states the two as independent layers, not a duplicate. | none |
| S-claim | Mutual exclusion over "is this item already claimed" for dispatch. | `tools/desk/cmd/deskclaim-ref/main.go` — the forge-durable, cross-machine dispatch claim (`refs/dispatch/<id>`); its header names it the path `deskdispatch` invokes for claim-acquire. | none | `tools/desk/cmd/deskclaim/main.go` — a flock-backed LOCAL lock (`~/.config/assay/claims`). Its own "SCOPE NOTE (2026-08-13)" retires the `dispatch` kind from this tool in favor of the forge-ref claim, stating a machine-local lock "did nothing for two desks on different machines, which is the case that double-dispatched" — the tool's own comment already disclaims the duplicate for dispatch use. Its other kinds (route/file/close/verify) remain a legitimately different, single-session-scoped meaning, not part of this row. | none beyond the above. | none |
| S-worktree | How a worktree is created and removed under a sanctioned prefix — the isolate-first mechanics. | `tools/desk/cmd/deskwt/deskwt.go` — its header states it exists so the isolate-first rule "is the path of least resistance instead of a prompt." | none | Six inline `git worktree add` call sites bypass it, each its own local implementation: `tools/desk/cmd/cellctl/launch.go`, `tools/desk/cmd/commsloop/dispatch_native.go`, `tools/desk/cmd/deskmerge/currency.go`, `tools/desk/cmd/deskreconcile/reconcile.go`, `tools/desk/cmd/scanloop/lane.go`, `tools/desk/cmd/verifyloop/dispatch_native.go`. | none — every site above is the same mechanism at the same boundary (local git plumbing), not a second trust layer. | none |
| S-decision-acceptance | Whether a human's ratification of a `gate:human` decision / `needs-decision` issue is real — the record the design-approval gate dereferences. | `statusgen/decisiongateanchor.go` (the sanctioned issue-closure anchor: closed by the blessed human, a per-record marker, the record links the issue) together with `statusgen/decisionruling.go` (the `ruling:` link resolution for `DR-<slug>` records, registers-v1 §7.5). | none | `tools/desk/internal/deskkit/signoff.go` — the single sign-off parser for a sibling ratification artifact, `docs/streams/issue-flow/rulings.md`, gating `deskclose`/`deskmerge` write authority, plus its `tools/desk/internal/deskkit/rulinggate.go` adapter; `statusgen/transcribeverdict.go` independently re-parses that SAME `rulings.md` R-6 sign-off a second time, across the module boundary from `deskkit/signoff.go` — same artifact, same question, two parsers. | `tools/desk/cmd/deskclose/superseded.go` — the write-side human-only-close label gate, a different trust boundary: it refuses the ACT, not just the read. | none |
| S-review-verdict | Whether a reviewer verdict is current/valid at the PR's head, and whether the PR may flip ready-for-human. | `tools/desk/cmd/deskflip` (`flip.go`) — the primary verdict-validity-plus-ready-flip path, including the risk-lane aggregation below. | `DR-independence-gate` — verifier independence and dispatch-authority derivation, the mechanical proof the flip's verdict rests on. | `tools/desk/cmd/deskpost/ready.go` (`runReady`) — a second, independent ready-flip implementation that re-verifies its own verdict-currency and security-review preconditions rather than calling deskflip's. Also seeded here per this brief's facts: `tools/desk/internal/deskkit/briefrisk.go` holds two independently-invoked risk derivations, `BriefRiskFromBody` and `AuthorsRiskFromBody`, both read by `deskflip/flip.go`'s own risk-class determination — additive inputs to one aggregator, not two competing owners, but worth a reviewer's eye under this row. | none beyond the above. | none |
| S-publication-scan | Whether an outward-bound body is safe to post to its target surface. | `tools/desk/internal/deskkit/bodycheck.go` — the shared credential-pattern scan run by `deskpost`/`deskpr`/`deskreply` over any text they write. | none | none found (`git grep -n 'ghp_\|AKIA\[0-9A-Z\]' -- tools/desk statusgen`) beyond bodycheck.go's own pattern definitions. | `tools/desk/internal/deskkit/selfcontain.go` — a second, independent check asking a different question at a different boundary: bodycheck.go asks "does this carry a credential", this file asks "does this carry a private-repo disclosure", specifically for a PUBLIC-repo target. Its own header states the split explicitly. | none |
| S-status-derivation | How a brief's board/STATUS.md status (`todo`/`in-progress`/`implemented`/…) is computed and read back. | `statusgen/emit.go` (`classifyAwaiting` and the rest of the regen writer) — the single generator of STATUS.md. | none | `tools/desk/internal/loopengine/reconcile.go` (`ParseBriefRowStatus`) and `tools/desk/cmd/fanoutloop/board.go` (`statusFromOriginMain`) — two independent parsers of the SAME rendered STATUS.md table, in two different `tools/desk` packages, rather than one shared reader. | none beyond the above — same boundary (reading the rendered table), not two trust layers. | none |
| S-exit-codes | The three-state-result-to-process-exit-code mapping (checked-clean / checked-failed / could-not-check → a fixed set of codes). | `tools/desk/internal/deskkit/exitcodes.go` — its own comment: "the shared contract every desk tool maps its typed errors to." | none | `statusgen` is a separate Go module that does not import `deskkit` and redefines its own equivalents per file: `statusgen/gatetelemetry.go` (`gtExitCheckedFailed=1`, `gtExitCouldNotCheck=3` — numerically collides with, but is semantically unrelated to, deskkit's `ExitDisabled=3`), `statusgen/briefinfo.go` (`briefInfoExitOK=0`), `statusgen/newbrief.go` (`newBriefExitOK=0`); `statusgen/migrate.go` names the duplication itself, in comment: "mirror the deskkit exit-code contract." | none — same meaning, two modules, not two trust boundaries. | none |
| S-semantic-index | Whether a shared meaning has exactly one recorded owner, its duplicates, and its enforcement points. | `docs/contracts.md` (this section). | none — this brief creates the index; its own row is `S-semantic-index`. | none found — before this brief no `S-<slug>` id existed anywhere in the tree (freshness-checked 2026-09-24 at `f7bde6bfa`). | none | none — this is the index itself, not a seam contract. |

**How a brief cites this.** A brief's `design-fit:` block names the `S-<slug>` id its change
fits under in `contract:`, or `none — <why>` when no row applies yet.
