### Added

- `deskevidence` lands a witness row carrying `(reads sha256:<hex>)` in one commit with its read-set file `docs/streams/<stream>/reads/<hex>`. The file comes from the run's state directory under the git dir, never from the worktree, and is copied before any forge call. If the file is absent there or hashes to another digest, the landing is refused before any write. On GitHub this is one `createCommitOnBranch` commit; on GitLab it is one multi-action commit. Read-set judgement is still off (`readSetRecorderShipped = false`).
