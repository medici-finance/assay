### Added

- `deskdispatch --context-snapshot DIR --context-receipt FILE` rechecks an implementer dispatch's context receipt against the snapshot before the claim, using the same resolver as `statusgen context recheck`. A current receipt, an unrelated edit or a product change to assess is admitted and carried into the worker prompt's `## Context pickup receipt` section. A conflict to reconcile, an unavailable input or partial coverage holds the dispatch (exit 5) before any claim or worktree exists. A receipt schema the build cannot validate is refused and names the manual bootstrap.

### Changed

- The `author-spec` and `author-brief` skills hand changed planned work over through `statusgen context discover` and `statusgen context check`: a context receipt, candidate brief patches and an `amendment-manifest-v1` file, checked before the brief PR opens. Both state the supported releases (v1.0.55 for the base verbs, v1.0.63 for the product-bound `context-handoff-v3`) and keep the manual bootstrap for an install without them. `worker-desk` and the implementation pickup reference point at the same procedure, and `docs/context-handoff.md` gains a compatibility and migration section.
