### Added
- Briefs can declare `budget:` beside `effort:` (an amount with its unit — `400k tokens`, `25 USD`) and `outcome:` (the requirement id the brief should move, or `none`). `statusgen --lint` flags a unitless budget, an outcome that is not a registered requirement id, and — as an advisory NOTICE — a newly authored brief that names no outcome at all.
- `metrics-harvest cost` reduces per-desk, per-model cost telemetry into a cost-per-passed-Verify-row line; a desk with no cost telemetry renders `could-not-check`, never `0`.
- The `worker-desk` skill gains a budget checkpoint: at 80% of a brief's `budget:` the worker files `help wanted` with the escalation packet instead of continuing silently. It adds a filing and never skips or satisfies a review, Verify row or human gate.
