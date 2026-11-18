---
brief: assay:assay:desk-tools:42
title: Narrow issue discovery projections before enrichment
why: "Worker discovery pays broad issue enrichment cost while worker-ready, intake and inbox populations require different coverage and decisions."
wave: 3
depends: ["desk-tools/38"]
unblocks: ["desk-tools/44", "graph-execution/27"]
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
issues: []
schema: brief-v2
version: 2
authored: 2026-10-17 by statusgen newbrief
sources:
  - docs/desk-workflow-efficiency-spec.md
  - docs/desk-workflow-efficiency-change-map.md
  - freshness-checked 2026-10-17 @ fb85924c039c27e7d2aa80e055bfce118e46125d (App-authenticated main fetch)
  - 'desk-tools: Narrow issue discovery projections before enrichment'
id: 1bd8acf4-cef5-4b69-b663-3a2f406a3f09
exec-tier: strong
exec-tier-why: Semantic extraction, cost coverage and cross-caller failure behavior require independent contract judgment.
domain: complicated
gate-why: Review/issue observations can expose scoped operational facts; implemented coverage and authority boundaries require the existing human acceptance.
decision-trigger: spec
design: DR-issue-projections
decision-issue: 5179
ruling: "https://github.com/medici-finance/assay/issues/5179#issuecomment-6567218820"
consumers:
  - "tools/desk/internal/deskclient/: follow-up graph-execution/27"
  - "plugins/assay/skills/worker-desk/SKILL.md: fixed-here"
  - "plugins/assay/skills/worker-desk/references/dispatch-runbook.md: fixed-here"
  - "plugins/assay/skills/intake-desk/SKILL.md: fixed-here"
  - "plugins/assay/scripts/assay-inbox.sh: fixed-here"
  - "tools/desk/cmd/deskinbox/: fixed-here"
  - "tools/desk/cmd/deskboard/: out-of-scope (independent board populations; intake-depth note already names the local intake reader)"
---

# Brief 42 — Narrow issue discovery projections before enrichment

## Context

files: `tools/desk/cmd/issueboard/` (existing), `tools/desk/internal/issueflow/` (planned), `tools/desk/internal/reviewtest/` (planned), `plugins/assay/skills/worker-desk/SKILL.md` (existing), `plugins/assay/skills/intake-desk/SKILL.md` (existing), `plugins/assay/scripts/assay-inbox.sh` (existing), `changelog/desk-tools-42.md` (planned)

facts: 17 October 2026 at the source pin above. Existing implementation is described in
approved spec §2. All named tests and paths marked planned below are deliverables, not
passing claims. desk-tools/23 remains implemented, /24–25 done, /27 retains its own
verification debt; desktools-v2/09 is verified and /45 implemented. Refresh actual source,
claims and open PRs at pickup. #3248 and #3302 are active CLI ports; #3325 is proposed
supervision guidance and #3305 proposed generic improvement planning. Neither draft is
assumed shipped. The initial blocker is the missing complete observation and attribution,
not the full future runtime.

layering: issueboard typed observation/projection adapters and their actual worker/intake callers. Contracts and pure computation remain separated from
credentialed collection/effects; adapters consume values through legal module APIs.
No new shipped service, runtime store or credential owner is authorized. The flow and boundary
mutation cases below enforce this placement.

design-fit:
  owner: issueboard typed observation/projection adapters and their actual worker/intake callers
  contract: approved spec §6 and A5; existing issue trust/escalation/closure owners
  retires: duplicate work only after the applicable caller and equivalence acceptance
  weight: verbs +0 baseline binaries, flags +1 projection selector at most, refusals +0 authority, rule-text +0 duplicate trust policy
  why-add: reuse existing contracts/runner/typed reads; a complete observation is required before removing the second read, and a new cache would preserve that duplication

## Human decision

