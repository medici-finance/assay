### Added
- **`forge-neutral` brief 16 — `deskclose` widened lanes.** Specifies three narrowly-scoped
  additions to `deskclose`'s closed mode set, each staying inside the identity model
  `forge-neutral/13` already put a human gate on rather than opening a new one: (a)
  `self-withdraw`, letting an author-App close its own superseded-or-abandoned draft pull
  request, gated by a login-AND-numeric-id authorship pin mirroring the blessing-authority
  check; (b) `verify-gate-refire`, a `verifier`-role-only reopen+close cycle scoped to
  `verify-gate`-labelled issues, whose inability to complete the human sign-off is enforced
  independently and server-side by `verify-gate-close.yml`, not by this lane's own gate; and
  (c) documentation of `deskclose manifest` as the already-shipped, sanctioned path for a
  human-ruled batch close, with no behavior change. Adds one new `Forge` operation
  (`ReopenIssue`, both backends).
