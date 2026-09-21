### Added
- Verify tables may declare a per-row **shell** in an optional `Shell` column
  (`sh` / `cmd` / `pwsh`, default `sh`). `statusgen verifyrun` dispatches each row
  to its declared shell — `bash -o pipefail -c` for `sh` (every inherited row,
  unchanged), `cmd /d /s /c` for `cmd`, `powershell -NoProfile -Command` for
  `pwsh` — so a native-Windows row such as `findstr /c:"…" a\b.md`, which only
  works under `cmd.exe`, keeps its authored meaning instead of failing with a
  bash-level error under Git-for-Windows bash. The shell is declared, never
  guessed from the command text.

### Fixed
- A native-Windows Verify row that passes under `cmd` but fails under bash no
  longer blocks a brief's closure with a false `fail exit=1`: it is either run
  under its declared shell (`cmd`/`pwsh`), or, when that shell is unavailable on
  the runner's OS (a `cmd`/`pwsh` row on Linux/macOS), recorded **could-not-run**
  with the reason — never `fail`, and never silently rewritten into another
  shell. `cmd`/PowerShell exit codes are read faithfully.
- `statusgen --lint` now flags an unrecognised `Shell` marker (a typo like `bash`
  or `powershell`) as a hard PROBLEM at authoring time, so a marker the tool
  cannot resolve is never silently treated as the default.
