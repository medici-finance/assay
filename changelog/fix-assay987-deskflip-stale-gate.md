### Fixed
- `deskflip`'s already-ready fast path no longer treats "not a draft, queue label already
  correct" as proof the review verdicts are current. It now re-runs the FULL condition gate —
  including the reviewer-approved and security-verdict lanes AT THE CURRENT HEAD — before
  reporting "nothing to do", and refuses (naming the stale lane and both the stale-verdict
  commit and the PR's current head) when a lane's last-seen verdict is behind the head.
  Before the fix, an already-ready PR could sit reading as mergeable indefinitely while new
  commits landed with content no review lane had seen. (#987)
- The stale-verdict refusal's diagnostic commit now names the SAME decisive security review
  that governs the pass/fail decision, never a later off-head review that never actually
  governed anything. A standing `Security-Review: fail` at commit A followed by a LATER
  `Security-Review: pass` pinned at an off-head commit B previously made the refusal claim
  the verdict was "pinned at B" — a plain staleness framing — when the real reason to refuse
  was the standing fail at A. `securityVerdictStanding` and `lastSecurityVerdictCommit` now
  share one reduction so they can no longer name different reviews. (#988)
