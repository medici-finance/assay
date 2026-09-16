### Added
- `deskevidence` refuses an Evidence landing that would introduce a new `statusgen --lint`
  PROBLEM, diffed against the landing worktree before the change so a pre-existing red
  elsewhere in the repo never blocks a clean landing (exit 5, naming the PROBLEM lines).
- `deskevidence` refuses a landing whose target path resolves outside `docs/streams/` —
  catching a stray root-level file before it lands, not after.
- The `statusgen --lint` PROBLEM-diff guard fails closed on a root it cannot evaluate: when
  `statusgen` exits nonzero with no `PROBLEM:` line (a structural failure — no
  `docs/streams` tree, a stream dir with no `README.md`, an incomplete checkout) the landing
  is could-not-check (exit 6) rather than treated as clean, so the guard cannot silently
  no-op against the scratchpad/bare-cwd roots the landing path is often handed.
