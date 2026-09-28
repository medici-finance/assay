### Fixed
- Added the missing `system-demo` row to the per-skill degradation table in the Claude Code
  binding (`plugins/assay/references/claude-code.md`). The row was left out when the skill's
  cells were added to the Codex and Cursor bindings, so `harnesslint bindings
  plugins/assay/references` exited 1 on main.
- Re-spelled three Verify rows in harness-portability brief 12 (rows 5, 9 and 11) so the
  execution witness runs the command each row means. No Expect was loosened, and row 11 now
  fails when either Cursor freshness entry is STALE.
