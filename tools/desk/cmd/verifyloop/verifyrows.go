package main

import (
	"strconv"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"github.com/medici-finance/assay/tools/desk/internal/verifycmd"
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
	ID      deskkit.VerifyRowID
	Class   string // check | check:ci | gate:model | gate:human
	Command string
	// Cell is the raw trimmed Command cell. Command is only its lift (the marked
	// command, else the first code span); a reader matching prose in the row, not
	// the command, reads Cell.
	Cell string
	// Refused is verifycmd.Refused of the raw cell: why the row's command must not
	// run (a shell comment, a no-op, a prose mention, a blank lift), or "".
	// runBriefRows never executes such a row (#2684, #3147).
	Refused string
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
// integer are outside verdict-v1; the scheduling reader retains exact suffixed
// IDs and letter-led labels (deskkit.ParseVerifyRowID).
func parseVerifyRows(briefContent string) []verifyRow {
	// Preserve the exact historical Atoi execution contract, independently of the
	// stricter canonical identities required before scheduling can reuse observations.
	return parseVerifyRowsMode(extractSection(briefContent, "## Verify"), false)
}

// parseVerifyRowsIn parses the rows of an already-extracted Verify SECTION body. It is the
// shared half of parseVerifyRows (exact `## Verify` heading, the runner's contract) and the
// per-row content derivation in briefscan.go, which reads the qualifier-aware extractVerify
// section — so both read the Command cell by the same header-named rule.
// ID retains an exact positive number plus optional ASCII letter suffix; Num is
// populated only for the separate integer-only verdict-v1 execution contract.
func parseVerifyRowsIn(section string) []verifyRow {
	return parseVerifyRowsMode(section, true)
}

func parseVerifyRowsMode(section string, scheduling bool) []verifyRow {
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
			cells := verifycmd.SplitRow(row)
			cell := func(j int) string {
				if j >= 0 && j < len(cells) {
					return strings.TrimSpace(cells[j])
				}
				return ""
			}
			rawID := cell(numCol)
			id, exact := deskkit.ParseVerifyRowID(rawID)
			num, numErr := strconv.Atoi(rawID)
			if scheduling {
				if !exact && rawID == "" {
					continue
				}
				// Keep every nonempty noncanonical cell in the inventory. Reuse must surface
				// them as could-not-check, never silently omit an original row.
				if !exact {
					id = deskkit.VerifyRowID(rawID)
				}
			} else {
				if numErr != nil {
					continue
				}
				id = deskkit.VerifyRowID(rawID)
			}
			class := "check"
			if hasClass {
				if c := cell(classCol); c != "" {
					class = c
				}
			}
			// verifycmd.Lift honours the `cmd:` marker (spec §4.4, #1805) and
			// otherwise lifts the first code span, exactly as statusgen does (#3147).
			// The split kept `\|` in the cell; the runner executes the un-escaped
			// pipeline (#1953). verifycmd.Refused judges the RAW cell: what statusgen
			// refuses as a comment, a no-op, a prose mention or a blank lift never
			// runs here.
			rows = append(rows, verifyRow{
				Num:     num,
				ID:      id,
				Class:   class,
				Command: verifycmd.UnescapePipes(verifycmd.Lift(cell(cmdCol))),
				Cell:    cell(cmdCol),
				Refused: verifycmd.Refused(cell(cmdCol)),
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
