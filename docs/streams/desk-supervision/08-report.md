# desk-supervision/08 — Objectives over transitions, measured

Two-arm comparison of the alternative "objective + status map" worker kit
(`tools/desk/cmd/deskdispatch/references/worker-prompt-objective.md`, `--kit worker-objective`)
against the current procedural worker kit (`worker-prompt.md`, `--kit worker`), run over the
five-task fixture set at `tools/skillbench/fixtures/worker-kit/`, reduced by the AI-free
`tools/skillbench` harness. Raw per-run artifacts are committed at
`docs/streams/desk-supervision/08-arms/{with-overlay,without-overlay}/run-*/` (15 runs per
arm — 5 tasks × 3 runs).

## Stored evidence and scope

This is artifact analysis of the 30 runs delivered in #1266, not a new comparison.
Each committed run directory holds `diff.patch` and `run.json`. The latter carries
`wall_seconds` and `check`; the reducer can reproduce the metrics below from those
records. The original account described independently dispatched sessions with the
same tasks, model and tier. Dispatch prompts, observer logs and `.start_ts` / `.end_ts`
files are not present in the committed artifacts, so that account and observer
coverage cannot be independently reconstructed from these directories.

No `usage.json` is present. Tokens and cost remain could-not-check, rather than zero.
The objective kit has also changed since this measurement; these stored results do
not measure the current kit revision.

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

The brief requires wedges read from brief 01's observer log, using the maximum gap
between observable events. The observer's `DefaultLivenessPolicy().HeartbeatGap` is
20 minutes (`tools/desk/internal/loopengine/liveness.go`). No observer log or evidence
of observer coverage is committed for either arm. The original report stated that
these sessions were never registered with the observer.

- **with-overlay: wedges could-not-check (15 stored runs, no observer log)**
- **without-overlay: wedges could-not-check (15 stored runs, no observer log)**

The recorded run durations are under one minute. They cannot establish that an
observer covered the runs, that events were observed, or that no heartbeat gap
occurred. The earlier duration-derived 0/15 counts are withdrawn. There are no
citable observer log lines from which to reconstruct those counts. Finding #2816
tracks this missing acceptance evidence.

## Decision

The pre-registered rule requires both a safety floor that is not lower and wedges
that are not higher. The stored reduction supports equal task-check pass rates
(100%, n=15/15 in each arm). The wedge comparison is could-not-check, so the rule
cannot produce either adopt-candidate or reject from the available evidence.

decision: could-not-check — check_pass_rate 100%/15 vs 100%/15; wedges could-not-check in both arms (observer logs absent)

No adoption conclusion follows from the stored artifact analysis. Brief Verify row 7
requires an observer-supported adopt-candidate or reject line and remains unmet.

The measured cost-side deltas remain useful: mean wall time is 31.7 vs 23.7 seconds
(+33.8%) and mean diff lines 8.3 vs 8.1 (+2.5%). Tokens and cost are unmeasured.
These values describe the stored runs, not the current kit or observer behavior.

## What a follow-up adoption brief would change

1. **Re-run with token/cost capture wired up.** This report's `tokens`/`cost_usd` columns are
   could-not-check in every run because the dispatch path used here has no committed
   per-run usage log. An adoption brief should dispatch through a harness that writes
   `usage.json`, so the cost-side comparison stops being could-not-check.
2. **Re-run against longer, multi-step briefs through the real desk with a live observer.**
   The missing logs leave wedges could-not-check. A follow-up must collect genuine
   observer logs covering every run and cite the observed events and gaps. Longer tasks
   would also exercise a worker going silent for tens of minutes mid-brief. Use real
   briefs dispatched through `deskdispatch --kit worker-objective`, with the observer
   reading their actual claim/roster state and no substitution of run durations.
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
