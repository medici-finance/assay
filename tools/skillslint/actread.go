// act-block read check: every read in an act block is the whole safe shape
//
//	NAME=; read -rs NAME || exit N
//	NAME=; read -rs NAME || { …; exit N; }
//
// run by the act function's own shell, so its exit really stops the act.
//
// The check reads the whole block as one stream of shell tokens, not one line
// at a time, so a subshell, command substitution, pipeline or background that a
// read sits in is seen wherever it opens or closes. It is a small tokenizer and
// a structure walk, not a shell parser: it knows quotes, backslashes, $( ),
// backticks, ( ), the group braces, if/fi, the loops, case/esac, function
// definitions and the list and pipe operators — what decides whether an exit
// ends the act or only a subshell inside it. It errs strict: a construct it
// does not model leaves a read unproved, so the read is flagged.
package main

import (
	"regexp"
	"strconv"
	"strings"
)

// actTok is one shell token of an act block.
type actTok struct {
	op    string // an operator: ; & && | || ( ) $( ` and "\n"; a redirect starts with < or >
	raw   string // a word as written
	val   string // the word with its quotes and backslashes removed
	plain bool   // the word carries no quote and no backslash
	line  int    // the block line the token starts on
}

func (k actTok) word(s string) bool { return k.op == "" && k.plain && k.raw == s }

func (k actTok) sep() bool { return k.op == ";" || k.op == "\n" }

// actTokens splits the block's code lines into shell tokens. A double-quoted
// span is part of its word, except a $( ) or backtick span inside it, which is
// code and is split like any other: a read there runs.
func actTokens(lines []string) []actTok {
	src := strings.Join(lines, "\n")
	type mode struct {
		kind  byte // 'n' top level, '$' a $( span, '`' a backtick span, 'd' a double quote
		depth int  // ( opened inside this span and not yet closed
	}
	var (
		stack    = []mode{{kind: 'n'}}
		toks     []actTok
		raw, val strings.Builder
		inWord   bool
		plain    bool
		wordLine int
		line     int
	)
	start := func() {
		if !inWord {
			inWord, plain, wordLine = true, true, line
		}
	}
	flush := func() {
		if inWord {
			toks = append(toks, actTok{raw: raw.String(), val: val.String(), plain: plain, line: wordLine})
			raw.Reset()
			val.Reset()
			inWord = false
		}
	}
	emit := func(op string) {
		flush()
		toks = append(toks, actTok{op: op, line: line})
	}
	// escaped writes a backslash and the character after it; a backslash before
	// a newline joins the lines.
	escaped := func(i int) int {
		start()
		plain = false
		raw.WriteByte('\\')
		if i+1 < len(src) {
			raw.WriteByte(src[i+1])
			if src[i+1] == '\n' {
				line++
			} else {
				val.WriteByte(src[i+1])
			}
			return i + 1
		}
		return i
	}
	for i := 0; i < len(src); i++ {
		c := src[i]
		top := &stack[len(stack)-1]
		if top.kind == 'd' {
			switch {
			case c == '"':
				stack = stack[:len(stack)-1]
				start()
				raw.WriteByte(c)
			case c == '\\':
				i = escaped(i)
			case c == '$' && i+1 < len(src) && src[i+1] == '(':
				emit("$(")
				stack = append(stack, mode{kind: '$'})
				i++
			case c == '`':
				emit("`")
				stack = append(stack, mode{kind: '`'})
			default:
				start()
				plain = false
				if c == '\n' {
					line++
				}
				raw.WriteByte(c)
				val.WriteByte(c)
			}
			continue
		}
		switch {
		case c == '\n':
			emit("\n")
			line++
		case c == ' ' || c == '\t' || c == '\r':
			flush()
		case c == '\'':
			start()
			plain = false
			j := strings.IndexByte(src[i+1:], '\'')
			if j < 0 {
				j = len(src) - i - 1
			}
			seg := src[i+1 : i+1+j]
			raw.WriteByte(c)
			raw.WriteString(seg)
			val.WriteString(seg)
			line += strings.Count(seg, "\n")
			i += j + 1
			if i < len(src) {
				raw.WriteByte(c)
			}
		case c == '"':
			start()
			plain = false
			raw.WriteByte(c)
			stack = append(stack, mode{kind: 'd'})
		case c == '\\':
			i = escaped(i)
		case c == '$' && i+1 < len(src) && src[i+1] == '(':
			emit("$(")
			stack = append(stack, mode{kind: '$'})
			i++
		case c == '`':
			emit("`")
			if top.kind == '`' {
				stack = stack[:len(stack)-1]
			} else {
				stack = append(stack, mode{kind: '`'})
			}
		case c == '(':
			emit("(")
			top.depth++
		case c == ')':
			emit(")")
			if top.depth > 0 {
				top.depth--
			} else if top.kind == '$' {
				stack = stack[:len(stack)-1]
			}
		case c == ';' || c == '&' || c == '|':
			op := string(c)
			if (c == '&' || c == '|') && i+1 < len(src) && src[i+1] == c {
				op += string(c)
				i++
			}
			emit(op)
		case c == '<' || c == '>':
			op := string(c)
			if i+1 < len(src) && strings.IndexByte("<>&|", src[i+1]) >= 0 {
				op += string(src[i+1])
				i++
			}
			emit(op)
		default:
			start()
			raw.WriteByte(c)
			val.WriteByte(c)
		}
	}
	flush()
	return toks
}

