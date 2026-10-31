package writepolicy

import "strings"

// --- gh output filter that reads beyond the JSON output (#5078) ---
//
// ghFilterReads reports whether a Bash command's text holds gh and a --jq / -q
// filter, written out in the call, that names one of the names jqNameReads
// lists. Selection and formatting filters are meant to stay allowed, whatever they
// spell in a field or a jq variable of their own, or in a string the reader takes as
// one.
//
// The reader tries to read the text as shell words, taking out the quotes and
// backslashes the shell removes, without parsing it as a program: a flag it finds in
// a word counts, in any command, as long as a gh word is somewhere in the text. It
// tries to read again a quoted word that could hold a command, and a command
// substitution. Text the reader does not model can leave it out of step with the
// shell, so resync reads the text again without what came before. These attempts do
// not always succeed.
// docs/write-policy.md states the limits.
//
// Cost: one pass for the gh word, at most ghFilterDepth+1 passes over any byte in
// each reading, each of which reads a filter value at most twice, and a shared
// budget for reading again: spent, the whole text is judged with no string credit.
func ghFilterReads(s string) bool {
	if !forgeHasWord(s, "gh", true) {
		return false
	}
	g := ghFilterScan{text: s, budget: ghReadAgainFloor + ghReadAgainWork*len(s)}
	g.commands(s, 0, ghToEnd, 0)
	g.resync(s, 0)
	return g.found
}

const (
	// ghFilterDepth bounds how far words nested in words are read. Past it the
	// rest of the text is judged with no string credit.
	ghFilterDepth = 8
	// ghExpansion stands in a word for what the shell expands there. It is no
	// byte of a name, so nothing inside the expansion is read as filter text.
	ghExpansion = 0
	// ghDollarQuote stands in a word for the `$` of `$"` and `$'`, which some of
	// the shells drop and others keep. A name right after it is read both ways.
	ghDollarQuote = 1
	// ghReadAgainWork and ghReadAgainFloor set the budget for reading text again,
	// in bytes: multiples of the text's length, with a floor.
	ghReadAgainWork  = 8
	ghReadAgainFloor = 1 << 16
)

// ghStop says where commands stops reading.
type ghStop int

const (
	ghToEnd     ghStop = iota // the end of the text
	ghToParen                 // the `)` closing a command substitution
	ghOneFilter               // the end of the first word, or of its filter if it is a flag
)

type ghFilterScan struct {
	found  bool
	spent  bool   // the budget is gone, and the whole text has been judged
	text   string // the whole text
	budget int
}

// spend takes n bytes from the budget, and reports whether any is left.
func (g *ghFilterScan) spend(n int) bool {
	if g.budget -= n; g.budget < 0 && !g.spent {
		g.spent = true
		g.found = g.found || jqReadsOutside(g.text, false)
	}
	return !g.spent
}

// resync reads s again with no state carried from earlier text.
func (g *ghFilterScan) resync(s string, d int) {
	for p := 0; p < len(s) && !g.found && !g.spent; p++ {
		if p > 0 && strings.IndexByte(" \t\n;&|(", s[p-1]) < 0 || strings.IndexByte("-'\"\\${", s[p]) < 0 {
			continue
		}
		g.spend(g.commands(s, p, ghOneFilter, d) - p)
	}
}

// commands reads the words of s from p until stop, and returns the offset it
// stopped at.
func (g *ghFilterScan) commands(s string, p int, stop ghStop, d int) int {
	if d > ghFilterDepth {
		g.found = g.found || jqReadsOutside(s[min(p, len(s)):], false)
		return len(s)
	}
	parens := 0
	pending, credit, keep, read := false, false, false, false
	for p < len(s) && !g.found {
		if stop == ghOneFilter && read && !pending {
			return p
		}
		c := s[p]
		switch {
		case c == ' ' || c == '\t':
			p++
		case c == '\\' && p+1 < len(s) && s[p+1] == '\n':
			p += 2
		case c == '<' || c == '>' || c == '&' && p+1 < len(s) && s[p+1] == '>':
			var both bool
			p, both = g.redirection(s, p, d)
			keep = keep || both
		case c == ')' && stop == ghToParen && parens == 0:
			return p + 1
		case strings.IndexByte("\n;&|()", c) >= 0:
			if c == '(' {
				parens++
			} else if c == ')' {
				parens--
			}
			pending, keep = false, false
			p++
		default:
			w := g.word(s, p, d)
			p, read = w.end, true
			if w.namesDescriptor(s) {
				continue
			}
			wasValue := pending
			if pending {
				if w.pieces == 0 && strings.Trim(w.text, "\x00") == "" {
					continue // an expansion alone may vanish: the next word can be the value
				}
				pending = keep // an ambiguous redirection: the next word may be the value too
				if w.brace || jqReadsOutside(w.text, credit && w.plain(0)) {
					g.found = true
					continue
				}
			}
			keep = false
			value, strs, next, flag := w.filterFlag()
			switch {
			case w.braceFlag():
				g.found = true
			case next:
				pending, credit = true, w.pieces == 0 && w.exact
			case flag:
				g.found = jqReadsOutside(value, strs)
			case !wasValue:
				g.nested(w, d)
			}
		}
	}
	return min(p, len(s))
}

