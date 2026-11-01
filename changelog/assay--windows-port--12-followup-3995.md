### Fixed
- The windows-port/12 Verify table now runs deterministically offline: each row asserts a named passing test or a tree-built binary instead of a vacuous `go test -run`, cross-compiles for Windows rather than executing a foreign binary, and the rows that need a native Windows host or a live forge identity are marked production-only.
