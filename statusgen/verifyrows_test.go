package main

import (
	"os"
	"strings"
	"testing"
)

// TestSplitRowEscaped pins the escape-aware cell split. The package-wide splitRow
// splits on every `|` byte, which shreds any row carrying a `\|` — exactly the
// rows this lint hunts. These cases would each be mis-split by splitRow.
func TestSplitRowEscaped(t *testing.T) {
	tests := []struct {
		name string
		line string
		want []string
	}{
		{
			name: "plain row",
			line: "| 1 | `go test` | 0 |",
			want: []string{" 1 ", " `go test` ", " 0 "},
		},
		{
			name: "escaped pipes stay inside their cell and keep their backslash",
			line: `| 2 | ` + "`ls x \\| grep -cE \"a\\|b\"`" + ` | 0 |`,
			want: []string{" 2 ", " `ls x \\| grep -cE \"a\\|b\"` ", " 0 "},
		},
		{
			name: "row without outer delimiters",
			line: "a | b",
			want: []string{"a ", " b"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := splitRowEscaped(tc.line)
			if len(got) != len(tc.want) {
				t.Fatalf("splitRowEscaped(%q) = %d cells %q, want %d %q", tc.line, len(got), got, len(tc.want), tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("cell %d = %q, want %q", i, got[i], tc.want[i])
				}
			}
		})
	}
}

// TestSplitRowEscapedPreservesBackslash is the regression pin for the bug this
// lint nearly shipped with: resolving `\|` to `|` at split time left the checker
// structurally unable to see the defect it reports on.
func TestSplitRowEscapedPreservesBackslash(t *testing.T) {
	cells := splitRowEscaped("| 2 | `grep -ciE \"arm64\\|amd64\"` | 1 |")
	if len(cells) < 2 || !strings.Contains(cells[1], `\|`) {
		t.Fatalf("the `\\|` must survive the split verbatim; got %q", cells)
	}
}

func TestCodeSpan(t *testing.T) {
	tests := []struct{ name, cell, want string }{
		{"bare code span", "`go test`", "go test"},
		{"code span with trailing prose", "`grep -c X f` (post apply)", "grep -c X f"},
		{"prose only", "the documented run command", "the documented run command"},
		{"double-tick fence", "``echo `x` ``", "echo `x`"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := codeSpan(tc.cell); got != tc.want {
				t.Errorf("codeSpan(%q) = %q, want %q", tc.cell, got, tc.want)
			}
		})
	}
}

// TestTokenizeCommandPipeContext pins the one markdown-aware decision: an
// UNQUOTED `\|` is a shell pipe (that is how a pipeline must be authored inside a
// GFM table cell), while a QUOTED `\|` is pattern content.
func TestTokenizeCommandPipeContext(t *testing.T) {
	toks := tokenizeCommand(`ls x \| grep -cE "a\|b"`)
	var ops, quoted []string
	for _, tk := range toks {
		if tk.op {
			ops = append(ops, tk.text)
		}
		if tk.quoted {
			quoted = append(quoted, tk.text)
		}
	}
	if len(ops) != 1 || ops[0] != "|" {
		t.Errorf("unquoted `\\|` should tokenize as one pipeline op; got %q", ops)
	}
	if len(quoted) != 1 || quoted[0] != `a\|b` {
		t.Errorf("quoted `\\|` should stay verbatim in the pattern token; got %q", quoted)
	}
}

// TestEscapedPipeOutsideBracket covers rule 1's core predicate, including the
// bracket-expression escape hatch.
func TestEscapedPipeOutsideBracket(t *testing.T) {
	tests := []struct {
		name string
		pat  string
		want bool
	}{
		// True positives — mis-escaped alternation.
		{"simple alternation", `alpha\|beta`, true},
		{"alternation inside a group", `\[(F\|I)-[0-9]+\]\(`, true},
		{"alternation after a bracket expression", `background:[[:space:]]*(#fff\|white)`, true},

		// False positives that must NOT fire.
		{"plain pattern, no pipe", `arm64`, false},
		{"real ERE alternation (already correct)", `alpha|beta`, false},
		{"literal pipe via a bracket class — the sanctioned escape hatch", `a[\|]b`, false},
		{"bracket class of pipe and dash", `[\|-]`, false},
		{"escaped dot, not a pipe", `0\.28\.0`, false},
		{"a `]` right after `[` is literal, not a close", `[]\|]`, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := escapedPipeOutsideBracket(tc.pat); got != tc.want {
				t.Errorf("escapedPipeOutsideBracket(%q) = %v, want %v", tc.pat, got, tc.want)
			}
		})
	}
}

func TestExpectsZeroCount(t *testing.T) {
	tests := []struct {
		name   string
		expect string
		want   bool
	}{
		{"bare zero", "0", true},
		{"zero with prose", "0 (loose copies retired)", true},
		{"zero in a code span", "`0` — no brief still triggers the NOTICE", true},
		{"zero with a trailing comma", "0, forbidden numbers absent", true},

		// The count is the OUTPUT, not a zero gate → must not fire.
		{"at-least three", "≥ 3 (options present)", false},
		{"at-least one", "≥1", false},
		{"a positive exact count", "2", false},
		{"empty", "", false},
		{"prose", "present", false},
		{"ten", "10 (≥1)", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := expectsZeroCount(tc.expect); got != tc.want {
				t.Errorf("expectsZeroCount(%q) = %v, want %v", tc.expect, got, tc.want)
			}
		})
	}
}

