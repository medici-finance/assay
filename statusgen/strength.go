package main

// Verify-row STRENGTH rules (verify-integrity/03) and the post-base promotion of
// the unfailable-row rules.
//
// R1–R10 (rowFindings) say a row CANNOT fail as written. R11–R13 say a row is
// too weak to tell a delivered tree from an undelivered one:
//
//   - R11 trivially-green — the command exits 0 whatever the tree holds.
//   - R12 no-output-assertion — Expect is a bare `exit 0` over a command whose
//     output nobody reads.
//   - R13 table-touches-no-files — no command in the table names any path the
//     brief's `files:` line declares.
//
// The catalogue, each rule with the incident that earned it, is
// docs/verify-row-strength.md.

import (
	"fmt"
	"path"
	"regexp"
	"sort"
	"strings"
)

const (
	// ruleTriviallyGreen — R11 — the command exits 0 whatever the tree holds.
	ruleTriviallyGreen = "trivially-green"
	// ruleNoOutputAssert — R12 — `exit 0` over an output-producing command.
	ruleNoOutputAssert = "no-output-assertion"
	// ruleTableNoFiles — R13 — no command references a declared `files:` path.
	ruleTableNoFiles = "table-touches-no-files"
)

// promotedRowRules are the ten unfailable-row rules (R1–R10). A finding under
// one of them on a brief THIS branch closes — verified/done in the README, and
// not already verified/done at the merge-base — is a PROBLEM; everywhere else
// it stays the NOTICE it always was. R11–R13 are strength heuristics and stay
// NOTICEs on every brief (verify-integrity/03 promotes the ten, not the new
// three).
var promotedRowRules = map[string]bool{
	ruleERELiteralPipe: true, // R1
	ruleGrepZeroCount:  true, // R2
	ruleExitSwallowed:  true, // R3
	ruleRE2LiteralPipe: true, // R4
	ruleMetavar:        true, // R5
	ruleGoRunExit:      true, // R6
	ruleBREAlternation: true, // R7
	ruleShreddedCell:   true, // R8
	ruleMovingRef:      true, // R9
	rulePortability:    true, // R10
}

// unfailableRowChecks is the lint surface: rowFindings plus the strength rules
// over every brief, with R1–R10 promoted to PROBLEMs for a post-base closure.
// When the merge-base cannot be resolved there is no observable closure to
// gate, so every finding stays a NOTICE and the run says it is degraded — the
// same posture as the UNRUN and witness gates.
func unfailableRowChecks(root string, streams []*Stream) (problems, notices []string) {
	grandfathered, baseOK := closedAtBase(root, streams)
	problems, notices = unfailableRowAudit(streams, grandfathered, baseOK, true)
	if !baseOK {
		notices = append(notices, "unfailable-row gate is running degraded: origin/main could not be resolved, so no brief can be shown to have been closed on THIS branch and every R1–R10 finding is a NOTICE. If this is CI, fetch origin/main before the lint step")
	}
	sort.Strings(problems)
	sort.Strings(notices)
	return problems, notices
}

