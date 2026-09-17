# desk-supervision/08 — Objectives over transitions, measured

Two-arm comparison of the alternative "objective + status map" worker kit
(`tools/desk/cmd/deskdispatch/references/worker-prompt-objective.md`, `--kit worker-objective`)
against the current procedural worker kit (`worker-prompt.md`, `--kit worker`), run over the
five-task fixture set at `tools/skillbench/fixtures/worker-kit/`, reduced by the AI-free
`tools/skillbench` harness. Raw per-run artifacts are committed at
`docs/streams/desk-supervision/08-arms/{with-overlay,without-overlay}/run-*/` (15 runs per
arm — 5 tasks × 3 runs).

## How the runs were produced

Each arm's 15 runs are real, independently dispatched fresh agent sessions (no shared
context between runs, no context carried from the kit-authoring work) working in an
isolated, git-initialized copy of one task's starting fixture. `with-overlay` runs received
only the objective kit's task framing (Objective / Status map / Tools available / Timing /
the non-negotiable clauses relevant to a self-contained fixture); `without-overlay` runs
received the same task under a numbered sequential-steps framing drawn from the procedural
kit's shape. The only deliberate difference between the two arms' dispatch prompts is that
framing — task text, model, and tier were identical across both arms, per the runbook in
`tools/skillbench/README.md`. Each run's own first and last actions wrote `.start_ts` /
`.end_ts` (real wall-clock timestamps), and `diff.patch` is the run's actual `git diff`
against the fixture's starting commit. `check` in each `run.json` was computed by
re-running that task's `check.sh` against the run's own edited tree, independently of the
dispatched agent's self-report — the AI-free half of the measurement.

