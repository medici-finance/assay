### Added

- `deskevidence` lands a witness row carrying `(reads sha256:<hex>)` in one commit with its read-set file `docs/streams/<stream>/reads/<hex>`. The file comes from the run's state directory under the git dir, never from the worktree, and is copied before any forge call. If the file is absent there or hashes to another digest, the landing is refused before any write. On GitHub this is one `createCommitOnBranch` commit; on GitLab it is one multi-action commit. Read-set judgement is still off (`readSetRecorderShipped = false`).
- `gitcore.GitDir` answers `git rev-parse --absolute-git-dir` without spawning git. `deskevidence` resolves the read-set state directory with it, and reads each state file with `O_NOFOLLOW|O_NONBLOCK`, checks the open descriptor is a regular file, and refuses one over 16 MiB.

### Changed

- `tools/desk/scripts/count-git-exec.sh` also counts call sites of deskkit's verifier read seam (`verifierGit(`, `verifierGitBytes(`, `VerifierGitPath(`). The pin moves from 205 to 246 because the measurement widened: `main` reads 246 by the same script, so no site was added.
