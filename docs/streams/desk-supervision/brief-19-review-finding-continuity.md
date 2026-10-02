---
brief: "assay:assay:desk-supervision:19"
title: "Persist review findings and apply the existing round cap across sessions"
why: "Review rounds lose continuity when a replacement agent rereads the whole PR and restates old objections. Persist what is disputed and what resolves it so follow-up review can converge and the existing round cap survives agent replacement."
wave: 0
depends: []
unblocks: ["desk-supervision/21"]
effort: "M"
gate: "model"
risk: {"regulatory": "no", "customer": "no", "irreversible": "no", "sensitive-data": "no"}
issues: []
schema: "brief-v2"
authored: "2026-09-20 by recovery planning session"
sources: ["docs/streams/desk-supervision/recovery-increments.md", "freshness-checked 2026-09-20 @ 3db05fb44; source paths and existing three-round review rule inspected"]
consumers: ["tools/desk/cmd/reviewloop: fixed-here", "tools/desk/cmd/deskpost: fixed-here", "tools/desk/cmd/deskreply: fixed-here", "tools/desk/cmd/deskdispatch/references: fixed-here", "plugins/assay/skills/pr-review-desk/SKILL.md: fixed-here"]
exec-tier: "strong"
exec-tier-why: "Cross-component scheduling state must survive partial failure without manufacturing completion or bypassing an existing role gate."
domain: "complicated"
version: 1
id: "bc4760f0-c1da-4c07-83e7-47f58da7917d"
---

# Brief 19 — Persist review findings and apply the existing round cap across sessions

## Context

Home: medici-finance/assay.

files:
- tools/desk/cmd/deskpost/
- tools/desk/cmd/deskreply/
- tools/desk/cmd/reviewloop/
- tools/desk/internal/deskkit/reviewfinding.go (planned)
- tools/desk/cmd/reviewloop/findingcontinuity_test.go (planned)
- tools/desk/cmd/deskdispatch/references/review-prompt.md
- tools/desk/cmd/deskdispatch/references/worker-prompt.md
- plugins/assay/skills/pr-review-desk/SKILL.md
- docs/streams/desk-supervision/review-finding-v1.md (planned)
- tools/desk/README.md
- changelog/review-finding-continuity.md (planned)

facts (2026-09-20; re-establish from the named files at pickup):
- reviewloop already keys actions by repo/PR/head/verb, and its RE-REVIEW action calls for a delta review. Keep that existing behavior.
- The canonical review skill already caps full verdict/fix/re-review rounds at three per finding class and then files an arbiter packet to the human decision lane. Do not introduce a second cap, change the threshold, or automatically overrule a reviewer.
- Current review/reply payloads carry prose verdicts. No typed persistent per-finding record was found in deskpost/deskreply/reviewloop at the source revision below.
- A shared CI fault is an integration blocker with a shared repair reference; it is not automatically a correctness defect in every dependent PR.

single-point-of-failure: the new scheduling classification; independent barriers remain the existing per-item claim and reviewer/verifier identity gates. A scheduling receipt can never authorize completion or a write.

## Read first

- [Recovery increments](recovery-increments.md) — scope, ordering, rollout and existing work.
- tools/desk/internal/loopengine/doc.go — existing executor and claim boundaries.
- The source files listed above; planned files are deliverables, not prerequisites.

## Interface contract

Add a backward-compatible versioned finding block to existing forge review/reply records. Each finding has a stable ID, class, originating review/head, severity, blocker kind (code/content or external prerequisite), concrete failure/reproduction, resolution condition, state, and evidence links. States: open, fixed-awaiting-review, disputed, resolved, and awaiting-arbitration. Derive actor identity and head from the authenticated forge event; worker prose cannot author a reviewer resolution.

A worker response references the finding ID and its fix or counterevidence. A reviewer resolves it with current-head evidence or explains why it remains. A genuinely new defect gets a new ID; a reopened resolved defect requires changed code or new evidence. A newly discovered occurrence of the same proposition retains its claim-class ID and round history; fixing one sentence never resets the class. Record any promotion from non-blocking to blocking with changed impact or new evidence. Full-review validity at the current head remains mandatory: delta targeting never carries an approval blindly across changes.

Count rounds from durable review -> worker response -> re-review transitions, not commits or poll ticks. At the existing class cap produce one deduplicated arbiter packet and hold the class. Preserve the existing human decision authority. Older reviews remain visible and are migrated by a reviewed classification action; do not infer a clean finding set from unparseable prose.

## Ground rules

