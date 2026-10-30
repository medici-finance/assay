package writepolicy

import (
	"strings"
	"testing"
	"time"
)

// regression: #4821
func TestAmbientForgeTokenClassify(t *testing.T) {
	read := func(cmd string) bool {
		return Classify(Request{Tool: "Bash", Target: cmd}, LexicalContext{}).ReadsAmbientForgeToken()
	}
	deep := func(inner string) string {
		return strings.Repeat("echo $(", maxForgeTokenDepth+2) + inner + strings.Repeat(")", maxForgeTokenDepth+2)
	}
	for _, tc := range []struct {
		cmd  string
		want bool
	}{
		{"gh auth token", true},
		{"gh 2>/dev/null auth token", true},
		{">out.txt gh auth token", true},
		{"gh auth token>/dev/null", true},
		{"gh auth status --show-token=false", false},
		{"gh auth status --show-token=true", true},
		{"gh auth status --show-token=TRUE", true},
		{"gh auth status -a", false},
		{"gh auth login --with-token <<< token", false},
		{"gh auth refresh", false},
		{"gh auth", false},
		{"gh", false},
		{"", false},
		{"echo \"it's gh auth token\"", false},
		{"echo 'unterminated; gh auth token", true},
		// Too deep to lex is undecided, and an undecided scan is refused.
		{deep("gh auth token"), true},
		{deep("ls"), true},
	} {
		if got := read(tc.cmd); got != tc.want {
			t.Errorf("ReadsAmbientForgeToken(%q) = %v, want %v", tc.cmd, got, tc.want)
		}
	}
}

// regression: #4821
func TestForgeWrapperChainLinear(t *testing.T) {
	// A wrapper chain is judged with one look per word: the judged-position
	// count is pinned linear in the chain length, and the work budget is not
	// what stops it (a scan that only finishes by exhausting the budget would
	// pass on the fail-closed rule while hiding the blow-up).
	for _, k := range []int{26, 400, 4000} {
		for _, tc := range []struct {
			cmd  string
			want bool
		}{
			{strings.Repeat("command nice ", k/2) + "true; gh auth token", true},
			{strings.Repeat("env ", k) + "true", false},
			{strings.Repeat("timeout 5 ", k/2) + "gh pr view 1", false},
		} {
			got, st := forgeTokenReadStats(tc.cmd)
			if got != tc.want {
				t.Fatalf("k=%d %q…: read = %v, want %v", k, tc.cmd[:24], got, tc.want)
			}
			if st.exhausted {
				t.Fatalf("k=%d %q…: budget exhausted (%d units), want a linear scan", k, tc.cmd[:24], st.spent)
			}
			if words := len(strings.Fields(tc.cmd)); st.judged > 2*words+8 {
				t.Fatalf("k=%d %q…: judged %d positions for %d words, want <= %d", k, tc.cmd[:24], st.judged, words, 2*words+8)
			}
		}
	}
}

// regression: #4821
func TestForgeBudgetFailsClosed(t *testing.T) {
	// Input built to outrun the work budget is undecided, and an undecided
	// scan is refused whatever follows (over-reach on pathological input only).
	chain := "env " + strings.Repeat("gh auth ", 3000)
	for _, tc := range []struct {
		cmd  string
		want bool
	}{
		{chain + "; echo gh auth token", true},
		{chain + "; echo ok", true},
	} {
		got, st := forgeTokenReadStats(tc.cmd)
		if !st.exhausted {
			t.Fatalf("fixture must exhaust the budget (spent %d)", st.spent)
		}
		if got != tc.want {
			t.Fatalf("exhausted scan: read = %v, want %v", got, tc.want)
		}
	}
}

// regression: #4821
func TestForgeANSICDecode(t *testing.T) {
	for in, want := range map[string]string{
		`tok\x65n`:      "token",
		`\147h`:         "gh",
		`\u0074oken`:    "token",
		`\U00000067h`:   "gh",
		`a\'b`:          "a'b",
		`\q`:            `\q`,
		`no escapes`:    "no escapes",
		`\x`:            `\x`,
		`line\nbreak\t`: "line\nbreak\t",
	} {
		if got := decodeANSIC(in); got != want {
			t.Errorf("decodeANSIC(%q) = %q, want %q", in, got, want)
		}
	}
	// An escaped quote inside the span does not end it, and the words after
	// it are still lexed as commands.
	for _, tc := range []struct {
		cmd  string
		want bool
	}{
		{`echo $'it\'s'; gh auth token`, true},
		{`echo $'it\'s gh auth token'`, false},
	} {
		if got := forgeTokenRead(tc.cmd); got != tc.want {
			t.Errorf("forgeTokenRead(%q) = %v, want %v", tc.cmd, got, tc.want)
		}
	}
}

// forgeWithin runs forgeTokenReadStats under a deadline: a hook timeout lets
// the call through, so a scan that does not finish in time is a fail-open.
func forgeWithin(t *testing.T, cmd string, d time.Duration) (bool, forgeStats) {
	t.Helper()
	type res struct {
		hit bool
		st  forgeStats
	}
	done := make(chan res, 1)
	go func() {
		hit, st := forgeTokenReadStats(cmd)
		done <- res{hit, st}
	}()
	select {
	case r := <-done:
		return r.hit, r.st
	case <-time.After(d):
		t.Fatalf("%d-byte command: no decision within %v", len(cmd), d)
	}
	return false, forgeStats{}
}

