### Added

- `cellctl new --kind container` registers an existing container launcher for `ls`, `check`, `desk`, coordinator-only `up`, and `down`. Calls retain model-pin checks and exclude inherited forge/model credentials from the launcher's environment; no host worktrees or credential symlinks are created.
