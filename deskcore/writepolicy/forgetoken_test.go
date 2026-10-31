package writepolicy

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// regression: #4821
func TestAmbientForgeTokenClassify(t *testing.T) {
	read := func(cmd string) bool {
		return Classify(Request{Tool: "Bash", Target: cmd}, LexicalContext{}).ReadsAmbientForgeToken()
	}
	deep := func(inner string) string { return strings.Repeat("eval ", 10) + inner }
	for _, tc := range []struct {
		cmd  string
		want bool
	}{
		{"gh auth token", true},
		{"gh 2>/dev/null auth token", true},
		{">out.txt gh auth token", true},
		{"gh auth token>/dev/null", true},
		{"gh auth status --show-token=true", true},
		{"gh auth status --show-token=TRUE", true},
		// A trigger word anywhere refuses: no shape holding one is allowed.
		{"gh auth status --show-token=false", true},
		{"gh auth status -a", true},
		{"gh auth login --with-token <<< token", true},
		{"gh auth refresh", true},
		{"gh auth", true},
		{"echo \"it's gh auth token\"", true},
		{"echo 'unterminated; gh auth token", true},
		{deep("gh auth token"), true},
		// No trigger word: allowed, however it is nested or quoted.
		{"gh", false},
		{"", false},
		{"echo 'unterminated; ls", false},
		{deep("ls"), false},
	} {
		if got := read(tc.cmd); got != tc.want {
			t.Errorf("ReadsAmbientForgeToken(%q) = %v, want %v", tc.cmd, got, tc.want)
		}
	}
}

// forgeWithin runs forgeTokenRead under a deadline: a hook timeout lets the
// call through, so a match that does not finish in time is a fail-open.
func forgeWithin(t *testing.T, cmd string, d time.Duration) bool {
	t.Helper()
	done := make(chan bool, 1)
	go func() { done <- forgeTokenRead(cmd) }()
	select {
	case hit := <-done:
		return hit
	case <-time.After(d):
		t.Fatalf("%d-byte command: no decision within %v", len(cmd), d)
	}
	return false
}

// regression: #4821
func TestForgeWrapperChainLinear(t *testing.T) {
	// A long wrapper chain is decided in time: the match is one pass per
	// trigger word, so the read after the chain is seen.
	for _, k := range []int{26, 400, 4000} {
		for _, tc := range []struct {
			cmd  string
			want bool
		}{
			{strings.Repeat("command nice ", k/2) + "true; gh auth token", true},
			{strings.Repeat("env ", k) + "true", false},
			{strings.Repeat("timeout 5 ", k/2) + "gh pr view 1", false},
		} {
			if got := forgeWithin(t, tc.cmd, 2*time.Second); got != tc.want {
				t.Fatalf("k=%d %q…: read = %v, want %v", k, tc.cmd[:24], got, tc.want)
			}
		}
	}
}

// regression: #4821
func TestForgeANSICDecode(t *testing.T) {
	// Escapes the shells, echo -e and printf decode are read as the byte they
	// stand for, inside or outside a `$'…'` span.
	for _, tc := range []struct {
		cmd  string
		want bool
	}{
		{`gh auth $'tok\x65n'`, true},
		{`$'\147h' $'\x61uth' $'to\u006ben'`, true},
		{`gh $'\U00000061uth' status`, true},
		{`printf '\164\157\153\145\156'`, true},
		{`echo $'it\'s'; gh auth token`, true},
		{`echo $'it\'s gh auth token'`, true},
		{`echo $'tab\there gh auth token'`, true},
		{`echo $'tab\there'; gh auth token`, true},
		{`echo $'it\'s'; ls`, false},
		{`printf '\q\x\n'`, false},
	} {
		if got := forgeTokenRead(tc.cmd); got != tc.want {
			t.Errorf("forgeTokenRead(%q) = %v, want %v", tc.cmd, got, tc.want)
		}
	}
}

// regression: #4821
func TestForgeScanWorkBounded(t *testing.T) {
	// Shapes that once made a shell-model scan slow or undecided: ANSI-C spans
	// holding an escaped quote, many heredoc operators on one line, many quoted
	// spans, substitutions holding heredocs, a long wrapper prefix and deep
	// nesting. Each is decided within a deadline, refused with a trigger word
	// in any spelling and allowed without one.
	const n = 600
	prefix := "sudo " + strings.Repeat("gh pr x ", 2500)
	nest := func(inner string) string { return strings.Repeat("eval ", 10) + inner }
	for _, tc := range []struct {
		name, cmd string
		want      bool
	}{
		{"ansi-c-quote", strings.Repeat(`printf $'q\'z' `, n) + "; gh auth token", true},
		{"ansi-c-quote-allow", strings.Repeat(`printf $'q\'z' `, n), false},
		{"heredoc-line", "sort " + strings.Repeat("<<E ", 8*n) + "; gh auth token\n" + strings.Repeat("E\n", 8*n), true},
		{"heredoc-line-allow", "sort " + strings.Repeat("<<E ", 8*n) + "\n" + strings.Repeat("E\n", 8*n) + "echo done", false},
		{"quoted-spans", strings.Repeat(`x "b" 'c' `, 4*n) + "; gh auth token", true},
		{"quoted-spans-allow", strings.Repeat(`x "b" 'c' `, 4*n), false},
		{"subst-heredoc", strings.Repeat("echo \"$(cat <<'M'\n1) a\nM\n)\"\n", n) + "gh auth token", true},
		{"subst-heredoc-allow", strings.Repeat("echo \"$(cat <<'M'\n1) a\nM\n)\"\n", n), false},
		{"body-subst", "cat <<B\n" + strings.Repeat("$(cat <<'M'\n1) a\nM\n)\n", n) + "B\ngh auth token", true},
		{"body-subst-allow", "cat <<B\n" + strings.Repeat("$(cat <<'M'\n1) a\nM\n)\n", n) + "B", false},
		{"prefix-ansi-c", prefix + `; gh auth $'\x74oken'`, true},
		{"prefix-split", prefix + "; gh auth to''ken", true},
		{"prefix-allow", prefix + "; echo ok", false},
		{"nest-split", nest("gh auth t'o'ken"), true},
		{"nest-ansi-c", nest(`gh $'\x61uth' token`), true},
		{"nest-allow", nest("ls"), false},
		{"megabyte", strings.Repeat("x ", 1<<19) + "; gh auth token", true},
		{"megabyte-allow", strings.Repeat("x ", 1<<19), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := forgeWithin(t, tc.cmd, 2*time.Second); got != tc.want {
				t.Errorf("read = %v, want %v", got, tc.want)
			}
		})
	}
}