func TestGrepCalls(t *testing.T) {
	tests := []struct {
		name         string
		cmd          string
		wantN        int
		wantExtended bool
		wantCount    bool
		wantPattern  string
		wantLast     bool
	}{
		{
			name: "bundled short flags", cmd: `grep -ciE "a\|b" f.md`,
			wantN: 1, wantExtended: true, wantCount: true, wantPattern: `a\|b`, wantLast: true,
		},
		{
			name: "egrep implies -E", cmd: `egrep -c "a\|b" f.md`,
			wantN: 1, wantExtended: true, wantCount: true, wantPattern: `a\|b`, wantLast: true,
		},
		{
			name: "long flags", cmd: `grep --extended-regexp --count "a\|b" f.md`,
			wantN: 1, wantExtended: true, wantCount: true, wantPattern: `a\|b`, wantLast: true,
		},
		{
			name: "-e takes the pattern, the operand is a file", cmd: `grep -cE -e "a\|b" f.md`,
			wantN: 1, wantExtended: true, wantCount: true, wantPattern: `a\|b`, wantLast: true,
		},
		{
			name: "basic grep is not extended", cmd: `grep -c "a\|b" f.md`,
			wantN: 1, wantExtended: false, wantCount: true, wantPattern: `a\|b`, wantLast: true,
		},
		{
			name: "grep after a && is still last", cmd: `test -f f.md && grep -ciE "a\|b" f.md`,
			wantN: 1, wantExtended: true, wantCount: true, wantPattern: `a\|b`, wantLast: true,
		},
		{
			name: "grep inside a brace group with a bang", cmd: `sed '/x/d' f.md \| { ! grep -qE "a\|b"; }`,
			wantN: 1, wantExtended: true, wantCount: false, wantPattern: `a\|b`, wantLast: true,
		},
		{
			name: "grep piped into awk is NOT last", cmd: `grep -rEc "a\|b" *.md \| awk -F: '{t+=$2}'`,
			wantN: 1, wantExtended: true, wantCount: true, wantPattern: `a\|b`, wantLast: false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := grepCalls(tokenizeCommand(tc.cmd))
			if len(got) != tc.wantN {
				t.Fatalf("grepCalls(%q) = %d calls, want %d", tc.cmd, len(got), tc.wantN)
			}
			g := got[0]
			if g.extended != tc.wantExtended {
				t.Errorf("extended = %v, want %v", g.extended, tc.wantExtended)
			}
			if g.count != tc.wantCount {
				t.Errorf("count = %v, want %v", g.count, tc.wantCount)
			}
			if g.last != tc.wantLast {
				t.Errorf("last = %v, want %v", g.last, tc.wantLast)
			}
			if len(g.patterns) == 0 || g.patterns[0] != tc.wantPattern {
				t.Errorf("patterns = %q, want first %q", g.patterns, tc.wantPattern)
			}
		})
	}
}

// TestGrepCallsIgnoresNonGrep pins that a cell with no grep yields no calls — the
// tokenizer must not invent one from a filename or an argument.
func TestGrepCallsIgnoresNonGrep(t *testing.T) {
	for _, cmd := range []string{`go test`, `go run ./tools/statusgen --lint`, `rg -c "a\|b" .`} {
		if got := grepCalls(tokenizeCommand(cmd)); len(got) != 0 {
			t.Errorf("grepCalls(%q) = %v, want none", cmd, got)
		}
	}
}

// TestGoTestRunPatterns pins extraction of the RE2 selector arguments from `go
// test` invocations, across the `-run PAT`, `-run=PAT` and `--run=PAT` forms,
// and confirms a non-`go test` command yields nothing. `-bench` is the half
// #374 named and the original rule missed: a `-bench` pattern that matches
// nothing prints "no benchmarks to run" and exits 0, exactly like `-run`.
func TestGoTestRunPatterns(t *testing.T) {
	tests := []struct {
		name string
		cmd  string
		want []goTestPattern
	}{
		{"space form", `go test ./tools/statusgen/ -run 'Dora\|Weekly\|Artifact'`, []goTestPattern{{"-run", `Dora\|Weekly\|Artifact`}}},
		{"single-dash equals form", `go test ./tools/statusgen/ -run='A\|B'`, []goTestPattern{{"-run", `A\|B`}}},
		{"double-dash equals form", `go test ./tools/statusgen/ --run='A\|B'`, []goTestPattern{{"-run", `A\|B`}}},
		{"unambiguous single token", `go test ./tools/statusgen/ -run Dora`, []goTestPattern{{"-run", "Dora"}}},
		{"-bench is the same RE2 surface", `go test -bench 'A\|B' ./...`, []goTestPattern{{"-bench", `A\|B`}}},
		{"-fuzz too", `go test -fuzz='A\|B' ./...`, []goTestPattern{{"-fuzz", `A\|B`}}},
		{"-run and -bench together", `go test -run X -bench Y ./...`, []goTestPattern{{"-run", "X"}, {"-bench", "Y"}}},
		{"no -run flag at all", `go test ./tools/statusgen/`, nil},
		{"a lookalike flag is not -run", `go test -count=1 -race ./...`, nil},
		{"not a go test invocation", `go build ./tools/...`, nil},
		{"not go at all", `make test`, nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := goTestRunPatterns(tokenizeCommand(tc.cmd))
			if len(got) != len(tc.want) {
				t.Fatalf("goTestRunPatterns(%q) = %+v, want %+v", tc.cmd, got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("pattern %d = %+v, want %+v", i, got[i], tc.want[i])
				}
			}
		})
	}
}

