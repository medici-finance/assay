# Publishing an existing Evidence branch

`deskevidence` can create commits through the forge Contents API. On GitHub these
commits carry the verifier App as author and GitHub as committer. `deskpr create`
can open their first draft PR without reauthoring those already-published objects.

Before the push, create lists the exact destination branch through the same
credentialed canonical endpoint used by its in-process push. It checks only
`<source-hash> ^refs/remotes/origin/<base> ^<confirmed-tip>`. A local tracking ref or a PR
lookup is never the evidence for that tip. A confirmed absent branch means the
whole base-to-HEAD range is new and must pass the identity check.

For an existing branch, an unreadable or unfetched head, a divergent history, or
an invalid branch response refuses. The proof binds the destination URL, branch,
and source commit. Create first requires the named push source to match the
worktree HEAD, then passes that immutable hash through identity enumeration,
remote-tip ancestry, proof and push. Moving HEAD cannot substitute a different
identity-check snapshot. The push requires the existing remote ref to remain at the
confirmed tip, including when the remote rewinds after checking. It remains a
plain fast-forward push; this precondition grants no force capability.

Every new commit still needs the acting role's exact author and committer identity.
An ordinary unpublished commit with a provider-looking address is not exempt.
Existing forge-merge rules are unchanged. No override, reauthoring, or raw-push
fallback is added.

`create --check` opens no connection, conservatively checks the whole range, and
reports that the remote branch head was not checked. It can therefore refuse a
branch that a real create admits after confirming its published history.

## Sibling paths and regression evidence

- `deskpr update` already narrows against the live PR head; its offline estimate
  and named-PR destination checks remain unchanged. Its live identity gate and
  push also share one immutable source hash; a changed source refuses. Existing mixed-author,
  stale-tracking-ref and foreign-commit tests cover this sibling.
- `deskevidence` checks the worktree from which it derives witness attribution;
  its base is already its landing branch. It does not publish a worktree branch
  through `deskpr create`, and continues offering no remote-tip exclusion. Its gate snapshots HEAD once.
- The existing caller inventory and planted second-call-site control cover every
  publish-identity caller. The create proof is now recorded in that inventory.

`tools/desk/cmd/deskpr/evidencepublish_test.go` covers unchanged provider-committed
history, new role commits, foreign author/committer identities, stale local refs,
wrong repository/branch, unreadable/divergent/unfetched state, proof mismatches,
source/HEAD disagreement, snapshot changes during identity enumeration, the sibling
update source binding, and a remote rewind from the pre-push hook. Its reproducible negative mutations
are in `tools/desk/cmd/deskpr/evidence-mutations.json`.

Related range contract: <https://github.com/medici-finance/assay/issues/1967>.