// redirection tries to step past a redirection at p and its target; a target in
// quotes can still hold a command (a here-string).
// A process substitution is left to the caller as a command list. The operator's
// `-` (`<<-`) and `!` are part of it. Only zsh reads a `!` that way: with a blank
// after it, the next word is a target there and an ordinary word elsewhere, so it
// is left unread, and both is set so that the caller reads it as either.
func (g *ghFilterScan) redirection(s string, p, d int) (end int, both bool) {
	for p < len(s) && strings.IndexByte("<>&|", s[p]) >= 0 {
		p++
	}
	if p < len(s) && s[p] == '-' && strings.HasSuffix(s[:p], "<<") {
		p++
	}
	bang := p < len(s) && s[p] == '!'
	if bang {
		p++
	}
	if p < len(s) && s[p] == '(' {
		return p, false
	}
	gap := p
	for p < len(s) && (s[p] == ' ' || s[p] == '\t' || strings.HasPrefix(s[p:], "\\\n")) {
		if s[p] == '\\' {
			p++
		}
		p++
	}
	if p == len(s) || strings.IndexByte("\n;&|()<>", s[p]) >= 0 {
		return p, false
	}
	if bang && p > gap {
		return p, true
	}
	w := g.word(s, p, d)
	g.nested(w, d)
	return w.end, false
}

// nested reads a word again as commands when quoting could be hiding some in it.
func (g *ghFilterScan) nested(w ghWord, d int) {
	if (w.pieces > 0 || !w.exact) && strings.ContainsAny(w.text, " \t\n;&|()<>`'\"\\$") && g.spend(len(w.text)) {
		g.commands(w.text, 0, ghToEnd, d+1)
		g.resync(w.text, d+1)
	}
}

// ghFilterFlag reports whether word is a filter flag, and where its value starts
// in the word; at is -1 when the value is the next word. The spellings are the
// ones gh accepts: `--jq x`, `--jq=x`, `-q x`, `-q=x`, `-qx`, and -q ending a
// short-flag cluster (`-iq x`, `-iqx`).
func ghFilterFlag(word string) (at int, flag bool) {
	switch {
	case word == "--jq":
		return -1, true
	case strings.HasPrefix(word, "--jq="):
		return 5, true
	case len(word) < 2 || word[0] != '-' || word[1] == '-':
		return 0, false
	}
	for i := 1; i < len(word); i++ {
		switch {
		case word[i] != 'q' && 'a' <= word[i]|0x20 && word[i]|0x20 <= 'z':
		case word[i] != 'q':
			return 0, false
		case i+1 == len(word):
			return -1, true
		case word[i+1] == '=':
			return i + 2, true
		default:
			return i + 1, true
		}
	}
	return 0, false
}

// ghStandIns removes the stand-ins from a word's text.
var ghStandIns = strings.NewReplacer(string(rune(ghExpansion)), "", string(rune(ghDollarQuote)), "")

// filterFlag reads the word as a filter flag. A stand-in is taken to give nothing,
// so the flag is read with it removed. value is what the word gives the filter,
// stand-ins kept, and strs whether its strings count as strings; next
// is set when the value is the next word.
func (w ghWord) filterFlag() (value string, strs, next, flag bool) {
	bare := ghStandIns.Replace(w.text)
	at, flag := ghFilterFlag(bare)
	switch {
	case !flag:
		return "", false, false, false
	case at < 0:
		return "", false, true, true
	case len(bare) == len(w.text):
		return w.text[at:], w.plain(at), false, true
	}
	for i, n := 0, 0; i < len(w.text); i++ {
		if n == at {
			return w.text[i:], false, false, true
		}
		if w.text[i] != ghExpansion && w.text[i] != ghDollarQuote {
			n++
		}
	}
	return "", false, false, true
}

