### Fixed
- `tools/ci-load/activation/ci.yml`, `assay-statusgen.yml`, and `evidence-automerge.yml`
  refreshed against current `.github/workflows/` so the staged trigger/concurrency edit no
  longer silently reverts three independent fixes already landed on `main` (the `tools/desk`
  `go test ./...` leg, `fetch-depth: 0` on the statusgen lint checkout, and the
  evidence-automerge script-based refusal decision + default-branch checkout). `ci-load.diff`
  regenerated to match. (#1187)
