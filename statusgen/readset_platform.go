package main

// readset_platform.go — the platform decision for the read-set recorder
// (build-less-brittle/15). The recorder observes a row's file reads with
// ptrace and a seccomp filter, which only Linux offers; on every other OS a
// row's Runner cell carries a could-not-check token and no read-set file is
// written, so the witness keeps today's conservative scope.

import "github.com/medici-finance/assay/statusgen/evaluation"

// readSetRecorderShipped is THE switch for read-set judgement (review S20).
// It is false while this build carries no recorder: no caller observes a
// read set, the go floor never runs, and the library judges every
// `(reads sha256:…)` token could-not-check (evaluation.ReasonNoRecorder),
// so a row with one comes out exactly as the same row without it. The
// recorder slice flips it, once it also supplies what the judge then needs:
// a landing-identity binding on autoflip, a declared compile step for every
// go row, and the kind of each listed child.
const readSetRecorderShipped = false

// honourReadSets is what coverageOptionsFor reads. It is
// readSetRecorderShipped; only tests assign it (recordedReadSets), to drive
// the recorded path through the real callers.
var honourReadSets = readSetRecorderShipped

// readSetOffLinuxToken is the Runner-cell token a row gets on goos, an OS the
// recorder does not run on. readset_other.go calls it with runtime.GOOS.
func readSetOffLinuxToken(goos string) string {
	return evaluation.ReadSetCellToken("", "recorder is Linux-only ("+goos+")")
}
