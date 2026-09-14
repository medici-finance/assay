### Added
- desk-tools brief 23 is authored: an opt-in, local-only usage-and-timing record written by
  the shared desk substrate at one place, kept as a 7-day UTC history under
  `~/.config/assay/perf/` and pruned on write, with a closed non-PII field set and no
  free-text field at all — plus a `deskperf` read verb for per-tool p50/p90/max wall time,
  refusal ratios and `deskboot` per-step cost. It inherits `docs/telemetry.md`'s promise and
  its exact `ASSAY_TELEMETRY` switch, leaves the append-only audit ledger untouched, and
  ships no sender: a remote sink is named as follow-up, so only the on-disk shape is fixed.
