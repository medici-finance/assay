### Fixed

- `statusgen --scan-issues`, `--transcribe-scan` and `--transcribe-scan-delta` now write the `issue-loop` stream `README.md` when they open that stream in a repo that does not have it yet, so the first scan's tree passes `statusgen --lint`. The README uses the same frontmatter and brief-table format as the starter stream from `statusgen init`. An existing README is never rewritten.
