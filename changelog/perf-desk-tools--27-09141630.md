### Changed
- `deskboard` brief 27 is authored: the plan to put `prs`, `stalled` and the always-on
  policy-drift probe onto the bounded worker pool `actions` and `health` already use, to make
  `throughput` resolve its roots once instead of re-running three whole verbs, and to evaluate
  `deskflip`'s conditions cheapest-first so the commonest refusal (`checks-green`) stops being
  the most expensive one to reach.

### Fixed
- Planned in the same brief: the drift self-check resolves only the bare `desk-tools` pin name,
  so a consumer pinning the per-platform `desk-tools-<os>-<arch>` line the distribution contract
  specifies reports a permanent could-not-check as STALE; body/schema refusals in `deskpost`,
  `deskpr` and `deskreply` never name the offline check that would have caught them; and a
  subcommand `--help` is charged to the append-only audit ledger as a refusal.
