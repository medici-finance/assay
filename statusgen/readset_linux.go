//go:build linux

package main

import "github.com/medici-finance/assay/statusgen/evaluation"

// readSetRecorderAbsent is the could-not-check reason a Linux row records
// while this build carries no tracing recorder: the row writes no read-set
// file, so its witness keeps the conservative scope (fail closed).
const readSetRecorderAbsent = "no read-set recorder in this build"

// rowReadSetToken is an executed row's read-set token on Linux.
func rowReadSetToken() string { return evaluation.ReadSetCellToken("", readSetRecorderAbsent) }
