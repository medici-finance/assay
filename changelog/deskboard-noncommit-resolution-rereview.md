### Fixed
- `deskboard` now re-flags a PR for **RE-REVIEW** when a standing `CHANGES_REQUESTED`
  at the current head is followed by a finding-relevant **non-commit** resolution — a
  `*:skip` resolution label added, or the PR body/title edited — after the last review.
  The re-review trigger was keyed on the head SHA alone, so a fix that changed no commit
  (a label add, a `body` edit) left the row `BLOCKED` indefinitely, invisible to the desk
  until a human flagged it. The head-sha trigger is unchanged for the common case; the
  label-add time is read from the `labeled` timeline events and the body-edit time from
  `lastEditedAt` (which moves only on a title/body edit), both compared against the last
  review's submitted time, so an unrelated update does not re-flag. The signal is
  self-limiting — once the re-review posts, its verdict time is newer than the label/edit —
  and a suspected forged no-op flip still takes precedence. The reviewer must still verify
  the check state at head, since a label/body edit does not re-run CI.
