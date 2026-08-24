package main

import (
	"strconv"
	"strings"
)

// verifyrows.go — parse a brief's `## Verify` table into typed rows the DETERMINISTIC
// runner can execute. The runner needs no model in its hot path: a row's Class says who
// runs it and its Command is the exact thing to run; the exit code IS the verdict.

// verifyRow is one parsed `## Verify` table row: its number, its Class (row-classes.md;
// default `check` when the table carries no Class column — the compatibility hinge) and its
// Command cell. When a row is scripted the Command cell IS the script path and nothing else
// (the verify.d convention).
type verifyRow struct {
	Num     int
	Class   string // check | check:ci | gate:model | gate:human
	Command string
}

// runnerExecuted reports whether this class is one the deterministic runner executes.
// ONLY check and check:ci run here; gate:model / gate:human are judgment lanes the runner
// never touches — this brief changes the LANDING, not the judgment lanes.
func (r verifyRow) runnerExecuted() bool {
	return r.Class == "check" || r.Class == "check:ci"
}

// parseVerifyRows extracts the rows of the `## Verify` pipe table from a brief. Columns are
// located by header NAME (#, class, command), never position — mirroring row-classes.md and
// the statusgen parser — so inserting a Class column disturbs nothing. A table with no
// `Class` column is legacy: every row defaults to `check`. Rows whose `#` cell is not an
// integer (a separator, a stray line) are skipped.
func parseVerifyRows(briefContent string) []verifyRow {
	section := extractSection(briefContent, "## Verify")
	if section == "" {
		return nil
	}
	lines := strings.Split(section, "\n")
	for i, line := range lines {
		if !strings.HasPrefix(strings.TrimSpace(line), "|") {
			continue
		}
		idx := headerIndex(line)
		numCol, hasNum := idx["#"]
		cmdCol, hasCmd := idx["command"]
		if !hasNum || !hasCmd {
			continue
		}
		classCol, hasClass := idx["class"]
		var rows []verifyRow
		for _, row := range lines[i+2:] { // skip the |---| separator row
			if !strings.HasPrefix(strings.TrimSpace(row), "|") {
				break
			}
			if sepRe.MatchString(row) {
				continue
			}
			cells := splitRow(row)
			cell := func(j int) string {
				if j >= 0 && j < len(cells) {
					return strings.TrimSpace(cells[j])
				}
				return ""
			}
			num, err := strconv.Atoi(cell(numCol))
			if err != nil {
				continue
			}
			class := "check"
			if hasClass {
				if c := cell(classCol); c != "" {
					class = c
				}
			}
			rows = append(rows, verifyRow{
				Num:     num,
				Class:   class,
				Command: stripInlineCode(cell(cmdCol)),
			})
		}
		return rows
	}
	return nil
}

// extractSection returns the body between an exact `## <heading>` line and the next `## `
// heading (or end of file). Mirrors frontmatter.go extractEvidence, generalised by heading.
func extractSection(content, heading string) string {
	lines := strings.Split(content, "\n")
	start := -1
	for i, l := range lines {
		if strings.TrimSpace(l) == heading {
			start = i + 1
			break
		}
	}
	if start < 0 {
		return ""
	}
	var out []string
	for _, l := range lines[start:] {
		if strings.HasPrefix(strings.TrimSpace(l), "## ") {
			break
		}
		out = append(out, l)
	}
	return strings.Join(out, "\n")
}

// stripInlineCode unwraps a `…` inline-code span from a Command cell so the runner sees the
// bare command; a cell with no wrapping backticks passes through unchanged.
func stripInlineCode(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && strings.HasPrefix(s, "`") && strings.HasSuffix(s, "`") {
		return strings.TrimSpace(strings.Trim(s, "`"))
	}
	return s
}