// regression: #4821
func TestForgeArithHeredoc(t *testing.T) {
	// A `((` is arithmetic or nested subshells as bash decides by where the
	// parens close. The guard models neither: the script is refused iff it
	// holds a trigger word, even as data (accepted false refusals).
	for _, tc := range []struct {
		cmd  string
		want bool
	}{
		// Nested subshells, then a data heredoc: the read after it is judged.
		{"((true); true)\ncat > notes.md <<'E'\nit's here\nE\ngh auth token\necho 'done'", true},
		{"((true); true)\ncat > notes.md <<'E'\nit's gh auth token\nE\necho 'done'", true},
		{"((true); true)\ncat > notes.md <<'E'\nit's here\nE\necho 'done'", false},
		{"(('n'); true)\ncat > notes.md <<'E'\nit's here\nE\ngh auth token\necho 'done'", true},
		{"(('x'<<E))\ngh auth token\nE", true},
		{"((x<<E))\ngh auth token\nE", true},
		{"((\"x\"<<E))\ngh auth token\nE", true},
		{"((n = 1<<2)) && cat > notes.md <<'E'\nit's gh auth token\nE", true},
		{"for ((i=0; i<2; i++)); do echo $i; done\ncat > n.md <<'E'\ngh auth token\nE", true},
		{"for ((i=0; i<${#a[@]}; i++)); do :; done\ncat > n.md <<'E'\nit's\nE", false},
		{"for ((i=0; i<${#a[@]}; i++)); do :; done\ncat > n.md <<'E'\nit's\nE\ngh auth token\necho 'x'", true},
		{"if (( $# > 0 )); then :; fi\ncat > n.md <<'E'\nit's\nE", false},
		{"(( \"$n\" > 1 )) && cat > n.md <<'E'\nit's\nE", false},
	} {
		if got := forgeTokenRead(tc.cmd); got != tc.want {
			t.Errorf("forgeTokenRead(%q) = %v, want %v", tc.cmd, got, tc.want)
		}
	}
}

// regression: #4821
func TestForgeShiftSiblings(t *testing.T) {
	// A `<<` a shell model could read as a heredoc (or not) where the shell
	// reads it the other way. Each refused shape runs the read in bash or zsh;
	// each allowed one holds no trigger word. A trigger word in data is
	// refused too.
	for _, tc := range []struct {
		cmd  string
		want bool
	}{
		{"x=$(( \"$y\" <<'E'\n$(gh auth token)\nE\n))", true},
		{"x=$((1<<2)); cat > n.md <<'E'\nit's gh auth token\nE", true},
		{"x=$((1<<2)); cat > n.md <<'E'\nit's here\nE", false},
		{"echo $[1<<E]\ngh auth token\nE]", true},
		{"echo ${x:-<<E}\ngh auth token\nE}", true},
		{"echo ${x:-y}; cat > n.md <<'E'\ngh auth token\nE", true},
		{"echo ${x:-y}; cat > n.md <<'E'\nit's\nE", false},
		{"a[x<<E]=1\ngh auth token\nE]=1", true},
		{"a[1]=2; cat > n.md <<'E'\ngh auth token\nE", true},
		{"a[1]=2; cat > n.md <<'E'\nit's\nE", false},
		// A heredoc in a substitution whose body holds the closing paren.
		{"x=$(cat <<'E'\n) it's\nE\n); gh auth token; echo 'ok'", true},
		{"x=$(cat <<'E'\nit's\nE\n); echo \"$x\"", false},
		// `<((` is a process substitution holding a subshell, not arithmetic.
		{"cat <((cat <<'E'\nit's\nE\n) ); gh auth token; echo 'ok'", true},
		// A substitution ends past the heredoc bodies inside it (bash 4+,
		// zsh) wherever it is: unquoted, double-quoted or in a heredoc body.
		{"printf '%s\\n' \"$(tr a-z A-Z <<'END'\nsee (a) and b)\nEND\ngh auth token\n)\"", true},
		{"wc -l <<END\n$(sort <<'TXT'\nb)\nTXT\ngh auth token\n)\nEND", true},
		{"git commit -m \"$(cat <<'MSG'\nwriteguard: tidy\n\nSteps: 1) lex 2) judge\nMSG\n)\"", false},
		{"wc -l <<END\n$(sort <<'TXT'\nb) a\nTXT\n)\nEND", false},
		{"n=$(wc -l <<'TXT'\nstep 2) done\nTXT\n); cat > n.md <<'E'\nit's\nE", false},
		// bash 3.2 ends it at the first `)` instead: that reading is judged too.
		{"n=$(wc -l <<'TXT'\n)\ngh auth token\nTXT\n)", true},
		{"echo \"$(cat <<'T'\n)\"; gh auth token; echo \"\nT\n)\"", true},
		// A delimiter line is the word exactly (leading tabs removed for `<<-`).
		{"tee n.md <<'DONE' >/dev/null\nDONE \nwe're here\nDONE\ngh auth token\nprintf 'ok'", true},
		{"cat <<END\n\tEND\nit's\nEND\ngh auth token\necho 'x'", true},
		{"cat <<-END\n\tbody\n\tEND\ngh auth token", true},
		{"cat > n.md <<'END'\n END\nEND\necho ok", false},
		// A quoted or escaped close does not end a `${…}` or `name[…]` span.
		{": ${opt:-\"a}b\"<<T}\ngh auth token\nT}", true},
		{"printf %s ${p%\\}<<T}\ngh auth token\nT}", true},
		{"m[\"k]\"<<T]=v\ngh auth token\nT]=v", true},
		{"echo ${v:-\"}\"}; cat > n.md <<'E'\nit's\nE", false},
		{"echo ${D:-$(pwd)}; cat > n.md <<'E'\nit's\nE", false},
		// A substitution in a delimiter word.
		{"cat <<$(echo E)\nbody\n$(echo E)\necho ok", false},
		{"cat <<$(echo E)\nbody\n$(echo E)\ngh auth token", true},
	} {
		if got := forgeTokenRead(tc.cmd); got != tc.want {
			t.Errorf("forgeTokenRead(%q) = %v, want %v", tc.cmd, got, tc.want)
		}
	}
}

// forgeTriggers spell the read in the ways the match must see through:
// plain, quoted, escaped, ANSI-C decoded, behind a computed command word,
// and the other token prints. "@" in a construct row stands for each. The
// TestUnmodeled* tables keep one row set per shell construct a lexer could
// misplace; the guard no longer models any of them.
var forgeTriggers = []string{
	"gh auth token", `gh a'u'th to\ken`, `gh $'\x61uth' token`, `$c auth "token"`,
	`gh "$s" status -t`, "gh config get oauth_token", "gh auth git-credential",
}

// forgeUnmodeled asserts each row is refused with every trigger in it, and
// allowed with `echo ok` there: the construct alone is not refused.
func forgeUnmodeled(t *testing.T, rows []string) {
	t.Helper()
	for _, row := range rows {
		for _, trig := range forgeTriggers {
			if cmd := strings.ReplaceAll(row, "@", trig); !forgeTokenRead(cmd) {
				t.Errorf("%q: the trigger in the construct must refuse", cmd)
			}
		}
		if cmd := strings.ReplaceAll(row, "@", "echo ok"); forgeTokenRead(cmd) {
			t.Errorf("%q: no trigger, so the construct must stay allowed", cmd)
		}
	}
}

// regression: #4821
func TestUnmodeledCmdSubst(t *testing.T) {
	forgeUnmodeled(t, []string{
		"x=$(printf '%s' \"(\"; @)", "echo \"$(case a in a) @ ;; esac)\"",
		"v=$(echo $(@))", "echo \"$(printf \")\"; @)\"", "y=$(echo $'\\''; @)",
		"echo $(# c\n@\n)", "echo $(printf %s \\); @)", "echo \"$(cat <<'M'\nm\nM\n)\"; @",
	})
}

