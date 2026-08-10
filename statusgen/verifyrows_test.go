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
			line: "| 1 | `dpm test` | 0 |",
			want: []string{" 1 ", " `dpm test` ", " 0 "},
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
		{"bare code span", "`dpm test`", "dpm test"},
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
	for _, cmd := range []string{`dpm test`, `go run ./tools/statusgen --lint`, `rg -c "a\|b" .`} {
		if got := grepCalls(tokenizeCommand(cmd)); len(got) != 0 {
			t.Errorf("grepCalls(%q) = %v, want none", cmd, got)
		}
	}
}

// TestGoTestRunPatterns pins extraction of the `-run` argument from `go test`
// invocations, across the `-run PAT`, `-run=PAT` and `--run=PAT` forms, and
// confirms a non-`go test` command yields nothing (#580).
func TestGoTestRunPatterns(t *testing.T) {
	tests := []struct {
		name string
		cmd  string
		want []string
	}{
		{"space form", `go test ./tools/statusgen/ -run 'Dora\|Weekly\|Artifact'`, []string{`Dora\|Weekly\|Artifact`}},
		{"single-dash equals form", `go test ./tools/statusgen/ -run='A\|B'`, []string{`A\|B`}},
		{"double-dash equals form", `go test ./tools/statusgen/ --run='A\|B'`, []string{`A\|B`}},
		{"unambiguous single token", `go test ./tools/statusgen/ -run Dora`, []string{"Dora"}},
		{"no -run flag at all", `go test ./tools/statusgen/`, nil},
		{"not a go test invocation", `go build ./tools/...`, nil},
		{"not go at all", `dpm test`, nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := goTestRunPatterns(tokenizeCommand(tc.cmd))
			if len(got) != len(tc.want) {
				t.Fatalf("goTestRunPatterns(%q) = %q, want %q", tc.cmd, got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("pattern %d = %q, want %q", i, got[i], tc.want[i])
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
		{"go package wildcard is not an elision", `go test ./tools/desk/... -count=1`, nil},
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
		{"tee swallows the real exit status", `dpm test \| tee out.log`, "tee"},
		{"tee at the end of a longer pipeline", `dpm test \| grep -v x \| tee out.log`, "tee"},
		{"no pipeline", `dpm test`, ""},
		{"grep as the last stage is a legitimate gate", `dpm test \| grep -q PASS`, ""},
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

// collect runs the row-level rules over a synthetic Verify section, returning the
// notices. It mirrors unfailableRowNotices' per-row body without the file I/O, so
// the table below can stay focused on command/expect pairs.
func collect(t *testing.T, section string) []string {
	t.Helper()
	var out []string
	verifyRowTable(section, func(num, cmdCell, expect string) {
		cmd := codeSpan(cmdCell)
		if cmd == "" {
			return
		}
		toks := tokenizeCommand(cmd)
		for _, g := range grepCalls(toks) {
			if g.extended {
				for _, p := range g.patterns {
					if escapedPipeOutsideBracket(p) {
						out = append(out, "rule1 row "+num)
						break
					}
				}
			}
			if g.count && g.last && expectsZeroCount(expect) && !forcesSuccess(toks) {
				out = append(out, "rule2 row "+num)
			}
		}
		if sink := pipelineSwallowsExit(toks); sink != "" {
			out = append(out, "rule3 row "+num)
		}
		for _, p := range goTestRunPatterns(toks) {
			if escapedPipeOutsideBracket(p) {
				out = append(out, "rule4 row "+num)
				break
			}
		}
		if mv := unsubstitutedMetavars(cmd); len(mv) > 0 {
			out = append(out, "rule5 row "+num)
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
			name: "rule 1: mis-escaped alternation in grep -E (example-poc/01 row 2)",
			row:  "| 2 | `grep -ciE \"arm64\\|amd64\\|version\" S1.md` | ≥1 |",
			want: []string{"rule1 row 2"},
		},
		{
			name: "rule 1: fires through a shell pipe (example-poc/08 row 4)",
			row:  "| 4 | `sed '/x/d' S7.md \\| { ! grep -qE \"x86-only\\|counter-based\"; }` | exit 0 |",
			want: []string{"rule1 row 4"},
		},
		{
			name: "rule 2: grep -c gated on a count of zero (assay-dogfood/04 row 2 shape)",
			row:  "| 2 | `grep -c \"medici-stuff\" docs/brand/README.md` | 0 (stale name fixed) |",
			want: []string{"rule2 row 2"},
		},
		{
			name: "rule 2: grep -c as the last stage of a pipeline (methodology/24 row 1)",
			row:  "| 1 | `go run ./tools/statusgen --lint 2>&1 \\| grep -c gate-why` | `0` — no brief trips the NOTICE |",
			want: []string{"rule2 row 1"},
		},
		{
			name: "both rules on one row — the two errors partially cancel",
			row:  "| 2 | `ls ~/.claude/skills/ \\| grep -cE \"the-desk\\|verify-desk\"` | 0 (loose copies retired) |",
			want: []string{"rule1 row 2", "rule2 row 2"},
		},
		{
			name: "rule 3: exit status swallowed by tee",
			row:  "| 1 | `dpm test \\| tee test.log` | 0 |",
			want: []string{"rule3 row 1"},
		},
		{
			name: "rule 4: mis-escaped alternation in go test -run (oit PR #578 row 5 shape)",
			row:  "| 5 | `go test ./tools/statusgen/ -run 'Dora\\|Weekly\\|Artifact'` | PASS |",
			want: []string{"rule4 row 5"},
		},
		{
			name: "rule 4: fires on the `-run=` form too",
			row:  "| 6 | `go test ./tools/statusgen/ -run='A\\|B'` | PASS |",
			want: []string{"rule4 row 6"},
		},
		{
			name: "rule 5: bare angle-bracket metavariable (desk-console-2/07 row 3)",
			row:  "| 3 | `gh issue view <N> --json comments` | the App |",
			want: []string{"rule5 row 3"},
		},
		{
			name: "rule 5: a multi-word metavariable (methodology-metrics/25 row 3)",
			row:  "| 3 | `yq eval '.on.schedule' <mm/22 workflow file>` | ≥ 3 cron entries |",
			want: []string{"rule5 row 3"},
		},
		{
			name: "rule 5: metavariable embedded in a path (methodology-metrics/22 row 5)",
			row:  "| 5 | `jq -e 'type==\"array\"' docs/reports/daily/<that-date>/prs.json` | exit 0 |",
			want: []string{"rule5 row 5"},
		},
		{
			name: "rule 5: an angle-bracket metavariable is also a shell redirect (desk-hardening/02 row 1)",
			row:  "| 1 | `grep -ci 'foreign commit' <batch-fanout SKILL.md>` | ≥ 1 |",
			want: []string{"rule5 row 1"},
		},
		{
			name: "rule 5: ellipsis elision (desk-tools/10 row 4 — the F-impl-claims-unproven case)",
			row:  "| 4 | `DESKPUSHGUARD_OFF=1 ...merged-fixture...; echo $?` | 0 with a warning |",
			want: []string{"rule5 row 4"},
		},
		{
			name: "rule 5: a unicode ellipsis elides the arguments just as well",
			row:  "| 2 | `deskpost … --repo medici-finance/assay-toolkit` | posted |",
			want: []string{"rule5 row 2"},
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
			name: "basic grep (no -E) where \\| IS alternation",
			row:  "| 1 | `grep -c \"a\\|b\" f.md` | ≥1 |",
			want: nil,
		},
		{
			name: "a plain non-grep command",
			row:  "| 1 | `dpm test` | 0 |",
			want: nil,
		},
		{
			name: "go test -run with a single unambiguous token — the recommended fix",
			row:  "| 5 | `go test ./tools/statusgen/ -run Dora` | PASS |",
			want: nil,
		},
		{
			name: "go test -run with an unescaped alternation — the recommended fix",
			row:  "| 5 | `go test ./tools/statusgen/ -run 'Dora|Weekly|Artifact'` | PASS |",
			want: nil,
		},
		{
			name: "go test -run with no pipe at all",
			row:  "| 5 | `go test ./tools/statusgen/` | PASS |",
			want: nil,
		},
		{
			name: "a prose row carries no command to analyse",
			row:  "| 1 | the official example dapp compiled + deployed | observed |",
			want: nil,
		},
		// rule 5 false positives — angle brackets and dots that are NOT placeholders
		{
			name: "rule 5: an HTML tag inside a QUOTED grep pattern is a regex, not a placeholder",
			row:  "| 5 | `! grep -Eiq -e \"<script[^>]*src=\" -e \"<link[^>]*stylesheet\" web/site/index.html` | exit 0 |",
			want: nil,
		},
		{
			name: "rule 5: `./...` is Go's package wildcard, not an elision",
			row:  "| 1 | `go test ./tools/desk/cmd/deskboard/... -count=1` | exit 0 |",
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
			name: "rule 5: process substitution is shell syntax, not a placeholder",
			row:  "| 4 | `diff <(statusgen --root . --lint) expected.txt` | no output |",
			want: nil,
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

// verifyRowStreams copies the #509 fixture tree into a temp root and loads it —
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
		if !strings.Contains(n, "#509") {
			t.Errorf("every notice must cite #509 so a reader can find the rule; got %q", n)
		}
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
