---
brief: assay:assay:desk-supervision:08
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
schema: brief-v2
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
version: 1
id: eb82d786-ffc7-46d0-ae5e-98172a4b78f9
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
### Non-implementer verifier run — VERIFY: FAIL (rows 2, 5, 9 red — all three Verify-row/instrument defects where the underlying property holds; the item's Evidence section was empty at merge and the rows have never been executable as written) — verify-desk-dispatch-20260920T0246Z (verify-desk dispatch), @ merged main `e4109205`, implementation commit badd7b58c (PR #1266), 2026-09-20

Isolated worktree at origin/main, offline envelope, non-implementer.

| # | Command | Expected | Observed | Date | Runner |
|---|---------|----------|----------|------|--------|
| 1 | cd tools/desk && GOWORK=off go run ./cmd/deskdispatch --kits | exit 0; output contains worker-objective | exit 0 — stdout lists review, verifier, worker, worker-objective | 2026-09-20 | verify-desk-dispatch-20260920T0246Z |
| 2 | go run ./cmd/deskdispatch --dry-run --kit worker-objective --root . assay/desk-supervision/08 \| grep -c 'KUBECONFIG=/dev/null' | output ≥1 | **FAIL — exit 1**; deskdispatch refuses the spelling: first argument must be the item-key, not a flag (--dry-run); grep count 0. Diagnostic (corrected arg order, all else identical): exit 0, count 2 — the property holds; the row as written cannot. The item-key-first guard is dispatch.go:157, landed b8cee7341 2026-08-25, BEFORE the brief was authored (2026-09-02) — the row has never been executable | 2026-09-20 | verify-desk-dispatch-20260920T0246Z |
| 3 | awk '/<!-- common-clauses:begin -->/,/<!-- common-clauses:end -->/' worker-prompt-objective.md \| diff - common-clauses.md | exit 0 (byte-identical inclusion) | PASS — exit 0, diff empty | 2026-09-20 | verify-desk-dispatch-20260920T0246Z |
| 4 | ls tools/skillbench/fixtures/worker-kit/ \| wc -l | output 5 | PASS — 5 (01-observable-probe, 02-run-stop-signal, 03-eligibility-reconcile, 05-per-class-caps, 06-workpad-upsert) | 2026-09-20 | verify-desk-dispatch-20260920T0246Z |
| 5 | cd tools/skillbench && GOWORK=off go run . --arms ../../docs/streams/desk-supervision/08-arms \| grep -c 'check_pass_rate' | output ≥1 | **FAIL** — grep count 0. skillbench writes NOTHING to stdout by design (0 bytes on a direct run; no stdout writer in main.go, unchanged since 9e7453804 2026-08-23): it writes a markdown report file and its notice to stderr. The generated report — and the committed 08-skillbench-report.md identically — render the safety floor as prose "Task-check pass rate", never the literal check_pass_rate. The row could never pass at any commit. Property holds: fresh run exit 0, report shows Task-check pass rate 100% (n=15/15) both arms, safety floor held. (Generated report removed afterwards; worktree clean.) | 2026-09-20 | verify-desk-dispatch-20260920T0246Z |
| 6 | for a in with-overlay without-overlay; do ls docs/streams/desk-supervision/08-arms/$a \| wc -l; done | each line ≥15 | PASS — 15 and 15 | 2026-09-20 | verify-desk-dispatch-20260920T0246Z |
| 7 | grep -E -e '^decision: adopt-candidate — ' -e '^decision: reject — ' docs/streams/desk-supervision/08-report.md | exit 0; exactly one line | PASS — exactly one line: decision: adopt-candidate — check_pass_rate 100%/15 vs 100%/15 (equal), wedges 0/15 vs 0/15 (equal), wall_seconds 31.7 vs 23.7 (+33.8%, cost-side regression), diff_lines 8.3 vs 8.1 (+2.5%, cost-side regression) | 2026-09-20 | verify-desk-dispatch-20260920T0246Z |
| 8 | grep -c 'wedges' docs/streams/desk-supervision/08-report.md | output ≥1 | PASS — 7 | 2026-09-20 | verify-desk-dispatch-20260920T0246Z |
| 9 | statusgen --root . --consumers --brief desk-supervision/08 | exit 0; no DISPROVED | **FAIL — exit 2 COULD-NOT-CHECK** on merged main (the brief file landed on main 2026-09-02, so the implementing diff never contains it; the merge-base remediation is inapplicable for a squash commit). Widening the base to 30c9934a..badd7b58c — the only diff holding both the claims and the implementation — gives exit 1: 1 CORROBORATED (worker-prompt-objective.md new kit: fixed-here) and 2 DISPROVED: (a) common-clauses.md claimed out-of-scope but the item's OWN commit edits it — real, though the edit is exactly the two common-clauses marker lines that row 3's extraction requires, no guard prose changed; (b) worker-desk/SKILL.md claimed out-of-scope but changed — range artifact: changed by OTHER items' commits in the two-week window, untouched by badd7b58c. Expectation met on no execution | 2026-09-20 | verify-desk-dispatch-20260920T0246Z |

