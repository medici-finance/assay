### Fixed
- `scripts/build-windows.ps1` parses under Windows PowerShell 5.1 again: the seven em dashes
  added with the opt-in `-Sign` switch are now ASCII `--`. Without a BOM, 5.1 reads the file
  as Windows-1252, where an em dash's `0x94` byte is a string delimiter, so the one inside a
  `throw "..."` string broke the parse. `scripts/bootstrap-windows.ps1` and
  `scripts/windows-bootstrap-hashcheck-smoke.ps1` had the same em dashes and are ASCII now too.

### Added
- A byte guard, `TestPS1EncodingIs51Safe` in `tools/desk/internal/deskkit`,
  fails CI when any `*.ps1` in the tree contains a byte above `0x7F` and does not start with a
  UTF-8 BOM. It runs in the existing build-test job, so no workflow change is needed to make it
  gate. The staged Windows CI leg (`ci/staged-workflows/windows-ci-leg.yml`) also gains a real
  Windows PowerShell 5.1 `ParseFile` check over every tracked `.ps1`; it takes effect once a
  maintainer promotes the file.
