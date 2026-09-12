---
brief: assay:assay:contributor-trust:08
title: "External-contribution metrics — inbound pull requests by tier and outcome, on the board"
why: >-
  Every control in this stream is a bet: that deeper review on unknown authors catches things,
  that the tiers get used, that the bar filters without deterring. None of those bets can be
  settled by argument, and today nothing counts inbound external pull requests at all — not
  how many arrive, not what tier they arrive at, not whether they merge or close. A small
  counted view is what lets the project loosen a control that is costing more than it catches,
  or tighten one that is catching nothing, on evidence instead of on the memory of the last
  bad week.
wave: 1
depends: ["contributor-trust/02"]
unblocks: []
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
authored: 2026-09-12 by a contributor-trust authoring session
sources:
  - "docs/streams/contributor-trust/spec.md §3 — the three consequences of a binary bar; this brief instruments whether the fix worked."
  - "docs/streams/contributor-trust/brief-02-trust-tiers-and-ledger.md — the tier vocabulary the counts are keyed on, and the rule that the ledger's rows are never published. This view publishes COUNTS PER TIER and no identity."
  - "statusgen/nextup.go and the board's existing generated sections — the established pattern for a generated board view between statusgen markers, which this view follows rather than inventing a second rendering path."
  - "freshness-checked 2026-09-12 @ e96b7f6d (origin/main) — the board carries no inbound/external view of any kind; nothing counts external pull requests."
consumers:
  - "statusgen/externalpr.go: follow-up contributor-trust/08 (this brief; the counter and its renderer)"
  - "docs/contributor-trust.md: follow-up contributor-trust/08 (this brief; the published model gains a note that counts are published and identities are not)"
  - "statusgen/nextup.go and the other board renderers: out-of-scope (this view is a new section rendered through the existing generated-section mechanism; no existing renderer changes, which is what keeps a board regression out of scope)"
version: 1
---

# Brief 08 — External-contribution metrics

## Context

files:
- `statusgen/externalpr.go` (new) — the counter: inbound pull requests from identities outside
  the roster, bucketed by tier at open and by outcome (merged / closed / open), over a rolling
  window. Pure over an injected lister so it tests offline.
- `statusgen/externalpr_test.go` (new).
- `statusgen/testdata/` — fixtures: a mixed window, an empty window, and a window the lister
  could not read.
- `docs/contributor-trust.md` (planned) — a note that counts are published and identities are not.
- `changelog/contributor-trust-metrics.md` (new).

facts:
- The view publishes COUNTS ONLY: per tier, the number of external pull requests opened,
  merged, closed and still open in the window. No login, no identifier, no per-person row.
  This is the same boundary the tier model draws — the model is public, the rows are not.
- A tier with a count of one is still a count, but a count of one in a small window can
  identify an individual by context. The window is therefore reported with its bounds, and
  the section states that counts are not attributions.
- Three states. An empty window renders as a legitimate zero with the window bounds shown; a
  window the lister could not read renders as could-not-check and is never rendered as zero.
  The distinction must be visible in the rendered section, not only in the return value.
- The section is generated between statusgen markers, through the existing generated-section
  mechanism. It is never hand-edited, exactly like every other generated region.
- The counter reads tier at OPEN time, not tier now, so a promotion does not retroactively
  re-bucket history.

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task
  instructions only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.
- Never render an external identity into any generated output, in any fixture or golden file.
- Do not commit the generated board file; the generated board is main's own single-writer
  output.

## Task

1. `externalpr.go`: the counter over an injected lister, bucketing by tier at open and by
   outcome, with explicit empty and could-not-check results.
2. The renderer: a marker-delimited section carrying the window bounds, the per-tier counts,
   and — on the unreadable path — a could-not-check line naming what could not be read.
3. Fixtures and tests for the mixed, empty and unreadable windows, plus a test asserting no
   identity appears in rendered output for any fixture.
4. The note in the published model, and the changelog fragment.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect | Class |
|---|---------|--------|-------|
| 1 | `cd statusgen && GOWORK=off go test . -run 'ExternalPR' -count=1` | exit 0; output contains `ok` | check |
| 2 | `cd statusgen && GOWORK=off go test . -run 'ExternalPREmptyWindow' -count=1 -v` | exit 0; output contains `PASS`; output does not contain `could-not-check` | check |
| 3 | `cd statusgen && GOWORK=off go test . -run 'ExternalPRUnreadableWindow' -count=1 -v` | exit 0; output contains `could-not-check`; output does not contain `0 opened` | check +mutation |
| 4 | `cd statusgen && GOWORK=off go test . -run 'ExternalPRNoIdentityRendered' -count=1 -v` | exit 0; output contains `PASS` (no login appears in any rendered fixture) | check +mutation |
| 5 | `cd statusgen && GOWORK=off go test . -run 'ExternalPRTierAtOpen' -count=1 -v` | exit 0; output contains `PASS` (a later promotion does not re-bucket a closed item) | check |
| 6 | `cd statusgen && GOWORK=off go build . && GOWORK=off go vet .` | exit 0 | check:ci |
| 7 | `statusgen --root . --lint; echo rc=$?` | output contains `rc=0`; output contains `LINT: PASS` (the new generated section does not disturb the board lint) | check:ci +neighbour |
| 8 | `grep -n -i 'counts are not attributions' docs/contributor-trust.md` | exit 0; at least one matching line | check |
| 9 | `cd statusgen && GOWORK=off go test . -run 'ExternalPRRenderedCountsMatchFixture' -count=1 -v` | exit 0; output contains `PASS` (the test compares each number in the rendered section against the fixture it was computed from, so a renderer that prints a plausible wrong total fails) | check +dereference |
| 10 | `cd statusgen && GOWORK=off go test . -run 'ExternalPRSectionRendersThroughBoard' -count=1 -v` | exit 0; output contains `PASS` (the section is produced through the shared generated-section mechanism as the board renderer drives it, not by calling the new renderer directly) | check +flow |
| 11 | `statusgen --root . --consumers --brief contributor-trust/08` | exit 0; output does not contain `DISPROVED` | check |

Pre-mortem to detection map. "A window the lister could not read renders as a clean set of
zeroes, so a broken counter looks like a quiet month" is caught by row 3. "A login leaks into
the rendered section through a fixture or a debug field" is caught by row 4. "A promotion
re-buckets history, so the numbers move under a reader who is comparing weeks" is caught by
row 5. "The new generated section collides with the board lint" is caught by row 7. "Counts
of one in a narrow window identify an individual by context" — no row; the mitigation is the
stated window bounds and the not-an-attribution line (row 8), and whether the window is wide
enough is a review-gate judgement.

## Evidence
<!-- appended at implementation time: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner).
     "verified" status in the stream README requires this section filled
     by someone who did NOT implement. -->

## Review
Gate: model (from frontmatter). Reviewer records verdict + date in the stream README table.
