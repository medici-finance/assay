### Fixed
- `statusgen` evidence coverage judges a reused witness by the content of the paths it speaks for, never by commit ancestry: a witness written on a squash-merged branch is no longer refused as `wrong-revision` when nothing it depends on differs, and a witness commit missing from the clone is `could-not-check`, naming why.

### Added
- A brief MAY declare `verify-depends:` in `## Context` — the paths its Verify rows read — so a `verified` row survives an unrelated release-bookkeeping change on main. An empty, unparseable or unresolvable declaration fails closed (`could-not-check`); there is no override.
