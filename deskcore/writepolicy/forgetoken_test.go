package writepolicy

import (
	"strings"
	"testing"
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
		// Too deep to lex fails closed on the words, never open.
		{deep("gh auth token"), true},
		{deep("ls"), false},
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
	// Input built to outrun the work budget is decided by the fail-closed
	// rule: refused when it names gh, auth and token at all, else allowed.
	chain := "env " + strings.Repeat("gh auth ", 3000)
	for _, tc := range []struct {
		cmd  string
		want bool
	}{
		{chain + "; echo gh auth token", true},
		{chain + "; echo ok", false},
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
