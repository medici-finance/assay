### Fixed
- `cmd/deskmerge`'s test suite now catches the "fast-forward instead of forcing a merge
  commit (`--no-ff` dropped)" mutation directly at the trial-merge step (`assess.go`),
  closing the shard 1/3 `NOT_CAUGHT` gap `#982` surfaced once the mutation-gate harness
  itself was fixed. (#979)
