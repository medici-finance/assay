---
brief: assay:assay:verification-handoff:03
title: "Require an accountable intake disposition before repair scheduling"
why: "Intake is triaging real verification issues, but comments such as routed to worker or owned by brief author can leave no accepted successor work. New issue scanning also does not enforce that semantic readiness was assessed."
wave: 1
depends: ["verification-handoff/01"]
unblocks: ["verification-handoff/04"]
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
gate-why: "Changes the intake and scanner adapters boundary used to schedule, preserve or resolve verification work; human review must confirm that trust, ownership, retention and independent acceptance cannot be inferred from routing."
decision-trigger: start
decision-issue: 4307
design: DR-handoff-intake
ruling: "https://github.com/medici-finance/assay/issues/4307#issuecomment-6399188555"
issues: []
schema: brief-v2
version: 1
id: c2da5fe1-953a-4605-8bed-76f89fe848f7
authored: "2026-10-23 by assay-worker-app[bot]"
exec-tier: strong
exec-tier-why: "Cross-module shared policy, partial writes and independent actor/source binding require coordinated design and negative controls."
domain: complicated
sources:
  - "docs/streams/verification-handoff/README.md — controlling implementation scope"
  - "spec/intake-exit-v1.md; docs/streams/desk-supervision/repair-obligation-v1.md; docs/verify-wake.md"
  - "freshness-checked 2026-10-23 @ fbe0331e2fc9f5c822c4edeafb8a18865c916828"
consumers:
  - "deskcore/issueflow/intake.go: follow-up verification-handoff/03 (this delivery; adapter or contract extension)"
  - "tools/desk/cmd/scanloop/run.go: follow-up verification-handoff/03 (this delivery; adapter or contract extension)"
  - "tools/desk/cmd/scanloop/monitor.go: follow-up verification-handoff/03 (this delivery; adapter or contract extension)"
  - "tools/desk/cmd/scanloop/observations.go: follow-up verification-handoff/03 (this delivery; adapter or contract extension)"
  - "tools/desk/cmd/scanloop/landverb.go: follow-up verification-handoff/03 (this delivery; adapter or contract extension)"
  - "tools/desk/cmd/scanloop/handoffrecord.go: follow-up verification-handoff/03 (this delivery; adapter or contract extension)"
  - "statusgen/main.go: follow-up verification-handoff/03 (this delivery; adapter or contract extension)"
  - "statusgen/model.go: follow-up verification-handoff/03 (this delivery; adapter or contract extension)"
  - "statusgen/nextup.go: follow-up verification-handoff/03 (this delivery; adapter or contract extension)"
  - "statusgen/placeholder.go: follow-up verification-handoff/03 (this delivery; adapter or contract extension)"
  - "statusgen/registerentries.go: follow-up verification-handoff/03 (this delivery; adapter or contract extension)"
  - "statusgen/intakehandoffs.go: follow-up verification-handoff/03 (this delivery; adapter or contract extension)"
  - "spec/intake-handoff-v1.md: follow-up verification-handoff/03 (this delivery; adapter or contract extension)"
  - "docs/contracts.md: follow-up verification-handoff/03 (this delivery; adapter or contract extension)"
  - "statusgen/testdata/verification-handoff/mutate.py: follow-up verification-handoff/03 (this delivery; adapter or contract extension)"
  - "tools/desk/cmd/scanloop/adapter.go: follow-up verification-handoff/03 (this delivery; existing owner or planned extension)"
  - "tools/desk/cmd/scanloop/lane.go: follow-up verification-handoff/03 (this delivery; existing owner or planned extension)"
  - "tools/desk/cmd/scanloop/exitrecord.go: follow-up verification-handoff/03 (this delivery; existing owner or planned extension)"
  - "tools/desk/cmd/issueboard/: follow-up verification-handoff/03 (this delivery; existing owner or planned extension)"
  - "statusgen/scanissues.go: follow-up verification-handoff/03 (this delivery; existing owner or planned extension)"
  - "statusgen/intakeexits.go: follow-up verification-handoff/03 (this delivery; existing owner or planned extension)"
  - "spec/intake-exit-v1.md: follow-up verification-handoff/03 (this delivery; existing owner or planned extension)"
  - "plugins/assay/skills/intake-desk/SKILL.md: follow-up verification-handoff/03 (this delivery; existing owner or planned extension)"
  - ".cursor/skills/intake-desk/SKILL.md: follow-up verification-handoff/03 (generated harness copy of the canonical intake skill)"
  - ".cursor/assay-managed.json: follow-up verification-handoff/03 (generated harness content digest)"
  - "tools/desk/internal/reviewtest/issue_projection_test.go: follow-up verification-handoff/03 (named production fault controls)"
  - "statusgen/testdata/verification-handoff/03-mutations.json: follow-up verification-handoff/03 (this delivery; existing owner or planned extension)"
  - "changelog/verification-handoff-03.md: follow-up verification-handoff/03 (this delivery; existing owner or planned extension)"
---

# Brief 03 — Require an accountable intake disposition before repair scheduling

## Context

files: .cursor/skills/intake-desk/SKILL.md, .cursor/assay-managed.json, tools/desk/internal/reviewtest/issue_projection_test.go, deskcore/issueflow/intake.go, tools/desk/cmd/scanloop/run.go, tools/desk/cmd/scanloop/monitor.go, tools/desk/cmd/scanloop/observations.go, tools/desk/cmd/scanloop/landverb.go, tools/desk/cmd/scanloop/handoffrecord.go, statusgen/main.go, statusgen/model.go, statusgen/nextup.go, statusgen/placeholder.go, statusgen/registerentries.go, statusgen/intakehandoffs.go, spec/intake-handoff-v1.md, docs/contracts.md, statusgen/testdata/verification-handoff/mutate.py, tools/desk/cmd/scanloop/adapter.go, tools/desk/cmd/scanloop/lane.go, tools/desk/cmd/scanloop/exitrecord.go, tools/desk/cmd/issueboard/, statusgen/scanissues.go, statusgen/intakeexits.go, spec/intake-exit-v1.md, plugins/assay/skills/intake-desk/SKILL.md, statusgen/testdata/verification-handoff/03-mutations.json (planned), changelog/verification-handoff-03.md (planned)

facts:
- Baseline main fbe0331e2fc9f5c822c4edeafb8a18865c916828, inspected 23 October 2026; source was draft and stream was parked at authoring. Scope approval and stream activation take effect on the maintainer's merge of PR #4206; per-brief gates remain separate.
- Intake discovery sees ordinary verifier bugs; 14 of a bounded 28 recent verifier-filed issues carried explicit intake triage, not proof of claims or repairs.
- The default loop filing/repair sinks print dry-run results. The live skill uses sanctioned deskfile/deskevidence paths; a printed URL is not durable work.
- Existing intake-exit-v1 has two writers and exact-key conformance tests. Its local log is not a fleet-wide source of assignment authority.
- Current scanner/readers differ on addressed handling. Default fanout STATUS.md input has no issue labels; the worker inbox needs an actual read.
- deskcore is public and imported by desk-tools; statusgen does not currently import it. Shared policy packages are planned, not shipped.
- Brief schemas v1-v3 are supported; no milestone or b-prefixed operational records are introduced by this authoring.