// TestExpectedNonZeroExits pins the Expect-cell reader rule 6 (#493) stands
// on: it must find the exit-code assertion in every spelling the corpus uses,
// and must NOT read a count, a bare number, or exit 0 as one.
func TestExpectedNonZeroExits(t *testing.T) {
	tests := []struct {
		name   string
		expect string
		want   []string
	}{
		{"rc= form", "rc=5", []string{"5"}},
		{"rc with spaces", "`rc = 6` (could-not-check)", []string{"6"}},
		{"exit N", "exit 3 (disabled)", []string{"3"}},
		{"exit code N", "exit code 4", []string{"4"}},
		{"exit status N", "exit status 5 — refused", []string{"5"}},
		{"$? form", "$? = 6", []string{"6"}},
		{"exits N", "exits 2", []string{"2"}},
		{"exit 1 counts — go run also exits 1 on a COMPILE failure", "exit 1", []string{"1"}},
		{"two codes in one cell", "exit 5 on refusal, rc=6 when unverifiable", []string{"5", "6"}},

		{"exit 0 is not a failure code", "exit 0", nil},
		// publication/05 row 6b — the BUILT-BINARY fix for #493. Its Expect
		// cell asserts `0` and then explains the defect, naming 1, 5 and 6.
		// Reading the commentary would fire the rule on its own remedy.
		{"commentary after the assertion does not bind", "`0`. This row exists because `go run` exits `1` for every non-zero exit; rows 7,9,10 turn on telling `5` from `6`", nil},
		{"an amendment note after an em dash does not bind", "exit 0 — measured: binary off PATH → rc=0, the capture-to-file form → rc=127", nil},
		{"rc=0 is not a failure code", "`rc=0`", nil},
		{"a bare count is not an exit code", "≥ 4", nil},
		{"prose", "the board renders", nil},
		{"non-zero without a specific code is not a specific code", "non-zero", nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := expectedNonZeroExits(tc.expect)
			if strings.Join(got, ",") != strings.Join(tc.want, ",") {
				t.Errorf("expectedNonZeroExits(%q) = %q, want %q", tc.expect, got, tc.want)
			}
		})
	}
}

// TestGoRunInvoked pins the `go run` detector: `run` must be the `go`
// subcommand, not a directory called run or a quoted word.
func TestGoRunInvoked(t *testing.T) {
	tests := []struct {
		name string
		cmd  string
		want bool
	}{
		{"plain go run", `go run ./cmd/tool --flag`, true},
		{"go run after a &&", `cd statusgen && go run . --lint`, true},
		{"go run in a pipeline stage", `go run ./cmd/t \| head -3`, true},
		{"go build then run the binary — the fix", `go build -o /tmp/t ./cmd/t && /tmp/t; echo $?`, false},
		{"a path segment named run is not the subcommand", `./scripts/run ./cmd/tool`, false},
		{"go test is not go run", `go test ./...`, false},
		{"quoted go run inside a grep pattern", `grep -c "go run" docs/brief-rules.md`, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := goRunInvoked(tokenizeCommand(tc.cmd)); got != tc.want {
				t.Errorf("goRunInvoked(%q) = %v, want %v", tc.cmd, got, tc.want)
			}
		})
	}
}

// TestPipeOutsideBracket pins rule 7's predicate (#262). It must fire on BOTH
// spellings of the defect — the source `\|` and the rendered bare `|` — while
// leaving the bracket-class escape hatch alone.
func TestPipeOutsideBracket(t *testing.T) {
	tests := []struct {
		name string
		pat  string
		want bool
	}{
		{"bare pipe (the rendered form)", `alpha|beta|gamma`, true},
		{"escaped pipe (the source form)", `alpha\|beta`, true},
		{"pipe inside a group", `(a|b)c`, true},

		{"no pipe at all", `arm64`, false},
		{"literal pipe via a bracket class", `a[\|]b`, false},
		{"bare pipe inside a bracket class", `a[|]b`, false},
		{"escaped dot is not a pipe", `0\.28\.0`, false},
		{"a `]` right after `[` is literal, not a close", `[]|]`, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := pipeOutsideBracket(tc.pat); got != tc.want {
				t.Errorf("pipeOutsideBracket(%q) = %v, want %v", tc.pat, got, tc.want)
			}
		})
	}
}

// TestTruncatedCodeSpan pins rule 8's detector (#374): a Command cell cut by a
// RAW pipe leaves its code span unterminated.
func TestTruncatedCodeSpan(t *testing.T) {
	tests := []struct {
		name string
		cell string
		want bool
	}{
		{"a raw pipe cut the span", " `go test ./x -run 'Dora", true},
		{"a complete code span", " `go test ./x -run Dora` ", false},
		{"prose cell, no span", " observed by the reviewer ", false},
		{"two spans in one cell", " `a` then `b` ", false},
		{"a ``…`` fence", " ``echo `x` `` ", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := truncatedCodeSpan(tc.cell); got != tc.want {
				t.Errorf("truncatedCodeSpan(%q) = %v, want %v", tc.cell, got, tc.want)
			}
		})
	}
}

