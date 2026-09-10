### Fixed
- `deskflip` checks-green no longer stays permanently could-not-check on tokens that
  lack the `administration` scope. When the legacy branch-protection required-status-checks
  endpoint answers `403`, `deskkit.RequiredStatusChecks` now re-resolves the required set
  through two admin-free endpoints — `GET /repos/{o}/{r}/branches/{b}` (`.protected`) and, for
  a protected branch, `GET /repos/{o}/{r}/rules/branches/{b}` (each `required_status_checks`
  rule's contexts, which also closes the ruleset gap the legacy endpoint never covered). Only
  when both admin-free reads fail does the result stay could-not-check.
