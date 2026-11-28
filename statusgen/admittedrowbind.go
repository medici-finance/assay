package main

import (
	"regexp"
	"slices"
	"strings"
)

// A row binds a name for itself when every read of it sees a value the row's
// own text supplies (#7134). The scrub (admittedRowEnv) has already removed
// any inherited value, so such a row runs as written under admission and is
// not refused (admittedRowScrubbedRefs). This file decides that from the
// command's text with a small lexer, not a shell parser. A read it finds no
// binding for counts, so the row stays refused; admittedRowSelfBound names
// forms the lexer can wrongly take as binding.

// admittedRowBindKind is how far one of the row's own assignments reaches.
type admittedRowBindKind int

const (
	// bindShell is an assignment in the row's own shell (NAME=value alone, or
	// ${NAME:=value}): later reads in that shell see it, a child does not.
	bindShell admittedRowBindKind = iota
	// bindExported is export NAME=value: later reads see it, a child too.
	bindExported
	// bindPrefix is NAME=value cmd or env NAME=value cmd: only cmd's own
	// environment carries it, so only a read in a single-quoted script handed
	// to cmd sees it. A read in cmd's own arguments sees the shell's value.
	bindPrefix
)

// admittedRowBind is one place the row's command gives a name its own value.
type admittedRowBind struct {
	name string
	kind admittedRowBindKind
	// from is the offset the value takes effect at, just past the assignment,
	// so a read inside the assigned value itself is not covered by it.
	from int
	// to ends a bindPrefix binding: the end of the one simple command.
	to int
	// quote is the single-quoted span a bindShell binding sits in, -1 for none.
	quote int
}

// covers reports whether a read at offset at, inside single-quoted span quote
// (-1 for none), sees this binding's value.
func (b admittedRowBind) covers(at, quote int) bool {
	if at < b.from {
		return false
	}
	switch b.kind {
	case bindExported:
		return true
	case bindPrefix:
		return at < b.to && quote >= 0
	default:
		return quote == b.quote
	}
}

// admittedRowSelfBound reports whether every read of name in the command sees
// a value the row supplies itself, never one it would inherit. A $NAME or
// ${NAME} read is the row's own when it is
//
//   - a non-empty default: ${NAME:-x}, ${NAME:=x}, ${NAME-x} or ${NAME=x};
//   - after ${NAME:=x} or ${NAME=x} in the same shell: not inside ( ), $( ) or
//     a backquote pair, not in a command that is part of a pipeline, follows
//     ||, or is sent to the background, and not behind an escaped $, since a
//     child shell or subshell makes that assignment and the row's shell does
//     not (a single-quoted script is read the same way, as a script of its own);
//   - after NAME=value standing alone, or export NAME=value, at the start of a
//     top-level command that is not part of a pipeline, not after ||, and not
//     sent to the background, since none of those assign in the row's shell;
//   - inside a single-quoted script of the one command a leading NAME=value
//     cmd or env NAME=value cmd assigns it for.
//
// Every other read is not. An $env:NAME or %NAME% read never is. Neither is a
// $NAME or ${NAME form that is not a non-empty default and that none of the
// bindings above covers: a bare read, an empty default (${NAME:-}, whose row
// still needs the input), ${NAME:?}, ${NAME:+x}, a length or indirection, or a
// read in a prefixed command's own arguments.
//
// An assignment made inside ( ), $( ) or a backquote pair, in a pipeline,
// after || or in the background binds nothing, written NAME=value or
// ${NAME:=x}. Quotes and then, do and else differ between the two. NAME=value
// binds nothing inside a quoted string, or after then, do or else on the same
// line. A double-quoted "${NAME:=x}" in the row's own shell does bind, one
// inside a single-quoted string binds only for reads in that same string, and
// then : "${NAME:=x}" binds.
//
// It reads the text, not the shell's parse or its control flow. An assignment
// the row may skip at run time (false && NAME=x; echo $NAME, or a line of an
// if body) still covers the later read. It also finds the contexts above only
// in their plain form, so it can take an assignment the shell does not keep as
// binding, for example: one after a ) inside ( ) or $( ) that belongs to a
// case pattern or a comment and so ends the span early; one inside a $( ) in a
// double-quoted string that holds double quotes of its own; one in a { } group
// or a loop that is piped or sent to the background as a whole; one in a
// pipeline continued on the next line, or in an && list sent to the
// background; a ${NAME:=x} written in a comment; and a here-document's lines,
// which are read as commands. The row then runs with the name unset, as a
// printenv NAME row already does; it never sees an inherited value, because
// the scrub has removed it.
func admittedRowSelfBound(command, name string) bool {
	binds, quotes := admittedRowBindings(command)
	quoteAt := func(at int) int {
		for k, q := range quotes {
			if q[0] <= at && at < q[1] {
				return k
			}
		}
		return -1
	}
	var own []admittedRowBind
	for _, b := range binds {
		if b.name == name {
			own = append(own, b)
		}
	}
	reads := 0
	var transient [][2]int
	for _, loc := range admittedRowRefRe.FindAllStringSubmatchIndex(command, -1) {
		start, end := loc[0], loc[1]
		switch {
		case loc[2] >= 0:
			if strings.EqualFold(command[loc[2]:loc[3]], name) {
				return false
			}
			continue
		case loc[6] >= 0:
			if strings.EqualFold(command[loc[6]:loc[7]], name) {
				return false
			}
			continue
		case command[loc[4]:loc[5]] != name:
			continue
		}
		reads++
		quote := quoteAt(start)
		if strings.HasPrefix(command[start:], "${") && command[start+2] != '#' && command[start+2] != '!' {
			if assigns, after, ok := admittedRowDefault(command, end); ok {
				if assigns {
					if transient == nil {
						transient = admittedRowTransient(command, 0, len(command))
						for _, q := range quotes {
							transient = append(transient, admittedRowTransient(command, q[0]+1, q[1])...)
						}
					}
					// An assignment inside ( ), $( ), a backquote, a pipeline
					// segment, after ||, in the background, or behind an
					// escaped $ is not made in the shell that reads later.
					if !admittedRowInSpan(transient, start) && !admittedRowEscaped(command, start) {
						own = append(own, admittedRowBind{name: name, kind: bindShell, from: after, quote: quote})
					}
				}
				continue
			}
		}
		if !slices.ContainsFunc(own, func(b admittedRowBind) bool { return b.covers(start, quote) }) {
			return false
		}
	}
	return reads > 0
}

