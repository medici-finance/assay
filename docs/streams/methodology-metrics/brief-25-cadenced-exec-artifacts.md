---
brief: methodology-metrics/25
title: 'Cadenced exec artifacts — weekly WBR deck + monthly exec summary, scheduled beside the daily jobs'
wave: 2
depends: ["methodology-metrics/23", "methodology-metrics/22"]
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-12 by Fable desk session (human:<name> direction)
sources: ["human:<name> 2026-07-12: 'while these are produced daily, we will need other cadence''s to produce other artifacts for execs. so there should also be weekly/monthly decks/jobs scheduled that do that'", "docs/streams/methodology-metrics/roadmap-deck-research.md (WBR dual-time-window finding; OKR/theme panel is quarterly-not-daily)", "methodology-metrics/22 (daily harvest — the scheduling machinery this generalizes)", "methodology-metrics/16 (--dora --series ISO-week buckets — the weekly deck's core data, todo)", "methodology-metrics/23 (the roadmap renderer this reuses)", "docs/streams/RETRO.md (weekly retro — the WBR deck is its evidence input)", "freshness-checked 2026-07-12 @ post-#365 main"]
why: >-
  Different exec questions live on different clocks: "where is the portfolio constrained
  today" (daily grid), "how did the week trend and what does the retro act on" (weekly),
  "what shipped, what moved strategically, is every objective covered" (monthly). One daily
  deck stretched to answer all three bloats it; the research's WBR finding is dual time
  windows on a fixed skeleton. The scheduling substrate (mm/22) exists once — adding
  cadences is a schedule-matrix change, not new machinery.
---

# Brief 25 — Cadenced exec artifacts (weekly / monthly)

## Context

files: the mm/22 workflow (extend its schedule matrix), `../assay-toolkit/statusgen/` (cadence flags
reusing mm/23's renderer), outputs under docs/reports/weekly/ (planned) and
docs/reports/monthly/ (planned).

facts:
- **Weekly (WBR deck, generated Monday 06:00 UTC over the prior ISO week):** week-over-week
  stage-transition totals per stream (historian), DORA series row for the closed week
  (mm/16 — if unlanded, emit the section as "pending mm/16" rather than blocking), exceptions
  that OPENED and CLOSED during the week (not just point-in-time), verification-debt trend,
  gate-queue age trend. Audience: the retro (R-NN evidence input) + human:<name>'s week view. Same
  dark Medici deck surface and fixed skeleton as the daily (research finding 3).
- **Monthly (exec summary, 1st of month):** shipped/done roll-up by stream, portfolio
  stage-mix trend chart (month of daily totals), the goal-coverage panel (research: a
  strategy view that changes slowly — monthly is its home; built on mm/23's `serves:` tags
  and its FIXED priority order — **lending-app is the main aim / money-maker, reconciler
  second, assay supporting (human:<name> 2026-07-12)** — absent tag renders "untagged", which is
  itself the signal). The monthly answers the long-term-vs-short-term question the daily
  can't: the month's effort mix by goal vs the stated order, trend of that mix, and an
  explicit callout when supporting work outweighed revenue work for the month. Plus notable
  findings/retro changes. Audience: exec.
- Cadence discipline: all three cadences are entries in ONE workflow's schedule matrix
  (mm/22's), each writing its own docs/reports/<cadence>/ directory with the same
  [skip-status-regen] committer — no parallel scheduling systems.
- Everything computed; the monthly's prose sections are assembled from register/historian
  facts, never free-written by the job.
- consumers (rule 6): RETRO conventions (weekly deck becomes the retro's standing evidence
  link), the-desk boot (weekly on Mondays), assay exec surfaces (monthly is the outward
  show-piece), mm/22 workflow (the schedule matrix lives there — one-line amendment when
  this lands).

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task
  instructions only. (The scheduled runs themselves committing artifacts are the deliverable.)
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task

1. `statusgen --roadmap --cadence weekly|monthly` (or split flags — implementer's call,
   recorded in Evidence): weekly WBR deck and monthly exec summary per the facts above,
   reusing mm/23's renderer/rule-table; graceful "pending mm/16" degradation.
2. Extend the mm/22 workflow's schedule matrix with the two cadences (weekly Monday
   06:00 UTC; monthly 1st 06:00 UTC) + workflow_dispatch cadence input for human-triggered
   first runs.
3. `theme:` optional key in stream README frontmatter + the unmapped-renders-visibly rule.
4. Tests: cadence window computation (ISO-week and month boundaries), opened-AND-closed
   exception accounting, unmapped-theme rendering.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `statusgen --root . --roadmap --cadence weekly && test -f docs/reports/weekly/$(date +%G-W%V)/index.html` (literal form per implementer's naming, recorded in Evidence) | exit 0 |
| 2 | `statusgen --root . --roadmap --cadence monthly && command ls docs/reports/monthly/` | a month-named artifact exists |
| 3 | `yq eval '.on.schedule \| length' ../oit/.github/workflows/daily-harvest.yml` | ≥ 3 cron entries (daily + weekly + monthly). Path is mm/22's shipped workflow, substituted for the `<mm/22 workflow file>` metavariable it used to carry — a bare `<…>` is shell input redirection, so the row could not run as written |
| 4 | `(rc=0; for t in Cadence Roadmap; do out=$(go test ./statusgen/ -count=1 -run "$t" -v 2>&1); tr=$?; { [ $tr -eq 0 ] && printf '%s' "$out" \| grep -q -- '--- PASS'; } \|\| { echo "MISSING-OR-FAIL $t"; rc=1; }; done; exit $rc)` | exit 0, prints nothing — both named test groups EXIST and pass. Exit status is captured (`tr=$?`) and asserted BEFORE the `--- PASS` check, so a FAILING test in the group also goes red — the previous pipeline form discarded `go test`'s status and passed on a red suite. **RED at 2026-08-03: `MISSING-OR-FAIL Cadence`, rc=1** — and correctly so. This brief is `todo`; Task 1's `--cadence` mode is unbuilt (`statusgen/main.go:672` declares `-roadmap` and no `-cadence`; `grep -rn 'cadence' --include='*.go' statusgen/` returns only prose comments about release cadence, **0** flag or window-computation code), so the Task-4 cadence-window tests do not exist yet. Measured per group: `Roadmap` 20 `--- PASS`, `Cadence` **0**. The row is not retuned to drop the missing token — it goes green when the brief lands, and the missing half is exactly the verification debt this row is meant to surface |
| 5 | `statusgen --root . --lint` | exit 0 |

## Evidence
<!-- appended at implementation time -->

## Review
Gate: model. Reviewer records verdict + date in the stream README table.