// rowFindingsCtx is rowFindings plus the per-row strength rules R11 and R12.
// declared are the brief's `files:` paths (R11 reads `test -e <path the brief
// adds>` as trivially green only for a declared path).
func rowFindingsCtx(cmdCell, expect string, declared []string) []rowFinding {
	out := rowFindings(cmdCell, expect)
	if hasFinding(out, ruleCmdMarkerVacuous) || hasFinding(out, ruleShreddedCell) {
		return out // already reported as a command that cannot fail / was cut
	}
	cmd := verifyCommand(cmdCell)
	if cmd == "" {
		return out
	}
	toks := tokenizeCommand(cmd)
	parts := splitSimpleCommands(toks)
	if len(parts) == 0 {
		return out
	}
	norm := normDeclaredPaths(declared)

	trivial := false
	if why := triviallyGreenWhy(toks, parts, norm); why != "" {
		trivial = true
		out = append(out, rowFinding{rule: ruleTriviallyGreen, msg: fmt.Sprintf("is trivially green: %s — the row exits 0 whatever the tree holds, so it passes on the merge-base exactly as on the head and proves nothing about the deliverable. Assert on what the change produces: a content grep with a counted Expect, a test that fails without the change, or a command whose exit status the deliverable decides", why)})
	}
	if !trivial && bareExitZeroExpect(expect) {
		if last := parts[len(parts)-1]; !quietAssertion(last.toks) {
			out = append(out, rowFinding{rule: ruleNoOutputAssert, msg: fmt.Sprintf("expects only `exit 0` from `%s`, a command that produces output nobody asserts on — a run that printed the wrong thing, or nothing, passes the same way. State what the output must show: `exit 0; output contains \"…\"`, `exit 0; N lines`, or a hash; or end the command in a quiet assertion (`grep -q …`, `test …`, `cmp -s …`)", cmdHead(last.toks))})
		}
	}
	return out
}

func hasFinding(fs []rowFinding, rule string) bool {
	for _, f := range fs {
		if f.rule == rule {
			return true
		}
	}
	return false
}

// simpleCmd is one simple command of a Verify command, and the operator that
// follows it ("" for the last).
type simpleCmd struct {
	toks []shellTok
	next string
}

// splitSimpleCommands splits a tokenized command on every operator (`|`, `||`,
// `&&`, `;`).
func splitSimpleCommands(toks []shellTok) []simpleCmd {
	var out []simpleCmd
	var cur []shellTok
	for _, t := range toks {
		if t.op {
			if len(cur) > 0 {
				out = append(out, simpleCmd{toks: cur, next: t.text})
			}
			cur = nil
			continue
		}
		cur = append(cur, t)
	}
	if len(cur) > 0 {
		out = append(out, simpleCmd{toks: cur})
	}
	return out
}