No `usage.json` was produced for any run: the harness used to dispatch these runs (this
session's own agent-dispatch tool) does not expose a per-run token/cost figure in a form
this brief could commit without guessing, so `tokens` / `cost_usd` are correctly
`could-not-check` in every run rather than a fabricated zero.

## Skillbench report (tools/skillbench, AI-free reducer)

AI-free reducer over session artifacts (`tools/skillbench`). The harness never invokes an agent and never reads GitHub or git; it reduces the committed per-run artifacts of two arms into the deltas below.

### Arms

| Arm | Runs |
|---|---|
| `with-overlay` | 15 |
| `without-overlay` | 15 |

### Per-metric deltas

Each figure is a mean over the runs that carried it; `n` is that count. A metric absent from a run (for example, no usage log) is `could-not-check`, never a measured zero, and a delta is emitted only when both arms measured the metric.

**Diff lines (added+removed)**
- with-overlay: 8.3 (n=15/15)
- without-overlay: 8.1 (n=15/15)
- delta: +0.2 (+2.5%) (regression vs baseline)

**Files touched**
- with-overlay: 2.0 (n=15/15)
- without-overlay: 2.0 (n=15/15)
- delta: +0.0 (+0.0%) (no change)

**Tokens**
- with-overlay: could-not-check (0/15 runs)
- without-overlay: could-not-check (0/15 runs)
- delta: could-not-check — a delta needs a measured mean in BOTH arms

**Cost (USD)**
- with-overlay: could-not-check (0/15 runs)
- without-overlay: could-not-check (0/15 runs)
- delta: could-not-check — a delta needs a measured mean in BOTH arms

**Wall time (s)**
- with-overlay: 31.7 (n=15/15)
- without-overlay: 23.7 (n=15/15)
- delta: +8.0 (+33.8%) (regression vs baseline)

**Task-check pass rate (the safety floor)**
- with-overlay: 100% (n=15/15)
- without-overlay: 100% (n=15/15)
- delta: +0 pp (+0.0%) (no change)

Full tool output, byte-identical to the above modulo heading levels: `docs/streams/desk-supervision/08-skillbench-report.md`.

## Wedges

The brief's decision rule reads wedges from "the observer log" — this repo's `desksupervise`
liveness observer (`tools/desk/internal/loopengine/liveness.go`), whose
`DefaultLivenessPolicy().HeartbeatGap` is **20 minutes**. These 30 runs were never registered
with a live dispatch claim or roster beacon (they are offline fixture runs, per the brief's
own offline envelope), so the real `desksupervise` binary has nothing to observe them
against — running it over them would be a probe against infrastructure this brief is
required to stay off of, not a real measurement. Instead, wedges are counted by applying the
observer's own definition directly to each run's real, independently-timestamped
`.start_ts`/`.end_ts` window: a wedge is a run with no observable progress event for longer
than the HeartbeatGap.

Every one of the 30 runs completed end-to-end in under one minute (15s–55s; see the
`wall_seconds` figures above and the per-run `run.json` files), so none came within two
orders of magnitude of the 20-minute gap.

- **with-overlay: 0 wedges (of 15 runs)**
- **without-overlay: 0 wedges (of 15 runs)**

This is a genuine result, not a null one — but it is also a limit of this fixture set: tasks
small enough to run offline in under a minute cannot exercise the multi-hour stall behaviour
the wedge signal is designed to catch. A future run of this same comparison against
longer-running, multi-step briefs (dispatched through the real desk with a live observer)
would be a stronger test of the wedge half of the decision rule; this report says so rather
than overclaiming what a 30-second fixture task can show.

## Decision

Per the rule stated before the runs (brief `## facts`): adopt-candidate only if
`check_pass_rate` is not lower AND wedges are not higher; otherwise reject.

- `check_pass_rate`: with-overlay 100% vs without-overlay 100% — **not lower**.
- wedges: with-overlay 0 vs without-overlay 0 — **not higher**.

Both conditions hold, so:

decision: adopt-candidate — check_pass_rate 100%/15 vs 100%/15 (equal), wedges 0/15 vs 0/15 (equal), wall_seconds 31.7 vs 23.7 (+33.8%, cost-side regression), diff_lines 8.3 vs 8.1 (+2.5%, cost-side regression)

**This is adopt-candidate on the decision rule's own stated terms, not a clean win.** The
rule keys only on the safety floor (`check_pass_rate`) and the wedge count, and both are
tied at their ceiling/floor on this fixture set. On the two cost-side metrics that had a
measured value in both arms, the objective kit ran ~34% slower in wall time and produced
marginally larger diffs on average (+2.5%) — regressions the decision rule does not gate on,
but that a follow-up adoption brief must weigh. The diff-lines delta is small and mostly a
single-run artifact rather than a systematic pattern: 14 of the 15 task/run pairs in each arm
match line-for-line across arms (5/5/5/6/6/6/10/10/10/10/10/10/11/11/11 vs the same shape),
and the one outlier is `without-overlay/run-03-eligibility-reconcile` (6 lines, a one-line
comparison `prState != "merged" && prState != "closed"`) against its two without-overlay
siblings and all three with-overlay eligibility runs (11 lines, a `switch` statement) — a
per-run implementation-style choice, not a kit-framing effect. `tokens` and `cost_usd` are
could-not-check in every run (see above), so the harness cannot say whether the wall-time
regression maps to a real cost regression or is dominated by dispatch-tool scheduling
variance across parallel runs.

## What a follow-up adoption brief would change

1. **Re-run with token/cost capture wired up.** This report's `tokens`/`cost_usd` columns are
   could-not-check in every run because the dispatch path used here has no committed
   per-run usage log. An adoption brief should dispatch through a harness that writes
   `usage.json`, so the cost-side comparison stops being could-not-check.
2. **Re-run against longer, multi-step briefs through the real desk with a live observer.**
   This fixture set's tasks complete in under a minute each, so the wedge measurement above
   is a genuine 0/0 but not a stress test of the failure mode the wedge signal exists to
   catch (a worker going silent for tens of minutes mid-brief). A follow-up should also
   measure against real briefs dispatched through `deskdispatch --kit worker-objective` on a
   live desk, with `desksupervise`'s real observer reading the actual claim/roster state,
   rather than the fixture-level approximation used here.
3. **Investigate the wall-time regression before adopting.** ~34% slower wall time on
   identical tasks under the objective-style framing is a real, measured signal (n=15/15
   both arms) even though it does not gate the decision rule. Whether it is dispatch-tool
   scheduling noise or an actual property of the objective framing (e.g. more exploration
   before committing to a fix) should be characterized, not assumed either way.
4. **If adopting, this brief's own scope excludes the skill-body change.** Per this brief's
   `consumers:`, `plugins/assay/skills/worker-desk/SKILL.md` is explicitly out of scope here;
   a follow-up adoption brief would be the one to flip `--kit worker` to default to the
   objective kit, or to retire the procedural kit, and would cite this report's numbers as
   its evidence.
