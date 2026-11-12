package main

import (
	"strings"
	"testing"
)

// All tests in this file share the `RowEnv` substring so one
// `go test -run RowEnv` exercises the whole Env-column surface (issue #6065).
//
// regression: #6065 — a Verify row that needs a live service could be classed
// `check:ci` (which CI re-executes network-off on the tree alone), and an
// env-bound row could name no environment at all, so nobody could tell where a
// non-implementer was meant to witness it.

// rowEnvProblems lints one fixture brief whose Verify section is `table`.
func rowEnvProblems(t *testing.T, table ...string) []string {
	t.Helper()
	dir := t.TempDir()
	briefV1WithVerify(t, dir, "01", strings.Join(table, "\n"))
	return verifyRowClassProblems([]*Stream{{Name: "t", Dir: dir, Root: dir}})
}

// wantOneEnvProblem asserts exactly one PROBLEM, naming every fragment.
func wantOneEnvProblem(t *testing.T, probs []string, fragments ...string) {
	t.Helper()
	if len(probs) != 1 {
		t.Fatalf("got %d problems %v, want exactly one", len(probs), probs)
	}
	for _, f := range fragments {
		if !strings.Contains(probs[0], f) {
			t.Errorf("problem %q does not name %q", probs[0], f)
		}
	}
}

const envHeader = "| # | Class | Env | Command | Expect |"
const envSep = "|---|-------|-----|---------|--------|"

// A `check:ci` row is re-executed by CI network-off on the checkout alone, so
// one that names a service CI cannot reach is mis-classed: red.
func TestRowEnv_CheckCINamesService(t *testing.T) {
	probs := rowEnvProblems(t, envHeader, envSep,
		"| 1 | check:ci | service:orders-staging | `true` | exit 0 |")
	wantOneEnvProblem(t, probs, "Verify row 1", "check:ci", "service:orders-staging", "#6065")
}

// The same holds for a host- or cluster-bound environment: CI has neither.
func TestRowEnv_CheckCINamesHost(t *testing.T) {
	probs := rowEnvProblems(t, envHeader, envSep,
		"| 1 | check:ci | host | `true` | exit 0 |",
		"| 2 | check:ci | cluster:devnet | `true` | exit 0 |")
	if len(probs) != 2 {
		t.Fatalf("got %d problems %v, want two (host, cluster)", len(probs), probs)
	}
}

// An env-bound row (`check`, `check:cluster`) that names no environment leaves
// the witness with nowhere to run it: red.
func TestRowEnv_CheckNamesNoEnv(t *testing.T) {
	probs := rowEnvProblems(t, envHeader, envSep,
		"| 1 | check |  | `true` | exit 0 |")
	wantOneEnvProblem(t, probs, "Verify row 1", "`check`", "names no environment")
}

func TestRowEnv_ClusterNamesNoEnv(t *testing.T) {
	probs := rowEnvProblems(t, envHeader, envSep,
		"| 1 | check:cluster |  | `true` | exit 0 |")
	wantOneEnvProblem(t, probs, "Verify row 1", "check:cluster", "names no environment")
}

// A table with an Env column but no Class column is legacy: every row resolves
// to legacy `check`, which the scheduled main re-run still executes. A row
// there naming a service would be run by CI anyway: red, until it is classed.
func TestRowEnv_UnclassedNamesService(t *testing.T) {
	probs := rowEnvProblems(t,
		"| # | Env | Command | Expect |",
		"|---|-----|---------|--------|",
		"| 1 | service:orders-staging | `true` | exit 0 |")
	wantOneEnvProblem(t, probs, "Verify row 1", "service:orders-staging", "Class")
}

// An Env cell outside the closed grammar routes the row nowhere: red, naming
// exactly what the author wrote.
func TestRowEnv_UnknownKindIsProblem(t *testing.T) {
	for _, cell := range []string{"staging", "service:", "tree:x", "service:Bad Name", "db:orders"} {
		probs := rowEnvProblems(t, envHeader, envSep,
			"| 1 | check | "+cell+" | `true` | exit 0 |")
		if len(probs) != 1 || !strings.Contains(probs[0], "environment") {
			t.Errorf("Env %q: got %v, want one PROBLEM about the environment", cell, probs)
		}
	}
}

// The negative controls: every well-formed shape is clean, and so is every
// table that carries no Env column (the whole inherited corpus).
func TestRowEnv_CleanShapes(t *testing.T) {
	clean := [][]string{
		{envHeader, envSep,
			"| 1 | check:ci | tree | `true` | exit 0 |",
			"| 2 | check:ci |  | `true` | exit 0 |",
			"| 3 | check | service:orders-staging | `true` | exit 0 |",
			"| 4 | check | `host` | `true` | exit 0 |",
			"| 5 | check | host:windows | `true` | exit 0 |",
			"| 6 | check:cluster | cluster:devnet | `true` | exit 0 |",
			"| 7 | gate:human |  | prose | a human reads it |",
			"| 8 | gate:model | service:orders-staging | prose | a model reads it |",
			"| 9 | check | tree | `true` | exit 0 |"},
		{"| # | Env | Command | Expect |", "|---|-----|---------|--------|",
			"| 1 | tree | `true` | exit 0 |",
			"| 2 |  | `true` | exit 0 |"},
		{"| # | Class | Command | Expect |", "|---|-------|---------|--------|",
			"| 1 | check | `true` | exit 0 |",
			"| 2 | check:ci | `true` | exit 0 |"},
		{"| # | Command | Expect |", "|---|---------|--------|",
			"| 1 | `true` | exit 0 |"},
	}
	for i, table := range clean {
		if probs := rowEnvProblems(t, table...); len(probs) != 0 {
			t.Errorf("clean table %d: got %v, want none", i, probs)
		}
	}
}

// The column is located by header NAME and carries no meaning when absent.
func TestRowEnv_ColumnParsed(t *testing.T) {
	var got []verifyRowCells
	verifyRowTable(strings.Join([]string{envHeader, envSep,
		"| 1 | check | service:orders-staging | `true` | exit 0 |"}, "\n"),
		func(r verifyRowCells) { got = append(got, r) })
	if len(got) != 1 || !got[0].EnvCol || got[0].Env != "service:orders-staging" {
		t.Fatalf("Env column: got %+v", got)
	}
	got = nil
	verifyRowTable("| # | Command | Expect |\n|---|---|---|\n| 1 | `true` | exit 0 |",
		func(r verifyRowCells) { got = append(got, r) })
	if len(got) != 1 || got[0].EnvCol || got[0].Env != "" {
		t.Fatalf("no Env column: got %+v", got)
	}
}
