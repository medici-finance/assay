// act-block driver-token pin: every `gh` step an Act block runs carries the
// driver's token, pinned once at the top of the act function.
//
// An Act block's `gh` steps run in the shell the driver pastes into. In that
// shell GH_TOKEN and GITHUB_TOKEN take precedence over any stored login, so an
// exported role-App token (the driver's shell may hold one from desk work) would
// run the act as the App: the wrong identity, and an App-posted change fails a
// human gate. Without an export the active account is used, which may not be the
// driver. So the act function pins the driver's own login for every step, fail
// closed, and clears the second variable `gh` reads:
//
//	GH_TOKEN="$(gh auth token --user <driver-login>)" || exit 1
//	export GH_TOKEN; unset GITHUB_TOKEN
//
// The lint (actPinIssues) holds, on every act block that has a `gh` step:
//
//  1. a pin, at the top level of the act function (not inside an if, loop,
//     group or other function), of the exact shape GH_TOKEN="$(gh auth token
//     --user LOGIN)" followed by || exit, || exit N or || { ...; exit N; };
//  2. after it, still at the top level and before the first `gh` step, an
//     `export GH_TOKEN` (a separate command: `export GH_TOKEN="$(...)"` returns
//     export's status and would swallow the failure) and an `unset` of
//     GITHUB_TOKEN;
//  3. every `gh` step stands inside the act function and after those lines
//     (a `gh` step before the pin, or outside the function, runs unpinned);
//  4. nothing after the pin assigns or unsets GH_TOKEN again, or assigns
//     GITHUB_TOKEN again, so an inline `GH_TOKEN=... gh ...` cannot swap it.
//
// A `gh` step is a word whose basename is gh (quotes and backslashes removed) in
// command position: at the start of a command, after a separator, pipe, && or
// ||, after then, else, do, {, (, ! and the like, and after run, command, exec,
// env, nohup and time. The `gh auth token --user` of the pin is not a step.
// Text inside a quoted string (an echo of what would run) is not a step. A block
// with no `gh` step needs no pin.
//
// A syntactic, best-effort check, like the read check beside it. It does not
// model a `gh` run through eval, sh -c, xargs, an alias, a variable-built
// command name or a function that wraps it, a case pattern `)` that is not
// closed by esac, or an env -u GH_TOKEN that clears the pin for one step; a
// step in one of those can pass.
package main

import (
	"fmt"
	"path"
	"strings"
)

// actPinIssue is one finding of actPinIssues: the block line index and a message.
type actPinIssue struct {
	line int
	msg  string
}

// actPassWords are command words after which the next word is a command.
var actPassWords = map[string]bool{
	"if": true, "then": true, "else": true, "elif": true, "do": true, "while": true,
	"until": true, "!": true, "time": true, "{": true, "run": true, "command": true,
	"exec": true, "env": true, "nohup": true, "builtin": true,
}

// actCloserFor maps a compound-command opener to the word that closes it.
var actCloserFor = map[string]string{
	"{": "}", "if": "fi", "while": "done", "until": "done", "for": "done",
	"select": "done", "case": "esac",
}

// actIsAssign reports whether raw starts an assignment NAME=.
func actIsAssign(raw string) bool {
	i := strings.IndexByte(raw, '=')
	if i <= 0 {
		return false
	}
	for j := 0; j < i; j++ {
		c := raw[j]
		if !(c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || j > 0 && c >= '0' && c <= '9') {
			return false
		}
	}
	return true
}

