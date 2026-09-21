### Added
- `cellctl` recognises `ASSAY_REPAIR_ADMISSION=on|off` as a cell.env key, so the
  dispatch-boundary repair-admission gate can be turned on durably for a cell rather than only
  via a one-off shell `export`. `cellctl set` accepts `on`/`off` only (a malformed value is
  refused, and `--force` does not lift the value rule), `cellctl show` and `DRY_RUN=1 cellctl
  desk` surface it, and every desk the cell launches carries it in its environment. `off` and
  unset are identical: the key is simply absent from the launched environment.
