### Fixed
- windows-port/13's no-shell-outs Verify row (row 3) now checks only that brief's own nine `deskinbox` shipping files, and fails on any `os/exec` import or `"make"`/`"jq"` literal in them. It had failed since windows-port/15 added `flow.go`'s sanctioned statusgen/deskboard exec to the same package; windows-port/15's own row 5 already scopes that exec (#1833).
