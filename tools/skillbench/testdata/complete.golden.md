# Skill-efficacy report — complete — 2026-08-21

AI-free reducer over session artifacts (`tools/skillbench`). The harness never invokes an agent and never reads GitHub or git; it reduces the committed per-run artifacts of two arms into the deltas below.

## Arms

| Arm | Runs |
|---|---|
| `with-overlay` | 2 |
| `without-overlay` | 2 |

## Per-metric deltas

Each figure is a mean over the runs that carried it; `n` is that count. A metric absent from a run (for example, no usage log) is `could-not-check`, never a measured zero, and a delta is emitted only when both arms measured the metric.

### Diff lines (added+removed)

- with-overlay: 8.0 (n=2/2)
- without-overlay: 20.0 (n=2/2)
- delta: -12.0 (-60.0%) (improvement)

### Files touched

- with-overlay: 1.0 (n=2/2)
- without-overlay: 2.0 (n=2/2)
- delta: -1.0 (-50.0%) (improvement)

### Tokens

- with-overlay: 12000.0 (n=2/2)
- without-overlay: 20000.0 (n=2/2)
- delta: -8000.0 (-40.0%) (improvement)

### Cost (USD)

- with-overlay: 0.1200 (n=2/2)
- without-overlay: 0.2000 (n=2/2)
- delta: -0.0800 (-40.0%) (improvement)

### Wall time (s)

- with-overlay: 120.0 (n=2/2)
- without-overlay: 200.0 (n=2/2)
- delta: -80.0 (-40.0%) (improvement)

### Task-check pass rate

- with-overlay: 100% (n=2/2)
- without-overlay: 100% (n=2/2)
- delta: +0 pp (+0.0%) (no change)

## Verdict — input to an adoption decision, not an adoption

This report states per-metric deltas and their `n`. The adopt/hold decision belongs to the consuming adoption brief; the harness draws no conclusion of its own.

- Safety floor (task-check pass rate): held — with-overlay pass rate 100% >= without-overlay 100%
- Cost-side movement (overlay vs baseline): 5 improved, 0 regressed, 0 could-not-check (of the 5 cost-side metrics)
