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
		// Outside a RISK-VALUE record, as a marker (ruling A on #4721 does
		// not read the lower-case word in free prose).
		"missing and COULD-NOT-CHECK cannot release assessment.",
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

// regression: #4200
func TestHeldRiskOutcomeMappings(t *testing.T) {
	const pass = "**VERIFY: PASS**\n\n"
	const positional = "RISK-VALUE: DERIVED — exit codes = 0,5,6,1 @ tools/desk/internal/cellop/result.go:48,50,52,54 — spec maps ok/refused/could-not-check/failure respectively and unknown outcomes must not imply success."
	const numbered = "- RISK-VALUE: DERIVED — exit codes = 0, 5, 6, 1 @ tools/desk/internal/cellop/result.go:48, 50, 52, 54 — the brief's table: 0 ok, 5 refused, 6 could-not-check, 1 failed or partial. Any other outcome maps to 1, never 0."
	const states = `- RISK-VALUE: DERIVED — settled states = "observed", "external-wait" @ tools/desk/cmd/scanloop/observations.go:75 and the landing gate @ handoffrecord.go:110 — the spec's retention section lists exactly these as discharging a retained event; owed, held or unreadable states stay presented, and a newer source event is a new key.`
	cases := []heldScanCase{
		{"positional mapping", pass + positional, false},
		{"code-labelled mapping", pass + numbered, false},
		{"state enumeration", pass + states, false},
		{"another contract and outcome order", pass + "RISK-VALUE: DERIVED — exit codes = 7,0 @ cli/exit.go:1-3 — spec maps could-not-check/ok respectively.", false},
		{"named row still held", pass + numbered + " Row 4 HELD pending access.", true},
		{"independent prose hold", pass + positional + " Separate smoke run: HELD.", true},
		{"unrouted verification result", pass + positional + "\n| # | Command | Result |\n|---|---|---|\n| 4 | check | could-not-check |", true},
		{"unsourced derivation", pass + strings.Replace(positional, " @ tools/desk/internal/cellop/result.go:48,50,52,54", "", 1), true},
		{"existing meaning with an exit declaration", pass + "RISK-VALUE: DERIVED — exit codes = 5,6 @ example.go:1 — a timeout maps to could-not-check, never to permitted.", false},
		{"numbered table disagrees with declaration", pass + strings.Replace(numbered, "6 could-not-check", "9 could-not-check", 1), true},
		{"mapping omits an exit", pass + strings.Replace(positional, "/failure respectively", " respectively", 1), true},
		{"run inside a sourced note", pass + strings.Replace(positional, "spec maps ok/refused/could-not-check/failure respectively", "the smoke run was could-not-check", 1), true},
		{"mapping in result cell", heldEv(positional), true},
		{"unreadable state is an actual hold", pass + strings.Replace(states, "owed, held or unreadable states stay presented", "the smoke run is held", 1), true},
	}
	runHeldLayerCases(t, cases)
}
