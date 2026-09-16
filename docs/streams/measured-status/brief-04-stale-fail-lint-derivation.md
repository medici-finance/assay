---
brief: assay:assay:measured-status:04
title: "statusgen --lint: derive stale-FAIL vs missing-card from commit dates, and route each state to the verify desk instead of nudging a worker to hand-file a sign-off"
why: >-
  A gate:human brief whose last Evidence entry is a VERIFY: FAIL that predates the merge that
  fixed the failing row draws a lint nudge that a worker reads as "hand-file a sign-off issue"
  — but that issue is not the verify-gate card, closing it flips nothing, and the brief keeps
  routing back to dispatch with an empty diff. The lint decides the state from the Evidence
  text alone; it should DERIVE whether the FAIL is stale by comparing the fix commits' dates to
  the Evidence date, and route a stale FAIL to a re-verify, never to a person's signature.
wave: 1
depends: []
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: [862]
schema: brief-v2
authored: 2026-09-16 by measured-status scoping session
sources:
  - "#862 — a stale VERIFY: FAIL on a gate:human brief nudges workers to hand-file a sign-off"
  - "statusgen/brieffile.go — the gate:human-at-implemented lint nudge (the `no decision-issue` message)"
  - "statusgen/testdata — the per-state fixture trees the lint tests read"
  - "freshness-checked 2026-09-16 @ e9fa19d3 — the nudge does not distinguish a canonical VERIFY: PASS-no-card state from a stale-FAIL state, and its text tells a worker to file rather than naming --decision-issues / a re-verify"
exec-tier: strong
exec-tier-why: changes lint routing logic that steers worker vs verify-desk vs human effort; a mis-derived state sends work down the wrong lane, which is the exact failure the issue reports
domain: complicated
value: med
version: 1
id: 666a7cba-7992-475b-ac9c-e5435b6e4be8
---

# Brief 04 — Derive the stale-FAIL vs missing-card lint state

## Context
files:
- `statusgen/brieffile.go` — the gate:human-at-`implemented` lint nudge.
- `statusgen/testdata/` — add a fixture tree per state under the lint's testdata.
- `statusgen/*_test.go` — the lint test that reads the new fixtures.
facts:
- three states the lint must distinguish (per #862): (1) gate:human, last Evidence is a
  canonical VERIFY: PASS, no card -> "sign-off card missing — verify-desk lands the marker";
  (2) gate:human/gate:model, last Evidence is VERIFY: FAIL and the brief's `files:` paths or
  the cited fix issue have commits on main NEWER than the Evidence date -> "stale FAIL — a
  non-implementer RE-VERIFY supersedes it", naming the newest commit; (3) Evidence empty or the
  newest FAIL is current -> the existing nudge stays, retexted to say the decision issue is
  filed by --decision-issues (the tool), not a worker.
- statusgen is pure over the tree plus git-readable history; deriving "newer than the Evidence
  date" uses the same commit-history read attribution.go already performs, degraded three-state
  when history is unavailable (could-not-check, never rounded to "current").
- statusgen is one Go module; tests run from the repo root.

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task instructions only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Split the single nudge into the three states above, deriving state (2) from a commit-date
   comparison (fix commits vs Evidence date), not from the Evidence text alone. Preserve the
   three-state read: an unavailable history is a could-not-check that keeps the conservative
   nudge, never a silent "current".
2. Retext state (3)'s message so it names `--decision-issues` (the tool) as the filer, not a worker.
3. Add one fixture tree per state under the lint's testdata and a test asserting the current
   nudge text no longer appears for states (1) and (2), and the stale-FAIL message names the
   newest commit.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `go test ./statusgen/ -run TestStaleFailVsMissingCard -count=1` | exit 0; output contains "ok" |
| 2 | `go test ./statusgen/ -run TestStaleFailVsMissingCard -count=1 -v 2>&1 \| grep -q 'PASS'` | exit 0 (all three fixture-state assertions ran and passed) |
| 3 | `ls statusgen/testdata/*stale*fail* statusgen/testdata/*missing*card* >/dev/null 2>&1` | exit 0 (a fixture tree exists per state) |
| 4 | `go test ./statusgen/ -run TestStaleFailVsMissingCard -count=1 -v 2>&1 \| grep -q 'stale FAIL'` | exit 0 (the stale-FAIL branch's message is exercised) |

## Evidence
<!-- appended at implementation time by a non-implementer -->

## Review
Gate: model (from frontmatter). Reviewer records verdict + date in the stream README table.
