package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// registerLinkRe matches a markdown link whose text is a typed register ID:
//
//	[F-15](../findings/2026-07-09-slug.md)
//	[I-23](../intake/2026-07-10-slug.md)
//
// Group 1 = link text (the ID), group 2 = target URL.
var registerLinkRe = regexp.MustCompile(`\[(F-\d+(?:-[a-z])?|I-\d+)\]\(([^)]+)\)`)

// bareRegisterRe matches a bare (unlinked) register ID token. Used by the
// backfill to find candidates for linking.
var bareRegisterRe = regexp.MustCompile(`\b(F-\d+(?:-[a-z])?|I-\d+)\b`)

// registerViewNames are the generated view files that should never be link
// targets.
var registerViewNames = map[string]bool{
	"FINDINGS.md": true,
	"INTAKE.md":   true,
}

// buildRegisterMap reads all per-entry files under docs/streams/findings/ and
// docs/streams/intake/ and returns two maps: ID → path relative to the repo
// root. An entry file whose frontmatter id is missing or unparseable is
// silently skipped (the register-integrity check in registers.go already flags
// duplicate IDs; this function just maps what it can).
func buildRegisterMap(root string) (findings, intake map[string]string) {
	findings = map[string]string{}
	intake = map[string]string{}

	fDir := filepath.Join(root, "docs", "streams", "findings")
	if entries, err := os.ReadDir(fDir); err == nil {
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
				continue
			}
			raw, err := os.ReadFile(filepath.Join(fDir, e.Name()))
			if err != nil {
				continue
			}
			fe, err := parseFindingFile(raw)
			if err != nil || fe.ID == "" {
				continue
			}
			findings[fe.ID] = filepath.Join("docs", "streams", "findings", e.Name())
		}
	}

	iDir := filepath.Join(root, "docs", "streams", "intake")
	if entries, err := os.ReadDir(iDir); err == nil {
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
				continue
			}
			raw, err := os.ReadFile(filepath.Join(iDir, e.Name()))
			if err != nil {
				continue
			}
			ie, err := parseIntakeFile(raw)
			if err != nil || ie.ID == "" {
				continue
			}
			intake[ie.ID] = filepath.Join("docs", "streams", "intake", e.Name())
		}
	}

	return findings, intake
}

