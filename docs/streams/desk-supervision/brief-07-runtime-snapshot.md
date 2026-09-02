---
brief: desk-supervision/07
title: Runtime snapshot — `desksupervise status` for operators and the console
why: >-
  Desks expose nothing but filed issues. "What is running, how long has it been silent,
  which timer is closest to firing, which stops are armed" has no reader, so a stale board
  and a dead desk look the same as a quiet one. Symphony's runtime snapshot answers those
  questions per session as one structured record. The observer (brief 01) already holds
  that state each tick; rendering it as a table and a JSON document makes it the instrument
  the worker desk's suppressor sweep and the console read instead of guessing.
wave: 1
depends: ["desk-supervision/01"]
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-09-02 by desk-supervision authoring session
sources:
  - "OpenAI Symphony SPEC.md §13.3 (runtime snapshot / monitoring interface), §13.5 (session metrics and token accounting), §13.7 (optional HTTP server) — https://github.com/openai/symphony/blob/main/SPEC.md"
  - "tools/desk/cmd/opmetrics — the house's aggregates-only, three-state, could-not-check-never-zero collector; the snapshot follows the same discipline"
  - "plugins/assay/skills/worker-desk/SKILL.md §Sources of work row 9 — queue suppressors are read today by `git ls-remote origin 'refs/dispatch/*'` plus the claim helper's list/show verbs"
  - "docs/three-state-instrument-rule.md — the rule every instrument here follows"
  - "desk-supervision/02 — deferred the pr-review-desk and verify-desk cadence reads of armed stops to this brief; they land here"
  - "freshness-checked 2026-09-02 @ 30c9934"
consumers:
  - "tools/desk/cmd/desksupervise/main.go: fixed-here (`status` renders the observer's per-claim state)"
  - "schemas/desksupervise-status-v1.json: fixed-here (the JSON shape is a published schema so a console can consume it without reading Go)"
  - "plugins/assay/skills/worker-desk/SKILL.md §Sources of work row 9 and §HARD GATE: fixed-here (row 9's instrument becomes `desksupervise status`; the raw ls-remote stays as the fallback)"
  - "plugins/assay/skills/pr-review-desk/SKILL.md and verify-desk/SKILL.md: fixed-here (each cadence sweep gains the same one-line read, so all three windows see armed stops — closes the follow-up brief 02 deferred)"
  - "an operator console rendering the JSON: out-of-scope (a private consumer; the schema is the contract it reads)"
---

# Brief 07 — Runtime snapshot

## Context

files:
- `tools/desk/cmd/desksupervise/main.go` (planned) — `status [--json] [--stops] [--repo O/N]`.
- `tools/desk/cmd/desksupervise/status.go` (new) — the snapshot type, table renderer,
  JSON renderer.
- `schemas/desksupervise-status-v1.json` (new) — JSON schema (the `schemas/` directory
  already holds published schemas).
- `tools/desk/cmd/desksupervise/testdata/` — snapshot fixtures.
- Skill bodies: the three cadence sections (one line each).

facts:
- Per in-flight claim the snapshot carries: `key`, `repo`, `item`, `state`
  (`claimed|dispatched`), `holder`, `claimed_at`, `dispatched_at`, `last_observed_at`,
  `observed_via`, `liveness` (`ALIVE|NEVER-STARTED|HEARTBEAT-EXPIRED|OVER-WALL-CAP|
  COULD-NOT-CHECK`), `timers` (`schedule_to_start_remaining`, `heartbeat_remaining`,
  `wall_cap_remaining`, each a duration or `n/a`), `stop` (`armed_at`, `reason`, or null),
  `tokens` (`could-not-check` — see next fact).
- **Token accounting is could-not-check by design** at this layer: a dispatched worker's
  usage is held by the harness, not by desk tools, and no read path exists. The field is
  present so a future harness binding can fill it; it is never rendered as zero. The
  three-state rule applies to every field: an unreadable source renders
  `could-not-check`, never a default.
- Aggregates: counts per liveness class, per state, armed stops, and `blind_sources`
  (the list of probe sources that could not be read this tick).
- `--json` output validates against the schema; the table is for humans and is not a
  contract.
- No HTTP server (Symphony §13.7 is optional and this house's service layer is elsewhere);
  `run --interval` may additionally write the JSON to `<StateDir>/supervise/status.json`
  atomically each tick for a local reader.

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task
  instructions only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task

1. `status.go`: build the snapshot from one observer evaluation (reuse `tick`'s
   classification without acting); table + JSON renderers; `--stops` filters to armed stops.
2. Schema file; a test validates the fixture output against it.
3. `run --interval` writes `status.json` atomically (temp + rename) each tick.
4. Skill edits: worker-desk row 9 instrument; pr-review-desk and verify-desk cadence
   lines; capability vocabulary only.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `cd tools/desk && GOWORK=off go test ./cmd/desksupervise/ -run 'Status\|Snapshot' -count=1` | exit 0; output contains `ok` |
| 2 | `cd tools/desk && GOWORK=off go build ./cmd/desksupervise && ./desksupervise status --json --now 2026-09-02T12:00:00Z --claims-fixture cmd/desksupervise/testdata/mixed.json --observations-fixture cmd/desksupervise/testdata/mixed-obs.json \| python3 -c 'import json,sys; d=json.load(sys.stdin); print(d["schema"], len(d["claims"]), d["claims"][0]["tokens"])'` | exit 0; output is `desksupervise-status-v1 3 could-not-check` |
| 3 | `cd tools/desk && GOWORK=off go test ./cmd/desksupervise/ -run TestStatusJSONValidatesAgainstSchema -v -count=1` | exit 0; output contains `--- PASS: TestStatusJSONValidatesAgainstSchema` |
| 4 | `cd tools/desk && ./desksupervise status --now 2026-09-02T12:00:00Z --claims-fixture cmd/desksupervise/testdata/mixed.json --observations-fixture cmd/desksupervise/testdata/blind-obs.json; echo rc=$?` | output contains `blind_sources` or `BLIND`, contains `could-not-check`, does not contain ` 0s` as a remaining timer for the blind claim, and contains `rc=6` |
| 5 | `cd tools/desk && ./desksupervise status --stops --now 2026-09-02T12:00:00Z --claims-fixture cmd/desksupervise/testdata/mixed.json --observations-fixture cmd/desksupervise/testdata/mixed-obs.json --stops-fixture cmd/desksupervise/testdata/stops.json` | exit 0; output contains exactly the keys in `stops.json` and no other claim key |
| 6 | `test -f schemas/desksupervise-status-v1.json && grep -c '"tokens"' schemas/desksupervise-status-v1.json` | output is `1` or more |
| 7 | `grep -l 'desksupervise status' plugins/assay/skills/worker-desk/SKILL.md plugins/assay/skills/pr-review-desk/SKILL.md plugins/assay/skills/verify-desk/SKILL.md \| wc -l` | output is `3` |
| 8 | `statusgen --root . --consumers --brief desk-supervision/07` | exit 0; output does not contain `DISPROVED` (run on the implementing branch: corroborates the `consumers:` routing against the diff) |

Pre-mortem → detection: "tokens render as 0 and someone reads it as free" → row 2; "a blind
tick prints a clean-looking table with exit 0" → row 4; "the JSON drifts from the schema
the console reads" → row 3; "only the worker desk reads stops" → row 7.

## Evidence
<!-- appended at implementation time: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner).
     "verified" status in the stream README requires this section filled
     by someone who did NOT implement. -->

## Review
Gate: model (from frontmatter). Reviewer records verdict + date in the stream README table.