// envAssignRe is a leading `NAME=value` on a simple command.
var envAssignRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*=`)

// cmdWords drops leading env assignments so words[0] is the command name.
func cmdWords(toks []shellTok) []string {
	var w []string
	lead := true
	for _, t := range toks {
		if lead && !t.quoted && envAssignRe.MatchString(t.text) {
			continue
		}
		lead = false
		w = append(w, t.text)
	}
	return w
}

func cmdHead(toks []shellTok) string {
	if w := cmdWords(toks); len(w) > 0 {
		return strings.Join(w, " ")
	}
	return ""
}

// alwaysZeroHead reports a simple command that exits 0 without looking at
// anything: `true`, `:`, `echo …`, `printf …`, `exit`/`exit 0`.
func alwaysZeroHead(w []string) bool {
	if len(w) == 0 {
		return false
	}
	switch w[0] {
	case "true", ":", "/bin/true", "/usr/bin/true", "echo", "printf":
		return true
	case "exit":
		return len(w) == 1 || (len(w) == 2 && w[1] == "0")
	}
	return false
}

// triviallyGreenWhy returns why the command is trivially green, or "".
func triviallyGreenWhy(toks []shellTok, parts []simpleCmd, declared []declaredPath) string {
	// A trailing `|| true` / `|| echo` / `|| :` / `|| exit 0` neutralises the
	// whole command — except on a `grep -c` row, where it is R2's sanctioned fix
	// (the count is read from stdout, not the status).
	if len(parts) >= 2 && parts[len(parts)-2].next == "||" && alwaysZeroHead(cmdWords(parts[len(parts)-1].toks)) {
		counted := false
		for _, g := range grepCalls(toks) {
			if g.count {
				counted = true
			}
		}
		if !counted {
			return fmt.Sprintf("it ends in `|| %s`, which turns every failure of the command before it into a pass", cmdHead(parts[len(parts)-1].toks))
		}
	}
	// Every simple command is one that cannot fail on a delivered-or-not tree.
	var whys []string
	for _, p := range parts {
		w := cmdWords(p.toks)
		switch {
		case alwaysZeroHead(w):
			whys = append(whys, "`"+strings.Join(w, " ")+"` exits 0 unconditionally")
		case gitLogGrep(w) && p.next != "|":
			whys = append(whys, "`git log --grep` exits 0 whether or not any commit matches")
		case existsDeclared(w, declared):
			whys = append(whys, "`"+strings.Join(w, " ")+"` only checks that a file this brief adds exists, not what it holds")
		default:
			return ""
		}
	}
	return strings.Join(whys, "; ")
}

// gitLogGrep: `git [-C dir] log … --grep …`.
func gitLogGrep(w []string) bool {
	if len(w) < 2 || w[0] != "git" {
		return false
	}
	i := 1
	for i < len(w) && strings.HasPrefix(w[i], "-") {
		if w[i] == "-C" || w[i] == "-c" {
			i++
		}
		i++
	}
	if i >= len(w) || w[i] != "log" {
		return false
	}
	for _, a := range w[i+1:] {
		if a == "--grep" || strings.HasPrefix(a, "--grep=") {
			return true
		}
	}
	return false
}

// existsDeclared: `test -e|-f|-d|-s <p>` or `[ -e <p> ]` where p is a declared
// `files:` path — an existence check on the brief's own deliverable.
func existsDeclared(w []string, declared []declaredPath) bool {
	if len(w) == 0 || len(declared) == 0 {
		return false
	}
	args := w[1:]
	switch w[0] {
	case "test":
	case "[", "[[":
		if len(args) == 0 || (args[len(args)-1] != "]" && args[len(args)-1] != "]]") {
			return false
		}
		args = args[:len(args)-1]
	default:
		return false
	}
	if len(args) != 2 {
		return false
	}
	switch args[0] {
	case "-e", "-f", "-d", "-s":
	default:
		return false
	}
	t := normPathToken(args[1])
	for _, d := range declared {
		if t == d.p || (d.glob && matchGlob(d.p, t)) {
			return true
		}
	}
	return false
}

// exitZeroExpectRe is an Expect cell that says nothing but "exit 0".
var exitZeroExpectRe = regexp.MustCompile(`(?i)^(?:exit(?:s)?(?:\s+(?:code|status))?)\s*[=:]?\s*0$`)

func bareExitZeroExpect(expect string) bool {
	e := strings.TrimSpace(strings.ReplaceAll(expect, "`", ""))
	e = strings.TrimSpace(strings.TrimSuffix(e, "."))
	return exitZeroExpectRe.MatchString(e)
}

// quietAssertion: the final stage is a command whose exit status IS the
// assertion and which prints nothing to judge — `test`, `[`, `grep -q`,
// `cmp -s`, `git diff --quiet|--exit-code`. A leading `!` is allowed.
func quietAssertion(toks []shellTok) bool {
	w := cmdWords(toks)
	if len(w) > 0 && w[0] == "!" {
		w = w[1:]
	}
	if len(w) == 0 {
		return false
	}
	has := func(flags ...string) bool {
		for _, a := range w[1:] {
			for _, f := range flags {
				if a == f {
					return true
				}
			}
			if strings.HasPrefix(a, "-") && !strings.HasPrefix(a, "--") {
				for _, f := range flags {
					if len(f) == 2 && strings.Contains(a[1:], f[1:]) {
						return true
					}
				}
			}
		}
		return false
	}
	switch w[0] {
	case "test", "[", "[[":
		return true
	case "grep", "egrep", "fgrep":
		return has("-q", "--quiet", "--silent")
	case "cmp":
		return has("-s", "--silent", "--quiet")
	case "git":
		return len(w) > 1 && w[1] == "diff" && has("--quiet", "--exit-code")
	}
	return false
}

// ---------------------------------------------------------------------------
// R13 — the table touches no declared `files:` path
// ---------------------------------------------------------------------------

// declaredPath is one normalized `files:` entry. A placeholder or glob entry
// (`changelog/<slug>.md`, `x/*.go`) keeps its pattern and the literal
// directory before the first metacharacter.
type declaredPath struct {
	p    string
	glob bool
	dir  string
}