// admittedRowDefault reads the operator after the name of a ${NAME read that
// ends at offset at. For a default form (:-, :=, - or =) whose default is not
// empty, it reports whether the form also assigns the name (:= and =) and the
// offset just past its closing brace. An empty default, a default of only
// quotes and blanks, any other operator and an unclosed brace are not one.
func admittedRowDefault(command string, at int) (assigns bool, after int, ok bool) {
	rest := command[at:]
	op := 0
	switch {
	case strings.HasPrefix(rest, ":-"), strings.HasPrefix(rest, ":="):
		op = 2
	case strings.HasPrefix(rest, "-"), strings.HasPrefix(rest, "="):
		op = 1
	default:
		return false, 0, false
	}
	depth := 1
	for i := at + op; i < len(command); i++ {
		switch command[i] {
		case '\\':
			i++
		case '{':
			depth++
		case '}':
			if depth--; depth == 0 {
				value := strings.Trim(command[at+op:i], "\"' \t")
				return rest[op-1] == '=', i + 1, value != ""
			}
		}
	}
	return false, 0, false
}

// admittedRowInSpan reports whether offset at lies inside one of spans.
func admittedRowInSpan(spans [][2]int, at int) bool {
	return slices.ContainsFunc(spans, func(s [2]int) bool { return s[0] <= at && at < s[1] })
}

// admittedRowEscaped reports whether the $ at offset at is escaped by an odd
// run of backslashes, so the shell that reads the text never expands it.
func admittedRowEscaped(command string, at int) bool {
	n := 0
	for k := at - 1; k >= 0 && command[k] == '\\'; k-- {
		n++
	}
	return n%2 == 1
}

