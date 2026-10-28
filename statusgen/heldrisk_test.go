package main

import (
	"os"
	"strings"
	"testing"
)

// regression: #4536
func TestHeldRiskDefinitions(t *testing.T) {
	data, err := os.ReadFile("testdata/held-risk-values.txt")
	if err != nil {
		t.Fatal(err)
	}
	var cases []heldScanCase
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		cases = append(cases, heldScanCase{line, "**VERIFY: PASS**\n\n" + line + "\n", false})
	}
	// A second source and another value name exercise the shared classifier.
	cases = append(cases, heldScanCase{"planted second definition", "**VERIFY: PASS**\n\nRISK-VALUE: DERIVED — Unavailable = \"could-not-check\" @ examples/facts.go:12 — missing and could-not-check cannot release assessment.\n", false})
	runHeldLayerCases(t, cases)
}

// regression: #4536
func TestHeldRiskKeepsHolds(t *testing.T) {
	const def = "RISK-VALUE: DERIVED — Unavailable = \"could-not-check\" @ examples/facts.go:12 — missing and could-not-check cannot release assessment."
	var cases []heldScanCase
	for _, tail := range []string{
		"; row 2 could-not-check — runner unavailable", "; HELD — human read owed",
		"; could-not-check pending runner", "; separate smoke run could-not-check",
		"\nrow 2 HELD", "\nrow 3 could-not-check", "; observed could-not-check",
	} {
		cases = append(cases, heldScanCase{tail, "**VERIFY: PASS**\n\n" + def + tail + "\n", true})
	}
	for _, line := range []string{

		// F-lit-anchor: a valid declaration cannot erase an observation prefix.
		"RISK-VALUE: DERIVED — row 3 Unavailable = \"could-not-check\" @ a.go:1 — runner offline",
		"RISK-VALUE: DERIVED — smoke run CouldNotCheck = \"could-not-check\" @ a.go:1 — runner offline",
		// F-lit-subject: a declaration-shaped observation is still a hold.
		"RISK-VALUE: DERIVED — row 3 status = \"held\" @ README.md:12 — no runner online.",
		"RISK-VALUE: DERIVED — row2 = \"could-not-check\" @ evidence.log:3 — runner unavailable.",
		"RISK-VALUE: DERIVED — smoke run = \"could-not-check\" @ runner.log:1 — no runner online.",
		"RISK-VALUE: DERIVED — smoke = \"held\" @ a.go:1 — pending",
		"RISK-VALUE: DERIVED — Row2 = \"could-not-check\" @ a.go:1 — runner offline",
		"RISK-VALUE: DERIVED — Unavailable = \"could-not-check\", row2 = \"could-not-check\" @ a.go:1 — runner offline",
		"RISK-VALUE: DERIVED — row 2 could-not-check — runner unavailable",
		"RISK-VALUE: NAMED, NOT DERIVED — Unavailable = \"could-not-check\" @ examples/facts.go:12 — definition pending",
		"RISK-VALUE: DERIVED — Unavailable = \"could-not-check\" — source unavailable",
		"RISK-VALUE: DERIVED — Unavailable = \"could-not-check\" @ examples/facts.go:12 — row 2 could-not-check",
		"RISK-VALUE: DERIVED — HELD @ examples/facts.go:12 — no runner",
		"RISK-VALUE: DERIVED — missing and could-not-check cannot release assessment.",
		"RISK-VALUE: DERIVED — Present = \"present\" @ examples/facts.go:12 — row 2 missing and could-not-check cannot release assessment.",
		"RISK-VALUE: DERIVED — Present = \"present\" @ examples/facts.go:12 — verifier could-not-check cannot release assessment.",
		"RISK-VALUE: DERIVED — Present = \"present\" @ examples/facts.go:12 — a runner maps to could-not-check, never to permitted.",
	} {
		cases = append(cases, heldScanCase{line, "**VERIFY: PASS**\n\n" + line + "\n", true})
	}
	cases = append(cases, heldScanCase{"definition in result cell", heldEv(def), true})
	runHeldLayerCases(t, cases)
}

// regression: #4536
func TestVflipRiskDefinition(t *testing.T) {
	const def = "RISK-VALUE: DERIVED — Unavailable = \"could-not-check\" @ examples/facts.go:12 — missing and could-not-check cannot release assessment."
	for _, tail := range []string{"", "; row 2 could-not-check — runner unavailable"} {
		t.Run(tail, func(t *testing.T) {
			o := vfDefaults()
			o.evidence = def + tail + "\n\n" + o.evidence
			root, readme := vfFixture(t, o)
			before, err := os.ReadFile(readme)
			if err != nil {
				t.Fatal(err)
			}
			code, _, errOut := runVF(t, root, "--dry-run")
			want := verifyflipExitOK
			if tail != "" {
				want = verifyflipExitRefused
			}
			if code != want {
				t.Fatalf("exit=%d want=%d: %s", code, want, errOut)
			}
			after, err := os.ReadFile(readme)
			if err != nil {
				t.Fatal(err)
			}
			if string(after) != string(before) {
				t.Fatal("preflight changed README")
			}
		})
	}
}