RISK-VALUE: NAMED, NOT DERIVED — task-count = 5 @ tools/skillbench/fixtures/worker-kit/ — missing: the derivation that these five tasks are representative of the states the procedural kit anticipates; the brief defers exactly that to Review-only, and its SPOF note names the task set as the one control, mitigated by two independent signals (per-task check_pass_rate + wedge count). Reversible: edit + re-run regenerates the measurement.
RISK-VALUE: DERIVED — runs-per-task-per-arm = 3 (15 per arm = 5×3) @ docs/streams/desk-supervision/08-arms/ — pinned by the brief's own pre-registered facts (≥3 runs per task per arm, decision rule stated before the runs); a minimum-variance floor for a two-arm comparison; reversible.
Enumeration notes: the decision rule and the byte-identity constraint are properties, not literals; the decision-line numbers @ docs/streams/desk-supervision/08-report.md:116 are observed data whose correctness rows 5-8 pin; HeartbeatGap = 20 minutes @ tools/desk/internal/loopengine/liveness.go is CITED by the report but pre-existing, not introduced by this diff. Nothing in the diff is irreversible; the irreversible act — flipping --kit worker's default to the objective kit — is explicitly out of scope per the brief's facts.

VERIFY: FAIL — rows 2, 5, 9 red on merged main; all three are Verify-row/instrument defects rather than property failures (row 2's spelling predates the CLI guard; row 5 expects stdout from a tool that writes only a report file; row 9 is structurally could-not-check for any brief authored on main before its implementation branch, and its only evidencing diff surfaces a real if benign routing DISPROVED on common-clauses.md — the item's commit edits a file its consumers: frontmatter claims out-of-scope, adding only the two extraction markers its own row 3 depends on). Class note: the item's Evidence section was empty at merge — nobody ran these rows as written; a brief whose Verify table is unexecutable at authoring time passes review green. Item does NOT advance. Row defects + the routing ruling are filed by the desk (see the linked issue).

### Non-implementer verifier run — VERIFY: FAIL — 6/9 pass, 1 could-not-check, 2 fail — 2026-09-23 claude-opus-4-8-verifier

Fresh classification pass on merged main 39866201ce48acdce1f9b14d1cae38eb2b7eff38. Isolated
worktree cut detached from origin/main, offline envelope (KUBECONFIG=/dev/null),
non-implementer. Result reproduces the 2026-09-20 FAIL: rows 2, 5, 9 are red as written;
all three are Verify-row/instrument defects, and every underlying deliverable property holds.

