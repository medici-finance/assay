### Changed
- `TestHiddenReadingsStopAtBudget` judges a race build against a 192 MiB allocation ceiling of its own instead of skipping the check, so a reading that ignores the spent budget fails under `go test -race` as well as in an ordinary build.
