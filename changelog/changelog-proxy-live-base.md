### Fixed
- `changelog-check` now reads a proxy fragment from the live tip of the PR's base branch instead of the base commit GitHub recorded when the PR was opened. That recorded sha never advances, so a `changelog/pr-<N>-<slug>.md` landed on the base branch *after* a fork PR opened — the only case the proxy path exists for — was invisible and the PR stayed red. (#923)
