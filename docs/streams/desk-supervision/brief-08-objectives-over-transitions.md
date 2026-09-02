---
brief: desk-supervision/08
title: Objectives over transitions — measure an objective-style worker kit with skillbench
why: >-
  The worker prompt kit and skill body are long and procedural, and workers wedge on states
  the procedure did not anticipate. OpenAI's stated lesson from Symphony is that treating
  the agent as a node in a state machine was too limiting: hand it an objective, the tools,
  a status map, and let it route. That may or may not hold under this house's guards — so
  this brief does not adopt it; it builds the alternative kit and measures it two-arm with
  the house's own AI-free reducer, with the safety floor and the wedge rate as the decision
  rule. Adoption, if any, is a follow-up brief citing the report.
wave: 1
depends: ["desk-supervision/06"]
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-09-02 by desk-supervision authoring session
sources:
  - "OpenAI Symphony post, 'Progress comes with new, different problems' — 'treating agents as rigid nodes in a state machine doesn't work well … we eventually moved toward giving agents objectives instead of strict transitions' — https://openai.com/index/open-source-codex-orchestration-symphony/"
  - "OpenAI Symphony example WORKFLOW.md — the objective + status-map prompt shape this kit adapts (default posture, status map, blocked-access escape hatch, PR feedback sweep) — https://github.com/openai/symphony/blob/main/elixir/WORKFLOW.md"
  - "tools/desk/cmd/deskdispatch/references/{common-clauses,worker-prompt}.md — the current kit; `deskdispatch --kits` lists kits; `--kit` selects one"
  - "tools/skillbench/README.md — the two-arm, AI-free reducer: diff_lines, files_touched, tokens, cost, wall_seconds, check_pass_rate (the safety floor); could-not-check never becomes a value"
  - "freshness-checked 2026-09-02 @ 30c9934 — one worker kit exists; no measurement of it exists"
exec-tier: strong
exec-tier-why: >-
  (a) and (c): the kit's wording is a design decision the facts do not pre-specify, and the
  kit is guardrail-adjacent — a rewrite that drops an invariant while every lint still
  passes is the failure to design against.
consumers:
  - "tools/desk/cmd/deskdispatch/references/worker-prompt-objective.md (new kit): fixed-here"
  - "tools/desk/cmd/deskdispatch/references/common-clauses.md: out-of-scope (the objective kit includes the common clauses byte-identical; the guards are not the variable under test)"
  - "plugins/assay/skills/worker-desk/SKILL.md: out-of-scope (no skill-body change in this brief; adoption is a separate brief citing the report)"
---

# Brief 08 — Objectives over transitions, measured

## Context

files:
- `tools/desk/cmd/deskdispatch/references/worker-prompt-objective.md` (new) — the
  alternative kit: objective, tools available, the status map (draft PR → review → rework
  → ready is the review desk's; the worker's states are `working`, `blocked`,
  `handed-off`), the workpad rule, the blocked-access escape hatch, and the common clauses
  included verbatim.
- `tools/desk/cmd/deskdispatch/main.go` — register the kit under `--kit worker-objective`.
- `tools/skillbench/fixtures/worker-kit/` (new) — the task set: five briefs from this
  repo's own closed streams, re-runnable offline, each with a deterministic `check`.
- `docs/streams/desk-supervision/08-report.md` (new) — the skillbench report plus the
  wedge count per arm and the decision line.

single-point-of-failure: the task set — if the five tasks do not exercise a state the
procedural kit anticipates and the objective kit must discover, the comparison measures
nothing. Behind it: the safety floor (`check_pass_rate`) is per task and the wedge count is
read from brief 01's observer log, two independent signals.

facts:
- Arms: `with-overlay` = objective kit, `without-overlay` = current `worker` kit; same
  tasks, same model tier, ≥3 runs per task per arm; artifacts per run exactly as
  `tools/skillbench/README.md` specifies (`diff.patch`, `run.json`, optional `usage.json`).
- Decision rule (stated before the runs): adopt-candidate only if `check_pass_rate` is not
  lower AND wedges (runs with no observable event for longer than the heartbeat gap, per
  the observer log) are not higher; otherwise `reject`, with the numbers. Either outcome is
  a complete deliverable.
- The common clauses are the constant: the objective kit must contain
  `common-clauses.md` byte-identical (a `diff` proves it), so the variable is the body
  shape only.
- The kit is never the default in this brief; `--kit worker` stays the dispatcher's
  default.

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task
  instructions only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task

1. Author the objective kit; register it; `deskdispatch --kits` lists it; `--dry-run
   --kit worker-objective` emits it with the common clauses embedded.
2. Build the five-task fixture set with deterministic checks.
3. Run both arms (≥3 runs each per task); lay out the artifacts; run
   `skillbench --arms <dir>`; record wedges per arm from the observer log.
4. Write `08-report.md`: the skillbench table, the wedge counts, the decision line
   `decision: adopt-candidate|reject — <numbers>`, and what a follow-up adoption brief
   would change.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `cd tools/desk && GOWORK=off go run ./cmd/deskdispatch --kits` | exit 0; output contains `worker-objective` |
| 2 | `cd tools/desk && GOWORK=off go run ./cmd/deskdispatch --dry-run --kit worker-objective --root . assay/desk-supervision/08 \| grep -c 'KUBECONFIG=/dev/null'` | output is `1` or more (common clauses present) |
| 3 | `cd tools/desk/cmd/deskdispatch/references && awk '/<!-- common-clauses:begin -->/,/<!-- common-clauses:end -->/' worker-prompt-objective.md \| diff - common-clauses.md` | exit 0 (byte-identical inclusion) |
| 4 | `ls tools/skillbench/fixtures/worker-kit/ \| wc -l` | output is `5` |
| 5 | `cd tools/skillbench && GOWORK=off go run . --arms ../../docs/streams/desk-supervision/08-arms \| grep -c 'check_pass_rate'` | output is `1` or more |
| 6 | `for a in with-overlay without-overlay; do ls docs/streams/desk-supervision/08-arms/$a \| wc -l; done` | each line is `15` or more (5 tasks × ≥3 runs) |
| 7 | `grep -E -e '^decision: adopt-candidate — ' -e '^decision: reject — ' docs/streams/desk-supervision/08-report.md` | exit 0; exactly one line |
| 8 | `grep -c 'wedges' docs/streams/desk-supervision/08-report.md` | output is `1` or more |
| 9 | `statusgen --root . --consumers --brief desk-supervision/08` | exit 0; output does not contain `DISPROVED` (run on the implementing branch: corroborates the `consumers:` routing against the diff) |

Pre-mortem → detection: "the objective kit quietly drops a guard" → row 3; "the report
declares a winner from one run" → row 6; "the decision is prose with no numbers" → row 7;
"wedge rate is asserted, not read from the observer" → row 8 plus review of the cited log
lines. Review-only: whether the five tasks are representative.

## Evidence
<!-- appended at implementation time: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner).
     "verified" status in the stream README requires this section filled
     by someone who did NOT implement. -->

## Review
Gate: model (from frontmatter). Reviewer records verdict + date in the stream README table.
