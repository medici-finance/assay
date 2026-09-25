package main

import (
	"fmt"
	"path"
	"strings"
)

// ---- which ADDED diff lines are CLAIMS (the --corroborate scope) ------------------
//
// THE DEFECT CLASS THIS FILE CLOSES (#1395): QUOTED NOTATION READ AS A CLAIM.
//
// --corroborate reads the PR's diff and treats an added line carrying
// `human:<name>` (the stamp lane) or "<name>'s sign-off ... #N" (the citation lane)
// as a CLAIM that a human acted. Until this file, each lane walked the diff with
// its own copy of one loop, and that loop classified a line by its leading "+"
// alone. Some "+" lines are not claims — they QUOTE the notation:
//
//   - The REMOVED side of a diff embedded in a committed `.patch` / `.diff` file.
//     Adding such a file adds every one of its lines with a "+" in front, so a
//     line the patch DELETES arrives as "+-...". A deleted line asserts nothing;
//     the patch applied REMOVES it. This binds BOTH lanes (and every other diff
//     walker in the package — see addedDiffLines).
//   - Stamp-shaped DATA in a TEST source file. statusgen reads human:<name>
//     stamps only out of RECORD files — the Markdown boards, briefs, decision
//     records and registers (and their frontmatter), and the JSONL ledgers. A
//     `human:<name>` in a test's fixture data is documentation of the notation:
//     the test asserts on it, it is never shipped as a record, and no board, brief
//     or gate reads it as a sign-off. The skip is scoped to TEST source only
//     (#1395 asks for test-fixture contexts), recognised by the test-file NAME
//     CONVENTION of a closed program-source extension (isTestSourceFile). A
//     non-test program or script — one whose job may be to WRITE a stamp into a
//     record, on a PR or outside one — is scanned exactly as before.
//   - A `#` line in a YAML file. Outside a block scalar it is a YAML comment,
//     dropped by every YAML parser and never a value; inside one (a workflow
//     `run: |` script, say) it is value text, but no statusgen stamp reader parses
//     a YAML file at all, so in neither case is it a record claim.
//
//     These two bind the STAMP lane only (stampClaimSurface); the citation lane is
//     scoped to all tracked prose by design (a ruling claim in a code comment is
//     still durable prose a reader may trust), so it keeps reading them.
//
// WHY THIS IS A NARROWING AND NOT AN EVASION SURFACE. Every exclusion below is
// decided by the CONTENT FORMAT, not by a path name: no directory, `testdata`,
// `fixtures` or `examples` heuristic is consulted (the declared-corpus mechanism in
// fixturecorpus.go stays the only way to exclude a Markdown record subtree). Each
// one is FAIL-CLOSED in the same direction:
//
//   - only a TEST source file (isTestSourceFile) is skipped; every non-test
//     program or script, and every extension not in stampInertSourceExt, is
//     scanned exactly as before — Markdown, JSONL, YAML data lines, extensionless
//     files, and any format added later all stay gated;
//   - in a YAML file only a line whose first non-blank character is `#` is
//     skipped; every value line still is scanned;
//   - in a `.patch` / `.diff` only the removed side of a HUNK is skipped; the
//     ADDED side (what the patch will write when applied), its context, and any
//     text outside a hunk (a `git format-patch` commit-message preamble) stay
//     gated.
//
// And each one is VISIBLE: runCorroborate prints a NOT-A-CLAIM notice for every
// stamp or citation a skipped line would have produced (quotedClaimNotices), so a
// skip is reviewable in the run log instead of silently missing from it.

// stampInertSourceExt lists the program-source / script extensions whose TEST
// files (isTestSourceFile) the stamp lane skips. It is a CLOSED list on purpose:
// an extension that is not here is scanned (the fail-closed direction), so
// growing the list is a reviewed widening, never a side effect.
var stampInertSourceExt = map[string]bool{
	".go": true, ".sh": true, ".bash": true, ".zsh": true, ".ps1": true,
	".py": true, ".js": true, ".mjs": true, ".cjs": true, ".ts": true,
	".rb": true, ".rs": true,
}

// testSourceStemSuffixes are the test-file name conventions isTestSourceFile
// accepts, matched on the file's stem (its base name without the extension):
// `x_test.go` (the form the Go toolchain itself compiles only under `go test`),
// and the `x.test.sh` / `x.test.ts` / `x_test.py` shapes other ecosystems use. A
// CLOSED list: a stem that matches neither is not a test file here.
var testSourceStemSuffixes = []string{"_test", ".test"}

// isTestSourceFile reports whether file is a TEST source file: its extension is
// on the closed stampInertSourceExt list AND its stem ends in a test-file name
// convention (testSourceStemSuffixes). It reads the file's own NAME only — never
// a directory, so no `testdata` / `fixtures` / `tests` path segment makes a
// non-test file one. conv names the matched convention, for the notice.
func isTestSourceFile(file string) (ok bool, conv string) {
	base := path.Base(file)
	ext := strings.ToLower(path.Ext(base))
	if !stampInertSourceExt[ext] {
		return false, ""
	}
	stem := strings.TrimSuffix(base, path.Ext(base))
	for _, suf := range testSourceStemSuffixes {
		if strings.HasSuffix(strings.ToLower(stem), suf) && len(stem) > len(suf) {
			return true, "*" + suf + ext
		}
	}
	return false, ""
}

// isEmbeddedPatchFile reports whether a PR file is itself a committed unified
// diff, whose own "-" lines are the removed side of the change it carries.
func isEmbeddedPatchFile(file string) bool {
	switch strings.ToLower(path.Ext(file)) {
	case ".patch", ".diff":
		return true
	}
	return false
}

