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

### Non-implementer verifier run — VERIFY: PASS — 2026-09-23 opus-5.5-verifier

| # | Command | Expected | Observed (exit + key output) | Date | Runner |
|---|---------|----------|------------------------------|------|--------|
| 1 | cd tools/desk && GOWORK=off go test ./cmd/reviewloop/ -run ^TestReviewFindingContinuityAcrossHeads$ -v -count=1 | exit 0; named PASS; fixed finding retains ID+evidence, follow-up targets changed surface, no stale approval | exit 0; `--- PASS: TestReviewFindingContinuityAcrossHeads (0.00s)`; `ok ...cmd/reviewloop` | 2026-09-23 | opus-5.5-verifier |
| 2 | cd tools/desk && GOWORK=off go test ./cmd/reviewloop/ -run ^TestReviewFindingCannotSelfResolve$ -v -count=1 | exit 0; named PASS; worker self-resolution, wrong-head evidence and malformed legacy prose cannot clear a blocking finding | exit 0; `--- PASS: TestReviewFindingCannotSelfResolve` with sub-PASS worker-self-resolve, wrong-head-evidence, legacy-prose | 2026-09-23 | opus-5.5-verifier |
| 3 | cd tools/desk && GOWORK=off go test ./cmd/reviewloop/ -run ^TestReviewFindingCapSurvivesRestart$ -v -count=1 | exit 0; named PASS; 3 rounds survive restart, next round emits one arbiter packet, dup sweeps do not refile, unrelated class separate, sibling sentences held | exit 0; `--- PASS: TestReviewFindingCapSurvivesRestart (0.00s)`; `ok ...cmd/reviewloop` | 2026-09-23 | opus-5.5-verifier |
| 4 | cd tools/desk && GOWORK=off go test ./cmd/reviewloop/ -run ^TestReviewFindingSharedCIBlocker$ -v -count=1 | exit 0; named PASS; multiple PRs cite one shared repair without inventing multiple content defects; ready-flip still requires checks | exit 0; `--- PASS: TestReviewFindingSharedCIBlocker (0.00s)`; `ok ...cmd/reviewloop` | 2026-09-23 | opus-5.5-verifier |
| W | statusgen verifyrun --brief docs/streams/desk-supervision/brief-19-review-finding-continuity.md (hermetic witness) | witness rows for the 4 check:ci rows | could-not-check: all 4 rows could-not-run (exit=-) — hermetic network-off sandbox uses `unshare --net`, a Linux facility, unavailable on this darwin host; not a fail, not a pass. Decisive evidence obtained by direct execution (rows 1-4 above) | 2026-09-23 | opus-5.5-verifier |

RISK-VALUE: DERIVED — RoundCap = 3 @ tools/desk/internal/deskkit/reviewfinding.go:340 — the brief forbids introducing a second cap or changing the threshold; the value equals the existing documented rule "Default cap N = 3 full verdict->fix->re-review rounds" (plugins/assay/skills/pr-review-desk/SKILL.md:463). The constant makes that same existing threshold derivable so it survives agent replacement; it is not a new or second cap.


## Review

Gate: model. Review the negative paths, migration compatibility and limits of enforcement. Any newly discovered need to alter authority is separate human-gated scope, not an implicit part of this brief.
