### Fixed
- `verifyPassHeldContradiction` (the `**VERIFY: PASS**`/`HELD` contradiction check
  introduced in PR #1304) no longer launders a same-line, un-routed
  `HELD`/`could-not-check` mention that trails a genuinely routed one. Routing is now
  bound to each `HELD`/`could-not-check` occurrence individually — a routing keyword
  and reference must occur at or after that occurrence's own position — instead of to
  the line as a whole, closing the same proximity-laundering class PR #1244 closed in
  the predecessor `entryIsHeld` mechanism. (Issue: #1444)