// TestMovingRefBases pins rule 9 (#639): a base that moves independently of
// the tree under test is flagged; a computed or pinned base is not.
func TestMovingRefBases(t *testing.T) {
	tests := []struct {
		name string
		cmd  string
		want []string
	}{
		{"the measured instance", `statusgen --consumers --base origin/main`, []string{"origin/main"}},
		{"equals form", `statusgen --consumers --base=origin/main`, []string{"origin/main"}},
		{"the long ref name is the SAME moving ref", `statusgen --base refs/remotes/origin/main`, []string{"refs/remotes/origin/main"}},
		{"a bare branch name", `statusgen --base main`, []string{"main"}},
		{"a two-dot git range", `git log --oneline origin/main..HEAD`, []string{"origin/main"}},
		{"a three-dot git range", `git diff --stat origin/master...HEAD`, []string{"origin/master"}},
		{"--since on a moving ref", `deskboard --since origin/main`, []string{"origin/main"}},

		{"a merge-base computation is the FIX, not the defect", `statusgen --consumers --base $(git merge-base origin/main HEAD)`, nil},
		{"a bare merge-base call", `git merge-base origin/main HEAD`, nil},
		{"a pinned SHA", `statusgen --consumers --base c7cd7ef`, nil},
		{"a shell variable holding the base", `statusgen --base "$BASE_SHA"`, nil},
		{"the PR event's own base sha", `statusgen --base ${{ github.event.pull_request.base.sha }}`, nil},
		{"HEAD alone is the tree under test, not a moving end", `git diff --stat HEAD`, nil},
		{"a feature branch is not a shared moving ref", `git log brief/ground-truth-03..HEAD`, nil},
		{"no base at all", `go test ./...`, nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := movingRefBases(tokenizeCommand(tc.cmd))
			if strings.Join(got, ",") != strings.Join(tc.want, ",") {
				t.Errorf("movingRefBases(%q) = %q, want %q", tc.cmd, got, tc.want)
			}
		})
	}
}

// TestGnuOnlyConstructs pins rule 10 (#650): the catalogued GNU-isms are
// named, and their portable equivalents stay silent.
func TestGnuOnlyConstructs(t *testing.T) {
	tests := []struct {
		name string
		cmd  string
		want []string
	}{
		{"process substitution — the measured instance", `grep -cE 'x' <(sed -n '/A/,/B/p' brief.md)`, []string{"`<(…)` process substitution"}},
		{"grep -P", `grep -Pc '(?<=x)y' f.md`, []string{"`grep -P`"}},
		{"GNU sed -i", `sed -i 's/a/b/' f.md`, []string{"`sed -i` with no backup suffix"}},
		{"readlink -f", `readlink -f ./statusgen`, []string{"`readlink -f`"}},
		{"date -d", `date -d '2026-08-13' +%s`, []string{"`date -d`"}},
		{"stat -c", `stat -c %s f.md`, []string{"`stat -c`"}},
		{"tac", `cat f.md \| tac`, []string{"`tac`"}},

		{"the portable pipe form", `sed -n '/A/,/B/p' brief.md \| grep -cE 'x'`, nil},
		{"grep -E is portable", `grep -cE 'a|b' f.md`, nil},
		{"BSD-compatible sed -i", `sed -i '' 's/a/b/' f.md`, nil},
		{"a quoted <( inside a pattern is not process substitution", `grep -c "<(" f.md`, nil},
		{"plain commands", `go test ./... && wc -l < out.txt`, nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			toks := tokenizeCommand(tc.cmd)
			got := gnuOnlyConstructs(tc.cmd, toks, grepCalls(toks))
			if strings.Join(got, ",") != strings.Join(tc.want, ",") {
				t.Errorf("gnuOnlyConstructs(%q) = %q, want %q", tc.cmd, got, tc.want)
			}
			for _, g := range got {
				if gnuOnlySubstitute[g] == "" {
					t.Errorf("construct %q has no portable substitute recorded — a notice that names no fix is a nag, not a check", g)
				}
			}
		})
	}
}

// TestStripQuotedRegions pins the one thing that keeps rule 5 off regexes: a
// quoted span is blanked, so `"<script[^>]*src="` cannot read as a placeholder.
// Length is preserved so offsets into the result stay meaningful.
func TestStripQuotedRegions(t *testing.T) {
	tests := []struct{ name, in, want string }{
		{"double quotes blanked", `grep -e "<a>" f.md`, `grep -e       f.md`},
		{"single quotes blanked", `grep -e '<a>' f.md`, `grep -e       f.md`},
		{"unquoted text survives", `gh issue view <N>`, `gh issue view <N>`},
		{"escaped quote does not end the span", `echo "a\"<b>" x`, `echo          x`},
		{"unterminated quote blanks to end", `echo "<a>`, `echo     `},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := stripQuotedRegions(tc.in)
			if got != tc.want {
				t.Errorf("stripQuotedRegions(%q) = %q, want %q", tc.in, got, tc.want)
			}
			if len(got) != len(tc.in) {
				t.Errorf("length must be preserved: got %d, want %d", len(got), len(tc.in))
			}
		})
	}
}