// ghWord is the reader's take on one shell word, with the quotes and backslashes it
// removes taken out.
type ghWord struct {
	text   string
	end    int  // offset just past the word
	exact  bool // no bare backslash or $'…', whose quoting may not be the filter's
	pieces int  // quoted pieces
	quoted int  // offset in text of the first quoted piece, or -1
	after  bool // unquoted text follows the last quoted piece
	brace  bool // holds an unquoted brace expansion
}

// plain reports whether the word from offset at is one piece of quoting with no
// flag quoted before it, the only case where its jq strings are taken at face value.
func (w ghWord) plain(at int) bool {
	return w.exact && (w.pieces == 0 || w.pieces == 1 && w.quoted == at && !w.after)
}

// namesDescriptor reports whether the word is the 2 of `2>f` or the {x} of `{x}>f`.
func (w ghWord) namesDescriptor(s string) bool {
	if w.pieces > 0 || !w.exact || w.end >= len(s) || s[w.end] != '<' && s[w.end] != '>' {
		return false
	}
	if n := len(w.text); n > 2 && w.text[0] == '{' && w.text[n-1] == '}' {
		return strings.TrimFunc(w.text[1:n-1], func(r rune) bool { return r < 0x80 && jqNameByte(byte(r)) }) == "" &&
			(w.text[1] < '0' || w.text[1] > '9')
	}
	return strings.Trim(w.text, "0123456789") == ""
}

// braceFlag reports whether a brace expansion could produce a filter flag. The
// expansion is not worked out, so this refuses one it finds that could.
func (w ghWord) braceFlag() bool {
	t := ghStandIns.Replace(w.text)
	if !w.brace || t == "" || t[0] != '-' && t[0] != '{' {
		return false
	}
	return strings.Contains(t, "-") && (strings.Contains(t, "q") || strings.Contains(t, ".."))
}

// word reads the shell word at p. An unterminated quote runs to the end of s.
func (g *ghFilterScan) word(s string, p, d int) ghWord {
	var b strings.Builder
	w := ghWord{exact: true, quoted: -1}
	piece := func() {
		if w.quoted < 0 {
			w.quoted = b.Len()
		}
		w.pieces++
		w.after = false
	}
	opened, parted := false, false
	for p < len(s) {
		c := s[p]
		switch {
		case c == '\'':
			piece()
			j := strings.IndexByte(s[p+1:], '\'')
			if j < 0 {
				b.WriteString(s[p+1:])
				p = len(s)
				break
			}
			b.WriteString(s[p+1 : p+1+j])
			p += j + 2
		case c == '$' && p+1 < len(s) && s[p+1] == '\'':
			piece()
			w.exact = false
			b.WriteByte(ghDollarQuote)
			for p += 2; p < len(s) && s[p] != '\''; p++ {
				if s[p] == '\\' && p+1 < len(s) {
					p++
				}
				b.WriteByte(s[p])
			}
			p++
		case c == '$' && p+1 < len(s) && s[p+1] == '"':
			w.exact = false
			b.WriteByte(ghDollarQuote)
			p++
		case c == '"':
			piece()
			p = g.doubleQuoted(&b, s, p+1, d)
		case c == '\\':
			w.exact = false
			if p+1 < len(s) && s[p+1] != '\n' {
				b.WriteByte(s[p+1])
				w.after = w.pieces > 0
			}
			p += 2
		case shellExpands(s, p):
			w.after = w.pieces > 0
			p = g.expansion(&b, s, p, false, d)
		case strings.IndexByte(" \t\n;&|()<>", c) >= 0:
			w.text, w.end = b.String(), p
			return w
		default:
			switch {
			case c == '{':
				opened = true
			case opened && (c == ',' || c == '.' && p+1 < len(s) && s[p+1] == '.'):
				parted = true
			case parted && c == '}':
				w.brace = true
			}
			b.WriteByte(c)
			w.after = w.pieces > 0
			p++
		}
	}
	w.text, w.end = b.String(), len(s)
	return w
}

// doubleQuoted reads a double-quoted piece from just past its opening quote.
func (g *ghFilterScan) doubleQuoted(b *strings.Builder, s string, p, d int) int {
	for p < len(s) {
		switch {
		case s[p] == '"':
			return p + 1
		case s[p] == '\\' && p+1 < len(s) && s[p+1] == '\n':
			p += 2
		case s[p] == '\\' && p+1 < len(s) && strings.IndexByte("\"\\$`", s[p+1]) >= 0:
			b.WriteByte(s[p+1])
			p += 2
		case shellExpands(s, p):
			p = g.expansion(b, s, p, true, d)
		default:
			b.WriteByte(s[p])
			p++
		}
	}
	return p
}

