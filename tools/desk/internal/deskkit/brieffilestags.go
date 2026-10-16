package deskkit

import (
	"regexp"
	"strings"
)

// BriefFilesTags reads the actual files list, excluding frontmatter and fenced examples.
// Dispatcher routing and verifier association use the same declaration grammar.
var briefFilesTagRe = regexp.MustCompile("^`?\\[([a-z][a-z0-9_-]{0,31})\\]`?(?:\\s|$)")

func BriefFilesTags(text string) []string {
	lines := strings.Split(text, "\n")
	start := 0
	if len(lines) > 0 && strings.TrimSpace(lines[0]) == "---" {
		for i := 1; i < len(lines); i++ {
			if strings.TrimSpace(lines[i]) == "---" {
				start = i + 1
				break
			}
		}
	}
	var tags []string
	take := func(entry string) {
		if m := briefFilesTagRe.FindStringSubmatch(strings.TrimSpace(entry)); m != nil {
			tags = append(tags, m[1])
		}
	}
	fence := "" // the open fence's run of backticks or tildes, "" outside a fenced block
	for i := start; i < len(lines); i++ {
		t := strings.TrimSpace(lines[i])
		if f := briefFenceRun(t); f != "" {
			switch {
			case fence == "":
				fence = f
			case f[0] == fence[0] && len(f) >= len(fence) && strings.TrimSpace(t[len(f):]) == "":
				fence = ""
			}
			continue
		}
		if fence != "" {
			continue // inside a fenced example: its files: line is not the brief's list
		}
		if !strings.HasPrefix(t, "files:") {
			continue
		}
		if rest := strings.TrimSpace(strings.TrimPrefix(t, "files:")); rest != "" {
			take(rest) // the inline form: one entry on the files: line itself
			return tags
		}
		for j := i + 1; j < len(lines); j++ {
			lt := strings.TrimSpace(lines[j])
			if lt == "" {
				break
			}
			if strings.HasPrefix(lt, "- ") {
				take(strings.TrimPrefix(lt, "- "))
				continue
			}
			if !strings.HasPrefix(lines[j], " ") && !strings.HasPrefix(lines[j], "\t") {
				break
			}
		}
		return tags
	}
	return nil
}

// fenceRun returns the leading run of a (trimmed) line that opens or closes a fenced code block —
// three or more backticks, or three or more tildes — or "" when the line is not a fence line.
func briefFenceRun(t string) string {
	if len(t) < 3 || (t[0] != '`' && t[0] != '~') {
		return ""
	}
	n := 1
	for n < len(t) && t[n] == t[0] {
		n++
	}
	if n < 3 {
		return ""
	}
	return t[:n]
}