// TestUnsubstitutedMetavars covers the two decidable placeholder shapes and the
// shell/regex syntax that must NOT be mistaken for them. The class this rule
// does not decide — a placeholder spelled as a plain word — is pinned by the
// last case: `url` is indistinguishable from a literal argument, so it must be
// silent, and the check is documented as a lower bound rather than complete.
func TestUnsubstitutedMetavars(t *testing.T) {
	tests := []struct {
		name string
		cmd  string
		want []string
	}{
		{"single-token metavariable", `gh issue view <N> --json body`, []string{"<N>"}},
		{"multi-word metavariable", `yq eval '.on.schedule' <mm/22 workflow file>`, []string{"<mm/22 workflow file>"}},
		{"two distinct metavariables", `gh issue view <N> --repo <repo>`, []string{"<N>", "<repo>"}},
		{"repeated metavariable reported once", `cp <sha>.txt <sha>.bak`, []string{"<sha>"}},
		{"ellipsis elision", `DESKPUSHGUARD_OFF=1 ...merged-fixture...; echo $?`, []string{"..."}},
		{"unicode ellipsis", `deskpost … --repo r`, []string{"…"}},

		{"quoted HTML regex is not a placeholder", `grep -Eiq -e "<script[^>]*src=" f.html`, nil},
		{"go package wildcard is not an elision", `go test ./tools/example/... -count=1`, nil},
		{"redirection is not a placeholder", `statusgen --lint > o.txt 2>&1 && wc -l < o.txt`, nil},
		{"heredoc is not a placeholder", `cat <<EOF > f.txt`, nil},
		{"process substitution is not a placeholder", `diff <(statusgen --lint) want.txt`, nil},
		{"command substitution is the recommended fix", `test -d reports/$(date +%F)`, nil},
		// The undecidable remainder — desk-tools/10 as originally written.
		{"a bare-word placeholder is NOT decidable and must stay silent", `go run ./cmd/deskpushguard origin url`, nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := unsubstitutedMetavars(tc.cmd)
			if strings.Join(got, ",") != strings.Join(tc.want, ",") {
				t.Errorf("unsubstitutedMetavars(%q)\n  got  %q\n  want %q", tc.cmd, got, tc.want)
			}
		})
	}
}

func TestForcesSuccess(t *testing.T) {
	tests := []struct {
		name string
		cmd  string
		want bool
	}{
		{"|| true neutralises the status", `grep -cE "a" f.md \|\| true`, true},
		{"|| echo neutralises the status", `grep -cE "a" f.md \|\| echo 0`, true},
		{"no || means the status is the gate", `grep -cE "a" f.md`, false},
		{"a plain pipe does not neutralise", `grep -cE "a" f.md \| cat`, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := forcesSuccess(tokenizeCommand(tc.cmd)); got != tc.want {
				t.Errorf("forcesSuccess(%q) = %v, want %v", tc.cmd, got, tc.want)
			}
		})
	}
}

func TestPipelineSwallowsExit(t *testing.T) {
	tests := []struct {
		name string
		cmd  string
		want string
	}{
		{"tee swallows the real exit status", `go test \| tee out.log`, "tee"},
		{"tee at the end of a longer pipeline", `go test \| grep -v x \| tee out.log`, "tee"},
		{"no pipeline", `go test`, ""},
		{"grep as the last stage is a legitimate gate", `go test \| grep -q PASS`, ""},
		{"awk as the last stage is not a known always-zero sink", `grep -c x f \| awk '{print}'`, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := pipelineSwallowsExit(tokenizeCommand(tc.cmd)); got != tc.want {
				t.Errorf("pipelineSwallowsExit(%q) = %q, want %q", tc.cmd, got, tc.want)
			}
		})
	}
}

// verifySection wraps Verify-table rows in a `## Verify` section body.
func verifySection(rows ...string) string {
	return strings.Join(append([]string{
		"| # | Command | Expect |",
		"|---|---------|--------|",
	}, rows...), "\n")
}

// collect runs the row rules over a synthetic Verify section, returning
// `<rule-tag> row <n>` strings. It drives rowFindings — the SAME function
// unfailableRowNotices calls — rather than re-implementing the rule bodies:
// the earlier duplicate meant a rule could be exercised here and silently
// absent from the real entrypoint.
func collect(t *testing.T, section string) []string {
	t.Helper()
	var out []string
	verifyRowTable(section, func(num, cmdCell, expect string) {
		for _, f := range rowFindings(cmdCell, expect) {
			out = append(out, f.rule+" row "+num)
		}
	})
	return out
}