// regression: #4821
func TestForgeScanWorkBounded(t *testing.T) {
	// Every pass the forge scan makes is charged to its work budget, so no
	// shape outruns it: each fixture is decided within a deadline, without
	// exhausting the budget, at a bounded number of units per byte. The
	// shapes are ANSI-C spans holding an escaped quote, many heredoc
	// operators on one line, many quoted spans, and many substitutions whose
	// two readings end differently (each script judges the other reading once).
	const n = 600
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
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, st := forgeWithin(t, tc.cmd, 2*time.Second)
			if got != tc.want {
				t.Errorf("read = %v, want %v", got, tc.want)
			}
			if st.exhausted || st.spent > 8*len(tc.cmd)+64 {
				t.Errorf("spent %d units on %d bytes (exhausted %v), want <= 8 per byte",
					st.spent, len(tc.cmd), st.exhausted)
			}
		})
	}
}

// regression: #4821
func TestForgeUndecidedRefused(t *testing.T) {
	// Where the scan cannot finish (the budget runs out, or nesting is deeper
	// than maxForgeTokenDepth) the command is refused outright. A raw-text
	// test there would let through any spelling the lexer decodes: ANSI-C
	// escapes, or a word split by quotes.
	prefix := "sudo " + strings.Repeat("gh auth x ", 2500)
	nest := func(inner string) string {
		return strings.Repeat(": $(", maxForgeTokenDepth+2) + inner + strings.Repeat(")", maxForgeTokenDepth+2)
	}
	for _, tc := range []struct {
		cmd      string
		exhausts bool
	}{
		{prefix + `; gh auth $'\x74oken'`, true},
		{prefix + "; gh auth to''ken", true},
		{prefix + "; echo ok", true},
		{nest("gh auth t'o'ken"), false},
		{nest(`gh $'\x61uth' token`), false},
		{nest("ls"), false},
	} {
		got, st := forgeWithin(t, tc.cmd, 2*time.Second)
		if st.exhausted != tc.exhausts {
			t.Fatalf("%q…: exhausted = %v, want %v", tc.cmd[len(tc.cmd)-24:], st.exhausted, tc.exhausts)
		}
		if !got {
			t.Errorf("%q…: an undecided scan must be refused", tc.cmd[len(tc.cmd)-24:])
		}
	}
	// Paired: the same reads, short enough to judge in full, are refused on
	// their own, and a plain command is not.
	for cmd, want := range map[string]bool{
		`gh auth $'\x74oken'`: true, "gh auth to''ken": true, "gh auth t'o'ken": true, "echo ok": false,
	} {
		if got, st := forgeTokenReadStats(cmd); got != want || st.exhausted {
			t.Errorf("%q: read = %v (exhausted %v), want %v", cmd, got, st.exhausted, want)
		}
	}
}

// regression: #4821
func TestForgeArithHeredoc(t *testing.T) {
	// A `((` is arithmetic only when its text closes with `))` the way bash
	// decides it; otherwise it opens nested subshells, and a `<<` after it is
	// a heredoc. Reading it as arithmetic anyway lexes the heredoc's body as
	// commands, and an apostrophe there opens a quote that runs past the
	// delimiter line and hides the read after it.
	for _, tc := range []struct {
		cmd  string
		want bool
	}{
		// Nested subshells, then a data heredoc: the read after it is judged.
		{"((true); true)\ncat > notes.md <<'E'\nit's here\nE\ngh auth token\necho 'done'", true},
		{"((true); true)\ncat > notes.md <<'E'\nit's gh auth token\nE\necho 'done'", false},
		// Text the scan cannot read as a parenthesis count: undecided.
		{"(('n'); true)\ncat > notes.md <<'E'\nit's here\nE\ngh auth token\necho 'done'", true},
		{"(('x'<<E))\ngh auth token\nE", true},
		// Confirmed arithmetic: its `<<` is a shift, so the next line runs.
		{"((x<<E))\ngh auth token\nE", true},
		{"((\"x\"<<E))\ngh auth token\nE", true},
		{"((n = 1<<2)) && cat > notes.md <<'E'\nit's gh auth token\nE", false},
		{"for ((i=0; i<2; i++)); do echo $i; done\ncat > n.md <<'E'\ngh auth token\nE", false},
		// `$#`, a plain `${…}` and a plain quoted word keep the count, so a
		// data heredoc after them is placed, and a read after its body judged.
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
	// The defect class: a `<<` the scan reads as a heredoc (or not) where the
	// shell reads it the other way. Each refused shape runs the read in bash
	// or zsh; each allowed one is the same construct, placed.
	for _, tc := range []struct {
		cmd  string
		want bool
	}{
		{"x=$(( \"$y\" <<'E'\n$(gh auth token)\nE\n))", true},
		{"x=$((1<<2)); cat > n.md <<'E'\nit's gh auth token\nE", false},
		{"echo $[1<<E]\ngh auth token\nE]", true},
		{"echo ${x:-<<E}\ngh auth token\nE}", true},
		{"echo ${x:-y}; cat > n.md <<'E'\ngh auth token\nE", false},
		{"a[x<<E]=1\ngh auth token\nE]=1", true},
		{"a[1]=2; cat > n.md <<'E'\ngh auth token\nE", false},
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
		// A substitution in a delimiter word: the body's end is undecided.
		{"cat <<$(echo E)\nbody\n$(echo E)\necho ok", true},
	} {
		if got := forgeTokenRead(tc.cmd); got != tc.want {
			t.Errorf("forgeTokenRead(%q) = %v, want %v", tc.cmd, got, tc.want)
		}
	}
}
