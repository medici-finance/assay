---
brief: assay:assay:desktools-v2:04
title: migrate deskclose off the hardcoded pullRequest GraphQL query (#1019)
why: >-
  deskclose hardcodes a pullRequest-shaped GraphQL query and so cannot target an issue — the
  exact reach-around the stream retires: a query shape built outside a backend. Routing it
  through a Forge op that addresses the target by kind (issue or change) both fixes the bug and
  proves the seam carries a real query, not just transport. It is a small, self-contained first
  migration whose old query is deleted as the seam op lands.
wave: 3
depends: ["desktools-v2/02", "desktools-v2/03"]
unblocks: []
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
authored: 2026-09-16 by desktools-v2 authoring session
sources:
  - "docs/streams/desktools-v2/spec.md §1 (#1019 row), §2 (commitment 4) — migrate-and-remove"
  - "docs/streams/desktools-v2/inventory.md (desktools-v2/01) — the deskclose query rows"
  - "tools/desk/cmd/deskclose/authority.go, tools/desk/cmd/deskclose/triage.go — where deskclose constructs its target query today"
  - "tools/desk/internal/deskkit/forge.go — GetIssueTyped / CloseIssueTyped / ListCommentsTyped already carry a TargetKind; the typed ops are the seam this migrates onto"
  - "freshness-checked 2026-09-16 @ e9fa19d3 — deskclose builds a pullRequest-shaped query in authority.go/triage.go; forge.go already exposes *Typed ops taking a TargetKind"
consumers:
  - "tools/desk/cmd/deskclose: follow-up desktools-v2/04 (this brief; the hardcoded query is replaced by a typed seam op and DELETED — flips to fixed-here when the implementation lands)"
  - "tools/desk/internal/deskkit/forge.go: out-of-scope (the *Typed ops already exist per forge.go; this brief consumes them, adds a method only if the inventory marked a GAP)"
exec-tier: any
domain: complicated
version: 1
id: b71f42a9-71c1-414b-a326-a9d5b2bdea8f
---

# Brief 04 — migrate deskclose off the hardcoded pullRequest GraphQL query

## Context

files:
- `tools/desk/cmd/deskclose/authority.go`, `tools/desk/cmd/deskclose/triage.go` — where the
  hardcoded `pullRequest`-shaped query lives (#1019); replaced by a typed seam op, then DELETED.
- `tools/desk/internal/deskkit/forge.go` — the `*Typed` ops (`GetIssueTyped`,
  `CloseIssueTyped`, `ListCommentsTyped`) that address a target by `TargetKind`.

single-point-of-failure: none required — this is a query-shape migration on a
non-credential, reversible read/close path (a close is state-reversible via `ReopenIssue`).
The independence that matters here is that the ban-lint (`desktools-v2/02`) catches a
RE-introduced `pullRequest` query literal in a different component (CI) than the unit test
that pins the issue-targeting behavior; the two fail on different signals.

facts:
- `forge.go` already exposes `GetIssueTyped` / `CloseIssueTyped` / `ListCommentsTyped`, each
  taking a `TargetKind`, precisely so a caller can address an issue OR a change without a
  hand-built query. If the inventory marked a needed op as a `GAP`, add it in the same change
  (freeze rule: a new method lands with its consuming call site).
- #1019's defect is that deskclose can only target a pullRequest. The fix is behavioral: it
  must close an ISSUE via the typed op. A Verify row must exercise the issue path, not only
  the change path.
- The old query literal is DELETED, not left as a fallback. The ban-lint's count must drop by
  the deskclose query rows.
- Out of scope: any credential change; touching another tool; editing the ban-lint.

## Ground rules
- NEVER git push to main / trigger workflows / run mutating infra commands. Feature branch +
  draft PR only.
- Stop at `implemented` — you do not set verified/done.
- NEVER commit `STATUS.md` or `docs/streams/FINDINGS.md` on a branch.
- Public repo: `example-*` placeholders; no absolute machine paths, private slugs, or session ids.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Replace deskclose's hardcoded `pullRequest`-shaped query in `authority.go`/`triage.go`
   with the typed seam op that addresses the target by `TargetKind`.
2. DELETE the old query literal in the same change (no dormant fallback).
3. Add/extend a test proving deskclose closes an ISSUE (not only a change) through the seam.
4. Record in the PR body the ban-lint count before/after (the deskclose query rows removed).

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `cd tools/desk && go build ./... && go vet ./cmd/deskclose/` | exit 0 |
| 2 | `cd tools/desk && go test ./cmd/deskclose/` | exit 0; the issue-targeting test passes |
| 3 | `grep -n 'pullRequest' tools/desk/cmd/deskclose/authority.go tools/desk/cmd/deskclose/triage.go` | exit 1 (no match) — the hardcoded `pullRequest` query literal is GONE from deskclose's non-test source (the removal, not just the addition; scoped to the two files that carry it today, since test stubs legitimately name it) |
| 4 | `cd tools/desk && go test ./cmd/deskclose/ -run TestDeskcloseTargetsIssue -v` | exit 0; the named test runs (`--- PASS`) proving deskclose closes an issue through the typed seam op — the dereferencing row for the #1019 behavior fix |
| 5 | `sh tools/desk/scripts/forge-ban.sh > /tmp/dv2-fb4.txt 2>&1; grep -oE 'reach-around sites: [0-9]+' /tmp/dv2-fb4.txt` | exit 0; count STRICTLY LOWER than brief 03's recorded value (deskclose's query reach-around is gone) |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item. -->

## Review
Gate: model (all four risk answers no — a query-shape migration on a reversible close path; no
credential change, no capability removed beyond the reach-around this stream exists to remove).
Row 3 is the removal check (the old literal is gone, exit 1 on match); row 4 dereferences the
#1019 behavior fix (an issue is actually closed through the seam). Reviewer records verdict +
date in the stream README table.