// admittedRowTransient returns the spans of command[lo:hi] in which an
// assignment does not reach the shell that runs the rest of the text: inside
// ( ), $( ), <( ) or a backquote pair, and in a command that is part of a
// pipeline, follows ||, or is sent to the background. A single-quoted span is
// skipped whole (the caller scans its inside as a script of its own), and an
// open quote or substitution makes the rest transient, so it is refused.
func admittedRowTransient(command string, lo, hi int) [][2]int {
	var spans [][2]int
	segStart, after := lo, false
	// seg closes the command that began at segStart and ended at end. It is
	// transient when a pipe or || led into it, or a pipe or a lone & follows.
	seg := func(end int, leaves bool) {
		if after || leaves {
			spans = append(spans, [2]int{segStart, end})
		}
	}
	for i := lo; i < hi; {
		switch c := command[i]; c {
		case '\\':
			i += 2
		case '\'':
			e := strings.IndexByte(command[i+1:hi], '\'')
			if e < 0 {
				return append(spans, [2]int{i, hi})
			}
			i += e + 2
		case '"':
			e, ok := admittedRowDoubleQuoteEnd(command, i)
			if !ok || e > hi {
				return append(spans, [2]int{i, hi})
			}
			for k := i + 1; k < e-1; k++ {
				switch {
				case command[k] == '\\':
					k++
				case command[k] == '`' || command[k] == '(' && command[k-1] == '$':
					m := admittedRowSubstEnd(command, k, e-1)
					spans = append(spans, [2]int{k, m})
					k = m - 1
				}
			}
			i = e
		case '`', '(':
			m := admittedRowSubstEnd(command, i, hi)
			spans = append(spans, [2]int{i, m})
			i = m
		case '#':
			if i == lo || strings.IndexByte(" \t;&|()\n", command[i-1]) >= 0 {
				nl := strings.IndexByte(command[i:hi], '\n')
				if nl < 0 {
					i = hi
				} else {
					i += nl
				}
				continue
			}
			i++
		case ';', '\n':
			seg(i, false)
			segStart, after = i+1, false
			i++
		case '&':
			switch {
			case i+1 < hi && command[i+1] == '&':
				seg(i, false)
				i += 2
				segStart, after = i, false
			case i > lo && command[i-1] == '>' || i+1 < hi && command[i+1] == '>':
				i++ // a redirection (2>&1, &>), not an operator
			default:
				seg(i, true)
				i++
				segStart, after = i, false
			}
		case '|':
			leaves := true
			if i+1 < hi && command[i+1] == '|' {
				leaves = false
				i++
			} else if i+1 < hi && command[i+1] == '&' {
				i++
			}
			seg(i, leaves)
			i++
			segStart, after = i, true
		default:
			i++
		}
	}
	seg(hi, false)
	return spans
}

// admittedRowSubstEnd returns the offset just past the substitution or
// subshell opening at k (a backquote, or a parenthesis, whose matching close it
// finds past quotes and nested pairs), or hi when it is never closed.
func admittedRowSubstEnd(command string, k, hi int) int {
	if command[k] == '`' {
		if e := strings.IndexByte(command[k+1:hi], '`'); e >= 0 {
			return k + e + 2
		}
		return hi
	}
	depth := 0
	for i := k; i < hi; i++ {
		switch command[i] {
		case '\\':
			i++
		case '\'':
			e := strings.IndexByte(command[i+1:hi], '\'')
			if e < 0 {
				return hi
			}
			i += e + 1
		case '"':
			e, ok := admittedRowDoubleQuoteEnd(command, i)
			if !ok || e > hi {
				return hi
			}
			i = e - 1
		case '(':
			depth++
		case ')':
			if depth--; depth == 0 {
				return i + 1
			}
		}
	}
	return hi
}

// admittedRowAssignRe matches an assignment word's NAME= at a word's start.
var admittedRowAssignRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*=`)

// admittedRowBindings lexes the command once. It returns the assignments made
// at the start of a top-level command (admittedRowLeading) and the offsets of
// every single-quoted span outside double quotes. Quoted text, comments and
// anything inside parentheses are skipped, by plain pairing of quotes and of (
// with ), so an assignment there is not seen.
func admittedRowBindings(command string) (binds []admittedRowBind, quotes [][2]int) {
	n := len(command)
	depth, start, assigning := 0, true, true
	for i := 0; i < n; {
		if start {
			i = admittedRowSkipBlanks(command, i)
			if i >= n {
				break
			}
			start = false
			if depth == 0 && assigning {
				if next, b := admittedRowLeading(command, i); next > i {
					binds = append(binds, b...)
					i = next
					continue
				}
			}
		}
		switch command[i] {
		case '\\':
			i += 2
			continue
		case '\'':
			e := strings.IndexByte(command[i+1:], '\'')
			if e < 0 {
				return binds, append(quotes, [2]int{i, n})
			}
			quotes = append(quotes, [2]int{i, i + e + 2})
			i += e + 2
			continue
		case '"':
			e, ok := admittedRowDoubleQuoteEnd(command, i)
			if !ok {
				return binds, quotes
			}
			i = e
			continue
		case '`':
			e := strings.IndexByte(command[i+1:], '`')
			if e < 0 {
				return binds, quotes
			}
			i += e + 2
			continue
		case '#':
			if i == 0 || strings.IndexByte(" \t;&|()\n", command[i-1]) >= 0 {
				nl := strings.IndexByte(command[i:], '\n')
				if nl < 0 {
					return binds, quotes
				}
				i += nl
				continue
			}
		case ';', '\n':
			start, assigning = true, true
		case '&':
			// After && or a backgrounded command, the next command still
			// runs in the row's own shell.
			if i+1 < n && command[i+1] == '&' {
				i++
			}
			start, assigning = true, true
		case '|':
			// After || the next command runs only if the last failed; after
			// | it runs in the pipeline's own subshell. Neither is counted.
			if i+1 < n && command[i+1] == '|' {
				i++
			}
			start, assigning = true, false
		case '(':
			depth++
			start = true
		case ')':
			if depth > 0 {
				depth--
			}
		}
		i++
	}
	return binds, quotes
}

