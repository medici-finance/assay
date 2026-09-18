# Skill-efficacy report — worker-objective-vs-worker — 2026-09-17

AI-free reducer over session artifacts (`tools/skillbench`). The harness never invokes an agent and never reads GitHub or git; it reduces the committed per-run artifacts of two arms into the deltas below.

## Arms

| Arm | Runs |
|---|---|
| `with-overlay` | 15 |
| `without-overlay` | 15 |

## Per-metric deltas

Each figure is a mean over the runs that carried it; `n` is that count. A metric absent from a run (for example, no usage log) is `could-not-check`, never a measured zero, and a delta is emitted only when both arms measured the metric.

### Diff lines (added+removed)

- with-overlay: 8.3 (n=15/15)
- without-overlay: 8.1 (n=15/15)
- delta: +0.2 (+2.5%) (regression vs baseline)

### Files touched

- with-overlay: 2.0 (n=15/15)
- without-overlay: 2.0 (n=15/15)
- delta: +0.0 (+0.0%) (no change)

### Tokens

- with-overlay: could-not-check (0/15 runs) — no run in this arm carried this metric
- without-overlay: could-not-check (0/15 runs) — no run in this arm carried this metric
- delta: could-not-check — a delta needs a measured mean in BOTH arms

### Cost (USD)

- with-overlay: could-not-check (0/15 runs) — no run in this arm carried this metric
- without-overlay: could-not-check (0/15 runs) — no run in this arm carried this metric
- delta: could-not-check — a delta needs a measured mean in BOTH arms

### Wall time (s)

- with-overlay: 31.7 (n=15/15)
- without-overlay: 23.7 (n=15/15)
- delta: +8.0 (+33.8%) (regression vs baseline)

### Task-check pass rate

- with-overlay: 100% (n=15/15)
- without-overlay: 100% (n=15/15)
- delta: +0 pp (+0.0%) (no change)

## Verdict — input to an adoption decision, not an adoption

This report states per-metric deltas and their `n`. The adopt/hold decision belongs to the consuming adoption brief; the harness draws no conclusion of its own.

- Safety floor (task-check pass rate): held — with-overlay pass rate 100% >= without-overlay 100%
- Cost-side movement (overlay vs baseline): 0 improved, 2 regressed, 2 could-not-check (of the 5 cost-side metrics)