// regression: #4821
func TestUnmodeledBacktick(t *testing.T) {
	forgeUnmodeled(t, []string{"echo `@`", "x=\"`date`\"; @", "echo `echo \\`@\\``"})
}

// regression: #4821
func TestUnmodeledHeredocBody(t *testing.T) {
	forgeUnmodeled(t, []string{
		"cat <<E\n`date`\nE\n@", "cat <<E\n$(cat <<'F'\nF\n)\nE\n@",
		"cat <<E\nnote \\" + "\nE\nit's\nE\n@\necho '", "cat <<E\n@", "cat <<$(echo E)\nx\n$(echo E)\n@",
		"cat <<E\n${v:-\"q\"}\nE\n@", "cat <<E\n$a[1]\nE\n@",
	})
}

// regression: #4821
func TestUnmodeledArith(t *testing.T) {
	forgeUnmodeled(t, []string{"x=$((1<<2)); @", "echo $(( (1) ))\n@", "echo $[1<<E]\n@\nE]"})
}

// regression: #4821
func TestUnmodeledDoubleParen(t *testing.T) {
	forgeUnmodeled(t, []string{
		"((x<<E))\n@\nE", "((true); true)\ncat > n.md <<'E'\nit's\nE\n@",
		"for ((i=0; i<2; i++)); do @; done",
	})
}

// regression: #4821
func TestUnmodeledProcSubst(t *testing.T) {
	forgeUnmodeled(t, []string{
		"diff <(sort <<'L'\nb) x\nL\n) /dev/null; @", "tee >(cat) </dev/null; @", "cat <(@)",
	})
}

// regression: #4821
func TestUnmodeledUnclosed(t *testing.T) {
	forgeUnmodeled(t, []string{
		"echo 'open; @", "echo \"open; @", "echo $'open; @", "echo ${open; @",
		"echo $(open; @", "echo `open; @", "cat <<'E\n@",
	})
}

// regression: #4821
func TestUnmodeledQuoteForms(t *testing.T) {
	forgeUnmodeled(t, []string{
		"echo $'it\\'s'; @", "printf %s \"${v:+\"'\"}\"; @; printf \"'\"", "echo \"$a[1 2]\"; @",
		"echo ${D:-$(pwd)}; @", "arr=(a b); @", "f() { :; }; @", "a[x y]=1; @",
	})
}

// regression: #4821
func TestForgeReviewShapes(t *testing.T) {
	// Neutral shapes of the classes earlier reviews found (each one runs the
	// read in bash 5, bash 3.2 or zsh): a span end a lexer once placed where
	// the shell does not. Each holds a trigger word, so it is refused.
	for name, cmd := range map[string]string{
		"ansi-c-in-dq-subst": "out=\"$(printf '%s\\n' $'k\\')' ; gh auth token ; printf )\"",
		"procsubst-heredoc":  "/bin/bash -c \"wc -l <(tac <<Q\nx)\ngh auth token\nQ\n)\"",
		"procsubst-out-twin": "/bin/bash -c \"tee >(cat <<Q\nx)\ngh auth token\nQ\n) </dev/null\"",
		"nested-dq-paren":    "n=$( cat <<< \"$(printf '%s' \")\" ; gh auth token)\" )",
		"case-in-dq-subst":   "printf '%s\\n' \"$( case q in q) printf \"b\" ; gh auth token ;; esac )\"",
		"quote-in-brace":     "printf '%s' \"${u-\"'\"}\" ; gh auth token ; printf \"'\"",
		"ansi-c-in-subst":    "v=$( printf %s $'\\'' ; printf ')' ) ; gh auth token ; printf 'v'",
		"heredoc-line-join":  "cat <<E\nnote \\" + "\nE\nit's\nE\ngh auth token\necho '",
		"zsh-equals-path":    "=gh auth token",
	} {
		if !forgeTokenRead(cmd) {
			t.Errorf("%s: %q must be refused", name, cmd)
		}
	}
}

// regression: #4821
func TestForgeIdiomOutcomes(t *testing.T) {
	// A message, body or comment naming a trigger word is refused however it
	// is passed on the command line; -F / --body-file is the way through.
	for cmd, want := range map[string]bool{
		"git commit -m \"$(cat <<'MSG'\nfix: tidy the lexer\nMSG\n)\"":         false,
		"git commit -m \"$(cat <<'MSG'\nfix: rotate the token cache\nMSG\n)\"": true,
		"git commit -F msg.txt": false,
		"git commit -m \"$(cat <<'T'\nit's 1)\nT\n)\"; y=$(cat <<'U'\nok (a\nU\n)":       false,
		"git commit -m \"$(cat <<'T'\nit's 1)\nT\n)\"; y=$(cat <<'U'\nok (a token\nU\n)": true,
		"git commit -m \"fix: rotate the token cache\"":                                  true,
		"gh pr edit 1 --body \"see the auth flow\"":                                      true,
		"gh pr create --title t --body \"$(cat <<'EOF'\nSummary\nEOF\n)\"":               false,
		"gh pr create --title t --body \"$(cat <<'EOF'\nthe auth flow\nEOF\n)\"":         true,
		"gh pr create --title t --body-file body.md":                                     false,
		"gh pr edit 1 --body \"$(cat body.md)\"":                                         false,
		"gh pr comment 1 --body \"$(printf 'see token docs')\"":                          true,
	} {
		if got := forgeTokenRead(cmd); got != want {
			t.Errorf("forgeTokenRead(%q) = %v, want %v", cmd, got, want)
		}
	}
}

// forgeRefuseAll asserts every command is refused.
func forgeRefuseAll(t *testing.T, cmds []string) {
	t.Helper()
	for _, cmd := range cmds {
		if !forgeWithin(t, cmd, 2*time.Second) {
			t.Errorf("%q: a trigger word in the text must refuse", cmd)
		}
	}
}

// regression: #4821
func TestForgeWordSpan(t *testing.T) {
	// A word the shell keeps whole (a carriage return or a leading `}` before
	// `#`) must not hide the rest of the line as a comment.
	forgeRefuseAll(t, []string{
		"wc -c pad\r#;gh auth token -h example.com",
		"mkdir -p }#a && gh auth token --hostname example.com",
		"cat -- }#notes 2>/dev/null || gh auth token -u bot",
	})
}

// regression: #4821
func TestForgeCommandPrefix(t *testing.T) {
	// Command-position prefixes outside any closed set: an append
	// assignment, a zsh precommand, and scheduling or clock tools.
	forgeRefuseAll(t, []string{
		"MANPATH+=:/usr/share/man gh auth token -h example.com",
		"- gh auth token -h example.com",
		"chrt --other 0 gh auth token -h example.com",
		"numactl --localalloc gh auth token",
		"faketime -f +1d gh auth token -u bot",
	})
}

