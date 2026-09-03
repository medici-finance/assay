### Added
- **`desksupervise`** — the liveness *observer* that finally supplies `internal/loopengine`'s
  fully-coded, fully-inert liveness taxonomy (`ObservableProbe`, `LivenessPolicy`) with real
  probes. `internal/loopengine/probes.go` adds `AuditProbe`, `BranchProbe`, `PRProbe` (each
  three-state — a probe that cannot reach its source reports could-not-check, never no-life),
  composed by `HouseProbes()`, plus `ClassifyLiveness`/`Disposition`, the taxonomy re-exported
  for a reader outside the engine's own in-flight tracker. `desksupervise tick` classifies
  every `state=dispatched` dispatch claim into `ALIVE` / `NEVER-STARTED` /
  `HEARTBEAT-EXPIRED` / `OVER-WALL-CAP` / `COULD-NOT-CHECK`, releasing a wedged claim
  (`RECLAIM-ELIGIBLE`) or landing a budget-blowing one `BLOCKED-TIMEOUT` (a filed
  `help wanted` issue, never re-dispatched blind) — turning a worker stuck behind the
  120-minute stale-claim backstop into a logged, minutes-scale reclaim with no human in the
  loop. `--dry-run` classifies and prints only; `run --interval` loops `tick` forever,
  mirroring `deskwt prune --interval`. `--claims-fixture`/`--observations-fixture` bypass the
  live claim tool and the forge/audit file entirely, so the whole classification path runs
  offline. `deskkit.PullRequest` gains an `UpdatedAt` field (GitHub and GitLab both wired) as
  the forge read PRProbe needs. See `tools/desk/README.md`'s tool-reference row and
  `docs/streams/desk-supervision/brief-01-observable-probes-and-observer.md`.