// shellExpands reports whether an unquoted or double-quoted s[p] starts an expansion.
func shellExpands(s string, p int) bool {
	if s[p] == '`' {
		return true
	}
	if s[p] != '$' || p+1 == len(s) {
		return false
	}
	c := s[p+1]
	return jqNameByte(c) || strings.IndexByte("({?!#*@-$", c) >= 0
}

// expansion writes the stand-in for the expansion at p and returns the offset
// past it. What the shell would compute is not filter text, but commands
// substituted there still run, so the reader tries to read them as commands. dq says
// whether it sits in double quotes.
func (g *ghFilterScan) expansion(b *strings.Builder, s string, p int, dq bool, d int) int {
	b.WriteByte(ghExpansion)
	switch {
	case s[p] == '`':
		var body strings.Builder
		for p++; p < len(s) && s[p] != '`'; p++ {
			if s[p] == '\\' && p+1 < len(s) && strings.IndexByte("`\\$", s[p+1]) >= 0 {
				p++
			}
			body.WriteByte(s[p])
		}
		g.commands(body.String(), 0, ghToEnd, d+1)
		return p + 1
	case s[p+1] == '(':
		return g.commands(s, p+2, ghToParen, d+1)
	case s[p+1] == '{':
		return g.parameter(s, p+2, dq, d)
	}
	q := p + 2 // a digit or a special parameter is one byte
	if c := s[p+1]; jqNameByte(c) && (c < '0' || c > '9') {
		for q < len(s) && jqNameByte(s[q]) {
			q++
		}
	}
	return q
}

// parameter steps past a ${…} body, trying to read a command substituted inside it. A
// brace in quotes does not close it; in double quotes a single quote is plain text.
func (g *ghFilterScan) parameter(s string, p int, dq bool, d int) int {
	inner := false
	for open := 1; p < len(s) && open > 0; {
		switch {
		case s[p] == '\\':
			p += 2
		case strings.HasPrefix(s[p:], "$(") || s[p] == '`':
			var sink strings.Builder
			p = g.expansion(&sink, s, p, false, d)
		case s[p] == '"':
			inner = !inner
			p++
		case inner:
			p++
		case s[p] == '\'' && !dq:
			j := strings.IndexByte(s[p+1:], '\'')
			if j < 0 {
				return len(s)
			}
			p += j + 2
		case s[p] == '{':
			open++
			p++
		case s[p] == '}':
			open--
			p++
		default:
			p++
		}
	}
	return p
}

// jqReadsOutside reports whether filter q names a jq function or variable that
// reads beyond the JSON gh hands it. With strs set, the body of a string
// literal is skipped, except for its \(…) interpolations, which are code. A
// backslash or `#` outside a string means this reading of the quotes cannot be
// trusted (a nested command, a comment holding a quote), so the scan restarts
// without string credit. A name right after one `.` is a field, not a call.
func jqReadsOutside(q string, strs bool) bool {
	var open []int
	depth, inStr := 0, false
	for i := 0; i < len(q); {
		c := q[i]
		switch {
		case inStr && c == '\\':
			if i+1 < len(q) && q[i+1] == '(' {
				open, depth, inStr = append(open, depth), 0, false
			}
			i += 2
		case inStr:
			inStr = c != '"'
			i++
		case strs && (c == '\\' || c == '#'):
			return jqReadsOutside(q, false)
		case strs && c == '"':
			inStr = true
			i++
		case strs && c == '(':
			depth++
			i++
		case strs && c == ')':
			if depth == 0 && len(open) > 0 {
				depth, open, inStr = open[len(open)-1], open[:len(open)-1], true
			} else {
				depth--
			}
			i++
		case jqNameByte(c) && (c < '0' || c > '9'):
			j := i
			for j < len(q) && jqNameByte(q[j]) {
				j++
			}
			if jqNameReads(q, i, j) {
				return true
			}
			i = j
		default:
			i++
		}
	}
	return false
}

func jqNameByte(c byte) bool {
	return c == '_' || '0' <= c && c <= '9' || 'a' <= c|0x20 && c|0x20 <= 'z'
}

// jqNameReads judges the name q[i:j] by what comes before it. The list holds the
// names that read beyond the JSON output on the gh version docs/write-policy.md names.
func jqNameReads(q string, i, j int) bool {
	name := q[i:j]
	if i > 0 && q[i-1] == '$' {
		return name == "ENV"
	}
	if i > 0 && q[i-1] == ghDollarQuote && name == "ENV" {
		return true
	}
	if i > 0 && q[i-1] == '.' && (i < 2 || q[i-2] != '.') {
		return false
	}
	switch name {
	case "env", "now", "localtime", "strflocaltime":
		return true
	}
	return false
}
