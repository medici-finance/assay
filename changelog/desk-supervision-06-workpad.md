### Added
- `deskreply --workpad` upserts ONE marked progress comment per PR — finds the newest
  unresolved comment authored by the worker identity carrying the `<!-- assay:workpad -->`
  marker and edits it in place, or creates the first one; `--dry-run` reports which without
  writing. Never edits a human's or a minimised comment.
- `internal/deskkit/workpad.go`: the marker, the fixed-section template (`Render`/`Parse`),
  and `Stamp(worktree, sha)` for the environment-stamp line — never a machine path.
- The worker prompt kit (`common-clauses.md`) now carries the workpad rule: keep one
  workpad per PR, no separate done/summary comments.
