### Fixed
- `statusgen --auto-flip-model` no longer refuses a verified brief because of its Evidence PR. A PR whose diff only changes verification records is never the delivering PR. That covers `## Evidence` lines, a README's Status/Verified/Reviewed cells, verify-outcome records and `STATUS.md`. The rule is judged by diff shape, whoever authored the PR. A PR whose patch or head content could not be read still needs its own approval.
- When a brief declares `files:`, the delivering PR must touch one of those paths. A newer PR carrying the same `Brief:` trailer but touching none of them is not credited. When no PR touches the files, the flip reports `COULD-NOT-CHECK: no PR touches the brief's files` and does not flip.

### Added
- `STATUS.md` has a **Stuck auto-flips** roll-up table listing every gate:model brief left at `verified`. `statusgen --auto-flip-model --check` exits 2 when any candidate is REFUSED or COULD-NOT-CHECK, so a stuck row fails the run visibly. See `docs/board-stuck-autoflips.md`.
