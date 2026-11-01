### Fixed
- `changelog-check` no longer cancels a run in progress when a label it does not read is added or removed. Before, a label swap (such as a ready-flip) left a cancelled check-run on the unchanged head, shown as a red X after a later run passed. Pushes and `changelog:skip` label changes still cancel the run in progress.
- `changelog-check` now runs on a GitHub-hosted runner instead of the shared self-hosted pool, so a label change no longer waits behind full CI builds before the PR can merge.
