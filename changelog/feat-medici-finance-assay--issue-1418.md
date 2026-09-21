### Fixed
- `statusgen verifyrun` no longer records a Verify row as a false `fail` when the shell
  itself never started. On native Windows, `bash.exe` on PATH is often the WSL launcher; with
  no WSL distro installed it exits 1 with `execvpe(/bin/bash) failed` *before* the row's own
  command runs. verifyrun now resolves and probes the shell once per run and, when no
  pipefail-capable POSIX shell can be started, records the affected rows as **could-not-run**
  (with the reason) rather than `fail` — so a shell-bootstrap failure is never mistaken for a
  genuine product-check failure that a human then has to roll back.

### Added
- `statusgen verifyrun` selects a platform-appropriate shell when the default `bash` cannot
  bootstrap: an explicit `ASSAY_VERIFY_SHELL` override first, then Git-for-Windows bash at its
  well-known install paths, and could-not-run (never a silent pass) if neither works.
  `pipefail` semantics are preserved on whatever shell is finally used — a candidate that does
  not support `-o pipefail` is treated as unusable — and it never falls back to PowerShell or
  `cmd`. On Linux/macOS a working `bash` on PATH runs every row exactly as before, at the cost
  of one cheap probe per run.
