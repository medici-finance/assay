### Fixed
- `deskfleet` now takes its default GitHub API base from the shared `deskkit.GitHubAPIBase` constant instead of restating the host string, so the command no longer adds a site to the forge reach-around count. A package test fails if the host string comes back as code in `deskfleet`'s non-test sources (#1864).
