### Fixed

- `statusgen --consumers --brief <id>` with no `--base` or `--delivery` no longer treats an uncommitted edit to the brief (for example a verifier's Evidence insertion) as delivery. The brief counts as "in the diff" only through committed changes, so the bare form keeps its derived-delivery or could-not-check verdict instead of exiting 0 with nothing checked.
