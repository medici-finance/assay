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
