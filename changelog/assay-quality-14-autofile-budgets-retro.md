### Added
- `qualgen` closes the quality loop: a pluggable issue-filer (`qualgen/filer`,
  with a GitHub Issues reference adapter and a first-class dry-run) turns
  above-threshold hotspots and duplicate-block clusters into **advisory,
  budgeted** refactor items — one per distinct target, degrading to dry-run/log
  once the filing budget is spent, and never self-dispatching work.
- Per-stream quality **error-budgets** (`qualgen/consumers`) in an alarm
  posture: a breach raises an alarm record rather than a dashboard line, and a
  budget refuses to arm until the stream has at least two measured windows
  (could-not-measure, never armed at zero).
- A **retrospective input feed** that emits the four-part input set — churn
  trend, gate yield, per-stage ledger, and budget status — as generated/logged
  output a cadence retrospective consumes.
