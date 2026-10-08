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
//     carries a trailing comment. Two checks, unioned: the per-line floor
//     flags any # after a blank or tab on a code line, quoted or not
//     (actTrailingCommentRe); a scanner that reads the block whole, with its
//     quotes, spans and heredocs, flags a # that starts a word in code after
//     code on its line, whether a blank or an operator such as ; or | ends the
//     word before it. The scanner also fails the block where it cannot settle
//     a reading: an unclosed quote, span, heredoc or line continuation at the
//     end, and the few constructs it does not model (actScanBlock);
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

// actTrailingCommentRe is the per-line floor: a `#` after a space or tab on a
// code line. It runs next to actScanBlock and flags such a line even inside
// quotes or a heredoc body, so the scanner can only add to what it flags.
var actTrailingCommentRe = regexp.MustCompile(`[ \t]#`)

// actScan is what actScanBlock finds in one act block, keyed by line index.
type actScan struct {
	trailing map[int]bool     // a trailing comment starts on the line
	problems map[int][]string // the block fails strict here: see actScanBlock
}

// actScanBlock reads an act block whole, the way a shell splits it into words,
// and returns the lines on which a trailing comment starts plus the lines the
// lint fails strict on.
//
// A trailing comment is a `#` that starts a word in code, not after a
// backslash, on a line that already holds code. A word starts after a blank, a
// newline, a list or pipe operator (; & |), a redirect (< >), a parenthesis, a
// backtick or a $( — so `echo dry;#;echo live` counts, which a first zsh paste
// runs as `echo dry`, `#` and `echo live`. A `#` that starts a word on a line
// holding nothing before it is a full-line comment, which the comment rule
// reads instead. Comment text is skipped, so a quote in it opens nothing; the
// comment rule and this rule already fail any comment that holds a quote.
//
// Modelled: single quotes; double quotes, with backslash escapes and the $( ),
// ${ } and backtick spans inside them; $'…' quotes, with backslash escapes;
// $"…" (read as a double quote); $( ) spans, nested ( ) inside them and $(( ));
// ${ } spans, where # is never a comment; backtick spans, with \` inside them;
// a backslash outside quotes, before a newline a line continuation; heredocs
// (<<WORD and <<-WORD, WORD bare or quoted in whole or in part, several on one
// line, inside $( ) too), whose body runs from the next line to the line that
// reads WORD (after leading tabs for <<-): with a quoted WORD the body is data,
// with a bare one it is read like a double quote without the quote, so $( ),
// ${ } and backtick spans in it are still code; here-strings (<<<), which are
// not heredocs.
//
// Fails strict (a problem on the line), where a reading cannot be settled:
//   - the block, or an unquoted heredoc body, ends inside a single, double or
//     $'…' quote, a $( ), ${ } or backtick span, a heredoc with no closing
//     line, or a line continuation;
//   - a \' inside $'…': bash and zsh read it as a quote character, but a shell
//     without $'…' quoting (dash) ends the quote there;
//   - a single quote inside ${ }: shells disagree on whether it quotes;
//   - the word case inside $( ): an unbalanced pattern ) would close the span
//     here, and older bash parses it differently from newer shells;
//   - a << with no delimiter word.
//
// Not modelled, and strict as a result: a << in arithmetic, $((1<<2)), is read
// as a heredoc, which then has no closing line; a # after a blank inside $(( ))
// counts as a trailing comment. eval, sh -c and aliases are not followed.
func actScanBlock(lines []string) actScan {
	src := strings.Join(lines, "\n")
	starts := []int{0}
	for i := 0; i < len(src); i++ {
		if src[i] == '\n' {
			starts = append(starts, i+1)
		}
	}
	s := &actScanner{src: src, starts: starts, out: actScan{trailing: map[int]bool{}, problems: map[int][]string{}}}
	s.scan(0, len(src), 'n', "")
	return s.out
}

// actScanner holds one block's text and what actScanBlock has found in it.
type actScanner struct {
	src    string
	starts []int // offset of each line's first byte
	out    actScan
}

// actFrameNames names each frame kind in an "ends inside" problem.
var actFrameNames = map[byte]string{
	'd': "a double quote", '$': "a $( span", '{': "a ${ span", '`': "a backtick span",
}

// lineOf returns the index of the line holding byte pos.
func (s *actScanner) lineOf(pos int) int {
	return sort.Search(len(s.starts), func(k int) bool { return s.starts[k] > pos }) - 1
}

// problem records msg once at the line holding byte pos.
func (s *actScanner) problem(pos int, msg string) {
	l := s.lineOf(pos)
	for _, m := range s.out.problems[l] {
		if m == msg {
			return
		}
	}
	s.out.problems[l] = append(s.out.problems[l], msg)
}

