package main

// readset_platform.go — the platform decision for the read-set recorder
// (build-less-brittle/15). The recorder observes a row's file reads with
// ptrace and a seccomp filter, which only Linux offers; on every other OS a
// row's Runner cell carries a could-not-check token and no read-set file is
// written, so the witness keeps today's conservative scope.

import "github.com/medici-finance/assay/statusgen/evaluation"

// readSetOffLinuxToken is the Runner-cell token a row gets on goos, an OS the
// recorder does not run on. readset_other.go calls it with runtime.GOOS.
func readSetOffLinuxToken(goos string) string {
	return evaluation.ReadSetCellToken("", "recorder is Linux-only ("+goos+")")
}