- Implement through a draft PR in this repository; stop at implemented. Independent verification owns acceptance.
- Work in an isolated checkout. Preserve existing role authority, stop flags, review lanes and human merge gates.
- No production queries, runtime activation, global configuration changes or autonomous-loop cutover in this code brief.

## Task

1. Add parser/validator and backward-compatible structured payload support to deskpost and deskreply; keep the existing role and verdict gates. Require a concrete reproduction or explicit evidence-based explanation for blocking findings.
2. Have reviewloop derive outstanding findings, disputed responses and per-class rounds from forge records. Pass that compact record into reviewer and worker prompts; reuse the current action dedupe.
3. Persist finding resolutions across agent replacement. Distinguish branch correctness findings from shared CI blockers while continuing to require the existing checks before ready-flip.
4. Implement the existing cap as a derived state and generate the current arbiter packet through the sanctioned filing path once per class. Missing actor/head/round evidence yields could-not-check, never automatic approval or an invented cap breach.
5. Update canonical skill instructions and docs to consume the record. Do not add a second review service, new policy thresholds or automatic class demotion; calibration remains separate existing work.

## Verify

All named tests below are planned deliverables. The verifier must observe each named PASS line; exit zero with no tests run is not a pass. Tests use injected forge/clock state, no production services.

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci | `cd tools/desk && GOWORK=off go test ./cmd/reviewloop/ -run ^TestReviewFindingContinuityAcrossHeads$ -v -count=1` | exit 0; named PASS for TestReviewFindingContinuityAcrossHeads; fix A and change B: A retains its ID and evidence; follow-up review targets both the fix and changed surface; no stale approval |
| 2 | check:ci | `cd tools/desk && GOWORK=off go test ./cmd/reviewloop/ -run ^TestReviewFindingCannotSelfResolve$ -v -count=1` | exit 0; named PASS for TestReviewFindingCannotSelfResolve; worker self-resolution, wrong-head evidence and malformed legacy prose cannot clear a blocking finding |
| 3 | check:ci | `cd tools/desk && GOWORK=off go test ./cmd/reviewloop/ -run ^TestReviewFindingCapSurvivesRestart$ -v -count=1` | exit 0; named PASS for TestReviewFindingCapSurvivesRestart; three full rounds of one class survive restart; next round emits one arbiter packet, duplicate sweeps do not refile; unrelated class starts separately; newly noticed sibling sentences retain the old class and cannot evade the cap |
| 4 | check:ci +flow | `cd tools/desk && GOWORK=off go test ./cmd/reviewloop/ -run ^TestReviewFindingSharedCIBlocker$ -v -count=1` | exit 0; named PASS for TestReviewFindingSharedCIBlocker; multiple PRs cite one shared repair without inventing multiple content defects; ready-flip still requires applicable checks |

Pre-mortem → detection: Findings are reset with each agent/head: row 1. Worker clears its own finding: row 2. Polls count as rounds or restart resets the cap: row 3. Shared red CI becomes blanket approval: row 4.

## Evidence

