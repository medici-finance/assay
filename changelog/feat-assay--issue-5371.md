### Added

- `cellctl gc <cell> [--role <role>]` reports what each existing reclaim target of a desk would reclaim (the scratch sweep, the Go cache's recorded measurement, and `deskwt prune` for each stream root), one JSON line per target. It is read-only; a failed target does not stop the rest. Its state-changing forms, `gc --run` and `gc --apply <plan-id>`, follow cell-op-v1 (refused off a terminal, typed confirm on one), and their apply is held: a confirmed apply reclaims nothing until cell-op-v1 specifies a non-interactive maintenance class.
