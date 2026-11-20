### Added

- `cellctl gc <cell> [--role <role>]` runs each existing reclaim target (the scratch sweep, Go cache maintenance, and `deskwt prune` for each stream root) and prints one JSON report line per target. It is a dry run unless `--apply`; a failed target does not stop the rest.
- Every running `cellctl desk` supervisor, interactive or cadenced, now runs the installed `cellctl gc <cell> --role <role> --apply --scheduled` for its own desk as a child every `CELL_GC_INTERVAL` (default `1h`, `0` off). Each desk cleans for itself, shared stores keep their existing locks, and a re-pinned install takes effect at the next tick without restarting a window.