// regression: #4821
func TestForgeDynamicArity(t *testing.T) {
	// One brace or computed word standing for several components, and a
	// substituted script handed to eval, a shell's -c or a here-string.
	forgeRefuseAll(t, []string{
		"gh {auth,} {token,} -h example.com",
		"gh $(echo auth; echo token)",
		"IFS=,; v=auth,token; gh $v -h example.com",
		"words='auth token'; nice gh $words",
		`x=$(printf 'gh auth token -h %s' example.com); eval "$x"`,
		`dash -c "$(printf '%s' 'gh auth token -u bot')"`,
		`zsh -s <<< "$(printf '%s\n' 'gh auth token')"`,
	})
}

// regression: #4821
func TestForgeClobberOperator(t *testing.T) {
	// `>|` (with or without a descriptor) between the read's words.
	forgeRefuseAll(t, []string{
		"env gh auth 5>| fd5.txt token --user bot",
		"gh 2>| /dev/null auth token -h example.com",
		"nice -n 1 gh auth 1>| /tmp/copy.txt token -u bot",
	})
}

// regression: #4821
func TestForgeStringToCommand(t *testing.T) {
	// A computed shell word, and builtins that run a string.
	forgeRefuseAll(t, []string{
		`"${BASH:-/bin/bash}" -c 'gh auth token -u bot'`,
		`$ZSH_NAME -c 'gh auth token -u x'`,
		`cat <<< 'gh auth token -h example.com' | "${SHELL}"`,
		"complete -C 'gh auth token -h example.com' mycmd",
		`bind -x '"\C-t": gh auth token'`,
	})
}

// regression: #4821
func TestForgePassThrough(t *testing.T) {
	// Arguments or scripts that reach gh or a shell through another command,
	// and a read spelled with literal words held in variables or an alias.
	forgeRefuseAll(t, []string{
		"sort -u <<< 'auth token' | xargs -r nice gh",
		"printf 'gh auth token -h example.com\\n' | tee -a log.txt | bash -s",
		"c=gh s=auth; $c $s token",
		"read -r p q <<< 'auth token'; gh $p $q",
		"alias gg='gh auth'; gg token -h example.com",
		"find /tmp -name none -prune -o -exec gh auth token {} +",
	})
}

// regression: #4821
func TestForgeWordBreak(t *testing.T) {
	// A backslash sequence a shell, echo -e or printf may read as a word
	// boundary: a line continuation a shell drops at the end of its input
	// (the last body line of a quoted heredoc fed to it), and an escape that
	// decodes to a blank or a newline, at the end or the start of a word.
	forgeRefuseAll(t, []string{
		"dash <<'END'\nexport PAGER=cat\ngh config get -h example.com oauth_token\\" + "\nEND",
		"zsh -s <<\"DONE\"\ngh -R example/repo config get oauth_token\\" + "\nDONE",
		"tee -a run.log <<'Q' | bash -s\ngh config get oauth_token\\" + "\nQ",
		`printf 'gh config get -h example.com oauth_token\n' | sh`,
		`printf 'gh\tauth\ttoken -u bot' | bash -s`,
		`printf 'gh\x20auth\x20token' | dash`,
		`printf 'gh\040auth\040token\n' > run.sh; sh run.sh`,
		`bash -c $'gh config get oauth_token\ntrue'`,
		`bash -c $'gh\cIauth\cItoken'`,
		`zsh -c $'gh\` + `u0020auth\` + `u0020token -h example.com'`,
		`echo -e 'gh config get oauth_token\r' | bash`,
		`printf 'gh config get oauth_token\x0a' | sh`,
		`bash -c $'gh\cJstatus'`,
	})
}

// regression: #4821
func TestForgeMentionsRefused(t *testing.T) {
	// The accepted false refusals: a trigger word as data (prose, a search
	// pattern, a comment, a heredoc to a file, a path, a field name).
	forgeRefuseAll(t, []string{
		"echo 'gh auth token'",
		"grep -n 'gh auth token' notes.md",
		"rg -n token src/",
		"# gh auth token\necho ok",
		"cat > notes.md <<'EOF'\nthe auth flow\nEOF",
		"mkdir -p auth",
		"gh auth status",
		"gh pr view token",
	})
}

// regression: #4821
func TestForgeNoTriggerAllowed(t *testing.T) {
	// The allow side: text holding no trigger word, including the sanctioned
	// mint, a minted token passed to one command, and the file-based ways
	// through for prose.
	for _, cmd := range []string{
		"gh pr view 12",
		"gh api user --jq .login",
		`GH_TOKEN=$(cat "$T") gh pr view 1`,
		"desktoken --no-rotate worker --repo example/repo",
		"deskclaim-ref release k --token-file f",
		"git status --short",
		"grep -n -f patterns.txt notes.md",
		"git commit -F msg.txt",
		"gh pr comment 1 --body-file body.md",
		"go test ./writepolicy/ -run TestAmbientToken",
		"command -v gh",
	} {
		if forgeTokenRead(cmd) {
			t.Errorf("%q: no trigger word, so it must stay allowed", cmd)
		}
	}
}

// regression: #5046
func TestForgeStatusComputedForms(t *testing.T) {
	// A computed word after gh could be anything, with or without a print flag.
	forgeRefuseAll(t, []string{
		`gh "$s" status "$f"`,
		`gh "$s" status -aat`,
		`gh "$s" status -aaat`,
		`gh "$s" status -t`,
		`gh "$s" status -at`,
		`gh "$s" status -ta`,
		`gh "$s" status -t=1`,
		`gh "$s" status --show-token`,
		`gh "$s" status --show-token=true`,
		`gh "$s" status --hostname example.com -t`,
		`gh "$s" status "-t"`,
		`gh "$s" status -"t"`,
		`gh "$s" status $'-\x74'`,
		`gh "$s" status --show-tok\en`,
		"gh \"$s\" status \\\n  -t",
		`f=-t; gh "$s" status $f`,
		`f=--show-token; gh "$s" status "$f"`,
		`gh "$s" status`,
		`gh $s status`,
		`gh ${s} status`,
		"gh `x` status",
		`gh $(x) status`,
		`gh "$s" "$u"; git status`,
		`gh "$s" view 1 --json status`,
		`gh pr "$v" --json status`,
		`gh pr"$x" view --json status`,
		`g=gh; $g "$s" status "$f"`,
		`/usr/local/bin/gh "$s" status -t`,
		`T=$(gh "$s" status -t)`,
		`T=$(desktoken worker); GH_TOKEN=$(cat "$T") gh "$s" status -t`,
		"bash <<'E'\ngh \"$s\" status -t\nE",
		`printf 'gh "$s" status -t' | sh`,
	})
}

