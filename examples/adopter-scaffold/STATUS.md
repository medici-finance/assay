<!-- GENERATED FILE — do not edit. Source of truth: docs/streams/*/README.md.
     Regenerate: go run ./tools/statusgen -->

# Project Status

## Roll-up

### Product

| Stream | Priority | Status | Briefs done | Last touched | Notes |
|---|---|---|---|---|---|
| [example-service](docs/streams/example-service/README.md) | P1 | active | 0/2 | 2026-08-11 | implement=cheap verify=strong |

## Next up

| Stream | Brief | Wave | Score |
|---|---|---|---|
| example-service | 01 — Signal/Quality observability model | 0 | 2000 |
| example-service | 02 — Prod half-probe honesty | 1 | 2000 |

## Intake queue

_0 untriaged entries — the front door is clear._

## Awaiting verification / review (0 desk-actionable of 0 total — 0 at implemented, 0 verified awaiting review)

_Gate-queue ordered by score: priorityWeight + staleness×stalenessPerDay + valueWeight + unblocksWeight×blockedCount. The weights are an evolving heuristic (F-09 discipline) — not a claim of truth. Board segmented by blocker owner: the desk-actionable headline counts only the queue the desk can actually drain._

_`done‡` / `verified‡` = closed over an **UNRUN risk-bearing Verify row**: a live/mutating check with no completed Evidence row behind it. UNRUN is DERIVED from Verify-vs-Evidence coverage — a row counts as run only when an Evidence row names it with a date and a runner, so silence reads as unrun. `--lint` names each one and whether it was routed to a follow-up._

_None._

## Unresolved findings

_None._

## Incomplete briefs

### example-service (2 open)

- 01 Signal/Quality observability model — todo (wave 0)
- 02 Prod half-probe honesty — todo (wave 1)

## Done briefs

_`done*` = unbacked (I-08 point quality): the row's Evidence section is empty and/or its Verified/Reviewed cells aren't dated+attributed per brief-16 — see `--lint` for the full list. Plain `done` is evidence-backed._

_`done‡` / `verified‡` = closed over an **UNRUN risk-bearing Verify row**: a live/mutating check with no completed Evidence row behind it. UNRUN is DERIVED from Verify-vs-Evidence coverage — a row counts as run only when an Evidence row names it with a date and a runner, so silence reads as unrun. `--lint` names each one and whether it was routed to a follow-up._

## Totals

**1** streams (**1** active, **0** paused) · **0/2** briefs done · completed initiatives: see `docs/archive/`
