### Fixed
- On GitLab, a `deskpost review --verdict approve|request-changes` verdict is now VISIBLE to
  `ReviewsAtHead` and to `deskflip`'s `reviewer-approved` gate. The write side already landed
  the verdict's reasoning as an MR NOTE carrying a `Verdict: approve|request-changes` line
  (an approve also POSTs a GitLab approval; a request-changes has no native GitLab review
  object at all), but the read side classified every non-system note as `COMMENTED`, so the
  gate reported "no APPROVED/CHANGES_REQUESTED correctness verdict" over a verdict that had
  really been rendered — the write and the read disagreed on the object (#798). `ReviewsAtHead`
  now reduces a verdict note to the review STATE a GitHub review of the same verdict reports
  (`APPROVED` / `CHANGES_REQUESTED`) via the new canonical `deskkit.CorrectnessNoteState`
  reader, so both sides agree on the same channel — the note — on GitLab CE and EE alike (where
  the approval object may not be head-pinned) and for request-changes, which has no approval
  object to read. The reducer mirrors the security-marker fence asymmetry (a fenced/quoted
  `Verdict: approve` is not a grant; a fenced `Verdict: request-changes` still blocks) and is
  identity-free — consumers still filter to the reviewer App login before acting.
