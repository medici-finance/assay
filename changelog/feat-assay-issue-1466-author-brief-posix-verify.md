### Added
- `statusgen newbrief` gains a `--shell` flag (`sh` default / `cmd` / `pwsh`) so a
  native-Windows Verify row can carry its shell marker at authoring time. It
  **refuses** a native-Windows `--verify-command` (e.g. `findstr …`) under the
  default `sh` shell — pointing the author at POSIX-izing it (`grep -F`,
  forward-slash paths) or declaring the shell — and, when `--shell cmd`/`pwsh` is
  given, emits the row with its `| # | Shell | Command | Expect |` marker attached
  in the same pass. This enforces at the brief-authoring front door the rule the
  author-brief guidance already states: a Windows-shaped row must never land as the
  default `bash -o pipefail` row with no marker, since an Evidence-only PR cannot
  rewrite the Verify table to fix it. The default POSIX row keeps its
  Shell-column-less shape byte-for-byte. (#1466, #1424)