layering: intake and scanner adapters owns this slice. Shared decision logic is an in-process public Go library call; no separate binary, daemon or subprocess protocol. Existing typed forge, custody, claims and evidence adapters own effects. No private import across modules and no new lifecycle database.

design-fit:
  owner: "intake and scanner adapters"
  contract: none — reuse the existing issue projection, intake-exit, repair-obligation and wake owners; update their canonical index on implementation.
  retires: ["implicit routing/assignment inference and the duplicate predicate or partial-success ambiguity named in Task"]
  weight: "New binaries 0; new services 0; bounded contract/library/adapter changes. Record actual rule, field, refusal and flag deltas at implementation review."
  why-add: "Reuse existing semantic owners; a shared meaning is needed across existing callers. A new process, queue or acceptance authority would add failure modes without closing the handoff."

single-point-of-failure: stale disposition or successor identity; independent trust/admission/claim and exact-source acceptance checks remain at effect boundaries.

## Read first

- [Controlling scope, coverage and owner decisions](README.md#implementation-scope).
- [Intake exit contract](../../../spec/intake-exit-v1.md).
- [Existing repair-obligation contract](../desk-supervision/repair-obligation-v1.md).
- [Wake and retry contract](../../verify-wake.md).
- [Library-first direction](../../library-first.md).
- [Semantic owners](../../contracts.md).

## Human decision

Confirm that the concrete implementation preserves independent verification and existing human-only decisions while changing how failed work is retained and assigned. A routing record must never grant acceptance or wider execution permissions.

Options:
1. Approve the bounded implementation after its independent review and verification.
2. Request corrections while retaining the existing holds.
3. Hold the proposed change.

Default if no answer: none — blocks until answered.

### Design for confirmation — DR-handoff-intake

Adopt the controlling scope and the shipped /01 contract, without adding an
assignment authority. Intake must retain the source event and the obligation to
decide after polling advances; a restart must reconstruct outstanding work from
durable observations and the existing forge/register owners. A polling cursor,
mechanical placeholder, exit log, routing comment or closed issue is insufficient
proof that a successor accepted the work.

Both intake writers keep `intake-exit-v1` byte-compatible. Accountable disposition
uses the separately negotiated companion contract, assessed through
`deskcore/issueflow` in-process. The issue lane and committed register expose the
same unresolved, scoping, repair, duplicate and external-prerequisite meanings.
Scoping needs a linked authoring task and a strong author; external work needs an
owner, action and wake condition; delivered code routes to qualification.
Scanner scheduling and issueboard consume that shared assessment while retaining
their independent trust, history, claim, human and acceptance checks. Legacy or
unreadable records remain classification-needed, never implicitly ready.

The proposed alternatives are to keep delta-only collection (loses unanswered
work), infer assignment from a label/comment/scan PR (confuses observation with
accountability), or add a central lifecycle database (duplicates existing owners).
This design chooses none of those. Its cost is a versioned companion read/write
path and explicit outstanding-work reconstruction; rollout must preserve old
records and negotiate reader support before enabling a required writer.

This is a proposal for `DR-handoff-intake`, not an approved decision record.
The decision issue must receive the driver's own comment naming this brief and
that record ID before a `design:` record can be claimed. The design gate and final
independent verification remain required. The source, approval metadata and
credited independent PASS for /01 are distinct from its still-unadvanced board row.

### Architecture pickup — 2026-10-24

Observed main: `385cb29e5682dcb9a886b298076f722b4823ec1b`.
The /01 public library and `verification-handoff-v1` contract are now shipped in
source; both consumer modules import `deskcore/issueflow` for selection, while
production callers do not yet consume its handoff assessment. The original authoring-time statements
above remain historical, rather than being rewritten as implementation evidence.

Inspected owners: `scanloop.SelectQueue`, the production monitor composition in
`tools/desk/cmd/scanloop/run.go`, `scanloop land` and `appendExitRecord`, the register export in
`statusgen/intakeexits.go`, `statusgen/scanissues.go`, placeholder eligibility in
`statusgen/nextup.go`, and issueboard classification. `SelectQueue` currently uses
only the monitor's current delta. An unresolved item with no new delta can vanish
after a restart. The current placeholder eligibility branch also accepts an
unblocked `todo` placeholder without a disposition assessment. These are the two
production boundaries the named Verify tests must exercise.

Pickup disposition: **hold implementation at the design gate; continue bounded
preparation and regression design**. No overlapping open implementation PR or
dispatch claim was found before this run took the /03 claim. The first scratch
restart regression reproduces an empty queue for quiet outstanding work and for
an unread outstanding-work input; this is a developer experiment, not the brief's
independent Evidence or a completed Verify row. The remaining regression work must
cover source/event identity, partial writes, duplicate retries, both writers,
successor/external obligations, and shared scanner/issueboard eligibility.

### Design confirmation and resumed pickup — 2026-10-25

**Ruling (2026-10-25): option 1.** The driver approved this brief and its bounded design, transcribed as [DR-handoff-intake](../decisions/DR-handoff-intake.md): https://github.com/medici-finance/assay/issues/4307#issuecomment-6399188555

This resolves the design hold in the dated pickup above. The new implementation
run starts from `617c0cd694d6b1698ec75f327ac6ac65fa434279`; the shared /01 delivery
and credited independent PASS remain available. Continue within the approved
Task, preserving the original Evidence and all final acceptance and activation
gates. The 2026-10-24 experiment remains developer preparation, not Verify credit.

## Ground rules

- Isolated feature branch and draft PR; no merge, source/stream/brief/PR-ready flip, release, installed pin change, workflow dispatch or live infrastructure effect in this brief.
- Recheck current main, claims, active PR overlap, controlling sources, semantic owners and module imports at pickup/resume; record observed revisions/coverage and continue, adapt or hold. A future API or draft spec is not shipped support.
- Preserve actor/authority separation, legacy identities and immutable Evidence/outcomes. Do not fabricate intake acknowledgement, assignment, historical execution or verified/done from labels or issue closure.
- New shared parsing/policy must be direct library calls, never a new helper executable. Preserve existing online read/custody boundaries.

## Task

1. Use /01 to expose observed-but-unacknowledged, disposition-owed and successor-owed blockers in existing intake reads. Do not lose an inbound item when polling advances or a session restarts. Acknowledge only through the existing authorized issue/record path and retain source-event identity, deciding role and evidence. Do not treat raised-by as trust or ownership and do not infer triage from issue closure, label presence or a mechanical scan PR.
2. Extend the existing intake judgment/land interface to record one current accountable disposition with a real successor or explicit owner/action; preserve per-event history. Scope/design-needed work queues author-spec/author-brief with a strong author and a linked authoring task/PR; only a sufficiently scoped direct repair can proceed as issue work. A source fix already delivered routes to installation/qualification rather than another implementation brief. External prerequisites require role, exact next action and wake condition; human/risk decisions keep their existing independent gate.
3. Make scanner placeholder scheduling and issueboard projections consume /01 rather than differing label predicates. Raw intake representation may exist before judgment, but must not be dispatchable repair work until its required disposition is settled. Keep untrusted, could-not-check, needs-decision and addressed work outside the ordinary ready lane. Old records lacking readiness remain visibly classification-needed; no bulk promotion or destructive regeneration.
4. Reconcile issue-lane and committed-register writers with the canonical versioned intake-exit contract, including restart/repeated-event behavior. A closed duplicate points to the existing owner and work; a stopped scan carrier leaves successor-owed work visible instead of treating the triage comment as a completed transfer. No new automatic judgment engine, global queue or unconditional to:intake stamping.

## Interface and completion contract

DoD: deliver the production behavior, regression and fault-injection coverage named below; route every affected caller to the canonical owner; update the stated contract/operator guidance and changelog. A contract declaration or function import alone is insufficient. Consumer routes are prospective authoring obligations, not already-correlated implementation evidence. Keep unknown/unreadable distinct from empty/ready, and retain explicit missing source, external owner and activation holds.

## Verify

These named tests and mutation manifests are FUTURE DELIVERABLES, not authoring-time evidence. Each exercises the actual entrypoint/adapter, with the wrong-but-plausible mutation named in Expect. Implement executable mutation commands in the declared manifest and record fail-first/red/green observations. The named PASS assertion prevents an absent test selector from passing.

| # | Class | Shell | Command | Expect |
|---|---|---|---|---|
| 1 | check:ci +flow +mutation | sh | `(cd tools/desk && handoff_out=$(mktemp "${TMPDIR:-/tmp}/handoff-verify.XXXXXX") && trap 'rm -f "$handoff_out"' 0 && GOWORK=off go test -count=1 -v -run '^TestIntakeHandoffDisposition$' ./cmd/scanloop > "$handoff_out" && grep -q -- '--- PASS: TestIntakeHandoffDisposition ' "$handoff_out")` | exit 0 and named PASS; actual lane/land path distinguishes needs-authoring, simple repair, already-delivered qualification and external prerequisite; restart retains unacknowledged work; a bare routing comment or mechanical placeholder cannot satisfy readiness. |
| 2 | check:ci +flow +mutation | sh | `(cd statusgen && handoff_out=$(mktemp "${TMPDIR:-/tmp}/handoff-verify.XXXXXX") && trap 'rm -f "$handoff_out"' 0 && GOWORK=off go test -count=1 -v -run '^TestScanHandoffAdmission$' . > "$handoff_out" && grep -q -- '--- PASS: TestScanHandoffAdmission ' "$handoff_out")` | exit 0 and named PASS; actual scanner and intake export preserve legacy records as unknown and align ordinary/addressed/held eligibility with issueboard fixtures; scheduling before disposition or ignoring the addressed exclusion fails. |

## Evidence

Independent non-implementer: gpt-6-astra, strong, dispatcher attestation [#4606](https://github.com/medici-finance/assay/issues/4606). Actual source `5e65f6de3585909c96fd8004552be6cc33ef6b3c`. Native admission was checked before execution, after preparation and before this Evidence edit. The home and execution clone were clean at that source before the Evidence-only edit. No test, command, Expect cell, board, index or deliverable was changed.

Verification-Table: delivery=0426c579e3a2bacd843381e9d57d612befff7bd2 base=7445d569bbe9c9e07e1ed90804530e99c88ab50d original=7445d569bbe9c9e07e1ed90804530e99c88ab50d

The delivery is [PR #4413](https://github.com/medici-finance/assay/pull/4413), first parent resolved through the native forge read and Git object. Native re-derivation reports two from-base rows and zero author-added rows. Original, delivered and admitted Verify sections are byte-identical (SHA256 `130507534528f21c3801a164c1ea6cec22ff57e5d00ae3d8b7f596943bff0cba`). The historical named tests are absent at the base: each exact command's named-PASS guard fails there. Those red runs prove exact-command discrimination, not injection of the new test bodies into old code; the 19 production mutations provide behavioral discrimination.

### Native execution witness

The following table and four binding lines are the closed native witness bytes, copied without rewriting Runner cells. The native bracket digest reports `verdict=match`, computed/recorded SHA256 `200a0fb935880ca8fb22291676ed4168a538c4ed1aa7651e8441e44e7ee01585`. This local digest match is not landed per-line credit; the publishing desk must preserve the witness and complete native landing checks.

| # | Command | Result | Output | Date | Runner | Base |
|---|---------|--------|--------|------|--------|------|
| 1 | `(cd tools/desk && handoff_out=$(mktemp "${TMPDIR:-/tmp}/handoff-verify.XXXXXX") && trap 'rm -f "$handoff_out"' 0 && GOWORK=off go test -count=1 -v -run '^TestIntakeHandoffDisposition$' ./cmd/scanloop > "$handoff_out" && grep -q -- '--- PASS: TestIntakeHandoffDisposition ' "$handoff_out")` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-10-27 | assay-verifier-app[bot] @ 5e65f6de3585 (on-behalf-of human:ian) (forge-identity) | base=7445d569bbe9 red rc=1 · head=5e65f6de3585 |
| 2 | `(cd statusgen && handoff_out=$(mktemp "${TMPDIR:-/tmp}/handoff-verify.XXXXXX") && trap 'rm -f "$handoff_out"' 0 && GOWORK=off go test -count=1 -v -run '^TestScanHandoffAdmission$' . > "$handoff_out" && grep -q -- '--- PASS: TestScanHandoffAdmission ' "$handoff_out")` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-10-27 | assay-verifier-app[bot] @ 5e65f6de3585 (on-behalf-of human:ian) (forge-identity) | base=7445d569bbe9 red rc=1 · head=5e65f6de3585 |
Tree-Digest: sha256:058190f77a79abd3d81329095746a642499b596626ebb385db9e9990bef0a07d
Container-Image: sha256:3a3cb3913f1311f1a9bf13ddd53c4f27361566f74ac90c89f78991e5452644b8
Verification-Attestation: medici-finance/assay#4606 run=dad16689ac0e7b1214c0a2e40bf2366496d6456744a1cfc665713537611e650a source=5e65f6de3585909c96fd8004552be6cc33ef6b3c brief=docs/streams/verification-handoff/brief-03-require-an-accountable-intake-disposition-before-repair-scheduling.md model=gpt-6-astra tier=strong
Admission-Bracket: run=dad16689ac0e7b1214c0a2e40bf2366496d6456744a1cfc665713537611e650a source=5e65f6de3585909c96fd8004552be6cc33ef6b3c tree=sha256:058190f77a79abd3d81329095746a642499b596626ebb385db9e9990bef0a07d witness=sha256:200a0fb935880ca8fb22291676ed4168a538c4ed1aa7651e8441e44e7ee01585 opened=2026-10-27T15:24:35Z closed=2026-10-27T15:34:02Z before=receipt after=check-verifier

### Execution envelope and observed output

Original-row execution began 2026-10-27T15:24:38Z and ended 15:26:28Z; controls and fixed reruns finished 15:30:23Z. Host bracket opened 15:24:35Z and closed 15:34:02Z. Darwin cannot provide the host network-namespace lane, so the installed native closed container lane was used. The operator-pinned already-present image was `ghcr.io/medici-finance/verify-runner:v1.0.63@sha256:3a3cb3913f1311f1a9bf13ddd53c4f27361566f74ac90c89f78991e5452644b8` (linux/amd64). Native statusgen v1.0.63, Git 2.56.0, Go 1.27.2; no runner was built, pulled or substituted. Container inspection records network none, read-only root filesystem, nonroot uid/gid, all capabilities dropped, no-new-privileges, 4 GiB memory and 1024 process limit. Sources were copied exactly; mutation snapshots and private writable caches were outside the admitted home. Loopback typed-forge fixtures were confined inside that network-none container.

Five exact, already-cached host modules (x/sys v0.49.0, x/text v0.43.0, x/term v0.47.0, x/crypto v0.58.0, x/net v0.61.0) supplemented the image cache through a read-only input mount; writable copies were owned by this task. Their expanded-tree, zip and mod hashes match the committed go.sum using Go's Hash1 algorithm; their existing ziphash files agree. The bounded input manifest records 2303 files / 65,998,119 bytes and per-file SHA256 provenance. GOPROXY=off, GOSUMDB=off and GOTOOLCHAIN=local with network none prevented downloads. This is no claim of a complete `go mod verify`: that command attempted unrelated uncached x/image metadata and was stopped; only exact required cached inputs were validated.

Both original shell commands intentionally redirect the verbose test output to temporary files, assert named PASS with grep, and remove the files. Their empty exposed output hashes are therefore expected. Supplemental clean reruns retained the actual lines `--- PASS: TestIntakeHandoffDisposition (0.08s)` and `--- PASS: TestScanHandoffAdmission (0.05s)` with exit 0. The full issueboard package also passed, separately labelled supplemental. No supplemental command replaces an original row.

### Declared mutation coverage

Executed the unchanged manifest command `python3 -I statusgen/testdata/verification-handoff/mutate.py --manifest 03-mutations.json` in the same admitted network-none container: exit 0. All 19 targets matched exactly once. The unchanged runner validates a nonzero child exit, the named FAIL and each expected diagnostic; it emits selected diagnostic lines, not each full child stream or numeric child exit. Every case has a passing shared module/test baseline and passing shared clean post-mutation rerun. This does not claim 19 independent fixed reruns. Fourteen cases discharge row 1's controls and five discharge row 2's.

| Control | Original row | Mutant observed; baseline/fixed |
|---|---|---|
| `production-retention` | 1 | caught: `handoff_test.go:41: production quiet work lost: err=<nil> output=scanloop run — intake-desk drain`; clean / clean |
| `pre-poll-retention` | 1 | caught: `handoff_test.go:185: restart 0: items=[] err=<nil>`; clean / clean |
| `partial-write-retry` | 1 | caught: `handoff_test.go:93: retry: [] <nil>`; clean / clean |
| `strong-author` | 1 | caught: `=== RUN   TestIntakeHandoffDisposition/accountable_dispositions_and_partial-write_retry/scoping_requires_strong_author`; clean / clean |
| `external-wake` | 1 | caught: `=== RUN   TestIntakeHandoffDisposition/accountable_dispositions_and_partial-write_retry/external_wake_missing`; clean / clean |
| `successor-existence` | 1 | caught: `=== RUN   TestIntakeHandoffDisposition/accountable_dispositions_and_partial-write_retry/missing_successor_file`; clean / clean |
| `placeholder-readiness` | 2 | caught: `intakehandoffs_test.go:36: raw scanner representation became work`; clean / clean |
| `addressed-admission` | 2 | caught: `intakehandoffs_test.go:76: to:worker entered ordinary work`; clean / clean |
| `closed-raw-retention` | 2 | caught: `intakehandoffs_test.go:44: closed raw intake disappeared before disposition`; clean / clean |
| `reopened-event` | 2 | caught: `intakehandoffs_test.go:90: reactivation reused an old disposition`; clean / clean |
| `direct-repair-target` | 2 | caught: `intakehandoffs_test.go:69: source placeholder dispatched for successor repair/01: ready=true want=false`; clean / clean |
| `settled-discharge` | 1 | caught: `handoff_test.go:240: pass 1: settled #77 queued again: [medici-finance/assay#77 medici-finance/assay#78]`; clean / clean |
| `rejection-discharge` | 1 | caught: `handoff_test.go:328: pass 1: rejected/watched must leave replay and the owed events must be presented: [medici-finance/assay#80 medici-finance/assay#81]`; clean / clean |
| `rejection-judgment-only` | 1 | caught: `handoff_test.go:328: pass 1: rejected/watched must leave replay and the owed events must be presented: [medici-finance/assay#83 medici-finance/assay#84]`; clean / clean |
| `replay-cap` | 1 | caught: `handoff_test.go:279: replay not bounded oldest-first: [medici-finance/assay#101 medici-finance/assay#102 medici-finance/assay#103 medici-finance/assay#104 medici-finance/assay#105]`; clean / clean |
| `quarantine-retire` | 1 | caught: `handoff_test.go:434: pass 2: retired item replayed: {Item:{HandoffState:disposition-owed HandoffRoute: HandoffSuccessor: Replayed:true Repo:medici-finance/assay Number:78 UpdatedAt:2026-08-24 11:51:00 +0000 UTC Raw:medici-finance/assay#78 2026-08-24T11:51:00Z} State:QUARANTINED Author:mallory Why:untrusted author with no standing blessing — visible and counted, never routed; one comment from ada admits it gone:false}`; clean / clean |
| `gone-retire` | 1 | caught: `handoff_test.go:430: pass 2: blind=[medici-finance/assay#77 medici-finance/assay#79] (gone item must not blind every pass; transient must)`; clean / clean |
| `gone-degraded-guard` | 1 | caught: `handoff_test.go:475: pass 2: a not-found under a degraded read was retired: []`; clean / clean |
| `transient-not-gone` | 1 | caught: `handoff_test.go:430: pass 2: blind=[] (gone item must not blind every pass; transient must)`; clean / clean |

### Original obligation trace

The original two Verify rows remain byte-equivalent at original/delivery/admitted source. `deskpathguard rederive --main 7445d569bbe9c9e07e1ed90804530e99c88ab50d` reported two from-base rows, zero author-added. Actual delivery is the single-parent squash 0426c579e3a2bacd843381e9d57d612befff7bd2, confirmed by native forge read and local Git object. Full-table marker uses delivery=0426c579e3a2bacd843381e9d57d612befff7bd2 base=7445d569bbe9c9e07e1ed90804530e99c88ab50d original=7445d569bbe9c9e07e1ed90804530e99c88ab50d. Live delivery labels omit wrote-to-the-test, but the current-head reviewer explicitly reports the protected-path guard finding it; this pass retains the stronger provenance accounting, not an exemption inferred from the missing label.

| Obligation | Production owner and trace | Original row / control |
|---|---|---|
| Quiet work survives poll/restart | run.go wires retainingMonitor; observations.go saves prior baseline under audit lock before poll, then retains after+delta; adapter.SelectQueue consumes reconstructed items | 1; production-retention, pre-poll-retention |
| Accountable event/actor and successor | cmdLand binds item/opened/lane; landHandoff assesses shared IntakeFacts then checks successor through typed forge or confined brief owner | 1; successor-existence; named subtests role spoofing/source event changed |
| Strong authoring vs direct repair vs qualification vs external | issueflow.AssessIntakeHandoff interprets route; writer additionally checks strong deciding tier and linked brief; explicit external action+wake | 1; strong-author, external-wake; named delivered-source subtests |
| Partial writes/repeated judgments | landHandoff retries companion after existing immutable exit; appendHandoffRecord requires exact revision and idempotent deep-equal retry | 1; partial-write-retry |
| Settled/latest-event/rejection discharge | observations.go latest map chooses newest issue event; settledHandoff and judgedRejection bind event before replay exclusion | 1; settled-discharge, rejection-discharge, rejection-judgment-only |
| Explicit replay bound and retention | replayCap shares existing knob, sorts oldest-first, emits deferred count while preserving history | 1; replay-cap |
| Independent trust/unknown vs empty | SelectQueue trust read still runs; retirement requires quarantined or definite gone+nondegraded; transient read remains owed | 1; quarantine-retire, gone-retire, gone-degraded-guard, transient-not-gone; current-trust subtest |
| Raw/legacy scanner does not schedule work | renderPlaceholder remains classification-needed; parsePlaceholderFile reads confined companion; toBrief carries assessment; eligibleBase requires settled direct repair | 2; placeholder-readiness, direct-repair-target |
| Addressed and held exclude ordinary work | shared WorkReady selector sees addressee/decision/help/exclusion; a placeholder flag is explicitly cleared before assessment | 2; addressed-admission; labels to:worker, to:, needs-decision, question, help wanted, review-request |
| Closed unclassified and reopened source preservation | planScan keeps unknown closed source visible, applyCloseOut removes stale handoff/opened pointer on reactivation | 2; closed-raw-retention, reopened-event |
| Both committed register exports | writeIntakeHandoffs emits legacy classification-needed, later observed companion; writeIntakeExits bytes compared before/after | 2 named test register section; v1 output unchanged |
| Issueboard agreement and independent reads | assessIssueHandoff calls same shared assessment with current issue labels, does not self-corroborate optional verification; board's WorkReady filter uses RepairDispositionSettled; closed-source path retains unclassified placeholder | supplementary full issueboard package; neither named row invokes this binary itself |

Nineteen controls map as fourteen to row 1, five to row 2. Each manifest target matched exactly once at admitted source before mutation. Mutation snapshots use the declared bounded tracked-file list and temporary directories outside the source; no target in the admitted home is edited. The baseline and fixed run for each case are the actual module/test baseline and post-mutation clean rerun, shared among that module's cases; the coverage JSON names this explicitly rather than claiming nineteen independent fixed reruns.

### Limits

The exact baseline command's named-PASS guard discriminates test absence as well as behavioral failure. The pre-delivery base predates the named tests; the fail-first reds therefore establish that these exact acceptance commands fail before delivery, not that these new test bodies were injected into old code. The executable source mutations provide the behavioral discrimination. No original command was replaced to strengthen that historical claim.

The issueboard package supplement covers production fixtures separately. The four internal/reviewtest integration tests are prior-review evidence, not executed by the original two rows or automatically credited in this pass. No release, installed-cell rollout, production endpoint, writer activation, live claim, successor acceptance, second model stamp or human final acceptance is established here. The typed forge fixtures run on loopback inside network-none; live trust/custody/claim gates are not exercised against production.

Prior A5/A6/A7/A8/N4/N5 remain explicitly carried. Failure/preflight attempts before the successful original-row run retain their own logs and timestamps; they are never relabelled passing witnesses.

### Risk-bearing values: enumerate, rank, derive

The mechanical inventory covers all 39 paths changed by delivery 0426c579e3a2bacd843381e9d57d612befff7bd2 against first parent 7445d569bbe9c9e07e1ed90804530e99c88ab50d: 1006 added/changed literal-bearing lines, including 400 production lines, with every captured string/raw string/number/boolean/nil token and exact delivery line retained in literal-inventory.json and literal-inventory.md. Repeated strings, fixture timestamps and generated metadata are deliberately retained. This is a lexical superset, not a claim that each fixture/string is an authority. Full diff is retained for checking tokens and context. A supplemental lexical scan records 3 added lines containing single-quoted strings/runes in literal-inventory-singlequotes.json, including Python literals and the record terminator; quoted prose may be overcaptured. The two inventories together define the enumeration scope; no AST exhaustiveness claim is made. Added prose constants in the brief and specification remain in the same inventory. New post-delivery scanner risk-gate notice is recorded separately in post-delivery-production.diff; it does not alter either Verify command or the handoff predicates.

Rank 1 is authority/event/readiness bindings: a wrong value could schedule the wrong repair, suppress an owed event or grant a false apparent completion. Source/code errors are reversible by correction, but already suppressed work needs reconciliation; neither passing tests nor this report grants human acceptance. Rank 2 is durable record identity and persistence permissions: a wrong version or mode can lose history or expose local evidence. Rank 3 is operational bounds and output/fixture values: reversible tuning, no independent derivation required by the risk-value procedure. No changed money-transfer, settlement ratio or billed-provider price is present.

#### Top-ranked exact values and derivations

- RISK-VALUE: DERIVED — IntakeHandoffSchema = "intake-handoff-v1" @ deskcore/issueflow/intake.go:14 — the separately negotiated spec/intake-handoff-v1.md Record table fixes this exact version, avoiding changes to the existing intake-exit-v1 keys; unknown versions remain classification-needed.
- RISK-VALUE: DERIVED — minimum revision = 1 @ deskcore/issueflow/intake.go:72; next revision increment = 1 @ tools/desk/cmd/scanloop/handoffrecord.go:65 — the Record contract requires positive per-event revisions beginning at 1. Comparing item plus opened and incrementing exactly once gives an ordered refinement history; deep-equal retries preserve the previous revision, conflicting retries refuse.
- RISK-VALUE: DERIVED — author_tier = "strong" and authoring route = "authoring" @ deskcore/issueflow/intake.go:119 — the brief Task 2 explicitly requires a linked strong author for scope/design work. Writer checks also require a strong deciding session (handoffrecord.go:100) and linked brief ExecTier (handoffrecord.go:217); the string is not inferred from a label.
- RISK-VALUE: DERIVED — allowed external owners = "human", "operator", "brief-author", "verifier" @ deskcore/issueflow/intake.go:105 — exact closed vocabulary in the companion Record table. The external branch additionally requires route="external", empty successor, and nonempty action/wake, so an external prerequisite cannot quietly become dispatchable repair.
- RISK-VALUE: DERIVED — delivered route = "qualification" @ deskcore/issueflow/intake.go:97 — Task 2 states already-delivered source routes to installation/qualification rather than another implementation. Wrong route returns successor-owed, not a passing state.
- RISK-VALUE: DERIVED — ordinary repair admission state = "observed", route = "repair" @ deskcore/issueflow/intake.go:147 — both literals plus DirectRepair must hold. DirectRepair additionally binds disposition Repair and case-insensitive same item/successor (line 140). This derives from Task 3's prohibition on scheduling representations before disposition and avoids scheduling both a routed successor and its source placeholder.
- RISK-VALUE: DERIVED — settled replay states = "observed" or "external-wait" @ tools/desk/cmd/scanloop/observations.go:75 — spec Observation retention states these exact judgments discharge a latest event, with next-event re-presentation. They discharge this routing decision only; they never grant assignment completion, verification or broader permission.
- RISK-VALUE: DERIVED — accountable rejection source = "issue" and nonempty opened = "" exclusion @ tools/desk/cmd/scanloop/observations.go:84 — spec requires the same issue event and judgment-decided rejected-watching exit; no mechanical discharge. Empty event time cannot establish that identity. ExitRejectedWatching and decidedJudgment are unchanged pre-existing literals used here, not newly invented values.
- RISK-VALUE: DERIVED — terminal retirement literals = "gone" and "quarantined" @ tools/desk/cmd/scanloop/observations.go:41-42 — the spec explicitly separates definite not-found in a clean repository read from untrusted/unblessed quarantine. adapter.go:339-342 retains the degraded-read guard; trust.go:124 uses existing IsForgeNotFound and HTTP StatusGone (410). A8's access-change ambiguity remains an advisory, not proved away by these literals.
- RISK-VALUE: DERIVED — handoffRecordsFile = "intake-handoffs-v1.jsonl" @ tools/desk/cmd/scanloop/handoffrecord.go:19; observationFile = "intake-observations-v1.json" @ tools/desk/cmd/scanloop/observations.go:18 — separately versioned existing-owner files preserve the old exit writer, and the observation format never becomes claim or acceptance authority.
- RISK-VALUE: DERIVED — companion history mode = 0600 @ tools/desk/cmd/scanloop/handoffrecord.go:72; observation directory mode = 0700 @ tools/desk/cmd/scanloop/observations.go:148; archived companion mode = 0600 @ statusgen/scanissues.go:1135 at admitted source — owner-only local persistence is consistent with sensitive-data=yes and does not expose evidence to other local users by default. Existing directories retain their existing modes; this is not a recursive permissions hardening claim.

#### Reversible knobs and non-authority literals

- replay default = 25 @ tools/desk/cmd/scanloop/observations.go:67 and positive override bound >0 at line 64. Spec explicitly reuses INBOUND_MONITOR_BURST_CAP with this default; old events beyond the cap remain durable and named deferred. A6 and A7 remain; no throughput claim or starvation-freedom proof is inferred.
- companion scanner initial buffer = 65536; maximum = 1024*1024 @ tools/desk/cmd/scanloop/handoffrecord.go:32. Exceeding the read bound produces an error, not empty/ready. These reversible operational bounds are named without inventing a domain derivation.
- atomic observation temporary-file prefix ".intake-observations-*" at observations.go:151 and record terminator '\n' at handoffrecord.go:76 are persistence encoding/atomic-write plumbing; full inventory retains them and their surrounding error branches.
- The mutation runner's 16000-file / 96-MiB snapshot cap, four-minute test timeout and 270-second subprocess timeout are diagnostic bounds, not execution/acceptance authority. Fixture dates, repository names, IDs, counts and failure messages are test inputs/oracles; the inventory lists them without promoting them to policy.

#### Review residuals explicitly retained

A5: observation type comment still says clean quarantine read, omitting overflowed threads. A6: deferred items can coexist with exit 0 because Blind counts degraded/burst reports. A7: retained history has no pruning policy. A8: access-change 404 between poll and probe can retire an event; first pass still records could-not-check. N4: archive copy leaves source companion JSON, and retained history grows. N5: static placeholder classification-needed line can go stale. Current-head security review keeps A5-A8 open advisory; current-head correctness approval keeps N4/N5 nonblocking. No merge or passing run resolves these by implication.

### Retained unsuccessful attempts and closure hold

Earlier attempts remain uncredited diagnostics: (1) the disposable clone's stale origin/main caused native fail-first to refuse the real delivering first parent, exit 2, zero rows; an ordinary local Git fetch of the independently checked canonical source ref repaired only that clone's remote-tracking input; (2) the bare image mutation baseline lacked current dependencies, exit 1, no credited controls; (3) broad go mod verify attempted unrelated uncached x/image metadata and exited 1 before rows. A fresh native bracket/witness was opened for the successful attempt. No earlier witness was relabelled.

The first host close from the house cwd refused with the misleading message “the receipt admits brief docs/streams/verification-handoff/brief-03-require-an-accountable-intake-disposition-before-repair-scheduling.md, but this run's brief is docs/streams/verification-handoff/brief-03-require-an-accountable-intake-disposition-before-repair-scheduling.md”. Read-only source inspection identified relWithin(home, brief) resolving the relative caller path. The same native close from the exact admitted home with the actual absolute brief succeeded, without editing receipt or guard. An initial digest check found no bracket as expected; one later digest call used positional syntax and refused; the corrected documented --witness call matched. All original diagnostics remain retained.

No original row or declared mutation remains unrun. The four internal/reviewtest integration tests referenced in prior review were not run in this pass and receive no new credit. Production infrastructure, released installation and live authorization/custody were not probed. Advisory A5/A6/A7/A8/N4/N5 remain open as described above. Design/start approval #4307 is not final acceptance. The human gate and independent review/floor obligations remain; only one actual dispatcher-bound model stamp is claimed. The publishing desk must land these Evidence bytes via its native branch path, retain the gate/lifecycle hold and route the Evidence PR to independent review. No native verified outcome is eligible before its required landed Evidence/status/stamp and human gate conditions hold.

**VERIFY: PASS** — 2/2 original acceptance rows passed; both exact-command fail-first runs were red; all 19 declared production mutations were caught. Human acceptance remains open. This result neither changes lifecycle state nor grants installation, activation, claim, routing, successor acceptance, or a second verifier/model stamp.
### Second independent stamp: claude-opus-5-5, strong (cross-model)

Independent non-implementer on a different model family from the first stamp: claude-opus-5-5, tier strong, dispatcher attestation [#4617](https://github.com/medici-finance/assay/issues/4617). Admitted source is merged main `3a140e11192866bf4c06bae7a318333d576d6f09`, which already contains the first stamp's landing (179b33db1ba5ad48646453222ec47cbad1eaa2fd). `deskdispatch --check-verifier` admitted run `3123a6bc4cdcb2d06d32d030d90479220cc01c3c3ac9fc517d2a35957de70346` (model claude-opus-5-5, tier strong) before any row and before this Evidence edit. The first verifier's block above is left as it was; nothing below reuses its witness, logs or inventory. No test, command, Expect cell, stream index or deliverable was changed.

Re-derivation for this pass: delivery is the single-parent squash 0426c579e3a2bacd843381e9d57d612befff7bd2 ([PR #4413](https://github.com/medici-finance/assay/pull/4413)) on the first-parent chain of the admitted source; its parent is 7445d569bbe9c9e07e1ed90804530e99c88ab50d. `deskpathguard rederive --main 7445d569bbe9c9e07e1ed90804530e99c88ab50d` reported 2 rows, both from-base, 0 author-added. The Verify section bytes are identical at base, delivery and admitted source (one SHA256 for all three, computed over the section body). The protected-table marker for this run, independently re-derived:

Verification-Table: delivery=0426c579e3a2bacd843381e9d57d612befff7bd2 base=7445d569bbe9c9e07e1ed90804530e99c88ab50d original=7445d569bbe9c9e07e1ed90804530e99c88ab50d

#### Container execution witness (closed bracket)

These are the closed witness file's bytes as the bracket steps wrote them. Host open at 18:39:06Z, container rows with fail-first against the delivery parent, host close at 18:45:06Z; `statusgen verifyrun --bracket-digest` on the closed file reported `verdict=match` with `span-lines=tree-digest:1,container-image:1,attestation:1`.

| # | Command | Result | Output | Date | Runner | Base |
|---|---------|--------|--------|------|--------|------|
| 1 | `(cd tools/desk && handoff_out=$(mktemp "${TMPDIR:-/tmp}/handoff-verify.XXXXXX") && trap 'rm -f "$handoff_out"' 0 && GOWORK=off go test -count=1 -v -run '^TestIntakeHandoffDisposition$' ./cmd/scanloop > "$handoff_out" && grep -q -- '--- PASS: TestIntakeHandoffDisposition ' "$handoff_out")` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-10-27 | assay-verifier-app[bot] @ 3a140e111928 (on-behalf-of human:ian) (forge-identity) | base=7445d569bbe9 red rc=1 · head=3a140e111928 |
| 2 | `(cd statusgen && handoff_out=$(mktemp "${TMPDIR:-/tmp}/handoff-verify.XXXXXX") && trap 'rm -f "$handoff_out"' 0 && GOWORK=off go test -count=1 -v -run '^TestScanHandoffAdmission$' . > "$handoff_out" && grep -q -- '--- PASS: TestScanHandoffAdmission ' "$handoff_out")` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-10-27 | assay-verifier-app[bot] @ 3a140e111928 (on-behalf-of human:ian) (forge-identity) | base=7445d569bbe9 red rc=1 · head=3a140e111928 |
Tree-Digest: sha256:60739cd7cd47c269cc1c35d0c634ace19ed97562a41e7d8e2a15173dda6010ba
Container-Image: sha256:3a3cb3913f1311f1a9bf13ddd53c4f27361566f74ac90c89f78991e5452644b8
Verification-Attestation: medici-finance/assay#4617 run=3123a6bc4cdcb2d06d32d030d90479220cc01c3c3ac9fc517d2a35957de70346 source=3a140e11192866bf4c06bae7a318333d576d6f09 brief=docs/streams/verification-handoff/brief-03-require-an-accountable-intake-disposition-before-repair-scheduling.md model=claude-opus-5-5 tier=strong
Admission-Bracket: run=3123a6bc4cdcb2d06d32d030d90479220cc01c3c3ac9fc517d2a35957de70346 source=3a140e11192866bf4c06bae7a318333d576d6f09 tree=sha256:60739cd7cd47c269cc1c35d0c634ace19ed97562a41e7d8e2a15173dda6010ba witness=sha256:cf75c0b916417c4adad4672e4e22f92ffe9461d625455a36bf4ffb786d4932b6 opened=2026-10-27T18:39:06Z closed=2026-10-27T18:45:06Z before=receipt after=check-verifier

#### Execution envelope

Darwin has no host network-namespace lane, so both original rows ran in the installed v1.0.63 runner image ghcr.io/medici-finance/verify-runner:v1.0.63@sha256:3a3cb3913f1311f1a9bf13ddd53c4f27361566f74ac90c89f78991e5452644b8 (linux/amd64; already present locally, not pulled or rebuilt), statusgen v1.0.63, Git 2.56.0, Go 1.27.2. Container flags: network none (only loopback and a down tunnel device present, both reported down or unknown), read-only root filesystem, all capabilities dropped, no-new-privileges, 6 GiB memory, 2048-process limit, non-root uid. The rows ran in a disposable full clone of the admitted source outside the home, because a linked worktree's git directory is not available inside the container and fail-first needs history. Before the bracket opened, a recursive comparison excluding the git directory showed the clone byte-identical to the home, and the container run's own tree check matched the receipt's tree digest.

Module inputs: GOPROXY was two read-only file proxies (the image's own module download cache, then an existing local module download cache mounted read-only), followed by off; GOSUMDB=off, GOFLAGS=-mod=readonly, GOTOOLCHAIN=local, writable GOCACHE and GOMODCACHE in a task-owned directory. The go command checks each downloaded module zip and go.mod against the committed go.sum and refuses a mismatch; none was reported. Before the bracket, one container pass ran only `go list -deps -test` for the three packages to fill the module cache; it ran no test and no row.

Both original commands send verbose output to a temporary file, check the named PASS line with grep, and delete the file, so their recorded output hash is the empty-input digest; that is expected. Fail-first used the delivery parent as base and both rows were red there (rc=1), so each exact command discriminates. That red run proves the named-PASS guard fails before delivery; it does not by itself show the new test bodies catching old code. The mutation run below supplies the behavioural discrimination.

Supplemental, no acceptance credit (Date 2026-10-27; Runner assay-verifier-app[bot] @ 3a140e111928), run after the bracket closed in the same image with the clone mounted read-only (18:46:17Z to 18:46:33Z): the same two test selectors printed `--- PASS: TestIntakeHandoffDisposition (0.08s)` and `--- PASS: TestScanHandoffAdmission (0.05s)`, both exit 0; the whole issueboard package passed (`ok` in 7.463s, exit 0). Neither replaces an original row.

#### Declared mutation coverage

Ran the unchanged manifest command python3 -I statusgen/testdata/verification-handoff/mutate.py --manifest 03-mutations.json in the same network-none container, after verifyrun finished at 18:40:53Z and inside the bracket window; it exited 0, its log completing at 18:44:55Z. The runner first passed both shared unmodified baselines (`checked-clean baseline: TestScanHandoffAdmission`, `checked-clean baseline: TestIntakeHandoffDisposition`), then applied each of the 19 manifest cases once to its own temporary snapshot outside the source; every one reported `checked-failed as expected`. The runner requires a nonzero test exit, the named FAIL and the case's expected diagnostic. The source clone was clean afterwards (empty status, ignored files included), so the green state for each case is the unmutated source the two rows passed on.

Controls and the first diagnostic line each produced (2026-10-27, same runner identity as the witness rows, inside the bracket window):

- production-retention (row 1): handoff_test.go:41: production quiet work lost: err=<nil>
- pre-poll-retention (row 1): handoff_test.go:185: restart 0: items=[] err=<nil>
- partial-write-retry (row 1): handoff_test.go:93: retry: [] <nil>
- strong-author (row 1): subtest scoping_requires_strong_author ran; parent subtest accountable_dispositions_and_partial-write_retry FAIL
- external-wake (row 1): subtest external_wake_missing ran; parent subtest accountable_dispositions_and_partial-write_retry FAIL
- successor-existence (row 1): subtest missing_successor_file ran; parent subtest accountable_dispositions_and_partial-write_retry FAIL
- placeholder-readiness (row 2): intakehandoffs_test.go:36: raw scanner representation became work
- addressed-admission (row 2): intakehandoffs_test.go:76: to:worker entered ordinary work
- closed-raw-retention (row 2): intakehandoffs_test.go:44: closed raw intake disappeared before disposition
- reopened-event (row 2): intakehandoffs_test.go:90: reactivation reused an old disposition
- direct-repair-target (row 2): intakehandoffs_test.go:69: source placeholder dispatched for successor repair/01: ready=true want=false
- settled-discharge (row 1): handoff_test.go:240: pass 1: settled fixture item queued again
- rejection-discharge (row 1): handoff_test.go:328: pass 1: rejected/watched must leave replay and the owed events must be presented
- rejection-judgment-only (row 1): handoff_test.go:328: pass 1: rejected/watched must leave replay and the owed events must be presented
- replay-cap (row 1): handoff_test.go:279: replay not bounded oldest-first
- quarantine-retire (row 1): handoff_test.go:434: pass 2: retired item replayed (fixture state QUARANTINED)
- gone-retire (row 1): handoff_test.go:430: pass 2: blind list held the gone item (gone item must not blind every pass; transient must)
- gone-degraded-guard (row 1): handoff_test.go:475: pass 2: a not-found under a degraded read was retired: []
- transient-not-gone (row 1): handoff_test.go:430: pass 2: blind=[] (gone item must not blind every pass; transient must)

Fourteen controls bear on row 1 and five on row 2. Together with the fail-first reds they cover each Expect clause: the four routes (needs-authoring via strong-author, simple repair via direct-repair-target, delivered qualification and external prerequisite via external-wake and successor-existence), restart retention (production-retention, pre-poll-retention), routing comment or placeholder not satisfying readiness (placeholder-readiness, settled-discharge, rejection-judgment-only), legacy records kept unknown (closed-raw-retention, reopened-event) and the addressed exclusion (addressed-admission).

#### Risk-bearing values: enumerate, rank, derive

Trigger: risk sensitive-data is yes. Enumeration scope: every non-test Go file changed by delivery 0426c579e3a2bacd843381e9d57d612befff7bd2 against its parent (16 files), every added line that is not a comment, every string, raw-string, numeric and boolean token; plus the record values the brief's Deliverables name through spec/intake-handoff-v1.md. That mechanical pass found 373 literal tokens. Non-string tokens: true 17, 0 14, 1 9, false 7, 1024 2, 0600 2, 6 2, 65536 1, 25 1, 0700 1; one further hit, 8601, comes from an ISO 8601 trailing comment in statusgen/placeholder.go and is not a value. The rest are struct tags, closed vocabularies, file names and messages. This is a lexical inventory, not an AST proof of completeness. Line numbers below are at the admitted source.

Ranking. First: authority, readiness and event-identity bindings, because a wrong value schedules the wrong repair, discharges an owed event or makes routing look like acceptance; the code is fixable by edit and redeploy, but work already dropped or wrongly dispatched needs reconciliation. Second: record identity and local file permissions, because a wrong value loses history or exposes local evidence. Last: operational knobs and exit codes, which are reversible and need no derivation.

- RISK-VALUE: DERIVED — IntakeHandoffSchema = "intake-handoff-v1" @ deskcore/issueflow/intake.go:14 — the companion spec's Record table requires the schema to be exactly this string, kept separate from intake-exit-v1 so both existing exit writers stay byte-compatible as the approved design requires; any other version decodes as classification-needed, never ready.
- RISK-VALUE: DERIVED — minimum Revision = 1 @ deskcore/issueflow/intake.go:72, next revision = revision+1 @ tools/desk/cmd/scanloop/handoffrecord.go:65 — the spec defines revision as positive and starting at 1 per source event; requiring exactly the next integer gives one ordered history per (item, opened) key, so a lost or replayed write is refused instead of overwriting.
- RISK-VALUE: DERIVED — AuthorTier = "strong" with Route = "authoring" @ deskcore/issueflow/intake.go:119; tierStrong = "strong" for the deciding session @ tools/desk/cmd/scanloop/handoffrecord.go:100 (constant at exitrecord.go:66); linked brief ExecTier = "strong" @ handoffrecord.go:217; TriagerTier = "strong" @ statusgen/intakehandoffs.go:62 — Task 2 requires scope or design work to queue with a strong author and a linked authoring task; the value is checked at three independent points (record, deciding session, linked brief) and never inferred from a label.
- RISK-VALUE: DERIVED — external owners = "human", "operator", "brief-author", "verifier" @ deskcore/issueflow/intake.go:105, with Route = "external" and an empty successor @ intake.go:101 — Task 2 requires an external prerequisite to name a role, an exact action and a wake condition; the closed list equals the spec's external_role vocabulary, and the empty-successor rule stops an external wait from also being dispatchable repair.
- RISK-VALUE: DERIVED — delivered Route = "qualification" @ deskcore/issueflow/intake.go:97 — Task 2 says source already delivered routes to installation or qualification, not another implementation brief; any other route leaves the item owed.
- RISK-VALUE: DERIVED — ordinary repair admission: State = "observed" and Route = "repair" and DirectRepair @ deskcore/issueflow/intake.go:147, DirectRepair binding the same item as successor @ intake.go:140 — Task 3 forbids scheduling before disposition and the spec says a different successor has its own admission, so only a recorded repair naming the same issue may enter the ordinary lane; this blocks a double dispatch of source and successor.
- RISK-VALUE: DERIVED — settled states = "observed", "external-wait" @ tools/desk/cmd/scanloop/observations.go:75 and the landing gate @ handoffrecord.go:110 — the spec's retention section lists exactly these as discharging a retained event; owed, held or unreadable states stay presented, and a newer source event is a new key.
- RISK-VALUE: DERIVED — retirement = "gone" and "quarantined" @ tools/desk/cmd/scanloop/observations.go:41-42, gone meaning HTTP 404 or 410 (http.StatusGone) @ trust.go:124 and only outside a degraded repository @ adapter.go:341 — the spec retires an event only on a definite not-found in a cleanly read repository or on an untrusted author with no blessing; transient and degraded reads are never retired, so unknown is not rounded to gone.
- RISK-VALUE: DERIVED — handoffRecordsFile = "intake-handoffs-v1.jsonl" @ tools/desk/cmd/scanloop/handoffrecord.go:19; observationFile = "intake-observations-v1.json" @ observations.go:18 — separate versioned files leave the intake-exit-v1 log untouched (the design's byte-compatibility requirement) and keep the observation file a retention aid, not an assignment authority.
- RISK-VALUE: DERIVED — file mode 0600 @ tools/desk/cmd/scanloop/handoffrecord.go:72 and statusgen/scanissues.go:1135; directory mode 0700 @ tools/desk/cmd/scanloop/observations.go:148 — with sensitive-data yes, local judgment and observation records should be readable only by the owning user; these modes do exactly that and nothing wider. They apply on create; an existing directory keeps its mode, so this is not a claim of retroactive hardening.

Reversible knobs, named without derivation: replay cap default 25 with override only when greater than 0 @ tools/desk/cmd/scanloop/observations.go:64-67 (the spec reuses the existing INBOUND_MONITOR_BURST_CAP default of 25; deferred events stay retained and are named every pass); companion read buffer 65536 initial, 1024*1024 maximum @ handoffrecord.go:32 (an over-long line is an error, not empty or ready); exit code 6 for could-not-check @ statusgen/intakehandoffs.go:56 and 65, matching the existing intake-exits convention; mutation-runner budgets (16000 files, 96 MiB, 4-minute test timeout, 270-second subprocess timeout) are diagnostic bounds only. No money, settlement ratio or price literal is in the diff.

#### Limits

The four internal/reviewtest integration tests were not run in this pass and get no credit here. No release, installed-cell rollout, writer activation, live claim, production endpoint or forge custody path was exercised; typed forge fixtures ran on loopback inside the network-none container. The review advisories the first stamp carried (A5 to A8, N4, N5) were not re-litigated by this pass and stay open as recorded there. Design approval on the decision issue is not final acceptance; the human gate stays open and this block flips no lifecycle state.

**VERIFY: PASS** — 2/2 original Verify rows passed on merged main 3a140e111928 in a closed network-none container bracket (bracket digest verdict=match); both exact commands red at the delivery parent 7445d569bbe9; 19/19 declared mutations caught. Second, cross-model stamp only; human acceptance remains open.

## Review

Gate: human, derived from risk. Independent review checks source/consumer contracts, operation compatibility and negative controls; this plan authorizes neither acceptance nor activation.
