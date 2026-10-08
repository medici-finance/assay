// act-block lint: every example Act block the plugin ships must stay safe to
// paste into an interactive zsh, and must not act on its dry paste.
//
// An Act block (ask-decision §"Act — the shape of the fifth part") is a fenced
// shell block a person pastes into a shell; pasting it must only print what it
// would do. zsh, the default macOS login shell, reads `#` as a comment at an
// interactive prompt only when `interactive_comments` is set, and it is unset
// by default. There a comment line is part of a COMMAND: a `;` ends it, and a
// backtick span or `$(…)` after it runs — which once re-ran the previous act's
// function live on the next block's first paste. The same holds for the text
// after a `#` on a code line.
//
// The rule that closes that on a block's FIRST paste is the plain-text comment
// rule: every comment line uses only plain characters (actCommentAllowed), and
// no comment trails code, so a comment runs nothing even where zsh reads it as
// a command. The zsh comment guard (actBlockGuardLine) does NOT close it alone:
// a terminal hands zsh a paste as one bracketed paste, which zsh reads whole
// before running its first line, so the guard covers only later pastes and
// lines typed after it. This lint holds, on every act block under plugins/:
//
//  1. the block's first non-blank line is the zsh comment guard;
//  2. the act function has a per-act name (driver_act_<id>), never the bare
//     driver_act, so a block that fails to parse leaves no earlier act's
//     function under the name the driver is told to type;
//  3. every full-line comment holds only allowed characters, and no code line
//     carries a trailing comment: a # that starts a word outside quotes after
//     code, whether a blank or an operator such as ; or | ends the word before
//     it (actTrailingCommentLines);
//  4. every read in the block is the whole shape NAME=; read -rs NAME || exit N
//     or NAME=; read -rs NAME || { …; exit N; } on one line (N from 1 to 255;
//     -r and -s both given, no other option; the clear in command position),
//     run by the act function's own shell: not in a subshell, command
//     substitution, pipeline or background, nor in another function, on this
//     line or any other. A shell whose read has no -s fails without assigning,
//     so an inherited value would pass as the secret, and an exit that runs in a
//     child shell ends only that child. The block is tokenized whole (quotes,
//     backslashes, $( ), backticks, ( ), { } and compound commands across
//     lines), and any word that is read once quotes and backslashes are removed
//     counts as a read wherever it sits, so the check errs strict. It does not
//     see a read run through eval, sh -c or a command name built from an
//     expansion.
//
// An act block is recognised as an sh, bash, zsh or shell fence whose body
// defines an act function (actFuncRe). HARD check: a violation is exit 1.
// Zero act blocks across the tree is could-not-check (exit 2) — the
// ask-decision skill's example must exist, so a matcher that silently stopped
// matching fails instead of reporting clean.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// actBlockGuardLine is the line every act block opens with.
const actBlockGuardLine = `[ -n "${ZSH_VERSION-}" ] && setopt interactive_comments`

// actBlockMarker is the prefix of the act function's name. A fence that
// defines a function whose name starts with it is an act block.
const actBlockMarker = "driver_act"

// actFuncRe matches the line that defines the act function; group 1 is its name.
var actFuncRe = regexp.MustCompile(`^\s*(` + actBlockMarker + `[A-Za-z0-9_]*)\s*\(\)`)

// actPerActNameRe is the per-act name shape: the marker, an underscore and an id.
var actPerActNameRe = regexp.MustCompile(`^` + actBlockMarker + `_[A-Za-z0-9_]+$`)

// actShellLangs are the fence info strings whose blocks the lint reads.
var actShellLangs = map[string]bool{"sh": true, "bash": true, "zsh": true, "shell": true}

