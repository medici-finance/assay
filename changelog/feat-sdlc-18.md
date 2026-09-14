### Added
- `statusgen --corroborate` now refuses to move a `gate: human` brief to `implemented`
  or `verified` while its decision issue is open with no recorded driver ruling — or
  absent altogether — distinguishing a genuine driver comment from a desk/bot relay of
  one. Deliberately scoped to the status transition only; the ready-flip is unchanged.
- `deskpr create` now requires a fixed, unmissable banner in the PR body of any PR
  delivering a `gate: human` brief, naming its decision issue and state.

### Changed
- `spec/lifecycle-v1.md` gains §4.5, documenting the new decision-gate transition block
  and its explicit non-extension into the ready-flip.
