### Added

- Prepare passive Cell ownership, cumulative-budget and operator-incarnation records with offline negative fixtures; runtime enforcement and GE/16 completion remain pending.
- Add `DecodeCellCheckpoint`, which refuses a checkpoint with an absent field, an unknown field or an unsupported schema, so a missing hold flag cannot read as not held.
