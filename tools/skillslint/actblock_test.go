package main

import (
	"fmt"
	"strings"
	"testing"
)

// actblock_test.go — the act-block lint (actblock.go): every example Act block
// under plugins/ opens with the zsh comment guard and keeps its comment lines
// free of shell metacharacters and in the block's header.

const actFence = "```"

// actSkill wraps one fenced block in a minimal skill file.
func actSkill(info string, lines ...string) string {
	return strings.Join(append(append([]string{
		"---", "name: some-desk", "description: x", "---", "", actFence + info,
	}, lines...), actFence, ""), "\n")
}

var cleanActLines = []string{
	actBlockGuardLine,
	"# 0. the act as one function. Pasting this only prints. Type driver_act_1 live to act.",
	"driver_act_1() (",
	`  DRY_RUN=1; [ "${1-}" = live ] && DRY_RUN=0`,
	"  echo '1. push the parked branch. git push has its own --dry-run.'",
	`  if [ "$DRY_RUN" = 1 ]; then git push --dry-run origin b; else git push origin b; fi`,
	`  if [ "$DRY_RUN" = 0 ]; then T=; read -rs T || exit 1; [ -n "$T" ] || exit 1; fi`,
	`  echo "would read ${#T} chars"`,
	")",
	"driver_act_1",
}

// cleanActLine is the file line number of cleanActLines[i] inside actSkill:
// six lines of header and opening fence come first.
func cleanActLine(i int) int { return i + 7 }

// withLine returns cleanActLines with line inserted at index i.
func withLine(i int, line string) []string {
	lines := append([]string{}, cleanActLines[:i]...)
	lines = append(lines, line)
	return append(lines, cleanActLines[i:]...)
}

// lintOne lints one skill holding one fenced block and returns the issues.
func lintOne(t *testing.T, info string, lines []string) (int, []Issue) {
	t.Helper()
	root := t.TempDir()
	writeSkill(t, root, "some-desk", actSkill(info, lines...))
	blocks, issues, err := ActBlockIssues(root)
	if err != nil {
		t.Fatalf("ActBlockIssues: %v", err)
	}
	return blocks, issues
}

// wantOneAt fails unless issues holds exactly one issue, at file line n,
// whose message contains want.
func wantOneAt(t *testing.T, issues []Issue, n int, want string) {
	t.Helper()
	if len(issues) != 1 || !strings.Contains(issues[0].Msg, fmt.Sprintf("line %d:", n)) ||
		!strings.Contains(issues[0].Msg, want) {
		t.Fatalf("issues=%+v, want exactly one at line %d containing %q", issues, n, want)
	}
}

// TestActBlockLintClean — a block that keeps every rule is counted and clean,
// in each shell fence language the lint reads.
func TestActBlockLintClean(t *testing.T) {
	for _, info := range []string{"sh", "bash", "zsh", "shell"} {
		blocks, issues := lintOne(t, info, cleanActLines)
		if blocks != 1 || len(issues) != 0 {
			t.Fatalf("%s: blocks=%d issues=%+v, want 1 block and no issue", info, blocks, issues)
		}
	}
}

