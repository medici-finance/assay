### Added
- `tools/gremlins/run.sh` runs a daily Gremlins mutation sweep over the desk tools' muhar-spec packages as discovery, never a gate: it installs a pinned Gremlins verified against the checksum database, rotates each package's files so the run fits a per-package budget on a 2-CPU runner, and emits Gremlins JSON.
- `gremlintriage` cross-references surviving mutants against the muhar specs and writes a per-package Markdown summary (killed, lived, not covered, timed out) listing survivors in code no spec covers as candidate guards with file:line.
- Staged workflow `ci/staged-workflows/gremlins-daily.yml` (pending maintainer activation) runs the sweep daily, writes the job summary and uploads the JSON artifact, and fails only when the sweep itself broke.
