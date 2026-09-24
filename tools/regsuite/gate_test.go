package main

import (
	"strings"
	"testing"
)

// TestGate_PlantedDeletion_Red is Verify row 2. base has 2 regression tests
// in module modx, head has 1 (a planted deletion, no rename). The gate must
// exit 1 and name the missing test.
func TestGate_PlantedDeletion_Red(t *testing.T) {
	head := materializeTxtar(t, "planted_deletion_head.txtar")
	base := materializeTxtar(t, "planted_deletion_base.txtar")

	r := runGate(gateOptions{Head: head, Base: base})

	if r.ExitCode != 1 {
		t.Fatalf("exit code = %d, want 1\nreport:\n%s", r.ExitCode, r.Report)
	}
	if !strings.Contains(r.Report, "count-drop") {
		t.Errorf("report does not name the count-drop reason:\n%s", r.Report)
	}
	if !strings.Contains(r.Report, "TestRegression_assay_9002_Second") {
		t.Errorf("report does not name the missing test TestRegression_assay_9002_Second:\n%s", r.Report)
	}
	if !strings.Contains(r.Report, "head listed 1 < base listed 2") {
		t.Errorf("report does not state the totals it compared:\n%s", r.Report)
	}
}

// TestGate_PlantedVacuousSelector_Red is Verify row 3. modx lists two
// regression tests in both trees (no count drop), but --selector matches
// neither — the "go test -run matches nothing" vacuous pass. The gate must
// exit 1 and say "vacuous".
func TestGate_PlantedVacuousSelector_Red(t *testing.T) {
	head := materializeTxtar(t, "planted_vacuous_selector.txtar")
	base := materializeTxtar(t, "planted_vacuous_selector.txtar")

	r := runGate(gateOptions{Head: head, Base: base, Selector: "^NoSuchTest$"})

	if r.ExitCode != 1 {
		t.Fatalf("exit code = %d, want 1\nreport:\n%s", r.ExitCode, r.Report)
	}
	if !strings.Contains(r.Report, "vacuous") {
		t.Errorf("report does not say vacuous:\n%s", r.Report)
	}
	if strings.Contains(r.Report, "count-drop") {
		t.Errorf("report wrongly reports count-drop for a vacuous-only case:\n%s", r.Report)
	}
}

// TestGate_FailingRegressionTest_Red is Verify row 4. A regression test
// genuinely fails (same count on both sides, so this isolates the "fail"
// reason from "count-drop"). The gate must exit 1.
func TestGate_FailingRegressionTest_Red(t *testing.T) {
	head := materializeTxtar(t, "failing_regression_test.txtar")
	base := materializeTxtar(t, "failing_regression_test.txtar")

	r := runGate(gateOptions{Head: head, Base: base})

	if r.ExitCode != 1 {
		t.Fatalf("exit code = %d, want 1\nreport:\n%s", r.ExitCode, r.Report)
	}
	if !strings.Contains(r.Report, "TestRegression_assay_9202_Fails") {
		t.Errorf("report does not name the failing test:\n%s", r.Report)
	}
	if strings.Contains(r.Report, "count-drop") {
		t.Errorf("report wrongly reports count-drop for a fail-only case:\n%s", r.Report)
	}
	if strings.Contains(r.Report, "vacuous") {
		t.Errorf("report wrongly reports vacuous for a fail-only case:\n%s", r.Report)
	}
}

// TestGate_RenameKeepsPrefix_Clean is Verify row 5, the negative control:
// base and head differ only by a prefixed rename (count unchanged, 1 == 1).
// The gate must exit 0 — proving the comparison is on COUNT, not name set.
func TestGate_RenameKeepsPrefix_Clean(t *testing.T) {
	head := materializeTxtar(t, "rename_keeps_prefix_head.txtar")
	base := materializeTxtar(t, "rename_keeps_prefix_base.txtar")

	r := runGate(gateOptions{Head: head, Base: base})

	if r.ExitCode != 0 {
		t.Fatalf("exit code = %d, want 0\nreport:\n%s", r.ExitCode, r.Report)
	}
	if !strings.Contains(r.Report, "listed(head=1 base=1)") {
		t.Errorf("report does not show equal head/base counts (1 == 1) despite the rename:\n%s", r.Report)
	}
	if !strings.Contains(r.Report, "clean") {
		t.Errorf("report does not say clean:\n%s", r.Report)
	}
}

// TestGate_BrokenModule_CouldNotCheck is Verify row 6. The head module's
// test file fails to compile. The gate must exit 2 and say
// "could-not-check" — never 0.
func TestGate_BrokenModule_CouldNotCheck(t *testing.T) {
	head := materializeTxtar(t, "broken_module_head.txtar")
	base := materializeTxtar(t, "broken_module_base.txtar")

	r := runGate(gateOptions{Head: head, Base: base})

	if r.ExitCode != 2 {
		t.Fatalf("exit code = %d, want 2\nreport:\n%s", r.ExitCode, r.Report)
	}
	if !strings.Contains(r.Report, "could-not-check") {
		t.Errorf("report does not say could-not-check:\n%s", r.Report)
	}
}

// TestGate_TestdataModulesExcluded is Verify row 7. A second module nested
// under a "testdata" path segment does not compile; if discovery ever
// stopped skipping "testdata" segments, resolving it would redden the run.
// The gate must exit 0 (clean): the broken nested module is never reached.
func TestGate_TestdataModulesExcluded(t *testing.T) {
	head := materializeTxtar(t, "testdata_modules_excluded.txtar")
	base := materializeTxtar(t, "testdata_modules_excluded.txtar")

	r := runGate(gateOptions{Head: head, Base: base})

	if r.ExitCode != 0 {
		t.Fatalf("exit code = %d, want 0 (the testdata/ nested module must be excluded from discovery, not attempted)\nreport:\n%s", r.ExitCode, r.Report)
	}
	if strings.Contains(r.Report, "brokenfixture") {
		t.Errorf("report mentions the testdata/ nested module — discovery did not skip it:\n%s", r.Report)
	}
	if !strings.Contains(r.Report, "modules examined: 1") {
		t.Errorf("report examined more than the one real module — the testdata/ nested module was not excluded:\n%s", r.Report)
	}
	if !strings.Contains(r.Report, "listed(head=1 base=1)") {
		t.Errorf("report does not show the real module's test listed and executed:\n%s", r.Report)
	}
}