// TestUnfailableRowRules is the end-to-end table over whole Verify rows: each
// case is a realistic row, and the false-positive half is the point — a rule that
// fires on a sound row is as corrosive as the defect it hunts.
func TestUnfailableRowRules(t *testing.T) {
	tests := []struct {
		name string
		row  string
		want []string
	}{
		// ---- true positives -------------------------------------------------
		{
			name: "ere-literal-pipe: mis-escaped alternation in grep -E",
			row:  "| 2 | `grep -ciE \"arm64\\|amd64\\|version\" S1.md` | ≥1 |",
			want: []string{"ere-literal-pipe row 2"},
		},
		{
			name: "ere-literal-pipe: fires through a shell pipe",
			row:  "| 4 | `sed '/x/d' S7.md \\| { ! grep -qE \"x86-only\\|counter-based\"; }` | exit 0 |",
			want: []string{"ere-literal-pipe row 4"},
		},
		{
			name: "grep-zero-count: grep -c gated on a count of zero",
			row:  "| 2 | `grep -c \"medici-stuff\" docs/brand/README.md` | 0 (stale name fixed) |",
			want: []string{"grep-zero-count row 2"},
		},
		{
			name: "grep-zero-count: grep -c as the last stage of a pipeline",
			row:  "| 1 | `go run ./tools/statusgen --lint 2>&1 \\| grep -c gate-why` | `0` — no brief trips the NOTICE |",
			want: []string{"grep-zero-count row 1"},
		},
		{
			name: "both grep rules on one row — the two errors partially cancel",
			row:  "| 2 | `ls ~/.claude/skills/ \\| grep -cE \"the-desk\\|verify-desk\"` | 0 (loose copies retired) |",
			want: []string{"ere-literal-pipe row 2", "grep-zero-count row 2"},
		},
		{
			name: "pipeline-exit-sunk: exit status swallowed by tee",
			row:  "| 1 | `go test \\| tee test.log` | 0 |",
			want: []string{"pipeline-exit-sunk row 1"},
		},
		{
			name: "rE2-literal-pipe: mis-escaped alternation in go test -run (row 5 shape)",
			row:  "| 5 | `go test ./tools/statusgen/ -run 'Dora\\|Weekly\\|Artifact'` | PASS |",
			want: []string{"rE2-literal-pipe row 5"},
		},
		{
			name: "rE2-literal-pipe: fires on the `-run=` form too",
			row:  "| 6 | `go test ./tools/statusgen/ -run='A\\|B'` | PASS |",
			want: []string{"rE2-literal-pipe row 6"},
		},
		{
			// #374's other half: `-bench` compiles the same RE2 surface, and a
			// pattern matching no benchmark prints "no benchmarks to run" and
			// exits 0. The original rule looked only at `-run`.
			name: "rE2-literal-pipe: -bench is the same RE2 surface (#374)",
			row:  "| 7 | `go test -bench 'Board\\|Lint' ./... -benchtime=1x` | PASS |",
			want: []string{"rE2-literal-pipe row 7"},
		},
		{
			name: "unsubstituted-metavar: bare angle-bracket metavariable",
			row:  "| 3 | `gh issue view <N> --json comments` | the App |",
			want: []string{"unsubstituted-metavar row 3"},
		},
		{
			name: "unsubstituted-metavar: a multi-word metavariable (example-app/25 row 3)",
			row:  "| 3 | `yq eval '.on.schedule' <mm/22 workflow file>` | ≥ 3 cron entries |",
			want: []string{"unsubstituted-metavar row 3"},
		},
		{
			name: "unsubstituted-metavar: embedded in a path (example-app/22 row 5)",
			row:  "| 5 | `jq -e 'type==\"array\"' docs/reports/daily/<that-date>/prs.json` | exit 0 |",
			want: []string{"unsubstituted-metavar row 5"},
		},
		{
			name: "unsubstituted-metavar: an angle-bracket metavariable is also a shell redirect",
			row:  "| 1 | `grep -ci 'foreign commit' <batch-fanout SKILL.md>` | ≥ 1 |",
			want: []string{"unsubstituted-metavar row 1"},
		},
		{
			name: "unsubstituted-metavar: ellipsis elision (desk-tools/10 row 4)",
			row:  "| 4 | `DESKPUSHGUARD_OFF=1 ...merged-fixture...; echo $?` | 0 with a warning |",
			want: []string{"unsubstituted-metavar row 4"},
		},
		{
			name: "unsubstituted-metavar: a unicode ellipsis elides the arguments just as well",
			row:  "| 2 | `deskpost … --repo medici-finance/assay` | posted |",
			want: []string{"unsubstituted-metavar row 2"},
		},

		// ---- the four new shapes (#493, #262, #374, #639) + portability -----
		{
			// #493: `go run` prints `exit status 5` and itself exits 1, so the
			// row cannot tell 5 (refused) from 6 (unverifiable) from a compile
			// failure. Measured on PR #487 rows 7-10: go run → 1, binary → 5.
			name: "gorun-exit: a specific non-zero exit asserted through go run (#493)",
			row:  "| 7 | `go run ./tools/desk/cmd/repohardenguard --repo x; echo $?` | rc=5 (refused) |",
			want: []string{"gorun-exit row 7"},
		},
		{
			name: "gorun-exit: the `exit status N` spelling too",
			row:  "| 8 | `go run ./cmd/deskpost ready --pr 1` | exit status 6 |",
			want: []string{"gorun-exit row 8"},
		},
		{
			// #262: BRE treats `|` as an ordinary character, so the pattern is
			// one long literal and the only line containing it is the row.
			name: "bre-alternation: alternation with no -E (#262)",
			row:  "| 1 | `grep -c \"a\\|b\" f.md` | ≥1 |",
			want: []string{"bre-alternation row 1"},
		},
		{
			name: "bre-alternation: the self-matching #257 shape",
			row:  "| 3 | `grep -c \"drain\\|refuse\\|watchdog\" docs/contract.md` | ≥3 |",
			want: []string{"bre-alternation row 3"},
		},
		{
			// #374's cell-splitter hazard: the raw pipe ends the cell, so the
			// command is cut and the "Expect" column is another fragment of it.
			name: "shredded-cell: a RAW pipe cut the Command cell (#374)",
			row:  "| 5 | `go test ./statusgen/ -run 'Dora|Weekly|Artifact'` | PASS |",
			want: []string{"shredded-cell row 5"},
		},
		{
			// #639: the identical command returned exit 1 and exit 2 on
			// consecutive runs because main advanced underneath it.
			name: "moving-ref: --base on a moving ref (#639)",
			row:  "| 4 | `statusgen --root . --consumers --base origin/main` | exit 0 |",
			want: []string{"moving-ref row 4"},
		},
		{
			name: "moving-ref: a git range endpoint moves too",
			row:  "| 2 | `git log --oneline origin/main..HEAD \\| wc -l` | 3 |",
			want: []string{"moving-ref row 2"},
		},
		{
			// #650: BSD grep reads /dev/fd/N as EMPTY, so the row returns 0
			// with the content plainly present — a false finding, not a miss.
			name: "gnu-only: process substitution into grep (#650)",
			row:  "| 1 | `grep -cE 'watchdog' <(sed -n '/3.2/,/3.3/p' contract.md)` | ≥ 18 |",
			want: []string{"gnu-only row 1"},
		},
		{
			name: "gnu-only: grep -P has no BSD equivalent",
			row:  "| 2 | `grep -Pc '(?<=v)[0-9]+' versions.txt` | ≥ 1 |",
			want: []string{"gnu-only row 2"},
		},

		// ---- false positives: these rows are SOUND and must stay silent ------
		{
			name: "a genuinely intended literal pipe via a bracket class",
			row:  "| 1 | `grep -cE \"a[\\|]b\" f.md` | ≥1 |",
			want: nil,
		},
		{
			name: "grep -c whose COUNT is the output, not a zero gate",
			row:  "| 3 | `grep -c \"docs/\" README.md` | ≥4 (doc index present) |",
			want: nil,
		},
		{
			name: "grep -c expecting zero but neutralised with || true",
			row:  "| 2 | `grep -cE \"30:1\" deck.md \\|\\| true` | 0 (F-12) |",
			want: nil,
		},
		{
			name: "expect-zero gated on exit status via ! grep -q — the recommended fix",
			row:  "| 4 | `! grep -qE \"30:1\" deck.md` | exit 0 |",
			want: nil,
		},
		{
			name: "alternation written with separate -e patterns — the recommended fix",
			row:  "| 2 | `grep -ciE -e arm64 -e amd64 S1.md` | ≥1 |",
			want: nil,
		},
		{
			name: "a shell pipe is not a regex alternation",
			row:  "| 5 | `ls docs/ \\| grep -c 2026-07` | ≥1 |",
			want: nil,
		},
		{
			name: "grep -c piped into awk: grep's exit is not the gate",
			row:  "| 3 | `grep -rc foo *.md \\| awk -F: '{t+=$2} END{print t}'` | ≥200 |",
			want: nil,
		},
		{
			name: "grep -F: a pipe is literal BY INTENT — the escape hatch",
			row:  "| 1 | `grep -cF \"a\\|b\" f.md` | ≥1 |",
			want: nil,
		},
		{
			name: "a plain non-grep command",
			row:  "| 1 | `go test` | 0 |",
			want: nil,
		},
		{
			name: "go test -run with a single unambiguous token — the recommended fix",
			row:  "| 5 | `go test ./tools/statusgen/ -run Dora` | PASS |",
			want: nil,
		},
		{
			name: "go test with &&-chained single-pattern runs — the other recommended fix",
			row:  "| 5 | `go test ./statusgen/ -run Dora && go test ./statusgen/ -run Weekly` | PASS |",
			want: nil,
		},
		{
			name: "go test -run with no pipe at all",
			row:  "| 5 | `go test ./tools/statusgen/` | PASS |",
			want: nil,
		},
		{
			name: "go run expecting exit 0 is unaffected by the flattening",
			row:  "| 6 | `go run . --root .. --consumers --brief gt/03` | exit 0 |",
			want: nil,
		},
		{
			name: "the #493 fix: build once, then assert the binary's real exit",
			row:  "| 7 | `go build -o /tmp/rhg ./cmd/repohardenguard && /tmp/rhg --repo x; echo $?` | rc=5 |",
			want: nil,
		},
		{
			name: "the #639 fix: a computed merge-base is a pure function of the PR",
			row:  "| 4 | `statusgen --consumers --base $(git merge-base origin/main HEAD)` | exit 0 |",
			want: nil,
		},
		{
			name: "the #650 fix: a pipe instead of a process substitution",
			row:  "| 1 | `sed -n '/3.2/,/3.3/p' contract.md \\| grep -cE 'watchdog'` | ≥ 18 |",
			want: nil,
		},
		{
			name: "a prose row carries no command to analyse",
			row:  "| 1 | the official example dapp compiled + deployed | observed |",
			want: nil,
		},
		// metavariable false positives — angle brackets and dots that are NOT placeholders
		{
			name: "rule 5: an HTML tag inside a QUOTED grep pattern is a regex, not a placeholder",
			row:  "| 5 | `! grep -Eiq -e \"<script[^>]*src=\" -e \"<link[^>]*stylesheet\" web/site/index.html` | exit 0 |",
			want: nil,
		},
		{
			name: "rule 5: `./...` is Go's package wildcard, not an elision",
			row:  "| 1 | `go test ./tools/example/cmd/exampleboard/... -count=1` | exit 0 |",
			want: nil,
		},
		{
			name: "rule 5: shell redirections and heredocs are not metavariables",
			row:  "| 2 | `statusgen --root . --lint > out.txt 2>&1 && wc -l < out.txt` | ≥1 |",
			want: nil,
		},
		{
			name: "rule 5: a `$(…)` derivation is the recommended fix, not a placeholder",
			row:  "| 3 | `test -d docs/reports/daily/$(date +%F)` | exit 0 |",
			want: nil,
		},
		{
			// Process substitution is not a PLACEHOLDER — the metavariable rule
			// must stay silent on it. It is, separately, a GNU-ism (#650), so
			// the portability rule is the only thing that fires here.
			name: "rule 5: process substitution is shell syntax, not a placeholder",
			row:  "| 4 | `diff <(statusgen --root . --lint) expected.txt` | no output |",
			want: []string{"gnu-only row 4"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := collect(t, verifySection(tc.row))
			if strings.Join(got, ",") != strings.Join(tc.want, ",") {
				t.Errorf("row %q\n  got  %v\n  want %v", tc.row, got, tc.want)
			}
		})
	}
}

