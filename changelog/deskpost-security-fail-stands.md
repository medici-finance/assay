### Fixed
- **`deskpost ready` no longer flips a PR ready over a standing `Security-Review: fail`
  when a content-preserving head move launders the finding.** This is the companion to the
  `deskflip` fix (#529): the deskpost ready path carried the identical flaw. A
  `Security-Review: fail` is a retraction of the reviewed *code*, not of a commit sha, so a
  resync / merge-from-main / any re-trigger that leaves the flagged code byte-identical must
  not clear it. The security-verdict reduction (`securityVerdictStanding`, formerly
  `securityVerdictAtHead`) now keeps a fail **standing regardless of the commit it was posted
  against**: only a later `Security-Review: pass` **at the current head** — or a genuine
  content change a reviewer re-reviews and passes — clears it; a bare head-sha change clears
  nothing. A `pass` keeps its existing at-head binding (new code needs a fresh review), so
  the two verdict kinds are deliberately asymmetric across a head move — fail-safe in both
  directions.
