## Awaiting verification / review (1 for the desk · 2 for the driver · 2 for workers · 2 for an operator · 1 runner-pending — of 10 total; 1 could-not-check)

_Gate-queue ordered by score: priorityWeight + staleness×stalenessPerDay + valueWeight + unblocksWeight×blockedCount. The weights are an evolving heuristic (F-09 discipline) — not a claim of truth. Board bucketed by owner (docs/board.md): four owned queues — the driver's human gate, workers' implementer rework, an operator's environment-blocked rows, and runner-pending rows CI or the verify runner moves — then the desk's judgement queue. Each row names its owner and its next act; a row whose inputs cannot be read is could-not-check, never a bucket. Paused and parked streams are not bucketed and count only in the total._

_`done‡` / `verified‡` = closed over an **UNRUN risk-bearing Verify row**: a live/mutating check with no completed Evidence row behind it. UNRUN is DERIVED from Verify-vs-Evidence coverage — a row counts as run only when an Evidence row names it with a date and a runner, so silence reads as unrun. `--lint` names each one and whether it was routed to a follow-up._

### Awaiting human gate (2)

| Stream | Brief | Status | Score | _Blocked_ | Age | Owner | Next act | Verified | Reviewed |
|---|---|---|---|---|---|---|---|---|---|
| active-s | 02 | implemented | 2000 | 0 | — | driver | close the sign-off card | — | — |
| kinds-s | 01 | implemented | 2000 | 0 | — | driver | human action, cite #51 | — | — |

### Awaiting implementer rework (2)

| Stream | Brief | Status | Score | _Blocked_ | Age | Owner | Next act | Verified | Reviewed |
|---|---|---|---|---|---|---|---|---|---|
| active-s | 03 | verified* | 2000 | 0 | — | worker | fix, cite #41 | 2026-07-08 | — |
| kinds-s | 03 | implemented | 2000 | 0 | — | worker | fix, cite #53 | — | — |

### Environment-blocked (2)

| Stream | Brief | Status | Score | _Blocked_ | Age | Owner | Next act | Verified | Reviewed |
|---|---|---|---|---|---|---|---|---|---|
| active-s | 04 | implemented | 2000 | 0 | — | operator | no command recorded (`blocked-by: env`) | — | — |
| kinds-s | 02 | implemented | 2000 | 0 | — | operator | environment blocker, cite #52 | — | — |

### Runner-pending (1)

| Stream | Brief | Status | Score | _Blocked_ | Age | Owner | Next act | Verified | Reviewed |
|---|---|---|---|---|---|---|---|---|---|
| active-s | 05 | verified* | 2000 | 0 | — | CI auto-flip | none; stuck after one main run → file | 2026-07-09 | — |

### Desk-actionable (1)

| Stream | Brief | Status | Score | _Blocked_ | Age | Owner | Next act | Verified | Reviewed |
|---|---|---|---|---|---|---|---|---|---|
| active-s | 01 | implemented | 2000 | 0 | — | verify-desk | triage, then re-bucket | — | — |

### Could-not-check (1)

| Stream | Brief | Status | Score | _Blocked_ | Age | Owner | Next act | Verified | Reviewed |
|---|---|---|---|---|---|---|---|---|---|
| active-s | 06 | implemented | 2000 | 0 | — | verify-desk | could-not-check: Evidence's last verdict is FAIL but no verify-outcome record names this brief — the blocker class is unrecorded | — | — |

### Paused stream (1)

| Stream | Brief | Status | Score | _Blocked_ | Age | Owner | Next act | Verified | Reviewed |
|---|---|---|---|---|---|---|---|---|---|
| paused-s | 01 | implemented | 2000 | 0 | — | — | — | — | — |