// actTrailingCommentLines returns the indexes of the block lines on which a
// trailing comment starts: a `#` that starts a word, outside quotes and not
// after a backslash, on a line that already holds code. A word starts after a
// blank, a newline, a list or pipe operator (; & |), a redirect (< >), a
// parenthesis, a backtick or a $( — so `echo dry;#;echo live` counts, which a
// first zsh paste runs as `echo dry`, `#` and `echo live`. The block is read
// whole, so a quote, $( or backtick span or line continuation opened on one
// line carries to the next; a `#` that starts a word on a line holding nothing
// before it is a full-line comment, which the comment rule reads instead. A
// heredoc body is read as code, which errs strict.
func actTrailingCommentLines(lines []string) map[int]bool {
	src := strings.Join(lines, "\n")
	type frame struct {
		kind  byte // 'n' code, 'd' a double quote, '$' a $( span, '`' a backtick span
		depth int  // ( opened inside a $( span and not yet closed
	}
	var (
		stack     = []frame{{kind: 'n'}}
		out       = map[int]bool{}
		line      int
		wordStart = true // the next character would start a word
		lineStart = true // nothing but blanks since the last newline
	)
	for i := 0; i < len(src); i++ {
		c := src[i]
		top := &stack[len(stack)-1]
		if top.kind == 'd' {
			switch {
			case c == '"':
				stack = stack[:len(stack)-1]
			case c == '\\' && i+1 < len(src):
				i++
				if src[i] == '\n' {
					line++
				}
			case c == '$' && i+1 < len(src) && src[i+1] == '(':
				stack = append(stack, frame{kind: '$'})
				i++
				wordStart = true
			case c == '`':
				stack = append(stack, frame{kind: '`'})
				wordStart = true
			case c == '\n':
				line++
			}
			continue
		}
		switch {
		case c == '\n':
			line++
			wordStart, lineStart = true, true
			continue
		case c == ' ' || c == '\t' || c == '\r':
			wordStart = true
			continue
		case c == '\\':
			// A backslash before a newline joins the lines, leaving the word
			// state as it was; before anything else it quotes that character.
			if i+1 < len(src) {
				i++
				if src[i] == '\n' {
					line++
					continue
				}
			}
			wordStart = false
		case c == '\'':
			j := strings.IndexByte(src[i+1:], '\'')
			if j < 0 {
				j = len(src) - i - 1
			}
			line += strings.Count(src[i+1:i+1+j], "\n")
			i += j + 1
			wordStart = false
		case c == '"':
			stack = append(stack, frame{kind: 'd'})
			wordStart = false
		case c == '#' && wordStart:
			if !lineStart {
				out[line] = true
			}
			for i+1 < len(src) && src[i+1] != '\n' {
				i++
			}
			continue
		case c == '$' && i+1 < len(src) && src[i+1] == '(':
			stack = append(stack, frame{kind: '$'})
			i++
			wordStart = true
		case c == '`':
			if top.kind == '`' {
				stack = stack[:len(stack)-1]
				wordStart = false
			} else {
				stack = append(stack, frame{kind: '`'})
				wordStart = true
			}
		case c == '(':
			if top.kind == '$' {
				top.depth++
			}
			wordStart = true
		case c == ')':
			if top.kind == '$' && top.depth == 0 {
				// The $( span closes mid-word: `$(cmd)#x` is one word.
				stack = stack[:len(stack)-1]
				wordStart = false
			} else {
				if top.kind == '$' {
					top.depth--
				}
				wordStart = true
			}
		case strings.IndexByte(";&|<>", c) >= 0:
			wordStart = true
		default:
			wordStart = false
		}
		lineStart = false
	}
	return out
}

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

// actCommentRuleText names the allowed set in every comment-rule issue.
const actCommentRuleText = "a comment line holds only letters, digits, spaces, tabs and . , : - / _ + = #"

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

// actBlockLines scans one file for shell fences and checks each one that is an
// act block. A fence opens on a run of three or more backticks or tildes and
// closes on a bare run of the same character at least as long.
func actBlockLines(rel, raw string) (int, []Issue) {
	type fenceLine struct {
		n    int
		text string
	}
	var (
		blocks int
		issues []Issue
		open   bool
		isSh   bool
		char   byte
		width  int
		body   []fenceLine
		lines  = strings.Split(raw, "\n")
	)
	add := func(n int, format string, a ...any) {
		issues = append(issues, Issue{Path: rel, Msg: fmt.Sprintf("line %d: ", n) + fmt.Sprintf(format, a...)})
	}
	checkSh := func() {
		if !isSh {
			return
		}
		name, nameAt := "", 0
		for _, l := range body {
			if m := actFuncRe.FindStringSubmatch(l.text); m != nil {
				name, nameAt = m[1], l.n
				break
			}
		}
		if name == "" {
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
			add(body[max(first, 0)].n, "an act block must open with the zsh comment guard %q, ahead of any # line (it covers later pastes; the plain-text comment rule covers the first)", actBlockGuardLine)
		}
		if !actPerActNameRe.MatchString(name) {
			add(nameAt, "the act function is %q; name it per act, %s_<id> with the issue number as id, so a block that fails to parse leaves no earlier act's function under the name the driver types", name, actBlockMarker)
		}
		code := make([]string, len(body))
		for i, l := range body {
			if !strings.HasPrefix(strings.TrimSpace(l.text), "#") {
				code[i] = l.text
			}
		}
		badReads := actUnsafeReadLines(code, name)
		texts := make([]string, len(body))
		for i, l := range body {
			texts[i] = l.text
		}
		trailing := actTrailingCommentLines(texts)
		for i, l := range body {
			t := strings.TrimSpace(l.text)
			if trailing[i] {
				add(l.n, "act block code line carries a trailing # comment — put the comment on its own line (%s); a first zsh paste passes trailing text to the command, and a # after ; or | runs what follows it", actCommentRuleText)
			}
			if strings.HasPrefix(t, "#") {
				for _, r := range t {
					if !actCommentAllowed(r) {
						add(l.n, "act block comment carries %q — %s, so it runs nothing in a shell that reads it as a command", r, actCommentRuleText)
						break
					}
				}
				continue
			}
			if badReads[i] {
				add(l.n, "act block read is not in the shape NAME=; read -rs NAME || exit 1, or NAME=; read -rs NAME || { ...; exit 1; }, run by the act function's own shell — clear the variable first, read with -r and -s only, end the failure branch with an exit from 1 to 255, and keep it out of any subshell, command substitution, pipeline or background, or a shell whose read has no -s keeps an inherited value as the secret")
			}
		}
	}
	for i, ln := range lines {
		t := strings.TrimSpace(ln)
		c, n := mdFenceRun(t)
		if !open {
			if n == 0 {
				continue
			}
			info := strings.Fields(t[n:])
			open, char, width, body = true, c, n, nil
			isSh = len(info) > 0 && actShellLangs[strings.ToLower(info[0])]
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
