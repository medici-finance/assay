### Fixed

- `deskevidence --outcome-record` (plain and with `--outcome-only`) now honours `--dry-run`. It runs every gate, prints the planned target, and makes no commits-API write and no write-budget spend. Before this it ignored the flag and committed the record. A flag the selected landing shape does not read (such as `--append-only`, `--allow-shrink` or `--row` with `--outcome-record`) is now refused instead of silently ignored (#4554, #5039).
