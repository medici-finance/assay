### Fixed
- `scanloop run --dry-run` no longer advances the inbound poller's per-repo baselines. A live dry-run now polls a throwaway copy of the state dir, removed when the pass ends, so the preview still reports the real delta and the next real `run` still sees it. If the copy cannot be made, the pass is refused with exit 5 before anything is polled. It never falls back to the real state dir.