// actPinIssues reads the code lines of one act block and returns the findings
// of the driver-token pin rule for the act function named name.
func actPinIssues(code []string, name string) []actPinIssue {
	toks := actTokens(code)
	var out []actPinIssue
	add := func(k int, format string, a ...any) {
		out = append(out, actPinIssue{line: toks[k].line, msg: fmt.Sprintf(format, a...)})
	}
	wordAt := func(k int, s string) bool { return k >= 0 && k < len(toks) && toks[k].op == "" && toks[k].val == s }
	// pinShape reports whether the assignment word at k opens a whole pin, and
	// the index of its last token.
	pinShape := func(k int) (int, bool) {
		if toks[k].raw != `GH_TOKEN="` || k+6 >= len(toks) || toks[k+1].op != "$(" ||
			!wordAt(k+2, "gh") || !wordAt(k+3, "auth") || !wordAt(k+4, "token") || !wordAt(k+5, "--user") {
			return 0, false
		}
		j := k + 6
		for j < len(toks) && toks[j].op != ")" && toks[j].op != "\n" {
			j++
		}
		if j == k+6 || j+3 >= len(toks) || toks[j].op != ")" || toks[j+1].raw != `"` || toks[j+2].op != "||" {
			return 0, false
		}
		b := j + 3
		endsHere := func(i int) bool { return i >= len(toks) || toks[i].sep() }
		switch {
		case wordAt(b, "exit") && endsHere(b+1):
			return b, true
		case wordAt(b, "exit") && b+1 < len(toks) && actExitStatusOK(toks[b+1]) && endsHere(b+2):
			return b + 1, true
		case wordAt(b, "{"):
			depth := 0
			for i := b; i < len(toks); i++ {
				switch {
				case toks[i].op == "" && toks[i].raw == "{":
					depth++
				case toks[i].op == "" && toks[i].raw == "}":
					depth--
					if depth == 0 {
						// the group ends ...; exit [N]; }
						ok := i >= 3 && toks[i-1].sep() &&
							(wordAt(i-2, "exit") || (actExitStatusOK(toks[i-2]) && wordAt(i-3, "exit")))
						if ok && endsHere(i+1) {
							return i, true
						}
						return 0, false
					}
				}
			}
		}
		return 0, false
	}

	var (
		stack      []string
		cmdPos     = true
		afterRedir bool
		actDepth   int // len(stack) while inside the act function's body, else 0
		bodyKnown  bool
		pinAt      = -1 // token index of the pin's assignment word
		pinEnd     = -1
		exportAt   = -1
		unsetAt    = -1
		firstGh    = -1
		ghOutside  []int
		ghInside   []int
	)
	top := func() string {
		if len(stack) == 0 {
			return ""
		}
		return stack[len(stack)-1]
	}
	inBody := func() bool { return actDepth > 0 && len(stack) >= actDepth }
	atTop := func() bool { return actDepth > 0 && len(stack) == actDepth }
	pop := func() {
		stack = stack[:len(stack)-1]
		if actDepth > 0 && len(stack) < actDepth {
			actDepth = 0
		}
	}
	argsOf := func(k int) []string {
		var a []string
		for i := k + 1; i < len(toks) && !toks[i].sep() && toks[i].op == ""; i++ {
			a = append(a, toks[i].val)
		}
		return a
	}
	has := func(a []string, s string) bool {
		for _, x := range a {
			if x == s {
				return true
			}
		}
		return false
	}

	for k := 0; k < len(toks); k++ {
		t := toks[k]
		if t.op != "" {
			switch t.op {
			case "\n", ";", "&", "&&", "||", "|":
				cmdPos, afterRedir = true, false
			case "(", "$(":
				stack = append(stack, ")")
				cmdPos = true
			case "`":
				if top() == "`" {
					pop()
					cmdPos = false
				} else {
					stack = append(stack, "`")
					cmdPos = true
				}
			case ")":
				switch top() {
				case ")":
					pop()
				case "esac":
					cmdPos = true // a case pattern's ) ahead of its commands
					continue
				}
				cmdPos = false
			default: // a redirect: its target is not a command
				afterRedir = true
			}
			continue
		}
		if afterRedir {
			afterRedir = false
			continue
		}
		isCmd := cmdPos
		cmdPos = false
		if !isCmd {
			continue
		}
		// command position
		if k+2 < len(toks) && toks[k+1].op == "(" && toks[k+2].op == ")" && t.plain {
			if t.raw == name && !bodyKnown {
				// the act function header: the body is the next group.
				bodyKnown = true
				k += 2
				j := k + 1
				for j < len(toks) && toks[j].op == "\n" {
					j++
				}
				switch {
				case j < len(toks) && toks[j].op == "(":
					stack = append(stack, ")")
					actDepth = len(stack)
					k = j
					cmdPos = true
				case j < len(toks) && toks[j].op == "" && toks[j].raw == "{" && toks[j].plain:
					stack = append(stack, "}")
					actDepth = len(stack)
					k = j
					cmdPos = true
				default:
					add(k, "the act function body is not a ( ) or { } group, so the driver-token pin cannot be located")
					return out
				}
				continue
			}
			k += 2 // another function's header
			cmdPos = true
			continue
		}
		switch {
		case actIsAssign(t.raw):
			if inBody() && pinAt >= 0 && k > pinEnd && (strings.HasPrefix(t.raw, "GH_TOKEN=") || strings.HasPrefix(t.raw, "GITHUB_TOKEN=")) {
				add(k, "act block assigns %s after the driver token is pinned; the pin must hold for every gh step", strings.SplitN(t.raw, "=", 2)[0])
			}
			if atTop() && pinAt < 0 {
				if end, ok := pinShape(k); ok {
					pinAt, pinEnd = k, end
				}
			}
			cmdPos = true // a leading assignment keeps command position
			// a pin's command substitution holds `gh auth token`, which is not a step:
			if pinAt == k {
				// skip tokens through the end of the pin
				for k < pinEnd {
					k++
				}
				cmdPos = false
			}
		case t.raw == "export" && t.plain:
			if atTop() && pinAt >= 0 && k > pinEnd && exportAt < 0 && has(argsOf(k), "GH_TOKEN") {
				exportAt = k
			}
		case t.raw == "unset" && t.plain:
			a := argsOf(k)
			if atTop() && pinAt >= 0 && k > pinEnd && unsetAt < 0 && has(a, "GITHUB_TOKEN") && !has(a, "GH_TOKEN") {
				unsetAt = k
			}
			if inBody() && pinAt >= 0 && k > pinEnd && has(a, "GH_TOKEN") {
				add(k, "act block unsets GH_TOKEN after the driver token is pinned; the pin must hold for every gh step")
			}
		case path.Base(t.val) == "gh":
			if wordAt(k+1, "auth") && wordAt(k+2, "token") && wordAt(k+3, "--user") {
				break
			}
			if firstGh < 0 {
				firstGh = k
			}
			if inBody() {
				ghInside = append(ghInside, k)
			} else {
				ghOutside = append(ghOutside, k)
			}
		case t.plain && actCloserFor[t.raw] != "":
			stack = append(stack, actCloserFor[t.raw])
			cmdPos = t.raw != "for" && t.raw != "select" && t.raw != "case"
		case t.plain && (t.raw == "}" || t.raw == "fi" || t.raw == "done" || t.raw == "esac") && top() == t.raw:
			pop()
		}
		if t.plain && actPassWords[t.raw] {
			cmdPos = true
		}
	}

	if firstGh < 0 {
		return out
	}
	need := "pin the driver's token at the top of the act function, before any gh step: GH_TOKEN=\"$(gh auth token --user <driver-login>)\" || exit 1, then export GH_TOKEN; unset GITHUB_TOKEN. An exported App token (GH_TOKEN or GITHUB_TOKEN) in the driver's shell otherwise wins over their login, and with none exported the active account runs the step"
	switch {
	case pinAt < 0:
		add(firstGh, "act block runs a gh step with no driver-token pin at the top level of the act function — %s", need)
	case exportAt < 0:
		add(firstGh, "act block pins GH_TOKEN but never runs a separate `export GH_TOKEN` at the top level of the act function after it — %s", need)
	case unsetAt < 0:
		add(firstGh, "act block pins GH_TOKEN but never runs `unset GITHUB_TOKEN` at the top level of the act function after it — %s", need)
	default:
		pinned := exportAt
		if unsetAt > pinned {
			pinned = unsetAt
		}
		for _, k := range ghInside {
			if k < pinned {
				add(k, "act block runs a gh step before the driver token is pinned (pin, export and unset must come first) — %s", need)
			}
		}
	}
	for _, k := range ghOutside {
		add(k, "act block runs a gh step outside the act function, where the driver-token pin does not reach — run it inside the function after the pin")
	}
	return out
}
