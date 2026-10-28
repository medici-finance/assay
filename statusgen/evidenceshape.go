package main

// evidenceshape.go — verify-reset/07 Task 1: which PRs are never a brief's
// delivering PR, judged by DIFF SHAPE alone.
//
// A verify Evidence PR lands a brief's `## Evidence` rows, moves its README
// row's status cells and adds a verify-outcome record. It carries `Brief: <id>`,
// so the trailer search finds it, and it is the newest change to the brief file,
// so the history walk meets it first. It merges on a human approval, never the
// reviewer App's, so treating it as the delivery refused every PR-landed PASS.
// Before verify-reset/07 the exclusion was keyed to the AUTHOR (the roster's
// verifier App): the same diff from any other author was a candidate, and the
// flip stalled on it.
//
// evidenceOnlyReason reads the diff and nothing else. A PR is Evidence-only when
// its file list is non-empty, nothing was renamed, and every changed path is
// one of:
//
//   - STATUS.md, the generated board (a status-only PR);
//   - a verify-outcome record: anything under outcomeRecordsDir, or a legacy
//     outcome log matching verifyOutcomesGlob (both named in verifyoutcomes.go,
//     the module's one place that spells those paths);
//   - a stream README (docs/streams/<stream>/README.md) whose every changed line
//     is a briefs-table row rewritten in place: same row, same `#`, and only the
//     Status, Verified and Reviewed cells differ;
//   - a brief file (docs/streams/<stream>/brief-*.md) whose every changed line
//     sits inside its `## Evidence` section at the PR's head, none of them a
//     `## ` heading (which would move the section's edge). The section is
//     located exactly as the brief parser locates it (splitFrontmatter, then
//     evidenceSectionSpan over the body), so a line the parser does not read as
//     Evidence — a frontmatter line, say — is never classified as Evidence.
//
// Every changed path must already be canonical (forward slashes, no `.` or
// `..` element, no doubled slash); a path the classifier would have to
// normalise first is not Evidence-only.
//
// Anything the forge did not report is NOT Evidence-only: a path with no patch
// (GitHub omits one for a binary or very large diff), a README or brief whose
// head content was not read, a patch that does not parse. In autoflip such a PR
// stays an ordinary candidate, judged on its own App approval: missing data
// never skips an approval. Reconcile instead requires the missing shape before
// admitting an implementation witness (briefevidence.go); it reports unknown
// and holds writes when that required read cannot be completed.
//
// The classifier takes no identity. TestEvidenceShapeNeverReadsIdentity holds
// it, and candidateGate, to that: neither may consult who authored a PR. The
// flip therefore no longer checks who authored an Evidence-shaped PR: what
// stands between such a PR and main is the default branch's own review rule,
// the same control that admits any other change.

import (
	"fmt"
	pathpkg "path"
	"regexp"
	"strconv"
	"strings"
)

// evidenceOnlyWhy is the walked-list note for a PR set aside by diff shape.
const evidenceOnlyWhy = "Evidence-only or status-only by diff shape — not a delivery candidate, and no reviewer-App approval is asked of it, exactly as for a direct-to-main Evidence commit"

// evidenceOnlyReason reports whether shape is Evidence-only or status-only.
// When it is not, why names the first path that disqualifies it.
func evidenceOnlyReason(shape prShape) (ok bool, why string) {
	if len(shape.Files) == 0 {
		return false, "no changed files were read"
	}
	if len(shape.RenamedFrom) > 0 {
		return false, fmt.Sprintf("it renames %s", strings.Join(shape.RenamedFrom, ", "))
	}
	for _, f := range shape.Files {
		if !isCanonicalRepoPath(f) {
			return false, fmt.Sprintf("it changes %q, which is not a canonical repository path", f)
		}
		if why := evidenceOnlyPathProblem(shape, f); why != "" {
			return false, why
		}
	}
	return true, ""
}

// isCanonicalRepoPath reports whether p is in the form the forge reports a
// path: forward slashes only, and unchanged by path.Clean (no `.` or `..`
// element, no doubled or trailing slash). The classifier refuses anything else
// rather than normalising it: a prefix test on an unclean path can be steered.
func isCanonicalRepoPath(p string) bool {
	return !strings.Contains(p, "\\") && pathpkg.Clean(p) == p
}

