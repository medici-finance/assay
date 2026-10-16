package main

// On-behalf-of principal stamp for App-run Evidence rows.
//
// statusgen's attribution lint requires every App-authored Evidence row dated at or after
// its cutover to carry the `on-behalf-of human:<subject>` annotation in its Runner cell
// (statusgen/attribution.go, principalAttributionProblems). verifyrun renders that
// annotation when it writes a witness row, but a row that reaches this tool any other way
// (a transcribed table, a re-run assembled from several witnesses) can arrive without it —
// and the only remedy was to hand-edit the Runner cell, which is exactly the kind of
// model-written attribution the annotation exists to replace.
//
// So the tool stamps it at ingest. The principal comes from ONE place: deskkit's
// ResolvePrincipal, the same config-home roster source the commit trailer
// (OnBehalfOfCommitSuffix) and verifyrun's own annotation read. It is never taken from a
// flag, an environment variable, or the content being landed. When it cannot be resolved
// the landing refuses (exit 5) and nothing is written — a placeholder is never stamped.
//
// What is touched, and what never is:
//   - Only data rows of a `## Evidence` table that THIS landing adds (a line not already on
//     the branch). A row already on the branch stays byte-identical, stamped or not.
//   - Only a Runner cell that names an App identity (a leading `<slug>[bot]` identifier) and
//     carries no `on-behalf-of` text at all. A human runner (`human:<name>`) needs no
//     principal; a cell that already carries an annotation — even one the lint will reject —
//     is left for the lint to judge, never rewritten.
//   - The annotation goes immediately before a trailing runner-source tag
//     (`(forge-identity)`, `(ci-env)`, `(git-config)`), which is the order verifyrun renders,
//     so the tag itself is never rewritten; with no such tag it is appended to the cell.
//
// Table reading mirrors statusgen's parseEvidenceRows (header = the row followed by a
// separator; Runner column by its header name, else the last cell; `\|` is cell content;
// closed HTML comments are not rows), so the row this stamps is the row the lint reads.

