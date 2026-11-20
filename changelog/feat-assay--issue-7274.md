### Fixed

- `cellctl desk` launches now set `CLAUDE_CODE_TMPDIR` to the managed scratch task's work directory, so the claude harness's per-session scratchpads land inside the task record the scratch sweep retires instead of under the system temp directory. Codex command environments receive the same value.

### Added

- `internal/harnessscratch.Sessions` lists the session directories under a harness temp root, classifies each as live, ended or unknown from a durable record (roster beacon), reports a directory holding Git data as held, and removes ended ones only when asked to apply. Nothing schedules it yet.

### Changed

- The dispatch common clauses name the task `TMPDIR` as the only scratch location and forbid private Go caches.
