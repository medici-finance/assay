package main

import (
	"fmt"
	"strings"
	"testing"
)

// actblock_test.go — the act-block lint (actblock.go): every example Act block
// under plugins/ opens with the zsh comment guard and keeps its comment lines
// free of shell metacharacters.

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
	"  # 1. push the parked branch. git push has its own --dry-run.",
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
// sits at the top level or indented inside the act function, as real step
// comments are. Each single-character row carries one forbidden character
// only, so allowing that one character turns its row green.
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
			wantOneAt(t, issues, cleanActLine(4), "comment carries")
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

// TestActBlockLintFlagsMissingGuard — a block whose first non-blank line is
// not the zsh guard is flagged, whether the guard is missing or comes after a
// comment line or a code line.
func TestActBlockLintFlagsMissingGuard(t *testing.T) {
	moved := func(after string) []string {
		return append([]string{after, actBlockGuardLine}, cleanActLines[1:]...)
	}
	for _, c := range []struct {
		name  string
		lines []string
	}{
		{"guard missing", cleanActLines[1:]},
		{"guard after a comment line", moved("# 0. plain text first")},
		{"guard after a code line", moved("true")},
	} {
		t.Run(c.name, func(t *testing.T) {
			_, issues := lintOne(t, "sh", c.lines)
			wantOneAt(t, issues, cleanActLine(0), "zsh comment guard")
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
		{"read outside the act function", 1, []string{"T=; read -rs T || exit 1"}, 0},
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
// the block is content, so a bad comment after it is still checked.
func TestActBlockLintFenceCloser(t *testing.T) {
	for _, c := range []struct{ name, open, inner, close string }{
		{"shorter run inside a longer fence", "````sh", "```", "````"},
		{"tilde run inside a backtick fence", "```sh", "~~~", "```"},
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
			wantOneAt(t, issues, cleanActLine(len(cleanActLines)+1), "comment carries")
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
