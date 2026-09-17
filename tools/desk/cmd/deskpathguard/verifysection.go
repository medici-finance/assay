package main

// verifysection.go — best-effort detection of whether a unified diff touches a brief's
// `## Verify` table specifically, from the diff TEXT alone.
//
// The forge seam's structured file list (ListChangedFiles → ChangedFile) carries no patch/
// hunk field — only the raw path and status — so this reads deskkit.Forge.ChangeDiff's raw
// unified-diff text instead, the same read `deskboard diff` uses for its human display. Two
// signals decide, either sufficient: (a) a hunk header's own trailing context
// (`@@ -a,b +c,d @@ <context>`, the line git's own function-context heuristic already
// attaches for many text file types) naming a heading, and (b) walking every context and
// changed line in the file's hunks and tracking which markdown ATX heading (`## ...`) they
// fall under. A CHANGED (+/-) line while the tracked heading is "Verify" (case-insensitive)
// marks the section touched; a heading LINE ITSELF being added or removed also updates the
// tracked heading before that line is tested, so opening or closing the section is itself a
// touch of whichever heading it opens.
//
// This is a heuristic, not a byte-exact section parse: a hunk whose limited context window
// includes neither the enclosing heading nor a git-supplied trailing-context heading cannot
// be attributed. That bound is accepted for this brief (effort: S) — the LOWER layer this
// brief documents as the safety net (README "single-point-of-failure") is verify-desk's
// merge-base re-derivation (rederive.go), which reads the FULL table content rather than a
// diff hunk and so does not share this approximation.
import "strings"

// verifySectionTouched reports whether diffText's hunks for path touch the `## Verify`
// section.
func verifySectionTouched(diffText, path string) bool {
	lines := strings.Split(diffText, "\n")
	inFile := false
	section := ""
	touched := false
	for _, line := range lines {
		switch {
		case strings.HasPrefix(line, "diff --git "):
			inFile = diffGitLineMatches(line, path)
			section = ""
			continue
		case strings.HasPrefix(line, "+++ "):
			if strings.TrimPrefix(strings.TrimPrefix(line, "+++ "), "b/") == path {
				inFile = true
			}
			continue
		case strings.HasPrefix(line, "--- "):
			continue
		}
		if !inFile {
			continue
		}
		if strings.HasPrefix(line, "@@") {
			if idx := strings.LastIndex(line, "@@"); idx > 1 {
				trailing := strings.TrimSpace(line[idx+2:])
				if h, ok := headingText(trailing); ok {
					section = h
				}
			}
			continue
		}
		if line == "" {
			continue
		}
		marker := line[0]
		if marker != '+' && marker != '-' && marker != ' ' {
			continue // e.g. "\ No newline at end of file"
		}
		content := line[1:]
		if h, ok := headingText(content); ok {
			section = h
		}
		if (marker == '+' || marker == '-') && strings.EqualFold(section, "Verify") {
			touched = true
		}
	}
	return touched
}

// diffGitLineMatches reports whether a `diff --git a/X b/Y` line names path as its "b" side.
func diffGitLineMatches(diffGitLine, path string) bool {
	return strings.HasSuffix(diffGitLine, " b/"+path) || strings.Contains(diffGitLine, " b/"+path+" ")
}

// headingText reports whether s (a line with its 1-char diff marker already stripped) is a
// markdown ATX heading, returning its trimmed text.
func headingText(s string) (string, bool) {
	t := strings.TrimSpace(s)
	if !strings.HasPrefix(t, "#") {
		return "", false
	}
	i := 0
	for i < len(t) && t[i] == '#' {
		i++
	}
	if i == 0 || i > 6 || i >= len(t) {
		return "", false
	}
	rest := strings.TrimSpace(t[i:])
	return rest, rest != ""
}