// TestActBlockLintFlagsComment — each character that runs in a
// comment-as-command is flagged, at the line it sits on, whether the comment
// sits in the header or indented inside the act function, where its placement
// is flagged as well. Each single-character row carries one forbidden
// character only, so allowing that one character turns its row green.
func TestActBlockLintFlagsComment(t *testing.T) {
	for _, c := range []struct{ name, comment string }{
		{"semicolon then backtick span", "# 0. pasting only prints; " + "`driver_act live`" + " acts"},
		{"command substitution", "# 1. push $(git branch)"},
		{"parentheses", "# 2. push the branch (git push has its own --dry-run)"},
		{"redirect", "# 3. log it > out.txt"},
		{"pipe", "# 4. read it | sh"},
		{"apostrophe", "# 5. don't skip this"},
		{"glob", "# 6. every *.md"},
		{"trailing backslash", `# 7. joins the next line \`},
		{"semicolon alone", "# 8. push the branch; then stop"},
		{"backtick alone", "# 9. push the " + "`" + "branch"},
		{"dollar alone", "# 10. push the branch for $USER"},
		{"ampersand alone", "# 11. push the branch & stop"},
	} {
		t.Run(c.name, func(t *testing.T) {
			_, issues := lintOne(t, "sh", withLine(2, c.comment))
			wantOneAt(t, issues, cleanActLine(2), "comment carries")
		})
		t.Run(c.name+", indented in the act function", func(t *testing.T) {
			_, issues := lintOne(t, "sh", withLine(4, "  "+c.comment))
			wantIssues(t, issues, cleanActLine(4), actWant{0, "comment carries"}, actWant{0, actPlaceMsg})
		})
	}
}

// TestActBlockLintFlagsTrailingComment — a # after code on the same line is
// flagged; a # inside a word, such as ${#T}, is not.
func TestActBlockLintFlagsTrailingComment(t *testing.T) {
	_, issues := lintOne(t, "sh", withLine(4, "  run gh issue edit 1 # plain text"))
	wantOneAt(t, issues, cleanActLine(4), "trailing # comment")
	_, issues = lintOne(t, "sh", withLine(4, `  echo "${#T} $#"`))
	if len(issues) != 0 {
		t.Fatalf("issues=%+v, want none for a # inside a word", issues)
	}
}

// TestActBlockLintFlagsCommentAfterOperator — a # that starts a word starts a
// comment in a shell that reads comments, whatever ends the word before it: a
// blank, a list or pipe operator, a redirect, a parenthesis, a backtick or a
// $( . Each row is flagged at the line the # sits on. In zsh without
// interactive_comments the same # is a command, so `echo dry;#;echo live`
// runs the live echo there.
func TestActBlockLintFlagsCommentAfterOperator(t *testing.T) {
	for _, c := range []struct {
		name  string
		lines []string
		at    int  // index into lines of the line the # sits on
		place bool // the # line is a full-line comment after code: its placement is flagged too
	}{
		{"semicolon", []string{"  echo dry;#;echo live"}, 0, false},
		{"pipe", []string{"  echo dry|#x", "  cat"}, 0, false},
		{"background", []string{"  echo dry&#x", "  wait"}, 0, false},
		{"and list", []string{"  echo dry&&#x", "  echo n"}, 0, false},
		{"or list", []string{"  true||#x", "  echo n"}, 0, false},
		{"open paren", []string{"  (#x", "  echo sub)"}, 0, false},
		{"close paren", []string{"  (echo sub)#x"}, 0, false},
		{"redirect out", []string{"  echo a>#f"}, 0, false},
		{"redirect in", []string{"  cat <#f"}, 0, false},
		{"backtick", []string{"  echo `#x", "  echo bq`"}, 0, false},
		{"command substitution", []string{"  echo $(#x", "  echo cs)"}, 0, false},
		{"command substitution in quotes", []string{`  echo "$(#x`, `  echo cs)"`}, 0, false},
		{"after a line continuation", []string{`  echo a \`, "  #c"}, 1, true},
		{"after a quote closed on an earlier line", []string{`  echo "a`, `  b";#c`}, 1, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			_, issues := lintOne(t, "sh", withLines(4, c.lines...))
			want := []actWant{{c.at, actTrailMsg}}
			if c.place {
				want = append(want, actWant{c.at, actPlaceMsg})
			}
			wantIssues(t, issues, cleanActLine(4), want...)
		})
	}
}

// TestActBlockLintHashInWordNotComment — a # that does not start a word, or
// sits inside quotes or after a backslash, starts no comment and is not
// flagged.
func TestActBlockLintHashInWordNotComment(t *testing.T) {
	for _, c := range []struct {
		name  string
		lines []string
	}{
		{"inside a word", []string{"  echo a#b"}},
		{"in double quotes", []string{`  echo "a#b;c"`}},
		{"in single quotes", []string{`  echo 'a#b;c'`}},
		{"escaped", []string{`  echo x\#y`}},
		{"escaped after an operator", []string{`  echo x;\#y`}},
		{"length and count expansions", []string{`  echo "${#T}" $# ${#T}`}},
		{"after a closing brace", []string{`  echo "${T}#b"`}},
		{"in a double quote across lines", []string{`  echo "a`, `  b#c;"`}},
		{"in a single quote across lines", []string{`  echo 'a`, `  b#c;'`}},
	} {
		t.Run(c.name, func(t *testing.T) {
			_, issues := lintOne(t, "sh", withLines(4, c.lines...))
			if len(issues) != 0 {
				t.Fatalf("issues=%+v, want none for a # that starts no comment", issues)
			}
		})
	}
}

// actWant is one issue a test row expects: at is an index into the row's
// lines, msg a substring of the issue.
type actWant struct {
	at  int
	msg string
}

// wantIssues fails unless issues holds exactly the wanted issues, in any
// order, each at file line base+at.
func wantIssues(t *testing.T, issues []Issue, base int, want ...actWant) {
	t.Helper()
	used := make([]bool, len(issues))
	for _, w := range want {
		found := false
		for i, is := range issues {
			if !used[i] && strings.Contains(is.Msg, fmt.Sprintf("line %d:", base+w.at)) && strings.Contains(is.Msg, w.msg) {
				used[i], found = true, true
				break
			}
		}
		if !found {
			t.Fatalf("issues=%+v, want one at line %d containing %q (want %+v)", issues, base+w.at, w.msg, want)
		}
	}
	if len(issues) != len(want) {
		t.Fatalf("issues=%+v, want exactly %d: %+v", issues, len(want), want)
	}
}

// actRowAt is where the quote-state rows go: after the act function's last
// read and before its closing parenthesis, so the read check sees the clean
// block's reads before any row's quotes.
var actRowAt = len(cleanActLines) - 2

const (
	actTrailMsg = "trailing # comment"
	actUnmodMsg = "the lint does not model"
	actOpenMsg  = "ends inside"
	actPlaceMsg = "comment line stands after code"
)

// TestActBlockLintQuoteStateBeforeTrailingComment — text that only looks like
// an open quote must not hide a later trailing comment. A heredoc body is
// data, so its apostrophe or double quote opens nothing; a $'…' quote ends at
// its first unescaped quote; comment text is not read for quotes. Each row is
// followed once by `echo dry #;echo live` and once by `echo dry;#;echo live`,
// and that line must be flagged. An interactive zsh without
// interactive_comments runs the live echo of both.
func TestActBlockLintQuoteStateBeforeTrailingComment(t *testing.T) {
	for _, c := range []struct {
		name  string
		lines []string
		extra []actWant
	}{
		{"heredoc with an apostrophe, quoted delimiter", []string{"  cat <<'EOF'", "it's data", "EOF"}, nil},
		{"heredoc with an apostrophe, unquoted delimiter", []string{"  cat <<EOF", "it's data", "EOF"}, nil},
		{"heredoc with an apostrophe, double-quoted delimiter", []string{`  cat <<"EOF"`, "it's data", "EOF"}, nil},
		{"heredoc with an apostrophe, backslash delimiter", []string{`  cat <<\EOF`, "it's data", "EOF"}, nil},
		{"heredoc with an apostrophe, delimiter partly quoted", []string{`  cat <<E"O"F`, "it's data", "EOF"}, nil},
		{"tab-stripped heredoc with an apostrophe", []string{"  cat <<-EOF", "\tit's data", "\tEOF"}, nil},
		{"heredoc with a double quote", []string{"  cat <<EOF", `say "hi`, "EOF"}, nil},
		{"heredoc with a backtick", []string{"  cat <<'EOF'", "a ` b", "EOF"}, nil},
		{"two heredocs on one line, apostrophe in the second", []string{"  cat <<A <<B", "a", "A", "it's data", "B"}, nil},
		{"heredoc in a command substitution", []string{"  x=$(cat <<'EOF'", "it's data", "EOF", "  )"}, nil},
		{"ANSI-C quote with an escaped quote", []string{`  echo $'it\'s'`}, []actWant{{0, actUnmodMsg}}},
		{"ANSI-C quote with an escaped backslash", []string{`  echo $'a\\' 'b'`}, nil},
		{"full-line comment with an apostrophe", []string{"  # don't"}, []actWant{{0, "comment carries"}, {0, actPlaceMsg}}},
		{"full-line comment with a double quote", []string{`  # say "hi`}, []actWant{{0, "comment carries"}, {0, actPlaceMsg}}},
		{"trailing comment with an apostrophe", []string{"  echo a # don't"}, []actWant{{0, actTrailMsg}}},
		{"trailing comment with a double quote", []string{`  echo a # say "hi`}, []actWant{{0, actTrailMsg}}},
		{"trailing comment after an operator, with an apostrophe", []string{"  echo a;# don't"}, []actWant{{0, actTrailMsg}}},
	} {
		for _, tail := range []string{"  echo dry #;echo live", "  echo dry;#;echo live"} {
			t.Run(c.name+" then "+strings.TrimSpace(tail), func(t *testing.T) {
				lines := append(append([]string{}, c.lines...), tail)
				_, issues := lintOne(t, "sh", withLines(actRowAt, lines...))
				want := append([]actWant{{len(c.lines), actTrailMsg}}, c.extra...)
				wantIssues(t, issues, cleanActLine(actRowAt), want...)
			})
		}
	}
}

// TestActBlockLintUnclosedAtBlockEnd — a block that ends inside a quote, a
// substitution, a heredoc or a line continuation fails at the line that opens
// it: a shell reads the rest of the block, and whatever is pasted next, inside
// it, so the lint cannot say what runs.
func TestActBlockLintUnclosedAtBlockEnd(t *testing.T) {
	opened := []actWant{{0, actOpenMsg}}
	for _, c := range []struct {
		name  string
		lines []string
		want  []actWant
	}{
		{"single quote", []string{"echo 'a"}, opened},
		{"double quote", []string{`echo "a`}, opened},
		{"ANSI-C quote", []string{`echo $'a`}, opened},
		{"command substitution", []string{"echo $(echo a"}, opened},
		{"parameter expansion", []string{"echo ${a"}, opened},
		{"backtick span", []string{"echo `echo a"}, opened},
		{"heredoc with no closing line", []string{"cat <<EOF", "a"}, opened},
		{"heredoc operator on the block's last line", []string{"cat <<EOF"}, opened},
		{"heredoc whose closing line is indented under <<", []string{"cat <<EOF", "a", "  EOF"}, opened},
		{"line continuation", []string{`echo a \`}, opened},
		{"command substitution in an unquoted heredoc body", []string{"cat <<EOF", "$(echo a", "EOF"}, []actWant{{1, actOpenMsg}}},
		{"single quote hiding a later trailing comment", []string{"echo 'a", "echo dry #;echo live"}, []actWant{{0, actOpenMsg}, {1, actTrailMsg}}},
	} {
		t.Run(c.name, func(t *testing.T) {
			_, issues := lintOne(t, "sh", withLines(len(cleanActLines), c.lines...))
			wantIssues(t, issues, cleanActLine(len(cleanActLines)), c.want...)
		})
	}
}

// TestActBlockLintQuoteStateNotComment — a # inside data or a word is not a
// comment: a heredoc body, a here-string, a $'…' or $"…" quote, a parameter
// expansion, arithmetic, $$, a case pattern at the top level and a
// backslash-newline inside quotes. Each # here follows a word character: the
// floor flags a # straight after a blank or an operator character even in such
// places (TestActFloorAnyWordEnd).
func TestActBlockLintQuoteStateNotComment(t *testing.T) {
	for _, c := range []struct {
		name  string
		lines []string
	}{
		{"heredoc body, quoted delimiter", []string{"  cat <<'EOF'", "x#y; it's (data) `here` $(not run)", "EOF"}},
		{"heredoc body, unquoted delimiter", []string{"  cat <<EOF", `x#y; it's "q`, "EOF"}},
		{"heredoc body, tab-stripped", []string{"  cat <<-EOF", "\tx#y; it's", "\tEOF"}},
		{"heredoc body read with the delimiter on the same line as more code", []string{"  cat <<'EOF' | cat", "x#y; it's", "EOF"}},
		{"heredoc in a command substitution", []string{"  x=$(cat <<'EOF'", "x#y; it's", "EOF", "  )", `  echo "$x"`}},
		{"here-string", []string{"  cat <<<'a#b;c'", "  echo it"}},
		{"ANSI-C quote", []string{`  echo $'a#b;c' $'c\\'`}},
		{"locale quote", []string{`  echo $"a#b;c"`}},
		{"parameter expansions", []string{`  echo ${T#x} ${T%%#*} ${#T} $# "${T:-a#b;}" ${T:-"a#b;"}`}},
		{"arithmetic", []string{"  echo $((16#ff + $#))", "  (( X = 16#ff ))"}},
		{"arithmetic with a nested group", []string{"  echo $(( (1+2) * 3 ))"}},
		{"process id", []string{`  echo "$$" $$ "$${" $${`}},
		{"subshell inside a command substitution", []string{"  echo $( (echo a) )x"}},
		{"case at the top level", []string{`  case "${1-}" in a) echo a;; *) echo b;; esac`}},
		{"backslash-newline in double quotes", []string{`  echo "a\`, `  b#c;"`}},
		{"backslash-newline in single quotes", []string{`  echo 'a\`, `  b#c;'`}},
	} {
		t.Run(c.name, func(t *testing.T) {
			_, issues := lintOne(t, "sh", withLines(actRowAt, c.lines...))
			if len(issues) != 0 {
				t.Fatalf("issues=%+v, want none", issues)
			}
		})
	}
}

// TestActBlockLintStrictWhereNotModelled — where the lint does not model a
// construct, or shells disagree on it, it fails rather than pass: a # after a
// blank is flagged even in quotes or a heredoc body (the per-line check kept
// next to the scanner); a code # in an unquoted heredoc's $( ) is flagged; a
// \' in a $'…' quote, a single quote in ${…}, a case in $( ) and a << in
// arithmetic are refused.
func TestActBlockLintStrictWhereNotModelled(t *testing.T) {
	for _, c := range []struct {
		name  string
		lines []string
		want  []actWant
	}{
		{"# after a blank in a heredoc body", []string{"  cat <<'EOF'", "x #y", "EOF"}, []actWant{{1, actTrailMsg}}},
		{"# after a blank in double quotes", []string{`  echo "a #b"`}, []actWant{{0, actTrailMsg}}},
		{"# after a blank in a parameter expansion", []string{`  echo ${T:-a #b}`}, []actWant{{0, actTrailMsg}}},
		{"# after an operator in an unquoted heredoc's command substitution", []string{"  cat <<EOF", "$(echo dry;#;echo live)", "EOF"}, []actWant{{1, actTrailMsg}, {1, actOpenMsg}}},
		{"escaped quote in an ANSI-C quote", []string{`  echo $'it\'s'`}, []actWant{{0, actUnmodMsg}}},
		{"single quote in a parameter expansion", []string{`  echo "${T:-'a'}"`}, []actWant{{0, actUnmodMsg}}},
		{"case in a command substitution", []string{`  x=$(case "${1-}" in a) echo a;; esac)`}, []actWant{{0, actUnmodMsg}}},
		{"heredoc with no delimiter word", []string{"  cat <<;echo a"}, []actWant{{0, actUnmodMsg}}},
		{"shift in arithmetic", []string{"  echo $((1<<2))"}, []actWant{{0, actUnmodMsg}}},
	} {
		t.Run(c.name, func(t *testing.T) {
			_, issues := lintOne(t, "sh", withLines(actRowAt, c.lines...))
			wantIssues(t, issues, cleanActLine(actRowAt), c.want...)
		})
	}
}

// TestActFloorAnyWordEnd — the per-line floor flags a # straight after a
// blank or after any character that ends a bash or zsh operator, whatever the
// quote, span or heredoc state around it, so no reading of the block can hide
// one. Quoted and heredoc rows fail too: spell those another way.
func TestActFloorAnyWordEnd(t *testing.T) {
	for _, c := range []struct {
		name  string
		lines []string
		at    int
	}{
		{"semicolon in double quotes", []string{`  echo "a ;#b"`}, 0},
		{"semicolon in single quotes", []string{`  echo 'a ;#b'`}, 0},
		{"semicolon in a double quote across lines", []string{`  echo "a`, `  b ;#c"`}, 1},
		{"semicolon in a single quote across lines", []string{`  echo 'a`, `  b ;#c'`}, 1},
		{"semicolon in a heredoc body", []string{"  cat <<'EOF'", "x;#y", "EOF"}, 1},
		{"semicolon in a here-string", []string{"  cat <<<'a;#b'"}, 0},
		{"semicolon in an ANSI-C quote", []string{`  echo $'a;#b'`}, 0},
		{"semicolon in a parameter expansion", []string{`  echo "${T:-a;#b}"`}, 0},
		{"after a command substitution", []string{"  echo $(echo a)#b"}, 0},
		{"after a backtick span", []string{"  echo `echo c`#d"}, 0},
		{"after a nested subshell", []string{"  echo $( (echo a) )#x"}, 0},
		{"after an output descriptor close", []string{"  echo dry >&-#;echo live"}, 0},
		{"after an input descriptor close", []string{"  cat <&-#;echo live"}, 0},
		{"after the zsh clobber redirect", []string{"  echo a >!#;echo live"}, 0},
		{"after the zsh disown operator", []string{"  sleep 1 &!#;echo live"}, 0},
		{"after a carriage return", []string{"  echo a\r#;echo live"}, 0},
		// Inside double quotes the scanner reads no word start, so only the
		// floor sees these: each pins one of its characters.
		{"pipe in double quotes", []string{`  echo "a|#b"`}, 0},
		{"ampersand in double quotes", []string{`  echo "a&#b"`}, 0},
		{"less-than in double quotes", []string{`  echo "a<#b"`}, 0},
		{"greater-than in double quotes", []string{`  echo "a>#b"`}, 0},
		{"open paren in double quotes", []string{`  echo "a(#b"`}, 0},
		{"carriage return in double quotes", []string{"  echo \"a\r#b\""}, 0},
	} {
		t.Run(c.name, func(t *testing.T) {
			_, issues := lintOne(t, "sh", withLines(actRowAt, c.lines...))
			wantOneAt(t, issues, cleanActLine(actRowAt+c.at), actTrailMsg)
		})
	}
}

// TestActScanReadsLikeShell — rows where the scanner once read the block
// differently from bash or zsh, so a # straight after an operator passed while
// interactive zsh ran the text after it. Each now fails: the floor flags the #
// line, and the scanner fails strict on the construct it does not model — a
// heredoc whose body would start inside a span opened after its operator, or
// whose span closes before its body; a << in arithmetic; a $ in a heredoc
// delimiter; a ${ followed by a blank or |; and $[ ]. $$ is one parameter, so
// "$${" opens no span.
func TestActScanReadsLikeShell(t *testing.T) {
	const tail = "  echo dry;#;echo live"
	trail := func(at int) actWant { return actWant{at, actTrailMsg} }
	unmod := actWant{0, actUnmodMsg}
	for _, c := range []struct {
		name  string
		lines []string
		want  []actWant
	}{
		{"heredoc whose span closes on its line", []string{"  x=$(cat <<EOF)", tail, "EOF"}, []actWant{unmod, trail(1)}},
		{"heredoc whose span closes on its line, backtick", []string{"  x=`cat <<EOF`", tail, "EOF"}, []actWant{unmod, trail(1)}},
		{"heredoc span closed, another opened", []string{"  x=$(cat <<EOF) y=$(", tail, "  )", "EOF"}, []actWant{unmod, trail(1)}},
		{"span left open after a heredoc", []string{"  cat <<EOF $(", tail, "  )", "x", "EOF"}, []actWant{unmod, trail(1)}},
		{"span with code left open after a heredoc", []string{"  cat <<true $(echo a", tail, "true", "  )", "body", "true"}, []actWant{unmod, trail(1)}},
		{"backtick left open after a heredoc", []string{"  cat <<true `echo a", tail, "true", "  `", "body", "true"}, []actWant{unmod, trail(1)}},
		{"shift in arithmetic with a closing line", []string{"  x=$((1<<2))", tail, "2"}, []actWant{unmod, trail(1)}},
		{"shift in an arithmetic command", []string{"  (( X = 1 << 2 ))", tail, "2"}, []actWant{unmod, trail(1)}},
		{"shift by a name in arithmetic", []string{"  echo $(( 1 << n ))", tail, "n"}, []actWant{unmod, trail(1)}},
		{"shift in quoted arithmetic", []string{`  echo "$((1<<2))"`, tail, "2"}, []actWant{unmod, trail(1)}},
		{"old-style arithmetic", []string{"  echo $[1<<2]", tail, "2"}, []actWant{unmod, trail(1)}},
		{"process id before a brace", []string{`  echo "$${"`, tail, `  echo "}"`}, []actWant{trail(1)}},
		{"backtick in a delimiter", []string{"  cat <<EOF`x`", tail, "EOF`x`"}, []actWant{unmod, trail(1)}},
		{"ANSI-C quoted delimiter", []string{"  cat <<$'EOF'", "x", "EOF", tail, "$EOF"}, []actWant{unmod, trail(3)}},
		{"bash 5.3 command substitution", []string{`  x="${ echo dry;#;echo live`, `  }"`}, []actWant{unmod, trail(0)}},
		{"bash 5.3 REPLY substitution", []string{`  x="${| REPLY=a;}"`}, []actWant{unmod}},
		{"arithmetic command closed by a single )", []string{"  ((echo a) | cat)"}, []actWant{unmod}},
		{"arithmetic expansion closed by a single )", []string{"  x=$((echo a) | cat)"}, []actWant{unmod}},
	} {
		t.Run(c.name, func(t *testing.T) {
			_, issues := lintOne(t, "sh", withLines(actRowAt, c.lines...))
			wantIssues(t, issues, cleanActLine(actRowAt), c.want...)
		})
	}
}

// TestActBlockLintFlagsMissingGuard — a block whose first non-blank line is
// not the zsh guard is flagged, whether the guard is missing or comes after a
// comment line or a code line. After a code line the header comment that
// follows the guard stands after code, so its placement is flagged too.
func TestActBlockLintFlagsMissingGuard(t *testing.T) {
	moved := func(after string) []string {
		return append([]string{after, actBlockGuardLine}, cleanActLines[1:]...)
	}
	guard := actWant{0, "zsh comment guard"}
	for _, c := range []struct {
		name  string
		lines []string
		want  []actWant
	}{
		{"guard missing", cleanActLines[1:], []actWant{guard}},
		{"guard after a comment line", moved("# 0. plain text first"), []actWant{guard}},
		{"guard after a code line", moved("true"), []actWant{guard, {2, actPlaceMsg}}},
	} {
		t.Run(c.name, func(t *testing.T) {
			_, issues := lintOne(t, "sh", c.lines)
			wantIssues(t, issues, cleanActLine(0), c.want...)
		})
	}
}

// TestActBlockLintFlagsBareName — the act function must carry a per-act name;
// the bare driver_act is flagged at its definition.
func TestActBlockLintFlagsBareName(t *testing.T) {
	lines := append([]string{}, cleanActLines...)
	lines[2] = "driver_act() ("
	lines[len(lines)-1] = "driver_act"
	_, issues := lintOne(t, "sh", lines)
	wantOneAt(t, issues, cleanActLine(2), "name it per act")
}

// TestActBlockLintFlagsUnsafeRead — every read clears its variable first and
// stops on a failed read; the shape that keeps an inherited value is flagged.
func TestActBlockLintFlagsUnsafeRead(t *testing.T) {
	for _, c := range []struct{ name, line string }{
		{"no clear, no stop", `  read -rs T; [ -n "$T" ] || exit 1`},
		{"clear but no stop on failure", `  T=; read -rs T; [ -n "$T" ] || exit 1`},
		{"stop but no clear", `  read -rs T || exit 1`},
		{"clears a different name", `  U=; read -rs T || exit 1`},
		{"read after then", `  if true; then read -r T; fi`},
		{"failure branch continues", `  T=; read -rs T || true`},
		{"failure branch only echoes", `  T=; read -rs T || echo failed`},
		{"no silent option", `  T=; read -r T || exit 1`},
		{"extra option", `  T=; read -rs -p x T || exit 1`},
		{"negated read after if", `  if ! read -rs T; then exit 1; fi`},
		{"read after if", `  if read -rs T; then :; fi`},
		{"read after while", `  while read -rs T; do :; done`},
		{"read in a pipe", `  echo x | read -rs T`},
		{"exit 0 on failure", `  T=; read -rs T || exit 0`},
		{"exit piped away", `  T=; read -rs T || exit 1 | cat`},
		{"group with no exit", `  T=; read -rs T || { echo failed; }`},
		{"group not ending in exit", `  T=; read -rs T || { exit 1 && true; }`},
		{"clear is conditional", `  false && T=; read -rs T || exit 1`},
		{"stop only leaves a subshell", `  (T=; read -rs T || exit 1)`},
		{"clear glued to a keyword", `  doT=; read -rs T || exit 1`},
		{"read in a backtick span", "  X=`read -rs T`"},
		// The clear inside a subshell or a piped group opened on the same line: the exit
		// ends only that subshell, and the act carries on with an inherited value.
		{"shape inside a same-line subshell", `  ( :; T=; read -rs T || exit 1; )`},
		{"shape inside a command substitution", `  X=$(:; T=; read -rs T || exit 1; echo "$T")`},
		{"shape inside a command substitution in double quotes", `  echo "$(:; T=; read -rs T || exit 1)"`},
		{"shape inside a backtick span", "  X=`:; T=; read -rs T || exit 1`"},
		{"shape in a group piped into", `  echo x | { T=; read -rs T || exit 1; }`},
		{"shape in a group piped from", `  { T=; read -rs T || exit 1; } | cat`},
		{"shape in a backgrounded group", `  { T=; read -rs T || exit 1; } &`},
		{"shape in an if piped into", `  echo x | if true; then T=; read -rs T || exit 1; fi`},
		// The failure group run in a pipeline or in the background: its exit ends only a
		// subshell.
		{"failure group piped away", `  T=; read -rs T || { exit 1; } | cat`},
		{"failure group backgrounded", `  T=; read -rs T || { exit 1; } &`},
		{"failure group followed by more of the list", `  T=; read -rs T || { exit 1; } || true`},
		// A read word that a punctuation character ends, not a blank.
		{"read in a command substitution", `  X=$(read)`},
		{"read in a backtick span, bare", "  X=`read`"},
		{"read in a subshell", `  (read)`},
		{"read with a redirect", `  read<&0`},
		{"read backgrounded", `  read&`},
		{"read piped", `  read|cat`},
		{"read in a quoted command substitution", `  echo "$(read -r X)"`},
		// A word that a shell runs as read once its quotes and backslashes are removed.
		{"read behind a backslash", `  \read -r T`},
		{"read in double quotes", `  "read" -r T`},
		{"read split by empty quotes", `  r''ead -r T`},
		// An exit status the shell reports as 0.
		{"exit 256 is status 0", `  T=; read -rs T || exit 256`},
	} {
		t.Run(c.name, func(t *testing.T) {
			_, issues := lintOne(t, "sh", withLine(4, c.line))
			wantOneAt(t, issues, cleanActLine(4), "NAME=; read -rs NAME ||")
		})
	}
}

// withLines returns cleanActLines with lines inserted at index i.
func withLines(i int, lines ...string) []string {
	out := append([]string{}, cleanActLines[:i]...)
	out = append(out, lines...)
	return append(out, cleanActLines[i:]...)
}

// TestActBlockLintFlagsUnsafeReadAcrossLines — the read check reads the whole block, not one
// line: a subshell, pipeline or background that a read sits in is seen wherever it opens or
// closes, and a read outside the act function, which the dry paste would run, is flagged.
func TestActBlockLintFlagsUnsafeReadAcrossLines(t *testing.T) {
	for _, c := range []struct {
		name  string
		at    int
		lines []string
		bad   int // index in lines of the read the issue names
	}{
		{"subshell opened on an earlier line", 4, []string{"  (", "  T=; read -rs T || exit 1", "  )"}, 1},
		{"group piped on a later line", 4, []string{"  {", "  T=; read -rs T || exit 1", "  } | cat"}, 1},
		{"group piped into from an earlier line", 4, []string{"  echo x |", "  {", "  T=; read -rs T || exit 1", "  }"}, 2},
		{"if backgrounded on a later line", 4, []string{"  if true; then", "  T=; read -rs T || exit 1", "  fi &"}, 1},
		{"clear after a pipe on an earlier line", 4, []string{"  echo x |", "  T=; read -rs T || exit 1"}, 1},
		{"failure group piped on a later line", 4, []string{"  T=; read -rs T || {", "  exit 1; } | cat"}, 0},
		{"read in another function", 4, []string{"  ask() { T=; read -rs T || exit 1; }"}, 0},
		{"read outside the act function", 2, []string{"T=; read -rs T || exit 1"}, 0},
	} {
		t.Run(c.name, func(t *testing.T) {
			_, issues := lintOne(t, "sh", withLines(c.at, c.lines...))
			wantOneAt(t, issues, cleanActLine(c.at+c.bad), "NAME=; read -rs NAME ||")
		})
	}
}

// TestActBlockLintSafeReadAcrossLines — the shape passes inside an if, a loop or a group
// that the act function runs in its own shell, wherever those open and close.
func TestActBlockLintSafeReadAcrossLines(t *testing.T) {
	for _, c := range [][]string{
		{"  if true; then :", "  else printf 'token: '", "    T=; read -rs T || { echo; echo \"no token read; nothing changed\" >&2; exit 1; }", "  fi"},
		{"  {", "  T=; read -rs T || exit 1", "  } >/dev/null"},
		{"  while true; do", "  T=; read -rs T || exit 1", "  break; done"},
		{"  T=; read -rs T || {", "  echo; exit 1; }"},
	} {
		t.Run(strings.Join(c, " / "), func(t *testing.T) {
			_, issues := lintOne(t, "sh", withLines(4, c...))
			if len(issues) != 0 {
				t.Fatalf("issues=%+v, want none", issues)
			}
		})
	}
}

// TestActBlockLintSafeReadForms — the documented read shape passes in each of
// its spellings, and the word read inside quotes or a longer word is no read.
func TestActBlockLintSafeReadForms(t *testing.T) {
	for _, line := range []string{
		`  T=; read -rs T || exit 1`,
		`  T=; read -sr T || exit 2`,
		`  T=; read -r -s T || exit 1; [ -n "$T" ] || exit 1`,
		`  T=; read -rs T || { echo; echo "no token read; nothing changed" >&2; exit 1; }`,
		`  echo "would read it; read -r X"`,
		`  echo 'read T'`,
		`  run gh api repos/o/r/readme --jq .already_read`,
		`  T=; read -rs T || exit 1; U=; read -rs U || exit 1`,
		`  T=; read -rs T || exit 255`,
		`  { T=; read -rs T || exit 1; }`,
		`  echo "$(printf x) would read it, read -r X"`,
	} {
		t.Run(line, func(t *testing.T) {
			_, issues := lintOne(t, "sh", withLine(4, line))
			if len(issues) != 0 {
				t.Fatalf("issues=%+v, want none", issues)
			}
		})
	}
}

// TestActBlockLintFenceCloser — only a bare run of the opening character, at
// least as long, closes the fence; a shorter run or the other character inside
// the block is content, so a bad comment after it is still checked. As content,
// a run of three backticks is also an unclosed backtick span to a shell.
func TestActBlockLintFenceCloser(t *testing.T) {
	for _, c := range []struct {
		name, open, inner, close string
		extra                    []actWant
	}{
		{"shorter run inside a longer fence", "````sh", "```", "````", []actWant{{0, actOpenMsg}}},
		{"tilde run inside a backtick fence", "```sh", "~~~", "```", nil},
	} {
		t.Run(c.name, func(t *testing.T) {
			body := strings.Join(append(append([]string{
				"---", "name: some-desk", "description: x", "---", "", c.open,
			}, append(append([]string{}, cleanActLines...), c.inner, "# bad; comment")...), c.close, ""), "\n")
			root := t.TempDir()
			writeSkill(t, root, "some-desk", body)
			_, issues, err := ActBlockIssues(root)
			if err != nil {
				t.Fatalf("ActBlockIssues: %v", err)
			}
			wantIssues(t, issues, cleanActLine(len(cleanActLines)), append([]actWant{{1, "comment carries"}, {1, actPlaceMsg}}, c.extra...)...)
		})
	}
}

// TestActBlockLintScope — only an sh fence that defines the act function is an
// act block: another sh fence, or the act text quoted in a text fence, is not
// checked. With no act block anywhere the lint is could-not-check.
func TestActBlockLintScope(t *testing.T) {
	root := t.TempDir()
	body := actSkill("sh", "# not an act; any text here (fine)", "echo hi") +
		strings.Join(append(append([]string{actFence + "text"}, "driver_act_1() ( # quoted; not run )"), actFence, ""), "\n")
	writeSkill(t, root, "some-desk", body)
	blocks, issues, err := ActBlockIssues(root)
	if err == nil {
		t.Fatalf("blocks=%d issues=%+v: want could-not-check with no act block in the tree", blocks, issues)
	}
}

// TestActBlockLintRealTree — the plugin tree's own act blocks keep both rules,
// and there is at least one (the ask-decision example).
func TestActBlockLintRealTree(t *testing.T) {
	blocks, issues, err := ActBlockIssues("../..")
	if err != nil {
		t.Fatalf("could-not-check: %v", err)
	}
	for _, is := range issues {
		t.Errorf("%s: %s", is.Path, is.Msg)
	}
	if blocks < 1 {
		t.Fatalf("found %d act blocks, want at least the ask-decision example", blocks)
	}
}

// TestActCommentPlacement — on a first zsh paste a # line is a command that
// fails. After && or ||, in an if, elif, while or until condition, last in a
// group, function or case arm, inside an array, or after a line continuation,
// that failure or its words change what runs. So a # line stands only in the
// header, straight after the guard: every other one is refused, between two
// complete commands as well, and a #-led line inside a heredoc body too.
func TestActCommentPlacement(t *testing.T) {
	place := func(at int) actWant { return actWant{at, actPlaceMsg} }
	trail := func(at int) actWant { return actWant{at, actTrailMsg} }
	for _, c := range []struct {
		name  string
		lines []string
		want  []actWant
	}{
		{"after an or list", []string{`  [ "$DRY_RUN" = 1 ] ||`, "    # live only", "    echo LIVE"}, []actWant{place(1)}},
		{"after an and list", []string{"  false &&", "  # note", "  echo LIVE"}, []actWant{place(1)}},
		{"last in an if condition", []string{`  if [ "$DRY_RUN" = 1 ]`, "  # dry path prints", "  then echo dry", "  else echo LIVE", "  fi"}, []actWant{place(1)}},
		{"last in a negated if condition", []string{"  if ! false", "  # note", "  then echo dry; else echo LIVE; fi"}, []actWant{place(1)}},
		{"last in an elif condition", []string{"  if false; then echo a", "  elif true", "  # note", "  then echo dry; else echo LIVE; fi"}, []actWant{place(2)}},
		{"last in an until condition", []string{"  until true", "  # note", "  do echo LIVE; break; done"}, []actWant{place(1)}},
		{"last in a while condition", []string{"  while false", "  # note", "  do echo LIVE; done"}, []actWant{place(1)}},
		{"last in a tested subshell", []string{"  (", "  true", "  # note", "  ) || echo LIVE"}, []actWant{place(2)}},
		{"last in a case arm", []string{"  case x in x) echo a", "  # note", "  ;; esac"}, []actWant{place(1)}},
		{"inside an array", []string{"  x=(a", "  # LIVE", "  b)"}, []actWant{place(1)}},
		{"after a line continuation, unindented", []string{`  NOTE=1\`, "# echo LIVE"}, []actWant{place(1)}},
		{"after a line continuation, indented", []string{`  NOTE=1\`, "  # echo LIVE"}, []actWant{trail(1), place(1)}},
		{"after a closed descriptor and a continuation", []string{`  echo dry 2>&-\`, "# LIVE"}, []actWant{place(1)}},
		{"between two complete commands", []string{"  echo a", "  # note", "  echo b"}, []actWant{place(1)}},
		{"last in the act function", []string{"  # last note"}, []actWant{place(0)}},
		{"in a heredoc body", []string{"  cat <<'EOF'", "# data", "EOF"}, []actWant{place(1)}},
	} {
		t.Run(c.name, func(t *testing.T) {
			_, issues := lintOne(t, "sh", withLines(actRowAt, c.lines...))
			wantIssues(t, issues, cleanActLine(actRowAt), c.want...)
		})
	}
	t.Run("after the call", func(t *testing.T) {
		_, issues := lintOne(t, "sh", withLines(len(cleanActLines), "# done"))
		wantIssues(t, issues, cleanActLine(len(cleanActLines)), place(0))
	})
	t.Run("before the act function, after code", func(t *testing.T) {
		_, issues := lintOne(t, "sh", withLines(1, "X=1", "# note"))
		wantIssues(t, issues, cleanActLine(1), place(1), place(2))
	})
	t.Run("header run with a blank line and a fill marker", func(t *testing.T) {
		_, issues := lintOne(t, "sh", withLines(2, "", "# fill: the publish token, typed at a hidden prompt on the live call"))
		if len(issues) != 0 {
			t.Fatalf("issues=%+v, want none for comments in the header", issues)
		}
	})
}