Accept or hold the delivered issue discovery projections as this brief's
implementation. Accepting lets the brief move to `implemented` and go to
independent verification. The source specification's approval is already
recorded; this decision covers implementation coverage and authority boundaries
only.

What is delivered on main (source-qualified, not independently verified):

- `issueboard issues --projection work-ready`, `--projection intake` and
  `--projection inbox --to ROLE` pick each caller's population from the complete
  typed open-issue listing before any trust, comment or label-history read. The
  full board and the older spellings still work unchanged.
- `deskinbox discovery` gives the human decision inbox script one typed listing
  per repository instead of four per-label reads, and keeps partial output with
  exit 2. The script picks its reader before any discovery, and the narrow form
  is not enforced against older producers by default:
  - Default `ASSAY_INBOX_READER=auto`: without a desk loop identity it reports
    could-not-check and reads nothing. With one, it uses the typed listing only
    when the producer reports `issue-discovery-v1`; an absent or older producer
    falls back to the four per-label reads.
  - Explicit `ASSAY_INBOX_READER=typed`: an absent or older producer is refused
    before any discovery. This is the only mode that refuses.
  - Explicit `ASSAY_INBOX_READER=legacy`: the four per-label reads. This is the
    rollback.
- Selection is one shared pure selection package (`issueflow`). Trust,
  identity, pagination, escalation, addressed-response and positive-closure
  reads keep their existing owners. No cache, service, credential owner,
  release, pin change or activation is added.
- The worker, intake and inbox instructions check the producer capability
  (`issue-projection-v1`, `issue-discovery-v1`) before using the narrow forms.
- Later changes on the same seam: one reader setup per repository per run,
  positive-state reads batched up to 20 per query, and escalation-comment reads
  batched up to 5 threads per query. Each batch refuses the whole sweep on a
  malformed or partial response and never falls back.
- Local qualification on a synthetic fixture: HTTP attempts per full cycle fall
  from 55 to 51 at 8 issues and from 172 to 132 at 80 issues. CPU and wall time
  moved both ways, so only less request work is shown, not faster runs.

