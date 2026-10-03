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
itself — the eligibility row's own duplicates cell finds one evaluator, no full duplicate —
though most of the *other* seeded meanings below do carry a real duplicate owner or a
colliding implementation, and each row's duplicates column says which. (The prose here names
no row id on purpose: the brief's presence check counts ids across this whole section, so an
id named in prose would keep it passing after that id's row was deleted.) This section is the
record: it names today's owner for each seeded meaning,
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

Every path and line number below was re-verified against `origin/main` at `8ad26beab`
(2026-09-25). A cell that
found nothing names the search that found nothing, per the three-state instrument rule —
"not found" and "not checked" are never the same cell.

| id | meaning | owner | decision record | duplicates (path: note) | enforcement points | contract parts (artifact/source-gate/consumer-run) |
|---|---|---|---|---|---|---|
| S-eligibility | Whether a brief satisfies its `gates:`/`feathers:`/`depends:` edges — satisfied / unsatisfied / could-not-check — the one verdict every dispatcher, board and CLI reads. | `statusgen/eligibility.go` — its own header calls it "the SINGLE deterministic verdict every consumer reads: Next-up (`eligibleBase`, nextup.go), the drive frontier (`briefFrontierState`, drivefrontier.go) and the `--eligibility` CLI". | none (`ls docs/streams/decisions/` — no `DR-<slug>` scoped to this meaning) | partial: `statusgen/flow.go` (`dependencySatisfiedAt`, `eligibleAt`) re-derives the SAME satisfying condition `resolveInRepoBriefRef` (eligibility.go) tests, replayed against the historian — but only over a brief's `depends:` edges; its own header states it does not look at `gates:`/`feathers:`, so it duplicates a strict subset of this meaning, not the full verdict. The wider search (`git grep -n 'ligib' -- tools/desk/cmd/fanoutloop tools/desk/cmd/reviewloop tools/desk/cmd/scanloop tools/desk/cmd/verifyloop tools/desk/cmd/deskboard tools/desk/cmd/deskdispatch tools/desk/cmd/desksupervise`) was scoped to `tools/desk/cmd` and does not reach this file inside the separate `statusgen` module; every checked `tools/desk/cmd` consumer still shells to the pinned `statusgen` binary or reads its rendered output rather than re-deriving. Caution, not a duplicate: `tools/desk/internal/loopengine/reconcile.go` defines its own `EligibilityVerdict` type for claim-liveness reconciliation — same name, a different question. | none — `flow.go`'s replay and the live evaluator sit at the same boundary (deriving the same verdict from state), not independent trust layers. | none |
| S-delivery | Whether an item's deliverable (a PR) has already landed for a given brief/dispatch id — the read that must agree everywhere, or a landed item gets re-dispatched. | none declared — four independent implementations answer this question today, none named canonical. | none | `tools/desk/cmd/fanoutloop/represented.go` (already-represented reconciliation feeding `plan`); `tools/desk/cmd/deskdispatch/phantom.go` (the phantom check, same reduction type, its own classification); `tools/desk/cmd/desksupervise/reconcile.go` (its own live-PR reader; by its own comment folds "merged" and "closed" into one state, a gap the other two do not share); `statusgen/reconcile.go` (a fourth, cross-module re-derivation — statusgen does not import `tools/desk`, so its "has this landed" read is wholly separate machinery). Partial shared plumbing: `represented.go` and `phantom.go` both reduce through `tools/desk/internal/deskkit/prrepr.go`, but each still owns its own classification/dispatch decision on top of it. | none distinct from the duplicates above — all four sit at the same boundary (a queue about to act on possibly-stale state), not independent trust boundaries. | none |
| S-identity | Which credential/App identity a session or commit resolves to (bot slug, numeric id, commit noreply address) — "who is this identity, and is it the right one." | `tools/desk/internal/deskkit/forgeidentity.go` — the per-forge identity grammar and commit-address shape. | none | none found (`git grep -n 'noreply.github.com\|CommitEmailSpec' -- tools/desk/cmd`) beyond call sites into forgeidentity.go. | `tools/desk/internal/deskkit/publishidentity.go` — a second, independent check at a different trust boundary: provisioning (`deskwt`, `deskdispatch`'s worktree-create) stamps identity when a worktree is CREATED; this file re-checks every commit's author/committer against the same rules at PUSH time, catching a worktree whose identity drifted after correct provisioning. Its own header states the two as independent layers, not a duplicate. | none |
| S-claim | Mutual exclusion over "is this item already claimed" for dispatch. | `tools/desk/cmd/deskclaim-ref/main.go` — the forge-durable, cross-machine dispatch claim (`refs/dispatch/<id>`); its header names it the path `deskdispatch` invokes for claim-acquire. The acquire itself (create-if-absent on the claim ref, then read the holder on rejection) is `cmdAcquire` in `tools/desk/cmd/deskclaim-ref/claim.go`, which `main.go`'s `run` reaches through `args.go`. The store behind it is chosen through one seam, `deskkit.ResolveClaimStore(repo)` (`tools/desk/internal/deskkit/claimstore.go`), called by both `deskclaim-ref/args.go` and `deskdispatch/dispatch.go` before any worktree is cut or credential minted. | `DR-forge-neutral-21` — lifts the store behind `ResolveClaimStore`, roster-resolved with no caller choice and refusal (exit 6) as the only fallback on an unusable store; an unset key keeps today's forge-ref resolution for one release window under a NOTICE. | `tools/desk/cmd/deskclaim/main.go` — a flock-backed LOCAL lock (`~/.config/assay/claims`). Its own "SCOPE NOTE (2026-08-13)" retires the `dispatch` kind from this tool in favor of the forge-ref claim, stating a machine-local lock "did nothing for two desks on different machines, which is the case that double-dispatched" — the tool's own comment already disclaims the duplicate for dispatch use. Its other kinds (route/file/close/verify) remain a legitimately different, single-session-scoped meaning, not part of this row. Separately, `tools/desk/internal/deskkit/claimref.go`'s `ClaimRefsPrefix` (`refs/heads/dispatch/*`) is a SECOND forge claim namespace, distinct from the one `deskclaim-ref` itself speaks (`refs/dispatch/*`) — `deskclaim-ref/main.go`'s own SCOPE NOTE says the two are not the same, and `tools/desk/cmd/fanoutloop/land.go` / `tools/desk/cmd/desksupervise/live.go` read against `ClaimRefsPrefix` only, so a claim taken through one namespace is invisible to a reader of the other. Two namespaces for one mutual-exclusion question is a duplicate owner, recorded here pending consolidation. | none beyond the above. | none |
| S-worktree | How a worktree is created and removed under a sanctioned prefix — the isolate-first mechanics. | `tools/desk/cmd/deskwt/deskwt.go` — its header states it exists so the isolate-first rule "is the path of least resistance instead of a prompt." | none | Seven inline `git worktree add` call sites bypass it, each its own local implementation: `tools/desk/cmd/cellctl/launch.go`, `tools/desk/cmd/commsloop/dispatch_native.go`, `tools/desk/cmd/deskmerge/currency.go`, `tools/desk/cmd/deskreconcile/reconcile.go`, `tools/desk/cmd/scanloop/lane.go`, `tools/desk/cmd/verifyloop/dispatch_native.go`, and `statusgen/lintaudit.go` (`productionWorktree`) in the separate `statusgen` module. This cell covers CREATION only; removal sites were not separately surveyed. | none — every site above is the same mechanism at the same boundary (local git plumbing), not a second trust layer. | none |
| S-decision-acceptance | Whether a human's ratification of a `gate:human` decision / `needs-decision` issue is real — the record the design-approval gate dereferences. | `statusgen/decisiongateanchor.go` (the sanctioned issue-closure anchor: closed by the blessed human, a per-record marker, the record links the issue) together with `statusgen/decisionruling.go` (the `ruling:` link resolution for `DR-<slug>` records, registers-v1 §7.5). The three human-stamp corroboration anchors are composed in `statusgen/corroborate.go`; `decisiongateanchor.go`'s own header calls itself the THIRD of them. | none | `tools/desk/internal/deskkit/signoff.go` — the single sign-off BLOCK parser for the rulings register that gates `deskclose`/`deskmerge` write authority, plus its `tools/desk/internal/deskkit/rulinggate.go` adapter. Its own header states what it deliberately does NOT decide: whether the URL resolves, who authored the artifact, and whether the text says the words a lane needs — those belong to the caller's fetch-and-verify step. `statusgen/transcribeverdict.go`'s `findR6SignoffLine` and `statusgen/transcribescan.go`'s `findR7SignoffLine` are a second, line-level parse of that SAME sign-off text — a genuine duplicate at the parse-only layer, since a shared parser here must still leave the identity check below to its own caller. | `tools/desk/cmd/deskclose/superseded.go` — the write-side human-only-close label gate, a different trust boundary: it refuses the ACT, not just the read. `transcribeVerdictEnactmentGate` (`statusgen/transcribeverdict.go`, from line 423) and its R-7 twin `transcribeEnactmentGate` (`transcribescan.go`, line 180) are a second, independent enforcement layer beneath the parse: each resolves the sign-off's URL and verifies the commenting author's login:id against the roster blessing authority, failing closed on an empty, unparseable, unresolvable or unconfigured sign-off — the identity check `deskkit/signoff.go`'s header explicitly leaves to the caller. They guard the verify-verdict-transcription and scan-transcription acts respectively, a different boundary from `deskclose/superseded.go`'s human-only-close gate. | none |
| S-review-verdict | Whether a reviewer verdict is current/valid at the PR's head, and whether the PR may flip ready-for-human. | No single owner of the ready flip: two alternative paths perform the same mutation (draft → ready-for-human), and each is the ONLY home of different precondition re-verifications, so neither is a plain copy of the other (see *enforcement points*). They are `tools/desk/cmd/deskflip` (`flip.go`) and `tools/desk/cmd/deskpost/ready.go` (`runReady`, the `deskpost ready` verb). The verdict-currency *reduction* underneath them does have a shared implementation: `deskkit.ReduceAppVerdict` (`tools/desk/internal/deskkit/prstate.go`), which `deskflip/flip.go` calls for its correctness-lane verdict and `tools/desk/cmd/deskboard/stalled.go` also calls. | `DR-independence-gate` — verifier independence and dispatch-authority derivation, the mechanical proof the flip's verdict rests on. `DR-external-prereq-21` — rules that "the ready gate" is the single place performing the authoritative independent re-verification of a declared external-prerequisite exemption, and that the board, review planner and ready gate agree that such a re-review is surfaced distinctly rather than as suspected forgery. "The ready gate" there is `deskpost ready`: the brief that enacted the decision implemented it in `tools/desk/cmd/deskpost/ready.go` and `tools/desk/cmd/deskpost/externalprerequisite.go`, and `deskflip` has no external-prerequisite code (`git grep -n -i 'external.\?prereq' -- tools/desk/cmd/deskflip ':!*_test.go'` finds nothing). | Duplicates of the verdict-currency reduction only (not of either path's exemption checks): `deskpost/ready.go`'s `latestAppVerdict` (`ready.go:398`) reduces the reviewer verdict itself instead of calling `deskkit.ReduceAppVerdict`, after filtering the stream to the correctness lane (`classifyLane`, `ready.go:507`). `tools/desk/cmd/deskboard/board.go`'s own `decisive` reduction (`board.go:855-866`) is a third, explicitly declared "KEEP IN SYNC with deskpost/ready.go's latestAppVerdict — with ONE DELIBERATE DIVERGENCE": it admits any reviewer-bot verdict, security lane included, and so flags a standing security-lane CHANGES_REQUESTED on its advisory surface where `latestAppVerdict` would not. Any retirement of the board's reduction onto a shared one must keep that security-lane flagging, or the board stops showing that block. **Consolidating the two ready paths is not a duplicate retirement:** it must keep the union of both paths' preconditions and exemption checks listed under *enforcement points*, and a PR doing it is an enforcement-point reclassification under the amendment rule above. Also seeded here per this brief's facts: `tools/desk/internal/deskkit/briefrisk.go` holds two independently-invoked risk derivations, `BriefRiskFromBody` and `AuthorsRiskFromBody`, both read by `deskflip/flip.go`'s own risk-class determination — additive inputs to one aggregator, not two competing owners, but worth a reviewer's eye under this row. | `tools/desk/cmd/deskpost/ready.go` with `tools/desk/cmd/deskpost/externalprerequisite.go` — the only home of the `DR-external-prereq-21` re-verification (called only from `ready.go:144`, `clearedByExternalPrereq`): it reads only reviewer-bot-authored records, filtered by authenticated login so a worker's prose cannot author a clearance, and observes each declared prerequisite afresh rather than trusting the reviewer's citation. `ready.go` also runs its own trust gate and `Security-Review` precondition. `tools/desk/cmd/deskflip/flip.go` — the only home of the check-only exemption (`checkOnlyCRCleared`) and the body-edit exemption (`bodyEditCRCleared`, gated by `deskkit.BodyEditDeclared`); `deskpost ready` runs neither. The desk-decided condition is NOT path-local: its one implementation is `deskkit.DeskDecidedRefusal` (`tools/desk/internal/deskkit/decidedgate.go`), and BOTH paths call it — `deskflip/flip.go` through `checkDeskDecided`, `deskpost/ready.go` through `deskDecidedRefusal` — each on its first read and again on its pre-mutation re-read (#1694 added the `deskpost ready` call; before it, that path ran no desk-decided check). Each path fails closed on its own checks, so moving any of them onto the other path without its preconditions (the reviewer-login filter, the fresh observation) reopens the laundering path `DR-external-prereq-21` rules out. | none |
| S-publication-scan | Whether an outward-bound body is safe to post to its target surface. | `tools/desk/internal/deskkit/bodycheck.go` — the shared credential-pattern scan run by `deskpost`/`deskpr`/`deskreply` over any text they write. | none | `git grep -n 'ghp_\|AKIA\[0-9A-Z\]' -- tools/desk statusgen ':!*_test.go' ':!tools/desk/internal/deskkit/bodycheck.go'` returns seven hits, none of them a second outward-body scanner: `tools/desk/cmd/deskadvisory/checkdefs/example-org/example-k8s.json:39` (adopter-facing example content); `tools/desk/cmd/deskpr/deskpr.go:918` (a comment in deskpr's diff-marker stripping, which prepares text for `deskkit.ScanSurfaceSecrets` rather than scanning it); `tools/desk/internal/deskkit/structured.go:36` (a comment in the exemption-span logic inside the scan itself); `tools/desk/internal/deskkit/mutations.json:4` and `tools/desk/internal/deskkit/trace-mutations.json:11` (mutation-test specs that disarm the scan and the trace redaction, to prove their tests catch it); `tools/desk/internal/deskkit/untrustcorpus/inert.go:18` and `tools/desk/internal/deskkit/untrustcorpus/testdata/README.md:49` (the untrusted-corpus inertness guard and its fixture notes). So none found still holds for a *duplicate owner*. | `tools/desk/internal/deskkit/selfcontain.go` — a second, independent check asking a different question at a different boundary: bodycheck.go asks "does this carry a credential", this file asks "does this carry a private-repo disclosure", specifically for a PUBLIC-repo target. Its own header states the split explicitly. | none |
| S-status-derivation | How a brief's board/STATUS.md status (`todo`/`in-progress`/`implemented`/…) is computed and read back. | `statusgen/emit.go` (`emit`) — the single generator that writes STATUS.md. Its input, each stream README's `## Briefs` Status cell, is read by `statusgen/parse.go`'s `parseBriefTable` (called from `parseStreamREADME`), which reads that table by column header. `parse.go` reads the stream README, not STATUS.md. | none | Two readers exist, over two different artifacts. **The stream README's Status cell** is re-parsed in `tools/desk` twice: `tools/desk/internal/loopengine/reconcile.go`'s `ParseBriefRowStatus` takes the 5th pipe-delimited cell BY POSITION, and `tools/desk/cmd/fanoutloop/board.go`'s `newLiveStatusReader`/`briefsTableRows` reads the same `## Briefs` table BY COLUMN HEADER, matching `statusgen/parse.go`'s convention rather than loopengine's positional one. **STATUS.md itself** (statusgen's rendered output) is parsed back in `tools/desk/cmd/fanoutloop/board.go`: `readNextUp` parses the rendered `## Next up` table and `readAwaitingRework` the rendered `### Awaiting implementer rework` table, both through `statusMDContent`, and each cross-checks every row against the README Status cell before offering it. `statusFromOriginMain` there is only the `git show` that fetches STATUS.md's bytes for `statusMDContent`, with no parsing of its own. Not counted as duplicates: `tools/desk/cmd/deskboot/boot.go`'s `summariseBoard` only counts the rows under the Next-up heading for a boot summary and derives no status, and `statusgen --check` (`statusgen/main.go`) byte-compares STATUS.md against a fresh render. Search: `git grep -nE '"STATUS\.md"\|:STATUS\.md' -- 'tools/desk/*.go' 'statusgen/*.go' ':!*_test.go'`. | none beyond the above — same boundary (reading a rendered table), not two trust layers. | none |
| S-exit-codes | The three-state-result-to-process-exit-code mapping (checked-clean / checked-failed / could-not-check → a fixed set of codes). | `tools/desk/internal/deskkit/exitcodes.go` — its own comment: "the shared contract every desk tool maps its typed errors to." Defines no dedicated checked-failed code; a could-not-verify precondition maps to `ExitUnverifiable=6`. | none | `statusgen` is a separate Go module that does not import `deskkit` and redefines its own equivalents per file — and the could-not-check meaning itself lands on THREE different codes across the tree, not the "semantically unrelated" numeric coincidence an earlier version of this row claimed: `statusgen/conform.go` uses `conformExitCouldNot`/`conformExitUsageError=2` for could-not-check ("usage/refusal shares the could-not-check code", its own comment), a convention `verifyrun`/`shardcheck` and `statusgen/newbrief.go` (`newBriefExitRefuse=2`) also share; `statusgen/gatetelemetry.go`'s `gtExitCouldNotCheck=3` uses a THIRD code for the identical meaning, its own comment explaining why 2 was unavailable ("claimed by the flag package's usage-error exit") — so the 3 is a real collision with deskkit's `ExitDisabled=3`, one meaning wearing three codes (2, 3, 6). `statusgen/briefinfo.go` (`briefInfoExitOK=0`) only duplicates the one code every contract agrees on (`ExitOK=0`). `statusgen/migrate.go` names the duplication itself, in comment: "mirror the deskkit exit-code contract." | none — same meaning, several modules, not distinct trust boundaries. | none |
| S-semantic-index | Whether a shared meaning has exactly one recorded owner, its duplicates, and its enforcement points. | `docs/contracts.md` (this section). | none — this brief creates the index; its own row is `S-semantic-index`. | none found — before this brief no `S-<slug>` id existed anywhere in the tree (freshness-checked 2026-09-24 at `f7bde6bfa`). | none | none — this is the index itself, not a seam contract. |

**How a brief cites this.** A brief's `design-fit:` block names the `S-<slug>` id its change
fits under in `contract:`, or `none — <why>` when no row applies yet.

**How code declares an implementation.** The function that computes a row's meaning carries the
line comment `// semantic: S-<slug>` directly above its declaration (on the computing function,
not a wrapper). Every repository path a row's *owner* or *duplicates* cell names in backticks
counts as a place the marker may sit: a file path is that one file, and a directory path is the
package in that directory, never its subdirectories. A path naming `tools/desk` itself, its
`cmd` or `internal` directory, or anything above them names no place, so a cell can mention the
module in prose without making every function in it a listed site. A marker anywhere else, a
marker naming no row, and a marker count above `tools/desk/internal/arch/markers.txt`'s ceiling
fail `R-one-implementation`. Markers in `_test.go` files are not read. An implementation that
carries no marker is invisible to that check: finding an undeclared duplicate is the design-fit
review stage's question, not the test's.

## Rule register

Every refusal, gate and guard the tooling enforces has one row here, next to the semantic index
it serves. A rule with no row cannot be reviewed, so it can never leave: the monthly diet below
reads this table, and nothing else. The register records rules **as they are**; a row is not an
endorsement, and nothing in this table retires a rule by itself.

**Row schema.** `id (R-<slug>) | rule (one line) | enforced at (path[:symbol]) | serves (S-<slug>)
| owner (module or role) | invariant | justifying issue(s) | catch source | last reviewed`.

- *serves* names the semantic-index row above whose meaning the rule enforces, or `none` when no
  row fits. A `none` row is an orphan, and orphans are diet candidates.
- *catch source* is how the rule is known to be able to catch anything: a named telemetry class,
  a test name that shows it firing, or `none`. It feeds the diet's three-state catch status.
- Rows marked **trust boundary** are security controls. They fire rarely by design, so each one
  names a test that shows it firing and starts `proven-able-to-fire`, never `zero-without-proof`.
  A trust-boundary row also names every override or bypass of its rule (flag, env toggle,
  exemption) in its *rule* cell, with the override's own site in *enforced at*, so a review of
  the row sees the whole control and not only its refusing half.
- *justifying issue(s)* cites the issues that motivated the rule, or the public PR that landed it
  when no separate issue exists.
- *last reviewed* is the date and the `main` commit the row was checked against.

**A new rule adds a row in the same PR.** A PR that adds a refusal, a gate or a guard adds its row
here in the same diff, and review stage 06 checks it. A PR that changes a row's enforcement site,
invariant or serving row amends the row in the same diff.

**Seed scope.** The rows below are the rules implicated in the recent public fix-caused-next-bug
chains, plus the weight ceiling. This is not a back-fill of every rule in the tree. Later rows
land with their own briefs, not here: `R-brittle-mark` (brief 08) and `R-dep-direction`,
`R-hub-allowlist`, `R-one-implementation` (brief 10).

| id | rule | enforced at | serves | owner | invariant | justifying issue(s) | catch source | last reviewed |
|---|---|---|---|---|---|---|---|---|
| R-model-floor | **Trust boundary.** An authority-bearing write (a review verdict via `deskpost review`, a ready-flip via `deskflip` or `deskpost ready`) refuses when the dispatcher-attested tier is below the floor or the stamp is present but unreadable; an unstamped write (no stamp, an `any` tier, or a stamp aged out on `deskpost`'s paths; `deskflip` leaves a released claim's stamp standing) proceeds with a NOTICE, except that on the review path only an unstamped verdict on a risk-classed PR refuses. **Override:** the env toggle `DESK_MODEL_FLOOR_OVERRIDE` bypasses the floor, and with it the risk overlay, for incident recovery; every bypass prints a loud `MODEL-FLOOR-OVERRIDE` line. | `tools/desk/internal/deskkit/modelfloor.go:ModelCapabilityFloor` (called by `tools/desk/cmd/deskflip/flip.go` and `tools/desk/cmd/deskpost/ready.go`), `tools/desk/internal/deskkit/modelfloor.go:ModelCapabilityFloorRiskAware` with `tools/desk/internal/deskkit/modelfloorrisk.go:FloorRiskOf` (called only by `tools/desk/cmd/deskpost/review.go`) | S-review-verdict | `tools/desk/internal/deskkit` | Outside the loud override, an attested below-tier session cannot record a correctness judgement or flip a PR, and an unstamped review verdict on a risk-classed PR is refused rather than read as attested. | #1459 #1497 #1498 | `TestModelCapabilityFloorFourCases`, `TestRiskAwareFloorRefusesUnstampedRiskClassed` (deskkit); override: `TestModelFloorReviewOverrideProceedsLoudly` (deskpost), `TestModelFloorOverrideProceedsLoudly` (deskflip) — `proven-able-to-fire` | 2026-09-30 @ `43420f7ec` |
| R-stamp-age-out | A dispatch stamp whose dispatch claim is positively released is ignored (the PR reads as unstamped); a failed or impossible liveness read leaves the stamp standing. | `tools/desk/internal/deskkit/stampage.go`, `tools/desk/cmd/deskpost/claimliveness.go` | S-review-verdict | `tools/desk/internal/deskkit` | A dead cycle's stamp never blocks a PR harder than no stamp, and a stamp only ages out on positive evidence of release. | #1497 #1498 | `TestForeignStampAgesOutWhenClaimReleased` (deskkit) | 2026-09-30 @ `b89b39572` |
| R-body-secret-scan | **Trust boundary.** Every body `deskpost`, `deskpr` and `deskreply` would write is scanned for credential shapes and high-entropy runs and refused on a hit. **Override:** `deskpr` (create, update, edit) and `deskreply` take `--force-scan-override <reason>` (reason of at least 12 characters), which lets a refused body through only after writing an audit row; `deskpost` has no override, and the override never applies to the impersonation guard. | `tools/desk/internal/deskkit/bodycheck.go:BodyCheck`, `tools/desk/internal/deskkit/scanoverride.go:HandleScanRefusal`, `tools/desk/cmd/deskpost/internal/bodycheck/bodycheck.go` | S-publication-scan | `tools/desk/internal/deskkit` | No known credential shape leaves the desk verbs in an outward-bound body without either a refusal or an audited, reasoned override; loosening the entropy heuristic never un-refuses a token shape. | #1642 #1643 | `TestTokenShapesStillRefused`, `TestNewShapesStillRefuse`; override: `TestScanOverrideIsLoggedOrRefused` (deskkit) — `proven-able-to-fire` | 2026-09-30 @ `43420f7ec` |
| R-new-issue-budget | `deskfile new` is capped per session and repo at a rate over a rolling window (3 per 24h shipped), env-overridable; an unparseable override falls back to the default with a NOTICE, and `--force-file --reason` raises it for one filing while still charging an audit line. | `tools/desk/cmd/deskfile/deskfile.go:newBudgetConfig` (`defaultNewRate`) | none | `deskfile` | A runaway session cannot flood a tracker, a typo can never silently disable the cap, and an override never erases the audit trail. | #955 #1204 #1209 | `TestNewRateUnparseableFallsBackWithNotice`, `TestForceFileDoesNotResetTheRateCount` (deskfile) | 2026-09-30 @ `b89b39572` |
| R-inherited-token | **Trust boundary.** An inherited `GH_TOKEN` is honoured by `deskdispatch` only when verified to be the dispatching role's App (viewer login and bot id against the roster; on GitLab, the role's PAT custody); otherwise it is ignored with a NOTICE and the role token is minted, and an unreadable identity refuses before any claim. | `tools/desk/cmd/deskdispatch/dispatch.go:resolveClaimAuth` (`verifyInheritedToken`), `tools/desk/internal/deskkit/tokenidentity.go` | S-identity | `deskdispatch` | A dispatch never claims, cuts a worktree or writes under an ambient identity that is not the dispatching role's App. | #1145 #1274 #1631 #1650 | `TestInheritedNonRoleTokenIsIgnoredAndTheRoleTokenMinted`, `TestInheritedTokenWithUnreadableIdentityRefusesBeforeAnyClaim` (deskdispatch) — `proven-able-to-fire` | 2026-09-30 @ `b89b39572` |
| R-shim-ambient-token | **Trust boundary.** The cell launcher's `gh` shim hands the operator's ambient credential over as `CELLCTL_GH_AMBIENT`, never as `GH_TOKEN`, so no desk verb reads it as its own. **Boundary:** a `GH_TOKEN` or `GH_ENTERPRISE_TOKEN` the caller already set passes through untouched; verifying that token is `R-inherited-token`'s job, not this rule's. | `tools/desk/cmd/cellctl/shims.go` | S-identity | `cellctl` | The ambient credential never reaches a desk verb under the name desk verbs read. | #1145 #1631 #1650 | shell test `tools/cellctl/tests/gen-shims-gh-token.test.sh` (defaults to the shell oracle; run with `CELLCTL=<built tools/desk/cmd/cellctl>` for the Go port; no CI workflow runs it yet) — `proven-able-to-fire` | 2026-09-30 @ `43420f7ec` |
| R-authoring-not-delivery | A PR whose diff only authors a brief is not that brief's delivery (phantom check and planner both exempt it); any doubt keeps the PR counted as delivery. | `tools/desk/cmd/deskdispatch/authoring.go:dropBriefAuthoringPRs`, `tools/desk/cmd/fanoutloop/represented.go`, `tools/desk/internal/deskkit/briefauthoring.go` | S-delivery | `deskdispatch`, `fanoutloop` | An authored-but-unimplemented brief is dispatchable, and a landed delivery is never re-dispatched because of the exemption. | #1339 #1419 #1502 #1558 #1641 | `TestPhantomRefusesDocsOnlyDelivery`, `TestPhantomAdmitsAuthoredBrief` (deskdispatch) | 2026-09-30 @ `b89b39572` |
| R-authors-trailer-proof | **Trust boundary.** An `Authors:` trailer is refused at PR create unless the diff is authoring-only for every named id, and at flip time a non-authoring diff carrying it is risk-classed. | `tools/desk/cmd/deskpr/deskpr.go:authoringTrailerGate`, `tools/desk/internal/deskkit/briefrisk.go:AuthorsRiskFromBody`, `tools/desk/cmd/deskflip/flip.go` | S-review-verdict | `deskpr`, `deskflip` | A trailer that relaxes the security lane is honoured only when the diff proves the PR is authoring-only. | #1339 #1641 | `TestCreateRefusesAuthorsOnNonAuthoringBranch` (deskpr), `TestAuthorsTrailerOnNonAuthoringDiffRiskClasses` (deskflip) — `proven-able-to-fire` | 2026-09-30 @ `43420f7ec` |
| R-same-head-reapproval | **Trust boundary.** An APPROVED at an unchanged head over a standing CHANGES_REQUESTED does not clear it; only a new commit, a human dismissal, or a typed exemption (check-only, external-prerequisite, documented body-edit) does. | `tools/desk/cmd/deskflip/flip.go:standingCRRefusal`, `tools/desk/cmd/deskpost/ready.go` (`noOpApproval`), `tools/desk/internal/deskkit/bodyeditcr.go:BodyEditDeclared` | S-review-verdict | `deskflip`, `deskpost` | A reviewer cannot launder its own standing block by re-approving the same code. | #1601 #1602 | `TestApprovalOverAStandingBlockAtTheSameHeadIsRefused`, `TestBodyEdit_UndocumentedReApprove` (deskflip), the same-head no-op approval test in `tools/desk/cmd/deskpost/ready_test.go` — `proven-able-to-fire` | 2026-09-30 @ `b89b39572` |
| R-dr-ruling-link | **Trust boundary.** A decision record whose `decided-by` is the placeholder counts as ratified only through a `ruling:` link resolving to an unedited, human-mapped comment on that record's decision issue naming the record; otherwise it is a PROBLEM, and a failing link outranks a PR approval. | `statusgen/decisionruling.go` | S-decision-acceptance | `statusgen` | A placeholder decision is never read as a human ruling on the strength of text, a bot, another human or a PR approval. | #1395 #1500 #1564 #1571 | `TestRuling_PlaceholderWithoutRuling`, `TestRuling_WrongAuthorOtherHuman`, `TestRuling_FailingLinkBeatsPRApproval` (statusgen) — `proven-able-to-fire` | 2026-09-30 @ `43420f7ec` |
| R-onbehalf-not-stamp | An on-behalf-of attribution line is never matched as a sign-off stamp, and stripping it never hides a real stamp on the same line. | `statusgen/corroborate.go:stripOnBehalfOf` | S-decision-acceptance | `statusgen` | Attribution and sign-off stay distinct: neither can stand in for, or mask, the other. | #1335 #1337 | `TestStampsInDiff_OnBehalfOfDoesNotShieldRealStamp` (statusgen) | 2026-09-30 @ `b89b39572` |
| R-quoted-not-claim | Corroboration does not read quoted notation as a human claim (the removed side of a committed patch, stamp-shaped data in test source), while true positives still count. | `statusgen/corroboratescope.go` | S-decision-acceptance | `statusgen` | Quoting a stamp's notation is not making the claim, and the scope narrowing never drops a real one. | #1395 #1593 | `TestQuotedNotationIsNotAClaim`, `TestQuotedNotationTruePositivesStillCount` (statusgen) | 2026-09-30 @ `b89b39572` |
| R-origin-git-resolved | **Trust boundary.** `deskgit` gates fetch and push on the origin URL as git itself resolves it (`git remote get-url --all origin`), refusing a multi-valued list; every other go-git remote-URL read is on an allow-list. | `tools/desk/cmd/deskgit/deskgit.go:effectiveOriginURL` | S-identity | `deskgit` | The repo the gate approves is the repo git actually talks to. | #1617 #1623 #1638 | `TestFetch_MultiValuedOriginURL_FailsClosed`, `TestRemoteURLCallers_AllowListed`, `TestRemoteURLCallers_DetectsPlantedCaller` (deskgit) — `proven-able-to-fire` | 2026-09-30 @ `43420f7ec` |
| R-push-destination | **Trust boundary.** `deskpr` pushes only when git resolves exactly one push destination and it is an https URL naming the gated repo (or a local path); SSH, a sentinel, a multi-valued list, another transport or another repo is refused before the token mint. `deskmerge` gates its push destinations the same way. | `tools/desk/cmd/deskpr/pushdest.go:pushDestinationGate`, `tools/desk/cmd/deskmerge/merge.go:gatePushDestinations` | S-identity | `deskpr`, `deskmerge` | A role-App push never goes out under the operator's SSH key or to a repo other than the one gated. | #1623 #1638 | `TestPushDestMultiValuedRefuses`, `TestPushDestInsteadOfToSSHRefuses`, `TestPushDestOtherRepoRefuses` (deskpr) — `proven-able-to-fire` | 2026-09-30 @ `b89b39572` |
| R-origin-one-parser | `deskpr`, `deskwt`, `deskreply` and the shared preflight parse an origin remote (https, scp-like with an SSH host alias, hybrid) to its owner/repo through one shared parser. **Known exceptions:** `deskclaim-ref` keeps its own `parseRemote`, and `deskdispatch` resolves the repo from origin with its own `repoSlugFromURL` when `--repo` is omitted; neither is covered. | `tools/desk/internal/deskkit/remoterepo.go:ParseRemoteRepo`, `tools/desk/internal/deskkit/preflight.go`; exceptions: `tools/desk/cmd/deskclaim-ref/gogit.go:parseRemote`, `tools/desk/cmd/deskdispatch/exec.go:repoSlugFromURL` | S-worktree | `tools/desk/internal/deskkit` | The verbs that share the parser read the same repo from the same remote. Claim-acquire is outside this rule until both `deskclaim-ref` and `deskdispatch` move onto the shared parser. | #1371 #1372 #1470 #1474 | `TestParseRemoteRepo` (deskkit) | 2026-09-30 @ `43420f7ec` |
| R-forge-cli-ceiling | **Trust boundary.** No desk tool execs a forge CLI outside the `AllowedInvocations` permit, the permit count may not exceed `allowedInvocationCeiling`, and an exec whose argv[0] is not a constant is registered as could-not-check, never cleared. | `tools/desk/internal/forgeban/allowlist.go`, `tools/desk/internal/forgeban/check.go` | S-identity | `tools/desk/internal/forgeban` | No desk write silently runs under whatever ambient CLI identity is active; the permit only shrinks. | #1145 #1154 | `TestScannerCatchesEveryLaunchSpelling`, `TestScannerReportsUnresolvedRatherThanClean` (forgeban) — `proven-able-to-fire` | 2026-09-30 @ `b89b39572` |
| R-weight-ceiling | `tools/desk` verbs, flags, refusals and rule-text lines may not exceed `ceiling.txt` unless a `# grow` line directly above cites the driver's `grow <PR#>` reply (advisory mode at landing). | `tools/desk/internal/weight/ceiling.txt`, `tools/desk/internal/weight/weight.go` | none | `tools/desk/internal/weight` | The tool's surface grows only by an explicit, cited decision. | #1660 #1672 | `TestCeilingRedOnGrowthFixture` (weight) | 2026-09-30 @ `b89b39572` |
| R-dep-direction | No package under `tools/desk/internal/` imports a command, and no `cmd/<x>` package imports `cmd/<y>` for y ≠ x (`cmd/<x>/internal/...` belongs to x). Test files are not read. Ceiling 0, nothing grandfathered. | `tools/desk/internal/arch/arch.go:Direction`, run by `tools/desk/internal/arch/arch_test.go` | S-semantic-index | `tools/desk/internal/arch` | Shared code flows from commands into `internal/`, never back, so a command can be changed or removed without breaking another. | #1660 | `TestDependencyDirection`, `TestRulesFixture` (arch) | 2026-09-30 @ `b00888042` |
| R-hub-allowlist | The hub package `tools/desk/internal/deskkit` imports exactly the `internal/` packages listed in `hub-allow.txt`: an unlisted import fails, and so does a listed package it no longer imports; a line added after landing needs a `# grow` line directly above it. | `tools/desk/internal/arch/arch.go:HubAllow`, `tools/desk/internal/arch/hub-allow.txt` | S-semantic-index | `tools/desk/internal/arch` | Every package the hub imports is a dependency of every package that imports the hub, so the hub's dependency set grows only by an edit to the list, and a removed dependency cannot silently come back. | #1660 | `TestHubAllowList`, `TestRulesFixture` (arch) | 2026-09-30 @ `b00888042` |
| R-one-implementation | A `// semantic: S-<slug>` marker sits directly above a func declaration, names a row of the semantic index, and lies in a file that row's owner or duplicates cell names or in the package of a directory it names (a path naming `tools/desk`, its `cmd` or `internal` directory, or above names no place); the markers per row stay at or under `markers.txt`'s ceiling; an owner inside `tools/desk` carries at least one marker. Without the semantic index (a copy of `tools/desk` alone) the check skips as could-not-check. | `tools/desk/internal/arch/arch.go:Markers`, `tools/desk/internal/arch/arch.go:Ratchet`, `tools/desk/internal/arch/markers.txt` | S-semantic-index | `tools/desk/internal/arch` | A registered meaning gains a second declared implementation only by an edit to its semantic-index row. | #1660 | `TestOneImplementationPerMeaning`, `TestRulesFixture` (arch) — declared implementations only; undeclared duplicates are the design-fit review stage's | 2026-09-30 @ `b00888042` |
| R-design-fit-basis | A PR that grows a ratcheted dimension, adds an `R-` row, touches a brittle-marked module or reddens an `internal/arch` test gets the design-fit stage (right layer, should it exist, what it replaces) before correctness; a "no" is a finding with basis `design-fit`, recorded but not holding the PR while the finding-class register marks the class `advisory`, and holding it once that cell reads `blocking`. | `tools/desk/internal/deskkit/reviewscope.go:BasisDesignFit` (in `ScopeBases`), `tools/desk/cmd/deskdispatch/references/review-prompt.md` clause 3 and its `reviewscope` block, `plugins/assay/skills/pr-review-desk/SKILL.md` finding-class register | S-review-verdict | `tools/desk/internal/deskkit` | A design-fit finding is always an in-scope basis, never could-not-check, and whether it holds a PR is set in one register cell, not in the model. | #1660 | `TestReviewScopeKitMatchesModel`, `TestDesignFitIsAScopeBasis` (deskdispatch) | 2026-09-30 @ `848577851` |
| R-ambient-token-read | **Trust boundary.** No function in a shipped `tools/desk` Go file both names a forge-token environment variable (`GH_TOKEN`, `GITHUB_TOKEN`, `GH_ENTERPRISE_TOKEN`, `GITHUB_ENTERPRISE_TOKEN`, `GITLAB_TOKEN` — literal, constant, or a list it ranges over) and reads the environment, outside a register of named, reasoned permits whose length is pinned to `ambientTokenReadCeiling` (4 at landing; TARGET 0). A stale permit fails, so a migrated reader cannot stay listed. **Permits:** `deskadvisory` `ghToken`, `deskdispatch` `resolveClaimAuth` and `stepDecision`, `deskclaim-ref` `resolveToken`, each with its reason at the register. | `tools/desk/internal/deskkit/ambienttoken_guard_test.go` (`ambientTokenReadPermits`, `ambientTokenReadCeiling`) | S-identity | `tools/desk/internal/deskkit` | A desk tool never takes the identity it acts as from whatever the environment holds; the permit only shrinks. | #628 #2025 | `TestNoAmbientEnvTokenRead` (deskkit) — `proven-able-to-fire` | 2026-10-02 @ `ec260632e` |
| R-signoff-read-whole-thread | **Trust boundary.** A sign-off permalink is resolved by matching the comment id on the WHOLE thread of the item the link names: `ListCommentsTyped` walks every kind to the end or returns could-not-check (page cap, cursorless next page), never a first page, so "no such comment" (refused, exit 5) means absent. No shipped function compares a `.DatabaseID` with `==`/`!=` on a `ListComments` read, the untyped read whose GitHub change thread is still the first 100 comments. Ceiling 0, nothing grandfathered. | `tools/desk/internal/deskkit/forge_github.go:listCommentsGQL`, `tools/desk/cmd/deskmerge/authority.go`, `tools/desk/cmd/deskclose/authority.go`, `tools/desk/internal/deskkit/commentlookup_guard_test.go` | S-decision-acceptance | `tools/desk/internal/deskkit` | A valid sign-off is never refused as deleted because it sits past a read's first page, and a refusal names a comment that is truly not on the named item. | #2025 | `TestTypedCommentsWalkEveryKind`, `TestNoIDLookupOnFirstPage`, `TestIDLookupGuardSeesPlant` (deskkit); `TestSignOffRead*` (deskmerge) — `proven-able-to-fire` | 2026-10-02 @ `ec260632e` |

## Rule diet (monthly)

Once a month the diet reads the rule register and asks the driver, in one reply, which rules to
keep. It never deletes a rule automatically. **The cadence and the running role are project
values**: the project layer names them. The diet runs with the monthly brittle pass (brief 08):
one role, one cadence, two tables.

**Three-state catch status.** Every row reads exactly one of:

- `could-not-check`: the counter is blind or absent. That is not zero. A rule whose catch
  source is a telemetry class that is unconfigured or unread reads this state.
- `proven-able-to-fire`: zero catches in the window, but the row's named test shows it firing.
- `zero-without-proof`: zero catches, and nothing shows the rule can fire at all.

**Zero catches is an alarm, never a deletion.** A rule that never fires may be guarding a threat
that has not arrived yet, or it may be dead code that looks like a guard. The diet asks which;
it never assumes.

**Candidates.** A row is listed on the month's decision issue when any of these holds:

- its status is `zero-without-proof`;
- more than half of its recorded fires in the window were false positives;
- it is an orphan (its *serves* cell is `none`, so no semantic-index row owns its meaning);
- a justifying issue it cites was closed as not-planned.

A candidate is a question, not a verdict. Every other row is kept without asking.

**One decision issue per month, immutable options.** The role files ONE decision issue listing
the month's candidates, each with its id, status and why it qualified. The options are fixed
when it is filed. Editing them does not edit the issue: it supersedes it with a new issue that
links the old one, and only the new one counts.

**Reply grammar.** The reply is one line, either `retire R-a R-b; keep rest` or `keep`. It
counts only from the driver's own login (a project value), read from the forge's author field,
never from anything the text says about who wrote it (spec §4.7). A reply counts only while it
is unedited (the forge's `updated_at` equals its `created_at`), the same check the decision-record
ruling link makes; an edited reply is quarantined, and a correction is a new comment. When more
than one counted reply is on the issue, the latest one posted before the month closes is the
decision. No tool enforces these checks today: the running role reads author, `created_at` and
`updated_at` from the forge, and the design brief's review and the driver's merge are the layers
behind it. At month close the role records the counted reply's comment id and URL on the issue
and in any design brief it starts, so a later deletion cannot silently change which reply was
acted on. A reply from any other login is quarantined: it is noted on the issue and never acted
on. **If the month closes with no counted reply, every row is kept.**

**Retirement is a design brief, never an in-loop edit.** A `retire` reply starts a design brief,
usually one per class of rule, with its own review and the driver's merge. The diet role never
removes a row, a guard or a test itself.

**Trust-boundary retirement.** Retiring a row marked **trust boundary**, or any row whose
retirement would delete, disable or weaken a security or identity control (worker kit §2's list),
also carries:

- spec §4.2 rule 2: the brief names the layer that still refuses the same threat and carries a
  Verify row proving it with the retired layer absent; and
- worker kit §2: the retirement brief is `gate: human`, because removing a security control is
  a human decision.

**Single point of failure.** The author-and-unedited check on the reply (the driver's own login,
read from the forge, on a comment never edited after posting) is the one control that starts a
retirement. Behind it sit the design brief's own review
and the driver's merge, plus, for a trust-boundary row, the §4.2 rule 2 Verify row and the
`gate: human` path.

## Brittle marks

A **brittle mark** names a module where fixes keep landing, so the strong-tier investigation
and the design-fit review stage spend their attention there rather than across the whole
tree. The mark is a **two-key decision, never the metric alone**: the change-history metric
**nominates** first, the class defect history **confirms** second, in that order. A ranking
nobody acts on changes nothing, which is why every mark is bound to a next step — the brittle
investigation.

**The module.** The unit marked is a module: an `S-` owner path from the table above, or a
`tools/desk/cmd/<verb>` directory — **never a bare file**. A file is what the metric ranks; a
module is what gets marked, investigated and cleared.

**Key 1 — nominated by the metric.** The hotspot report (`tools/desk/internal/hotspot/hotspot.go`,
printed by `TestPrintHotspots` in `tools/desk/internal/hotspot/hotspot_test.go`) ranks every
non-test `.go` file under `tools/desk` by churn × complexity over first-parent history. A
module is nominated when at least one of its files is in the top `N%` by score (project
value; default 2%) with `fixes ≥ 2` in the window. Temporal coupling is an architecture
signal, not a defect predictor: it never nominates on its own. A module that only holds the
other end of a nominated file's coupling pair is `watch`, and the pair is recorded on the
nominated module's row for the investigation to read.

**Key 2 — confirmed by history.** An `error-class` issue records at least two counted
instances in the module, OR a chain arrow at `introduced-by-commit` lands in it (the
baseline's re-graded chains).

**States.** Both keys: `brittle`, a row below. Key 1 only: `watch` — reported, not marked.
Key 2 only: a class-issue matter, no mark. The pass that applies the keys is monthly, the
rule diet's sibling, run by the same role on the same cadence; the report itself is never a
CI gate (it needs full history, and on a shallow clone it reports could-not-check).

**Where a mark lands.** (a) A row in the table below. (b) A findings entry with id
`F-brittle-<module>` under the stream findings directory, `affects:` naming the streams whose
briefs touch the module and `resolved: false` — the board already renders such an entry under
"Unresolved findings" and counts it in the change-fail proxy, so no generator change is
needed; the first mark's entry creates that directory. (c) The label `brittle` on the class
issue (a label, not a status token).

**Clearing.** A mark is **cleared** when the investigation's recommendation has merged AND the
next monthly pass no longer nominates the module. The PR that clears it fills the row's
`cleared` cell and flips the findings entry to `resolved: true`.

No module is marked yet: the first marks are made at the first monthly pass after a project's
go-live.

| module | S- row | since | churn | fixes | complexity | coupling partner(s) | class issue | investigation | cleared |
|---|---|---|---|---|---|---|---|---|---|
