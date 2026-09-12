### Added
- `desktoken` mints a seventh role, `cell-issues` — the write-issues App identity
  (`issues:write` + `metadata:read` only), selectable only by explicit name; no loop's
  default identity changes.

### Changed
- Nothing widens: the App's grant is fixed server-side and untouched by this change, and the
  loop→role table still resolves no window to the write App implicitly.
