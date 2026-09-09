### Fixed
- `statusgen`'s DORA-timing recorder no longer shells out to the `gh` CLI. Its three reads —
  main-branch `actions/runs`, closed `pulls`, and a PR's `pulls/{n}/commits` — now go straight to
  GitHub's REST API over `net/http`, authenticated from `GH_TOKEN` (falling back to
  `GITHUB_TOKEN`), with the same pagination and the same field selection as before. On any runner
  image without `gh` on PATH every read had been failing with
  `exec: "gh": executable file not found in $PATH`; because the recorder is fail-open by design
  nothing went red, and `docs/streams/.dora-timing.jsonl` simply never accrued a record — the two
  DORA numbers that can only be answered from a recorded series (`change_lead_time`,
  `time_to_restore`) had no series to answer from. The failure shape is unchanged (a failed read
  still records NOTHING, never fabricates an interval, and never fails the record job); the
  `.dora-timing.jsonl` schema is unchanged (#699).

### Changed
- The recorder's `DEGRADED` line now names the HTTP status that actually failed
  (`restore-episode read: HTTP 401: Bad credentials`) and points at REST reachability and the
  token env vars, instead of telling the operator to "investigate gh availability" — a cause that
  can no longer apply. Which of the two reads could-not-check is still named separately, so a
  partial failure is not smeared into a total one (#699).