func normDeclaredPaths(raw []string) []declaredPath {
	var out []declaredPath
	for _, r := range raw {
		p := normPathToken(r)
		if p == "" {
			continue
		}
		d := declaredPath{p: p}
		if i := strings.IndexAny(p, "*?[<{"); i >= 0 {
			d.glob = true
			if j := strings.LastIndex(p[:i], "/"); j > 0 {
				d.dir = p[:j]
			}
		}
		out = append(out, d)
	}
	return out
}

// normPathToken strips quoting, a `--flag=` prefix, `./`, a sibling-repo
// `../<repo>/` prefix and a trailing `/...` or `/`.
func normPathToken(s string) string {
	s = strings.Trim(strings.TrimSpace(s), "`'\"")
	if strings.HasPrefix(s, "-") {
		_, v, ok := strings.Cut(s, "=")
		if !ok {
			return ""
		}
		s = v
	}
	for strings.HasPrefix(s, "./") {
		s = s[2:]
	}
	if strings.HasPrefix(s, "../") {
		if i := strings.Index(s[3:], "/"); i >= 0 {
			s = s[3+i+1:]
		}
	}
	if s != "..." {
		s = strings.TrimSuffix(s, "/...")
	}
	return strings.TrimSuffix(s, "/")
}

func matchGlob(pattern, name string) bool {
	ok, err := path.Match(pattern, name)
	return err == nil && ok
}

func under(a, dir string) bool { return dir != "" && dir != "." && strings.HasPrefix(a, dir+"/") }

// refsDeclared reports whether one command token references a declared path:
// the path itself, a directory above it or below it, a glob over it, the same
// file basename (a `cd dir && grep x FILE.md` row), or `./...` over Go source.
func refsDeclared(t string, d declaredPath) bool {
	if t == "" || t == "." {
		return false
	}
	if t == "..." {
		return strings.HasSuffix(d.p, ".go")
	}
	if d.glob {
		return matchGlob(d.p, t) || (d.dir != "" && (t == d.dir || under(t, d.dir) || under(d.dir, t)))
	}
	if t == d.p || under(d.p, t) || under(t, d.p) {
		return true
	}
	if strings.ContainsAny(t, "*?[") && matchGlob(t, d.p) {
		return true
	}
	b := path.Base(d.p)
	return strings.Contains(b, ".") && path.Base(t) == b
}

// tableFilesCheck is R13 over one brief. label says whether the brief carries
// a `files:` line at all. It returns the finding message ("" when clean), and
// couldNotCheck when the label is present but yields no path — never a
// finding, because an unreadable declaration is not evidence of a weak table.
func tableFilesCheck(cmds, declared []string, label bool) (msg string, couldNotCheck bool) {
	if !label {
		return "", false
	}
	norm := normDeclaredPaths(declared)
	if len(norm) == 0 {
		return "", true
	}
	if len(cmds) == 0 {
		return "", false
	}
	for _, c := range cmds {
		for _, tk := range tokenizeCommand(c) {
			if tk.op {
				continue
			}
			t := normPathToken(tk.text)
			for _, d := range norm {
				if refsDeclared(t, d) {
					return "", false
				}
			}
		}
	}
	names := make([]string, 0, len(norm))
	for _, d := range norm {
		names = append(names, d.p)
	}
	return fmt.Sprintf("no Verify command references any path the brief's `files:` line declares (%s) — the table can pass without the deliverable it lists ever being read, built or run. Point at least one row at a declared path (grep it, test its package, run its binary)", strings.Join(names, ", ")), false
}

// filesLabelPresent reports a `files:` label in the brief's `## Context`.
func filesLabelPresent(body string) bool {
	for _, l := range strings.Split(extractSectionByPrefix(body, "Context"), "\n") {
		if contextFilesLabelRe.MatchString(l) {
			return true
		}
	}
	return false
}