Source check at main `2c6c21336`, not Evidence: the projection flow suite and
the projection fault suite both pass. The consumer-corroboration check reports 5
consumers corroborated and 1 unchecked (the deskboard out-of-scope entry, which
is the reviewer's call).

These holds stay in place whichever option is chosen:

- The prerequisite brief desk-tools/38 (desk performance attribution and counter
  coverage) still needs independent verification before this brief can be
  `verified` or `done`. Its acceptance transition has its own decision issue.
- The consumer-corroboration check's command was changed after delivery to pin
  the delivery and its base. That change counts only through its own
  human-approved acceptance transition at verification time.
- Live activation, releases, pin changes, collection consent and executor
  switches stay separate gates.

The one open obligation is a perf-v4 before/after measurement of the
reader-reuse change. It cannot run yet: the perf-v4 envelope and producer briefs
(desktools-v2/67 and /68) are still `todo`, and neither one names these callers.

Options:
1. **Accept the delivered implementation (recommended)**: the brief moves to `implemented` and verification proceeds under the holds above. The perf-v4 measurement becomes a separate follow-up once desktools-v2/68 ships, not a condition of this brief.
2. **Hold for the perf-v4 measurement**: the brief stays `in-progress` until desktools-v2/67 and /68 ship and the before/after measurement is recorded here.
3. **Revise**: name the missing coverage or authority boundary; a follow-up change fixes it before acceptance.

Default if no answer: none — blocks until answered.

Ruled 2 November 2026 on decision issue #5179: option 1, accept the delivered
implementation; the perf-v4 before/after measurement is a separate follow-up
after desktools-v2/68 (tracked in #5278). The holds listed above still apply.

## Read first

- [Approved source](../../desk-workflow-efficiency-spec.md) and [consolidation](../../desk-workflow-efficiency-change-map.md).
- [Delivery waves](README.md#review-efficiency-follow-ons) and existing source files declared above.
- `docs/library-first.md`, `docs/brief-rules.md` and `docs/telemetry.md` for the applicable boundary/consent rules.

## Ground rules

- Preserve exact-head independent verdicts, claims, trust, budgets, audit and consequential rechecks.
- Use fake providers and disposable local Git for acceptance; live activation and collection are separate gates.
- Stop at implemented; independent verification supplies Evidence. Do not rewrite completed brief Evidence.

## Task

1. Introduce narrow work-ready, intake and inbox projections over shared issue observations; select
   candidate fields before expensive comment/history enrichment. Preserve required histories for the
   selected decision. Batch typed reads and then tune bounded concurrency only using measured
   residual cost.
2. Preserve trusted author/edit and identity rules, complete comment/page handling, escalation,
   addressed-item and positive closure semantics. Declare per-projection freshness and completeness;
   never answer whole-intake idle from an empty worker projection.
3. Extract only the shared pure issue selection seam into tools/desk/internal/issueflow where
   needed; no separate service, credentials, cache or duplicate trust predicates. Use delivered
   issueboard CLI adapter (desktools-v2/45), existing typed forge reads and legal module boundaries.
4. Migrate actual worker/intake/inbox discovery invocations using a supported producer/plugin
   version matrix and reversible pin change. Inventory full-board users that remain. Preserve
   watcher health and existing desk-simplify writer/custody gates; a narrower reader does not switch
   executors. Record calls removed and complete-cycle cost with old broad reads disabled in the
   subject.
5. Update affected command/operator contracts and the declared changelog; regenerate applicable
   documentation with the existing statusgen README generator. No generated global board is branch-
   committed. All planned tests must be wired to actual production paths, not a parallel test
   implementation.

## Verify

Run from the public repository root; each named test is a planned deliverable and its PASS
line is mandatory (zero selected tests is a failure). No authoring-time execution Evidence
is asserted. Each test covers production flow plus the stated wrong-but-plausible mutation.

| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check:ci +flow +mutation | `(cd tools/desk && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestIssueProjectionFlow$' ./internal/reviewtest/... > "$o" && grep -F -- '--- PASS: TestIssueProjectionFlow ' "$o")` | exit 0; named PASS. Actual worker/intake/inbox paths select distinct typed projections before history enrichment; oracle outputs match old semantics and unnecessary histories are not requested. |
| 2 | check:ci +flow +mutation | `(cd tools/desk && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestIssueProjectionFaults$' ./internal/reviewtest/... > "$o" && grep -F -- '--- PASS: TestIssueProjectionFaults ' "$o")` | exit 0; named PASS. Author/edit trust, identity, comments, overflow, permission failures, escalation and positive closure mutations fail; empty work-ready cannot prove intake/inbox idle. Adjacent full board output stays compatible. |
| 3 | check:ci +dereference | `statusgen --root . --consumers --brief desk-tools/42 --delivery a92fea2720a5d2fab84bc5a47c827413adaf762b --base 2f44050f8a706d0865e1a95ca3cd8709abe1bd2f` | exit 0; implementation consumer routing is corroborated against the actual diff. Out-of-scope judgments remain review obligations. |

## Pre-mortem and detection

| Failure mode | Caught by |
|---|---|
| Adapter produces plausible output while duplicating or omitting work | row 1, independent oracle and actual entrypoint/count instrumentation |
| A partial/unsupported observation or hidden consumer creates a false success | row 2, planted invalid input and explicit refusal/coverage |
| Operational activation or control deletion inferred from source qualification | review-only: review scope and separate existing adoption gates; no live effects in the test harness |

## Evidence

<!-- Independent verifier appends one row per Verify command after implementation. -->

### Independent verification — 2026-11-18 (source 1a259b85f7b5, statusgen v1.0.87)

Observed source: merged main 1a259b85f7b59b39d2d6663c7965a9cc0b1c5e6c, in a detached verifier home admitted by `deskdispatch --check-verifier` (rc 0, attestation medici-finance/assay#6926). The verifier is not this brief's implementer. Delivery: merged PR medici-finance/assay#3541 (two-parent merge a92fea2720a5, first parent 2f44050f8a70), with the same-seam follow-ups medici-finance/assay#3584 (merge 4affbe8c1409) and medici-finance/assay#3931 (merge af92fe827146). Row 3's Command was amended after delivery by merged PR medici-finance/assay#4417 (merge 50cf440cbf00), which pinned `--delivery` and `--base`. Rows 1 and 2 are unchanged from the delivered table.

The brief's risk line answers `sensitive-data: yes`, so every row is risk-bearing and owes a fail-first witness.

Rows 1 and 2 (container lane, fail-first). The table directly below is the execution witness: `statusgen verifyrun` v1.0.87, non-dry, `--sandbox=container-netns --rows 1,2 --fail-first --delivering-commit a92fea2720a5d2fab84bc5a47c827413adaf762b`, with the admitted history pack imported for the baseline checkout. It ran in the release-pinned verify-runner image with `--network none`, inside a closed admission bracket, with a fresh module cache downloaded at the open step and mounted read-only (`GOPROXY=off`, `GOTOOLCHAIN=local`). The bracket was opened on the host at 2026-11-18T16:31:21Z and closed on the host at 16:36:26Z. `statusgen verifyrun --bracket-digest` on the closed witness reports `verdict=match` and `span-lines=tree-digest:1,container-image:1,attestation:1`. The witness bytes, including the `Admission-Bracket:` line, follow unchanged.

| # | Command | Result | Output | Date | Runner | Base |
|---|---------|--------|--------|------|--------|------|
| 1 | `(cd tools/desk && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestIssueProjectionFlow$' ./internal/reviewtest/... > "$o" && grep -F -- '--- PASS: TestIssueProjectionFlow ' "$o")` | pass exit=0 sandbox=container-netns | sha256:30ea558f836f | 2026-11-18 | assay-verifier-app[bot] @ 1a259b85f7b5 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) | base=2f44050f8a70 unproven · head=1a259b85f7b5 |
| 2 | `(cd tools/desk && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestIssueProjectionFaults$' ./internal/reviewtest/... > "$o" && grep -F -- '--- PASS: TestIssueProjectionFaults ' "$o")` | pass exit=0 sandbox=container-netns | sha256:19dc8fd8cc3e | 2026-11-18 | assay-verifier-app[bot] @ 1a259b85f7b5 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) | base=2f44050f8a70 unproven · head=1a259b85f7b5 |
Tree-Digest: sha256:19179b2cba9b5a822c9cfdb867bd8fcbf7ae470db462e893cb8805b79cbe1453
Container-Image: sha256:c67baa7a3c8ba3d514128f31340489146525a50c29ebb641a68899c650d354a1
Module-Cache: sha256:7f8fa9800956b328a3851321def73d53e5ec63663bf2209818288608b6dc2ed6
Verification-Attestation: medici-finance/assay#6926 run=1e2a1d660090bfff790d0e0cc5929d6e066863155a18a84b7bd70ab37cbdb2c7 source=1a259b85f7b59b39d2d6663c7965a9cc0b1c5e6c brief=docs/streams/desk-tools/brief-42-narrow-issue-discovery-projections-before-enrichment.md model=claude-opus-5-5 tier=strong
Admission-Bracket: run=1e2a1d660090bfff790d0e0cc5929d6e066863155a18a84b7bd70ab37cbdb2c7 source=1a259b85f7b59b39d2d6663c7965a9cc0b1c5e6c tree=sha256:19179b2cba9b5a822c9cfdb867bd8fcbf7ae470db462e893cb8805b79cbe1453 witness=sha256:b5d7ef4b77b830b389f96bb3c61d8fbf697fc3437454f3139d6ee9df9fc1b154 opened=2026-11-18T16:31:21Z closed=2026-11-18T16:36:26Z before=receipt after=check-verifier

Both rows pass at head with the exact named `--- PASS:` line. At base 2f44050f8a70 both are `unproven`: the runner could not execute them there, so neither row has been shown to fail without the change. The base log directory is empty because each row redirects its own output. Two facts read from the admitted history explain the base result. First, neither named test exists at base. Second, the base `tools/desk/go.mod` and `go.sum` differ from head (207 insertions, 31 deletions), and the open step downloads modules for the head tree only, so an offline base build cannot resolve its modules. This is an environment gap in the fail-first lane (related: medici-finance/assay#4683). It is not a defect in the rows.

Row 3 (host lane). Row 3 reads git history (`--delivery` and `--base`). The verifier home is a linked worktree, and its git metadata lies outside the mounted home, so the container cannot run this row; the history pack is accepted only for baseline checkouts under `--fail-first`. Row 3 was run on the host instead with `statusgen verifyrun --rows 3` (v1.0.87, non-dry), in the declared verifier Git environment. On this darwin host the check:ci network-off sandbox (`unshare --net`) does not exist, so verifyrun recorded the row as could-not-run before executing it (medici-finance/assay#1800). The witness it appended follows unchanged.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 3 | `statusgen --root . --consumers --brief desk-tools/42 --delivery a92fea2720a5d2fab84bc5a47c827413adaf762b --base 2f44050f8a706d0865e1a95ca3cd8709abe1bd2f` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-11-18 | assay-verifier-app[bot] @ 1a259b85f7b5 (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#6926 run=1e2a1d660090bfff790d0e0cc5929d6e066863155a18a84b7bd70ab37cbdb2c7 source=1a259b85f7b59b39d2d6663c7965a9cc0b1c5e6c brief=docs/streams/desk-tools/brief-42-narrow-issue-discovery-projections-before-enrichment.md model=claude-opus-5-5 tier=strong

Supplemental host probe (no acceptance credit). This diagnoses the row 3 blocker only. It discharges no Verify row. The row 3 command, as written, was run directly on the host, without the network-off sandbox, in the declared verifier Git environment, at the same source.

| Probe | Command | Exit | Key output | Date | Runner |
|-------|---------|------|------------|------|--------|
| S3 (supplemental, no acceptance credit) | Verify row 3 command, as written, run directly on the host | 0 | `consumers corroboration — base 2f44050f8a706d0865e1a95ca3cd8709abe1bd2f..a92fea2720a5d2fab84bc5a47c827413adaf762b (immutable delivered snapshot)`; `summary: 5 corroborated, 0 disproved, 1 unchecked, 0 brief(s) claiming nothing`; the one UNCHECKED entry is the deskboard out-of-scope route (a reviewer judgement); output sha256 de1a2ffb4f87 | 2026-11-18 | assay-verifier-app[bot] (claude-opus-5-5 verifier; supplemental, no acceptance credit) (on-behalf-of human:ian) |

Held obligations that this run does not clear:

- Rows 1 and 2 have no fail-first discharge: their base runs are `unproven` (base modules absent from the offline cache; related medici-finance/assay#4683). They need a base run whose module cache covers the base tree.

- Row 3 has no admissible witness. The host has no network-off sandbox (medici-finance/assay#1800), and the container lane cannot read git history for a linked-worktree home outside a fail-first run. The row needs a Linux runner that provides `unshare --net`, or a container route that supplies history to ordinary rows.
- Row 3's current Command is the post-delivery amendment from medici-finance/assay#4417. The brief's Human decision says this change counts only through its own human-approved acceptance transition at verification time. This verifier found no `Acceptance-Transition` record for this brief at the observed source. Closure of row 3 also needs either that approval or a witness of the delivered command.
- The prerequisite desk-tools/38 is still `implemented` on the board at the observed source. Its independent verification is a completion hold for this brief.
- The brief is risk-flagged (sensitive-data), so a model cannot sign it off. It stays at its current lifecycle state for the human gate.

Risk-bearing values. Enumerated over the literals the delivery and its same-seam follow-ups introduce, plus the bounds the brief's Human decision names. Both entries are reversible read-batching knobs: a wrong value changes request count or response size and is undone by an edit and a release. Correctness does not depend on either one, because every batch refuses the whole sweep on a malformed or partial response and never falls back.

- RISK-VALUE: DERIVED — IssueCommentBatchSize = 5 @ tools/desk/internal/deskkit/issuecomments.go:17 — five aliases at the retained first:100 comment selection keep one GraphQL page to at most 500 comment nodes, which is the repo's stated bound; each thread keeps its own 20-page limit, so the batch size changes lookahead only, never a thread's history.
- RISK-VALUE: NAMED, NOT DERIVED — IssueStateBatchSize = 20 @ forgeread/adapters/github/issuestates.go:12 (used as PositiveIssueStateBatchSize @ tools/desk/internal/deskkit/issuestates.go:15) — the repo states it as a per-observation bound but gives no cost derivation; the perf-v4 before/after measurement that would justify the size is still pending (medici-finance/assay#5278). Correctness is independent of the size: every alias must return its exact number, a known state and a title, or the sweep refuses.

**VERIFY: BLOCKED** — rows 1 and 2 pass at head in the network-off container witness at source 1a259b85f7b5, inside a closed admission bracket (`verdict=match`), but their fail-first base runs are `unproven` (base modules absent from the offline cache). Row 3 is could-not-run: the darwin host has no network-off sandbox (medici-finance/assay#1800), and the container lane has no git history for head rows in this home. No row was observed to fail. The row 3 acceptance transition and the desk-tools/38 prerequisite remain holds. 2 of 3 rows pass at head; 0 of 3 rows are discharged under fail-first.


## Review

Retain the derived frontmatter gate. Approval of the source spec is recorded; this brief
adds implementation acceptance rather than reopening that decision. Existing adopter
writer/custody/equivalence and collection consent gates remain separate.

## Implementation acceptance hold

The actual deskinbox consumer preserves the inbox script's positive ruling screen,
cards and flow. The merged [perf-v3 integration](https://github.com/medici-finance/assay/pull/3584)
now connects both readers to the canonical schema/context/provider seam from
desktools-v2/63–65. The [qualification report](../../issue-projection-qualification.md)
records the actual selected-profile writer/reader/reducer qualification; this is
source qualification, not an independent verification claim.

Shared delivery/acceptance and the independent desk-tools/38 verification prerequisite
remain completion holds. The later [reader-reuse follow-up](https://github.com/medici-finance/assay/pull/3931)
also leaves its requested perf-v4 before/after measurement pending the canonical codec
and producer, as documented in [issue projections](../../issue-projections.md). Its resolver
counts do not satisfy that measurement. None of these source qualifications activates
any deployment, writer, credential, pin, release or collection policy.

## Standing-desk consumer routing — 2026-10-18

graph-execution/27 consumes this brief's planned pure policy/projection API from a
tools/desk-local binding. It does not import cmd/main across modules, duplicate the
classifier, change this brief's acceptance or add a process supervisor here. Preserve
the current oracle, authority boundary and planned library home. This is a forward
consumer, not a dependency on future workflow/controller delivery.

## Prospective planner consumer checkpoint — 23 October 2026

The efficiency spec §2.1/A11–A12 nominates the existing issueflow projection as a
candidate input to graph-execution/27's shared role planner. Preserve the currently
assigned Task/Verify/revision until its owner acknowledges any material amendment.
At pickup, retain all work-ready/intake/inbox populations and projection-before-enrichment;
qualify per-operation batching only through this owner and existing forge-neutral/37–38.
No hidden global scan per root, parallel issue classifier or status/acceptance claim is added.
This checkpoint records requested integration, not new execution Evidence or activation.