// verifyRowStreams copies the fixture tree into a temp root and loads it —
// its own controlled fixture set, the same isolation verifySectionProbs uses for
// the presence lint.
func verifyRowStreams(t *testing.T) []*Stream {
	t.Helper()
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS("testdata/verifyrows")); err != nil {
		t.Fatal(err)
	}
	streams, _, err := loadStreams(root)
	if err != nil {
		t.Fatal(err)
	}
	return streams
}

func verifyRowNotices(t *testing.T) []string {
	t.Helper()
	return unfailableRowNotices(verifyRowStreams(t))
}

// TestUnfailableRowNoticesOnFixture drives the real entrypoint end-to-end: the
// unfailable brief trips every rule, the sound brief trips none, and the legacy
// brief is exempt despite carrying the defect.
func TestUnfailableRowNoticesOnFixture(t *testing.T) {
	notices := verifyRowNotices(t)

	var unfailable, sound, legacy int
	for _, n := range notices {
		switch {
		case strings.Contains(n, "brief-01-unfailable.md"):
			unfailable++
		case strings.Contains(n, "brief-02-sound.md"):
			sound++
		case strings.Contains(n, "brief-03-legacy.md"):
			legacy++
		}
	}

	// vr/01: row 1 → rule 1; row 2 → rule 2; row 3 → rules 1 AND 2; row 4 → rule 3;
	// rows 5 and 6 → rule 5 (bracket metavariable, ellipsis elision).
	if want := 7; unfailable != want {
		t.Errorf("unfailable brief: got %d notices, want %d\n%s", unfailable, want, strings.Join(notices, "\n"))
	}
	if sound != 0 {
		t.Errorf("sound brief must produce NO notices — a rule that fires on a sound row is as corrosive as the defect it hunts; got %d:\n%s", sound, strings.Join(notices, "\n"))
	}
	if legacy != 0 {
		t.Errorf("legacy (non brief-v1) files are exempt; got %d notices", legacy)
	}
}