// opened records that the text ends inside what opened at byte pos, with an
// optional detail. where is "act block" or "act block heredoc body".
func (s *actScanner) opened(pos int, where, what, detail string) {
	if detail != "" {
		detail = " (" + detail + ")"
	}
	s.problem(pos, fmt.Sprintf("%s ends inside %s opened on this line%s — a shell reads the rest, and whatever is pasted after it, as part of it, so the lint cannot tell what runs and fails rather than pass", where, what, detail))
}

// unmodelled records a construct at byte pos that the lint fails strict on.
func (s *actScanner) unmodelled(pos int, what, why string) {
	s.problem(pos, fmt.Sprintf("act block uses %s, which the lint does not model: %s — spell it another way", what, why))
}

// actHeredoc is a heredoc whose body starts after the current line.
type actHeredoc struct {
	word   string // the delimiter, quotes and backslashes removed
	strip  bool   // <<-: leading tabs are stripped before the match
	quoted bool   // any part of the delimiter was quoted: the body is data
	at     int    // byte offset of the <<
}

// scan reads src[lo:hi]. base is 'n' for the block (code) or 'h' for an
// unquoted heredoc body (text, like a double quote that no " closes).
func (s *actScanner) scan(lo, hi int, base byte, where string) {
	src := s.src
	if where == "" {
		where = "act block"
	}
	type frame struct {
		kind  byte // 'n' code, 'h' heredoc body, 'd' double quote, '$' $( span, '{' ${ span, '`' backtick span
		depth int  // ( opened inside a $( span and not yet closed
		at    int  // byte offset where the frame opened
	}
	var (
		stack     = []frame{{kind: base, at: lo}}
		pending   []actHeredoc
		wordStart = true // the next character would start a word
		lineStart = true // nothing but blanks since the last newline
	)
	push := func(kind byte, at int) {
		stack = append(stack, frame{kind: kind, at: at})
		wordStart, lineStart = true, false
	}
	for i := lo; i < hi; i++ {
		c := src[i]
		top := &stack[len(stack)-1]
		code := top.kind == 'n' || top.kind == '$' || top.kind == '`'
		// A backslash quotes the next character in every frame read here; before
		// a newline it joins the lines, and as the last character it leaves a
		// line continuation open.
		if c == '\\' {
			if i+1 >= hi {
				s.opened(i, where, "a line continuation", "")
				return
			}
			i++
			if src[i] != '\n' {
				wordStart, lineStart = false, false
			}
			continue
		}
		if !code {
			switch {
			case top.kind == 'd' && c == '"':
				stack = stack[:len(stack)-1]
				wordStart = false
			case top.kind == '{' && c == '}':
				stack = stack[:len(stack)-1]
				wordStart = false
			case top.kind == '{' && c == '"':
				push('d', i)
				wordStart = false
			case top.kind == '{' && c == '\'':
				s.unmodelled(i, "a single quote inside ${ }", "shells disagree on whether it quotes there")
			case c == '$' && i+1 < hi && src[i+1] == '(':
				push('$', i)
				i++
			case c == '$' && i+1 < hi && src[i+1] == '{':
				push('{', i)
				i++
			case c == '`':
				push('`', i)
			}
			continue
		}
		if c == '\n' && len(pending) > 0 {
			// The heredoc bodies start on the next line, one after another.
			for _, h := range pending {
				ls, ok := s.heredocEnd(i+1, hi, h)
				if !ok {
					s.opened(h.at, where, "the heredoc <<"+h.word, "no line after it reads "+h.word)
					return
				}
				if !h.quoted && ls-1 > i+1 {
					s.scan(i+1, ls-1, 'h', "act block heredoc body")
				}
				for i = ls; i < hi && src[i] != '\n'; i++ {
				}
			}
			pending = nil
			wordStart, lineStart = true, true
			continue
		}
		switch {
		case c == '\n':
			wordStart, lineStart = true, true
			continue
		case c == ' ' || c == '\t' || c == '\r':
			wordStart = true
			continue
		case c == '\'':
			j := strings.IndexByte(src[i+1:hi], '\'')
			if j < 0 {
				s.opened(i, where, "a single quote", "")
				return
			}
			i += j + 1
			wordStart = false
		case c == '$' && i+1 < hi && src[i+1] == '\'':
			j := i + 2
			for ; j < hi && src[j] != '\''; j++ {
				if src[j] == '\\' && j+1 < hi {
					if src[j+1] == '\'' {
						s.unmodelled(j, `\' inside $'…'`, `bash and zsh read it as a quote character, but a shell without $'…' quoting, such as dash, ends the quote there`)
					}
					j++
				}
			}
			if j >= hi {
				s.opened(i, where, "a $'…' quote", "")
				return
			}
			i = j
			wordStart = false
		case c == '"':
			push('d', i)
			wordStart = false
		case c == '#' && wordStart:
			if !lineStart {
				s.out.trailing[s.lineOf(i)] = true
			}
			for i+1 < hi && src[i+1] != '\n' {
				i++
			}
			continue
		case c == '<' && i+1 < hi && src[i+1] == '<':
			if i+2 < hi && src[i+2] == '<' {
				i += 2 // a here-string, not a heredoc
				wordStart = true
				break
			}
			h, next := s.heredocWord(i, hi)
			if h.word == "" && !h.quoted {
				s.unmodelled(i, "a << with no delimiter word", "the lint cannot tell where its body ends")
				i++
				wordStart = true
				break
			}
			pending = append(pending, h)
			i = next - 1
			wordStart = false
		case c == '$' && i+1 < hi && src[i+1] == '(':
			push('$', i)
			i++
		case c == '$' && i+1 < hi && src[i+1] == '{':
			push('{', i)
			i++
			wordStart = false
		case c == '`':
			if top.kind == '`' {
				stack = stack[:len(stack)-1]
				wordStart = false
			} else {
				push('`', i)
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
		case wordStart && top.kind == '$' && strings.HasPrefix(src[i:hi], "case") &&
			(i+4 == hi || strings.IndexByte(" \t\n;", src[i+4]) >= 0):
			s.unmodelled(i, "case inside $( )", "a pattern's ) would close the span here, and older bash parses it differently")
			i += 3
			wordStart = false
		default:
			wordStart = false
		}
		lineStart = false
	}
	for _, h := range pending {
		s.opened(h.at, where, "the heredoc <<"+h.word, "no line after it reads "+h.word)
	}
	for k := len(stack) - 1; k > 0; k-- {
		s.opened(stack[k].at, where, actFrameNames[stack[k].kind], "")
	}
}

// heredocWord reads the heredoc operator at byte at (<< or <<-) and the
// delimiter word after it, and returns the heredoc and the offset just past
// the word.
func (s *actScanner) heredocWord(at, hi int) (actHeredoc, int) {
	src := s.src
	h := actHeredoc{at: at}
	j := at + 2
	if j < hi && src[j] == '-' {
		h.strip = true
		j++
	}
	for j < hi && (src[j] == ' ' || src[j] == '\t') {
		j++
	}
	var w strings.Builder
	for j < hi && strings.IndexByte(" \t\n;&|<>()", src[j]) < 0 {
		switch src[j] {
		case '\'':
			k := strings.IndexByte(src[j+1:hi], '\'')
			if k < 0 {
				k = hi - j - 1
			}
			w.WriteString(src[j+1 : j+1+k])
			h.quoted = true
			j += k + 2
		case '"':
			h.quoted = true
			for j++; j < hi && src[j] != '"'; j++ {
				if src[j] == '\\' && j+1 < hi && strings.IndexByte("\"\\$`", src[j+1]) >= 0 {
					j++
				}
				w.WriteByte(src[j])
			}
			j++
		case '\\':
			h.quoted = true
			if j+1 < hi {
				w.WriteByte(src[j+1])
			}
			j += 2
		default:
			w.WriteByte(src[j])
			j++
		}
	}
	h.word = w.String()
	return h, min(j, hi)
}

// heredocEnd finds the closing line of heredoc h among the lines from byte
// from, and returns the offset of that line's first byte.
func (s *actScanner) heredocEnd(from, hi int, h actHeredoc) (int, bool) {
	for ls := from; ls <= hi; {
		le := strings.IndexByte(s.src[ls:hi], '\n')
		if le < 0 {
			le = hi
		} else {
			le += ls
		}
		text := s.src[ls:le]
		if h.strip {
			text = strings.TrimLeft(text, "\t")
		}
		if text == h.word {
			return ls, true
		}
		if le >= hi {
			break
		}
		ls = le + 1
	}
	return 0, false
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
		scan := actScanBlock(texts)
		for i, l := range body {
			t := strings.TrimSpace(l.text)
			if scan.trailing[i] || (!strings.HasPrefix(t, "#") && actTrailingCommentRe.MatchString(t)) {
				add(l.n, "act block code line carries a trailing # comment — put the comment on its own line (%s); a first zsh paste passes trailing text to the command, and a # after ; or | runs what follows it", actCommentRuleText)
			}
			for _, p := range scan.problems[i] {
				add(l.n, "%s", p)
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
