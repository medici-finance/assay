### Added
- `statusgen --lint` now flags a Verify row that runs `go test -run` with no
  `--- PASS` assertion in the same command (`gotest-run-vacuous`, advisory) —
  `go test` exits 0 and prints "no tests to run" whether or not the named test
  exists, is built, or was ever renamed away, so an unasserted row is a vacuous
  pass from the day it ships. A closed (`done`/`verified`) brief's rows are
  summarised in one run-wide NOTICE instead of flagged per row, so historical
  records are surfaced without being rewritten.
