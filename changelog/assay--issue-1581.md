### Added
- Three new `quality` stream briefs plan a regression suite. Brief 17 sets a
  `TestRegression_<repo>_<issue>` naming convention and adds a CI gate that goes red when a
  regression test fails, when the regression-test count drops against the base, or when the
  test selector runs nothing. Brief 18 adds a stub-coverage report, report-only at first,
  listing test seams that every test stubs and none runs in production form. Brief 19 adds
  a qualgen re-fix metric: how often a fix repairs a defect an earlier fix had already
  addressed. The briefs are planning only; the tools, the workflow and the metric land
  when each brief is implemented.
- A new `statusgen` stream brief (14) plans an advisory `--lint` rule for a Verify row
  whose `go test -run` selector can pass on "no tests to run", because the row never
  asserts that the named test actually ran.
