### Added
- `muhar -state PATH` persists run-bound mutation observations and phase timings, refuses stale or concurrent reuse, and resumes completed definitive verdicts after interruption.
- Interrupting `muhar` now restores planted source and cleans up worker processes, workspaces and the per-run build cache before exit.
- Resumable state also binds the running executable; directly observed Unix SIGKILL/SIGTERM exits remain infrastructure errors, and selection/schemata timing records describe the commands actually executed.
