### Added
- `cellctl cache CELL wipe` clears inactive build/module caches regardless of cache pressure.
- `cellctl reset CELL` stops owned consumers, clears service state and caches, and removes cell worktrees while preserving messages and branches. `--hard` discards messages and refreshes every CELL_ROOTS repository, removing linked worktrees and non-main local branches. Both preserve cell configuration and signing keys.
- `cellctl comms CELL init` provisions private signing/trust material and a pinned contained Codex reader using an existing login.
- Each cell uses its own Herdr workspace for role and comms tabs, with ownership checks for reuse and closure.
- A two-session hello integration test verifies bidirectional messages, sender/payload preservation, repeatable polling and durable acknowledgment through the shipped comms binaries.

### Fixed
- Managed cache lock admission allows time for full cache scans instead of timing out before a normal measurement completes.