// isYAMLFile reports whether a PR file is YAML by extension.
func isYAMLFile(file string) bool {
	switch strings.ToLower(path.Ext(file)) {
	case ".yml", ".yaml":
		return true
	}
	return false
}

// stampClaimSurface reports whether an ADDED line of file is a surface a
// human:<name> stamp can be a claim on. When it is not, reason names why, for the
// NOT-A-CLAIM notice. It is the STAMP lane's scope only — see the file header.
func stampClaimSurface(file, content string) (ok bool, reason string) {
	if ok, conv := isTestSourceFile(file); ok {
		return false, fmt.Sprintf("test source (%s): no stamp reader parses it", conv)
	}
	if isYAMLFile(file) && strings.HasPrefix(strings.TrimLeft(content, " \t"), "#") {
		return false, "YAML # line: no stamp reader parses YAML"
	}
	return true, ""
}

// diffAddedLine is one ADDED line of a PR diff with the diff's own leading "+"
// stripped. Section increments at every file header, so a consumer that keeps
// per-file state (stampsInDiff's table header) resets it exactly where the walk
// crossed into a new file.
type diffAddedLine struct {
	File    string
	Section int
	Content string
}

// quotedDiffLine is an ADDED line the walker set aside as quoted notation, with
// the reason, so runCorroborate can announce what it did not read as a claim.
type quotedDiffLine struct {
	File    string
	Content string
	Reason  string
}

const reasonEmbeddedPatchRemoved = "removed (-) side of a diff embedded in a committed patch file"

// walkAddedDiffLines is the ONE diff walker every --corroborate lane reads
// through (stamps, citations, decision records). It tracks the current file from
// the "diff --git" / "+++ " headers, keeps only ADDED lines (a "+" that is not the
// "+++" header), skips a declared fixture corpus (isExcludedFixturePath) exactly as
// each lane did, and sets aside the removed side of an embedded patch's hunks as
// quoted.
//
// root is the checkout root a declared fixture-corpus marker is resolved against;
// "" means no checkout, in which case only the hardcoded education prefix
// excludes (the marker mechanism fails closed to "fully scanned").
func walkAddedDiffLines(root, diff string) (added []diffAddedLine, quoted []quotedDiffLine) {
	curFile := ""
	section := 0
	// inHunk tracks, inside an embedded patch file, whether the walk is in the body
	// of one of ITS hunks — after an "@@" line, while each line is a hunk line
	// (" ", "+", "-" or "\"). Only there is a "-" line the removed side; before the
	// first hunk (a `git format-patch` commit-message preamble) or after a hunk
	// ends, a "-" line is ordinary added text and stays scanned (fail-closed).
	inHunk := false
	for _, line := range strings.Split(diff, "\n") {
		trimmed := strings.TrimRight(line, "\r")
		if strings.HasPrefix(trimmed, "diff --git ") {
			// "diff --git a/path b/path"
			if fields := strings.Fields(trimmed); len(fields) >= 4 {
				curFile = strings.TrimPrefix(fields[3], "b/")
			}
			section++
			inHunk = false
			continue
		}
		if strings.HasPrefix(trimmed, "+++ ") {
			curFile = strings.TrimPrefix(trimmed, "+++ b/")
			section++
			inHunk = false
			continue
		}
		// Only added lines (not the "+++" header itself).
		if !strings.HasPrefix(trimmed, "+") || strings.HasPrefix(trimmed, "+++") {
			continue
		}
		if isExcludedFixturePath(root, curFile) {
			continue
		}
		content := strings.TrimPrefix(trimmed, "+")
		if isEmbeddedPatchFile(curFile) {
			switch {
			case strings.HasPrefix(content, "@@"):
				inHunk = true
			case inHunk && content != "" && strings.ContainsRune(" +-\\", rune(content[0])):
				if content[0] == '-' {
					quoted = append(quoted, quotedDiffLine{File: curFile, Content: content, Reason: reasonEmbeddedPatchRemoved})
					continue
				}
			default:
				inHunk = false
			}
		}
		added = append(added, diffAddedLine{File: curFile, Section: section, Content: content})
	}
	return added, quoted
}

// addedDiffLines is walkAddedDiffLines without the quoted set — what a lane reads.
func addedDiffLines(root, diff string) []diffAddedLine {
	added, _ := walkAddedDiffLines(root, diff)
	return added
}

// quotedClaimNotices lists, for the run log, every stamp or configured-human
// citation that a line --corroborate set aside as quoted notation WOULD have
// produced — the removed side of an embedded patch (both lanes) and a stamp on a
// non-claim surface (stampClaimSurface). It changes no verdict; it exists so a
// skip is reviewable rather than silent.
func quotedClaimNotices(root, diff string) []string {
	var out []string
	seen := map[string]bool{}
	add := func(s string) {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	stampNotices := func(file, content, reason string) {
		scan := stripOnBehalfOf(content)
		names := []string{}
		for _, m := range humanStampRe.FindAllStringSubmatch(scan, -1) {
			names = append(names, strings.ToLower(m[1]))
		}
		for _, n := range confusableStampNames(scan) {
			names = append(names, strings.ToLower(n))
		}
		for _, n := range names {
			add(fmt.Sprintf("human:%s in %s NOT-A-CLAIM — %s", n, file, reason))
		}
	}
	added, quoted := walkAddedDiffLines(root, diff)
	for _, q := range quoted {
		stampNotices(q.File, q.Content, q.Reason)
		for _, c := range detectCitations(q.File, q.Content) {
			add(fmt.Sprintf("citation of %s in %s NOT-A-CLAIM — %s", c.Name, q.File, q.Reason))
		}
	}
	for _, l := range added {
		if ok, reason := stampClaimSurface(l.File, l.Content); !ok {
			stampNotices(l.File, l.Content, reason)
		}
	}
	return out
}
