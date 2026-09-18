### Fixed
- `plugins/assay/references/standing-note.md` now carries the
  `<!-- assay:harnesslint non-matrix-reference — ... -->` declaration that its harness-neutral
  siblings `desk-shell.md` and `tick-contract.md` already carry. Without it, `tools/harnesslint`'s
  `bindings` mode mistook the file (added 2026-09-14) for a capability-binding matrix and checked
  it against the full closed vocabulary it was never written to satisfy, redding an independent
  `harnesslint bindings plugins/assay/references` pass. (#1182)
