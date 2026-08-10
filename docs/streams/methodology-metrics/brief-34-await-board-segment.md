---
brief: methodology-metrics/34
title: Segment the Awaiting board by blocker owner (desk / human-gate / rework / paused / env)
wave: 0
depends: []
unblocks: []
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: [885]
schema: brief-v1
authored: 2026-07-20 by Opus 4.8 authoring session (intake Tier-2, #885)
sources: ["[I-board-segment](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-17-segment-awaiting-board-by-blocker-owner.md)", "docs/streams/methodology/verify-desk-bottleneck-2026-07-17.md (R1)", "methodology-metrics/10 (the debt alarm this refines)", "tools/statusgen/emit.go debtCounts + Awaiting emit", "#885 (tracking)"]
why: >-
  The single "N awaiting verification" headline conflates five different blockers with
  different owners: desk-actionable, awaiting-human-gate (PASS Evidence recorded, waiting on
  human:<name>), awaiting-implementer-rework (VERIFY:FAIL), paused-stream, and env-blocked. Measured
  2026-07-17, ~two-thirds of ~40 rows were NOT desk-actionable, yet all count at full weight
  and the oldest rows are all in the non-desk classes — so the headline overstates desk
  failure and the desk burns boot-time re-triaging rows it cannot move.
---

# Brief 34 — Segment the Awaiting board by blocker owner

## Context
files:
- `../assay-toolkit/statusgen/emit.go` — `debtCounts()` (awaiting = implemented+verified today) and the
  Awaiting-heading + table emit (`## Awaiting verification / review (…)`, emit.go:149-157);
  `debtNotice()` (mm/10 alarm)
- `../assay-toolkit/statusgen/emit_test.go` — table + heading tests
- `../assay-toolkit/statusgen/model.go` — `Stream` (carries stream `status`) / `Brief`
- `docs/streams/methodology-metrics/README.md` — one convention line

facts:
- Blocker-owner classification for an awaiting (implemented|verified) row:
  - **human-gate** — `gate: human` AND the LAST verdict recorded in Evidence is a model
    `VERIFY: PASS` (waiting on human:<name>'s sign-off, per mm/12's surface — NOT desk-actionable).
  - **rework** — the LAST verdict recorded in Evidence is `VERIFY: FAIL` (waiting on the
    implementer, not the desk).
  - **Last verdict WINS, not "any occurrence ever"** — Evidence sections accumulate, so a brief
    that failed, was reworked and passed carries both markers. Substring-matching `VERIFY: FAIL`
    anywhere pins a passed brief in rework permanently (live instance when this was written:
    `issue-loop/09`, status `verified`, three superseded FAILs above its promoting PASS). The
    verdict test tolerates the marker forms actually in use (the bold span wraps a longer
    phrase far more often than not) and ignores markers inside code fences, blockquotes and
    struck-through spans, which are quotations rather than verdicts. It stays SEPARATE from
    `hasVerifyPass`, which gates human sign-off and must keep failing closed.
  - **paused** — the owning stream's frontmatter `status` is `paused` (e.g. midnight-poc).
  - **env-blocked** — the brief row carries an explicit env-block marker (a `blocked: env`
    frontmatter field OR the label the intake names; if no such marker exists yet, define it
    as an optional brief-v1 `blocked-by: env` field — additive, absent = not env-blocked).
  - **desk-actionable** — everything else (the residual; this is the class the desk drains).
- Design: the Awaiting emitter renders sub-tables (or a per-row Owner column) keyed by blocker
  owner. The desk-actionable count becomes the headline; paused + human-gated + rework + env
  stay VISIBLE (hiding debt is the failure mode) but out of the headline number.
- **mm/10 coupling (REQUIRED):** the debt alarm (`debtNotice`) and any oldest-first drain
  order key off the **desk-actionable** count ONLY, not the conflated total. Update
  `debtCounts` / `debtNotice` accordingly (the threshold now measures the queue the desk can
  actually move). Do not change the threshold constant itself — only which count it compares.
- Classification is derived at render time from data already loaded (stream status, Evidence
  text, gate, the env marker) — no new historian dependency, no network. Reads current state,
  so wave 0.
- Out of scope: age percentiles per class (needs mm/01 history; mm/03 owns time-series); the
  gate-score ordering (mm/11 owns that); deskboard's PR-side view (desk-tools).

## Ground rules
- NEVER git push to main / trigger workflows / mutating kubectl. Feature branch + draft PR only.
- Stop at `implemented` — you do not set verified/done.
- NEVER commit STATUS.md on a branch (single writer = main's CI).
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. TDD — failing tests first: a fixture set of awaiting briefs spanning all five classes;
   assert each lands in the right segment; assert the headline/desk-actionable count excludes
   paused + human-gate + rework + env; assert the debt NOTICE keys off the desk-actionable
   count, not the total.
2. Implement the classifier + segmented render in emit.go; wire `debtCounts`/`debtNotice` to
   the desk-actionable count. If an `env` marker field is introduced, parse it in the brief
   frontmatter (additive optional field; document at its declaration).
3. README: one line under conventions — the Awaiting board segments by blocker owner; the debt
   alarm measures the desk-actionable slice only.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `go test ./statusgen/ -run Segment -v` | exit 0; `TestSegment*` covers the five-class split, headline-excludes-non-desk, and alarm-keys-off-the-desk-actionable-count |
| 1b | `go test ./statusgen/ -run TestBlockedBy -v` | exit 0 **AND** the output carries a `--- PASS:` line for all three of `TestBlockedByParsedWithoutExecTierWhy`, `TestBlockedByParsedAlongsideExecTierWhy`, `TestBlockedByInvalidValueIsProblem`. `no tests to run` is a **FAIL** of this row, not a pass — `go test` exits 0 when a `-run` pattern matches nothing, so the exit code alone proves nothing (#509, #580). Proves the PARSE path: `blocked-by` parses on a brief with no `exec-tier-why`, and an invalid value is a PROBLEM that never reaches the row |
| 1c | `go test ./statusgen/ -run TestGateAndEvidenceWiredOntoRow -v` | exit 0 **AND** a `--- PASS: TestGateAndEvidenceWiredOntoRow` line; `no tests to run` is a FAIL. Kept a separate row because it shares no substring with row 1b's tests and is **not** matched by row 1's `-run Segment` — one `-run` token per row is the only form that is unambiguous in both a shell and a GFM table cell (a bare `\|` alternation splits the cell; an escaped `\\|` is a literal pipe that matches nothing) |
| 1d | mutation row (brief-rules 16): delete `row.Gate`, `row.Evidence`, the `row.BlockedBy` wiring, or the invalid-`blocked-by` `add(...)` from `statusgen/brieffile.go`, then `go test ./statusgen/` | RED each time — a green suite here means the wiring is untested and a misplaced brace ships |
| 2 | `go test ./statusgen/ && go vet ./statusgen/` | exit 0 |
| 3 | `statusgen --root . && grep -A2 'Awaiting verification' STATUS.md` | heading/segments render; paused + human-gate rows appear under their own segment, not the headline (STATUS regen local-only, NOT committed) |
| 4 | `statusgen --root . --lint; echo $?` | 0 |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item. -->

## Review
Gate: model (all four risk answers no — repo-internal Go tooling; a board-truthfulness
presentation change plus retargeting the mm/10 alarm at the desk-actionable count). Reviewer
records verdict + date in the stream README table.
