package main

import "testing"

// Tests for the subject-column exception of rule 1 (#4993). The Evidence of
// verification-handoff/03 carries an obligation-trace table with no row-key
// column, so the scan read it whole and refused a strict PASS on the word
// "held" in the obligation name "Addressed and held exclude ordinary work".
// That cell names what a row must establish, not how it came out.

// b03Row is the obligation-trace row the scan refused on, as committed.
const b03Row = "| Addressed and held exclude ordinary work | shared WorkReady selector sees addressee/decision/help/exclusion; a placeholder flag is explicitly cleared before assessment | 2; addressed-admission |"

// subjectTable is a three-column table under the given header line, with a
// separator row and the given data rows.
func subjectTable(header string, rows ...string) string {
	s := header + "\n|---|---|---|\n"
	for _, r := range rows {
		s += r + "\n"
	}
	return s
}

// b03Header is the obligation-trace table's header as committed.
const b03Header = "| Obligation | Production owner and trace | Original row / control |"

// pass is one entry that ran two rows clean and closes on a strict PASS, with
// note between its results table and its verdict.
func pass(title, note string) string {
	return proseRun(title, 2, allOK, note, "**VERIFY: PASS**")
}

// TestHeldScopeSubjectCell: a lower-case held in a subject cell of a table
// that is not a results table is not a disposition, in an earlier entry and in
// the last one. A second, planted instance (a Requirement column naming
// could-not-check) is admitted the same way (defect class).
// regression: #4993
func TestHeldScopeSubjectCell(t *testing.T) {
	trace := subjectTable(b03Header, b03Row)
	req := subjectTable("| Requirement | Owner | Trace |",
		"| retried could-not-check reads stay owed | scanloop | 1; transient-not-gone |")
	runHeldLayerCases(t, []heldScanCase{
		{"b03 row, earlier entry", pass("Run 1", trace) + pass("Run 2", ""), false},
		{"b03 row, last entry", pass("Run 1", "") + pass("Run 2", trace), false},
		{"b03 row, one entry", pass("Run 1", trace), false},
		{"planted requirement row", pass("Run 1", req) + pass("Run 2", ""), false},
		{"plural, emphasised header", pass("Run 1", subjectTable(
			"| **Obligations** | Production owner and trace | Original row / control |", b03Row)), false},
		{"table with no results table", "### Trace\n\n" + trace + "\n**VERIFY: PASS**\n", false},
	})
}

// TestHeldScopeSubjectKeeps: the exception stays narrow. Each case is one
// change to an admitted shape that must keep the hold read.
// regression: #4993
func TestHeldScopeSubjectKeeps(t *testing.T) {
	row := func(subject string) string {
		return pass("Run 1", subjectTable(b03Header, "| "+subject+" | trace | 2; control |"))
	}
	runHeldLayerCases(t, []heldScanCase{
		{"held in another column", pass("Run 1", subjectTable(b03Header,
			"| Addressed work is excluded | held for a runner | 2; control |")), true},
		{"subject names a row", row("Row 2 held for a runner"), true},
		{"subject names a key", row("#2 held for a runner"), true},
		{"upper-case HELD", row("Addressed and HELD exclude ordinary work"), true},
		{"mixed-case Held", row("Held and addressed exclude ordinary work"), true},
		{"upper could-not-check", row("COULD-NOT-CHECK reads stay owed"), true},
		{"struck span on the line", row("~~old~~ addressed and held exclude ordinary work"), true},
		{"misaligned row", pass("Run 1", subjectTable(b03Header,
			"| Addressed and held exclude ordinary work | a|b | 2 |")), true},
		{"no separator row", pass("Run 1", b03Header+"\n"+b03Row+"\n"), true},
		{"header not exact", pass("Run 1", subjectTable(
			"| Obligation status | Production owner and trace | Original row / control |", b03Row)), true},
		{"header row itself", pass("Run 1", subjectTable(
			"| Obligation | held | Original row / control |", "| a | b | c |")), true},
		{"line after the table", pass("Run 1", subjectTable(b03Header, b03Row)+
			"Addressed and held exclude ordinary work\n"), true},
		// A subject column of a results table stays an unrecognised result
		// column and is read.
		{"results-table subject", pass("Run 1", "| # | Obligation | Result |\n|---|---|---|\n"+
			"| 1 | Addressed and held exclude ordinary work | ok |\n"), true},
	})
}