// evidenceOnlyPathProblem returns "" when changed path p is Evidence-shaped in
// this PR, else why it is not.
func evidenceOnlyPathProblem(shape prShape, p string) string {
	switch {
	case p == "STATUS.md", isVerifyOutcomePath(p):
		return ""
	case isStreamReadmePath(p):
		patch, head, why := patchAndHead(shape, p)
		if why != "" {
			return why
		}
		return readmeStatusOnlyProblem(p, patch, head)
	case isStreamBriefPath(p):
		patch, head, why := patchAndHead(shape, p)
		if why != "" {
			return why
		}
		return briefEvidenceOnlyProblem(p, patch, head)
	}
	return fmt.Sprintf("it changes %s, which is not an Evidence section, a status cell or a verify-outcome record", p)
}

func isVerifyOutcomePath(p string) bool {
	if strings.HasPrefix(p, outcomeRecordsDir+"/") {
		return true
	}
	m, err := pathpkg.Match("docs/streams/"+verifyOutcomesGlob, p)
	return err == nil && m
}

func isStreamReadmePath(p string) bool {
	m, err := pathpkg.Match("docs/streams/*/README.md", p)
	return err == nil && m
}

func isStreamBriefPath(p string) bool {
	m, err := pathpkg.Match("docs/streams/*/brief-*.md", p)
	return err == nil && m
}

func patchAndHead(shape prShape, p string) (patch, head, why string) {
	patch, ok := shape.Patches[p]
	if !ok || strings.TrimSpace(patch) == "" {
		return "", "", fmt.Sprintf("the forge reported no patch for %s, so its hunks cannot be shown to be Evidence-only", p)
	}
	head, ok = shape.Heads[p]
	if !ok {
		return "", "", fmt.Sprintf("%s was not read at the PR's head, so its hunks cannot be placed", p)
	}
	return patch, head, ""
}

// diffLine is one changed line of a unified diff. pos is its new-side line
// number for an addition, and for a deletion the new-side line it sat before.
type diffLine struct {
	add  bool
	text string
	pos  int
}

// diffChange is one run of consecutive changed lines.
type diffChange struct{ del, add []diffLine }

var hunkHeaderRe = regexp.MustCompile(`^@@ -(\d+)(?:,(\d+))? \+(\d+)(?:,(\d+))? @@`)

