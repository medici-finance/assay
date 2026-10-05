// act-block lint: every example Act block the plugin ships must stay safe to
// paste into an interactive zsh.
//
// An Act block (ask-decision §"Act — the shape of the fifth part") is a fenced
// `sh` block a person pastes into a shell; pasting it must only print what it
// would do. zsh, the default macOS login shell, reads `#` as a comment at an
// interactive prompt only when `interactive_comments` is set, and it is unset
// by default. Without it a comment line is a COMMAND: a `;` ends it, and a
// backtick span or `$(…)` after it runs — which once re-ran the previous act's
// function live on the next block's first paste. Two independent rules close
// that, and this lint holds both on every act block under plugins/:
//
//  1. the block's first non-blank line is the zsh comment guard
//     (actBlockGuardLine), so comments are comments in zsh too;
//  2. every comment line uses only plain characters (actCommentAllowed), so a
//     comment runs nothing even in a shell that reads it as a command.
//
// An act block is recognised as an `sh` fence whose body defines the act
// function (actBlockMarker). HARD check: a violation is exit 1. Zero act
// blocks across the tree is could-not-check (exit 2) — the ask-decision
// skill's example must exist, so a matcher that silently stopped matching
// fails instead of reporting clean.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// actBlockGuardLine is the line every act block opens with.
const actBlockGuardLine = `[ -n "${ZSH_VERSION-}" ] && setopt interactive_comments`

// actBlockMarker identifies an `sh` fence as an act block: it defines the act
// function the ask-decision skill names.
const actBlockMarker = "driver_act()"

// actCommentAllowed reports whether r may appear in an act block's comment
// line: letters, digits, space, tab and a short list of punctuation no shell
// gives a meaning to in a command's arguments.
func actCommentAllowed(r rune) bool {
	switch {
	case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		return true
	}
	return strings.ContainsRune(" \t.,:-/_+=#", r)
}

// ActBlockIssues walks every *.md under plugins/ and returns the number of act
// blocks it found plus one issue per broken rule. err is set when the tree
// cannot be read or holds no act block at all (could-not-check).
func ActBlockIssues(root string) (blocks int, issues []Issue, err error) {
	base := filepath.Join(root, filepath.FromSlash(pluginTreeDir))
	info, serr := os.Stat(base)
	if serr != nil || !info.IsDir() {
		return 0, nil, fmt.Errorf("no %s/ directory under %s — nothing to lint, which is never a pass", pluginTreeDir, root)
	}
	var files []string
	walkErr := filepath.WalkDir(base, func(path string, d os.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if !d.IsDir() && strings.EqualFold(filepath.Ext(path), ".md") {
			files = append(files, path)
		}
		return nil
	})
	if walkErr != nil {
		return 0, nil, fmt.Errorf("walk %s: %w", base, walkErr)
	}
	sort.Strings(files)
	for _, abs := range files {
		rel, rerr := filepath.Rel(root, abs)
		if rerr != nil {
			rel = abs
		}
		rel = filepath.ToSlash(rel)
		raw, readErr := os.ReadFile(abs)
		if readErr != nil {
			return blocks, issues, fmt.Errorf("cannot read %s: %v", rel, readErr)
		}
		n, is := actBlockLines(rel, string(raw))
		blocks += n
		issues = append(issues, is...)
	}
	if blocks == 0 {
		return 0, nil, fmt.Errorf("no act block (an sh fence defining %s) under %s — the ask-decision example should be one; a check that found nothing proved nothing", actBlockMarker, base)
	}
	return blocks, issues, nil
}

// actBlockLines scans one file for `sh` fences and checks each one that is an
// act block. A fence opens on a run of three or more backticks or tildes and
// closes on a bare run of the same character at least as long.
func actBlockLines(rel, raw string) (int, []Issue) {
	type fenceLine struct {
		n    int
		text string
	}
	var (
		blocks  int
		issues  []Issue
		open    bool
		isSh    bool
		char    byte
		width   int
		body    []fenceLine
		lines   = strings.Split(raw, "\n")
		checkSh = func() {
			if !isSh {
				return
			}
			isAct := false
			for _, l := range body {
				if strings.Contains(l.text, actBlockMarker) {
					isAct = true
					break
				}
			}
			if !isAct {
				return
			}
			blocks++
			first := -1
			for i, l := range body {
				if strings.TrimSpace(l.text) != "" {
					first = i
					break
				}
			}
			if first < 0 || strings.TrimSpace(body[first].text) != actBlockGuardLine {
				at := 0
				if first >= 0 {
					at = body[first].n
				} else if len(body) > 0 {
					at = body[0].n
				}
				issues = append(issues, Issue{Path: rel, Msg: fmt.Sprintf("line %d: an act block must open with the zsh comment guard %q, ahead of any # line (interactive zsh runs comment lines as commands without it)", at, actBlockGuardLine)})
			}
			for _, l := range body {
				t := strings.TrimSpace(l.text)
				if !strings.HasPrefix(t, "#") {
					continue
				}
				for _, r := range t {
					if !actCommentAllowed(r) {
						issues = append(issues, Issue{Path: rel, Msg: fmt.Sprintf("line %d: act block comment carries %q — a comment line holds only letters, digits, spaces and . , : - / _ + = #, so it runs nothing in a shell that reads it as a command", l.n, r)})
						break
					}
				}
			}
		}
	)
	for i, ln := range lines {
		t := strings.TrimSpace(ln)
		c, n := mdFenceRun(t)
		if !open {
			if n == 0 {
				continue
			}
			info := strings.Fields(t[n:])
			open, char, width, body = true, c, n, nil
			isSh = len(info) > 0 && strings.EqualFold(info[0], "sh")
			continue
		}
		if n >= width && c == char && strings.TrimSpace(t[n:]) == "" {
			checkSh()
			open = false
			continue
		}
		body = append(body, fenceLine{n: i + 1, text: ln})
	}
	return blocks, issues
}

// mdFenceRun returns the fence character and run length opening t, or (0, 0)
// when t does not open with three or more backticks or tildes.
func mdFenceRun(t string) (byte, int) {
	if t == "" || (t[0] != '`' && t[0] != '~') {
		return 0, 0
	}
	n := 0
	for n < len(t) && t[n] == t[0] {
		n++
	}
	if n < 3 {
		return 0, 0
	}
	return t[0], n
}
