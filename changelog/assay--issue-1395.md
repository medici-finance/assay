### Fixed
- `statusgen --corroborate` no longer reads quoted notation as a claim that a human acted.
  Two kinds of line used to fail the check falsely. The first is a line on the removed (`-`)
  side of a hunk in a diff inside a committed `.patch` or `.diff` file; neither the stamp
  scan nor the citation scan reads it now. The second is `human:<name>` text in a test
  source file (a closed extension list plus a test-file name convention such as
  `x_test.go` or `x.test.sh`) or on a YAML `#` line; the stamp scan skips it. Record files,
  non-test programs and scripts, YAML value lines, other extensions, and the added side,
  context and pre-hunk preamble of an embedded patch are still scanned. Every skipped stamp
  or citation is listed in a `NOT-A-CLAIM` section of the run output.

### Changed
- All three `--corroborate` lanes (stamps, citations, decision records) now read the diff
  through one walker. The guard test `TestCorroborateDiffWalkersShareOneWalker` fails if
  another function in the package walks the diff with its own loop.
