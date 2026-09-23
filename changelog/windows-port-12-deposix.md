### Changed
- De-POSIXed the desk-role skill prose: `pr-review-desk`, `pr-shepherd`, and
  `worker-desk` now name the scratch-file and config-home MECHANISMS
  (`desk-shell.md` §Scratch files, §Config home) instead of spelling
  `mktemp`/`/tmp/`/`~/.config` literally, so a dispatched session on a
  non-POSIX shell has a mechanism to follow rather than a command to improvise.
  `deskboard` gained an `--out <path>` flag (a portable substitute for a shell
  `>` redirect) and `reviewloop plan` accepts a no-op `--dry-run` flag.
- Added `deskpushguard hook-install`, which writes the pre-push hook shim(s)
  into `.githooks/` resolving the guard binary from PATH instead of a
  hardcoded `/opt/desk-tools/bin` literal, and writes the `pre-push.cmd` pair
  on a Windows target. `make desk-hook-install` and
  `scripts/build-windows.ps1`'s `Target-DeskHookInstall` now call it.

### Fixed
- `deskrelease` no longer hardcodes `/opt/desk-tools/bin/desktoken`; it
  resolves the co-located `desktoken` binary next to the running `deskrelease`
  binary, falling back to a `PATH` lookup only when no co-located binary is
  found.
- `tools/skillslint` gained an advisory `posix-token` NOTICE row (never
  exit-affecting) that flags a skill body spelling `mktemp`/`/tmp/`/`~/.config`
  literally outside a fenced "unix example" block, so the fix above does not
  silently regress.
