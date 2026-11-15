package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/medici-finance/assay/statusgen/briefdoc"
	"github.com/medici-finance/assay/tools/desk/internal/verifycmd"
)

// count.go — the safe:count fact source. A count row is re-baselineable only when two facts
// both hold, each proven positively:
//
//   - countDrift: the row pins ONE count (`output is \`N\``), the command still runs with the
//     pinned exit, its output is now a larger bare integer M, and the same Expect with M in
//     place of N passes. That is CountShaped.
//   - countAdditionsOnly: git history shows every file the command names changed ONLY by
//     additions between the Evidence commit (the commit that recorded the row's last
//     passing Evidence line) and HEAD: no deleted line, no rename, no delete, no mode
//     change, no binary change, every target resolvable at both ends. That is OnlyAdditions.
//
// Every step fails closed. A missing Evidence line, an unresolvable commit, a shallow clone,
// a dirty target, a binary numstat (`-`) or any git error leaves OnlyAdditions false, and the
// row is refused, never re-baselined.

// digitRun matches one run of decimal digits.
var digitRun = regexp.MustCompile(`[0-9]+`)

// bareCount matches an output line that is only a non-negative decimal integer.
var bareCount = regexp.MustCompile(`^[0-9]+$`)

// countDrift reports whether a probed row's failure is an upward count drift and, if so, the
// count it measures now. rc and out are the probe's exit and combined output.
//
// The Expect must be decidable, name exactly one number (so which number is the count is
// not a guess), carry it as its `output is` line, set no `≥` floor, and the run must keep
// the exit the Expect pins. The output's last non-empty line must be a bare integer larger
// than the pinned one: a lower count is a loss, not growth. Finally the Expect rewritten to
// the new count must pass against the same run, so the rewrite --open writes is the rewrite
// proven here.
func countDrift(rc int, out string, row verifyRow) (now int, ok bool) {
	spec := parseExpect(row.ExpectCell)
	if !spec.decidable() || spec.minSet || rc != spec.exitCode {
		return 0, false
	}
	if !bareCount.MatchString(spec.wantLine) {
		return 0, false
	}
	pinned, err := strconv.Atoi(spec.wantLine)
	if err != nil || strconv.Itoa(pinned) != spec.wantLine {
		return 0, false
	}
	for _, cell := range []string{row.Expect, row.ExpectCell} {
		runs := digitRun.FindAllString(cell, -1)
		if len(runs) != 1 || runs[0] != spec.wantLine {
			return 0, false
		}
	}
	last := lastLine(out)
	if !bareCount.MatchString(last) {
		return 0, false
	}
	measured, err := strconv.Atoi(last)
	if err != nil || strconv.Itoa(measured) != last || measured <= pinned {
		return 0, false
	}
	if parseExpect(recount(row.ExpectCell, measured)).judge(true, rc, out) != judgePass {
		return 0, false
	}
	return measured, true
}

// recount rewrites the one number an Expect cell names to n. countDrift has already proven
// the cell names exactly one.
func recount(cell string, n int) string {
	return digitRun.ReplaceAllString(cell, strconv.Itoa(n))
}

// lastLine returns the last non-empty line of out, trimmed.
func lastLine(out string) string {
	lines := strings.Split(out, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if l := strings.TrimSpace(lines[i]); l != "" {
			return l
		}
	}
	return ""
}

