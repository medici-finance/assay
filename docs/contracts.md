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
diverge and the fix becomes another classification rule bolted onto one of them. This brief's
starting premise was that readiness alone has been computed this way about seven times across
this tree; the sweep below did not confirm that count for the readiness/eligibility meaning
itself — S-eligibility's own duplicates cell finds one evaluator, no full duplicate — though
several *other* seeded meanings below (S-delivery, S-status-derivation, S-exit-codes,
S-review-verdict, S-decision-acceptance) do carry a real duplicate owner or a colliding
implementation. This section is the record: it names today's owner for each seeded meaning,
the duplicates that exist alongside it, and — where one exists — the decision record that
chose the owner.

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
argued and recorded, not assumed. Moving a path out of *enforcement points* (or into
*duplicates*) is the edit that exposes a layer to retirement, so a PR that removes or
reclassifies an enforcement point carries a security review in the same PR.

Every path below was verified against `origin/main` at `d1c130ce5` (2026-09-25). A cell that
found nothing names the search that found nothing, per the three-state instrument rule —
"not found" and "not checked" are never the same cell.

| id | meaning | owner | decision record | duplicates (path: note) | enforcement points | contract parts (artifact/source-gate/consumer-run) |
|---|---|---|---|---|---|---|
| S-eligibility | Whether a brief satisfies its `gates:`/`feathers:`/`depends:` edges — satisfied / unsatisfied / could-not-check — the one verdict every dispatcher, board and CLI reads. | `statusgen/eligibility.go` — its own header calls it "the SINGLE deterministic verdict every consumer reads: Next-up (`eligibleBase`, nextup.go), the drive frontier (`briefFrontierState`, drivefrontier.go) and the `--eligibility` CLI". | none (`ls docs/streams/decisions/` — no `DR-<slug>` scoped to this meaning) | partial: `statusgen/flow.go` (`dependencySatisfiedAt`, `eligibleAt`) re-derives the SAME satisfying condition `resolveInRepoBriefRef` (eligibility.go) tests, replayed against the historian — but only over a brief's `depends:` edges; its own header states it does not look at `gates:`/`feathers:`, so it duplicates a strict subset of this meaning, not the full verdict. The wider search (`git grep -n 'ligib' -- tools/desk/cmd/fanoutloop tools/desk/cmd/reviewloop tools/desk/cmd/scanloop tools/desk/cmd/verifyloop tools/desk/cmd/deskboard tools/desk/cmd/deskdispatch tools/desk/cmd/desksupervise`) was scoped to `tools/desk/cmd` and does not reach this file inside the separate `statusgen` module; every checked `tools/desk/cmd` consumer still shells to the pinned `statusgen` binary or reads its rendered output rather than re-deriving. Caution, not a duplicate: `tools/desk/internal/loopengine/reconcile.go` defines its own `EligibilityVerdict` type for claim-liveness reconciliation — same name, a different question. | none — `flow.go`'s replay and the live evaluator sit at the same boundary (deriving the same verdict from state), not independent trust layers. | none |
| S-delivery | Whether an item's deliverable (a PR) has already landed for a given brief/dispatch id — the read that must agree everywhere, or a landed item gets re-dispatched. | none declared — four independent implementations answer this question today, none named canonical. | none | `tools/desk/cmd/fanoutloop/represented.go` (already-represented reconciliation feeding `plan`); `tools/desk/cmd/deskdispatch/phantom.go` (the phantom check, same reduction type, its own classification); `tools/desk/cmd/desksupervise/reconcile.go` (its own live-PR reader; by its own comment folds "merged" and "closed" into one state, a gap the other two do not share); `statusgen/reconcile.go` (a fourth, cross-module re-derivation — statusgen does not import `tools/desk`, so its "has this landed" read is wholly separate machinery). Partial shared plumbing: `represented.go` and `phantom.go` both reduce through `tools/desk/internal/deskkit/prrepr.go`, but each still owns its own classification/dispatch decision on top of it. | none distinct from the duplicates above — all four sit at the same boundary (a queue about to act on possibly-stale state), not independent trust boundaries. | none |
| S-identity | Which credential/App identity a session or commit resolves to (bot slug, numeric id, commit noreply address) — "who is this identity, and is it the right one." | `tools/desk/internal/deskkit/forgeidentity.go` — the per-forge identity grammar and commit-address shape. | none | none found (`git grep -n 'noreply.github.com\|CommitEmailSpec' -- tools/desk/cmd`) beyond call sites into forgeidentity.go. | `tools/desk/internal/deskkit/publishidentity.go` — a second, independent check at a different trust boundary: provisioning (`deskwt`, `deskdispatch`'s worktree-create) stamps identity when a worktree is CREATED; this file re-checks every commit's author/committer against the same rules at PUSH time, catching a worktree whose identity drifted after correct provisioning. Its own header states the two as independent layers, not a duplicate. | none |
| S-claim | Mutual exclusion over "is this item already claimed" for dispatch. | `tools/desk/cmd/deskclaim-ref/main.go` — the forge-durable, cross-machine dispatch claim (`refs/dispatch/<id>`); its header names it the path `deskdispatch` invokes for claim-acquire. The store behind it is chosen through one seam, `deskkit.ResolveClaimStore(repo)` (`tools/desk/internal/deskkit/claimstore.go`), called by both `deskclaim-ref/args.go` and `deskdispatch/dispatch.go` before any worktree is cut or credential minted. | `DR-forge-neutral-21` — lifts the store behind `ResolveClaimStore`, roster-resolved with no caller choice and refusal (exit 6) as the only fallback on an unusable store; an unset key keeps today's forge-ref resolution for one release window under a NOTICE. | `tools/desk/cmd/deskclaim/main.go` — a flock-backed LOCAL lock (`~/.config/assay/claims`). Its own "SCOPE NOTE (2026-08-13)" retires the `dispatch` kind from this tool in favor of the forge-ref claim, stating a machine-local lock "did nothing for two desks on different machines, which is the case that double-dispatched" — the tool's own comment already disclaims the duplicate for dispatch use. Its other kinds (route/file/close/verify) remain a legitimately different, single-session-scoped meaning, not part of this row. Separately, `tools/desk/internal/deskkit/claimref.go`'s `ClaimRefsPrefix` (`refs/heads/dispatch/*`) is a SECOND forge claim namespace, distinct from the one `deskclaim-ref` itself speaks (`refs/dispatch/*`) — `deskclaim-ref/main.go`'s own SCOPE NOTE says the two are not the same, and `tools/desk/cmd/fanoutloop/land.go` / `tools/desk/cmd/desksupervise/live.go` read against `ClaimRefsPrefix` only, so a claim taken through one namespace is invisible to a reader of the other. Two namespaces for one mutual-exclusion question is a duplicate owner, recorded here pending consolidation. | none beyond the above. | none |
| S-worktree | How a worktree is created and removed under a sanctioned prefix — the isolate-first mechanics. | `tools/desk/cmd/deskwt/deskwt.go` — its header states it exists so the isolate-first rule "is the path of least resistance instead of a prompt." | none | Seven inline `git worktree add` call sites bypass it, each its own local implementation: `tools/desk/cmd/cellctl/launch.go`, `tools/desk/cmd/commsloop/dispatch_native.go`, `tools/desk/cmd/deskmerge/currency.go`, `tools/desk/cmd/deskreconcile/reconcile.go`, `tools/desk/cmd/scanloop/lane.go`, `tools/desk/cmd/verifyloop/dispatch_native.go`, and `statusgen/lintaudit.go` (`productionWorktree`) in the separate `statusgen` module. This cell covers CREATION only; removal sites were not separately surveyed. | none — every site above is the same mechanism at the same boundary (local git plumbing), not a second trust layer. | none |
| S-decision-acceptance | Whether a human's ratification of a `gate:human` decision / `needs-decision` issue is real — the record the design-approval gate dereferences. | `statusgen/decisiongateanchor.go` (the sanctioned issue-closure anchor: closed by the blessed human, a per-record marker, the record links the issue) together with `statusgen/decisionruling.go` (the `ruling:` link resolution for `DR-<slug>` records, registers-v1 §7.5). The three human-stamp corroboration anchors are composed in `statusgen/corroborate.go`; `decisiongateanchor.go`'s own header calls itself the THIRD of them. | none | `tools/desk/internal/deskkit/signoff.go` — the single sign-off BLOCK parser for the rulings register that gates `deskclose`/`deskmerge` write authority, plus its `tools/desk/internal/deskkit/rulinggate.go` adapter. Its own header states what it deliberately does NOT decide: whether the URL resolves, who authored the artifact, and whether the text says the words a lane needs — those belong to the caller's fetch-and-verify step. `statusgen/transcribeverdict.go`'s `findR6SignoffLine` and `statusgen/transcribescan.go`'s `findR7SignoffLine` are a second, line-level parse of that SAME sign-off text — a genuine duplicate at the parse-only layer, since a shared parser here must still leave the identity check below to its own caller. | `tools/desk/cmd/deskclose/superseded.go` — the write-side human-only-close label gate, a different trust boundary: it refuses the ACT, not just the read. `statusgen/transcribeVerdictEnactmentGate` (`transcribeverdict.go`, from line 423) and its R-7 twin `transcribeEnactmentGate` (`transcribescan.go`, line 180) are a second, independent enforcement layer beneath the parse: each resolves the sign-off's URL and verifies the commenting author's login:id against the roster blessing authority, failing closed on an empty, unparseable, unresolvable or unconfigured sign-off — the identity check `deskkit/signoff.go`'s header explicitly leaves to the caller. They guard the verify-verdict-transcription and scan-transcription acts respectively, a different boundary from `deskclose/superseded.go`'s human-only-close gate. | none |
| S-review-verdict | Whether a reviewer verdict is current/valid at the PR's head, and whether the PR may flip ready-for-human. | `tools/desk/cmd/deskflip` (`flip.go`) — the primary verdict-validity-plus-ready-flip path, including the risk-lane aggregation below. | `DR-independence-gate` — verifier independence and dispatch-authority derivation, the mechanical proof the flip's verdict rests on. `DR-external-prereq-21` — rules that the ready gate is the single place performing the authoritative independent re-verification of a declared external-prerequisite exemption, and that the board, review planner and ready gate must agree that re-review is surfaced distinctly rather than as suspected forgery. | `tools/desk/cmd/deskpost/ready.go` (`runReady`) — a second, independent ready-flip implementation that re-verifies its own verdict-currency and security-review preconditions rather than calling deskflip's. `tools/desk/cmd/deskboard/board.go`'s own decisive-verdict reduction (`board.go:840-860`) is a THIRD independent implementation of the same reduction, explicitly declared "KEEP IN SYNC with deskpost/ready.go's latestAppVerdict — with ONE DELIBERATE DIVERGENCE": `latestAppVerdict` filters its input stream to the correctness lane (`classifyLane`, `ready.go:465`) before reducing, while the board's `decisive` admits any reviewer-bot verdict, security lane included, documented as erring toward flagging on its advisory surface rather than granting a flip. Also seeded here per this brief's facts: `tools/desk/internal/deskkit/briefrisk.go` holds two independently-invoked risk derivations, `BriefRiskFromBody` and `AuthorsRiskFromBody`, both read by `deskflip/flip.go`'s own risk-class determination — additive inputs to one aggregator, not two competing owners, but worth a reviewer's eye under this row. | none beyond the above. | none |
| S-publication-scan | Whether an outward-bound body is safe to post to its target surface. | `tools/desk/internal/deskkit/bodycheck.go` — the shared credential-pattern scan run by `deskpost`/`deskpr`/`deskreply` over any text they write. | none | `git grep -n 'ghp_\|AKIA\[0-9A-Z\]' -- tools/desk statusgen` does return hits outside `bodycheck.go`: `tools/desk/cmd/deskadvisory/checkdefs/example-org/example-k8s.json:39` (adopter-facing example content, not a scanner), `tools/desk/internal/deskkit/untrustcorpus/inert.go:18` and `tools/desk/internal/deskkit/structured.go:36` (pattern definitions/test fixtures for the scan itself, not a competing scanner), and `tools/desk/cmd/deskpr/deskpr.go:900` (a call site into bodycheck.go, not a second implementation). None of the four is a second outward-body scanner, so none found still holds for a *duplicate owner*. | `tools/desk/internal/deskkit/selfcontain.go` — a second, independent check asking a different question at a different boundary: bodycheck.go asks "does this carry a credential", this file asks "does this carry a private-repo disclosure", specifically for a PUBLIC-repo target. Its own header states the split explicitly. | none |
| S-status-derivation | How a brief's board/STATUS.md status (`todo`/`in-progress`/`implemented`/…) is computed and read back. | `statusgen/emit.go` (`emit`) — the single generator that writes STATUS.md; the Status cell is read back by `statusgen/parse.go`'s `parseBriefTable`, which reads the `## Briefs` table by column header. | none | Two `tools/desk` re-parsers of a stream board README's Status cell (not STATUS.md itself, which only `statusgen/parse.go` reads back): `tools/desk/internal/loopengine/reconcile.go`'s `ParseBriefRowStatus`, which takes the 5th pipe-delimited cell BY POSITION; and `tools/desk/cmd/fanoutloop/board.go`'s `newLiveStatusReader`/`briefsTableRows`, which reads the same `## Briefs` table BY COLUMN HEADER, matching `statusgen/parse.go`'s own convention rather than loopengine's positional one. `fanoutloop/board.go`'s `statusFromOriginMain` is not a third parser — it is a bare `git show` of STATUS.md's bytes with no parsing of its own — and is not counted as a duplicate here. | none beyond the above — same boundary (reading a rendered table), not two trust layers. | none |
| S-exit-codes | The three-state-result-to-process-exit-code mapping (checked-clean / checked-failed / could-not-check → a fixed set of codes). | `tools/desk/internal/deskkit/exitcodes.go` — its own comment: "the shared contract every desk tool maps its typed errors to." Defines no dedicated checked-failed code; a could-not-verify precondition maps to `ExitUnverifiable=6`. | none | `statusgen` is a separate Go module that does not import `deskkit` and redefines its own equivalents per file — and the could-not-check meaning itself lands on THREE different codes across the tree, not the "semantically unrelated" numeric coincidence an earlier version of this row claimed: `statusgen/conform.go` uses `conformExitCouldNot`/`conformExitUsageError=2` for could-not-check ("usage/refusal shares the could-not-check code", its own comment), a convention `verifyrun`/`shardcheck` and `statusgen/newbrief.go` (`newBriefExitRefuse=2`) also share; `statusgen/gatetelemetry.go`'s `gtExitCouldNotCheck=3` uses a THIRD code for the identical meaning, its own comment explaining why 2 was unavailable ("claimed by the flag package's usage-error exit") — so the 3 is a real collision with deskkit's `ExitDisabled=3`, one meaning wearing three codes (2, 3, 6). `statusgen/briefinfo.go` (`briefInfoExitOK=0`) only duplicates the one code every contract agrees on (`ExitOK=0`). `statusgen/migrate.go` names the duplication itself, in comment: "mirror the deskkit exit-code contract." | none — same meaning, several modules, not distinct trust boundaries. | none |
| S-semantic-index | Whether a shared meaning has exactly one recorded owner, its duplicates, and its enforcement points. | `docs/contracts.md` (this section). | none — this brief creates the index; its own row is `S-semantic-index`. | none found — before this brief no `S-<slug>` id existed anywhere in the tree (freshness-checked 2026-09-24 at `f7bde6bfa`). | none | none — this is the index itself, not a seam contract. |

**How a brief cites this.** A brief's `design-fit:` block names the `S-<slug>` id its change
fits under in `contract:`, or `none — <why>` when no row applies yet.
