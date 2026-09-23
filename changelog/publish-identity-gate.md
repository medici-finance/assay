### Added
- A **publish-identity gate** now refuses, at the push boundary, to publish a commit whose
  author or committer is not the session role's bound bot identity. `deskpr create`,
  `deskpr update` and `deskevidence` run it before any network write (and `deskpr --check`
  reports it as a local gate): every commit in `refs/remotes/origin/<base>..HEAD` must be
  authored **and** committed by the role's identity — a GitHub bot-USER-id noreply address,
  a GitLab service-account noreply shape, or a trusted GitLab session address — or the write
  is refused (exit 5) naming the commit, the identity found, the identity expected, and the
  remedy. Forge-created merge commits (GitHub's "Merge pull request" / "Update branch") are
  exempt; there is no override flag. This is the push-time layer that stops a worktree with a
  stale `user.*` (a shared checkout, a manual `git worktree add`, an editor's git) from
  publishing commits attributed to the wrong actor, however the worktree acquired its
  identity.