// regression: #5046
func TestForgeStatusAliasForms(t *testing.T) {
	// A literal word after gh may be an alias, and a pair under a command may
	// be too (`gh issue mine`). Only the listed pairs cannot be one.
	forgeRefuseAll(t, []string{
		`gh au status "$f"`,
		`gh au status -aat`,
		`gh au status -t`,
		`gh au status --show-token`,
		`gh au status`,
		`gh au -at status`,
		`gh au "$f" status`,
		`gh au "$x" "$f"; git status`,
		`echo status; gh au "$x" "$f"`,
		`g=gh; $g au status "$f"`,
		`gh pr view 1 --json status; "$g" au status "$f"`,
		"gh au status -at",
		"git status && gh au",
		"git status && gh issue mine",
		"gh pr mine --json status",
		"gh run mine; git status",
		"gh status",
		"gh pr; git status",
		`gh "pr view" 1 --json status`,
		`gh pr\ view 1 --json status`,
		`gh p\x72 view 1 --json status`,
		"gh pr{,x} view 1 --json status",
		"GH pr view 1 --json status",
		"gh api repos/x/y --json status",
		"gh alias set au x; git status",
		"gh extension exec x; git status",
		`gh config get "$k"; git status`,
	})
}

// regression: #5046
func TestForgeStatusOtherCommands(t *testing.T) {
	// Beside a listed read, anything that could print the credential another
	// way still refuses.
	forgeRefuseAll(t, []string{
		"gh pr view 1 --json status; ./x.sh",
		"gh pr view 1 --json status && ./x.sh",
		"gh pr view 1 --json status & ./x.sh",
		"gh pr view 1 --json status; sh x.sh",
		"gh pr view 1 --json status | sh",
		`eval "$c"; git status; gh pr view 1`,
		"bash -c 'gh pr view 1 --json status'",
		"cat ~/.config/gh/hosts.yml; gh pr view 1 --json status",
		"security find-generic-password; gh pr view 1 --json status",
		"gh pr view 1 --json status | grep -r x",
		"gh pr view 1 --json status | grep x hosts.yml",
		`gh pr view 1 --json status | grep "-rx"`,
		"gh pr view 1 --json status | grep -f hosts.yml",
		"gh pr view 1 --json status | grep --include=x y",
		"gh pr view 1 --json status | grep h*",
		"gh pr view 1 --json status | head hosts.yml",
		"gh pr view 1 --json status | tail -f hosts.yml",
		"gh pr view 1 --json status | wc hosts.yml",
		"gh pr view 1 --json status | jq x",
		"gh pr view 1 --json status > x",
		"gh pr view 1 --json status < x",
		"PAGER=x gh pr view 1 --json status",
		"GH_DEBUG=api gh pr view 1 --json status",
		`T=$(desktoken worker); GH_TOKEN=$(cat "$(x)") gh pr view 1 --json status`,
		`T=$(desktoken worker); GH_TOKEN=$(x) gh pr view 1 --json status`,
		`T=$(desktoken worker); PAGER=$(cat "$T") gh pr view 1 --json status`,
		`T=$(desktoken worker); GH_TOKEN=$(cat "$T") "$g" pr view 1 --json status; gh pr view 1`,
		"gh pr view 1 --web --json status",
		`gh pr view 1 "--web" --json status`,
		"gh pr view 1 -w; git status",
		"gh issue list -cw; git status",
		"T=$(desktokenx); gh pr view 1 --json status",
		"T=$(desktoken $(x)); gh pr view 1 --json status",
		"T=$(desktoken x > y); gh pr view 1 --json status",
		"git -c core.pager=x status; gh pr view 1",
		"git log; gh pr view 1 --json status",
		"gh pr view 1 --json status \\\n; x",
		"gh\rpr view 1 --json status",
		"gh pr view `x` --json status",
		`gh pr view 1 \--web --json status`,
		"gh pr view 1 (x) --json status",
		"(gh pr view 1 --json status)",
		"gh pr view 1 --json status\n./x.sh",
		`GH_DEBUG=$(cat "$T") gh pr view 1 --json status`,
		"gh pr checks 1 --json status | tail -f",
	})
}

// regression: #5046
func TestForgeStatusReadsAllowed(t *testing.T) {
	// `status` as gh output or another command's word is not the credential
	// read, on its own or with the role mint and a command-scoped GH_TOKEN.
	for _, cmd := range []string{
		"git status --short && gh pr view 1 --json state",
		"gh run view 1 --json status,conclusion",
		"gh pr checks 1 | grep status",
		`gh issue list -R x/y --search "status"`,
		"gh pr view 1 --json statusCheckRollup",
		"gh api repos/x/y/commits/abc/status",
		"git status -sb",
		`T=$(desktoken worker); GH_TOKEN=$(cat "$T") git status --short && gh pr view 1 --json state`,
		`T=$(desktoken worker); GH_TOKEN=$(cat "$T") gh run view 1 --json status,conclusion`,
		`T=$(desktoken worker); GH_TOKEN=$(cat "$T") gh pr checks 1 | grep status`,
		`T=$(desktoken worker); GH_TOKEN=$(cat "$T") gh issue list -R x/y --search "status"`,
		`T=$(desktoken --no-rotate worker --repo x/y 2>/dev/null); GH_TOKEN=$(cat "$T") gh run view 1 --json status,conclusion`,
		`T=$(desktoken worker); GH_TOKEN=$(cat "$T") gh pr checks 1 | grep status`,
		"git status && gh pr view 1",
		"gh run list --json status,name",
		"gh pr view 1 --json status --jq .status",
		`gh pr view 1 --json status --jq ".status"`,
		"gh pr list --json status --jq=.a.name",
		"gh pr list --json status -q .a.name",
		"gh run list --json status -q=.a.status",
		"gh run list --json status -q.a.status",
		"gh run view 1 --json status --jq .jobs.status",
		"gh pr view 1 --json status --jq .a.b",
		"gh issue list --repo x/y --json status",
		"gh issue list -R=x/y --json status",
		"gh issue list -Rx/y --json status",
		"gh run list --status completed --limit=5 -L 3 --json status",
		"gh run view 1 --log-failed --json status",
		"gh pr list --assignee @me --state=open --json status",
		"gh pr checks 1 | grep -A3 status",
		"gh pr checks 1 | grep -A 3 -i status | head -n 5 | wc -l",
		"gh pr checks 1 | tail -5 | grep status",
		"gh pr list -s open; git status --porcelain",
		"gh issue view 1 --json state; git status",
	} {
		if forgeTokenRead(cmd) {
			t.Errorf("%q: a listed read must stay allowed", cmd)
		}
	}
}