// registerRefProblems checks every markdown link whose text is a register ID
// (F-NN / I-NN) against the register entry files. Returns hard PROBLEMs and
// advisory NOTICEs per the brief-33 spec:
//
//	(a) target file must exist
//	(b) target file's frontmatter id: must EQUAL link text
//	(c) linking to a generated view (FINDINGS.md / INTAKE.md) is a NOTICE
//
// Bare (unlinked) references are never checked — they stay legal.
func registerRefProblems(root string, files []string) (problems, notices []string) {
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		rel, _ := filepath.Rel(root, f)
		content := string(raw)

		// Skip code-fenced blocks: find their spans and exclude them.
		fenceFree := stripFences(content)

		for _, m := range registerLinkRe.FindAllStringSubmatch(fenceFree, -1) {
			linkText := m[1] // F-15 or I-23
			target := strings.TrimSpace(m[2])

			// Resolve anchor-free target path.
			if i := strings.Index(target, "#"); i >= 0 {
				target = target[:i]
			}
			if target == "" {
				continue
			}
			// External citations: a register reference is always a repo-relative
			// path to a per-entry file; an F-NN/I-NN link whose target is an
			// external URL is prose citing the register, not a register link, so
			// it can't be id-cross-checked. Skip it (surfaced
			// once outbound docs entered docFiles' scope).
			if strings.HasPrefix(target, "http") {
				continue
			}

			// Check (c): link to generated view.
			base := filepath.Base(target)
			if registerViewNames[base] {
				notices = append(notices, fmt.Sprintf(
					"%s: register reference [%s](%s) links to generated view %s — link the per-entry file under findings/ or intake/ instead",
					rel, linkText, target, base))
				continue
			}

			// Resolve target relative to the brief's directory.
			resolved := filepath.Join(filepath.Dir(f), target)
			resolved = filepath.Clean(resolved)
			// Defend against path traversal: the resolved file must stay
			// within the repo's docs/streams/ tree. A relative link
			// escaping to e.g. ../../../etc is a PROBLEM.
			docsRoot := filepath.Join(root, "docs", "streams")
			if !strings.HasPrefix(resolved, docsRoot+string(filepath.Separator)) && resolved != docsRoot {
				problems = append(problems, fmt.Sprintf(
					"%s: register reference [%s](%s) resolves outside docs/streams/, path traversal refused",
					rel, linkText, target))
				continue
			}
			if !fileExists(resolved) {
				problems = append(problems, fmt.Sprintf(
					"%s: register reference [%s](%s) points to a file that does not exist",
					rel, linkText, target))
				continue
			}

			// Check (b): target file's frontmatter id: must equal link text.
			// Register links must target per-entry files under findings/ or
			// intake/ — linking to any other file with a register-ID link
			// text is a PROBLEM (the link convention is register-ID → own
			// entry file; linking a register ID to a brief or other non-entry
			// file is a convention violation and can't be id-cross-checked).
			var entryID string
			var entryParseErr error
			if strings.Contains(resolved, "/findings/") {
				if raw, err := os.ReadFile(resolved); err == nil {
					if fe, err := parseFindingFile(raw); err == nil {
						entryID = fe.ID
					} else {
						entryParseErr = err
					}
				} else {
					entryParseErr = err
				}
			} else if strings.Contains(resolved, "/intake/") {
				if raw, err := os.ReadFile(resolved); err == nil {
					if ie, err := parseIntakeFile(raw); err == nil {
						entryID = ie.ID
					} else {
						entryParseErr = err
					}
				} else {
					entryParseErr = err
				}
			} else {
				problems = append(problems, fmt.Sprintf(
					"%s: register reference [%s](%s) targets a file outside findings/ or intake/ — register links must point to their own per-entry file",
					rel, linkText, target))
				continue
			}
			if entryParseErr != nil {
				problems = append(problems, fmt.Sprintf(
					"%s: register reference [%s](%s) targets a file whose frontmatter cannot be parsed: %v",
					rel, linkText, target, entryParseErr))
				continue
			}
			if entryID == "" {
				problems = append(problems, fmt.Sprintf(
					"%s: register reference [%s](%s) targets a register entry with no id: field in frontmatter",
					rel, linkText, target))
				continue
			}
			if entryID != linkText {
				problems = append(problems, fmt.Sprintf(
					"%s: register reference [%s](%s) points to entry whose frontmatter id: is %q — link text must equal the entry's id",
					rel, linkText, target, entryID))
			}
		}
	}
	sort.Strings(problems)
	sort.Strings(notices)
	return problems, notices
}

// stripFences returns the content with all code-fenced blocks (``` … ```)
// replaced by blank lines of equal length, so regex matches that span fence
// boundaries are naturally broken while line numbering is preserved.
func stripFences(content string) string {
	lines := strings.Split(content, "\n")
	var out []string
	inFence := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			inFence = !inFence
			out = append(out, strings.Repeat(" ", len(line)))
			continue
		}
		if inFence {
			out = append(out, strings.Repeat(" ", len(line)))
		} else {
			out = append(out, line)
		}
	}
	return strings.Join(out, "\n")
}

// ---------------------------------------------------------------------------
// Backfill: rewrite bare F-NN / I-NN tokens in brief files to linked form.
// ---------------------------------------------------------------------------

// backfillRegisterRefs scans every brief-*.md under docs/streams/*/ and
// rewrites bare register-ID tokens (F-NN / I-NN) to markdown links pointing at
// the per-entry file. It is idempotent — already-linked refs are left
// untouched. Code-fenced blocks, inline code spans, and Verify table rows are
// skipped. Returns the total number of replacements made.
func backfillRegisterRefs(root string) (int, error) {
	fMap, iMap := buildRegisterMap(root)

	streamsDir := filepath.Join(root, "docs", "streams")
	entries, err := os.ReadDir(streamsDir)
	if err != nil {
		return 0, fmt.Errorf("reading streams dir: %w", err)
	}

	total := 0
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if reservedRegisterNames[e.Name()] {
			continue
		}
		briefs, _ := filepath.Glob(filepath.Join(streamsDir, e.Name(), "brief-*.md"))
		for _, bp := range briefs {
			n, err := backfillOneFile(bp, fMap, iMap)
			if err != nil {
				return total, fmt.Errorf("%s: %w", bp, err)
			}
			total += n
		}
	}
	return total, nil
}

