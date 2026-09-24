### Fixed
- `statusgen --corroborate` no longer reads quoted notation as a claim that a human acted.
  Two kinds of line used to fail the check falsely. The first is a line on the removed (`-`)
  side of a diff inside a committed `.patch` or `.diff` file; neither the stamp scan nor
  the citation scan reads it now. The second is `human:<name>` text in program source,
  scripts and test fixtures (a closed extension list) or on a YAML `#` comment line; the
  stamp scan skips it. Record files, YAML value lines, other extensions, and the added
  and context sides of an embedded patch are still scanned. Every skipped stamp or
  citation is listed in a `NOT-A-CLAIM` section of the run output.

### Changed
- All three `--corroborate` lanes (stamps, citations, decision records) now read the diff
  through one walker. The guard test `TestCorroborateDiffWalkersShareOneWalker` fails if
  another function in the package walks the diff with its own loop.
