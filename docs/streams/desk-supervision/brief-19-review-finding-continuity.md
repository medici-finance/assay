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

## Review

Gate: model. Review the negative paths, migration compatibility and limits of enforcement. Any newly discovered need to alter authority is separate human-gated scope, not an implicit part of this brief.