// actFrame is one construct the walk is inside.
type actFrame struct {
	kind   byte   // 'r' the block, '(' subshell, '$' command substitution, '`' backtick span, '{' group, 'i' if, 'l' loop, 'c' case, 'a' the act function's body, 'f' another function's body
	closer string // the token that closes it
	piped  bool   // its opener follows a |
	open   int    // the opener's token index
	reads  []int  // reads proved safe inside it, invalidated if it turns out piped or backgrounded
	failOf int    // for a safe shape's failure group: the read's token index + 1
}

// actAssignRe is the empty assignment NAME= that clears the name.
var actAssignRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*=$`)

// actReadOptsRe is one option word the read may carry.
var actReadOptsRe = regexp.MustCompile(`^-[rs]+$`)

// actUnsafeReadLines returns the block lines holding a read that is not proved
// to be a whole safe shape run by the act function's own shell. A read is any
// word whose text, quotes and backslashes removed, is read — wherever it sits.
func actUnsafeReadLines(lines []string, actName string) map[int]bool {
	toks := actTokens(lines)
	safe := map[int]bool{}
	groupOf := map[int]int{} // the { token of a candidate's failure group -> read index + 1
	stack := []*actFrame{{kind: 'r'}}
	cmdPos, afterRedirect := true, false
	pendingFunc := ""

	prevSig := func(k int) int {
		for k--; k >= 0 && toks[k].op == "\n"; k-- {
		}
		return k
	}
	// pipedBefore reports whether the command starting at token k is the right
	// side of a pipe.
	pipedBefore := func(k int) bool {
		p := prevSig(k)
		for p >= 0 && (toks[p].word("!") || toks[p].word("time")) {
			p = prevSig(p)
		}
		return p >= 0 && toks[p].op == "|"
	}
	// followedByPipeOrBackground reports whether the construct closed at token k
	// is piped on or run in the background, past any redirects after it.
	followedByPipeOrBackground := func(k int) bool {
		j := k + 1
		for j < len(toks) {
			switch {
			case actRedirect(toks[j]):
				j += 2
			case actFDWordRe.MatchString(toks[j].raw) && toks[j].plain && j+1 < len(toks) && actRedirect(toks[j+1]):
				j += 3
			default:
				return toks[j].op == "|" || toks[j].op == "&"
			}
		}
		return false
	}
	push := func(k int, kind byte, closer string) *actFrame {
		if pendingFunc != "" {
			kind = 'f'
			if pendingFunc == actName {
				kind = 'a'
			}
			pendingFunc = ""
		}
		f := &actFrame{kind: kind, closer: closer, piped: pipedBefore(k), open: k}
		stack = append(stack, f)
		return f
	}
	// contextSafe reports whether an exit at this point ends the act: inside the
	// act function's body, through nothing but groups, ifs, loops and cases that
	// are not the right side of a pipe.
	contextSafe := func() bool {
		inAct := false
		for _, f := range stack[1:] {
			switch {
			case f.kind == 'a':
				inAct = true
			case strings.IndexByte("{ilc", f.kind) < 0 || f.piped:
				return false
			}
		}
		return inAct
	}
	register := func(r int) {
		safe[r] = true
		top := stack[len(stack)-1]
		top.reads = append(top.reads, r)
	}
	invalidate := func(f *actFrame) {
		for _, r := range f.reads {
			safe[r] = false
		}
		f.reads = nil
	}
	closeFrame := func(k int) {
		f := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		parent := stack[len(stack)-1]
		bad := f.piped || followedByPipeOrBackground(k)
		if bad {
			invalidate(f)
		} else {
			parent.reads = append(parent.reads, f.reads...)
		}
		if f.failOf == 0 {
			return
		}
		// A failure group proves its read only when its last command is exit N
		// at its own level and the group itself is a plain list item.
		n := k - 4
		if !bad && n >= f.open && toks[k-1].sep() && toks[k-3].word("exit") && actExitStatusOK(toks[k-2]) &&
			(n == f.open || toks[n].sep()) && (k+1 == len(toks) || toks[k+1].sep()) {
			safe[f.failOf-1] = true
			parent.reads = append(parent.reads, f.failOf-1)
		}
	}
	unmatched := func() { invalidate(stack[len(stack)-1]) }

	// shape tries the safe shape whose clear is the word at k, in command
	// position. A bare exit branch proves the read at once; a failure group
	// proves it when the group closes (closeFrame).
	shape := func(k int) {
		t := toks[k]
		if !t.plain || !actAssignRe.MatchString(t.raw) {
			return
		}
		if k > 0 {
			p := toks[k-1]
			switch {
			case p.op == ";":
			case p.op == "\n":
				if q := prevSig(k); q >= 0 && (toks[q].op == "|" || toks[q].op == "&&" || toks[q].op == "||") {
					return
				}
			case p.word("{") || p.word("then") || p.word("else") || p.word("do"):
			default:
				return
			}
		}
		name := strings.TrimSuffix(t.raw, "=")
		j := k + 1
		if j+1 >= len(toks) || toks[j].op != ";" || !toks[j+1].word("read") {
			return
		}
		r := j + 1
		opts := ""
		for j = r + 1; j < len(toks) && toks[j].plain && actReadOptsRe.MatchString(toks[j].raw); j++ {
			opts += toks[j].raw
		}
		if !strings.Contains(opts, "r") || !strings.Contains(opts, "s") {
			return
		}
		if j+2 >= len(toks) || !toks[j].word(name) || toks[j+1].op != "||" {
			return
		}
		b := j + 2
		if !contextSafe() {
			return
		}
		switch {
		case toks[b].word("exit") && b+1 < len(toks) && actExitStatusOK(toks[b+1]) &&
			(b+2 == len(toks) || toks[b+2].sep()):
			register(r)
		case toks[b].word("{"):
			groupOf[b] = r + 1
		}
	}

	for k := 0; k < len(toks); k++ {
		t := toks[k]
		if pendingFunc != "" && t.op != "\n" && t.op != "(" && !(cmdPos && actCompoundOpeners[t.raw] && t.plain) {
			pendingFunc = ""
		}
		if t.op != "" {
			switch t.op {
			case "\n", ";", "&", "&&", "||", "|":
				cmdPos, afterRedirect = true, false
			case "(":
				push(k, '(', ")")
				cmdPos = true
			case "$(":
				push(k, '$', ")")
				cmdPos = true
			case "`":
				if stack[len(stack)-1].kind == '`' {
					closeFrame(k)
					cmdPos = false
				} else {
					push(k, '`', "`")
					cmdPos = true
				}
			case ")":
				switch top := stack[len(stack)-1]; {
				case top.kind == 'c':
					cmdPos = true
				case top.closer == ")":
					closeFrame(k)
					cmdPos = false
				default:
					unmatched()
					cmdPos = false
				}
			default:
				afterRedirect = true
			}
			continue
		}
		if afterRedirect {
			afterRedirect = false
			continue
		}
		if !cmdPos || !t.plain {
			cmdPos = false
			continue
		}
		if k+2 < len(toks) && toks[k+1].op == "(" && toks[k+2].op == ")" {
			pendingFunc = t.raw
			k += 2
			cmdPos = true
			continue
		}
		if t.raw == "function" && k+1 < len(toks) && toks[k+1].op == "" {
			pendingFunc = toks[k+1].raw
			k++
			if k+2 < len(toks) && toks[k+1].op == "(" && toks[k+2].op == ")" {
				k += 2
			}
			cmdPos = true
			continue
		}
		switch t.raw {
		case "{":
			f := push(k, '{', "}")
			if r := groupOf[k]; r != 0 {
				f.failOf = r
			}
			cmdPos = true
		case "if":
			push(k, 'i', "fi")
			cmdPos = true
		case "while", "until":
			push(k, 'l', "done")
			cmdPos = true
		case "for", "select":
			push(k, 'l', "done")
			cmdPos = false
		case "case":
			push(k, 'c', "esac")
			cmdPos = false
		case "then", "else", "elif", "do", "!", "time":
			cmdPos = true
		case "}", "fi", "done", "esac":
			if stack[len(stack)-1].closer == t.raw {
				closeFrame(k)
			} else {
				unmatched()
			}
			cmdPos = false
		default:
			shape(k)
			cmdPos = false
		}
	}
	bad := map[int]bool{}
	for k, t := range toks {
		if t.op == "" && t.val == "read" && !safe[k] {
			bad[t.line] = true
		}
	}
	return bad
}

// actCompoundOpeners are the words that open a compound command, which a
// function definition's body can be.
var actCompoundOpeners = map[string]bool{"{": true, "if": true, "while": true, "until": true, "for": true, "select": true, "case": true}

// actFDWordRe is a file-descriptor number ahead of a redirect, as in 2>&1.
var actFDWordRe = regexp.MustCompile(`^[0-9]+$`)

// actRedirect reports whether t is a redirect operator.
func actRedirect(t actTok) bool { return strings.HasPrefix(t.op, "<") || strings.HasPrefix(t.op, ">") }

// actExitStatusOK reports whether t is an exit status a shell reports as
// non-zero: 1 to 255.
func actExitStatusOK(t actTok) bool {
	if t.op != "" || !t.plain {
		return false
	}
	n, err := strconv.Atoi(t.raw)
	return err == nil && n >= 1 && n <= 255 && t.raw[0] != '0'
}
