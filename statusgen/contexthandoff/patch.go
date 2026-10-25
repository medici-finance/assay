package contexthandoff

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// patch.go is a strict single-file unified-diff reader and applier. It has no
// fuzz and no offset search: every hunk applies at exactly the line it names,
// with exactly the counts it declares, or the patch does not apply. A patch
// that applies "nearly" is the failure a base-revision binding exists to catch.

type hunk struct {
	oldStart, oldCount int
	newStart, newCount int
	lines              []string // each with its ' ', '-' or '+' prefix
	oldNoEOL, newNoEOL bool     // "\ No newline at end of file" after an old or new line
}

type filePatch struct {
	oldPath, newPath string // "" for /dev/null
	hunks            []hunk
}

var hunkHeaderRe = regexp.MustCompile(`^@@ -([0-9]+)(?:,([0-9]+))? \+([0-9]+)(?:,([0-9]+))? @@`)

func patchPath(header, prefix string) (string, error) {
	p := strings.TrimPrefix(header, prefix)
	if i := strings.IndexByte(p, '\t'); i >= 0 {
		p = p[:i]
	}
	switch {
	case p == "/dev/null":
		return "", nil
	case strings.HasPrefix(p, "a/") || strings.HasPrefix(p, "b/"):
		return p[2:], nil
	}
	return "", fmt.Errorf("path %q has no a/ or b/ prefix", p)
}

func atoiDefault(s string, def int) int {
	if s == "" {
		return def
	}
	n, _ := strconv.Atoi(s)
	return n
}

// parsePatch reads one file's unified diff. Git's "diff --git" and "index"
// header lines are accepted before the file header; anything else outside a
// hunk, and any second file, is refused.
func parsePatch(raw []byte) (*filePatch, error) {
	if len(raw) == 0 {
		return nil, fmt.Errorf("empty patch")
	}
	if raw[len(raw)-1] != '\n' {
		return nil, fmt.Errorf("patch does not end with a newline")
	}
	lines := strings.Split(string(raw[:len(raw)-1]), "\n")
	i := 0
	for i < len(lines) && !strings.HasPrefix(lines[i], "--- ") {
		l := lines[i]
		if !(strings.HasPrefix(l, "diff --git ") || strings.HasPrefix(l, "index ") ||
			strings.HasPrefix(l, "new file mode ") || strings.HasPrefix(l, "deleted file mode ")) {
			return nil, fmt.Errorf("line %d: unexpected %q before the file header", i+1, l)
		}
		i++
	}
	if i+1 >= len(lines) || !strings.HasPrefix(lines[i+1], "+++ ") {
		return nil, fmt.Errorf("no ---/+++ file header")
	}
	p := &filePatch{}
	var err error
	if p.oldPath, err = patchPath(lines[i], "--- "); err != nil {
		return nil, err
	}
	if p.newPath, err = patchPath(lines[i+1], "+++ "); err != nil {
		return nil, err
	}
	i += 2
	for i < len(lines) {
		m := hunkHeaderRe.FindStringSubmatch(lines[i])
		if m == nil {
			if strings.HasPrefix(lines[i], "--- ") || strings.HasPrefix(lines[i], "diff --git ") {
				return nil, fmt.Errorf("line %d: the patch touches more than one file", i+1)
			}
			return nil, fmt.Errorf("line %d: expected a hunk header, got %q", i+1, lines[i])
		}
		h := hunk{oldStart: atoiDefault(m[1], 0), oldCount: atoiDefault(m[2], 1), newStart: atoiDefault(m[3], 0), newCount: atoiDefault(m[4], 1)}
		i++
		var olds, news int
		for i < len(lines) && (olds < h.oldCount || news < h.newCount || strings.HasPrefix(lines[i], `\`)) {
			l := lines[i]
			switch {
			case strings.HasPrefix(l, `\`):
				if len(h.lines) == 0 {
					return nil, fmt.Errorf("line %d: no-newline marker before any hunk line", i+1)
				}
				switch h.lines[len(h.lines)-1][0] {
				case '-':
					h.oldNoEOL = true
				case '+':
					h.newNoEOL = true
				default:
					h.oldNoEOL, h.newNoEOL = true, true
				}
			case strings.HasPrefix(l, " "):
				olds++
				news++
			case strings.HasPrefix(l, "-"):
				olds++
			case strings.HasPrefix(l, "+"):
				news++
			default:
				return nil, fmt.Errorf("line %d: hunk line %q has no ' ', '-' or '+' prefix", i+1, l)
			}
			if !strings.HasPrefix(l, `\`) {
				h.lines = append(h.lines, l)
			}
			i++
		}
		if olds != h.oldCount || news != h.newCount {
			return nil, fmt.Errorf("hunk @@ -%d,%d +%d,%d @@ has %d old and %d new lines", h.oldStart, h.oldCount, h.newStart, h.newCount, olds, news)
		}
		p.hunks = append(p.hunks, h)
	}
	if len(p.hunks) == 0 {
		return nil, fmt.Errorf("patch has no hunks")
	}
	return p, nil
}

// applied is the result of a strict apply: the new content, the old line
// indices removed, and the old positions insertions went in front of (an
// insertion at len(old) appends).
type applied struct {
	content  []byte
	deleted  []int
	inserted []int
}

// apply applies p to old exactly. Hunks must be in order, must not overlap,
// and must match the old lines and end-of-file newline exactly.
func (p *filePatch) apply(old []byte) (*applied, error) {
	if p.oldPath == "" || p.newPath == "" {
		return nil, fmt.Errorf("a patch that creates or deletes the brief is not an amendment")
	}
	if p.oldPath != p.newPath {
		return nil, fmt.Errorf("a patch that renames %s to %s is not an amendment", p.oldPath, p.newPath)
	}
	oldLines := splitLines(old)
	oldEOL := len(old) > 0 && old[len(old)-1] == '\n'
	out := &applied{}
	var next []string
	cursor := 0
	newEOL := oldEOL
	for n, h := range p.hunks {
		pos := h.oldStart - 1
		if h.oldCount == 0 {
			pos = h.oldStart
		}
		if pos < cursor || pos > len(oldLines) {
			return nil, fmt.Errorf("hunk %d starts at old line %d, outside the remaining file", n+1, h.oldStart)
		}
		next = append(next, oldLines[cursor:pos]...)
		at := pos
		for _, l := range h.lines {
			body := l[1:]
			switch l[0] {
			case ' ', '-':
				if at >= len(oldLines) || oldLines[at] != body {
					return nil, fmt.Errorf("hunk %d: old line %d does not match the patch", n+1, at+1)
				}
				if l[0] == ' ' {
					next = append(next, body)
				} else {
					out.deleted = append(out.deleted, at)
				}
				at++
			case '+':
				next = append(next, body)
				out.inserted = append(out.inserted, at)
			}
		}
		touchesEnd := at == len(oldLines)
		if h.oldNoEOL && (!touchesEnd || oldEOL) {
			return nil, fmt.Errorf("hunk %d: the patch says the old file has no final newline, but it does", n+1)
		}
		if touchesEnd && h.oldCount > 0 && !h.oldNoEOL && !oldEOL {
			return nil, fmt.Errorf("hunk %d: the old file has no final newline, but the patch does not say so", n+1)
		}
		if touchesEnd {
			newEOL = !h.newNoEOL
		}
		cursor = at
	}
	next = append(next, oldLines[cursor:]...)
	content := strings.Join(next, "\n")
	if newEOL && len(next) > 0 {
		content += "\n"
	}
	out.content = []byte(content)
	return out, nil
}
