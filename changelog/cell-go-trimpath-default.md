### Changed
- Managed cell Go caches now build with `-trimpath` by default (`GOFLAGS`, set with `GOCACHE`), so the same code built in each new dispatch worktree reuses the shared compiler cache instead of compiling and storing it again under a path-specific key. A cell opts out with `CELL_GO_TRIMPATH=off`; an ambient `GOFLAGS` is extended, never replaced.

### Fixed
- `metrics-harvest` finds its default `domains.yaml` when built with `-trimpath`.