// regression: #5046
func TestForgeStatusJqFieldPathsOnly(t *testing.T) {
	// Only literal field paths clear, in every spelling of the flag.
	forgeRefuseAll(t, []string{
		"gh pr view 1 --json status --jq x",
		"gh pr view 1 --json status --jq x.y",
		`gh pr view 1 --json status --jq "x.y"`,
		`gh pr view 1 --json status --jq 'x.y'`,
		`gh pr view 1 --json status "--jq" x.y`,
		"gh pr view 1 --json status --jq=x.y",
		`gh pr view 1 --json status "--jq=x.y"`,
		`gh pr view 1 --json status --jq="x.y"`,
		"gh pr view 1 --json status -q x.y",
		`gh pr view 1 --json status "-q" x.y`,
		"gh pr view 1 --json status -qx.y",
		"gh pr view 1 --json status -q=x.y",
		`gh pr view 1 --json status -q"x.y"`,
		"gh run list --json status --jq x.y",
		"gh issue list --json status -q x",
		`T=$(desktoken worker); GH_TOKEN=$(cat "$T") gh pr view 1 --json status --jq x.y`,
		"gh pr view 1 --json status --jq .a --jq x.y",
		"gh pr view 1 --search --jq x.y status",
		"gh pr view 1 --json status --jq",
		"gh pr view 1 --json status -q",
		"gh pr view 1 --json status --jq .",
		"gh pr view 1 --json status --jq .a,.b",
		"gh pr view 1 --json status --jq .a+.b",
		"gh pr view 1 --json status --jq length",
		"gh pr view 1 --json status --jq .a-b",
		"gh pr list --json status --jq=.[0].name",
		"gh pr list --json status -q .[0].name",
		"gh run list --json status -q=.[0].status",
		"gh run list --json status -q.[0].status",
		"gh run view 1 --json status --jq .jobs[0].status",
		`gh pr view 1 --json status --jq ""`,
		"gh pr view 1 --json status --jq=",
		"gh pr view 1 --json status --template x",
		"gh pr view 1 --json status --template=x",
		"gh pr view 1 --json status -t x",
		"gh pr view 1 --json status -tx",
		"gh pr view 1 --json status -t=x",
		`gh run view 1 --json status -t '{{.status}}'`,
		"gh pr view 1 --json status -cq x.y",
		"gh pr list --json status -Lq x.y",
		"gh pr list --json status -L5",
	})
}

// regression: #5046
func TestForgeStatusQuotedBlank(t *testing.T) {
	// A quoted blank is one word to the shell and several here, so it refuses,
	// on a filter and on a gh read alike.
	const pre = "git status; gh pr list | "
	var rows []string
	for _, f := range []string{
		`grep "x -A" 123`, `grep 'x -B' 123`, `grep "x "-C 123`, `grep -v "nomatch -A" 456`,
		`grep "x	-A" 123`, `grep "x -A"`, `grep 'a b'`, `head "-n 5"`, `grep "x`,
	} {
		rows = append(rows, pre+f)
	}
	forgeRefuseAll(t, append(rows,
		`gh pr view 1 --search "x --search" --jq x.y --json status`,
		`gh pr view 1 "x --json" --jq x.y status`,
		`gh issue list --search "a b" --json status`,
		`gh pr view 1 --json status --jq ".a .b"`,
		`gh pr view 1 --json status --jq" "x.y`,
		`gh pr view "1 --json status`,
		`git status "a b" && gh pr view 1`,
	))
}

// regression: #5046
func TestForgeStatusExpansionBesideRead(t *testing.T) {
	// git status and a filter take free arguments, so an expansion, a substitution
	// or a redirection there refuses on its bytes, and a newline ends a command.
	forgeRefuseAll(t, []string{
		`git status $x; gh pr view 1`,
		`git status "$x"; gh pr view 1`,
		"git status `x`; gh pr view 1",
		`git status \x; gh pr view 1`,
		`git status (x); gh pr view 1`,
		`git status >x; gh pr view 1`,
		`git status <x; gh pr view 1`,
		"git status -s\n./x.sh\ngh pr view 1",
	})
}

// regression: #5046
func TestForgeStatusGhArgsClosed(t *testing.T) {
	// Every word on a listed gh read is on the list: no unknown flag, no host.
	forgeRefuseAll(t, []string{
		"gh pr view 1 --json status --unknown",
		"gh pr view 1 --json status --unknown=x",
		"gh pr view 1 --json status -x",
		"gh pr view 1 --json status --",
		"gh pr view 1 --json status -",
		"gh pr view https://example.com/x/y/pull/1 --json status",
		"gh issue list -R example.com/x/y --json status",
		"gh issue list --repo=example.com/x/y --json status",
		"gh issue list -Rexample.com/x/y --json status",
		"gh issue list -R x --json status",
		"gh issue list -R --json status",
		"gh pr list --search is:open --json status",
		"gh pr list --search=is:open --json status",
		"gh pr list --state= --json status",
		`gh pr view "" --json status`,
		"gh pr view 1 h* --json status",
		"gh pr view ~x --json status",
		"gh pr view 1 --json status --help",
		"gh run list -w x --json status",
	})
}

// regression: #5046
func TestForgeStatusWebValueForms(t *testing.T) {
	// A bool flag takes `=value`, and gh still opens the browser program.
	forgeRefuseAll(t, []string{
		"gh pr view 1 --web=true --json status",
		"gh pr view 1 --web=1 --json status",
		"gh pr view 1 --web=false --json status",
		"gh pr view 1 --web= --json status",
		`gh pr view 1 "--web=true" --json status`,
		`gh pr view 1 '--web=true' --json status`,
		`gh pr view 1 --web="true" --json status`,
		`gh pr view 1 "--web"=true --json status`,
		`gh pr view 1 --we"b=true" --json status`,
		"gh run view 1 --web=true --json status",
		"gh pr checks 1 --web=true | grep status",
		"gh issue list --web=true; git status",
		"gh pr view 1 -w=true --json status",
	})
}

// regression: #5046
func TestForgeStatusFilterNumbers(t *testing.T) {
	// A number is a count only straight after a flag that takes one. Anywhere
	// else it is a file operand: `head 123` opens a file named 123.
	const pre = "git status; gh pr list | "
	var refused []string
	for _, f := range []string{
		"head 123", "tail 123", "wc 123", "wc -l 123", `head "123"`, `head ""`,
		"grep status 123", "grep 123 456", `grep "" 123`, "grep -A 3 status 123", "grep -3 status 123",
		"head -n 5 123", "head -n5 123", "head -5 123", "tail -n 5 123", "tail -5 123", "head -c 5 123",
		"head -n", "grep -A", "head -n x", `head -n ""`, "head -n -c 5", "grep -A -i 3",
		"head -5c 123", "tail -5c 123", "head -5n 123", "head -n5c", "head -nc 5",
		"grep -3A 2 status", "grep -A3i status",
	} {
		refused = append(refused, pre+f)
	}
	forgeRefuseAll(t, refused)
	for _, f := range []string{
		"head -n 5", "head -n5", "head -5", "head -c 5", `head -n "5"`, "tail -n 5", "tail -5", "tail -c 5",
		"grep -A 3 status", "grep -A3 status", "grep -iA 3 status", "grep -B 2 -C 3 status", "grep -C3 status",
		"grep -3 status", "grep -c status", "grep -n status", "grep 123", "wc -l", "wc",
	} {
		if forgeTokenRead(pre + f) {
			t.Errorf("%q: a listed read must stay allowed", pre+f)
		}
	}
}

// regression: #5046
func TestForgeStatusKeepsAuthForms(t *testing.T) {
	// The clearing never reaches the literal words: every gh auth form still
	// refuses, whichever way status appears beside it.
	forgeRefuseAll(t, []string{
		"gh auth status",
		"gh auth status -a",
		"gh auth status --hostname example.com",
		"gh auth status -t",
		"git status && gh auth token",
		"gh run view 1 --json status; gh auth status --show-token",
		"gh pr checks 1 | grep status; gh config get oauth_token",
		"gh pr view auth --json status",
	})
}

