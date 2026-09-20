---
brief: "assay:assay:desk-supervision:21"
title: "Reverify changed external prerequisites without a synthetic push"
why: "An unchanged PR can become valid when its external prerequisite lands. A commit-only review gate forces unrelated pushes and repeated reviews; accept independently verified prerequisite changes without allowing unchanged code findings to be cleared."
wave: 1
depends: ["desk-supervision/19"]
unblocks: []
effort: "M"
gate: "human"
risk: {"regulatory": "no", "customer": "no", "irreversible": "no", "sensitive-data": "no"}
gate-why: "Changes which review findings can hold a PR or how a standing rejection can clear; approve the precise rule before changing that control."
decision-trigger: "start"
issues: [1387]
schema: "brief-v2"
authored: "2026-09-20 by recovery planning session"
sources: ["https://github.com/medici-finance/assay/issues/1387", "docs/streams/desk-supervision/recovery-increments.md", "source freshness 2026-09-20 @ 3db05fb44"]
consumers: ["tools/desk/internal/deskkit/checkonlycr.go: fixed-here", "tools/desk/cmd/deskboard/board.go: fixed-here", "tools/desk/cmd/deskpost/ready.go: fixed-here", "tools/desk/cmd/reviewloop/: fixed-here", "tools/desk/cmd/deskdispatch/references/review-prompt.md: fixed-here", "plugins/assay/skills/pr-review-desk/SKILL.md: fixed-here"]
exec-tier: "strong"
exec-tier-why: "Must distinguish genuine changed evidence from a bypass of a standing review rejection."
domain: "complicated"
version: 1
id: "b7bf8dad-179d-4cdc-93f7-cbb9b2f69bf8"
---

# Brief 21 — Reverify changed external prerequisites without a synthetic push

## Context

Home: medici-finance/assay. Planning only; issue 1387 explicitly records a proposed fix, not implementation authority. This brief contributes part of that issue; do not close the issue until both 20 and 21 and the class-counting behavior in 19 are accepted.

files:
- tools/desk/internal/deskkit/checkonlycr.go
- tools/desk/internal/deskkit/reviewfinding.go (planned in 19)
- tools/desk/internal/deskkit/externalprerequisite.go (planned)
- tools/desk/cmd/deskboard/board.go
- tools/desk/cmd/deskpost/ready.go
- tools/desk/cmd/reviewloop/
- tools/desk/cmd/deskdispatch/references/review-prompt.md
- plugins/assay/skills/pr-review-desk/SKILL.md
- tools/desk/cmd/deskpost/externalprerequisite_test.go (planned)

facts:
- Review-prompt clause 8 currently requires a whole-diff search on re-review and treats repository search as optional where cheap; it does not mandate a complete first-pass inventory.
- The canonical skill already caps full rounds at three per finding class. Preserve that threshold and human arbitration authority.
- Clause 11 forbids same-head clearance except a narrow required-check case. The code has an explicit check-only marker parser; this is an existing extension point, not license for prose-inferred exemptions.
- A prompt-only exception cannot repair a contradictory ready gate. Coordinate every producer and reader named in this brief.

## Read first

- [Recovery increments](recovery-increments.md).
- [Issue 1387](https://github.com/medici-finance/assay/issues/1387).
- The existing source paths above; new test and schema paths are explicitly planned.

## Human decision

Approve the precise review-control change described here before implementation begins.
A standing rejection based solely on explicitly recorded external prerequisites could clear without a source commit, only after independent, fresh verification of every prerequisite. Mixed code findings, missing evidence and unsatisfied security reviews would still block. Closure alone would never establish that a human decision was made.

Options:
1. **Approve this evidence rule** — implement the narrow exception consistently in the board, planner and ready gate with negative-path tests.
2. **Retain the commit-only rule** — keep the current refusal and existing human resolution path.

Default if no answer: none — blocks implementation.

## Interface contract

Extend the typed finding contract from brief 19 with an explicitly declared external-prerequisite-only rejection and references to every blocking condition. Legacy free text cannot qualify. The authenticated reviewer must state that there are no outstanding code/content findings; ambiguous, mixed or unreadable records fail closed. Preserve the existing check-only exemption and its tests.

A fresh independent reader must prove each named prerequisite changed after the rejection: for example an exact referenced PR merged at a cited commit, or an authorized decision event satisfied the recorded question. Issue closure alone is not proof of the required decision. Bind evidence to repository, object, condition, observation time and reviewer. No production query is introduced.

Permit a same-head re-review only when the source tree is unchanged, all the declared conditions are independently satisfied, and no later blocking finding or revocation exists. The board, review planner and ready gate must agree. Revalidate at the ready boundary to avoid a stale cached external pass. A security lane still supplies its own required current-head verdict. A worker marker or fabricated reference never grants approval. Do not ask for a comment-only or empty commit merely to move the SHA.

## Ground rules

- No runtime activation, production probes, automatic merge or reinterpretation of an existing human ruling.
- Implement in one isolated branch and draft PR after the decision; stop at implemented.
- Use synthetic fixtures. Do not copy private incident identifiers or review transcripts into this public repository.

## Task

1. Implement the exact approved contract in the named canonical producers and readers; do not change the numeric round threshold.
2. Keep the new review behavior consistent with worker instructions and the existing filing, verdict and ready boundaries.
3. Add synthetic regression cases for the named positive and negative paths. Record fail-first evidence for guard changes.
4. Document the contract and compatibility, add a changelog fragment and any required warmup marker. Release and pin through the existing procedure; source merge is not adoption.

## Verify

Named tests below are planned. The verifier must see each named PASS; exit zero with no matching test is not acceptance. Use offline injected forge state.

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci | `cd tools/desk && GOWORK=off go test ./cmd/deskpost/ -run ^TestExternalPrerequisiteSameHead$ -v -count=1` | named PASS for TestExternalPrerequisiteSameHead; a sole typed prerequisite becoming satisfied allows independent re-review at unchanged head and consistent board/ready behavior |
| 2 | check:ci | `cd tools/desk && GOWORK=off go test ./cmd/deskpost/ -run ^TestExternalPrerequisiteMixedAndForged$ -v -count=1` | named PASS for TestExternalPrerequisiteMixedAndForged; mixed content findings, worker-authored clearance, quoted markers, unrelated objects and unreadable source data cannot clear the rejection |
| 3 | check:ci | `cd tools/desk && GOWORK=off go test ./cmd/deskpost/ -run ^TestExternalPrerequisiteFreshnessAndLanes$ -v -count=1` | named PASS for TestExternalPrerequisiteFreshnessAndLanes; wrong revision, prerequisite predating the rejection, later revocation and a standing security failure continue to block |
| 4 | check:ci | `cd tools/desk && GOWORK=off go test ./cmd/deskpost/ -run ^TestExternalPrerequisiteRestartAndCheckOnly$ -v -count=1` | named PASS for TestExternalPrerequisiteRestartAndCheckOnly; restart preserves the exact condition and observation; existing check-only behavior and rejection of unchanged content findings remain intact |

## Evidence

Pending implementation and independent verification. Authoring claims no acceptance result.

## Review

Gate: human. Verify the recorded decision and every bypass-negative before acceptance. This is a review-control change, not automatic approval of the incident PR.
