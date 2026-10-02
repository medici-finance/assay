### Fixed
- `deskack` reports malformed receipts as correctable usage errors (exit 2), preserves the 12-word cap, and detects misplaced receipt flags. Desk skills explain correction before continuing; guard and identity failures still stop the pass.
- `deskwt role-init` refreshes the requested role's credential before the initial HTTPS fetch and overrides inherited helpers for that command without changing source checkout configuration. Network origins must use HTTPS without embedded credentials; local and explicit offline initialization remain supported.
