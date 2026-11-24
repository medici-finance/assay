### Fixed
- `deskboot --repo` now reaches the `deskroster preflight` step as well as the token mint, so the envelope check and the mint resolve the same repository and forge. Before, an explicit `--repo` naming a GitLab-mapped repository could not correct a preflight that fell back to the GitHub default.

### Changed
- Security control: the command-priority git config allowlist (`GIT_CONFIG_COUNT`) now also qualifies exactly `core.quotePath=false` on its own, so a harness that sets only that key no longer makes the scoped remote read refuse. Every other key, value, spelling or combination still refuses.