// regression: #5046
func TestForgeStatusRetainedRefusals(t *testing.T) {
	// False refusals this design keeps, each named in docs/write-policy.md. Each
	// one can run as a listed read, as a file-backed body, or in two calls.
	forgeRefuseAll(t, []string{
		"git status && gh pr create -t title",
		"gh status",
		"gh api repos/x/y/pulls | grep status",
		"cd x && gh pr view 1 --json status",
		`[ "$n" -lt 5 ] && gh run view 1 --json status`,
		`gh run view "$id" --json status`,
		"gh pr list --json status | head -5c",
		"gh pr checks 1 | grep -3A 2 status",
		"gh run view 1 --json status --jq '.status | ascii_upcase'",
		"gh run view 1 --json status --jq length",
		"gh pr list --json status --jq '.[0].status'",
		"gh pr view 1 --json status --template x",
		`gh issue list --search "a b" --json status`,
		"gh pr checks 1 | grep 'a b status'",
		"gh pr list --search is:open --json status",
		"gh pr list -L5 --json status",
		"gh pr list --json status | grep ^a",
		"gh pr list --json status | grep a#",
		"F=$(desktoken worker); gh pr view 1 --json status",
		`GH_TOKEN=$(cat "$T") gh pr view 1 --json status`,
		`git status && T=$(desktoken worker); GH_TOKEN=$(cat "$T") gh pr view 1 --json status`,
	})
}

// forgePrefixForms: spellings of the role mint then a GH_TOKEN prefix that clear.
var forgePrefixForms = []string{
	`T=$(desktoken worker); GH_TOKEN=$(cat "$T") gh pr view 1 --json status`,
	"T=$(desktoken worker)\nGH_TOKEN=$(cat \"$T\") gh pr view 1 --json status",
	`T=$(desktoken worker) && GH_TOKEN=$(cat "$T") gh pr view 1 --json status`,
	`TF=$(desktoken worker); GH_TOKEN=$(cat "$TF") gh pr view 1 --json status`,
	`git status; T=$(desktoken worker); GH_TOKEN=$(cat "$T") gh pr view 1 --json status`,
	`T=$(desktoken worker); git status; GH_TOKEN=$(cat "$T") gh pr view 1 --json status`,
	`T=$(desktoken worker); GH_TOKEN=$(cat "$T") gh pr list --json status; GH_TOKEN=$(cat "$T") gh pr view 1`,
	`T=$(desktoken worker); TF=$(desktoken worker); GH_TOKEN=$(cat "$TF") gh pr view 1 --json status`,
}

// regression: #5046
func TestForgeStatusPrefixNeedsMint(t *testing.T) {
	// The prefix clears only after a mint of the same variable that is sure to run
	// in this shell before it. A later mint, another variable, or a mint a separator
	// or a comment could skip or fork, does not count.
	const read = `GH_TOKEN=$(cat "$T") gh pr view 1 --json status`
	forgeRefuseAll(t, []string{
		read,
		`GH_TOKEN=$(cat "$TF") gh pr view 1 --json status`,
		read + "; T=$(desktoken worker)",
		`TF=$(desktoken worker); ` + read,
		`T=$(desktoken worker); GH_TOKEN=$(cat "$TF") gh pr view 1 --json status`,
		`T=$(desktoken worker) | ` + read,
		`T=$(desktoken worker) & ` + read,
		`T=$(desktoken worker) &; ` + read,
		`T=$(desktoken worker) || ` + read,
		"T=$(desktoken worker) |\n" + read,
		`git status || T=$(desktoken worker); ` + read,
		`git status && T=$(desktoken worker); ` + read,
		`git status | T=$(desktoken worker); ` + read,
		`git status & T=$(desktoken worker); ` + read,
		"git status &&\nT=$(desktoken worker); " + read,
		"git status |\nT=$(desktoken worker); " + read,
		"git status # ; T=$(desktoken worker)\n" + read,
		"git status #\nT=$(desktoken worker); " + read,
	})
	// A closed set of names on both sides, each name on both sides of one command.
	for _, name := range []string{"X", "TFF", "TT", "t", "T0", "GH_TOKEN", "PATH"} {
		forgeRefuseAll(t, []string{fmt.Sprintf(`%[1]s=$(desktoken worker); GH_TOKEN=$(cat "$%[1]s") gh pr view 1 --json status`, name)})
	}
	forgeRefuseAll(t, []string{"TFF=$(desktoken worker); gh pr view 1 --json status"})
	for _, cmd := range forgePrefixForms {
		if forgeTokenRead(cmd) {
			t.Errorf("%q: must clear", cmd)
		}
	}
}

