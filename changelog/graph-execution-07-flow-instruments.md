### Added
- `statusgen --flow` (graph-execution/07): per-brief `eligible_to_start` /
  `active_work_time` / `external_wait` / `verification_time` durations derived
  from the historian, fleet-wide medians (gated on `gtSmallN`), `ci_slot_saturation`
  (real network access only with `--forge`, never on credential presence alone)
  and `gate_catch_override` (re-emitted from `--gate-telemetry`'s own sources),
  with an environment stamp on every report. `--bottleneck`'s stage-age heuristic
  is unchanged and renders unaffected beside it.
