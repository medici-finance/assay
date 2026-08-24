---
brief: forge-gitlab/02
title: gitlab forge implementation — MRs, notes, approvals, statuses over REST v4
why: >-
  With the Forge seam in place, GitLab support is one implementation file away. This
  brief delivers it: every fleet operation (draft MRs, review approvals, verdict
  comments, check reads, award-emoji gates, issue filing) working against GitLab REST
  v4 with the same semantics the goldens pin for GitHub — the difference between
  "portable in principle" and a fleet that runs.
wave: 2
depends: ["forge-gitlab/01"]
unblocks: ["forge-gitlab/04"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-08-24 by forge-gitlab authoring session
sources:
  - "docs/streams/forge-gitlab/spec.md §6 (concept mapping), §2 (identity model)"
  - "docs/streams/forge-gitlab/brief-01-forge-interface-extraction.md — the interface + goldens this implements against"
  - "freshness-checked 2026-08-24 @ 5c4a67d — no GitLab code exists anywhere in tools/"
exec-tier: strong
exec-tier-why: "semantic-equivalence mapping across forges (question b): a plausible-but-wrong mapping (e.g. approval vs approval-rule eligibility) survives happy-path tests."
domain: complicated
---

# Brief 02 — gitlab forge implementation

## Context
files:
- `tools/desk/internal/deskkit/forge_gitlab.go` (new) + `forge_gitlab_test.go`.
- `docs/streams/forge-gitlab/inventory.md` (planned) — created by forge-gitlab/01; tick each op as implemented.

single-point-of-failure: the concept-mapping table is the one design control — backed by
contract tests that run the SAME golden scenarios against a recorded GitLab fixture set,
so a wrong mapping fails a fixture, not a pilot.

facts:
- Concept mapping (spec §6): draft PR ↔ MR with `Draft:` title prefix; review approval ↔
  MR approval endpoint; required check ↔ pipeline status at head SHA; reaction admission
  gate ↔ award emoji on the issue/MR; `Fixes #N` ↔ `Closes #N`; review-at-head ↔
  approvals + notes filtered by head SHA.
- Auth: PAT via `PRIVATE-TOKEN` header, token VALUE read from the custody file path —
  never construct URLs embedding tokens; custody mechanics are forge-gitlab/03's
  deliverable, this brief consumes the file contract only.
- Error mapping must distinguish 401 (credential), 403 (permission/tier), 404
  (visibility) — tier-gated features (approval rules, external status checks) return
  errors the tools must surface as could-not-check, never as clean.
- Pagination: GitLab uses `X-Next-Page` headers, not link relations — cover in tests.

## Ground rules
- NEVER git push / trigger workflows / run mutating infra commands. Commit only per
  the task instructions.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Implement `forge_gitlab.go` covering every inventory operation.
2. Contract tests: replay brief-01's golden scenarios against recorded GitLab REST
   fixtures (httptest server) — same scenario names, so a gap is a named missing row.
3. Three-state error surface: tier/permission failures return could-not-check errors
   distinct from empty results.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `go build ./... && go test ./tools/desk/internal/deskkit/ -run TestForgeGitlab -v` | exit 0; output contains `PASS` |
| 2 | `go test ./tools/desk/internal/deskkit/ -run TestForgeGitlabCoverage -v` | exit 0 — the test reads inventory.md and fails naming any inventoried op with no gitlab contract test (dereference: coverage measured against the committed inventory, not asserted) |
| 3 | `go test ./tools/desk/internal/deskkit/ -run TestForgeGitlabTierErrors -v` | exit 0; output contains `could-not-check` for a 403 fixture |

## Evidence
<!-- one row per Verify item — filled by a NON-implementer -->

## Review
Gate: model (from frontmatter). Reviewer records verdict + date in the stream README
table. Reviewer spot-checks two mappings against live GitLab API docs, not the fixture
set alone.
