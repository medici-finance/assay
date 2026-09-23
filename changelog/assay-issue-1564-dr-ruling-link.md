### Added
- Design-decision records can carry a `ruling:` link to the human's ruling comment on the record's decision issue. `statusgen --corroborate` resolves the link through the forge and passes only when the comment exists, was never edited, names the record by its `DR-<slug>` id, sits on this record's decision issue in the same repository, and was written by a mapped human (not a bot). A ruling corroborates only the human who wrote it, never another name in the same `decided-by`. Each failure has a named reason, and `statusgen --lint` checks the link's shape (registers-v1 §7.5).

### Changed
- `statusgen --corroborate` now gates every decision record a pull request adds or edits. A `decided-by: "human:<name>"` placeholder with no resolvable `ruling:` link is MISSING-CORROBORATION (`placeholder-unratified`). Previously the placeholder was never read. Records already merged are not re-checked until a pull request edits them.
