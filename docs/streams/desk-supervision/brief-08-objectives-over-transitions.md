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
delivery:
  - {in: "assay#1266", covers: partial, note: "kit, fixtures and both arms' run artifacts only; the arms carry no observer log, so Task 3's wedge counts and Task 4's decision line remain owed (tracked in assay#2816)"}
  - {in: "assay#3239", covers: partial, note: "repairs the Verify commands and withdraws the unsupported wedge counts and adoption conclusion; Task 3 and Task 4 remain owed until observer-backed runs exist"}
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
  - "tools/desk/cmd/deskdispatch/references/common-clauses.md: fixed-here (extraction markers only; common-clause guard prose is unchanged)"
  - "plugins/assay/skills/worker-desk/SKILL.md: out-of-scope (no skill-body change in this brief; adoption is a separate brief citing the report)"
version: 1
id: eb82d786-ffc7-46d0-ae5e-98172a4b78f9
---

# Brief 08 — Objectives over transitions, measured

## Context

files:
- `tools/desk/cmd/deskdispatch/references/worker-prompt-objective.md` (new) — the alternative kit: objective, tools available, the status map (draft PR → review → rework → ready is the review desk's; the worker's states are `working`, `blocked`, `handed-off`), the workpad rule, the blocked-access escape hatch, and the common clauses included verbatim.
- `tools/desk/cmd/deskdispatch/main.go` — register the kit under `--kit worker-objective`.
- `tools/skillbench/fixtures/worker-kit/` (new) — the task set: five briefs from this repo's own closed streams, re-runnable offline, each with a deterministic `check`.
- `docs/streams/desk-supervision/08-report.md` — the stored skillbench reduction and the observer-dependent decision, with missing evidence reported as could-not-check.
- `docs/streams/desk-supervision/verify.d/08-reduction.sh` — rerun the reducer with a scratch output path and check its report-file oracle.
- `docs/streams/desk-supervision/verify.d/08-consumers.py` — corroborate all three consumer routes against the exact historical delivery diff.
- `docs/streams/desk-supervision/verify.d/08-mutations.py` — plant a failed task result and an additional consumer in scratch; require each oracle to reject it.
- `docs/streams/desktools-v2/cli-migration.json` — classify these three source-only acceptance instruments as tests, using exact-path entries.

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
| # | Command | Expect | Class |
|---|---------|--------|-------|
| 1 | `cd tools/desk && GOWORK=off go run ./cmd/deskdispatch --kits` | exit 0; output contains `worker-objective` | check |
| 2 | `bash -o pipefail -c 'cd tools/desk && GOWORK=off GOPROXY=off go run ./cmd/deskdispatch assay/desk-supervision/08 --dry-run --kit worker-objective --root ../.. \| grep -F "KUBECONFIG=/dev/null" >/dev/null'` | exit 0 (common clauses present; producer failure is retained) | check |
| 3 | `cd tools/desk/cmd/deskdispatch/references && awk '/<!-- common-clauses:begin -->/,/<!-- common-clauses:end -->/' worker-prompt-objective.md \| diff - common-clauses.md` | exit 0 (byte-identical inclusion) | check |
| 4 | `ls tools/skillbench/fixtures/worker-kit/ \| wc -l` | output is `5` | check |
| 5 | `bash docs/streams/desk-supervision/verify.d/08-reduction.sh` | exit 0; output contains `Safety floor (task-check pass rate): held` | check |
| 6 | `for a in with-overlay without-overlay; do ls docs/streams/desk-supervision/08-arms/$a \| wc -l; done` | each line is `15` or more (5 tasks × ≥3 runs) | check |
| 7 | `grep -E -e '^decision: adopt-candidate — ' -e '^decision: reject — ' docs/streams/desk-supervision/08-report.md` | exit 0; exactly one line | check |
| 8 | `grep -c 'wedges' docs/streams/desk-supervision/08-report.md` | output is `1` or more | check |
| 9 | `python3 docs/streams/desk-supervision/verify.d/08-consumers.py` | exit 0; output contains `CORROBORATED: 3 consumer routes` | check |
| 10 | `python3 docs/streams/desk-supervision/verify.d/08-mutations.py` | exit 0; output contains `2 planted defects rejected` | check +mutation |

Pre-mortem → detection: "the objective kit quietly drops a guard" → row 3; "the report
declares a winner from one run" → row 6; "the decision is prose with no numbers" → row 7;
"wedge rate is asserted, not read from the observer" → row 8 plus review of the cited log
lines. Review-only: whether the five tasks are representative.

## Rework boundary — 2026-10-17

The kit, fixtures and 30 stored runs were delivered by #1266. This rework repairs the
command/oracle defects in rows 2, 5 and 9 tracked in #1363; it does not repeat that
comparison. Row 9 reads only delivery `badd7b58c` against its first parent, so later
skill edits cannot be misattributed to this item. Its consumer route now records the
actual marker-only common-clause change; it never permits a guard-prose change.

The rework CLI classifications are read by `tools/desk/internal/clicontract`;
`TestCLIInventory` and `TestCLIRoutingMutations` validate their exact paths and
retain the independent discovery, stale-rule and shipped-entrypoint checks. These
classifications do not change the original delivery's three historical kit consumers.

The later finding #2816 remains an acceptance blocker: the stored artifacts include
no observer log, so duration records cannot establish wedges. The report now states
could-not-check for both wedges and the adoption decision. **Row 7 remains red until
the original observer-based decision rule can actually be evaluated.** A row 8 word
count only locates the discussion; it does not prove observer coverage or satisfy
Task 3. No revised offline criterion or new comparison is claimed here. The separate
row 3 execution-witness parser issue remains tracked in #2679; the byte comparison
still applies unchanged. Historical verifier receipts below remain historical.

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
| 9 | statusgen --root . --consumers --brief desk-supervision/08 | exit 0; output does not contain DISPROVED | COULD-NOT-CHECK — exit 2 (run from the repo root with the relative `--root .` shown). statusgen reports the brief is not in the diff against the merged head, so the run carries no evidence about its consumers claims — none corroborated, none disproved. Structural: the brief file has been on main since 2026-09-02 (add-stream commit), so no implementing-branch diff ever contains it. No DISPROVED observed, but the row's exit-0 expectation is not met | 2026-09-23 | claude-opus-4-8-verifier |

RISK-VALUE: NAMED, NOT DERIVED — task-count = 5 @ tools/skillbench/fixtures/worker-kit/ (five task dirs; stated at docs/streams/desk-supervision/08-report.md:6 and :9) — missing: the derivation that these five tasks are representative of the states the procedural kit anticipates and the objective kit must discover. The brief defers exactly that to Review-only, and its single-point-of-failure note names the task set as the one control, mitigated by two independent signals (per-task check pass rate + wedge count). Reversible: edit the fixture set and re-run regenerates the measurement.
RISK-VALUE: DERIVED — runs-per-task-per-arm = 3 (15 per arm = 5 tasks x 3 runs) @ docs/streams/desk-supervision/08-arms/ (stated at docs/streams/desk-supervision/08-report.md:9) — pinned by the brief's own pre-registered facts (>=3 runs per task per arm, decision rule stated before the runs); a minimum-variance floor for a two-arm comparison; reversible.
Enumeration notes: the decision rule and the byte-identity constraint are properties, not literals. The decision-line numbers at docs/streams/desk-supervision/08-report.md are observed measurement data whose correctness rows 5-8 pin, not pinned constants. HeartbeatGap = 20 minutes is cited by the report at 08-report.md:83 but is pre-existing, not introduced by this diff. Nothing in the diff is irreversible; the one irreversible act — flipping the --kit worker default to the objective kit — is explicitly out of scope per the brief's facts.

Rows 2 and 5 fail as written and row 9 cannot run post-merge; all three are the Verify-table defects tracked at #1363. Every deliverable property holds.

### Verification — 2026-09-25 (assay-verifier-app[bot] @ 7aa3835d7f33 (claude-opus-5-5) (on-behalf-of human:ian))

