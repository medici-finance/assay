---
brief: "assay:assay:review-profiles:05"
title: "Integrate approved advisory routing and finding continuity"
why: "An isolated prompt is not a working review profile. Routing, current-head evidence, findings, round limits and readiness must agree before a bounded live shadow is useful."
wave: 3
depends: ["review-profiles/04"]
unblocks: []
effort: "L"
gate: "human"
risk: {"regulatory": "no", "customer": "no", "irreversible": "no", "sensitive-data": "no"}
issues: []
schema: "brief-v2"
version: 1
id: "3e093714-945a-49e7-914b-9c33ae8710cf"
authored: "2026-10-15 by review profile authoring"
sources: ["docs/streams/review-profiles/spec.md", "freshness-checked 2026-10-15 @ 7d52be61b66cb81f5ad2ab7e1d250747a6f125de"]
exec-tier: "strong"
exec-tier-why: "Cross-profile evidence, algorithm and authority judgments require the applicable contracts and negative controls."
domain: "complicated"
outcome: "none"
consumers: ["tools/desk/internal/deskkit/reviewlanes.go: follow-up review-profiles/05", "tools/desk/internal/deskkit/reviewlanes_test.go: follow-up review-profiles/05", "tools/desk/internal/deskkit/reviewfinding.go: follow-up review-profiles/05", "tools/desk/internal/deskkit/reviewfinding_lane_test.go: follow-up review-profiles/05", "tools/desk/internal/deskkit/reviewscope.go: follow-up review-profiles/05", "tools/desk/cmd/deskdispatch/references/review-prompt.md: follow-up review-profiles/05", "plugins/assay/skills/pr-review-desk/SKILL.md: follow-up review-profiles/05"]
gate-why: "Select the concrete advisory routing, publication scope and budget from the calibration evidence; this does not authorize blocking status."
decision-trigger: "spec"
---

# Brief 05 — Integrate approved advisory routing and finding continuity

## Context

files: `tools/desk/internal/deskkit/reviewlanes.go`, `tools/desk/internal/deskkit/reviewlanes_test.go`, `tools/desk/internal/deskkit/reviewfinding.go`, `tools/desk/internal/deskkit/reviewfinding_lane_test.go`, `tools/desk/internal/deskkit/reviewscope.go`, `tools/desk/cmd/deskdispatch/references/review-prompt.md`, `plugins/assay/skills/pr-review-desk/SKILL.md` — New profile, calibration and operations files are planned deliverables; existing desk owners remain controlling.

facts:
- At the pinned main there are four review lanes, existing advisory design-fit and architectural fitness tests; no calibrated performance/architecture routing policy exists.
- Review finding identity, scope and rounds retain their existing owner; required acceptance evidence and budget accounting do not depend on optional telemetry.
- Risk answers are all `no` because this slice is opt-in, advisory and revertible through configuration (Task 4), prepares offline work only and begins live routing solely after a recorded scoped human approval (`gate: human`). It still edits the lane-routing and ready-aggregation code that decides which reviews count; the security lane weighs that surface at review, and the answers are re-derived from the actual changed files at pickup.
- This brief implements only its Task; the profile spec remains a draft and actual model/data use requires a permitted scoped budget.

layering: pure assessment contracts/evaluation in tools/desk/internal/reviewprofile; deskkit and existing commands adapt inputs/effects. No controller, credential store, general service or parallel finding ledger. Task and Verify exercise that boundary.

design-fit:
  owner: existing review/policy and continuing-operations meanings; reviewprofile owns only assessment validation/evaluation
  contract: none — register the bounded assessment extension in review-profiles/01 without cloning existing owners
  retires: []
  weight: verbs +0; flags +0 until concrete adapter design; refusals +0 authoritative policy; rule-text increases by the bounded profile definition
  why-add: Two requested assessment perspectives need explicit evidence and calibration; extend existing owners instead of adding standing services or unconditional lanes. Measure actual adapter weight at implementation.

## Read first

