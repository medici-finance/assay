package main

import (
	"strings"
	"testing"
)

func TestVerifyRowPortability(t *testing.T) {
	t.Run("notice-flow", func(t *testing.T) {
		dir := t.TempDir()
		briefV1WithVerify(t, dir, "01", "| # | Command | Expect |\n|---|---|---|\n| 1 | `cat /tmp/proof` | exit 0 |\n| 2 | `cat ${TMPDIR:-/tmp}/proof` | exit 0 |\n| 3 | `findstr PASS output` | exit 0 (OS: Windows) |")
		got := verifyPortabilityNotices([]*Stream{{Name: "t", Dir: dir}})
		if len(got) != 1 || !strings.Contains(got[0], "Verify row 1 [verify-row-portability]") {
			t.Fatalf("notice routing = %v", got)
		}
	})

	for _, tc := range []struct {
		name, command, expect string
		notice                bool
	}{
		{"tmp", "`cat /tmp/proof`", "exit 0", true},
		{"sh", "`sh -c 'go test .'`", "exit 0", true},
		{"bash", "`bash -c 'go test .'`", "exit 0", true},
		{"findstr", "`findstr PASS output`", "exit 0", true},
		{"tmpdir", "`cat ${TMPDIR:-/tmp}/proof`", "exit 0", false},
		{"marked", "`cat /tmp/proof`", "exit 0 (OS: POSIX)", false},
		{"marked-command", "`findstr PASS output` (Windows-only)", "exit 0", false},
		{"go", "`go test .`", "exit 0", false},
		{"word", "`go test ./findstr_test`", "exit 0", false},
		{"tmpdir-plus-hardcode", "`cp ${TMPDIR:-/tmp}/a /tmp/b`", "exit 0", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := verifyPortability(verifyRowCells{Command: tc.command, Expect: tc.expect}); got != tc.notice {
				t.Fatalf("notice=%v, want %v", got, tc.notice)
			}
		})
	}
}
