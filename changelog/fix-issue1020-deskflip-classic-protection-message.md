### Fixed
- `deskflip`'s checks-green could-not-check, when a branch is protected but the admin-free
  rules API names no required contexts (the tell of CLASSIC branch protection, invisible to
  that API), now names the exact permission gap: the calling App token lacks
  `administration: read`, the permission the legacy branch-protection endpoint requires to see
  classic protection's required-checks list. Before the fix the message described only the
  mechanism and never the fix, so every draft PR on a classically-protected repo
  (`assay-site`, `assay-toolkit`, `medici-platform`, `assay-console`) read as an ordinary
  could-not-check and was re-litigated by a human on every flip attempt instead of being
  escalated once as a permission grant. `RequiredStatusChecks` itself was already correct —
  it tries the legacy endpoint first and uses it directly whenever it is readable (#760); a
  new fixture pins that direct-success path so a future change cannot regress it once the
  permission is granted. (#1020)
