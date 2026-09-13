### Fixed
- `deskflip`'s already-ready fast path no longer treats "not a draft, queue label already
  correct" as proof the review verdicts are current. It now re-runs the FULL condition gate —
  including the reviewer-approved and security-verdict lanes AT THE CURRENT HEAD — before
  reporting "nothing to do", and refuses (naming the stale lane and both the stale-verdict
  commit and the PR's current head) when a lane's last-seen verdict is behind the head.
  Before the fix, an already-ready PR could sit reading as mergeable indefinitely while new
  commits landed with content no review lane had seen. (#987)
