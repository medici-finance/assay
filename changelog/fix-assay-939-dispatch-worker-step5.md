### Fixed
- The `capability:dispatch-worker` row of `plugins/assay/references/codex.md` and the Codex
  dispatch config step in `docs/adopting-assay.md` §3 are corrected against **codex-cli 0.154.0**:
  the `[features] multi_agent` flag has graduated (`codex features list` reports `stable`/`true`)
  and no longer gates the subagent tools — a child spawns with it set `false` — so the retired
  "`multi_agent` off → dispatch unavailable" reading is replaced. The convenience-degradation
  floor still stands as the design contract, now keyed to the reachable trigger: the `[agents]
  max_concurrent_threads_per_session` concurrency cap. The dependent per-skill degradation rows
  are re-worded from "if `multi_agent` is off" to "where parallel dispatch is unavailable" for
  consistency. (#939)

### Changed
- `docs/codex-smoke-protocol.md` **Step 5** is re-baselined from the now-unreachable
  `multi_agent`-off precondition to forcing `max_concurrent_threads_per_session = 1`, per the
  2026-09-17 human ruling on `#939`. It asserts the serial-dispatch floor observably: the fan-out
  degrades to serial (non-overlapping child lifetimes) with every item completed and every
  guarantee — isolation, evidence, review — intact, and carries a re-open condition for when a
  future CLI re-gates dispatch. (#939)
