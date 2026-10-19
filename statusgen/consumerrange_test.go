package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// regression: #1915
func TestConsumerRange(t *testing.T) {
	for _, tc := range []struct {
		cmd, expect string
		bad         bool
	}{
		{"statusgen --consumers --brief alpha/01", "exit 0", true},
		{"statusgen --brief=alpha/01 --consumers --diff-base abc", "exit 0", true},
		{"statusgen --consumers --base= --brief alpha/01", "exit 0", true},
		{"statusgen --consumers --brief alpha/01 --base 0123456789abcdef0123456789abcdef01234567", "exit 0", true},
		{"echo checkout && statusgen --consumers --brief alpha/01 --base abc", "exit 0", true},
		{"git -C other checkout --detach abc && statusgen --root . --consumers --brief alpha/01 --base def", "exit 0", true},
		{"git -C snap checkout --detach abc && statusgen --root snap --consumers --brief alpha/01 --base def", "exit 0", false},
		{"statusgen --consumers --base abc --delivery def", "exit 0", true},
		{"statusgen --consumers --brief alpha/01 --base $(git merge-base refs/remotes/origin/main HEAD)", "exit 0", true},
		{"statusgen --consumers --brief alpha/01; echo --base abc", "exit 0", true},
		{"statusgen --consumers --brief alpha/01 --base abc --delivery def", "exit 0", false},
		{"statusgen --consumers --brief alpha/01 --base abc", "pre-merge only; exit 0", false},
		{"statusgen --consumers --brief alpha/01 --base $(git merge-base main HEAD)", "pre-merge only; exit 0", false},
		{"git checkout --detach abc && statusgen --consumers --brief alpha/01 --base def", "exit 0", false},
		{"echo '--consumers --brief alpha/01'", "exit 0", false},
		{"echo --consumers --brief alpha/01", "exit 0", false},
	} {
		t.Run(tc.cmd, func(t *testing.T) {
			got := hasFinding(rowFindings("`"+tc.cmd+"`", tc.expect), ruleConsumerRange)
			if got != tc.bad {
				t.Fatalf("consumer range finding=%v, want %v", got, tc.bad)
			}
		})
	}
}

// regression: #1915
func TestConsumerRangePlanted(t *testing.T) {
	root := consumersFixture(t, []string{"web/fixed.go: fixed-here"}, nil)
	p := filepath.Join(root, "docs/streams/alpha/brief-01-claims.md")
	content, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	mustWrite(t, p, strings.TrimRight(string(content), "\n")+"\n| 91 | `statusgen --consumers --brief alpha/01` | exit 0 |\n")
	streams, _, err := loadStreams(root)
	if err != nil {
		t.Fatal(err)
	}
	notices := unfailableRowNotices(streams)
	if !strings.Contains(strings.Join(notices, "\n"), "Verify row 91 ["+ruleConsumerRange+"]") {
		t.Fatalf("planted second row not detected: %v", notices)
	}
}
