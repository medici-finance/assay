### Fixed
- `deskpr create`/`update` now decide the repo and the push destination on what git itself
  resolves (`git remote get-url [--push] --all origin`: every config scope, `insteadOf` /
  `pushInsteadOf`, multi-valued lists) instead of a read of the repository config file alone.
  The push is refused (exit 5), before any token is minted, unless git's resolved push URL list
  is exactly one https URL naming the origin repo (or a local path). The refusal names each
  value, the config key, scope and file it came from, and a one-line worktree-scoped remedy — a
  disabled-push sentinel or a second, inherited `pushurl` value no longer fails late or pushes
  twice.
- `deskmerge` gates its fetch on git's resolved origin URL and its push on git's resolved push
  destinations, read in the scratch worktree the push leaves from; a multi-valued list or a
  destination naming another project is refused, in the dry run as in the real run.