Pending implementation and independent verification. No acceptance result claimed by authoring.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/desk && GOWORK=off go test ./cmd/reviewloop/ -run ^TestReviewFindingContinuityAcrossHeads$ -v -count=1` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-27 | assay-verifier-app[bot] @ 874d56de38a7 (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd tools/desk && GOWORK=off go test ./cmd/reviewloop/ -run ^TestReviewFindingCannotSelfResolve$ -v -count=1` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-27 | assay-verifier-app[bot] @ 874d56de38a7 (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd tools/desk && GOWORK=off go test ./cmd/reviewloop/ -run ^TestReviewFindingCapSurvivesRestart$ -v -count=1` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-27 | assay-verifier-app[bot] @ 874d56de38a7 (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd tools/desk && GOWORK=off go test ./cmd/reviewloop/ -run ^TestReviewFindingSharedCIBlocker$ -v -count=1` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-27 | assay-verifier-app[bot] @ 874d56de38a7 (on-behalf-of human:ian) (forge-identity) |

### Non-implementer verifier run — VERIFY: BLOCKED — 0/4 pass, 4 could-not-check, 0 fail — 2026-09-27 claude-opus-5-5-verifier

The runner is not the implementer. Rows ran on darwin/arm64 with go1.27.1, offline
(KUBECONFIG=/dev/null), from a worktree cut detached at merged main
874d56de38a73228dad62ed25482d2af55692849. The implementing commit is 861abb937 (PR #1406). All
four Verify rows are check:ci. The hermetic statusgen verifyrun witness above could not run any
of them: its network-off sandbox needs Linux unshare --net, and this host is darwin. The pinned
harness container image is not present locally, and pulling it would reach an external
registry, which the offline envelope rules out. Each row is therefore held COULD-NOT-CHECK, with
its direct non-hermetic run recorded below. Every direct run passed. The block comes from the
darwin execution-witness environment, not from the implementation. A Linux runner re-running
the four rows under verifyrun is what advances the item.

Per-row direct-run notes (supporting only, not the hermetic witness):

- Row 1: exit 0; `--- PASS: TestReviewFindingContinuityAcrossHeads`. The test asserts that A keeps its ID, its class and its original evidence ev-A1 across the h1 to h2 change. A is not resolved at the new head (ResolvedAtHead false), B is present, and both are outstanding blocking findings.
- Row 2: exit 0; `--- PASS: TestReviewFindingCannotSelfResolve` with three subtests passing: worker_cannot_self-resolve, wrong-head_evidence_cannot_resolve and legacy_prose_cannot_resolve. The worker case also checks the write gate: ValidateReviewFindingBlock refuses the block as a worker and accepts it as a reviewer. The legacy case checks that a malformed block fails to parse.
- Row 3: exit 0; `--- PASS: TestReviewFindingCapSurvivesRestart`. Rounds for classC equal RoundCap, the class is held, and exactly one arbiter packet is produced. Re-deriving the same records gives the same state. A duplicate sweep refiles nothing. The sibling finding C-sibling is set to awaiting-arbitration. Unrelated classD is not held and has 0 rounds. Three reviewer polls with no worker response in between count 0 rounds.
- Row 4: exit 0; `--- PASS: TestReviewFindingSharedCIBlocker`. The shared-CI finding is absent from ContentDefects on both PRs, while PR one's own defect remains. The two PRs cite exactly one distinct shared repair. The shared blocker stays in OpenBlocking on both PRs.
- Supporting: the full reviewloop package passes (exit 0). The deskkit review-finding tests pass (exit 0): TestReviewFindingBlockRoundTrips, TestReviewFindingBlockMalformed, TestReviewFindingRoleGate and TestValidateReviewFindingBlockNoBlock.

Risk-bearing value enumeration. The item's risk metadata is present and all "no". The trigger
still fires because the diff encodes a threshold that the canonical review skill pins. Literals
introduced by the diff:

- RoundCap = 3 @ tools/desk/internal/deskkit/reviewfinding.go:340. This is the threshold that decides when a class stops re-litigating and goes to the human lane.
- FindingBlockSchema = "review-finding/v1" @ tools/desk/internal/deskkit/reviewfinding.go:40, and findingBlockOpen = the HTML comment opener, one space, then "assay:review-finding:v1" @ the same file, line 47. These are wire-format markers. Changing them makes older records unparseable, which is reversible by an edit plus a migration.
- Authority binding: RoleReviewer = "reviewer" / RoleWorker = "worker" @ the same file, lines 116-117. The worker write gate is at lines 254-262: a worker cannot set resolved on a blocking finding or set awaiting-arbitration.
- Enumerated vocabulary with no numeric weight: the state strings (lines 58-71), blocker kinds (90-91), severities (103-104), record kinds (300-301) and verdicts (308-311).

Ranking: nothing here is irreversible. A wrong RoundCap sends a PR's class to the human lane too
early or too late, and an edit plus a redeploy undoes it. It still ranks first because it is the
one threshold the brief forbids changing.

RISK-VALUE: DERIVED — RoundCap = 3 @ tools/desk/internal/deskkit/reviewfinding.go:340 — this is the existing canonical cap and is not a new value. The pr-review-desk skill has carried "Default cap N = 3 full verdict→fix→re-review rounds on the SAME finding class" since commit 5fe5b3c06 (2026-09-06), which predates this brief's authoring (2026-09-20). The brief's facts and Task 5 forbid changing the threshold, and the constant equals it. Semantics match too: the round count saturates at 3, and the next re-review (round N+1) yields the single arbiter packet. That is the skill's "On round N+1 for that class … files … an arbiter packet".

Observations (non-blocking, recorded for the human reader):

1. The skill calls the cap "adopter-tunable", but the code has RoundCap as a compile-time constant with no configuration path. An adopter who tunes the skill's N gets prose and derivation that disagree. This is outside this brief's Verify rows. The brief forbids new policy thresholds, so the gap is reported here rather than fixed.
2. Actor role and head in the reactor come from the records payload's role and head fields (reviewloop plan --records reads a file). This diff ships no producer that builds that payload from authenticated forge events. The "derived from the authenticated forge event" guarantee therefore depends on whoever assembles the payload. The write-time role gate in deskreply/deskpost and the derivation-time refusal both exist and are tested. This matches the brief's rows, which use injected forge state.
3. The derivation computes the arbiter packet but does not file it. Filing stays with the reviewer, through deskfile needs-decision per the skill. That matches the brief's "preserve the existing human decision authority".
4. statusgen --lint raises a risk-files-crossread NOTICE. The brief answers all four risk questions "no" but declares tools/desk/cmd/deskpost/, which is a security-path trigger. The deskpost change is 8 added lines: a pre-network ValidateReviewFindingBlock refusal in the reviewer role. The deskreply change is its worker-role twin, 10 lines. Both changes can only refuse a write. Neither widens what can be posted, and neither touches the existing verdict or role gates. Whether the risk answers stand is the gate owner's call, not the verifier's. The same lint run also notes a +dereference obligation and a consumers-corroboration row that the Verify table does not carry.

VERIFY: BLOCKED — 0/4 pass on the hermetic witness, 4 could-not-check (darwin: no unshare --net network-off sandbox; harness image not local), 0 fail. The direct non-hermetic runs of all four rows passed with their named PASS lines and assertions that match each Expect cell. Held at implemented until a Linux runner records the hermetic witness.
### Verification — 2026-09-30 (assay-verifier-app[bot] @ b0088804294b (claude-opus-5-5) (on-behalf-of human:ian)) — 2026-09-30 claude-opus-5-5-verifier

Non-implementer verification on merged main b0088804294b8b68ad8d06f341e6f0fd9dd2637d, gate: model, all four risk answers no. First table: the `statusgen verifyrun` execution witness, landed verbatim; it ran on Linux (golang:1.25-bookworm pinned by digest, `--network none`, `unshare --net` available), statusgen built in-container from a clone pinned to this SHA. Second table: the hand run.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/desk && GOWORK=off go test ./cmd/reviewloop/ -run ^TestReviewFindingContinuityAcrossHeads$ -v -count=1` | pass exit=0 | sha256:bd71e40e17a9 | 2026-09-30 | assay-verifier-app[bot] @ b0088804294b (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd tools/desk && GOWORK=off go test ./cmd/reviewloop/ -run ^TestReviewFindingCannotSelfResolve$ -v -count=1` | pass exit=0 | sha256:fd9e790952e6 | 2026-09-30 | assay-verifier-app[bot] @ b0088804294b (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd tools/desk && GOWORK=off go test ./cmd/reviewloop/ -run ^TestReviewFindingCapSurvivesRestart$ -v -count=1` | pass exit=0 | sha256:5945d74d70b1 | 2026-09-30 | assay-verifier-app[bot] @ b0088804294b (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd tools/desk && GOWORK=off go test ./cmd/reviewloop/ -run ^TestReviewFindingSharedCIBlocker$ -v -count=1` | pass exit=0 | sha256:2dda55483e85 | 2026-09-30 | assay-verifier-app[bot] @ b0088804294b (on-behalf-of human:ian) (forge-identity) |

| # | Command | Expect | Observed | Date | Runner |
|---|---------|--------|----------|------|--------|
| 1 | `cd tools/desk && GOWORK=off go test ./cmd/reviewloop/ -run ^TestReviewFindingContinuityAcrossHeads$ -v -count=1` | exit 0; named PASS; A keeps ID and evidence across heads; follow-up carries both A and B; no stale approval | exit 0; `--- PASS: TestReviewFindingContinuityAcrossHeads`; test asserts A keeps class claimA and original evidence ev-A1 across h1 to h2, A not resolved at h2 (ResolvedAtHead false), B present, both A and B in OpenBlocking | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ b0088804294b (on-behalf-of human:ian) |
| 2 | `cd tools/desk && GOWORK=off go test ./cmd/reviewloop/ -run ^TestReviewFindingCannotSelfResolve$ -v -count=1` | exit 0; named PASS; worker self-resolution, wrong-head evidence, malformed legacy prose cannot clear a blocker | exit 0; `--- PASS: TestReviewFindingCannotSelfResolve` plus 3 subtests PASS (worker_cannot_self-resolve, wrong-head_evidence_cannot_resolve, legacy_prose_cannot_resolve); worker case also checks ValidateReviewFindingBlock refuses as worker and accepts as reviewer | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ b0088804294b (on-behalf-of human:ian) |
| 3 | `cd tools/desk && GOWORK=off go test ./cmd/reviewloop/ -run ^TestReviewFindingCapSurvivesRestart$ -v -count=1` | exit 0; named PASS; 3 rounds survive restart; one arbiter packet; no refile; unrelated class separate; sibling keeps class | exit 0; `--- PASS: TestReviewFindingCapSurvivesRestart`; classC rounds equal RoundCap and held, exactly 1 packet, identical on re-derive, duplicate sweep refiles nothing, C-sibling set awaiting-arbitration, classD 0 rounds and not held, 3 reviewer polls count 0 rounds | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ b0088804294b (on-behalf-of human:ian) |
| 4 | `cd tools/desk && GOWORK=off go test ./cmd/reviewloop/ -run ^TestReviewFindingSharedCIBlocker$ -v -count=1` | exit 0; named PASS; one shared repair cited, no invented content defects; ready-flip still needs checks | exit 0; `--- PASS: TestReviewFindingSharedCIBlocker`; shared-CI finding absent from ContentDefects on both PRs, PR one own defect kept, exactly 1 distinct shared repair across both PRs, shared blocker stays in OpenBlocking on both PRs | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ b0088804294b (on-behalf-of human:ian) |

RISK-VALUE: DERIVED — RoundCap = 3 @ tools/desk/internal/deskkit/reviewfinding.go:340 — this is the pre-existing canonical cap, not a new value. The pr-review-desk skill line 494 has read "Default cap N = 3 full verdict→fix→re-review rounds on the SAME finding class" since 5fe5b3c06 (2026-09-06), before this brief was authored (2026-09-20), and the brief forbids changing it. The semantics also match: in applyReviewer a round is counted only on a re-review after a worker response, the count saturates at 3, and the next genuine re-review (round N+1) emits the single deduplicated packet and holds the class. That is the skill's "On round N+1 for that class … files an arbiter packet". The value can be reversed by an edit plus a redeploy.

Notes:
- PASS on all four rows, by hand and in the Linux witness (4/4 pass, non-dry and dry), which settles the 2026-09-27 BLOCKED run. Status is NOT flipped in this landing: `statusgen brief --check-verified` still exits 1 on the brief as it stands, for a reason in earlier Evidence prose rather than in this witness; that is tracked at #1939, and the flip follows its fix. The one RISK-VALUE line is DERIVED.
- Grounding was written before any tests or diffs were read. All planned deliverables exist on main: reviewfinding.go, findingcontinuity_test.go (all 4 named tests), review-finding-v1.md, and the README, skill and prompt-kit references. The changelog fragment was folded into CHANGELOG.md by the v1.0.21 aggregation commit, so its absence under changelog/ is expected.
- Every Verify row is check:ci, so the Linux network-off witness above was used. The darwin sandbox cannot run check:ci rows. Neither known environment issue (loopback down, old git) affected any row, because these tests are pure derivations with no local server and no git calls.
- This pass settles the 2026-09-27 BLOCKED run, whose 4 witness rows were could-not-run on darwin. All 4 rows now pass on both the hand run and the witness.
- `statusgen brief --check-verified desk-supervision/19` was run on a throwaway --no-hardlinks clone after the witness table was appended and README row 19 was set to verified with the Verified cell "2026-09-30 opus-5.5-verifier". With the brief exactly as it stands on main plus the appended table, it exits 1 and reports "verified outcome requires passing execution witnesses: row 1..4: could-not-run". The cause is in the prior run's Evidence prose, not in the new witness. That cause is tracked at #1939. With that edit, `--check-verified` exits 0, `verifyrun --check` reports 4 pass 0 fail, and `--lint` exits 0.
- Landing note: once the table is visible, lint adds a NOTICE (verified-runner-attribution). The Verified cell "opus-5.5-verifier" does not match the Evidence runner "assay-verifier-app[bot] @ b0088804294b (on-behalf-of human:ian) (forge-identity)". Credit the Evidence runner in the Verified cell.
- The lint NOTICEs from 2026-09-27 still stand and are advisory. risk-files-crossread: the brief declares a deskpost path but answers all risk questions "no". The deskpost and deskreply changes only add a pre-network refusal, so they can only refuse writes. Whether the risk answers stand is the gate owner's call. The other notices are a consumers row with no `statusgen --consumers` run, and a +dereference verify-obligation not carried by any row.
- Scope limits, recorded but not failing any row. Row 4's "ready-flip still requires applicable checks" is proven at the ledger level (the shared blocker stays in OpenBlocking); the test does not drive the ready-flip verb. Row 1's "targets both the fix and changed surface" is proven by A staying outstanding next to the new B. The derivation reads role and head from the records payload, and this diff ships no producer that assembles that payload from authenticated forge events. The skill calls N "adopter-tunable", but RoundCap is a compile-time constant.

VERIFY: PASS

## Review

Gate: model. Review the negative paths, migration compatibility and limits of enforcement. Any newly discovered need to alter authority is separate human-gated scope, not an implicit part of this brief.
