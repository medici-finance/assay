### Fixed
- `deskboard`'s READ path now authenticates as the session role's GitHub App
  installation instead of falling through to the HOME keyring: every `gh` call is
  handed the cached installation token for the account the call targets, resolved once
  per account. Under a desk config home that is not the operator's own, the reads
  previously came back 401 for every private repository — and on the GraphQL path an
  unusable keyring account can return a bogus rate-limit error or an empty result, an
  absence that reads like an answer. A session with no loop identity keeps the ambient
  credential and says so on stderr; the outward verbs take the opposite rule.
- `deskboard prs` reports a watched repository the App installation cannot resolve as
  per-repo could-not-check (`repoCoverage` in the JSON, a `COULD NOT CHECK` line on the
  table) instead of failing the whole sweep. That failure is permanent for such a repo,
  so aborting on it cost the board every other repo's rows. Every other read failure —
  401, rate limit, timeout, parse — still fails the run closed.
- `deskflip` refuses (exit 5, naming the role and the token path) when the review role's
  App installation token cannot be minted or read, and every forge call it makes runs
  under that token. It previously proceeded on the operator's ambient `gh` login, so the
  ready-flip and its queue labels were written under a human identity and read afterwards
  as a human decision.
- Every outward desk verb (`deskflip`, `deskpost`, `deskpr`, `deskreply`, `deskfile`,
  `deskevidence`) refuses when `$DESK_LOOP` is unset, in the same words `deskboot` already
  used. With the variable unset a `STOP.<loop>` flag a human is holding matches nothing, so
  the halt silently failed and the verb kept writing.

### Changed
- The dispatched REVIEWER kit now requires every path-existence claim to be resolved in the
  PR's OWN repository at the PR head, to name the tree it was resolved against, and to be
  reported as could-not-check when it cannot — a reviewer that checked paths in the
  dispatching desk's checkout reported files as missing that were present in the PR's
  repository.
- The loop-to-App-role table moved into `deskkit`, so the identity a window presents and
  the identity its calls carry are read from one place.