Re-verify on merged main 7aa3835d7f3323b04f7f4f69aafc82a8afefc37e after the 2026-09-23 verify-fail receipt (blocker_kind check-definition, #1363). Inputs changed since that receipt (deskdispatch sources, statusgen), but the Verify table itself is unchanged and #1363 is still open. Isolated detached worktree cut from origin/main, offline envelope (KUBECONFIG=/dev/null), non-implementer. Every row ran under a throwaway HOME so no test or tool could append to the live desk audit log; the house roster file (read-only config) was copied into that HOME, because without it deskdispatch refuses with "no roster configured" (exit 6) and row 1 fails for a reason that belongs to the environment rather than the item. Row 9 resolved statusgen from a PATH entry that points at a binary built from this tree. No check:ci rows in the table.

Execution witness (statusgen verifyrun --dry-run, built from this tree, run from the repo root):

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/desk && GOWORK=off go run ./cmd/deskdispatch --kits` | pass exit=0 | sha256:1a33205685e7 | 2026-09-25 | assay-verifier-app[bot] @ 7aa3835d7f33 (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd tools/desk && GOWORK=off go run ./cmd/deskdispatch --dry-run --kit worker-objective --root . assay/desk-supervision/08 \| grep -c 'KUBECONFIG=/dev/null'` | fail exit=1 | sha256:d82189b1b9ca | 2026-09-25 | assay-verifier-app[bot] @ 7aa3835d7f33 (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd tools/desk/cmd/deskdispatch/references && awk '/<!-- common-clauses:begin -->/,/<!-- common-clauses:end -->/' worker-prompt-objective.md \| diff - common-clauses.md` | pass exit=0 | sha256:e3b0c44298fc | 2026-09-25 | assay-verifier-app[bot] @ 7aa3835d7f33 (on-behalf-of human:ian) (forge-identity) |
| 4 | `ls tools/skillbench/fixtures/worker-kit/ \| wc -l` | pass exit=0 | sha256:30179a803d48 | 2026-09-25 | assay-verifier-app[bot] @ 7aa3835d7f33 (on-behalf-of human:ian) (forge-identity) |
| 5 | `cd tools/skillbench && GOWORK=off go run . --arms ../../docs/streams/desk-supervision/08-arms \| grep -c 'check_pass_rate'` | fail exit=1 | sha256:5f845879cfac | 2026-09-25 | assay-verifier-app[bot] @ 7aa3835d7f33 (on-behalf-of human:ian) (forge-identity) |
| 6 | `for a in with-overlay without-overlay; do ls docs/streams/desk-supervision/08-arms/$a \| wc -l; done` | pass exit=0 | sha256:9be9b418f3b8 | 2026-09-25 | assay-verifier-app[bot] @ 7aa3835d7f33 (on-behalf-of human:ian) (forge-identity) |
| 7 | `grep -E -e '^decision: adopt-candidate — ' -e '^decision: reject — ' docs/streams/desk-supervision/08-report.md` | pass exit=0 | sha256:eeaffe3f2081 | 2026-09-25 | assay-verifier-app[bot] @ 7aa3835d7f33 (on-behalf-of human:ian) (forge-identity) |
| 8 | `grep -c 'wedges' docs/streams/desk-supervision/08-report.md` | fail exit=0 | sha256:10159baf262b | 2026-09-25 | assay-verifier-app[bot] @ 7aa3835d7f33 (on-behalf-of human:ian) (forge-identity) |
| 9 | `statusgen --root . --consumers --brief desk-supervision/08` | fail exit=2 | sha256:dc361d0bc812 | 2026-09-25 | assay-verifier-app[bot] @ 7aa3835d7f33 (on-behalf-of human:ian) (forge-identity) |

Direct runs of the same commands (hand-written; key output lines):

| # | Command | Expected | Observed (exit + key output) | Date | Runner |
|---|---------|----------|------------------------------|------|--------|
| 1 | cd tools/desk && GOWORK=off go run ./cmd/deskdispatch --kits | exit 0; output contains worker-objective | PASS — exit 0; stdout lists review, verifier, worker, worker-objective | 2026-09-25 | assay-verifier-app[bot] @ 7aa3835d7f33 (claude-opus-5-5) (on-behalf-of human:ian) |
| 2 | cd tools/desk && GOWORK=off go run ./cmd/deskdispatch --dry-run --kit worker-objective --root . assay/desk-supervision/08 \| grep -c 'KUBECONFIG=/dev/null' | output is 1 or more | FAIL — grep count 0, pipeline exit 1. deskdispatch refuses the spelling: "first argument must be the item-key, not a flag (--dry-run)", exit status 5. Diagnostic with the item-key first and every other argument identical: exit 0, grep count 2. The property holds; the row as written cannot pass | 2026-09-25 | assay-verifier-app[bot] @ 7aa3835d7f33 (claude-opus-5-5) (on-behalf-of human:ian) |
| 3 | cd tools/desk/cmd/deskdispatch/references && awk '/<!-- common-clauses:begin -->/,/<!-- common-clauses:end -->/' worker-prompt-objective.md \| diff - common-clauses.md | exit 0 (byte-identical inclusion) | PASS — exit 0, diff empty | 2026-09-25 | assay-verifier-app[bot] @ 7aa3835d7f33 (claude-opus-5-5) (on-behalf-of human:ian) |
| 4 | ls tools/skillbench/fixtures/worker-kit/ \| wc -l | output is 5 | PASS — 5 (01-observable-probe, 02-run-stop-signal, 03-eligibility-reconcile, 05-per-class-caps, 06-workpad-upsert) | 2026-09-25 | assay-verifier-app[bot] @ 7aa3835d7f33 (claude-opus-5-5) (on-behalf-of human:ian) |
| 5 | cd tools/skillbench && GOWORK=off go run . --arms ../../docs/streams/desk-supervision/08-arms \| grep -c 'check_pass_rate' | output is 1 or more | FAIL — grep count 0, pipeline exit 1; skillbench exit 0. skillbench writes 0 bytes to stdout; its only stderr line is "skillbench: wrote reports/skillbench/2026-09-25-08-arms.md". The report renders the safety floor as the prose heading "Task-check pass rate" and states "Safety floor (task-check pass rate): held — with-overlay pass rate 100% >= without-overlay 100%". The property holds; the row greps a stream that is always empty. Generated report removed afterwards | 2026-09-25 | assay-verifier-app[bot] @ 7aa3835d7f33 (claude-opus-5-5) (on-behalf-of human:ian) |
| 6 | for a in with-overlay without-overlay; do ls docs/streams/desk-supervision/08-arms/$a \| wc -l; done | each line is 15 or more | PASS — 15 and 15 | 2026-09-25 | assay-verifier-app[bot] @ 7aa3835d7f33 (claude-opus-5-5) (on-behalf-of human:ian) |
| 7 | grep -E -e '^decision: adopt-candidate — ' -e '^decision: reject — ' docs/streams/desk-supervision/08-report.md | exit 0; exactly one line | PASS — exit 0; exactly one line: decision adopt-candidate — check_pass_rate 100%/15 vs 100%/15 (equal), wedges 0/15 vs 0/15 (equal), wall_seconds 31.7 vs 23.7 (+33.8%, cost-side regression), diff_lines 8.3 vs 8.1 (+2.5%, cost-side regression) | 2026-09-25 | assay-verifier-app[bot] @ 7aa3835d7f33 (claude-opus-5-5) (on-behalf-of human:ian) |
| 8 | grep -c 'wedges' docs/streams/desk-supervision/08-report.md | output is 1 or more | PASS — exit 0, grep count 7 (7 is 1 or more). The witness row marks this row fail with "no output line equals 1": verifyrun's expectation parser reads "1 or more" as "a line equals 1". That is an instrument artifact, and the direct run satisfies the row as written | 2026-09-25 | assay-verifier-app[bot] @ 7aa3835d7f33 (claude-opus-5-5) (on-behalf-of human:ian) |
| 9 | statusgen --root . --consumers --brief desk-supervision/08 | exit 0; output does not contain DISPROVED | COULD-NOT-CHECK — exit 2. statusgen reports "--consumers: COULD-NOT-CHECK: assay:assay:desk-supervision:08 is not in the diff against 7aa3835d7f33 ... no entry was corroborated and none was disproved". No DISPROVED in the output, but the exit-0 expectation is not met. Structural: the brief file has been on main since 2026-09-02, so no post-merge diff contains it | 2026-09-25 | assay-verifier-app[bot] @ 7aa3835d7f33 (claude-opus-5-5) (on-behalf-of human:ian) |

Risk-bearing value enumeration (scope: implementation commit badd7b58c, #1266, non-artifact files plus the brief's Deliverables):
- task-count = 5 @ tools/skillbench/fixtures/worker-kit/ (five task directories; also stated at docs/streams/desk-supervision/08-report.md:9)
- runs-per-task-per-arm = 3 (15 per arm) @ docs/streams/desk-supervision/08-report.md:8-9
- kit count = 5 @ tools/desk/cmd/deskdispatch/kittext_test.go:111, and kit-name count = 4 @ tools/desk/cmd/deskdispatch/deskdispatch_test.go:791 (test pins on the registry size)
- fixture stale threshold = 20*time.Minute @ tools/skillbench/fixtures/worker-kit/01-observable-probe/testdata/start/heartbeat_test.go:11 (fixture test data), plus the fixture cap maps in 05-per-class-caps/testdata/start/caps_test.go
- decision-line numbers @ docs/streams/desk-supervision/08-report.md (observed measurement data, not pinned constants)

Ranking: nothing in the diff is irreversible. Every entry can be undone with an edit and a re-run. The one irreversible-leaning act, making the objective kit the default for --kit worker, is out of scope under the brief's facts. The kit counts and fixture thresholds are test pins and fixture data, and they rank last.

RISK-VALUE: NAMED, NOT DERIVED — task-count = 5 @ tools/skillbench/fixtures/worker-kit/ — missing: a derivation that these five tasks cover the states the procedural kit anticipates and the objective kit must discover. The brief assigns that question to review only, and its single-point-of-failure note names the task set as the one control, backed by two independent signals (per-task check pass rate and wedge count). Reversible.
RISK-VALUE: DERIVED — runs-per-task-per-arm = 3 @ docs/streams/desk-supervision/08-report.md:9 — the brief's own facts pre-register "≥3 runs per task per arm" before any run, and 3 is the smallest value that meets that floor. Reversible.

VERIFY: FAIL — 6/9 pass, 2 fail, 1 could-not-check. The result matches the 2026-09-20 and 2026-09-23 runs. Rows 2 and 5 fail as written, and row 9 is structurally could-not-check after merge. All three are Verify-table defects tracked at #1363 (still open). Every underlying deliverable property holds. Row 8 is red in the witness only because verifyrun misparses "1 or more"; the direct run passes. The input changes since the last receipt did not touch the rows' failure causes. Evidence only; status stays implemented.

### Non-implementer verifier re-run: 2026-10-02T21:25:43Z (UTC), assay-verifier-app[bot] (claude-opus-5-5[1m]) (on-behalf-of human:ian), merged main 5a108baba705c5f9b458501065fc52a12557c969

Every row was run by hand as authored, from the root of a detached worktree at the merged head. Rows 1 and 2 ran with a throwaway HOME holding a read-only copy of the roster, so the desk tools could not write to a live audit log; nothing else about the commands was altered.

| # | Command | Expect | Observed (exit + key output line) | Date / runner |
|---|---------|--------|-----------------------------------|---------------|
| 1 | cd tools/desk && GOWORK=off go run ./cmd/deskdispatch --kits | exit 0; output contains worker-objective | PASS — exit 0; kit list is review, verifier, worker, worker-objective | 2026-10-02 assay-verifier-app[bot] |
| 2 | cd tools/desk && GOWORK=off go run ./cmd/deskdispatch --dry-run --kit worker-objective --root . assay/desk-supervision/08 \| grep -c 'KUBECONFIG=/dev/null' | output is 1 or more | FAIL — grep count 0, pipeline exit 1. deskdispatch refuses with "deskdispatch: first argument must be the <item-key>, not a flag (--dry-run)" (go run reports exit status 5) and prints no prompt. Supplementary, not the row as written: with the item key placed first the same command exits 0 and the count is 2, so the common clauses are present in the rendered kit | 2026-10-02 assay-verifier-app[bot] |
| 3 | cd tools/desk/cmd/deskdispatch/references && awk '/<!-- common-clauses:begin -->/,/<!-- common-clauses:end -->/' worker-prompt-objective.md \| diff - common-clauses.md | exit 0 (byte-identical inclusion) | PASS — diff exit 0, empty output | 2026-10-02 assay-verifier-app[bot] |
| 4 | ls tools/skillbench/fixtures/worker-kit/ \| wc -l | output is 5 | PASS — exit 0; 5 (01-observable-probe, 02-run-stop-signal, 03-eligibility-reconcile, 05-per-class-caps, 06-workpad-upsert) | 2026-10-02 assay-verifier-app[bot] |
| 5 | cd tools/skillbench && GOWORK=off go run . --arms ../../docs/streams/desk-supervision/08-arms \| grep -c 'check_pass_rate' | output is 1 or more | FAIL — grep count 0, pipeline exit 1; skillbench itself exits 0. Its stdout is empty; the only line is on stderr: "skillbench: wrote reports/skillbench/2026-10-02-08-arms.md". The written report carries the safety floor as prose: "Safety floor (task-check pass rate): held — with-overlay pass rate 100% >= without-overlay 100%". The generated report was removed afterwards | 2026-10-02 assay-verifier-app[bot] |
| 6 | for a in with-overlay without-overlay; do ls docs/streams/desk-supervision/08-arms/$a \| wc -l; done | each line is 15 or more | PASS — exit 0; 15 and 15 | 2026-10-02 assay-verifier-app[bot] |
| 7 | grep -E -e '^decision: adopt-candidate — ' -e '^decision: reject — ' docs/streams/desk-supervision/08-report.md | exit 0; exactly one line | PASS — exit 0; one line: "decision: adopt-candidate — check_pass_rate 100%/15 vs 100%/15 (equal), wedges 0/15 vs 0/15 (equal), wall_seconds 31.7 vs 23.7 (+33.8%, cost-side regression), diff_lines 8.3 vs 8.1 (+2.5%, cost-side regression)" | 2026-10-02 assay-verifier-app[bot] |
| 8 | grep -c 'wedges' docs/streams/desk-supervision/08-report.md | output is 1 or more | PASS — exit 0; count 7 | 2026-10-02 assay-verifier-app[bot] |
| 9 | statusgen --root . --consumers --brief desk-supervision/08 | exit 0; output does not contain DISPROVED | COULD-NOT-CHECK — exit 2 (statusgen v1.0.31): "--consumers: COULD-NOT-CHECK: assay:assay:desk-supervision:08 is not in the diff against 5a108baba705... no entry was corroborated and none was disproved". The output has no DISPROVED, but the exit-0 expectation is not met. The row is authored for the implementing branch and has no diff to read on merged main | 2026-10-02 assay-verifier-app[bot] |

Execution witness (statusgen verifyrun v1.0.31, same head): 5/9 pass, command exit 1. Rows 1, 3, 4, 6, 7 pass. Row 2 fail (exit 1, expected 0); row 5 fail (exit 1, expected 0); row 9 fail (exit 2, expected 0); row 8 fail with exit 0 and "no output line equals 1". The row 8 witness result is an expectation-parse artifact: the witness reads "1 or more" as "a line equals 1", while the direct run returns 7 and satisfies the row as written. The witness's write to this file was discarded.

Findings:
- The verdict has not changed since 2026-09-25: 6/9 pass, rows 2 and 5 fail as written, row 9 is could-not-check.
- The Verify table is unchanged, and so is the whole brief file apart from Evidence: its content hash equals the one in the 2026-09-25 outcome record. Rows 2, 5 and 9 are therefore the same commands that failed before.
- What did change on main since the last verdict does not touch any failing row. The objective kit file changed by one line in #1879, inside the common-clauses block together with common-clauses.md, and row 3 still shows the two byte-identical. The dispatcher sources changed in #1650, #1727, #1919, #2000 and #2009; the item-key-first refusal that fails row 2 is still present (now at dispatch.go:177). The skillbench entry point, the arms directory, the fixtures and the report are unchanged.
- Row 2: still failing. The cause is the argument order in the row, not the kit.
- Row 5: still failing. skillbench writes its report to a file and a notice to stderr, never stdout, and the literal string check_pass_rate does not appear in the rendered report.
- Row 9: still could-not-check, and it stays that way on any merged head.
- Every underlying deliverable property holds (kit registered, common clauses byte-identical and rendered, five fixtures, 15 runs per arm, safety floor held at 100% in both arms, one numeric decision line, wedge counts reported).
- All three are Verify-table defects tracked at #1363, which is still open with no re-specification landed. The consumers-marker routing question raised there is also unanswered.
- Risk-bearing values: the frontmatter marks all four risk fields no and the item is not irreversible, so the fail-safe trigger does not fire on this pass. No deliverable constant changed since the 2026-09-25 enumeration above, which stands (task-count = 5 named, not derived; runs-per-task-per-arm = 3 derived).

Status is left at implemented. Re-running this item before rows 2, 5 and 9 are re-specified will reproduce this result.

VERIFY: FAIL — 6/9 pass; rows 2 and 5 fail as written and row 9 is could-not-check on merged main; all three are check-definition defects tracked at #1363 (open), and no deliverable property is failing.
### Non-implementer verifier re-run: 2026-10-07, assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian), merged main 91f04b81ba064394aa121a4940cf11a138b55402

Every row was run by hand as authored, from the root of a detached worktree at the merged head, offline (KUBECONFIG=/dev/null). Row 9 and the witness used the pinned statusgen v1.0.32 from the desk-tools install directory. The Verify table is byte-identical to the one that failed on 2026-10-02 (5a108bab). Since then the objective kit, worker kit and dispatcher sources changed on main; skillbench, the arms directory and the report did not.

| # | Command | Expected | Observed (exit + key output line) | Date | Runner |
|---|---------|----------|-----------------------------------|------|--------|
| 1 | cd tools/desk && GOWORK=off go run ./cmd/deskdispatch --kits | exit 0; output contains worker-objective | PASS — exit 0; kit list is review, verifier, worker, worker-objective | 2026-10-07 | assay-verifier-app[bot] @ 91f04b81ba06 (on-behalf-of human:ian) (forge-identity) |
| 2 | cd tools/desk && GOWORK=off go run ./cmd/deskdispatch --dry-run --kit worker-objective --root . assay/desk-supervision/08 \| grep -c 'KUBECONFIG=/dev/null' | output is 1 or more | FAIL — grep count 0, pipeline exit 1. deskdispatch refuses with "deskdispatch: first argument must be the <item-key>, not a flag (--dry-run)" (go run reports exit status 5); the guard is now at dispatch.go:185. Supplementary, not the row as written: with the item key placed first the same command exits 0, prints a dry-run PLAN with kit=worker-objective, and the count is 2 | 2026-10-07 | assay-verifier-app[bot] @ 91f04b81ba06 (on-behalf-of human:ian) (forge-identity) |
| 3 | cd tools/desk/cmd/deskdispatch/references && awk '/<!-- common-clauses:begin -->/,/<!-- common-clauses:end -->/' worker-prompt-objective.md \| diff - common-clauses.md | exit 0 (byte-identical inclusion) | PASS — diff exit 0, empty output (the objective kit changed on main since 2026-10-02 and still holds the common clauses byte-identical) | 2026-10-07 | assay-verifier-app[bot] @ 91f04b81ba06 (on-behalf-of human:ian) (forge-identity) |
| 4 | ls tools/skillbench/fixtures/worker-kit/ \| wc -l | output is 5 | PASS — exit 0; 5 (01-observable-probe, 02-run-stop-signal, 03-eligibility-reconcile, 05-per-class-caps, 06-workpad-upsert) | 2026-10-07 | assay-verifier-app[bot] @ 91f04b81ba06 (on-behalf-of human:ian) (forge-identity) |
| 5 | cd tools/skillbench && GOWORK=off go run . --arms ../../docs/streams/desk-supervision/08-arms \| grep -c 'check_pass_rate' | output is 1 or more | FAIL — grep count 0, pipeline exit 1; skillbench itself exits 0 and writes 0 bytes to stdout. Its only line is on stderr: "skillbench: wrote reports/skillbench/2026-10-07-08-arms.md". That report has 0 occurrences of check_pass_rate and carries the safety floor as prose: "Safety floor (task-check pass rate): held — with-overlay pass rate 100% >= without-overlay 100%". The generated report was removed afterwards | 2026-10-07 | assay-verifier-app[bot] @ 91f04b81ba06 (on-behalf-of human:ian) (forge-identity) |
| 6 | for a in with-overlay without-overlay; do ls docs/streams/desk-supervision/08-arms/$a \| wc -l; done | each line is 15 or more | PASS — exit 0; 15 and 15 | 2026-10-07 | assay-verifier-app[bot] @ 91f04b81ba06 (on-behalf-of human:ian) (forge-identity) |
| 7 | grep -E -e '^decision: adopt-candidate — ' -e '^decision: reject — ' docs/streams/desk-supervision/08-report.md | exit 0; exactly one line | PASS — exit 0; one line: "decision: adopt-candidate — check_pass_rate 100%/15 vs 100%/15 (equal), wedges 0/15 vs 0/15 (equal), wall_seconds 31.7 vs 23.7 (+33.8%, cost-side regression), diff_lines 8.3 vs 8.1 (+2.5%, cost-side regression)" | 2026-10-07 | assay-verifier-app[bot] @ 91f04b81ba06 (on-behalf-of human:ian) (forge-identity) |
| 8 | grep -c 'wedges' docs/streams/desk-supervision/08-report.md | output is 1 or more | PASS — exit 0; count 7 | 2026-10-07 | assay-verifier-app[bot] @ 91f04b81ba06 (on-behalf-of human:ian) (forge-identity) |
| 9 | statusgen --root . --consumers --brief desk-supervision/08 | exit 0; output does not contain DISPROVED | COULD-NOT-CHECK — exit 2 (statusgen v1.0.32): "--consumers: COULD-NOT-CHECK: assay:assay:desk-supervision:08 is not in the diff against 91f04b81ba06... no entry was corroborated and none was disproved". The output has no DISPROVED, but the exit-0 expectation is not met; the row is authored for the implementing branch and has no diff to read on merged main | 2026-10-07 | assay-verifier-app[bot] @ 91f04b81ba06 (on-behalf-of human:ian) (forge-identity) |

Execution witness (statusgen verifyrun v1.0.32, non-dry, same head): command exit 1. Rows 1, 3, 4, 6, 7 pass. Row 2 fail (exit 1); row 5 fail (exit 1); row 9 fail (exit 2); row 8 fail with exit 0 and "no output line equals 1". The row 8 witness result is an expectation-parse artifact: the witness reads "1 or more" as "a line equals 1", while the direct run returns 7 and satisfies the row as written. The witness's write to this file was discarded after capture.

Risk-bearing values: the frontmatter marks all four risk fields no and irreversible: no, so the fail-safe trigger does not fire. No deliverable constant changed since the 2026-09-25 enumeration, which stands.
RISK-VALUE: NAMED, NOT DERIVED — task-count = 5 @ docs/streams/desk-supervision/08-report.md:9 (five task dirs under tools/skillbench/fixtures/worker-kit) — missing: a derivation that these five tasks cover the states the procedural kit anticipates; the brief assigns representativeness to review only. Reversible.
RISK-VALUE: DERIVED — runs-per-task-per-arm = 3 @ docs/streams/desk-supervision/08-report.md:9 — the brief's facts pre-register at least 3 runs per task per arm; 3 is the smallest value meeting that floor. Reversible.

VERIFY: FAIL — 6/9 pass; rows 2 and 5 fail as written and row 9 is could-not-check on merged main. All three are check-definition defects tracked at #1363 (still open, last updated 2026-09-30, no re-specification landed). No deliverable property is failing. Status stays implemented.

Blocker re-confirmed and attached: https://github.com/medici-finance/assay/issues/1363#issuecomment-6037649434. The NAMED, NOT DERIVED value (task count 5 at 08-report.md:9) is recorded on that issue.

### Non-implementer verifier re-run: 2026-10-11, assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian), merged main 251e9473928e6e8c8c5ffbd331833900331043f1

Pre-work admission: deskdispatch --check-verifier exited 0 for this home and brief (binding run 1f5762ee6a27, source 251e9473928e, model claude-opus-5-5, tier any; attestation issue medici-finance/assay#2563). Every row was run by hand as authored, from the root of a detached worktree at the merged head, offline (KUBECONFIG=/dev/null). Rows 1, 2 and 5 and the witness ran with a throwaway HOME holding a read-only copy of the roster, because deskdispatch appends to the desk audit log even for --kits and --dry-run; nothing else about the commands was altered. The report file skillbench writes into the tree was moved out of the worktree after each run. Row 9 used the pinned statusgen v1.0.37 from the desk-tools install directory. The Verify table is byte-identical to the one that failed on 2026-10-07 (91f04b81); its section hash is unchanged. Since then the objective kit changed on main (#2452, #2366, #2272, #2218) and the dispatcher sources changed (#2515, #2474, #2451, #2449, #2445, #2364); skillbench, the fixtures, the arms directory, the report and the common clauses did not. The table has no check:ci rows, so the network-off sandbox (unshare, absent on Darwin) did not apply to any row.

| # | Command | Expected | Observed (exit + key output line) | Date | Runner |
|---|---------|----------|-----------------------------------|------|--------|
| 1 | cd tools/desk && GOWORK=off go run ./cmd/deskdispatch --kits | exit 0; output contains worker-objective | PASS — exit 0; kit list is review, verifier, worker, worker-objective | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 2 | cd tools/desk && GOWORK=off go run ./cmd/deskdispatch --dry-run --kit worker-objective --root . assay/desk-supervision/08 \| grep -c 'KUBECONFIG=/dev/null' | output is 1 or more | FAIL — grep count 0, pipeline exit 1. deskdispatch refuses with "deskdispatch: first argument must be the <item-key>, not a flag (--dry-run)" (go run reports exit status 5); the guard is still at dispatch.go:185. Supplementary, not the row as written: with the item key placed first the same command exits 0, prints a dry-run PLAN line with kit=worker-objective, and the count is 1 (it was 2 on 2026-10-07; the rendered prompt changed with #2452) | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 3 | cd tools/desk/cmd/deskdispatch/references && awk '/<!-- common-clauses:begin -->/,/<!-- common-clauses:end -->/' worker-prompt-objective.md \| diff - common-clauses.md | exit 0 (byte-identical inclusion) | PASS — diff exit 0, empty output (the objective kit grew from 464 to 639 lines on main since implementation and still holds the common clauses byte-identical) | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 4 | ls tools/skillbench/fixtures/worker-kit/ \| wc -l | output is 5 | PASS — exit 0; 5 (01-observable-probe, 02-run-stop-signal, 03-eligibility-reconcile, 05-per-class-caps, 06-workpad-upsert) | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 5 | cd tools/skillbench && GOWORK=off go run . --arms ../../docs/streams/desk-supervision/08-arms \| grep -c 'check_pass_rate' | output is 1 or more | FAIL — grep count 0, pipeline exit 1; skillbench itself exits 0 and writes 0 bytes to stdout. Its only line is on stderr: "skillbench: wrote reports/skillbench/2026-10-11-08-arms.md". That report has 0 occurrences of check_pass_rate and carries the safety floor as prose: "Safety floor (task-check pass rate): held — with-overlay pass rate 100% >= without-overlay 100%" | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 6 | for a in with-overlay without-overlay; do ls docs/streams/desk-supervision/08-arms/$a \| wc -l; done | each line is 15 or more | PASS — exit 0; 15 and 15 | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 7 | grep -E -e '^decision: adopt-candidate — ' -e '^decision: reject — ' docs/streams/desk-supervision/08-report.md | exit 0; exactly one line | PASS — exit 0; exactly one line (grep -c gives 1): "decision: adopt-candidate — check_pass_rate 100%/15 vs 100%/15 (equal), wedges 0/15 vs 0/15 (equal), wall_seconds 31.7 vs 23.7 (+33.8%, cost-side regression), diff_lines 8.3 vs 8.1 (+2.5%, cost-side regression)" | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 8 | grep -c 'wedges' docs/streams/desk-supervision/08-report.md | output is 1 or more | PASS — exit 0; count 7 | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 9 | statusgen --root . --consumers --brief desk-supervision/08 | exit 0; output does not contain DISPROVED | COULD-NOT-CHECK — exit 2 (statusgen v1.0.37): "--consumers: COULD-NOT-CHECK: assay:assay:desk-supervision:08 is not in the diff against 251e9473928e... no entry was corroborated and none was disproved". The output has no DISPROVED, but the exit-0 expectation is not met. Supplementary, in a separate local clone at the implementing commit badd7b58c0e1 with --base set to its parent 87f7f33fd20e (the statusgen hint): also COULD-NOT-CHECK, exit 2, because the implementing commit did not change the brief file, so the brief is in no diff at all. A hand comparison of that diff against the consumers list: the objective kit is in the diff (fixed-here, matches); the worker-desk skill body is not (out-of-scope, matches); common-clauses.md is in the diff although marked out-of-scope, a two-line change adding only the begin/end marker comments that row 3 depends on. That is the consumers-marker routing question already open on #1363 | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |

Execution witness (statusgen verifyrun v1.0.37, non-dry, same head, run from the home): command exit 1. Rows 1, 3, 4, 6, 7 pass. Row 2 fail (exit 1, expected 0); row 5 fail (exit 1, expected 0); row 9 fail (exit 2, expected 0); row 8 fail with exit 0 and "no output line equals 1". The row 8 witness result is an expectation-parse artifact: the witness reads "1 or more" as "a line equals 1", while the direct run returns 7 and satisfies the row as written. The witness table follows as verifyrun wrote it:

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/desk && GOWORK=off go run ./cmd/deskdispatch --kits` | pass exit=0 | sha256:1b35dc337f07 | 2026-10-11 | assay-verifier-app[bot] @ 251e9473928e (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd tools/desk && GOWORK=off go run ./cmd/deskdispatch --dry-run --kit worker-objective --root . assay/desk-supervision/08 \| grep -c 'KUBECONFIG=/dev/null'` | fail exit=1 | sha256:783bd463e84b | 2026-10-11 | assay-verifier-app[bot] @ 251e9473928e (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd tools/desk/cmd/deskdispatch/references && awk '/<!-- common-clauses:begin -->/,/<!-- common-clauses:end -->/' worker-prompt-objective.md \| diff - common-clauses.md` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-11 | assay-verifier-app[bot] @ 251e9473928e (on-behalf-of human:ian) (forge-identity) |
| 4 | `ls tools/skillbench/fixtures/worker-kit/ \| wc -l` | pass exit=0 | sha256:30179a803d48 | 2026-10-11 | assay-verifier-app[bot] @ 251e9473928e (on-behalf-of human:ian) (forge-identity) |
| 5 | `cd tools/skillbench && GOWORK=off go run . --arms ../../docs/streams/desk-supervision/08-arms \| grep -c 'check_pass_rate'` | fail exit=1 | sha256:3a115ea4858a | 2026-10-11 | assay-verifier-app[bot] @ 251e9473928e (on-behalf-of human:ian) (forge-identity) |
| 6 | `for a in with-overlay without-overlay; do ls docs/streams/desk-supervision/08-arms/$a \| wc -l; done` | pass exit=0 | sha256:9be9b418f3b8 | 2026-10-11 | assay-verifier-app[bot] @ 251e9473928e (on-behalf-of human:ian) (forge-identity) |
| 7 | `grep -E -e '^decision: adopt-candidate — ' -e '^decision: reject — ' docs/streams/desk-supervision/08-report.md` | pass exit=0 | sha256:eeaffe3f2081 | 2026-10-11 | assay-verifier-app[bot] @ 251e9473928e (on-behalf-of human:ian) (forge-identity) |
| 8 | `grep -c 'wedges' docs/streams/desk-supervision/08-report.md` | fail exit=0 | sha256:10159baf262b | 2026-10-11 | assay-verifier-app[bot] @ 251e9473928e (on-behalf-of human:ian) (forge-identity) |
| 9 | `statusgen --root . --consumers --brief desk-supervision/08` | fail exit=2 | sha256:40a0b6379c62 | 2026-10-11 | assay-verifier-app[bot] @ 251e9473928e (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#2563 run=1f5762ee6a27d40a56ce8bab0432ebbf24e7da4e1257a6152e7965e8797e5f97 source=251e9473928e6e8c8c5ffbd331833900331043f1 brief=docs/streams/desk-supervision/brief-08-objectives-over-transitions.md model=claude-opus-5-5 tier=any

Risk-bearing values: the frontmatter marks all four risk fields no and irreversible: no, and no deliverable constant changed since the 2026-09-25 enumeration (skillbench, the fixtures, the arms directory and the report are unchanged on main since 2026-10-07; the objective-kit changes are wording outside the common-clauses block and carry no new literal the brief's Deliverables name), so the fail-safe trigger does not fire and that enumeration stands.
RISK-VALUE: NAMED, NOT DERIVED — task-count = 5 @ docs/streams/desk-supervision/08-report.md:9 (five task dirs under tools/skillbench/fixtures/worker-kit) — missing: a derivation that these five tasks cover the states the procedural kit anticipates; the brief assigns representativeness to review only. Reversible.
RISK-VALUE: DERIVED — runs-per-task-per-arm = 3 @ docs/streams/desk-supervision/08-report.md:9 — the brief's facts pre-register at least 3 runs per task per arm; 3 is the smallest value meeting that floor. Reversible.

Findings:
- Same verdict as 2026-09-20, 09-23, 09-25, 10-02 and 10-07: rows 2 and 5 fail as written and row 9 is could-not-check. All three are check-definition defects tracked at #1363 (open, last updated 2026-10-07, no re-specification landed). No deliverable property is failing.
- New this run: row 9 is could-not-check even against the implementing diff (badd7b58c0e1 against its parent), because the implementing commit did not change the brief file. Re-running row 9 "on the implementing branch", as the row's own note says, cannot make it pass either; it needs a re-specified row or a statusgen change.
- Instrument defect (statusgen v1.0.37): verifyrun --check reports row 3 as could-not-run ("the witness records a different command than the Verify row now carries") against the witness verifyrun wrote moments earlier in this run, byte-for-byte unedited. The same result appears on a copy of the brief whose Evidence holds only this witness table, so the write and check paths disagree about this row's command; the likely cause is the HTML comment markers inside the row's awk pattern. The audit therefore reads 4 pass, 4 fail, 1 could-not-run, where the witness run itself recorded row 3 as pass exit=0.
- Observation for a follow-up adoption brief: the objective kit on main is 639 lines against 464 at implementation (10 commits since), so the 08-report measurement and its adopt-candidate decision describe an earlier revision of the kit.

Status stays implemented. Re-running this item before rows 2, 5 and 9 are re-specified will reproduce this result.

**VERIFY: FAIL** — 6/9 pass; rows 2 and 5 fail as written and row 9 is could-not-check on merged main (and at the implementing commit). All three are check-definition defects tracked at #1363. No deliverable property is failing.

### Non-implementer verifier re-run: 2026-10-15, assay-verifier-app[bot] (claude-opus-5-5 verifier, on behalf of verify-desk), merged main 44054ca3756857c34eb818601a4ddcb19a7b4da5

Pre-work admission: deskdispatch --check-verifier exited 0 for this home and brief (binding run 1a380652124c, source 44054ca37568, model claude-opus-5-5, tier any; attestation issue medici-finance/assay#2992). The delivering change is #1266 (merge commit badd7b58c0e1ed6f344384a72e3eb7381fb42d6a). Every row was run by hand exactly as authored, from the root of a detached worktree at the merged head, offline (KUBECONFIG=/dev/null, GOPROXY=off). Rows 1, 2 and 5 ran with a throwaway HOME holding a read-only copy of the roster, so the desk tools could not append to a live audit log. The row 2 supplementary run first hit a lock on a shared Go build cache that another live dispatcher process held (exit status 6, retryable); it was re-run with a scratch Go build cache outside the worktree, and nothing else changed. The report file that skillbench writes into the tree was moved out of the worktree after row 5. Row 9 used the pinned statusgen v1.0.42. The Verify table is unchanged since the 2026-10-11 run (251e9473). Since then the objective kit changed once on main (#2620, wording in the dependency-ladder text outside the common-clauses block, +13/-4). The dispatcher sources also changed (#2853, #2764, #2761, #2754, #2757, #2427, #2560). skillbench, the fixtures, the arms directory, the report and the common clauses did not change. The table has no check:ci rows.

| # | Command | Expected | Observed (exit + key output line) | Date | Runner |
|---|---------|----------|-----------------------------------|------|--------|
| 1 | cd tools/desk && GOWORK=off go run ./cmd/deskdispatch --kits | exit 0; output contains worker-objective | PASS — exit 0; kit list is review, verifier, worker, worker-objective | 2026-10-15 | assay-verifier-app[bot] (claude-opus-5-5 verifier, on behalf of verify-desk) (on-behalf-of human:ian) |
| 2 | cd tools/desk && GOWORK=off go run ./cmd/deskdispatch --dry-run --kit worker-objective --root . assay/desk-supervision/08 \| grep -c 'KUBECONFIG=/dev/null' | output is 1 or more | FAIL — grep count 0, pipeline exit 1. deskdispatch refuses with "deskdispatch: first argument must be the <item-key>, not a flag (--dry-run)" (go run reports exit status 5); the guard is now at dispatch.go:207. Supplementary, not the row as written: with the item key placed first the same command exits 0, prints "deskdispatch: PLAN (dry run — nothing touched) item=assay/desk-supervision/08 repo=medici-finance/assay tier=any kit=worker-objective", and the count is 1 | 2026-10-15 | assay-verifier-app[bot] (claude-opus-5-5 verifier, on behalf of verify-desk) (on-behalf-of human:ian) |
| 3 | cd tools/desk/cmd/deskdispatch/references && awk '/<!-- common-clauses:begin -->/,/<!-- common-clauses:end -->/' worker-prompt-objective.md \| diff - common-clauses.md | exit 0 (byte-identical inclusion) | PASS — diff exit 0, empty output (objective kit now 648 lines, block at lines 529-648; common clauses 120 lines) | 2026-10-15 | assay-verifier-app[bot] (claude-opus-5-5 verifier, on behalf of verify-desk) (on-behalf-of human:ian) |
| 4 | ls tools/skillbench/fixtures/worker-kit/ \| wc -l | output is 5 | PASS — exit 0; 5 (01-observable-probe, 02-run-stop-signal, 03-eligibility-reconcile, 05-per-class-caps, 06-workpad-upsert) | 2026-10-15 | assay-verifier-app[bot] (claude-opus-5-5 verifier, on behalf of verify-desk) (on-behalf-of human:ian) |
| 5 | cd tools/skillbench && GOWORK=off go run . --arms ../../docs/streams/desk-supervision/08-arms \| grep -c 'check_pass_rate' | output is 1 or more | FAIL — grep count 0, pipeline exit 1; skillbench itself writes 0 bytes to stdout. Its only line is on stderr: "skillbench: wrote reports/skillbench/2026-10-15-08-arms.md". That report has 0 occurrences of check_pass_rate and carries the safety floor as prose: "Safety floor (task-check pass rate): held — with-overlay pass rate 100% >= without-overlay 100%" | 2026-10-15 | assay-verifier-app[bot] (claude-opus-5-5 verifier, on behalf of verify-desk) (on-behalf-of human:ian) |
| 6 | for a in with-overlay without-overlay; do ls docs/streams/desk-supervision/08-arms/$a \| wc -l; done | each line is 15 or more | PASS — exit 0; 15 and 15 | 2026-10-15 | assay-verifier-app[bot] (claude-opus-5-5 verifier, on behalf of verify-desk) (on-behalf-of human:ian) |
| 7 | grep -E -e '^decision: adopt-candidate — ' -e '^decision: reject — ' docs/streams/desk-supervision/08-report.md | exit 0; exactly one line | PASS — exit 0; exactly one line (grep -c gives 1): "decision: adopt-candidate — check_pass_rate 100%/15 vs 100%/15 (equal), wedges 0/15 vs 0/15 (equal), wall_seconds 31.7 vs 23.7 (+33.8%, cost-side regression), diff_lines 8.3 vs 8.1 (+2.5%, cost-side regression)" | 2026-10-15 | assay-verifier-app[bot] (claude-opus-5-5 verifier, on behalf of verify-desk) (on-behalf-of human:ian) |
| 8 | grep -c 'wedges' docs/streams/desk-supervision/08-report.md | output is 1 or more | PASS — exit 0; count 7 | 2026-10-15 | assay-verifier-app[bot] (claude-opus-5-5 verifier, on behalf of verify-desk) (on-behalf-of human:ian) |
| 9 | statusgen --root . --consumers --brief desk-supervision/08 | exit 0; output does not contain DISPROVED | COULD-NOT-CHECK — exit 2 (statusgen v1.0.42): "--consumers: COULD-NOT-CHECK: assay:assay:desk-supervision:08 is not in the diff against 44054ca3756857c34eb818601a4ddcb19a7b4da5, so this run carries no evidence about its claims — no entry was corroborated and none was disproved". The output has no DISPROVED, but the exit-0 expectation is not met. As recorded on 2026-10-11, the implementing commit did not change the brief file either, so this row has no diff to read on merged main or at the implementing commit | 2026-10-15 | assay-verifier-app[bot] (claude-opus-5-5 verifier, on behalf of verify-desk) (on-behalf-of human:ian) |

Verification-Attestation: medici-finance/assay#2992 run=1a380652124ce82c30759589754aeccb7c2d4153c71cb9458cbbff2b72e22c5f source=44054ca3756857c34eb818601a4ddcb19a7b4da5 brief=docs/streams/desk-supervision/brief-08-objectives-over-transitions.md model=claude-opus-5-5 tier=any

Risk-bearing values: the frontmatter marks all four risk fields no, and irreversible is no. The fail-safe trigger does not fire. Since the 2026-10-11 run, no deliverable constant has changed: skillbench, the fixtures, the arms directory and the report are unchanged, and the one objective-kit change is prose outside the common-clauses block with no literal the brief's Deliverables name. The 2026-09-25 enumeration stands.
RISK-VALUE: NAMED, NOT DERIVED — task-count = 5 @ docs/streams/desk-supervision/08-report.md:9 (five task dirs under tools/skillbench/fixtures/worker-kit) — missing: a derivation that these five tasks cover the states the procedural kit anticipates; the brief assigns representativeness to review only. Reversible.
RISK-VALUE: DERIVED — runs-per-task-per-arm = 3 @ docs/streams/desk-supervision/08-report.md:9 — the brief's facts pre-register at least 3 runs per task per arm; 3 is the smallest value meeting that floor. Reversible.

Findings:
- This is the same verdict as every run since 2026-09-20. Rows 2 and 5 fail as written and row 9 is could-not-check. All three are check-definition defects tracked at #1363, which is open (last updated 2026-10-13) with no re-specification landed. No deliverable property is failing.
- The objective kit has grown to 648 lines since the measurement. The 08-report decision line still describes an earlier revision of the kit, as noted on 2026-10-11.

Status stays implemented. Re-running this item before rows 2, 5 and 9 are re-specified will reproduce this result.

**VERIFY: FAIL** — 6/9 pass; rows 2 and 5 fail as written and row 9 is could-not-check on merged main. All three are check-definition defects tracked at #1363. No deliverable property is failing.

### Non-implementer verifier re-run: 2026-10-22, assay-verifier-app[bot] (claude-opus-5-5 verifier), merged main 05dc9fbc88f2b1b23cd78bebc8634c564327eca4

Pre-work admission: deskdispatch --check-verifier exited 0 for this home and brief before any row (binding run 88254e933a8f, source 05dc9fbc88f2, model claude-opus-5-5, tier strong; attestation issue medici-finance/assay#4024). This is the first run against the reworked 10-row Verify table delivered by #3239 (repair commit 75e338ca0); the original delivery is #1266 (merge commit badd7b58c). The execution witness below was written by statusgen verifyrun v1.0.54 in write mode from the admitted home at the source sha on a clean tree; its stamps and attestation line are exactly as the tool wrote them. Every row was then also run by hand from the same home, offline (KUBECONFIG=/dev/null; rows 2 and 5 carry GOPROXY=off themselves), with TMPDIR, GOCACHE and XDG_CACHE_HOME in a scratch directory outside the home; row 5's reducer report went to that scratch directory, never into the tree. No row reads a sibling checkout. The table has no check:ci rows; at write time the witness ran all ten rows (8 pass, 2 fail), and its later audit marks row 3 could-not-run through the parser issue noted under row 3 below.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/desk && GOWORK=off go run ./cmd/deskdispatch --kits` | pass exit=0 | sha256:62a34e5296d6 | 2026-10-22 | assay-verifier-app[bot] @ 05dc9fbc88f2 (on-behalf-of human:ian) (forge-identity) |
| 2 | `bash -o pipefail -c 'cd tools/desk && GOWORK=off GOPROXY=off go run ./cmd/deskdispatch assay/desk-supervision/08 --dry-run --kit worker-objective --root ../.. \| grep -F "KUBECONFIG=/dev/null" >/dev/null'` | pass exit=0 | sha256:d73b08998027 | 2026-10-22 | assay-verifier-app[bot] @ 05dc9fbc88f2 (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd tools/desk/cmd/deskdispatch/references && awk '/<!-- common-clauses:begin -->/,/<!-- common-clauses:end -->/' worker-prompt-objective.md \| diff - common-clauses.md` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-22 | assay-verifier-app[bot] @ 05dc9fbc88f2 (on-behalf-of human:ian) (forge-identity) |
| 4 | `ls tools/skillbench/fixtures/worker-kit/ \| wc -l` | pass exit=0 | sha256:30179a803d48 | 2026-10-22 | assay-verifier-app[bot] @ 05dc9fbc88f2 (on-behalf-of human:ian) (forge-identity) |
| 5 | `bash docs/streams/desk-supervision/verify.d/08-reduction.sh` | pass exit=0 | sha256:b45ebfe2696b | 2026-10-22 | assay-verifier-app[bot] @ 05dc9fbc88f2 (on-behalf-of human:ian) (forge-identity) |
| 6 | `for a in with-overlay without-overlay; do ls docs/streams/desk-supervision/08-arms/$a \| wc -l; done` | pass exit=0 | sha256:9be9b418f3b8 | 2026-10-22 | assay-verifier-app[bot] @ 05dc9fbc88f2 (on-behalf-of human:ian) (forge-identity) |
| 7 | `grep -E -e '^decision: adopt-candidate — ' -e '^decision: reject — ' docs/streams/desk-supervision/08-report.md` | fail exit=1 | sha256:e3b0c44298fc | 2026-10-22 | assay-verifier-app[bot] @ 05dc9fbc88f2 (on-behalf-of human:ian) (forge-identity) |
| 8 | `grep -c 'wedges' docs/streams/desk-supervision/08-report.md` | fail exit=0 | sha256:06e9d52c1720 | 2026-10-22 | assay-verifier-app[bot] @ 05dc9fbc88f2 (on-behalf-of human:ian) (forge-identity) |
| 9 | `python3 docs/streams/desk-supervision/verify.d/08-consumers.py` | pass exit=0 | sha256:5cffef493b51 | 2026-10-22 | assay-verifier-app[bot] @ 05dc9fbc88f2 (on-behalf-of human:ian) (forge-identity) |
| 10 | `python3 docs/streams/desk-supervision/verify.d/08-mutations.py` | pass exit=0 | sha256:fe939745f9de | 2026-10-22 | assay-verifier-app[bot] @ 05dc9fbc88f2 (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#4024 run=88254e933a8f44f1d3ed223f932e3f6ecb4103450a048054ae774a7bcdd24fe6 source=05dc9fbc88f2b1b23cd78bebc8634c564327eca4 brief=docs/streams/desk-supervision/brief-08-objectives-over-transitions.md model=claude-opus-5-5 tier=strong

Observed output from the hand runs (the same commands, same home, same source, run by assay-verifier-app[bot] @ 05dc9fbc88f2b1b23cd78bebc8634c564327eca4 (claude-opus-5-5 verifier) on 2026-10-22). These are prose observations next to the witness, not a second Evidence table:

- Row 1: PASS. Exit 0; the kit list is review, verifier, worker, worker-objective.
- Row 2: PASS. Exit 0; the dry run prints "deskdispatch: PLAN (dry run — nothing touched) item=assay/desk-supervision/08 repo=medici-finance/assay tier=any kit=worker-objective" and the emitted prompt contains the KUBECONFIG=/dev/null clause.
- Row 3: PASS. diff exit 0 with empty output (objective kit 661 lines, common clauses 120 lines). The witness audit (statusgen verifyrun --check) reports this row could-not-run because it reads the recorded command as different from the row; that is the witness parser issue the rework boundary tracks at #2679, and the byte comparison itself is clean.
- Row 4: PASS. Exit 0; 5 (01-observable-probe, 02-run-stop-signal, 03-eligibility-reconcile, 05-per-class-caps, 06-workpad-upsert).
- Row 5: PASS. Exit 0; stdout "- Safety floor (task-check pass rate): held — with-overlay pass rate 100% >= without-overlay 100%"; the report went to a scratch temp file.
- Row 6: PASS. Exit 0; 15 and 15.
- Row 7: FAIL. Exit 1, no output. The report's only decision line (line 96) is "decision: could-not-check — check_pass_rate 100%/15 vs 100%/15; wedges could-not-check in both arms (observer logs absent)", so neither adopt-candidate nor reject is present.
- Row 8: PASS. Exit 0; count 6. The witness marks this row fail with "no output line equals 1" because its expectation parser reads "1 or more" as an exact match; the direct run meets the row as written.
- Row 9: PASS. Exit 0; "CORROBORATED: 3 consumer routes; common-clause change is markers only; no guard prose changed".
- Row 10: PASS. Exit 0; "rejected planted failed task result", "rejected planted additional consumer route", "2 planted defects rejected".

Risk-bearing values: the frontmatter carries all four risk fields as no (irreversible: no), and the rework diff touches only verification scripts, the report and a CLI classification file, so the fail-safe trigger does not fire. Enumeration over the rework commit's verification scripts and the brief's Deliverables, for the record: date = 2026-09-17 @ docs/streams/desk-supervision/verify.d/08-reduction.sh:9; expected floor matches = 2 @ docs/streams/desk-supervision/verify.d/08-reduction.sh:11; DELIVERY = badd7b58c0e1ed6f344384a72e3eb7381fb42d6a @ docs/streams/desk-supervision/verify.d/08-consumers.py:14; route count = 3 @ docs/streams/desk-supervision/verify.d/08-consumers.py:31; subprocess timeouts 20 and 90 seconds @ docs/streams/desk-supervision/verify.d/08-mutations.py:17 and :25. All are reversible check-definition knobs.
RISK-VALUE: NAMED, NOT DERIVED — task-count = 5 @ docs/streams/desk-supervision/08-report.md:6 (five task dirs under tools/skillbench/fixtures/worker-kit) — missing: a derivation that these five tasks cover the states the procedural kit anticipates; the brief assigns representativeness to review only. Reversible.
RISK-VALUE: DERIVED — runs-per-task-per-arm = 3 @ docs/streams/desk-supervision/08-report.md:9 — the brief's facts pre-register at least 3 runs per task per arm; 3 is the smallest value meeting that floor, and row 6 observes 15 = 5 x 3 per arm. Reversible.
RISK-VALUE: DERIVED — DELIVERY = badd7b58c0e1ed6f344384a72e3eb7381fb42d6a @ docs/streams/desk-supervision/verify.d/08-consumers.py:14 — it is the #1266 merge commit named in the rework boundary, and row 9 reads only its first-parent diff, so later edits cannot be misattributed to this item. Reversible.

Findings:
- The rework fixed the three check-definition defects from earlier runs. Rows 2, 5 and 9 now pass as written, and row 10's positive controls show that the reduction and consumer oracles reject planted defects.
- Row 7 fails as the brief's own rework boundary predicts. The stored arms carry no observer log, so wedges and the adoption decision are could-not-check, and the report says so honestly instead of claiming a winner. This is the acceptance blocker already tracked at #2816; Task 3 and Task 4 remain owed until observer-backed runs exist. No new defect was observed.
- The objective kit is now 661 lines; the measured comparison describes an earlier revision of it, as noted in earlier runs.

Status stays implemented.

**VERIFY: FAIL** — 9/10 pass; row 7 fails (no adopt-candidate or reject decision line; observer logs absent, tracked at #2816). Row 8 is red in the witness only through its "1 or more" parse; the direct run passes.
### Non-implementer verifier re-run: 2026-10-27, assay-verifier-app[bot] (claude-opus-5-5 verifier), merged main dda6c8c67d3bbdb185c16428609e6e11678ca63a

Pre-work admission: deskdispatch --check-verifier exited 0 for this home and brief before any row (binding run 56c8c51a290e, source dda6c8c67d3b, model claude-opus-5-5, tier strong; attestation issue medici-finance/assay#4650). The Verify table is the 10-row table from the #3239 rework, unchanged since the 2026-10-22 run. The original delivery is #1266 (merge commit badd7b58c). Since the 2026-10-22 run (source 05dc9fbc88f2), the only change to a deliverable is #4481: it rewords the changelog-fragment paragraph of the objective kit (slug with every / replaced by -, one flat file), outside the common-clauses block. The objective kit is now 664 lines. skillbench, the fixtures, the arms directory, the report, the verify.d scripts and the common clauses did not change. The execution witness below was written by statusgen verifyrun v1.0.63 in write mode, from the admitted home at the source sha on a clean tree. Its stamps and attestation line are exactly as the tool wrote them. verifyrun runs its own admission check, which needs the verifier App credential, so the witness run used the operator's normal home directory; a throwaway home holding only the roster copy was refused before any row ran ("cannot obtain the verifier GitHub App installation token"). Every row was then also run by hand from the same home, offline (KUBECONFIG=/dev/null; rows 2 and 5 carry GOPROXY=off themselves), with TMPDIR in a scratch directory outside the home. Row 5's reducer report went to that scratch directory, never into the tree. No row reads a sibling checkout. The table has no check:ci rows. No row is risk-bearing for fail-first: the brief's four risk fields are all no, and no row carries a risk-bearing, live, mutating or end-to-end tag. At write time the witness ran all ten rows, with 8 pass and 2 fail.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/desk && GOWORK=off go run ./cmd/deskdispatch --kits` | pass exit=0 | sha256:62a34e5296d6 | 2026-10-27 | assay-verifier-app[bot] @ dda6c8c67d3b (on-behalf-of human:ian) (forge-identity) |
| 2 | `bash -o pipefail -c 'cd tools/desk && GOWORK=off GOPROXY=off go run ./cmd/deskdispatch assay/desk-supervision/08 --dry-run --kit worker-objective --root ../.. \| grep -F "KUBECONFIG=/dev/null" >/dev/null'` | pass exit=0 | sha256:20fc51262c8d | 2026-10-27 | assay-verifier-app[bot] @ dda6c8c67d3b (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd tools/desk/cmd/deskdispatch/references && awk '/<!-- common-clauses:begin -->/,/<!-- common-clauses:end -->/' worker-prompt-objective.md \| diff - common-clauses.md` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-27 | assay-verifier-app[bot] @ dda6c8c67d3b (on-behalf-of human:ian) (forge-identity) |
| 4 | `ls tools/skillbench/fixtures/worker-kit/ \| wc -l` | pass exit=0 | sha256:30179a803d48 | 2026-10-27 | assay-verifier-app[bot] @ dda6c8c67d3b (on-behalf-of human:ian) (forge-identity) |
| 5 | `bash docs/streams/desk-supervision/verify.d/08-reduction.sh` | pass exit=0 | sha256:7c04138450e6 | 2026-10-27 | assay-verifier-app[bot] @ dda6c8c67d3b (on-behalf-of human:ian) (forge-identity) |
| 6 | `for a in with-overlay without-overlay; do ls docs/streams/desk-supervision/08-arms/$a \| wc -l; done` | pass exit=0 | sha256:9be9b418f3b8 | 2026-10-27 | assay-verifier-app[bot] @ dda6c8c67d3b (on-behalf-of human:ian) (forge-identity) |
| 7 | `grep -E -e '^decision: adopt-candidate — ' -e '^decision: reject — ' docs/streams/desk-supervision/08-report.md` | fail exit=1 | sha256:e3b0c44298fc | 2026-10-27 | assay-verifier-app[bot] @ dda6c8c67d3b (on-behalf-of human:ian) (forge-identity) |
| 8 | `grep -c 'wedges' docs/streams/desk-supervision/08-report.md` | fail exit=0 | sha256:06e9d52c1720 | 2026-10-27 | assay-verifier-app[bot] @ dda6c8c67d3b (on-behalf-of human:ian) (forge-identity) |
| 9 | `python3 docs/streams/desk-supervision/verify.d/08-consumers.py` | pass exit=0 | sha256:5cffef493b51 | 2026-10-27 | assay-verifier-app[bot] @ dda6c8c67d3b (on-behalf-of human:ian) (forge-identity) |
| 10 | `python3 docs/streams/desk-supervision/verify.d/08-mutations.py` | pass exit=0 | sha256:fe939745f9de | 2026-10-27 | assay-verifier-app[bot] @ dda6c8c67d3b (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#4650 run=56c8c51a290ec5121e2389a4c6dcb5757deeeed94008868a326e693e942c2841 source=dda6c8c67d3bbdb185c16428609e6e11678ca63a brief=docs/streams/desk-supervision/brief-08-objectives-over-transitions.md model=claude-opus-5-5 tier=strong


Observed output from the hand runs (same commands, home and source sha, run by assay-verifier-app[bot] (claude-opus-5-5 verifier) on 2026-10-27). These are prose observations next to the witness, not a second Evidence table:

- Row 1: PASS. Exit 0. The kit list is review, verifier, worker, worker-objective.
- Row 2: PASS. Exit 0. The dry run prints "deskdispatch: PLAN (dry run — nothing touched) item=assay/desk-supervision/08 repo=medici-finance/assay tier=any kit=worker-objective branch=feat/assay-desk-supervision-08", and the emitted prompt contains the KUBECONFIG=/dev/null clause.
- Row 3: PASS. diff exits 0 with empty output (objective kit 664 lines, common clauses 120 lines). statusgen verifyrun --check reports this row could-not-run because it reads the recorded command as different from the row. That is the witness parser issue tracked at #2679 (literal HTML-comment tokens inside a command), which is still open. The byte comparison itself is clean.
- Row 4: PASS. Exit 0; output 5 (01-observable-probe, 02-run-stop-signal, 03-eligibility-reconcile, 05-per-class-caps, 06-workpad-upsert).
- Row 5: PASS. Exit 0. stdout is "- Safety floor (task-check pass rate): held — with-overlay pass rate 100% >= without-overlay 100%". The report went to a scratch temp file.
- Row 6: PASS. Exit 0; 15 and 15.
- Row 7: FAIL. Exit 1, no output. The report's only decision line (line 96) is "decision: could-not-check — check_pass_rate 100%/15 vs 100%/15; wedges could-not-check in both arms (observer logs absent)". It has neither adopt-candidate nor reject.
- Row 8: PASS on the direct run. Exit 0; count 6. The witness marks this row fail with "no output line equals 1" because its expectation parser reads "1 or more" as an exact match. That is the same row-wording parse class recorded at #1702. The direct run meets the row as written.
- Row 9: PASS. Exit 0. "CORROBORATED: 3 consumer routes; common-clause change is markers only; no guard prose changed".
- Row 10: PASS. Exit 0. "rejected planted failed task result", "rejected planted additional consumer route", "2 planted defects rejected".

statusgen verifyrun --check on this brief, after the witness was written: "7 pass, 2 fail, 1 could-not-run/missing (of 10 Verify rows); 0 author-added with no acceptance credit" (exit 2).

Risk-bearing values: the frontmatter marks all four risk fields no (irreversible: no), so the fail-safe trigger does not fire. Enumeration, for the record, covers the #4481 change to the objective kit, the rework's verification scripts and the brief's Deliverables. The #4481 change adds no numeric literal; its one rule is a character substitution in a slug. The rework literals are the same as on 2026-10-22: date = 2026-09-17 @ docs/streams/desk-supervision/verify.d/08-reduction.sh:9; expected floor matches = 2 @ docs/streams/desk-supervision/verify.d/08-reduction.sh:11; DELIVERY = badd7b58c0e1ed6f344384a72e3eb7381fb42d6a @ docs/streams/desk-supervision/verify.d/08-consumers.py:14; subprocess timeouts of 20 and 90 seconds @ docs/streams/desk-supervision/verify.d/08-mutations.py:17 and :25. All of them are reversible check-definition knobs.
RISK-VALUE: NAMED, NOT DERIVED — task-count = 5 @ docs/streams/desk-supervision/08-report.md:6 (five task dirs under tools/skillbench/fixtures/worker-kit) — missing: a derivation that these five tasks cover the states the procedural kit anticipates; the brief assigns representativeness to review only. Reversible.
RISK-VALUE: DERIVED — runs-per-task-per-arm = 3 @ docs/streams/desk-supervision/08-report.md:9 — the brief's facts pre-register at least 3 runs per task per arm; 3 is the smallest value meeting that floor, and row 6 observes 15 = 5 x 3 per arm. Reversible.
RISK-VALUE: DERIVED — DELIVERY = badd7b58c0e1ed6f344384a72e3eb7381fb42d6a @ docs/streams/desk-supervision/verify.d/08-consumers.py:14 — it is the #1266 merge commit named in the rework boundary, and row 9 reads only its first-parent diff, so later edits (including #4481) cannot be misattributed to this item. Reversible.

Findings:
- This run gets the same result as 2026-10-22. Row 7 fails as the brief's own rework boundary predicts: the stored arms carry no observer log, so wedges and the adoption decision are could-not-check. The report says so instead of declaring a winner. This is the acceptance blocker tracked at #2816 (open). Task 3 and Task 4 remain owed until observer-backed runs exist.
- The #4481 rewording leaves the common-clauses block byte-identical (row 3). The measured comparison still describes an earlier revision of the kit.
- No new deliverable defect was observed.

Status stays implemented.

**VERIFY: FAIL** — 9/10 rows meet their Expect on direct run. Row 7 fails: there is no adopt-candidate or reject decision line, because observer logs are absent (tracked at #2816). In the witness audit, row 8 also reads fail through its "1 or more" parse (#1702 class), and row 3 reads could-not-run through the parser issue tracked at #2679.
**VERIFY: FAIL** — 9/10 rows meet their Expect on direct run. Row 7 fails: there is no adopt-candidate or reject decision line, because observer logs are absent (tracked at #2816). In the witness audit, row 8 also reads fail through its "1 or more" parse (#1702 class), and row 3 reads could-not-run through the parser issue tracked at #2679.
### Non-implementer verifier re-run: 2026-10-29, assay-verifier-app[bot] (claude-opus-5-5 verifier), merged main 2f1ede291fb4296a9bf657df59a7a20857627dcb

Pre-work admission: deskdispatch --check-verifier exited 0 for this home and brief before any row (binding run a2a94de8e101, source 2f1ede291fb4, model claude-opus-5-5, tier strong; attestation issue medici-finance/assay#4869). The Verify table is the 10-row table from the #3239 rework, unchanged since the 2026-10-27 run. The original delivery is #1266 (merge commit badd7b58c). Since the 2026-10-27 run (source dda6c8c67d3b), no deliverable changed: the objective kit and common clauses, skillbench, the fixtures, the arms directory, the report and the verify.d scripts are byte-identical. The changes in that window under tools/desk/cmd/deskdispatch touch other dispatcher code and tests, not the kit references. The execution witness below was written by statusgen verifyrun v1.0.65 in write mode, from the admitted home at the source sha on a clean tree. Its stamps and attestation line are exactly as the tool wrote them. Every row was also run by hand from the same home, offline (KUBECONFIG=/dev/null; rows 2 and 5 carry GOPROXY=off themselves), with TMPDIR in a scratch directory outside the home. Row 5's reducer report went to that scratch directory, never into the tree. No row reads a sibling checkout. The table has no check:ci rows, so no container bracket applies. No row is risk-bearing for fail-first: the brief's four risk fields are all no, and no row carries a risk-bearing, live, mutating or end-to-end tag. At write time the witness ran all ten rows, with 8 pass and 2 fail.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/desk && GOWORK=off go run ./cmd/deskdispatch --kits` | pass exit=0 | sha256:62a34e5296d6 | 2026-10-29 | assay-verifier-app[bot] @ 2f1ede291fb4 (on-behalf-of human:ian) (forge-identity) |
| 2 | `bash -o pipefail -c 'cd tools/desk && GOWORK=off GOPROXY=off go run ./cmd/deskdispatch assay/desk-supervision/08 --dry-run --kit worker-objective --root ../.. \| grep -F "KUBECONFIG=/dev/null" >/dev/null'` | pass exit=0 | sha256:c0b3e02e14bf | 2026-10-29 | assay-verifier-app[bot] @ 2f1ede291fb4 (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd tools/desk/cmd/deskdispatch/references && awk '/<!-- common-clauses:begin -->/,/<!-- common-clauses:end -->/' worker-prompt-objective.md \| diff - common-clauses.md` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-29 | assay-verifier-app[bot] @ 2f1ede291fb4 (on-behalf-of human:ian) (forge-identity) |
| 4 | `ls tools/skillbench/fixtures/worker-kit/ \| wc -l` | pass exit=0 | sha256:30179a803d48 | 2026-10-29 | assay-verifier-app[bot] @ 2f1ede291fb4 (on-behalf-of human:ian) (forge-identity) |
| 5 | `bash docs/streams/desk-supervision/verify.d/08-reduction.sh` | pass exit=0 | sha256:c5cbc7c8cdc2 | 2026-10-29 | assay-verifier-app[bot] @ 2f1ede291fb4 (on-behalf-of human:ian) (forge-identity) |
| 6 | `for a in with-overlay without-overlay; do ls docs/streams/desk-supervision/08-arms/$a \| wc -l; done` | pass exit=0 | sha256:9be9b418f3b8 | 2026-10-29 | assay-verifier-app[bot] @ 2f1ede291fb4 (on-behalf-of human:ian) (forge-identity) |
| 7 | `grep -E -e '^decision: adopt-candidate — ' -e '^decision: reject — ' docs/streams/desk-supervision/08-report.md` | fail exit=1 | sha256:e3b0c44298fc | 2026-10-29 | assay-verifier-app[bot] @ 2f1ede291fb4 (on-behalf-of human:ian) (forge-identity) |
| 8 | `grep -c 'wedges' docs/streams/desk-supervision/08-report.md` | fail exit=0 | sha256:06e9d52c1720 | 2026-10-29 | assay-verifier-app[bot] @ 2f1ede291fb4 (on-behalf-of human:ian) (forge-identity) |
| 9 | `python3 docs/streams/desk-supervision/verify.d/08-consumers.py` | pass exit=0 | sha256:5cffef493b51 | 2026-10-29 | assay-verifier-app[bot] @ 2f1ede291fb4 (on-behalf-of human:ian) (forge-identity) |
| 10 | `python3 docs/streams/desk-supervision/verify.d/08-mutations.py` | pass exit=0 | sha256:fe939745f9de | 2026-10-29 | assay-verifier-app[bot] @ 2f1ede291fb4 (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#4869 run=a2a94de8e1011f244ca547cc27a829ba33d0a10eaf196caeb7e1eeb4bc674c58 source=2f1ede291fb4296a9bf657df59a7a20857627dcb brief=docs/streams/desk-supervision/brief-08-objectives-over-transitions.md model=claude-opus-5-5 tier=strong

Observed output from the hand runs (same commands, home and source sha, run by assay-verifier-app[bot] (claude-opus-5-5 verifier) on 2026-10-29). These are prose observations next to the witness, not a second Evidence table:

- Row 1: PASS. Exit 0. The kit list is review, verifier, worker, worker-objective.
- Row 2: PASS. Exit 0. The dry run prints "deskdispatch: PLAN (dry run — nothing touched) item=assay/desk-supervision/08 repo=medici-finance/assay tier=any kit=worker-objective branch=feat/assay-desk-supervision-08", and the emitted prompt contains the KUBECONFIG=/dev/null clause.
- Row 3: PASS. diff exits 0 with empty output (objective kit 664 lines, common clauses 120 lines). statusgen verifyrun --check reports this row could-not-run because it reads the recorded command as different from the row. That is the witness parser issue tracked at #2679 (literal HTML-comment tokens inside a command), still open. The byte comparison itself is clean.
- Row 4: PASS. Exit 0; output 5 (01-observable-probe, 02-run-stop-signal, 03-eligibility-reconcile, 05-per-class-caps, 06-workpad-upsert).
- Row 5: PASS. Exit 0. stdout is "- Safety floor (task-check pass rate): held — with-overlay pass rate 100% >= without-overlay 100%". The report went to a scratch temp file.
- Row 6: PASS. Exit 0; 15 and 15.
- Row 7: FAIL. Exit 1, no output. The report's only decision line (line 96) is "decision: could-not-check — check_pass_rate 100%/15 vs 100%/15; wedges could-not-check in both arms (observer logs absent)". It has neither adopt-candidate nor reject.
- Row 8: PASS on the direct run. Exit 0; count 6. The witness marks this row fail with "no output line equals 1" because its expectation parser reads "1 or more" as an exact match, the row-wording parse class recorded at #1702 (still open). The direct run meets the row as written.
- Row 9: PASS. Exit 0. "CORROBORATED: 3 consumer routes; common-clause change is markers only; no guard prose changed".
- Row 10: PASS. Exit 0. "rejected planted failed task result", "rejected planted additional consumer route", "2 planted defects rejected".

statusgen verifyrun --check on this brief, after the witness was written: "7 pass, 2 fail, 1 could-not-run/missing (of 10 Verify rows); 0 author-added with no acceptance credit" (exit 2).

Risk-bearing values: the frontmatter marks all four risk fields no (irreversible: no), so the fail-safe trigger does not fire. No deliverable changed since the 2026-10-27 run, so the enumeration is unchanged. It covers the rework's verification scripts and the brief's Deliverables: date = 2026-09-17 @ docs/streams/desk-supervision/verify.d/08-reduction.sh:9; expected floor matches = 2 @ docs/streams/desk-supervision/verify.d/08-reduction.sh:11; DELIVERY = badd7b58c0e1ed6f344384a72e3eb7381fb42d6a @ docs/streams/desk-supervision/verify.d/08-consumers.py:14; route count = 3 @ docs/streams/desk-supervision/verify.d/08-consumers.py:31; subprocess timeouts of 20 and 90 seconds @ docs/streams/desk-supervision/verify.d/08-mutations.py:17 and :25. All of them are reversible check-definition knobs.
RISK-VALUE: NAMED, NOT DERIVED — task-count = 5 @ docs/streams/desk-supervision/08-report.md:6 (five task dirs under tools/skillbench/fixtures/worker-kit) — missing: a derivation that these five tasks cover the states the procedural kit anticipates; the brief assigns representativeness to review only. Reversible.
RISK-VALUE: DERIVED — runs-per-task-per-arm = 3 @ docs/streams/desk-supervision/08-report.md:9 — the brief's facts pre-register at least 3 runs per task per arm; 3 is the smallest value meeting that floor, and row 6 observes 15 = 5 x 3 per arm. Reversible.
RISK-VALUE: DERIVED — DELIVERY = badd7b58c0e1ed6f344384a72e3eb7381fb42d6a @ docs/streams/desk-supervision/verify.d/08-consumers.py:14 — it is the #1266 merge commit named in the rework boundary, and row 9 reads only its first-parent diff, so later edits cannot be misattributed to this item. Reversible.

Findings:
- Same result as 2026-10-22 and 2026-10-27. Row 7 fails as the brief's own rework boundary predicts: the stored arms carry no observer log, so wedges and the adoption decision are could-not-check, and the report says so instead of declaring a winner. This is the acceptance blocker tracked at #2816 (open). Task 3 and Task 4 remain owed until observer-backed runs exist.
- No new deliverable defect was observed. A further re-verify cannot change this verdict until #2816 is resolved, since no deliverable has moved.

Status stays implemented.

### Non-implementer verifier re-run: 2026-10-30, assay-verifier-app[bot] (claude-opus-5-5 verifier), merged main c958c4cdc2e5c26ef4042901c5ff80ae5d691879

**VERIFY: FAIL** — row 7: the report has no adopt-candidate or reject decision line, because the stored arms carry no observer log (#2816, open).

Pre-work admission: deskdispatch --check-verifier exited 0 for this home and brief before any row (binding run 412a37f85618, source c958c4cdc2e5, model claude-opus-5-5, tier strong; attestation issue medici-finance/assay#5055). The Verify table is the 10-row table from the #3239 rework; deskpathguard rederive reports 10 rows, 0 author-added, base c958c4cdc2e5. The original delivery is #1266 (merge commit badd7b58c). Since the 2026-10-29 run (source 2f1ede291fb4), the objective kit and common clauses, deskdispatch main.go, skillbench, the fixtures, the arms directory, the report and the verify.d scripts are byte-identical. Among the brief's declared files, only docs/streams/desktools-v2/cli-migration.json changed (5 lines each way); no Verify row reads it. The execution witness below was written by statusgen verifyrun v1.0.67 in write mode, from the admitted home at the source sha on a clean tree. Its stamps and attestation line are exactly as the tool wrote them. Every row was also run by hand from the same home, offline (KUBECONFIG=/dev/null; rows 2 and 5 carry GOPROXY=off themselves), with TMPDIR in a scratch directory outside the home. The table has no check:ci rows, so no container bracket applies. No row is risk-bearing for fail-first: the brief's four risk fields are all no, and no row carries a risk-bearing, live, mutating or end-to-end tag. At write time the witness ran all ten rows, with 8 pass and 2 fail.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/desk && GOWORK=off go run ./cmd/deskdispatch --kits` | pass exit=0 | sha256:62a34e5296d6 | 2026-10-30 | assay-verifier-app[bot] @ c958c4cdc2e5 (on-behalf-of human:ian) (forge-identity) |
| 2 | `bash -o pipefail -c 'cd tools/desk && GOWORK=off GOPROXY=off go run ./cmd/deskdispatch assay/desk-supervision/08 --dry-run --kit worker-objective --root ../.. \| grep -F "KUBECONFIG=/dev/null" >/dev/null'` | pass exit=0 | sha256:19f781c35188 | 2026-10-30 | assay-verifier-app[bot] @ c958c4cdc2e5 (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd tools/desk/cmd/deskdispatch/references && awk '/<!-- common-clauses:begin -->/,/<!-- common-clauses:end -->/' worker-prompt-objective.md \| diff - common-clauses.md` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-30 | assay-verifier-app[bot] @ c958c4cdc2e5 (on-behalf-of human:ian) (forge-identity) |
| 4 | `ls tools/skillbench/fixtures/worker-kit/ \| wc -l` | pass exit=0 | sha256:30179a803d48 | 2026-10-30 | assay-verifier-app[bot] @ c958c4cdc2e5 (on-behalf-of human:ian) (forge-identity) |
| 5 | `bash docs/streams/desk-supervision/verify.d/08-reduction.sh` | pass exit=0 | sha256:4befbf6427e4 | 2026-10-30 | assay-verifier-app[bot] @ c958c4cdc2e5 (on-behalf-of human:ian) (forge-identity) |
| 6 | `for a in with-overlay without-overlay; do ls docs/streams/desk-supervision/08-arms/$a \| wc -l; done` | pass exit=0 | sha256:9be9b418f3b8 | 2026-10-30 | assay-verifier-app[bot] @ c958c4cdc2e5 (on-behalf-of human:ian) (forge-identity) |
| 7 | `grep -E -e '^decision: adopt-candidate — ' -e '^decision: reject — ' docs/streams/desk-supervision/08-report.md` | fail exit=1 | sha256:e3b0c44298fc | 2026-10-30 | assay-verifier-app[bot] @ c958c4cdc2e5 (on-behalf-of human:ian) (forge-identity) |
| 8 | `grep -c 'wedges' docs/streams/desk-supervision/08-report.md` | fail exit=0 | sha256:06e9d52c1720 | 2026-10-30 | assay-verifier-app[bot] @ c958c4cdc2e5 (on-behalf-of human:ian) (forge-identity) |
| 9 | `python3 docs/streams/desk-supervision/verify.d/08-consumers.py` | pass exit=0 | sha256:5cffef493b51 | 2026-10-30 | assay-verifier-app[bot] @ c958c4cdc2e5 (on-behalf-of human:ian) (forge-identity) |
| 10 | `python3 docs/streams/desk-supervision/verify.d/08-mutations.py` | pass exit=0 | sha256:fe939745f9de | 2026-10-30 | assay-verifier-app[bot] @ c958c4cdc2e5 (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#5055 run=412a37f856189aa47885bd85ea1902aeb364b6bf7845db20ab59146d936aac83 source=c958c4cdc2e5c26ef4042901c5ff80ae5d691879 brief=docs/streams/desk-supervision/brief-08-objectives-over-transitions.md model=claude-opus-5-5 tier=strong

Observed output from the hand runs (same commands, home and source sha, run by assay-verifier-app[bot] (claude-opus-5-5 verifier) on 2026-10-30). These are prose observations next to the witness, not a second Evidence table:

- Row 1: PASS. Exit 0. The kit list is review, verifier, worker, worker-objective.
- Row 2: PASS. Exit 0, with the producer failure retained by pipefail; the dry-run prompt contains the KUBECONFIG=/dev/null clause.
- Row 3: PASS. diff exits 0 with empty output (objective kit 664 lines, common clauses 120 lines). statusgen verifyrun --check reports this row could-not-run because it reads the recorded command as different from the row. That is the witness parser issue tracked at #2679 (literal HTML-comment tokens inside a command), still open. The byte comparison itself is clean.
- Row 4: PASS. Exit 0; output 5 (01-observable-probe, 02-run-stop-signal, 03-eligibility-reconcile, 05-per-class-caps, 06-workpad-upsert).
- Row 5: PASS. Exit 0. stdout is "- Safety floor (task-check pass rate): held — with-overlay pass rate 100% >= without-overlay 100%". The report went to a scratch temp file.
- Row 6: PASS. Exit 0; 15 and 15.
- Row 7: FAIL. Exit 1, no output. The report's only decision line (line 96) is "decision: could-not-check — check_pass_rate 100%/15 vs 100%/15; wedges could-not-check in both arms (observer logs absent)". It has neither adopt-candidate nor reject. All 30 run directories hold only diff.patch and run.json; the six arm paths with "observ" in the name are the 01-observable-probe task runs, not observer logs.
- Row 8: PASS on the direct run. Exit 0; count 6. The witness marks this row fail with "no output line equals 1" because its expectation parser reads "1 or more" as an exact match, the row-wording parse class recorded at #1702 (still open). The direct run meets the row as written.
- Row 9: PASS. Exit 0. "CORROBORATED: 3 consumer routes; common-clause change is markers only; no guard prose changed".
- Row 10: PASS. Exit 0. "rejected planted failed task result", "rejected planted additional consumer route", "2 planted defects rejected".

statusgen verifyrun --check on this brief, after the witness was written: "7 pass, 2 fail, 1 could-not-run/missing (of 10 Verify rows); 0 author-added with no acceptance credit".

Risk-bearing values: the frontmatter marks all four risk fields no (irreversible: no), so the fail-safe trigger does not fire. The enumeration is unchanged from the 2026-10-29 run because none of these files changed. It covers the rework's verification scripts and the brief's Deliverables: date = 2026-09-17 @ docs/streams/desk-supervision/verify.d/08-reduction.sh:9; expected floor matches = 2 @ docs/streams/desk-supervision/verify.d/08-reduction.sh:11; DELIVERY = badd7b58c0e1ed6f344384a72e3eb7381fb42d6a @ docs/streams/desk-supervision/verify.d/08-consumers.py:14; route count = 3 @ docs/streams/desk-supervision/verify.d/08-consumers.py:31; subprocess timeouts of 20 and 90 seconds @ docs/streams/desk-supervision/verify.d/08-mutations.py:17 and :25. All of them are reversible check-definition knobs.
RISK-VALUE: NAMED, NOT DERIVED — task-count = 5 @ docs/streams/desk-supervision/08-report.md:6 (five task dirs under tools/skillbench/fixtures/worker-kit) — missing: a derivation that these five tasks cover the states the procedural kit anticipates; the brief assigns representativeness to review only. Reversible.
RISK-VALUE: DERIVED — runs-per-task-per-arm = 3 @ docs/streams/desk-supervision/08-report.md:9 — the brief's facts pre-register at least 3 runs per task per arm; 3 is the smallest value meeting that floor, and row 6 observes 15 = 5 x 3 per arm. Reversible.
RISK-VALUE: DERIVED — DELIVERY = badd7b58c0e1ed6f344384a72e3eb7381fb42d6a @ docs/streams/desk-supervision/verify.d/08-consumers.py:14 — it is the #1266 merge commit named in the rework boundary, and row 9 reads only its first-parent diff, so later edits cannot be misattributed to this item. Reversible.

Findings:
- Same result as 2026-10-22, 2026-10-27 and 2026-10-29. Row 7 fails as the brief's own rework boundary predicts: the stored arms carry no observer log, so wedges and the adoption decision are could-not-check, and the report says so instead of declaring a winner. This is the acceptance blocker tracked at #2816 (open). Task 3 and Task 4 remain owed until observer-backed runs exist.
- No new deliverable defect was observed.

Status stays implemented.

### Non-implementer verifier re-run: 2026-11-01, assay-verifier-app[bot] (claude-opus-5-5 verifier), merged main 891f6865d44140c37963c96709bc24da5ea87312

**VERIFY: FAIL** — row 7: the report has no adopt-candidate or reject decision line, because the stored arms carry no observer log (#2816, open).

Pre-work admission: deskdispatch --check-verifier exited 0 for this home and brief before any row (binding run d2762fe38708, source 891f6865d441, model claude-opus-5-5, tier strong; attestation issue medici-finance/assay#5137). The Verify table is the 10-row table from the #3239 rework; deskpathguard rederive reports 10 rows, 0 author-added, base 891f6865d441. The original delivery is #1266 (merge commit badd7b58c). This run was woken by a change to docs/streams/desktools-v2/cli-migration.json: since the 2026-10-30 run (source c958c4cdc2e5), one owner field moved from desktools-v2/47 to desktools-v2/80 on a pending entry, and no Verify row depends on that entry. The objective kit, common clauses, deskdispatch main.go, skillbench, the fixtures, the arms directory, the report and the verify.d scripts are byte-identical to that run. The execution witness below was written by statusgen verifyrun v1.0.67 in write mode, from the admitted home at the source sha on a clean tree, with a throwaway HOME holding a copy of the roster and a scratch TMPDIR. Its stamps and attestation line are exactly as the tool wrote them. Every row was also run by hand from the same home, offline (KUBECONFIG=/dev/null; rows 2 and 5 carry GOPROXY=off themselves). The table has no check:ci rows, so no container bracket applies. No row is risk-bearing for fail-first: the brief's four risk fields are all no, and no row carries a risk-bearing, live, mutating or end-to-end tag. At write time the witness ran all ten rows, with 8 pass and 2 fail.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/desk && GOWORK=off go run ./cmd/deskdispatch --kits` | pass exit=0 | sha256:62a34e5296d6 | 2026-11-01 | assay-verifier-app[bot] @ 891f6865d441 (on-behalf-of human:ian) (forge-identity) |
| 2 | `bash -o pipefail -c 'cd tools/desk && GOWORK=off GOPROXY=off go run ./cmd/deskdispatch assay/desk-supervision/08 --dry-run --kit worker-objective --root ../.. \| grep -F "KUBECONFIG=/dev/null" >/dev/null'` | pass exit=0 | sha256:2a6937a2af39 | 2026-11-01 | assay-verifier-app[bot] @ 891f6865d441 (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd tools/desk/cmd/deskdispatch/references && awk '/<!-- common-clauses:begin -->/,/<!-- common-clauses:end -->/' worker-prompt-objective.md \| diff - common-clauses.md` | pass exit=0 | sha256:e3b0c44298fc | 2026-11-01 | assay-verifier-app[bot] @ 891f6865d441 (on-behalf-of human:ian) (forge-identity) |
| 4 | `ls tools/skillbench/fixtures/worker-kit/ \| wc -l` | pass exit=0 | sha256:30179a803d48 | 2026-11-01 | assay-verifier-app[bot] @ 891f6865d441 (on-behalf-of human:ian) (forge-identity) |
| 5 | `bash docs/streams/desk-supervision/verify.d/08-reduction.sh` | pass exit=0 | sha256:ee02fac08d85 | 2026-11-01 | assay-verifier-app[bot] @ 891f6865d441 (on-behalf-of human:ian) (forge-identity) |
| 6 | `for a in with-overlay without-overlay; do ls docs/streams/desk-supervision/08-arms/$a \| wc -l; done` | pass exit=0 | sha256:9be9b418f3b8 | 2026-11-01 | assay-verifier-app[bot] @ 891f6865d441 (on-behalf-of human:ian) (forge-identity) |
| 7 | `grep -E -e '^decision: adopt-candidate — ' -e '^decision: reject — ' docs/streams/desk-supervision/08-report.md` | fail exit=1 | sha256:e3b0c44298fc | 2026-11-01 | assay-verifier-app[bot] @ 891f6865d441 (on-behalf-of human:ian) (forge-identity) |
| 8 | `grep -c 'wedges' docs/streams/desk-supervision/08-report.md` | fail exit=0 | sha256:06e9d52c1720 | 2026-11-01 | assay-verifier-app[bot] @ 891f6865d441 (on-behalf-of human:ian) (forge-identity) |
| 9 | `python3 docs/streams/desk-supervision/verify.d/08-consumers.py` | pass exit=0 | sha256:5cffef493b51 | 2026-11-01 | assay-verifier-app[bot] @ 891f6865d441 (on-behalf-of human:ian) (forge-identity) |
| 10 | `python3 docs/streams/desk-supervision/verify.d/08-mutations.py` | pass exit=0 | sha256:fe939745f9de | 2026-11-01 | assay-verifier-app[bot] @ 891f6865d441 (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#5137 run=d2762fe387087a168d8c5bf42af8cdbd3f41da3e52c950696ef2c3fb466cfb08 source=891f6865d44140c37963c96709bc24da5ea87312 brief=docs/streams/desk-supervision/brief-08-objectives-over-transitions.md model=claude-opus-5-5 tier=strong

Observed output from the hand runs (same commands, home and source sha, run by assay-verifier-app[bot] (claude-opus-5-5 verifier) on 2026-11-01). These are prose observations next to the witness, not a second Evidence table:

- Row 1: PASS. Exit 0. The kit list is review, verifier, worker, worker-objective.
- Row 2: PASS. Exit 0, with the producer failure retained by pipefail; the dry-run prompt contains the KUBECONFIG=/dev/null clause.
- Row 3: PASS. diff exits 0 with empty output (objective kit 664 lines, common clauses 120 lines). statusgen verifyrun --check reports this row could-not-run because it reads the recorded command as different from the row. That is the witness parser issue tracked at #2679 (literal HTML-comment tokens inside a command), still open. The byte comparison itself is clean.
- Row 4: PASS. Exit 0; output 5.
- Row 5: PASS. Exit 0. stdout is "- Safety floor (task-check pass rate): held — with-overlay pass rate 100% >= without-overlay 100%". The report went to a scratch temp file.
- Row 6: PASS. Exit 0; 15 and 15.
- Row 7: FAIL. Exit 1, no output. The report's only decision line (line 96) is "decision: could-not-check — check_pass_rate 100%/15 vs 100%/15; wedges could-not-check in both arms (observer logs absent)". It has neither adopt-candidate nor reject. All 30 run directories hold only diff.patch and run.json.
- Row 8: PASS on the direct run. Exit 0; count 6. The witness marks this row fail with "no output line equals 1" because its expectation parser reads "1 or more" as an exact match, the row-wording parse class recorded at #1702 (still open). The direct run meets the row as written.
- Row 9: PASS. Exit 0. "CORROBORATED: 3 consumer routes; common-clause change is markers only; no guard prose changed".
- Row 10: PASS. Exit 0. "rejected planted failed task result", "rejected planted additional consumer route", "2 planted defects rejected".

statusgen verifyrun --check on this brief, after the witness was written: "7 pass, 2 fail, 1 could-not-run/missing (of 10 Verify rows); 0 author-added with no acceptance credit".

Risk-bearing values: the frontmatter marks all four risk fields no (irreversible: no), so the fail-safe trigger does not fire. The enumeration is unchanged from the 2026-10-30 run because none of these files changed. It covers the rework's verification scripts and the brief's Deliverables: date = 2026-09-17 @ docs/streams/desk-supervision/verify.d/08-reduction.sh:9; expected floor matches = 2 @ docs/streams/desk-supervision/verify.d/08-reduction.sh:11; DELIVERY = badd7b58c0e1ed6f344384a72e3eb7381fb42d6a @ docs/streams/desk-supervision/verify.d/08-consumers.py:14; route count = 3 @ docs/streams/desk-supervision/verify.d/08-consumers.py:31; subprocess timeouts of 20 and 90 seconds @ docs/streams/desk-supervision/verify.d/08-mutations.py:17 and :25. All of them are reversible check-definition knobs.
RISK-VALUE: NAMED, NOT DERIVED — task-count = 5 @ docs/streams/desk-supervision/08-report.md:6 (five task dirs under tools/skillbench/fixtures/worker-kit) — missing: a derivation that these five tasks cover the states the procedural kit anticipates; the brief assigns representativeness to review only. Reversible.
RISK-VALUE: DERIVED — runs-per-task-per-arm = 3 @ docs/streams/desk-supervision/08-report.md:9 — the brief's facts pre-register at least 3 runs per task per arm; 3 is the smallest value meeting that floor, and row 6 observes 15 = 5 x 3 per arm. Reversible.
RISK-VALUE: DERIVED — DELIVERY = badd7b58c0e1ed6f344384a72e3eb7381fb42d6a @ docs/streams/desk-supervision/verify.d/08-consumers.py:14 — it is the #1266 merge commit named in the rework boundary, and row 9 reads only its first-parent diff, so later edits cannot be misattributed to this item. Reversible.

Findings:
- Same result as 2026-10-30. Row 7 fails as the brief's own rework boundary predicts: the stored arms carry no observer log, so wedges and the adoption decision are could-not-check, and the report says so instead of declaring a winner. This is the acceptance blocker tracked at #2816 (open). Task 3 and Task 4 remain owed until observer-backed runs exist.
- The declared input that woke this run does not touch any row's failure cause. No new deliverable defect was observed.

Status stays implemented.

## Review
Gate: model (from frontmatter). Reviewer records verdict + date in the stream README table.