// parsePatchChanges reads a forge patch (hunks only, no file headers) into its
// runs of changed lines. Anything that is not a hunk header, a context, added
// or deleted line, or the no-newline marker is an error.
func parsePatchChanges(patch string) ([]diffChange, error) {
	var out []diffChange
	var cur *diffChange
	flush := func() {
		if cur != nil && (len(cur.del) > 0 || len(cur.add) > 0) {
			out = append(out, *cur)
		}
		cur = nil
	}
	pos, inHunk := 0, false
	for _, l := range strings.Split(strings.TrimRight(patch, "\n"), "\n") {
		if m := hunkHeaderRe.FindStringSubmatch(l); m != nil {
			flush()
			n, err := strconv.Atoi(m[3])
			if err != nil {
				return nil, err
			}
			newLen := 1
			if m[4] != "" {
				newLen, _ = strconv.Atoi(m[4])
			}
			// A zero-length new side ("+5,0") names the line BEFORE the gap.
			if newLen == 0 {
				n++
			}
			pos, inHunk = n, true
			continue
		}
		if !inHunk {
			return nil, fmt.Errorf("patch line %q precedes any hunk header", l)
		}
		switch {
		case strings.HasPrefix(l, `\`):
			// "\ No newline at end of file"
		case strings.HasPrefix(l, "+"):
			if cur == nil {
				cur = &diffChange{}
			}
			cur.add = append(cur.add, diffLine{add: true, text: l[1:], pos: pos})
			pos++
		case strings.HasPrefix(l, "-"):
			if cur == nil {
				cur = &diffChange{}
			}
			cur.del = append(cur.del, diffLine{text: l[1:], pos: pos})
		case strings.HasPrefix(l, " "), l == "":
			flush()
			pos++
		default:
			return nil, fmt.Errorf("patch line %q is not a diff line", l)
		}
	}
	flush()
	if len(out) == 0 {
		return nil, fmt.Errorf("the patch changes no line")
	}
	return out, nil
}

// briefEvidenceOnlyProblem: every changed line must sit inside the head's
// `## Evidence` section, as briefEvidenceLines locates it, and none may itself
// be a `## ` heading.
func briefEvidenceOnlyProblem(p, patch, head string) string {
	changes, err := parsePatchChanges(patch)
	if err != nil {
		return fmt.Sprintf("its patch for %s could not be read (%v)", p, err)
	}
	first, end, why := briefEvidenceLines(head)
	if why != "" {
		return fmt.Sprintf("%s %s at the PR's head", p, why)
	}
	for _, c := range changes {
		for _, d := range append(append([]diffLine(nil), c.del...), c.add...) {
			if isSectionHeading(d.text) {
				return fmt.Sprintf("it changes the section heading %q in %s", strings.TrimSpace(d.text), p)
			}
			inside := d.pos >= first && d.pos < end
			if !d.add {
				inside = d.pos >= first && d.pos <= end // a deletion sits before line pos
			}
			if !inside {
				return fmt.Sprintf("it changes line %d of %s, outside the `## Evidence` section", d.pos, p)
			}
		}
	}
	return ""
}

// briefEvidenceLines returns the 1-based line range of a brief's `## Evidence`
// content in the whole file: lines first..end-1, end being the next section
// heading's line (or one past the last line). It splits the frontmatter with
// splitFrontmatter and finds the section with evidenceSectionSpan, the two
// calls the brief parser makes, and offsets the body's lines back to file
// lines. A brief the parser would read no Evidence section from — no
// frontmatter, an unterminated one, or no `## Evidence` heading in the body —
// yields a why and no range.
func briefEvidenceLines(head string) (first, end int, why string) {
	content := strings.ReplaceAll(head, "\r\n", "\n")
	_, body, err := splitFrontmatter(content)
	if err != nil {
		return 0, 0, fmt.Sprintf("has no frontmatter the brief parser can split (%v), so it reads no `## Evidence` section from it", err)
	}
	start, stop, ok := evidenceSectionSpan(strings.Split(body, "\n"))
	if !ok {
		return 0, 0, "has no `## Evidence` section"
	}
	// The body is the file's lines after the closing fence: offset lines precede it.
	offset := strings.Count(content, "\n") - strings.Count(body, "\n")
	return offset + start + 1, offset + stop + 1, ""
}

// readmeStatusOnlyProblem: every change must rewrite briefs-table rows in place
// (as many deleted as added lines, paired in order), each pair keeping its `#`
// and every cell but Status, Verified and Reviewed, and the new row must sit in
// the head's briefs table.
func readmeStatusOnlyProblem(p, patch, head string) string {
	changes, err := parsePatchChanges(patch)
	if err != nil {
		return fmt.Sprintf("its patch for %s could not be read (%v)", p, err)
	}
	cols, rowsFrom, rowsTo := briefsTableAt(head)
	if cols == nil {
		return fmt.Sprintf("%s has no briefs table at the PR's head", p)
	}
	// Status and Verified are what a verify landing moves; Reviewed is free
	// because the close-verify landing stamps it too. The model flip reads none
	// of the three to decide a delivery: they are board bookkeeping.
	free := map[int]bool{}
	for _, name := range []string{"status", "verified", "reviewed"} {
		if j, ok := cols[name]; ok {
			free[j] = true
		}
	}
	for _, c := range changes {
		if len(c.del) != len(c.add) {
			return fmt.Sprintf("it adds or removes lines in %s rather than rewriting status cells in place", p)
		}
		for i := range c.add {
			oldRow, newRow := c.del[i], c.add[i]
			if newRow.pos < rowsFrom || newRow.pos > rowsTo {
				return fmt.Sprintf("it changes line %d of %s, outside the briefs table", newRow.pos, p)
			}
			oc, nc := splitRow(oldRow.text), splitRow(newRow.text)
			if !strings.HasPrefix(strings.TrimSpace(oldRow.text), "|") || len(oc) != len(nc) || len(nc) <= cols["#"] {
				return fmt.Sprintf("it changes line %d of %s into a different row shape", newRow.pos, p)
			}
			for j := range nc {
				if !free[j] && strings.TrimSpace(oc[j]) != strings.TrimSpace(nc[j]) {
					return fmt.Sprintf("it changes a cell other than Status, Verified or Reviewed on line %d of %s", newRow.pos, p)
				}
			}
		}
	}
	return ""
}

// briefsTableAt finds the first table whose header has `#`, `Brief` and `Status`
// columns (the table flipRowToDone rewrites) and returns its column index and
// the 1-based line range of its rows.
func briefsTableAt(raw string) (cols map[string]int, from, to int) {
	lines := strings.Split(raw, "\n")
	for i, line := range lines {
		if !strings.HasPrefix(strings.TrimSpace(line), "|") {
			continue
		}
		idx := map[string]int{}
		for j, c := range splitRow(line) {
			idx[strings.ToLower(strings.TrimSpace(c))] = j
		}
		_, a := idx["#"]
		_, b := idx["brief"]
		_, s := idx["status"]
		if !a || !b || !s {
			continue
		}
		from = i + 3 // header on i+1, separator on i+2 (1-based)
		to = from - 1
		for k := i + 2; k < len(lines) && strings.HasPrefix(strings.TrimSpace(lines[k]), "|"); k++ {
			to = k + 1
		}
		return idx, from, to
	}
	return nil, 0, 0
}
