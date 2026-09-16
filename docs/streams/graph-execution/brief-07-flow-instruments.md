---
brief: assay:assay:graph-execution:07
title: Flow instruments — service/wait split, CI-slot saturation, gate catch/override
why: >-
  Today's bottleneck reading is WIP multiplied by how long an item has sat in its current
  stage. That number cannot tell "the verifier is busy" from "the item waited for a human",
  so every capacity decision made from it is a guess. Before anyone widens dispatch or adds
  an admission cap, the fleet needs the four durations that separate scheduling delay from
  service time, plus the two numbers that distinguish throughput from churn: how saturated
  the CI slots are, and whether gates are catching defects or being waived.
wave: 1
depends: ["graph-execution/01"]
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
authored: 2026-09-16 by graph-execution authoring session (fable-5.1, author-brief)
sources:
  - "docs/streams/graph-execution/spec.md §2.6 (the seven durations, CI-slot saturation, gate catch/override, environment identity on every comparison) and §3 (stage-age heuristic row)"
  - "graph-execution/01 — `statusgen --eligibility` and the eligible-at instant this brief measures FROM"
  - "statusgen/history.go (the historian: ts is the REGEN run that observed the change, not the event), statusgen/briefefficiency.go (--flow-efficiency: the existing touch/wait PROXY from historian dwell, gtSmallN thin-data rule), statusgen/gatetelemetry.go (--gate-telemetry: override-rate / catch-rate / ceremonial-gate families, exit-code contract 0/1/2/3), statusgen/doratiming.go (.dora-timing.jsonl: PR commit→merge lead time and restore episodes, single-writer), statusgen/bottleneck.go (WIP × stage-age, NOT replaced here)"
  - "Rajamohan, trajectory consistency vs aggregate task performance, https://youtu.be/BIBDhLDgMdE?t=434 (measure valid outcomes and constraints, not obedience to one sequence)"
  - "freshness-checked 2026-09-16 @ d96fd3ba: `grep -rn 'ci_slot\\|ci-slot\\|slot saturation' statusgen/*.go docs/*.md` returns zero hits; `--flow-efficiency` and `--gate-telemetry` exist (statusgen/main.go) and are REUSED, not re-implemented"
exec-tier: strong
exec-tier-why: "(b) correctness depends on joining three independent logs (.history.jsonl, .dora-timing.jsonl, forge checks/reviews) on one brief identity without fabricating an interval; (a) which observed instant stands in for each unrecorded event is a design decision the facts do not pre-specify"
domain: complicated
consumers:
  - "docs/iso9001-mapping.md (the row that names --flow-efficiency / --gate-telemetry as monitoring instruments): follow-up graph-execution/07 (this brief; flips to fixed-here when the implementation adds the --flow row)"
  - "statusgen/bottleneck.go (the stage-age score): out-of-scope (deliberately NOT replaced — this brief joins it with a second reading; replacing the score is a later decision from data this brief produces)"
version: 1
id: 76e53f95-97ce-4244-9a45-e88cb56244ec
---

# Brief 07 — Flow instruments

## Context
files: `statusgen/flow.go` (planned), `statusgen/flow_test.go` (planned), `statusgen/testdata/flow/` (planned) — fixture historian + timing + checks JSON, `example-org/*` names only), `statusgen/main.go` (the `--flow` flag and its `--json` / `--since` / `--until` reuse), `statusgen/gatetelemetry.go` (read-only; its report is CONSUMED for the catch/override families), `statusgen/briefefficiency.go` (read-only; its completed-dwell walker is REUSED), `docs/iso9001-mapping.md` (the monitoring-instrument row gains `--flow`), `docs/dependency-graph-design.md` (one paragraph: which instants the lifecycle graph now records), `changelog/graph-execution-07-flow-instruments.md` (planned)

