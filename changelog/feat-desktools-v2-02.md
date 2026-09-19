### Added
- `docs/streams/desktools-v2/seam-contract.md`: the one-page statement of the v2 seam
  contract — the four GitHub-fact classes (`gh` subprocess, hardcoded `"origin"`,
  `pullRequest`/`mergeRequest` GraphQL block, `api.github.com` host literal) and where each may
  legitimately appear (the two `Forge` backends and their tests, plus `forge.go`'s
  `GitHubAPIBase` for the host literal).
- `tools/desk/scripts/forge-ban.sh`: a portable (macOS + Linux) advisory counter for
  reach-around sites, covering `tools/desk/**`, `tools/cellctl/**`, `plugins/assay/**` and
  `statusgen/**` (the last of which is not under the existing `forgeban` register), reporting
  desk/statusgen counts separately and per class; `--baseline` records the total to
  `docs/streams/desktools-v2/forge-ban-baseline.txt`.
- `.github/workflows/forge-surface-control.yml`: an advisory step running the new counter
  alongside the existing shell-exec ban / no-passthrough / single-construction-site checks.
