### Added
- `loopadmin/process` now has a test pinning that a launch with a nil environment starts the child with an empty environment and inherits nothing from the parent (graph-execution/40 residue).
- `docs/streams/decisions/DR-process-extract.md` proposes the design record for the process-runner move (one runner behind a mandatory host-custody port). It awaits a human ruling, and graph-execution/40 now cites it with `design:` and carries its `## Human decision` section.