Read [the controlling spec](spec.md), [the plan](README.md), `tools/desk/internal/deskkit/reviewlanes.go`, `docs/streams/desk-supervision/review-finding-v1.md`, `docs/streams/desk-supervision/brief-20-review-scope-and-first-pass.md`, and the exact consumer paths above where already present. Refresh source/claims at pickup; planned paths are not existing capabilities.

## Human decision

After preparing the concrete routing and pilot evidence, decide whether to enable the proposed bounded advisory review scope. The decision packet must name eligible changes, publishing identity, cost ceilings, expiry, stopping rules and rollback. Offline implementation and synthetic qualification may proceed before this decision.

Options:
1. **Enable the bounded advisory scope** — use only the demonstrated profiles and limits, preserve existing required approvals and collect outcomes.
2. **Hold activation** — keep current routing and address the missing evidence first.

Default if no answer: none — blocks until answered.

No live routing or publication begins. Blocking status requires a separate decision.

## Ground rules

- Own isolated branch and draft PR; no main push, merge, workflow dispatch or live infrastructure contact.
- Use synthetic/public fixtures unless an explicit source/provider grant permits more. No live activation follows from this brief alone.
- Preserve correctness/security obligations and independent verification; stop at implemented.
- Missing information is explicit; do not convert unknown into zero or approval.

## Task

1. At pickup present the frozen calibration results, unresolved coverage, concrete selected repository/path/profile routing, numeric cost limits and rollback to the decision owner. Offline implementation may be prepared; live publication/routing begins only with the recorded scoped approval. No new mandatory approval lane or blocking policy is authorized.

2. Implement opt-in deterministic applicability routing from changed hot paths, algorithm/resource claims, contract/ownership changes and declared impact. Explain selected/not-applicable/unsupported. Architecture extends design-fit rather than running two independent assessments of the same meaning. Preserve existing contributor-tier, security, fact-check and fail-first obligations.

3. Flow the assessment through the existing packet, lane/profile identity, verdict and finding publication, current candidate/contract binding, class deduplication, first-pass inventory, three-round cap and ready aggregation. Advisory unavailable/failed runs remain visible; they must not grant missing required correctness/security evidence or become a new implicit blocker.

4. Qualify current-head and stale-head, same-class re-review, duplicate cross-profile findings, repeated dispatch, absent optional telemetry, unavailable profile and exhausted run budgets. Measure added end-to-end delay and resource cost. Revert scoped routing through configuration without deleting prior findings/evidence. A later blocking proposal needs a separate policy decision.

## Interface contract

Versioned, scoped inputs and exact source revisions → explained assessment with evidence, alternatives, coverage and resource provenance. Existing runtime, finding and acceptance owners retain authority. See Task for the exact producer/consumer seam this brief delivers.

## Verify

The named tests below are implementation deliverables, not claims that they exist today. Each must contain positive, negative and flow assertions described in Task; an empty test selection is not a pass.

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci +flow | `(cd tools/desk && o=$(mktemp) && trap 'rm -f "$o"' 0 && go test -count=1 -v -run '^TestReviewProfileIntegration$' ./internal/deskkit > "$o" && grep -F -- '--- PASS: TestReviewProfileIntegration ' "$o")` | Full dispatch-to-readiness fixtures preserve current required approvals and round counts; stale/advisory evidence never substitutes for required current-head evidence. |
| 2 | check:ci | `cd tools/desk && go test -count=1 ./internal/arch ./internal/deskkit` | Existing ownership, dependency, scope and finding tests remain green; do not weaken controls to pass. |
| 3 | check:ci +dereference | `statusgen --root . --consumers --brief review-profiles/05` | exit 0; the brief's routing claims are corroborated against the implementation diff, including each changed consumer. |

## Definition of done

All Task cases are demonstrated through the Verify tests and a reproducible local record with source versions, actual cost/coverage and unresolved limits. Runtime claims require observed evidence. No profile promotion or cadence activation is inferred.

## Evidence

Independent verifier appends evidence after implementation; none is claimed by authoring.

## Review

Apply the frontmatter gate and existing review scope. Acceptance remains independent of implementation.
