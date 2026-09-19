### Added
- `capability:cadence-tick` joins the closed capability vocabulary: the scheduled recurring prompt that wakes a desk-role window on the clock, bound per harness in `plugins/assay/references/{claude-code,codex,cursor}.md` (Claude Code: the recurring-prompt loop the coordinator window already arms; Codex/Cursor: the launcher's or an outer scheduler's tick-mode interval, stated as a degradation).

### Changed
- `verify-desk` skill: arming the cadence tick is a REQUIRED, named boot step (a window that cannot arm it says `could-not-check` and files it, never runs keystroke-driven); "never end a turn with a non-empty dispatchable queue" plus a printed stand-down checklist replace the round-summary-as-stopping-point; a desk-set width is re-asserted on every tick so it no longer decays mid-drain; the stale-heartbeat case is named in the default-forward list as a STOP class, never a question for the driver (#1310).
- `deskroster`: the verify-desk default width is now 6 (the measured safe width, equal to its declared ceiling) instead of the sequential 1 it decayed back to after the one-hour width TTL.
