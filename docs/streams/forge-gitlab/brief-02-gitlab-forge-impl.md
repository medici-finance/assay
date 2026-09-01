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
unblocks: ["forge-gitlab/04", "forge-gitlab/08"]
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
tier: free
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

## Edition
Minimum GitLab tier: **free** (Community Edition). Every operation this brief implements is
Free-tier — merge requests and the `Draft:` prefix, notes, award emoji, the approve endpoint,
pipeline and commit-status reads, issues, project reads (edition-matrix.md table A, rows
1-14, one docs citation per row).

What degrades on CE: **`ReviewsAtHead`'s head pin**. The approvals endpoint carries no
per-approval SHA, so pinning an approval to the head relies on the project setting "Remove all
approvals when commits are added to the source branch", which is `Tier: Premium, Ultimate`
(https://docs.gitlab.com/user/project/merge_requests/approvals/settings/). Fallback: the
verdict NOTE carries the head SHA — the desk already writes it — so at-head reading works from
the note body, and on CE the raw approval flag is treated as unpinned and advisory. The
three-state error surface this brief already builds (task 3) is the right home for that
distinction: an unpinnable approval is could-not-check, never clean.

Making an approval *required* before merge is a different, Premium control that lives in
brief 04's provisioning, not here — see edition-matrix.md rows B3 and B4.

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

### Non-implementer verifier run — VERIFY: PASS — 2026-09-01 opus-4.8[1m]-verifier (verify-desk dispatch), merged main `09de1a1`

Runner ≠ implementer. Own temporary worktree off `origin/main` at `09de1a1`, offline (`KUBECONFIG=/dev/null`); no live forge contacted (tests use `httptest` fixtures). Path note: the module root is `tools/desk` (multi-module repo, no root `go.work`), so each row was run module-scoped from `tools/desk/` with the module-relative selector — the identical package and test set the repo-root-relative command names.

| # | Command | Exit | Result |
|---|---------|------|--------|
| 1 | `go build ./...` + `go test ./internal/deskkit/ -run TestForgeGitlab -v` (from `tools/desk`) | 0 | build 0; `ok …/deskkit`; 18 `TestForgeGitlabGolden` subtests (post_comment, post_review_*, mark_ready_for_review, file_issue, close_issue, delete_ref_*, push_transport_hint, error_not_found, error_forbidden_tier, error_forbidden_approval_config_tier) all PASS |
| 2 | `go test ./internal/deskkit/ -run TestForgeGitlabCoverage -v` | 0 | PASS — "gitlab contract corpus covers all 15 operations of the frozen Forge interface"; committed inventory reconciles (15 ops, all covered) — measured against the committed inventory, not asserted inline |
| 3 | `go test ./internal/deskkit/ -run TestForgeGitlabTierErrors -v` | 0 | PASS — `could-not-check` emitted for the 403 fixture (4×); subtest `tier_failure_is_not_an_empty_list` confirms a Premium-gated 403 ≠ empty list |

`RISK-VALUE: DERIVED — gitlabDraftPrefix = "Draft: " @ tools/desk/internal/deskkit/forge_gitlab.go:269` — the single control deciding draft vs open-for-review; derived, not merely named: the create path asserts the returned MR came back `mr.Draft` and refuses with could-not-check otherwise (forge_gitlab.go:987-995), replayed by TestForgeGitlabGolden against the recorded fixture. No fail-safe trigger fires (gate:model, risk block all-no, non-risk-classed path; client-lib version pins live in go.mod, not in guard code).

**VERIFY: PASS** — all three Verify rows checked-clean by a non-implementer. Advancing `implemented → verified`.

## Review
Gate: model (from frontmatter). Reviewer records verdict + date in the stream README
table. Reviewer spot-checks two mappings against live GitLab API docs, not the fixture
set alone.

## Answer recorded 2026-08-30 — "either/both as libraries"

- **Seam:** `Forge` interface, `tools/desk/internal/deskkit/forge.go:173` — a closed set of
  typed operations, no generic `Do(endpoint)` passthrough on either backend.
- **GitHub backend** = `github.com/cli/go-gh/v2` v2.13.0 (pinned in `tools/desk/go.mod`),
  `pkg/api` REST client, `forge_github.go:12` (#214).
- **GitLab backend** = `gitlab.com/gitlab-org/api/client-go` v1.46.0, `forge_gitlab.go:13`
  (#218, this brief) — the same official client `glab` itself ships on top of.
- Both are library-backed transports; neither `gh` nor `glab` is exec'd on the Forge path.
- **Enforcement:** `tools/desk/internal/forgeban` (#230) is a two-layer static scan
  banning `exec.Command("gh"|"glab", …)` across `tools/desk`, asserted by
  `TestNoForgeCLIShellout` (`forge_surface_test.go:46`) and `TestForgeNoPassthrough`
  (`forge_surface_test.go:117`, confirming no arbitrary-endpoint method survives).
- **Remaining exceptions:** a permit-only allowlist ratcheted at 23 call sites
  (`tools/desk/internal/forgeban/allowlist.go`, ceiling asserted at
  `forgeban_test.go:330`) — mostly tools that run under the caller's ambient CLI identity by
  documented design (deskclose, deskfile, deskdigest, deskflip, deskpr, deskreply), plus
  unenumerated ops (label create/edit, PR listing, branch→PR lookup, repo-hardening reads).
- No brief retires these yet: #230 closed the *enumerated* op surface. The identity
  question is ruled — #230, comment 5470351713 (2026-08-30) relays the driver's answer
  as option (a): every desk tool moves to a minted role-App identity, `deskpr`'s
  `--as-app=false` ambient fallback retires — but the migration of the 23 allowlisted sites
  is follow-up work in this stream, not yet authored as briefs.