facts:
- **What timestamps exist today (2026-09-16, this repo's main).** `.history.jsonl` records `ts` as the regen run that OBSERVED a status change, never the event itself (`statusgen/history.go` header). `.dora-timing.jsonl` records per-PR commit→merge lead time and restore episodes, single-writer from main's regen (`statusgen/doratiming.go`). There is NO recorded work-start event and NO recorded eligible-at instant (`statusgen/briefefficiency.go` lines 3–11 say so and ship a proxy). Brief 01 adds the eligible-at instant to the evaluator's output; this brief is the first consumer of it.
- **Four durations, and the instant each is measured between.** eligible-to-start delay = 01's eligible-at → the historian's first `in-progress` observation; active work time = `in-progress` dwell (the existing --flow-efficiency touch proxy, reused verbatim); external wait = `implemented` dwell that overlaps an open human-gate issue or a `blocked` row, else counted as verification wait; verification time = `implemented` → `verified` dwell. Every interval is counted only when COMPLETE (a later record closes it), the same no-fabricated-interval discipline `briefefficiency.go` and `doratiming.go` already keep.
- **Observation, not event: say so in the output.** Because `ts` is a regen instant, every duration carries a `resolution` field = the median regen cadence observed in the window; a duration shorter than one cadence is reported as `< resolution`, never as a number.
- **CI-slot saturation** = (merged PRs per day × median CI wall-clock per PR) ÷ available CI hours per day. The first two terms come from forge check-run timestamps read through `gh` (three-state: no token or no `checks: read` ⇒ `could-not-check`); the divisor is a declared value in the report's input (`--ci-hours-per-day`, no default — absent ⇒ `could-not-check`, never an invented number).
- **Gate catch/override** = the `--gate-telemetry` override-rate and catch-rate families, CONSUMED from `gatetelemetry.go`'s report over the same window, re-emitted under `--flow` with their existing three-state semantics and exit codes (0 / 1 malformed / 3 could-not-check). Nothing about a gate is re-measured here.
- **Environment identity on every number:** `statusgen_version` (the build's own version string, as `docs/telemetry.md` already defines it), the window bounds, the historian's regen cadence, and `model_versions: could-not-check` unless a run-record source (a later brief) supplies them. A number without an environment stamp is a lint PROBLEM in the fixture test.
- **The stage-age score is not replaced.** `--bottleneck` keeps its current output; `--flow` is a second reading beside it. A reader that wants one number is told there is not one.
- Single-point-of-failure note: the ONE control is the fixture test that asserts every emitted duration is closed by a later record. Second, independent layer: the `resolution` field, computed in a different function from a different input (regen cadence), makes a sub-cadence number visibly unreliable even if the closing check regresses. Third, out-of-band: `--gate-telemetry`'s own three-state exit code propagates, so an unread source cannot read as zero.

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task instructions only.
- Public tree: fixtures and tests use `example-org/*` names; no adopter's numbers appear anywhere.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. `statusgen --flow [--json] [--since --until] [--ci-hours-per-day N]`: per brief, the four durations from the facts, each with `status: measured | < resolution | could-not-check` and the instants it was measured between; a `resolution` field per window; fleet-level medians only over `measured` rows and only when the count ≥ `gtSmallN` (reuse the constant).
2. `ci_slot_saturation` and `gate_catch_override` blocks per the facts, three-state, with the source each read (`gh` checks, `gatetelemetry` report) or the reason it could not.
3. `environment` block on every report; the fixture test fails the report if it is absent or any field is empty rather than `could-not-check`.
4. Docs: the `docs/iso9001-mapping.md` instrument row; one paragraph in `docs/dependency-graph-design.md` naming the instants the lifecycle graph records after 01 + this brief, and the ones it still does not (a true work-start event).
5. Changelog fragment.

## Verify
| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check:ci | cd statusgen && go test -run 'TestFlow' ./... | exit 0 |
| 2 | check:ci +mutation | cd statusgen && go test -run 'TestFlowRefusesOpenInterval' ./... | exit 0; the fixture holds a brief whose current stage has no closing record and the report emits `could-not-check` for it, never a duration |
| 3 | check +flow +dereference | cd statusgen && go run . --root testdata/flow/window-a --flow --json | jq -e '.environment.statusgen_version != "" and .resolution.seconds > 0 and (.briefs[] | select(.id=="example-app/02") | .eligible_to_start.status=="measured")' | exit 0 — the numbers come from the fixture's records, not from the report's own defaults |
| 4 | check +neighbour | cd statusgen && go run . --root testdata/flow/window-a --bottleneck | grep -c 'constraint' | count >= 1 — the existing stage-age report still renders unchanged beside the new one |
| 5 | check | cd statusgen && env -u GH_TOKEN -u GITHUB_TOKEN go run . --root testdata/flow/window-a --flow --json | jq -r '.ci_slot_saturation.status' | prints `could-not-check` (no token ⇒ the forge-derived term is unread, never zero) |
| 6 | check | cd statusgen && go run . --root testdata/flow/window-a --flow --json; echo rc=$? | tail -1 | `rc=3` when any source is could-not-check, `rc=0` only when every source was read — the gate-telemetry exit contract carried through |
| 7 | check | statusgen --root . --consumers --brief assay:assay:graph-execution:07 --diff-base origin/main; echo rc=$? | tail -1 | `rc=0` on the implementing branch — the iso9001 row flipped to fixed-here is corroborated by the diff, the bottleneck out-of-scope entry is listed for the reviewer |
| 8 | check | statusgen --root . --lint; echo rc=$? | tail -1 | `rc=0` |

## Evidence
<!-- appended at implementation time: one row per Verify item — (command, exit code, output line(s) or hash, date, runner). "verified" requires this section filled by someone who did NOT implement. -->

## Review
Gate: model (from frontmatter). Reviewer records verdict + date in the stream README table.
