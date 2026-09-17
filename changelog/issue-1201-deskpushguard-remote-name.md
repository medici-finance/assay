### Fixed
- `deskpushguard`'s foreign-commit/merge-masquerade base check no longer hardcodes the remote
  name `"origin"` when resolving the pushed branch's base (`refs/remotes/<remote>/main`) or
  excluding a branch's own already-published commits. It now resolves the ACTUAL push-target
  remote from the pre-push hook's own `<remote-name>` argument (falling back to `"origin"` only
  when that argument is absent), so a worktree whose `origin` remote points at a different repo
  than the branch actually being pushed no longer has every genuine commit on the branch
  misreported as a "foreign commit dragged in from a sibling branch" against the wrong repo's
  history. (#1201)
