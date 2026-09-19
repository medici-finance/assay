### Added
- Every per-run `cellctl` choice is now a flag, persistable and readable (#1303): `desk`/`up`
  accept `--kind`, `--cockpit`, `--harness`, `--provider`, `--model` for one run; `--set` persists
  every override given in that invocation (each to its own `cell.env` key, one backup first);
  `cellctl set <cell> --kind/--cockpit/--harness/--provider` is sugar for the matching
  `KEY=VALUE` under the same validation; a kind change refuses before writing when the target
  kind's precondition (`CELL_CONTAINER_LAUNCHER`, `CELL_ROOTS`, `CELL_REPO_SLUG`) is missing; and
  a new `cellctl show <cell>` prints each effective value with its source
  (`flag` / `cell.env` / `default`).
