package main

import "regexp"

// RegressionTestPrefix is the ONE thing this whole tool counts: a top-level
// Go test whose name starts with this prefix is a regression test. No flag,
// environment variable, or workflow input can change it — docs/test-policy.md
// "## Regression suite" states this same literal prefix, and
// TestConvention_DocMatchesConstant pins the doc and this constant together
// so they cannot drift apart silently.
const RegressionTestPrefix = "TestRegression_"

// regressionListPattern is the `go test -list` / default `-run` pattern used
// to discover and (unless --selector overrides it) execute regression tests:
// an unanchored prefix match, so nothing here has to predict the suffix.
const regressionListPattern = "^" + RegressionTestPrefix

// regressionNameShapePattern is the naming convention's full shape:
// TestRegression_<repo>_<issue>[_<Desc>]. docs/test-policy.md "## Regression
// suite" states this exact pattern text; TestConvention_DocMatchesConstant
// asserts the doc and this string agree byte-for-byte.
const regressionNameShapePattern = `^TestRegression_[a-z0-9]+_[0-9]+(_[A-Za-z0-9_]+)?$`

// regressionNameShape is the compiled form of regressionNameShapePattern,
// used to flag a NOTICE (never a failure — Task step 2) for a name that
// carries the prefix but not the full shape.
var regressionNameShape = regexp.MustCompile(regressionNameShapePattern)
