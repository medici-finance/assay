// Package runnercmds lists the Command cells of a brief's `## Verify` table that
// the deterministic runner executes (example-stream/50).
//
// It exists beside internal/verifycmd rather than inside internal/deskkit: the
// hub may import only the packages on its allow-list, and reading a Command
// column is verifycmd's business. A caller that needs the list (the dispatcher's
// prepare step, the loop's item admission) reads it here and hands it to the hub.
package runnercmds

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/verifycmd"
)

var sepRe = regexp.MustCompile(`^\s*\|?\s*-{2,}`)

// Commands returns the lifted, un-escaped command of every row the runner runs:
// class check or check:ci (default check when there is no Class column), an
// integer # cell, and a Command cell verifycmd.Refused does not refuse. A refused
// cell (a comment, a no-op, prose) is never executed, so it names nothing.
func Commands(brief string) []string {
	lines := strings.Split(verifySection(brief), "\n")
	for i, line := range lines {
		if !strings.HasPrefix(strings.TrimSpace(line), "|") {
			continue
		}
		idx := map[string]int{}
		for j, c := range verifycmd.SplitRow(line) {
			idx[strings.ToLower(strings.TrimSpace(c))] = j
		}
		numCol, hasNum := idx["#"]
		cmdCol, hasCmd := idx["command"]
		if !hasNum || !hasCmd {
			continue
		}
		classCol, hasClass := idx["class"]
		var out []string
		for _, row := range lines[i+1:] {
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
			if _, err := strconv.Atoi(cell(numCol)); err != nil {
				continue
			}
			class := "check"
			if hasClass && cell(classCol) != "" {
				class = cell(classCol)
			}
			if class != "check" && class != "check:ci" {
				continue
			}
			if verifycmd.Refused(cell(cmdCol)) != "" {
				continue
			}
			out = append(out, verifycmd.UnescapePipes(verifycmd.Lift(cell(cmdCol))))
		}
		return out
	}
	return nil
}

// verifySection is the body under the exact `## Verify` heading.
func verifySection(content string) string {
	lines := strings.Split(content, "\n")
	start := -1
	for i, l := range lines {
		if strings.TrimSpace(l) == "## Verify" {
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