import (
	"regexp"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// evidenceSeparatorRe matches a Markdown table separator row (statusgen's separatorRowRe).
var evidenceSeparatorRe = regexp.MustCompile(`^[\s:|-]+$`)

// evidenceCommentRe matches a CLOSED HTML comment (statusgen's htmlCommentClosedRe).
var evidenceCommentRe = regexp.MustCompile(`(?s)<!--.*?-->`)

// runnerSourceTags are the trailing runner-source parentheticals verifyrun renders after
// the on-behalf-of annotation (statusgen/verifyrun.go runnerSource* constants).
var runnerSourceTags = []string{" (forge-identity)", " (ci-env)", " (git-config)"}

const onBehalfOfCellMarker = "on-behalf-of"

// principalSubjectFn resolves the subject the annotation names for a landing into repo.
// A seam only so a test can count resolutions; production is resolveStampSubject.
var principalSubjectFn = resolveStampSubject

// resolveStampSubject resolves the annotation's subject from the roster — the neutral name
// on a public target, the login on a target the roster states private (deskkit.Principal.Subject) — or
// refuses, wrapping ResolvePrincipal's own refusal so the operator sees which step needed it.
func resolveStampSubject(repo string) (string, error) {
	p, err := deskkit.ResolvePrincipal("", repo)
	if err != nil {
		return "", deskkit.Refused("refused: an App-run Evidence row in this landing carries no on-behalf-of " +
			"principal and the tool cannot stamp one — nothing was written: " + err.Error())
	}
	subject := p.Subject()
	if subject == "" {
		return "", deskkit.Refused("refused: an App-run Evidence row in this landing carries no on-behalf-of " +
			"principal and the roster resolved an empty one — nothing was written")
	}
	return subject, nil
}

// stampEvidencePrincipal returns content with the on-behalf-of annotation stamped into every
// App-run Evidence row content adds relative to base (see the file comment), plus a map from
// each stamped row's trimmed original text to its trimmed stamped text. resolve is called at
// most once, and only when some row needs stamping; its error is returned unchanged.
func stampEvidencePrincipal(content, base []byte, resolve func() (string, error)) ([]byte, map[string]string, error) {
	lines := strings.Split(string(content), "\n")
	start, end, ok := evidenceSpan(lines)
	if !ok {
		return content, nil, nil
	}
	onBranch := map[string]bool{}
	for _, l := range strings.Split(string(base), "\n") {
		onBranch[l] = true
	}
	// Blank closed comments out byte-for-byte (newlines kept) so offsets and line numbers in
	// the blanked view are the original's.
	section := strings.Join(lines[start:end], "\n")
	blanked := evidenceCommentRe.ReplaceAllStringFunc(section, func(m string) string {
		b := []byte(m)
		for i := range b {
			if b[i] != '\n' {
				b[i] = ' '
			}
		}
		return string(b)
	})
	view := strings.Split(blanked, "\n")

	subject := ""
	resolved := false
	var stamped map[string]string
	runnerIdx := -1
	for i := 0; i < len(view); i++ {
		line := strings.TrimSpace(view[i])
		if !strings.HasPrefix(line, "|") {
			runnerIdx = -1
			continue
		}
		if evidenceSeparatorRe.MatchString(strings.Trim(line, "|")) {
			continue
		}
		spans := cellSpans(view[i])
		if i+1 < len(view) && evidenceSeparatorRe.MatchString(strings.Trim(strings.TrimSpace(view[i+1]), "|")) {
			runnerIdx = -1
			for j, sp := range spans {
				if strings.ToLower(strings.TrimSpace(view[i][sp[0]:sp[1]])) == "runner" {
					runnerIdx = j
				}
			}
			i++
			continue
		}
		if len(spans) == 0 {
			continue
		}
		orig := lines[start+i]
		if onBranch[orig] {
			continue
		}
		sp := spans[len(spans)-1]
		if runnerIdx >= 0 && runnerIdx < len(spans) {
			sp = spans[runnerIdx]
		}
		// Decide on the comment-blanked view (what the lint reads); insert into the original
		// bytes at the same offset, so everything else in the line is left exactly as it was.
		cell := view[i][sp[0]:sp[1]]
		runner := strings.TrimSpace(cell)
		if !needsPrincipalStamp(runner) {
			continue
		}
		if !resolved {
			s, err := resolve()
			if err != nil {
				return nil, nil, err
			}
			subject, resolved = s, true
		}
		lead := len(cell) - len(strings.TrimLeft(cell, " \t"))
		pos := sp[0] + lead + len(runner)
		if tag, ok := trailingSourceTag(runner); ok {
			pos -= len(tag)
		}
		newLine := orig[:pos] + " (" + onBehalfOfCellMarker + " human:" + subject + ")" + orig[pos:]
		if stamped == nil {
			stamped = map[string]string{}
		}
		stamped[strings.TrimSpace(orig)] = strings.TrimSpace(newLine)
		lines[start+i] = newLine
	}
	if stamped == nil {
		return content, nil, nil
	}
	return []byte(strings.Join(lines, "\n")), stamped, nil
}

// needsPrincipalStamp reports whether a trimmed Runner cell names an App identity and carries
// no on-behalf-of text at all (statusgen's isAppRunnerToken / isHumanRunnerToken reading).
func needsPrincipalStamp(runner string) bool {
	if runner == "" || strings.HasPrefix(strings.ToLower(runner), "human:") {
		return false
	}
	if strings.Contains(strings.ToLower(runner), onBehalfOfCellMarker) {
		return false
	}
	id := runner
	if i := strings.IndexAny(id, " ("); i >= 0 {
		id = id[:i]
	}
	return strings.HasSuffix(id, "[bot]")
}

// trailingSourceTag reports the runner-source tag a trimmed Runner cell ends with, if any. The
// annotation goes before it — verifyrun's rendering order — so the tag is never rewritten.
func trailingSourceTag(runner string) (string, bool) {
	for _, tag := range runnerSourceTags {
		if strings.HasSuffix(runner, tag) {
			return tag, true
		}
	}
	return "", false
}

// evidenceSpan locates the `## Evidence` section's lines: [start,end), end being the next
// `## ` heading or the end of content (statusgen's evidenceSectionSpan).
func evidenceSpan(lines []string) (start, end int, ok bool) {
	start = -1
	for i, l := range lines {
		if strings.TrimSpace(l) == "## Evidence" {
			start = i + 1
			break
		}
	}
	if start < 0 {
		return 0, 0, false
	}
	end = len(lines)
	for i := start; i < len(lines); i++ {
		if strings.HasPrefix(strings.TrimSpace(lines[i]), "## ") {
			end = i
			break
		}
	}
	return start, end, true
}

// cellSpans returns each table cell's [start,end) byte offsets in line, splitting on `|` but
// not on an escaped `\|`, and dropping at most one blank cell at each end — the cell
// boundaries statusgen's splitRowEscaped reads.
func cellSpans(line string) [][2]int {
	var spans [][2]int
	cur := 0
	for i := 0; i < len(line); i++ {
		switch {
		case line[i] == '\\' && i+1 < len(line) && line[i+1] == '|':
			i++
		case line[i] == '|':
			spans = append(spans, [2]int{cur, i})
			cur = i + 1
		}
	}
	spans = append(spans, [2]int{cur, len(line)})
	if len(spans) > 0 && strings.TrimSpace(line[spans[0][0]:spans[0][1]]) == "" {
		spans = spans[1:]
	}
	if n := len(spans); n > 0 && strings.TrimSpace(line[spans[n-1][0]:spans[n-1][1]]) == "" {
		spans = spans[:n-1]
	}
	return spans
}

// stampFragment applies stampEvidencePrincipal's line map to a --brief-path fragment, so the
// fragment merged into the brief is the stamped one and the block-level idempotency check
// compares like with like.
func stampFragment(fragment []byte, stamped map[string]string) []byte {
	lines := strings.Split(string(fragment), "\n")
	for i, l := range lines {
		key := strings.TrimSpace(l)
		if to, ok := stamped[key]; ok {
			lines[i] = strings.Replace(l, key, to, 1)
		}
	}
	return []byte(strings.Join(lines, "\n"))
}