// TestRuleFixturesGoRed is the stream convention "a check ships with proof it
// can fail", made mechanical.
//
// Each new rule owns a fixture ROOT under testdata/verifyrows/<shape>/ holding
// a RED brief (the positive control — every row carries the defect) and a GREEN
// brief (the negative control — the same checks written correctly). The test
// asserts both halves: the rule fires on every red row, and fires on NO green
// row. Half of that is the usual test; the other half is the one that catches
// a rule which cannot fail, which is the exact defect this whole lint exists
// to kill (#488: eight unfailable checks in one day).
//
// It also asserts the OTHER rules stay off the green brief, so a fixture
// written to exercise one shape cannot quietly accumulate a second.
func TestRuleFixturesGoRed(t *testing.T) {
	tests := []struct {
		dir     string // fixture root under testdata/verifyrows/
		rule    string // the tag its red rows must produce
		redRows int    // how many red rows must fire — a LOWER bound would let
		// the rule regress to firing once and still pass
	}{
		{"gorun-exit", ruleGoRunExit, 4},
		{"bre-alternation", ruleBREAlternation, 4},
		{"literal-pipe", ruleRE2LiteralPipe, 3},
		{"moving-ref", ruleMovingRef, 4},
		{"portability", rulePortability, 5},
	}
	for _, tc := range tests {
		t.Run(tc.dir, func(t *testing.T) {
			root := t.TempDir()
			if err := os.CopyFS(root, os.DirFS("testdata/verifyrows/"+tc.dir)); err != nil {
				t.Fatal(err)
			}
			streams, _, err := loadStreams(root)
			if err != nil {
				t.Fatal(err)
			}
			if len(streams) == 0 {
				t.Fatal("fixture loaded 0 streams — a check that could not look must never report clean")
			}
			notices := unfailableRowNotices(streams)

			var red, green int
			for _, n := range notices {
				switch {
				case strings.Contains(n, "brief-01-red.md"):
					if strings.Contains(n, "["+tc.rule+"]") {
						red++
					}
				case strings.Contains(n, "brief-02-green.md"):
					green++
				}
			}
			if red != tc.redRows {
				t.Errorf("red fixture: %s fired on %d rows, want %d — a rule with no observed red run is the defect it hunts\n%s",
					tc.rule, red, tc.redRows, strings.Join(notices, "\n"))
			}
			if green != 0 {
				t.Errorf("green fixture must be SILENT; got %d notice(s):\n%s", green, strings.Join(notices, "\n"))
			}
		})
	}
}

// TestRuleTagsAreUnique guards the interface the row-audit inventory and any CI
// step select on: two rules sharing a tag would make their counts
// uninterpretable.
func TestRuleTagsAreUnique(t *testing.T) {
	tags := []string{
		ruleERELiteralPipe, ruleGrepZeroCount, ruleExitSwallowed, ruleRE2LiteralPipe,
		ruleMetavar, ruleGoRunExit, ruleBREAlternation, ruleShreddedCell,
		ruleMovingRef, rulePortability,
	}
	seen := map[string]bool{}
	for _, tag := range tags {
		if tag == "" {
			t.Error("a rule tag is empty")
		}
		if seen[tag] {
			t.Errorf("duplicate rule tag %q", tag)
		}
		seen[tag] = true
	}
}

// TestUnfailableRowNoticesAreDeterministic pins the sorted output — the notice
// list feeds a CI gate, which must not churn between runs. It re-runs the check
// over ONE loaded fixture tree: a fresh t.TempDir() per call would vary the path
// prefix and make the comparison meaningless.
func TestUnfailableRowNoticesAreDeterministic(t *testing.T) {
	streams := verifyRowStreams(t)
	first := strings.Join(unfailableRowNotices(streams), "\n")
	for i := 0; i < 3; i++ {
		if got := strings.Join(unfailableRowNotices(streams), "\n"); got != first {
			t.Fatalf("notice order is not stable across runs:\n%s\n---\n%s", first, got)
		}
	}
}
