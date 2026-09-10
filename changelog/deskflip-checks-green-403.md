### Fixed
- `deskflip` checks-green no longer stays permanently could-not-check on tokens that
  lack the `administration` scope. When the legacy branch-protection required-status-checks
  endpoint answers `403`, `deskkit.RequiredStatusChecks` now re-resolves the required set
  through admin-free endpoints — `GET /repos/{o}/{r}/branches/{b}` (`.protected`) and, for a
  protected branch, `GET /repos/{o}/{r}/rules/branches/{b}` (each `required_status_checks`
  ruleset rule's contexts). Being a flip gate it fails CLOSED: only a positively unprotected
  branch is read as empty/green; a protected branch whose required set cannot be determined
  admin-free (classic branch protection, invisible to the rules API, or an unreadable endpoint)
  stays could-not-check rather than being read as green.
