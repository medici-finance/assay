### Changed
- `tools/desk/scripts/count-git-exec.sh` is now a ratchet: it fails when the desk tools' git-exec site count grows above the pinned count in `tools/desk/scripts/git-exec-pin.txt` (205), fails closed on a missing or garbled pin, and gains a `--count` mode. A `go test` in the required build job enforces the pin and re-measures the PR's base with the same script.

### Fixed
- The git-exec counter's exclusion of the audited `internal/gitexec` home never matched, because grep prints relative paths; the audited spawn was counted. The exclusion is now anchored.

### Added
- A test asserting go-git stays at or above v5.13 (CVE-2025-21613, CVE-2025-21614) in both the desk tools and gitcore modules, and Dependabot update alerting for the go-git family.
