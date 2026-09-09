### Fixed
- `scripts/build-windows.ps1` now parses and runs under Windows PowerShell 5.1
  (`powershell.exe`), not only `pwsh` 7+. A `>>>` inside a `Write-Host` string
  that 5.1 lexes as a redirection operator is gone, and every em-dash (which
  5.1's default non-UTF-8 encoding mangled in error/log strings) is now an ASCII
  `--`, so the documented Windows rebuild path works on a host that has only
  `powershell.exe` (#678).

### Added
- `tools/winparity` gained a second, fail-closed assertion alongside the target
  parity check: `scripts/build-windows.ps1` must be Windows PowerShell 5.1-clean
  (ASCII-only, no `>>>` in strings). The scan reads the file as bytes, so a
  regression reddens on the Linux CI leg — no PowerShell needed — before it
  reaches a native Windows host. The Windows script runs the same guard as a
  preflight (#678).
