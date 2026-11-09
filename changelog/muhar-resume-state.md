### Added
- `muhar -state PATH` persists run-bound mutation observations and phase timings, refuses stale or concurrent reuse, and resumes completed definitive verdicts after interruption.
- Interrupting `muhar` now restores planted source and cleans up worker processes, workspaces and the per-run build cache before exit.