// backfillOneFile rewrites one brief file and writes it back. Returns the
// number of replacements made.
func backfillOneFile(path string, fMap, iMap map[string]string) (int, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	content := string(raw)

	lines := strings.Split(content, "\n")
	var out []string
	inFence := false
	inVerifyTable := false
	count := 0
	// verifyHeaderRe detects the column-header row of a Verify table.
	verifyHeaderRe := regexp.MustCompile(`\|\s*#\s*\|\s*Command\s*\|\s*Expect\s*\|`)

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Code-fence toggle.
		if strings.HasPrefix(trimmed, "```") {
			inFence = !inFence
			out = append(out, line)
			continue
		}
		if inFence {
			out = append(out, line)
			continue
		}

		// Verify-table tracking: skip data rows after the header.
		if inVerifyTable {
			if strings.HasPrefix(trimmed, "|") {
				out = append(out, line)
				continue
			}
			inVerifyTable = false
		}
		if verifyHeaderRe.MatchString(trimmed) {
			inVerifyTable = true
			out = append(out, line)
			continue
		}

		// Replace bare refs on this line.
		newLine, n := replaceBareRefs(line, fMap, iMap, path)
		count += n
		out = append(out, newLine)
	}

	if count > 0 {
		newContent := strings.Join(out, "\n")
		if err := os.WriteFile(path, []byte(newContent), 0o644); err != nil {
			return count, err
		}
	}
	return count, nil
}

// replaceBareRefs finds bare F-NN/I-NN tokens on a single line and replaces
// them with markdown links. It skips tokens that are already inside a markdown
// link [text](url) or inside an inline code span `text`.
func replaceBareRefs(line string, fMap, iMap map[string]string, briefPath string) (string, int) {
	// Find all markdown-link spans on this line: [text](url)
	type span struct{ start, end int }
	var linkSpans []span
	for _, m := range mdLinkRe.FindAllStringSubmatchIndex(line, -1) {
		// m[0],m[1] = full match [text](url) — the entire thing is linked already
		linkSpans = append(linkSpans, span{m[0], m[1]})
	}

	// Find all inline-code spans: `text`
	backtickRe := regexp.MustCompile("`[^`]*`")
	var codeSpans []span
	for _, m := range backtickRe.FindAllStringIndex(line, -1) {
		codeSpans = append(codeSpans, span{m[0], m[1]})
	}

	// Helper: is position pos inside any span?
	inside := func(pos int, spans []span) bool {
		for _, s := range spans {
			if pos >= s.start && pos < s.end {
				return true
			}
		}
		return false
	}

	// Find all bare register refs and replace them (working right-to-left
	// so offsets stay valid).
	type replacement struct {
		start, end int
		id         string
		linkTarget string
	}
	var repls []replacement

	for _, m := range bareRegisterRe.FindAllStringSubmatchIndex(line, -1) {
		start, end := m[0], m[1]
		id := line[start:end]

		// Skip if inside a link or code span.
		if inside(start, linkSpans) || inside(start, codeSpans) {
			continue
		}

		// Find the entry file.
		var entryPath string
		if strings.HasPrefix(id, "F-") {
			entryPath = fMap[id]
		} else {
			entryPath = iMap[id]
		}
		if entryPath == "" {
			continue // no entry file → leave bare
		}

		// Compute relative path from the brief's directory.
		briefDir := filepath.Dir(briefPath)
		rel, err := filepath.Rel(briefDir, entryPath)
		if err != nil {
			continue
		}

		repls = append(repls, replacement{start, end, id, rel})
	}

	if len(repls) == 0 {
		return line, 0
	}

	// Apply replacements left-to-right (building a new string — no in-place
	// position shifting).
	var result strings.Builder
	prev := 0
	// Sort by start ascending (leftmost first).
	sort.Slice(repls, func(i, j int) bool { return repls[i].start < repls[j].start })
	for _, r := range repls {
		result.WriteString(line[prev:r.start])
		fmt.Fprintf(&result, "[%s](%s)", r.id, r.linkTarget)
		prev = r.end
	}
	result.WriteString(line[prev:])

	return result.String(), len(repls)
}
