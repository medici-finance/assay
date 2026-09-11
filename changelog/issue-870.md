### Fixed
- Verify rows across `harness-portability`, `forge-gitlab`, `statusgen` and `desk-tools` now
  resolve as literally written from the repo root. The rows shelled into per-tool Go modules
  root-relatively (`go run ./tools/freshness`, `go test ./tools/desk/internal/deskkit/`,
  `go test ./statusgen/`), which resolved only in a tree carrying a root `go.work` — the
  published tree carries none, so every such row died on `go: go.mod file not found` and had to
  be hand-substituted at verification time. Each row is now module-scoped
  (`cd <module> && GOWORK=off go …`, or a `go build -C <module>` whose binary runs from the root
  so path arguments keep their repo-root meaning), matching the form CI already uses. The
  explicit `GOWORK=off` makes a row behave identically in a tree that has a `go.work` and one
  that does not, so this class of breakage cannot pass in one tree and fail in the other again.