// countAdditionsOnly proves, from local git history only, that every target the row's
// command names changed only by additions since the row's Evidence commit. It returns a
// one-line evidence record and ok=true only when every step positively holds.
func countAdditionsOnly(repoRoot string, row verifyRow) (evidence string, ok bool) {
	rel, ok := briefRel(repoRoot, row.BriefPath)
	if !ok {
		return "", false
	}
	// The brief's lines are blamed at HEAD, so the file on disk must be HEAD's, and the
	// history must be whole: a shallow boundary would blame every line on the graft.
	if st, ok := gitOutput(repoRoot, "-C", repoRoot, "status", "--porcelain", "--", rel); !ok || st != "" {
		return "", false
	}
	if sh, ok := gitOutput(repoRoot, "-C", repoRoot, "rev-parse", "--is-shallow-repository"); !ok || sh != "false" {
		return "", false
	}
	raw, err := os.ReadFile(filepath.Join(repoRoot, rel))
	if err != nil {
		return "", false
	}
	lines := strings.Split(string(raw), "\n")

	rowLine, ok := verifyRowLine(lines, row.Num)
	if !ok {
		return "", false
	}
	evLine, ok := evidencePassLine(lines, row)
	if !ok {
		return "", false
	}
	rowSha, ok := blameLine(repoRoot, rel, rowLine)
	if !ok {
		return "", false
	}
	evSha, ok := blameLine(repoRoot, rel, evLine)
	if !ok {
		return "", false
	}
	// The Verify row must be the one the Evidence recorded: it was last written at or before
	// the Evidence commit. A row edited after its Evidence has no Evidence of its own.
	if _, ok := gitOutput(repoRoot, "-C", repoRoot, "merge-base", "--is-ancestor", rowSha, evSha); !ok {
		return "", false
	}

	targets := countTargets(row.Command)
	if len(targets) == 0 {
		return "", false
	}
	for _, t := range targets {
		if !fileExists(repoRoot, t) {
			return "", false
		}
		for _, rev := range []string{evSha, "HEAD"} {
			if _, ok := gitOutput(repoRoot, "-C", repoRoot, "cat-file", "-e", rev+":"+t); !ok {
				return "", false
			}
		}
	}
	// The probe counted the working tree, so the targets must be exactly HEAD's: no
	// modified, untracked or ignored file under them.
	stArgs := append([]string{"-C", repoRoot, "status", "--porcelain", "--untracked-files=all", "--ignored", "--"}, targets...)
	if st, ok := gitOutput(repoRoot, stArgs...); !ok || st != "" {
		return "", false
	}

	if !onlyAddedOrModified(repoRoot, evSha, targets) {
		return "", false
	}
	added, ok := addedLinesOnly(repoRoot, evSha, targets)
	if !ok {
		return "", false
	}
	short := evSha
	if len(short) > 12 {
		short = short[:12]
	}
	return fmt.Sprintf("Evidence commit %s..HEAD: %d target(s), +%d -0 lines, no rename, delete, mode or binary change", short, len(targets), added), true
}

// briefRel returns the brief's path relative to repoRoot, or ok=false when it has none or
// it lies outside the checkout.
func briefRel(repoRoot, briefPath string) (string, bool) {
	if briefPath == "" {
		return "", false
	}
	absRoot, err := canonicalPath(repoRoot)
	if err != nil {
		return "", false
	}
	absBrief, err := canonicalPath(briefPath)
	if err != nil {
		return "", false
	}
	rel, err := filepath.Rel(absRoot, absBrief)
	if err != nil || rel == "." || filepath.IsAbs(rel) || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}
	return filepath.ToSlash(rel), true
}

// countTargets is the set of paths the command names, read with the same path tokenizer the
// rename proof uses.
func countTargets(command string) []string {
	var out []string
	for _, tok := range dedupe(pathToken.FindAllString(command, -1)) {
		tok = strings.Trim(tok, "./")
		if tok != "" {
			out = append(out, tok)
		}
	}
	return dedupe(out)
}

// tableLine finds, within lines[lo:hi], the lines of pipe-table rows whose `#` column is k,
// in tables whose header names every column in need. It returns their line indexes and
// cells, with the header's column map.
func tableLines(lines []string, lo, hi, k int, need ...string) (idxs []int, rows [][]string, cols []map[string]int) {
	for i := lo; i < hi; i++ {
		if !strings.HasPrefix(strings.TrimSpace(lines[i]), "|") {
			continue
		}
		head := headerIndex(lines[i])
		complete := true
		for _, n := range need {
			if _, ok := head[n]; !ok {
				complete = false
			}
		}
		j := i + 1
		for ; j < hi && strings.HasPrefix(strings.TrimSpace(lines[j]), "|"); j++ {
			if !complete || j == i+1 || sepRe.MatchString(lines[j]) {
				continue
			}
			cells := verifycmd.SplitRow(lines[j])
			c := head["#"]
			if c < len(cells) && strings.TrimSpace(cells[c]) == strconv.Itoa(k) {
				idxs, rows, cols = append(idxs, j), append(rows, cells), append(cols, head)
			}
		}
		i = j
	}
	return idxs, rows, cols
}

// verifyRowLine is the line index of Verify row k, read like parseVerifyRows reads it.
func verifyRowLine(lines []string, k int) (int, bool) {
	lo, hi := verifySectionBounds(lines)
	if lo < 0 {
		return 0, false
	}
	idxs, _, _ := tableLines(lines, lo, hi, k, "#", "command")
	if len(idxs) == 0 {
		return 0, false
	}
	return idxs[0], true
}

