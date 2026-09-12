### Added
- Release notes now **credit the external contributor** a change came from. At each
  cut the aggregator resolves every `changelog/` fragment back to the pull request
  it arrived on — from git alone — and appends ` — thanks @<login>` to that
  fragment's bullets when the author is somebody the operator's roster does not
  already list. A maintainer, a mapped human or a role automation account is never
  thanked, a contributor can opt out with `<!-- changelog-credit: no -->` on a line
  of its own in the pull-request body, and a credit that cannot be resolved is a
  named line in the release log — never a refused cut. See `changelog/README.md`,
  "Credit in the release notes".
