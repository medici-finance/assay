---
brief: "assay:assay:review-profiles:06"
title: "Qualify operational assessment and the bounded repair handoff"
why: "A good PR review cannot see fleet-wide repeated work or accumulated drift. Reuse the profiles on operational evidence and prove the improvement handoff without adding another scheduler."
wave: 3
depends: ["review-profiles/04"]
unblocks: []
effort: "M"
gate: "model"
risk: {"regulatory": "no", "customer": "no", "irreversible": "no", "sensitive-data": "no"}
issues: []
schema: "brief-v2"
version: 1
id: "a2e1f361-5ffb-4399-8077-56228ca7baaf"
authored: "2026-10-15 by review profile authoring"
sources: ["docs/streams/review-profiles/spec.md", "freshness-checked 2026-10-15 @ 7d52be61b66cb81f5ad2ab7e1d250747a6f125de"]
exec-tier: "strong"
exec-tier-why: "Cross-profile evidence, algorithm and authority judgments require the applicable contracts and negative controls."
domain: "complicated"
outcome: "none"
consumers: ["docs/review-profile-operations.md: follow-up review-profiles/06", "tools/desk/internal/reviewprofile/operations.go: follow-up review-profiles/06", "tools/desk/internal/reviewprofile/operations_test.go: follow-up review-profiles/06", "tools/desk/internal/reviewprofile/testdata/operations/: follow-up review-profiles/06", "docs/streams/continuing-operations/proposed-briefs.md: follow-up review-profiles/06"]
---

# Brief 06 — Qualify operational assessment and the bounded repair handoff

## Context

files: `docs/review-profile-operations.md` (planned), `tools/desk/internal/reviewprofile/operations.go` (planned), `tools/desk/internal/reviewprofile/operations_test.go` (planned), `tools/desk/internal/reviewprofile/testdata/operations/` (planned), `docs/streams/continuing-operations/proposed-briefs.md` — New profile, calibration and operations files are planned deliverables; existing desk owners remain controlling.

facts:
- At the pinned main there are four review lanes, existing advisory design-fit and architectural fitness tests; no calibrated performance/architecture routing policy exists.
- Review finding identity, scope and rounds retain their existing owner; required acceptance evidence and budget accounting do not depend on optional telemetry.
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

## Ground rules

- Own isolated branch and draft PR; no main push, merge, workflow dispatch or live infrastructure contact.
- Use synthetic/public fixtures unless an explicit source/provider grant permits more. No live activation follows from this brief alone.
- Preserve correctness/security obligations and independent verification; stop at implemented.
- Missing information is explicit; do not convert unknown into zero or approval.

## Task

1. Provide a local/imported-evidence manual operational assessment using the same profile/manifest versions. Separate daily performance ranking by frequency×cost, actual CPU/tokens/forge/wall/queue and verified-brief cost from periodic architecture direction/drift/temporary-path assessment. No collection deployment or cell adoption prerequisite.

2. Name observation window, source coverage, accountable practice owner and independent evaluator. Preserve actual versus proxy usage and source freshness; absent tools cannot appear efficient merely because uninstrumented. Include claimed PR benefits requiring later exposure and adverse/disconfirmed outcomes.

3. Exercise confirmed observation → deduplicated issue proposal → bounded admitted repair → draft PR → independent review/verification → exposure reassessment using existing intake/work/effect interfaces and synthetic offline fixtures. Preserve occurrence/effect identity, cumulative budgets, cancellation, stop/escalation and restart semantics. The draft-PR step is exercised as a synthetic fixture of the performance-collection/09 proposal path (its executor writes the `performance-repair` label and body marker, and its repair PRs are never admitted to an automated merge lane); this brief opens no repair PR of its own. This brief implements no automatic posting, merge, deployment, target rewrite or new scheduler.

4. Document future mapping to continuing-operations O08/O10/O11 and the single cell controller: definitions, pure policy, coordinated attempts, isolated workers, role-bound effects, canonical evidence and optional performance copies remain distinct. Qualification must include stale evidence, duplicate cadence triggers, lost authorization and exhausted budget. Any actual cadence or repair authority is separately approved.

## Interface contract

Versioned, scoped inputs and exact source revisions → explained assessment with evidence, alternatives, coverage and resource provenance. Existing runtime, finding and acceptance owners retain authority. See Task for the exact producer/consumer seam this brief delivers.

## Verify

The named tests below are implementation deliverables, not claims that they exist today. Each must contain positive, negative and flow assertions described in Task; an empty test selection is not a pass.

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci +flow | `(cd tools/desk && o=$(mktemp) && trap 'rm -f "$o"' 0 && go test -count=1 -v -run '^TestOperationalAssessment$' ./internal/reviewprofile > "$o" && grep -F -- '--- PASS: TestOperationalAssessment ' "$o")` | Manual offline journey works without telemetry backend; duplicate/stale/over-budget repair proposals cannot become new effects; adverse outcomes remain assessable. |
| 2 | check:ci | `cd tools/desk && go test -count=1 ./internal/arch ./internal/deskkit` | Existing ownership, dependency, scope and finding tests remain green; do not weaken controls to pass. |
| 3 | check:ci +dereference | `statusgen --root . --consumers --brief review-profiles/06` | exit 0; the brief's routing claims are corroborated against the implementation diff, including each changed consumer. |

## Definition of done

All Task cases are demonstrated through the Verify tests and a reproducible local record with source versions, actual cost/coverage and unresolved limits. Runtime claims require observed evidence. No profile promotion or cadence activation is inferred.

## Evidence

Independent verifier appends evidence after implementation; none is claimed by authoring.

## Review

Apply the frontmatter gate and existing review scope. Acceptance remains independent of implementation.
