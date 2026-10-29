### Fixed
- `cellctl check` on a gitlab house cell reports the gh config link `n/a` (as on a k8s gitlab cell) instead of a `MISS`, and `cellctl new --kind house` honours `--forge gitlab`: it writes `CELL_FORGE=gitlab` with the GitLab endpoint and never links a gh config. An unknown `--forge` on a house cell is now refused (#4817).