| # | Command | Expected | Observed (exit + key output) | Date | Runner |
|---|---------|----------|------------------------------|------|--------|
| 1 | cd tools/desk && GOWORK=off go run ./cmd/deskdispatch --kits | exit 0; output contains worker-objective | PASS — exit 0; stdout lists review, verifier, worker, worker-objective | 2026-09-23 | claude-opus-4-8-verifier |
| 2 | cd tools/desk && GOWORK=off go run ./cmd/deskdispatch --dry-run --kit worker-objective --root . assay/desk-supervision/08 \| grep -c 'KUBECONFIG=/dev/null' | output is 1 or more | FAIL — grep count 0. deskdispatch refuses this spelling: "first argument must be the item-key, not a flag (--dry-run)", exit status 5 (guard at tools/desk/cmd/deskdispatch/dispatch.go:162, predates the 2026-09-02 brief). Diagnostic with item-key first (all else identical): deskdispatch exit 0, grep count 2 — property holds; row as written cannot pass | 2026-09-23 | claude-opus-4-8-verifier |
| 3 | cd tools/desk/cmd/deskdispatch/references && awk '/<!-- common-clauses:begin -->/,/<!-- common-clauses:end -->/' worker-prompt-objective.md \| diff - common-clauses.md | exit 0 (byte-identical inclusion) | PASS — exit 0, diff empty | 2026-09-23 | claude-opus-4-8-verifier |
| 4 | ls tools/skillbench/fixtures/worker-kit/ \| wc -l | output is 5 | PASS — 5 (01-observable-probe, 02-run-stop-signal, 03-eligibility-reconcile, 05-per-class-caps, 06-workpad-upsert) | 2026-09-23 | claude-opus-4-8-verifier |
| 5 | cd tools/skillbench && GOWORK=off go run . --arms ../../docs/streams/desk-supervision/08-arms \| grep -c 'check_pass_rate' | output is 1 or more | FAIL — grep count 0. skillbench writes 0 bytes to stdout by design; it writes a report file (reports/skillbench/2026-09-23-08-arms.md) and its notice to stderr. The report renders the safety floor as prose "Task-check pass rate", never the literal check pass rate token the row greps for. Property holds: run exit 0, report shows Task-check pass rate 100% (n=15/15) both arms, safety floor held. Generated report removed after; worktree left clean of it | 2026-09-23 | claude-opus-4-8-verifier |
| 6 | for a in with-overlay without-overlay; do ls docs/streams/desk-supervision/08-arms/$a \| wc -l; done | each line is 15 or more | PASS — 15 and 15 | 2026-09-23 | claude-opus-4-8-verifier |
| 7 | grep -E -e '^decision: adopt-candidate — ' -e '^decision: reject — ' docs/streams/desk-supervision/08-report.md | exit 0; exactly one line | PASS — exit 0; exactly one line: decision adopt-candidate — check pass rate 100%/15 vs 100%/15 (equal), wedges 0/15 vs 0/15 (equal), wall seconds 31.7 vs 23.7 (+33.8% cost-side), diff lines 8.3 vs 8.1 (+2.5% cost-side) | 2026-09-23 | claude-opus-4-8-verifier |
| 8 | grep -c 'wedges' docs/streams/desk-supervision/08-report.md | output is 1 or more | PASS — grep count 7, exit 0 (7 is 1 or more). Note: statusgen verifyrun's literal expectation parser misreads "1 or more" as "line equals 1" and flags this row fail; that is a verifyrun instrument artifact, not a property failure — the direct grep satisfies the row | 2026-09-23 | claude-opus-4-8-verifier |
| 9 | statusgen --root . --consumers --brief desk-supervision/08 | exit 0; output does not contain DISPROVED | COULD-NOT-CHECK — exit 2 (ran with an absolute --root per the desk addendum). statusgen reports the brief is not in the diff against the merged head, so the run carries no evidence about its consumers claims — none corroborated, none disproved. Structural: the brief file has been on main since 2026-09-02 (add-stream commit), so no implementing-branch diff ever contains it. No DISPROVED observed, but the row's exit-0 expectation is not met | 2026-09-23 | claude-opus-4-8-verifier |

RISK-VALUE: NAMED, NOT DERIVED — task-count = 5 @ tools/skillbench/fixtures/worker-kit/ (five task dirs; stated at docs/streams/desk-supervision/08-report.md:6 and :9) — missing: the derivation that these five tasks are representative of the states the procedural kit anticipates and the objective kit must discover. The brief defers exactly that to Review-only, and its single-point-of-failure note names the task set as the one control, mitigated by two independent signals (per-task check pass rate + wedge count). Reversible: edit the fixture set and re-run regenerates the measurement.
RISK-VALUE: DERIVED — runs-per-task-per-arm = 3 (15 per arm = 5 tasks x 3 runs) @ docs/streams/desk-supervision/08-arms/ (stated at docs/streams/desk-supervision/08-report.md:9) — pinned by the brief's own pre-registered facts (>=3 runs per task per arm, decision rule stated before the runs); a minimum-variance floor for a two-arm comparison; reversible.
Enumeration notes: the decision rule and the byte-identity constraint are properties, not literals. The decision-line numbers at docs/streams/desk-supervision/08-report.md are observed measurement data whose correctness rows 5-8 pin, not pinned constants. HeartbeatGap = 20 minutes is cited by the report at 08-report.md:83 but is pre-existing, not introduced by this diff. Nothing in the diff is irreversible; the one irreversible act — flipping the --kit worker default to the objective kit — is explicitly out of scope per the brief's facts.

Rows 2 and 5 fail as written and row 9 cannot run post-merge; all three are the Verify-table defects tracked at #1363. Every deliverable property holds.


## Review
Gate: model (from frontmatter). Reviewer records verdict + date in the stream README table.
