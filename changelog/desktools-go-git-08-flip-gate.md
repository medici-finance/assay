### Changed
- `tools/desk/scripts/count-git-exec.sh` is now a ratchet: it fails when the desk tools' git-exec site count grows above the pinned count in `tools/desk/scripts/git-exec-pin.txt` (205), fails closed on a missing or garbled pin, and gains a `--count` mode. A `go test` in the required build job enforces the pin and re-measures the PR's base with the same script.

### Fixed
- The git-exec counter's exclusion of the audited `internal/gitexec` home never matched, because grep prints relative paths; the audited spawn was counted. The exclusion is now anchored, and so is the `*_test.go` exclusion, which also matched that text inside a production line's content and dropped the site.

### Added
- A test asserting go-git stays at or above v5.17.1 (CVE-2025-21613 and CVE-2025-21614, fixed in v5.13.0; CVE-2026-33762, fixed in v5.17.1) in both the desk tools and gitcore modules (a prerelease or pseudo-version of v5.17.1 is refused), and Dependabot update alerting for the go-git family.
