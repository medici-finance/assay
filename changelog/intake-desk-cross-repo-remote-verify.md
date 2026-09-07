### Fixed
- **Cross-repo triage evidence now binds to the remote, not a bare sibling checkout** —
  `intake-desk`'s shared rules add an explicit clause: a triage/verification claim about
  another repo's current state must be resolved against that repo's remote (a forge read, or
  a sibling working copy fetched and SHA-confirmed current *this cycle*), never a local
  checkout read as-is. A stale sibling tree drifts arbitrarily far behind with no visible
  signal and a grep against it returns confident, precise, wrong evidence.
- `verify-desk`'s existing sibling-checkout rule is tightened the same way: "resync" now means
  confirmed-current (`HEAD` compared against `origin/main` after the fetch), because a silent
  `git fetch` failure leaves the tree exactly as stale as before — a mismatch is
  could-not-check for that row, never a row run against whatever the tree happened to hold.
