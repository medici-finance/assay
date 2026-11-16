### Fixed
- `TestHiddenReadingsStopAtBudget` no longer fails under `go test -race`: its allocation ceiling is judged only in an uninstrumented build, where the race detector's own allocations are not counted. The refusal assertion still runs under `-race`.
