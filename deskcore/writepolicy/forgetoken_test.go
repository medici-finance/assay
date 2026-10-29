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
	// operators on one line, and many quoted spans.
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
