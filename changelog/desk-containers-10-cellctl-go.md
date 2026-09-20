### Changed
- `cellctl` is now a Go program (`tools/desk/cmd/cellctl`), built and shipped like every other
  desk verb instead of being a `sed`-stamped shell script copied into the release tarball. The
  shell launcher stays in the tree as the parity ORACLE: `tools/cellctl/tests/parity.test.sh`
  diffs both implementations' `DRY_RUN=1` plans across every kind × harness × cockpit × verb,
  and the existing behavioural suites now run against either implementation through a `CELLCTL`
  override. The binary is unix-only until the windows-port stream lands its build-tag split.
