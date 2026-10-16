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
//	GH_TOKEN="$(gh auth token --user "<driver-login>")" || exit 1
//	export GH_TOKEN; unset GITHUB_TOKEN
//
// The lint (actPinIssues) holds, on every act block that has a `gh` step:
//
//  0. an act function whose body is a ( ) subshell, not a { } group (a group
//     runs in the driver's shell: the pin's exit ends it, the export outlives
//     the call);
//  1. a pin, at the top level of the act function (not inside an if, loop,
//     group or other function), of the exact shape GH_TOKEN="$(gh auth token
//     --user LOGIN)", LOGIN one literal word, quoted ("<driver-login>": unquoted
//     it is a redirect and the block fails to parse) and never an expansion,
//     followed by || exit, || exit N or || { ...; exit N; }, and
//     standing as its own statement: not after ||, &&, | or |&, not under ! or a
//     wrapper such as time or env, and not followed by a pipe;
//  2. after it, still at the top level and before the first `gh` step, an
//     `export GH_TOKEN` (a separate command: `export GH_TOKEN="$(...)"` returns
//     export's status and would swallow the failure) and an `unset` of
//     GITHUB_TOKEN;
//  3. every `gh` step stands inside the act function and after those lines
//     (a `gh` step before the pin, or outside the function, runs unpinned);
//  4. nothing after the pin sets either variable again: a GH_TOKEN= or
//     GITHUB_TOKEN= assignment (also +=, and as an argument of export, declare,
//     typeset, local or readonly), an unset of GH_TOKEN, or `export -n
//     GH_TOKEN` / `declare +x GH_TOKEN`, which takes it out of the environment
//     (the builtin's name may be quoted or escaped: \unset, "export");
//  5. no `gh` step after the pin runs through env -i, env -u GH_TOKEN (also
//     -uGH_TOKEN, --unset GH_TOKEN, --unset=GH_TOKEN, --ignore-environment, a
//     short cluster such as -vi or -vu GH_TOKEN, a unique abbreviation such as
//     --ignore-env),
//     sudo or doas, which start the step without the pinned token.
//
// A `gh` step is a word whose basename is gh (quotes and backslashes removed) in
// command position: at the start of a command, after a separator, pipe, && or
// ||, after then, else, do, {, (, ! and the like, and after the words run,
// command, exec, env, nohup, builtin, time, sudo and doas, whose own options
// (-i, -p, -E, -- and, for env, command, time, exec, sudo and doas, the listed
// options that take a separate argument such as env -u NAME and sudo -u USER)
// and leading NAME=value words are skipped. command -v and command -V only look
// a name up and are not a step. The `gh auth token --user` of the pin is not a
// step. Text inside a quoted string (an echo of what would run) is not a step.
// A block with no `gh` step needs no pin.
//
// A syntactic, best-effort check, like the read check beside it.
//
// The lint refuses the shapes it models; the pin inside the function is the control. Not
// modelled (examples, not exhaustive; a `gh` step in one can run with no pin, and a token
// change in one can undo the pin): a `gh` run through `eval`, `sh -c`, `bash -c`,
// `xargs`, `nice`, `timeout`, `stdbuf`, `exec -c` or a wrapper command the lint does not
// name; a wrapper option that takes a separate argument and is not in the lint's list
// (`env -S`, say); an alias; a command name built from an expansion; a function that
// wraps `gh`; a `gh` step inside a process substitution (`<( )`, `>( )`), after a leading
// redirect such as `2>&1`, or in a `case` arm whose pattern opens with `(`; a `case`
// pattern `)` that no `esac` closes; an `export` or `unset` of the token run under `||`,
// `&` or a pipe; a change of either token variable by `read`, `printf -v`, `for`,
// `select`, `let`, `mapfile`, `getopts`, `source`, `.`, a nameref, `eval`, or zsh
// `${GH_TOKEN::=…}`. The pin governs `gh` only: a `git push` uses git's credential
// helper, and `GH_HOST` and the enterprise-token variables are left alone, which matters
// only against a GitHub Enterprise Server host.
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
	"exec": true, "env": true, "nohup": true, "builtin": true, "sudo": true, "doas": true,
}

// actWrappers are the pass words that run another command and take options of
// their own; the options are skipped to reach it. The map holds the options of
// each that take a separate argument (env's are read by actEnvOption).
var actWrappers = map[string][]string{
	"env": nil, "command": nil, "time": {"-f", "-o"},
	"exec": {"-a"}, "nohup": nil, "builtin": nil,
	"sudo": {"-u", "-g", "-h", "-p", "-r", "-t", "-C", "-D", "-R", "-T", "-U"},
	"doas": {"-u", "-C"},
}