// evidencePassLine is the line index of the row's LATEST Evidence line: the last line for
// row k in an `## Evidence` table with `#`, `command` and `result` columns. That line must
// record a pass of the very command the row pins; a later failing line, a different command
// or no line at all is no Evidence commit.
func evidencePassLine(lines []string, row verifyRow) (int, bool) {
	lo, hi, ok := briefdoc.SectionBounds(lines, "Evidence", false)
	if !ok {
		return 0, false
	}
	idxs, rows, cols := tableLines(lines, lo, hi, row.Num, "#", "command", "result")
	if len(idxs) == 0 {
		return 0, false
	}
	last := len(idxs) - 1
	cells, head := rows[last], cols[last]
	cell := func(name string) string {
		if c := head[name]; c < len(cells) {
			return strings.TrimSpace(cells[c])
		}
		return ""
	}
	result := strings.Fields(strings.ToLower(stripInlineCode(cell("result"))))
	if len(result) == 0 || result[0] != "pass" {
		return 0, false
	}
	if verifycmd.UnescapePipes(verifycmd.Lift(cell("command"))) != row.Command {
		return 0, false
	}
	return idxs[last], true
}

// fullSha is a full object name.
var fullSha = regexp.MustCompile(`^[0-9a-f]{40}([0-9a-f]{24})?$`)

// blameLine returns the commit that last wrote line index i of rel at HEAD. A root commit is
// an ordinary commit here (--root); a boundary (a shallow graft) or an unparseable answer is
// no proof.
func blameLine(repoRoot, rel string, i int) (string, bool) {
	n := strconv.Itoa(i + 1)
	out, ok := gitOutput(repoRoot, "-C", repoRoot, "blame", "--porcelain", "--root", "-L", n+","+n, "HEAD", "--", rel)
	if !ok || out == "" {
		return "", false
	}
	lines := strings.Split(out, "\n")
	fields := strings.Fields(lines[0])
	if len(fields) < 3 || !fullSha.MatchString(fields[0]) || strings.Trim(fields[0], "0") == "" {
		return "", false
	}
	for _, l := range lines[1:] {
		if l == "boundary" {
			return "", false
		}
		if strings.HasPrefix(l, "\t") {
			break
		}
	}
	return fields[0], true
}

// onlyAddedOrModified reads the raw diff from the Evidence commit to HEAD over the targets,
// with rename detection on, and accepts only in-place modifications that keep the file mode
// (M with equal modes) and added files (A). A rename, copy, delete, type change or anything
// else refuses.
func onlyAddedOrModified(repoRoot, evSha string, targets []string) bool {
	args := append([]string{"-C", repoRoot, "diff", "--raw", "--no-abbrev", "-M", "--no-ext-diff", evSha, "HEAD", "--"}, targets...)
	out, ok := gitOutput(repoRoot, args...)
	if !ok {
		return false
	}
	for _, line := range strings.Split(out, "\n") {
		if line == "" {
			continue
		}
		meta, _, found := strings.Cut(line, "\t")
		f := strings.Fields(strings.TrimPrefix(meta, ":"))
		if !found || len(f) != 5 {
			return false
		}
		switch {
		case f[4] == "M" && f[0] == f[1]:
		case f[4] == "A":
		default:
			return false
		}
	}
	return true
}

// addedLinesOnly reads the numstat from the Evidence commit to HEAD over the targets and
// returns the total lines added. Any deleted line, a binary entry (`-`), an unparseable line
// or no added line at all (a higher count with no growth in the targets) refuses.
func addedLinesOnly(repoRoot, evSha string, targets []string) (int, bool) {
	args := append([]string{"-C", repoRoot, "diff", "--numstat", "-M", "--no-ext-diff", "--no-textconv", evSha, "HEAD", "--"}, targets...)
	out, ok := gitOutput(repoRoot, args...)
	if !ok {
		return 0, false
	}
	total := 0
	for _, line := range strings.Split(out, "\n") {
		if line == "" {
			continue
		}
		f := strings.SplitN(line, "\t", 3)
		if len(f) != 3 {
			return 0, false
		}
		add, err1 := strconv.Atoi(f[0])
		del, err2 := strconv.Atoi(f[1])
		if err1 != nil || err2 != nil || add < 0 || del != 0 {
			return 0, false
		}
		total += add
	}
	if total < 1 {
		return 0, false
	}
	return total, true
}
