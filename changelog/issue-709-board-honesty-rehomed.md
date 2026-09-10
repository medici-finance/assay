### Fixed
- `statusgen` board-honesty: the `NON-DISPATCHABLE (re-homed)` NOTICE now keys on
  the ROW's own re-home marker (`[homed→…]`, `deliverable-repo:`, or explicit
  do-not-re-implement wording in the brief cell), never on a stream-level README
  inference. A stream whose README merely mentions re-homing no longer flags every
  file-less `todo` row as re-homed (statusgen #709).
