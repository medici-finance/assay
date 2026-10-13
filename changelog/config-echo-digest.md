### Changed
- Desk tools now print the `assay-config:` effective-config echo as one digest line by default
  (`class=… source=… configured=… digest=sha256:<12> humans=N bots=N roles=N repos=N`) instead of
  the ~22-line block on every run. The full block still prints in CI (`CI=true` or `CI=1`, or
  `GITHUB_ACTIONS=true`), on any `REFUSED` or `unrecognised-keys` line, on any run whose digest
  differs from the last recorded one (tracked in `config-echo.state` beside the roster; an
  unreadable or unwritable state file keeps the full block), and whenever
  `ASSAY_CONFIG_ECHO=full` is set. Adopter Verify steps that grep individual echo lines now set
  `ASSAY_CONFIG_ECHO=full`.
