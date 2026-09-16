### Added
- `deskevidence` refuses an Evidence landing that would introduce a new `statusgen --lint`
  PROBLEM, diffed against the landing worktree before the change so a pre-existing red
  elsewhere in the repo never blocks a clean landing (exit 5, naming the PROBLEM lines).
- `deskevidence` refuses a landing whose target path resolves outside `docs/streams/` —
  catching a stray root-level file before it lands, not after.
