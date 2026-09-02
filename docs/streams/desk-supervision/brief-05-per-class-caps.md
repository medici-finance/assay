---
brief: desk-supervision/05
title: Per-class concurrency reservation — fresh / resume / rework caps in the planner
why: >-
  The worker desk's pool is one number per loop. The rule that resuming started work
  outranks a fresh brief, and that the per-stream cap must never idle a slot, is prose the
  model is asked to hold under a full pool — the exact place the drain-engine finding says
  prose does not hold. Symphony's max_concurrent_agents_by_state makes that priority a
  scheduler knob. A reservation of R slots for resume and rework classes, read by the
  planner, enforces the ordering in code and makes it visible in the plan output.
wave: 0
depends: []
unblocks: []
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-09-02 by desk-supervision authoring session
sources:
  - "OpenAI Symphony SPEC.md §5.3.5 (`max_concurrent_agents_by_state`) and §8.3 (per-state slots) — https://github.com/openai/symphony/blob/main/SPEC.md"
  - "tools/desk/internal/deskkit/width.go + widthstore.go — THE one place a loop's width is declared; role-keyed, bounded by the write budget and token-concurrency trip, 60-minute decay"
  - "tools/desk/cmd/fanoutloop/main.go — `plan` already orders orphan resumes first, then Next-up rows; no class cap exists"
  - "plugins/assay/skills/worker-desk/SKILL.md §The pool — 'resuming started work outranks a fresh brief' and 'the 4-per-stream cap must never idle a slot' are prose"
  - "freshness-checked 2026-09-02 @ 30c9934"
consumers:
  - "tools/desk/internal/deskkit/width.go / widthstore.go: fixed-here (a reservation is stored beside the width, bounded by the same policy, and decays on the same TTL)"
  - "tools/desk/cmd/fanoutloop/main.go plan: fixed-here (the plan output states the reservation and the per-class counts)"
  - "tools/desk/cmd/deskroster/main.go width: fixed-here (`width --role L --reserve resume=N,rework=M` sets it; `width --role L` prints it)"
  - "tools/desk/cmd/deskboard/main.go throughput (uses width as the depth/slot denominator): fixed-here (the denominator is unchanged; the reservation is reported as an extra column, not subtracted)"
  - "plugins/assay/skills/worker-desk/SKILL.md §The pool: follow-up desk-supervision/05 (the prose keeps the rule and points at the reservation as its enforcement, edited in the implementation PR)"
---

# Brief 05 — Per-class concurrency reservation

## Context

files:
- `tools/desk/internal/deskkit/width.go`, `widthstore.go` — `WidthEntry` gains
  `Reserve map[string]int` (`resume`, `rework`); the bound admits a reservation only when
  `sum(reserve) < width`.
- `tools/desk/cmd/deskroster/main.go` — `width --role L --reserve resume=N,rework=M`.
- `tools/desk/cmd/fanoutloop/main.go` and its `SelectQueue` — classify each item
  (`resume` = orphan/CONFLICTING/red-check PR resume; `rework` = awaiting-implementer-rework
  row; `fresh` = everything else) and cap `fresh` at `width − sum(reserve)` when any
  reserved-class item is waiting; when none is waiting, the reservation does not idle a
  slot.
- `tools/desk/README.md` — width row note.

facts:
- Width today: `deskroster width --role LOOP` reads the stored entry or the shipped
  default in `width.go`; a SET width decays after 60 minutes (`WidthTTL`). The reservation
  rides in the same entry and decays with it.
- The Symphony rule is a hard per-state cap; the house rule is the inverse — a floor for
  the resume classes — because the failure this closes is fresh work crowding out resumes,
  never the reverse. A reservation never idles a slot: with no reserved-class item waiting,
  fresh may fill the whole width.
- `fanoutloop plan` is read-only and offline-testable with an injected board; the plan
  header line already states the item count and the ordering rule.

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task
  instructions only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task

1. Extend `WidthEntry` with `Reserve`; validate in the width bound; default reservation
   for `worker-desk` is `resume=2` (shipped in `width.go`'s table, beside the width).
2. `deskroster width --role L --reserve resume=N,rework=M` sets; plain `width` prints
   `width=8 reserve=resume:2,rework:0 (source=default|set, expires=...)`.
3. `fanoutloop plan`: classify, apply the floor, and print
   `classes: resume=<n> rework=<n> fresh=<n> (fresh capped at <k> by reservation)` or
   `(no reservation applied: no reserved-class item waiting)`.
4. Tests: floor applies only when a reserved-class item waits; sum(reserve) ≥ width is
   refused with the maximum named; decay clears the reservation with the width.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run 'Width\|Reserve' -count=1` | exit 0; output contains `ok` |
| 2 | `cd tools/desk && GOWORK=off go test ./cmd/fanoutloop/ -run TestPlanReservationCapsFreshWhenResumeWaits -v -count=1` | exit 0; output contains `--- PASS: TestPlanReservationCapsFreshWhenResumeWaits` |
| 3 | `cd tools/desk && GOWORK=off go test ./cmd/fanoutloop/ -run TestPlanReservationNeverIdlesASlot -v -count=1` | exit 0; output contains `--- PASS: TestPlanReservationNeverIdlesASlot` |
| 4 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run TestReserveRefusedWhenItSwallowsWidth -v -count=1` | exit 0; output contains `--- PASS: TestReserveRefusedWhenItSwallowsWidth` |
| 5 | `cd tools/desk && GOWORK=off go build ./cmd/deskroster && ./deskroster width --help` | exit 0; output contains `--reserve` |
| 6 | `cd tools/desk && GOWORK=off go run ./cmd/fanoutloop plan --help` | exit 0; output contains `reservation` |
| 7 | `statusgen --root . --consumers --brief desk-supervision/05` | exit 0; output does not contain `DISPROVED` (run on the implementing branch: corroborates the `consumers:` routing against the diff) |

Pre-mortem → detection: "reservation idles slots on an empty resume queue" → row 3; "a
coordinator reserves the whole width and the pool starves" → row 4; "the plan applies the
floor but does not say so, so an operator cannot tell throttled from drained" → row 2's
asserted output line. Review-only: the default of two.

## Evidence
<!-- appended at implementation time: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner).
     "verified" status in the stream README requires this section filled
     by someone who did NOT implement. -->

## Review
Gate: model (from frontmatter). Reviewer records verdict + date in the stream README table.