// actDeclarers are the builtins that set a shell variable from NAME=value words.
var actDeclarers = map[string]bool{
	"export": true, "declare": true, "typeset": true, "local": true, "readonly": true,
}

// actTokenVar returns GH_TOKEN or GITHUB_TOKEN when w assigns it (NAME= or NAME+=), else "".
func actTokenVar(w string) string {
	for _, n := range []string{"GH_TOKEN", "GITHUB_TOKEN"} {
		if strings.HasPrefix(w, n+"=") || strings.HasPrefix(w, n+"+=") {
			return n
		}
	}
	return ""
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
	toks := actPipeAmp(actTokens(code))
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
		// the login is one literal word: an unquoted <driver-login> is a redirect
		// (the block fails to parse), and an expansion can name any login.
		if j != k+7 || toks[k+6].op != "" || toks[k+6].val == "" {
			return 0, false
		}
		if strings.ContainsAny(toks[k+6].raw, "$`") {
			add(k, "the pin's login is an expansion; name the driver's login as one literal word, \"<driver-login>\" or the resolved login, so the pin cannot name another account")
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
		braceBody  int    // token index + 1 of a { } act function body, else 0
		bodyOpen   = -1   // token index of the act function body's opener
		wrap       string // the wrapper word just seen, whose options are skipped
		dropAt     = -1   // token index of an env or sudo that drops the pinned token
		dropWhy    string
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

	// pinAlone reports whether the command word at k stands as its own
	// statement: past newlines and NAME=value words, the token before it is
	// the body's opener, the start, a ; or & or a finished command, and not
	// ||, && or | (the pin may never run, or runs in a pipe's subshell), nor
	// ! or a wrapper word (time, env, ...) that runs it as an argument.
	pinAlone := func(k int) bool {
		p := k - 1
		for p >= 0 && p != bodyOpen && (toks[p].op == "\n" || toks[p].op == "" && actIsAssign(toks[p].raw)) {
			p--
		}
		if p < 0 || p == bodyOpen {
			return true
		}
		switch toks[p].op {
		case "|", "||", "&&":
			return false
		case "":
			return !(toks[p].plain && actPassWords[toks[p].raw])
		}
		return true
	}

	for k := 0; k < len(toks); k++ {
		t := toks[k]
		if t.op != "" {
			wrap = ""
			switch t.op {
			case "\n", ";", "&", "&&", "||", "|":
				cmdPos, afterRedir = true, false
				dropAt = -1
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
		w := wrap
		wrap = ""
		if !isCmd {
			continue
		}
		if w != "" && strings.HasPrefix(t.val, "-") {
			// an option of the wrapper before it: skip it (and its argument) to
			// reach the command it runs. command -v and -V only look a name up.
			if w == "command" && (t.val == "-v" || t.val == "-V") {
				continue
			}
			next := ""
			if k+1 < len(toks) && toks[k+1].op == "" {
				next = toks[k+1].val
			}
			takes := false
			if w == "env" {
				var why string
				why, takes = actEnvOption(t.val, next)
				if why != "" && inBody() && pinAt >= 0 && k > pinEnd && dropAt < 0 {
					dropAt, dropWhy = k, why
				}
			} else {
				for _, o := range actWrappers[w] {
					if t.val == o {
						takes = true
					}
				}
			}
			if takes && k+1 < len(toks) && toks[k+1].op == "" {
				k++
			}
			cmdPos, wrap = true, w
			continue
		}
		if actDeclarers[t.val] && inBody() && pinAt >= 0 && k > pinEnd {
			named := false
			unexport := false
			for i := k + 1; i < len(toks); i++ {
				switch toks[i].op {
				case "\n", ";", "&", "&&", "||", "|":
					i = len(toks)
					continue
				case "":
				default:
					continue
				}
				v := toks[i].val
				if n := actTokenVar(v); n != "" {
					add(i, "act block assigns %s after the driver token is pinned; the pin must hold for every gh step", n)
				}
				if v == "GH_TOKEN" {
					named = true
				}
				if t.val == "export" && v == "-n" || t.val != "export" && strings.HasPrefix(v, "+") && strings.Contains(v, "x") {
					unexport = true
				}
			}
			if named && unexport {
				add(k, "act block removes the export of GH_TOKEN after the driver token is pinned; the pin must hold for every gh step")
			}
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
					k, bodyOpen = j, j
					cmdPos = true
				case j < len(toks) && toks[j].op == "" && toks[j].raw == "{" && toks[j].plain:
					stack = append(stack, "}")
					actDepth = len(stack)
					braceBody = j + 1
					k, bodyOpen = j, j
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
		case actIsAssign(t.raw) || actTokenVar(t.raw) != "":
			if n := actTokenVar(t.raw); n != "" && inBody() && pinAt >= 0 && k > pinEnd {
				add(k, "act block assigns %s after the driver token is pinned; the pin must hold for every gh step", n)
			}
			if atTop() && pinAt < 0 {
				if end, ok := pinShape(k); ok {
					if pinAlone(k) {
						pinAt, pinEnd = k, end
					} else {
						add(k, "the driver-token pin must be its own statement: not after ||, && or |, not under ! or a wrapper such as time or env")
					}
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
		case t.val == "export":
			if atTop() && pinAt >= 0 && k > pinEnd && exportAt < 0 && has(argsOf(k), "GH_TOKEN") {
				exportAt = k
			}
		case t.val == "unset":
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
			if dropAt >= 0 && inBody() && pinAt >= 0 && k > pinEnd {
				add(k, "act block runs a gh step the pinned token does not reach: %s", dropWhy)
				dropAt = -1
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
			if _, ok := actWrappers[t.raw]; ok {
				wrap = t.raw
			}
			if (t.raw == "sudo" || t.raw == "doas") && inBody() && pinAt >= 0 && k > pinEnd && dropAt < 0 {
				dropAt, dropWhy = k, t.raw+" resets the environment and runs it as another user"
			}
		}
	}

	if firstGh < 0 {
		return out
	}
	if braceBody > 0 {
		add(braceBody-1, "the act function body is a { } group, which runs in the driver's own shell: the pin's exit would end that shell and the exported token would outlive the call; write the body as a ( ) subshell, driver_act_<id>() ( ... )")
	}
	need := "pin the driver's token at the top of the act function, before any gh step: GH_TOKEN=\"$(gh auth token --user \"<driver-login>\")\" || exit 1, then export GH_TOKEN; unset GITHUB_TOKEN. An exported App token (GH_TOKEN or GITHUB_TOKEN) in the driver's shell otherwise wins over their login, and with none exported the active account runs the step"
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

// actPipeAmp folds the |& pipe, which the tokenizer reads as | then &, into one
// | token, so it is a pipe and not a pipe followed by a background &.
func actPipeAmp(toks []actTok) []actTok {
	out := make([]actTok, 0, len(toks))
	for i := 0; i < len(toks); i++ {
		out = append(out, toks[i])
		if toks[i].op == "|" && i+1 < len(toks) && toks[i+1].op == "&" {
			i++
		}
	}
	return out
}

// actEnvOption reads one option word of env (val) and the word after it
// (next). why is non-empty when the option starts the command without the
// pinned GH_TOKEN: it empties the environment (-i, -, --ignore-environment,
// and a short cluster holding i, such as -vi), or unsets GH_TOKEN (-u, --unset,
// and a short cluster ending in u, such as -vu). takesNext reports that the
// option takes the next word as its argument. GNU long options may be
// abbreviated to a unique prefix.
func actEnvOption(val, next string) (why string, takesNext bool) {
	empty := "env " + val + " starts it with an empty environment"
	unset := func(name string) string {
		if name == "GH_TOKEN" {
			return "env " + val + " unsets GH_TOKEN for it"
		}
		return ""
	}
	switch {
	case val == "-":
		return empty, false
	case strings.HasPrefix(val, "--"):
		opt, arg, hasArg := strings.Cut(val[2:], "=")
		switch {
		case len(opt) >= len("ignore-e") && strings.HasPrefix("ignore-environment", opt):
			return empty, false
		case opt != "" && strings.HasPrefix("unset", opt):
			if hasArg {
				return unset(arg), false
			}
			return unset(next), true
		case opt != "" && strings.HasPrefix("chdir", opt):
			return "", !hasArg
		}
	case strings.HasPrefix(val, "-"):
		s := val[1:]
		for i := 0; i < len(s); i++ {
			switch s[i] {
			case 'i':
				return empty, false
			case 'u':
				if rest := s[i+1:]; rest != "" {
					return unset(rest), false
				}
				return unset(next), true
			case 'C':
				return "", s[i+1:] == ""
			}
		}
	}
	return "", false
}
