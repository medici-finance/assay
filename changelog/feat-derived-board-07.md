### Added
- `statusgen reconcile --backfill` — the declared, reviewable history-only
  fallback promised by the v1.0.0 release note: when no PR carries a `Brief:`
  trailer, a merged PR whose branch name or body names the brief in
  `<stream>/<NN>` or `<stream>-<NN>` form counts as a witness, tagged so it
  reads apart from a real trailer link. A hand-asserted
  implemented/verified/done with neither renders `unknown` naming the
  hand-asserted state and commit — never a silent demotion to `todo`.
- `statusgen reconcile --backfill --report` additionally writes
  `docs/streams/board-drift-<date>.md`: one row per brief where the last
  hand-edited (pre-generation) README cell disagrees with what the run
  derives, for a human to resolve by linking the PR or accepting the
  demotion.