// regression: #5046
func TestForgeStatusPrefixFormsRunTheMint(t *testing.T) {
	// Every form that clears reads the file the mint just assigned, in a real shell,
	// whatever the variable held before it ran.
	dir := t.TempDir()
	file := filepath.Join(dir, "minted")
	for name, body := range map[string]string{
		"minted":    "",
		"desktoken": "#!/bin/sh\nprintf '%s\\n' '" + file + "'\n",
		"gh":        "#!/bin/sh\n",
		"git":       "#!/bin/sh\n",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	for _, shell := range [][]string{{"sh"}, {"bash"}, {"zsh", "-f"}} {
		t.Run(strings.Join(shell, " "), func(t *testing.T) {
			bin := forgeShell(t, shell[0])
			for _, form := range forgePrefixForms {
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()
				cmd := exec.CommandContext(ctx, bin, append(append([]string{}, shell[1:]...), "-c", form)...)
				cmd.Dir, cmd.Env = dir, []string{"HOME=" + dir, "PATH=" + dir + ":" + os.Getenv("PATH"), "T=held-before", "TF=held-before"}
				if out, err := cmd.CombinedOutput(); err != nil || len(out) != 0 {
					t.Errorf("%q under %v: output %q, run error %v", form, shell, out, err)
				}
			}
		})
	}
}

// regression: #5046
func TestForgeStatusFilterOperandQuotes(t *testing.T) {
	// `^` and `#` refuse outside a quoted span and clear inside one. `=` clears
	// either way: a shell's `=` expansion gives one word at most, the grep pattern.
	forgeRefuseAll(t, []string{
		"gh pr list --json status | grep ^a",
		"gh pr list --json status | grep -v ^a",
		"gh pr list --json status | grep a#",
		"gh pr list --json status | grep ^'a'",
		"gh pr list --json status | grep 'a'^a",
		`gh pr list --json status | grep "a"#`,
		"gh pr list --json status | grep '^a'#",
		`gh pr list --json status | grep '"'^a`,
		`gh pr list --json status | grep "'"^a`,
	})
	for _, cmd := range []string{
		"gh pr list --json status | grep '^a'",
		`gh pr list --json status | grep "^a"`,
		"gh pr list --json status | grep 'a#'",
		`gh pr list --json status | grep "a#"`,
		"gh pr list --json status | grep '^'a",
		"gh pr list --json status | grep a'#'",
		"gh pr list --json status | grep =a",
		"gh pr list --json status | grep a=0",
		`gh pr list --json status | grep '"^a'`,
		`gh pr list --json status | grep "'^a"`,
		"gh pr list --json status | grep -v '^a'",
	} {
		if forgeTokenRead(cmd) {
			t.Errorf("%q: must clear", cmd)
		}
	}
}

// regression: #5046
func TestForgeStatusClosedSets(t *testing.T) {
	// Each closed set, walked byte by byte, so one more byte in a set goes red.
	for _, b := range "*?[]{}~" {
		forgeRefuseAll(t, []string{
			fmt.Sprintf("gh pr list --json status | grep a%c", b),
			fmt.Sprintf("gh pr list --json status | grep '%ca'", b),
		})
	}
	for filter, flags := range map[string]string{"grep": "AaBCEFHchilnoqsvwx", "head": "nc", "tail": "nc", "wc": "lwcm"} {
		for b := 'A'; b <= 'z'; b++ {
			if (b <= 'Z' || b >= 'a') && !strings.ContainsRune(flags, b) {
				forgeRefuseAll(t, []string{fmt.Sprintf("gh pr list --json status | %s -%c", filter, b)})
			}
		}
	}
	// Rows, so this holds on a machine without the shell the read-back test needs.
	for _, b := range "*?[]{}~^#=!%:+" {
		forgeRefuseAll(t, []string{fmt.Sprintf("gh pr list --json status a%c", b)})
	}
	forgeRefuseAll(t, []string{
		"gh pr list --json status | wc -5",
		"gh pr list --json status | wc -l5",
		"T=$(desktoken --repo=x/y worker); gh pr view 1 --json status",
		"gh pr list --repo a:b/c --json status",
		"gh pr list --repo a/b:c --json status",
		"gh pr list -R a:b/c --json status",
		"PATH=$(desktoken worker); gh pr view 1 --json status",
		"HOME=$(desktoken worker); gh pr view 1 --json status",
		"BASH_ENV=$(desktoken worker); gh pr view 1 --json status",
		"GH_HOST=$(desktoken worker); gh pr view 1 --json status",
		"XT=$(desktoken worker); gh pr view 1 --json status",
		"XGH_TOKEN=$(desktoken worker); gh pr view 1 --json status",
	})
	for _, name := range []string{"GH_TOKEN", "T", "TF"} {
		if cmd := name + "=$(desktoken worker); gh pr view 1 --json status"; forgeTokenRead(cmd) {
			t.Errorf("%q: the role mint must clear", cmd)
		}
	}
}

// forgeWords lists every string over alphabet up to length max.
func forgeWords(alphabet string, max int) []string {
	words, last := []string{}, []string{""}
	for n := 0; n < max; n++ {
		var next []string
		for _, w := range last {
			for _, r := range alphabet {
				next = append(next, w+string(r))
			}
		}
		words, last = append(words, next...), next
	}
	return words
}

// regression: #5046
func TestForgeStatusClearedWordsSurviveExpansion(t *testing.T) {
	// A word that clears must read the same after a real shell has expanded it,
	// or the guard has checked a word gh never receives. This does not list the
	// spellings to refuse: it tries every short word over the bytes shells expand,
	// keeps those the matcher clears, and has each shell expand them beside files
	// any pattern among them could match.
	dir := t.TempDir()
	plant := func(name string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(dir, "a"), 0o700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"a/a", "a/0", "a/a0"} {
		plant(name)
	}
	for _, s := range forgeWords("a0_.-", 3) {
		if s == "a" || s == "." || s == ".." {
			continue
		}
		plant(s)
		for _, prefix := range []string{"--jq=", "-q", "-q=", "--repo=", "-R", "--search="} {
			plant(prefix + s)
		}
	}
	checked := []string{"--jq %s", "--jq=%s", "-q %s", "-q%s", "-q=%s"}
	filters := []string{"| grep %s", "| grep -v %s", "| head %s", "| wc %s"}
	templates := append([]string{"--repo %s", "--repo=%s", "-R %s", "-R%s", "--search %s", "--search=%s", "%s"}, checked...)
	type batch struct {
		templates []string
		values    []string
	}
	cleared, seen := []string{}, map[string]bool{}
	for _, b := range []batch{
		{templates, forgeWords(`.[]*?{}~,-/=^#!%a0_@`, 3)},
		{checked, forgeWords(".[]a0", 6)},
		// No `=` here: zsh turns a word starting `=` into one path or stops, so it
		// reads back changed by design, and stays one word (TestForgeStatusFilterOperandQuotes).
		{filters, forgeWords(`.[]*?{}~,-/^#!%a0_@`, 3)},
		{filters, []string{"'^a'", `"a#"`, "'^'a", "a'#'", "'=a'", "a'^'0"}},
		{templates, []string{"{a,0}", "a{0,a}", ".a{,0}", "a/a{,0}", "'.a'", `".a"`, "'a'/'a'", "a/a*", "a/a?", "a/a[0]", "a/a{,0}"}},
	} {
		for _, tmpl := range b.templates {
			for _, v := range b.values {
				args := fmt.Sprintf(tmpl, v)
				if forgeTokenRead("gh pr view 1 --json status " + args) {
					continue
				}
				for _, w := range strings.Fields(args) {
					if w != "|" && w != "grep" && w != "head" && w != "wc" && !seen[w] {
						seen[w] = true
						cleared = append(cleared, w)
					}
				}
			}
		}
	}
	for _, w := range []string{".a", ".a.a", "a/a", "--jq=.a", "-q.a", "'^a'", `"a#"`} {
		if !seen[w] {
			t.Errorf("%q: this literal word must clear, or the check below proves nothing", w)
		}
	}
	var script strings.Builder
	for _, w := range cleared {
		fmt.Fprintf(&script, "printf '%%s\\n' %s\n", w)
	}
	path := filepath.Join(t.TempDir(), "words.sh")
	if err := os.WriteFile(path, []byte(script.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, shell := range [][]string{{"sh"}, {"bash"}, {"bash", "-O", "extglob"}, {"zsh", "-f"}, {"zsh", "-f", "-o", "extendedglob"}} {
		t.Run(strings.Join(shell, " "), func(t *testing.T) {
			bin := forgeShell(t, shell[0])
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, bin, append(append([]string{}, shell[1:]...), path)...)
			cmd.Dir, cmd.Env = dir, []string{"HOME=" + dir, "LC_ALL=C", "PATH=" + os.Getenv("PATH")}
			out, err := cmd.CombinedOutput()
			got := strings.Split(strings.TrimSuffix(string(out), "\n"), "\n")
			for i, w := range cleared {
				if want := forgeDropQuotes.Replace(w); i >= len(got) || got[i] != want {
					t.Errorf("%q: %v does not read this word back unchanged (run error: %v)", w, shell, err)
					break
				}
			}
			if err != nil || len(got) != len(cleared) {
				t.Errorf("%q: %v gave %d lines for %d words (run error: %v)", shell[0], shell, len(got), len(cleared), err)
			}
		})
	}
}

// forgeShell finds a shell, or skips the subtest by name, so a shell this machine
// lacks reads as SKIP in the test output and never as a pass.
func forgeShell(t *testing.T, name string) string {
	t.Helper()
	bin, err := exec.LookPath(name)
	if err != nil {
		t.Skipf("could not check: %s is not on PATH", name)
	}
	return bin
}
