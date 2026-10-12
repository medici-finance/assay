# Publishing an existing Evidence branch

Status: implementation scope; acceptance checks pending.

`deskevidence` can create commits through the forge Contents API. On GitHub these
commits carry the verifier App as author and GitHub as committer. Opening a draft
PR with `deskpr create` currently judges the whole base-to-HEAD range, including
these already-published commits, and refuses their committer identity.

The implementation must let a sanctioned Evidence branch become a draft PR while
preserving its commit objects and the publishing identity guard. It must establish
what the destination actually holds before excluding already-published commits.
A local tracking ref alone is not proof of the current remote branch head.

New commits introduced by a push must still satisfy the acting role's author and
committer rules. A forge-looking email alone must never exempt an ordinary local
commit. Missing, unreadable, mismatched, or divergent remote state must fail closed.
No identity override, raw-push fallback, reauthoring, or force push is in scope.

Acceptance checks:

- A branch containing only confirmed remote Contents API commits can open a draft
  PR without changing those commits.
- A valid new role-authored commit above that branch is checked and can publish.
- A new commit with a foreign author or committer is rejected before a write.
- An unpublished ordinary commit using a forge committer address is rejected.
- Wrong-repository, wrong-branch, stale, divergent, and unreadable remote-head
  evidence cannot silently suppress the identity check.
- Offline `--check` describes any remote-dependent proof it cannot establish.
- The same defect class is checked at sibling publishing call sites, with each
  sibling either covered by the change or explicitly scoped out with evidence.

Regression tests should reproduce the original refusal, include a second instance
of the same range error, and retain the existing negative identity cases.

Related range contract: <https://github.com/medici-finance/assay/issues/1967>.
