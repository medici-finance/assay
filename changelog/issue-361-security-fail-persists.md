### Fixed
- **`deskflip` no longer flips a PR ready over a standing `Security-Review: fail` when a
  content-preserving head move launders the finding (#361).** A `Security-Review: fail` is a
  retraction of the reviewed *code*, not of a commit sha, so a resync / merge-from-main / any
  re-trigger that leaves the flagged code byte-identical must not clear it. The security-lane
  reduction (`securityVerdictStanding`, formerly `securityVerdictAtHead`) now keeps a fail
  **standing regardless of the commit it was posted against**: only a later
  `Security-Review: pass` **at the current head** — or a genuine content change a reviewer
  re-reviews and passes — clears it; a bare head-sha change clears nothing. A `pass` keeps its
  existing at-head binding (new code needs a fresh review), so the two verdict kinds are
  deliberately asymmetric across a head move — fail-safe in both directions. The explicit-fail
  rule already blocked risk-classed and non-risk-classed PRs alike; this fix is what makes that
  rule reachable after the head moves.

### Changed
- **`deskflip` risk-classification is now unioned across visibility, the security-surface
  label, and the changed-path triggers (#361 item 3), never path alone.** A PR carrying
  `surface:core` is risk-classed — and therefore requires a `Security-Review: pass` at head —
  even when none of its changed paths hit the compiled trigger set. The label term is
  **additive only**: its presence can only ADD scrutiny and its absence never waives the gate,
  so the fail-open direction `riskpath.go` warns of (a label read that could *waive* the gate
  on a mislabeled PR) is not reachable — only the fail-closed, tightening direction is used.
