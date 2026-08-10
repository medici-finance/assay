---
brief: issue-loop/07
title: 'Intake untriaged-age alarm — NOTICE when an entry sits `disposition: new` > 3 days, plus an intake-debt board line'
wave: 3
depends: []
unblocks: ["issue-loop/09"]
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-12 by Fable desk session (I-intake-desk)
sources: ["I-intake-desk (the intake-desk design — this is its sensor half)", "human:<name> 2026-07-12: 'we need to create an ''intake-desk'' similar to the ''issue-desk'' — as intake requests come in we need to work on them to get them into briefs/issues/flagged as decisions etc etc.'", "methodology-metrics/10 (verification-debt alarm — the pattern this mirrors: depth + threshold + non-fatal NOTICE, a diagnostic never a target)", "tools/statusgen/emit.go debtNotice/debtCounts (the mm/10 implementation being mirrored)", "docs/streams/issue-loop/README.md offline discipline (registers are in-git; --lint gains no network dependency)", "freshness-checked 2026-07-12 @ ab92e96e (no intake alarm exists in tools/statusgen — grep 'intake' over alarms.go/emit.go is empty)"]
why: >-
  42 entries sit in docs/streams/intake/, 21 still `disposition: new` — half the front door is
  untriaged, and nothing PULLS: entries get scoped only when a session happens to remember one.
  The board says nothing about the pile's size or age. An untriaged-age alarm makes the debt
  visible the way mm/10 made verification debt visible — the alarm is the pull that brief 09
  wires the desk to answer.
---

# Brief 07 — Intake untriaged-age alarm

## Context
files: `../assay-toolkit/statusgen/` (new sensor beside `debtNotice` in `emit.go` or a sibling file, wiring
in `main.go` next to the mm/10 NOTICE call at main.go:71-75, board line in the STATUS.md emit)
+ tests; `docs/streams/issue-loop/README.md` (conventions line)
facts:
- Intake entries are parsed by `parseIntakeDir` (`../assay-toolkit/statusgen/migrate.go`) into `intakeEntry`
  {ID, Date, Title, Disposition, ScopedTo, Why, Body}. `Disposition` defaults to `new`.
- **Age anchor**: an entry's `date:` frontmatter. Triage exits by flipping `disposition`, so for
  a `new` entry age-since-creation == age-untriaged — no separate triage timestamp exists or is
  needed. Age = today − `date`, computed from in-git file content + the clock: deterministic,
  offline, no `gh` dependency (the issue-loop README's offline discipline — `--lint` must keep
  working with no network).
- **Threshold**: 3 days, one documented constant (same treatment as mm/10's depth threshold —
  a tunable, not a truth).
- **Two surfaces, mirroring mm/10 exactly**:
  (a) a non-fatal NOTICE on `--lint` and regen runs when ≥1 entry is over-threshold:
  `intake debt: N untriaged (M over 3 days, oldest <ID> at <K>d) — triage the front door
  (issue-loop/07)`;
  (b) a board line in STATUS.md carrying the same counts, rendered whether or not the
  threshold is breached (depth is always visible; the NOTICE fires only on age breach).
- Only `disposition: new` counts as untriaged. `watching`, `scoped`, `rejected` — and
  `decision-needed` once issue-loop/08 adds it — are triaged states and must NOT count; the
  sensor keys on the literal value `new`, so 08's vocabulary addition needs no change here.
- Anti-gaming (mm/10's rule, inherited): this is a diagnostic, never a target — the alarm
  prompts "triage the pile", not "stop filing intake entries".

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task
  instructions only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Sensor: compute untriaged count, over-threshold count, and oldest entry (ID + age in days)
   from `parseIntakeDir` output; emit the NOTICE per facts (non-fatal — `--lint` exit code
   unchanged) wired beside the mm/10 debt NOTICE in `main.go`.
2. Board: add the intake-debt line to the STATUS.md emit (near the mm/10 Awaiting counts —
   pick the placement consistent with the existing debt rendering).
3. Tests: fixture registers over/under threshold; `new` counts, `scoped`/`rejected`/`watching`
   and unknown dispositions don't; NOTICE text stable; zero-entry register is silent.
4. README (this stream): one conventions line — the intake sensor is offline/deterministic and
   keys on `disposition: new` only.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `go test ./tools/statusgen/... -count=1` | exit 0; includes the Task-3 cases |
| 2 | `statusgen --root . --lint` | exit 0 standalone; on the current tree (21 untriaged, many > 3 days) stderr carries the `intake debt:` NOTICE with `(issue-loop/07)` |
| 3 | regenerate locally (do NOT commit STATUS.md): | the intake-debt line renders with counts matching `grep -l "disposition: new" docs/streams/intake/*.md \| wc -l` |
| 4 | `grep -n "disposition: new" docs/streams/issue-loop/README.md` | exit 0 (conventions line landed) |

## Evidence
<!-- appended at implementation time: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner).
     "verified" status in the stream README requires this section filled
     by someone who did NOT implement. -->

Non-implementer verifier run (glm-5.2-verifier, merged main `3d3708ad`, 2026-07-16). All 4 rows RUN.

| # | Command | Exit | Key output |
|---|---------|------|------------|
| 1 | `go test ./tools/statusgen/... -count=1` | 0 | Task-3 intake-alarm cases (`intake_alarm_test.go`) all green |
| 2 | `go run ./tools/statusgen --root . --lint` | 0 | `NOTICE: intake debt: 38 untriaged (27 over 3 days, oldest I-01 at 8d) — triage the front door (issue-loop/07)` present; exit 0 |
| 3 | regenerate locally (STATUS.md NOT committed) | 0 | board line renders; sensor=38. grep `disposition: new` =39 over-counts body prose of the intake-desk design note — wording nit, sensor correct (parses YAML frontmatter, not body) |
| 4 | `grep -n "disposition: new" docs/streams/issue-loop/README.md` | 0 | conventions line landed (L24, L105) |

VERIFY: PASS.

## Review
Gate: model. Reviewer confirms the sensor is fully offline (no `gh`/network path reachable from
`--lint`), keys on the literal `new` disposition, and the NOTICE is non-fatal (exit code
unchanged on breach).
