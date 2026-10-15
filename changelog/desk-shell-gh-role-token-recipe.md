### Added
- `desk-shell.md` §Forge CLI as the role: the canonical recipe for a desk's `gh` call. It mints the role App token with `desktoken` from `$DESK_LOOP`, scopes `GH_TOKEN` to the one command, and fails closed with a named diagnostic when the role or its custody is missing. It also separates a desk-authentication failure from a host with no human `gh` login, and personal OAuth is never the repair (#3055). A test holds the recipe's loop-to-role mapping to the desk tools' own table, retired loop names included.
- `skillslint` ambient-auth check (hard): no skill or reference may tell a desk to read a human's `gh auth token`. The canonical section and the driver's own `gh auth token --user <driver-login>` command are the only exemptions.

### Fixed
- The shared shell-mechanics pointer in every desk skill now names the forge-CLI recipe. The ask-decision, pr-shepherd and intake-desk identity lines point at it. The pr-review-desk out-of-scope-filing reference and the `tools/desk` README no longer say `deskfile` files under whatever `gh` login is ambient: it files as the session's role App.
