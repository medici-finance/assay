### Added

- Prepare passive Cell ownership, cumulative-budget and operator-incarnation records with offline negative fixtures; runtime enforcement and GE/16 completion remain pending.
- Add `CheckControlGrant` and `CheckNotRestored`, which refuse an executor grant that reads signing material and a current incarnation that reuses restored control state, per the GE/16 design ruling.
- Add `DecodeCellCheckpoint`, which refuses a checkpoint with an absent field, an unknown field or an unsupported schema, so a missing hold flag cannot read as not held.
