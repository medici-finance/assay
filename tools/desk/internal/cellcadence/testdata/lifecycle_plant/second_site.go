package plant

// This file is the committed positive control for TestCellLifecycleClassGuard. It is never built
// (testdata is skipped by the go tool); the guard parses it as if it were production source, and
// every function in it that is marked PLANT must be rejected by the rule it names, while the
// function marked CLEAN must not. The planted sites are the second copies the model exists to
// prevent: a reused-pid judgment, a checkpoint finish without the recorded owner generation, a
// checkpoint writer, and a plan precondition built outside the model.

import (
	"github.com/medici-finance/assay/tools/desk/internal/cellcadence"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// PLANT process-identity: compares a start time itself, so a reused pid reads as a match or a
// mismatch by whatever this site happens to do with it.
func secondSiteReusedPid(pid int, recorded string) bool {
	start, gone, err := deskkit.ProcessStart(pid)
	return err == nil && !gone && start == recorded
}

// PLANT stop-generation: finishes the checkpoint of an owner named by a bare pid, with no
// settle decision and no owner generation.
func secondSiteStaleGeneration(lease *cellcadence.Lease, pid int) error {
	return lease.RecordStopped(pid, "stopped")
}

// PLANT checkpoint-writer: a second function that writes the checkpoint.
func (l *Lease) secondSiteWriter(s State) error {
	return write(l.dir, s)
}

// PLANT plan-class: a plan precondition built outside the model.
func secondSitePlan(role string) string {
	return roleKey(role, "class")
}

// CLEAN: judges the process through the model, so it is not a second site.
func cleanSite(pid int, recorded string) bool {
	start, gone, err := deskkit.ProcessStart(pid)
	return cellcadence.JudgeProcess(recorded, cellcadence.ProcessProbe{Start: start, Gone: gone, Err: err}).Alive()
}
