### Fixed
- `TestHiddenReadingsStopAtBudget` no longer fails under `go test -race`. A race build's `sync.Pool` keeps fewer of the regexp matchers put back into it, so the reading rebuilds them and allocates far more than an ordinary build; the 32 MiB ceiling now binds only an ordinary build, and the refusal assertion runs in both.
