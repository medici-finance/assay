### Fixed
- `statusgen verifyrun` now runs a `cmd`-shell Verify row under a raw Windows
  command line built as `cmd /d /s /c "<row>"`, instead of letting `os/exec`
  escape each argument. The default escaping wrapped the row in an extra quote
  pair and backslash-escaped the row's own inner quotes; `cmd /s /c` strips only
  the outer pair, so a native-Windows row such as
  `findstr /c:"…" docs\…` reached `findstr` with broken quoting and exited 1
  under verifyrun even though the identical line passes when typed at a prompt.
  `sh` and `pwsh` rows are unchanged. (#1424)