// admittedRowLeading reads the assignment words at the start of the command
// at offset i (after an optional export or env), and returns the offset past
// them with what they bind. With no assignment word it returns i.
func admittedRowLeading(command string, i int) (int, []admittedRowBind) {
	j, export, env := i, false, false
	for _, kw := range []string{"export", "env"} {
		if strings.HasPrefix(command[j:], kw+" ") || strings.HasPrefix(command[j:], kw+"\t") {
			export, env = kw == "export", kw == "env"
			j = admittedRowSkipBlanks(command, j+len(kw))
			break
		}
	}
	type word struct {
		name string
		end  int
	}
	var words []word
	for {
		m := admittedRowAssignRe.FindString(command[j:])
		if m == "" {
			break
		}
		end, ok := admittedRowWordEnd(command, j+len(m))
		if !ok {
			return i, nil
		}
		words = append(words, word{m[:len(m)-1], end})
		j = admittedRowSkipBlanks(command, end)
	}
	if len(words) == 0 {
		return i, nil
	}
	cmdEnd, persists := admittedRowCommandEnd(command, j)
	var binds []admittedRowBind
	for _, w := range words {
		switch {
		case env || !export && cmdEnd > j:
			binds = append(binds, admittedRowBind{name: w.name, kind: bindPrefix, from: w.end, to: cmdEnd, quote: -1})
		case persists && export:
			binds = append(binds, admittedRowBind{name: w.name, kind: bindExported, from: w.end, quote: -1})
		case persists:
			binds = append(binds, admittedRowBind{name: w.name, kind: bindShell, from: w.end, quote: -1})
		}
	}
	return j, binds
}

// admittedRowCommandEnd returns where the simple command whose words start at
// j ends, and whether an assignment it makes stays in the row's own shell:
// true when it ends at the text's end, a ; or newline, or &&; false at a
// pipe, ||, a background & or a closing parenthesis, and for a word the lexer
// cannot close.
func admittedRowCommandEnd(command string, j int) (int, bool) {
	for j < len(command) {
		switch command[j] {
		case ' ', '\t':
			j++
			continue
		case ';', '\n':
			return j, true
		case '&':
			return j, j+1 < len(command) && command[j+1] == '&'
		case '|', ')':
			return j, false
		case '#':
			if command[j-1] == ' ' || command[j-1] == '\t' {
				return j, true
			}
		}
		e, ok := admittedRowWordEnd(command, j)
		if !ok {
			return len(command), false
		}
		if e == j {
			e++
		}
		j = e
	}
	return j, true
}

// admittedRowWordEnd returns the offset just past the shell word that starts
// at k, honouring quotes, backslashes, backticks and nested $( ) and ${ }.
// It reports false for a quote or substitution left open.
func admittedRowWordEnd(command string, k int) (int, bool) {
	depth := 0
	for k < len(command) {
		switch c := command[k]; c {
		case '\\':
			k += 2
			continue
		case '\'', '`':
			e := strings.IndexByte(command[k+1:], c)
			if e < 0 {
				return 0, false
			}
			k += e + 2
			continue
		case '"':
			e, ok := admittedRowDoubleQuoteEnd(command, k)
			if !ok {
				return 0, false
			}
			k = e
			continue
		case '(', '{':
			depth++
		case ')', '}':
			if depth == 0 && c == ')' {
				return k, true
			}
			if depth > 0 {
				depth--
			}
		case ' ', '\t', ';', '&', '|', '\n', '<', '>':
			if depth == 0 {
				return k, true
			}
		}
		k++
	}
	return k, depth == 0
}

// admittedRowDoubleQuoteEnd returns the offset just past the double-quoted
// string opening at k, or false when it is never closed.
func admittedRowDoubleQuoteEnd(command string, k int) (int, bool) {
	for i := k + 1; i < len(command); i++ {
		switch command[i] {
		case '\\':
			i++
		case '"':
			return i + 1, true
		}
	}
	return 0, false
}

// admittedRowSkipBlanks returns the first offset at or after i that is not a
// space or a tab.
func admittedRowSkipBlanks(command string, i int) int {
	for i < len(command) && (command[i] == ' ' || command[i] == '\t') {
		i++
	}
	return i
}
