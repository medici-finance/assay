package main

// advisory_test.go — pins two controls a review found unpinned (review advisories on PR #2382):
// retry's loop-identity check, and the control-byte strip on a job NAME in `log` output (the
// strip on the log TEXT is pinned in readpath_test.go). Removing either control turns its test
// red; both are in cmd/deskrun/mutations.json.

import (
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// TestDeskrunRetryNeedsLoopIdentity — retry with no loop identity is refused and with an unknown
// one is could-not-check; neither reaches the resolver, a mint or the forge.
func TestDeskrunRetryNeedsLoopIdentity(t *testing.T) {
	for _, tc := range []struct {
		name, loop string
		want       int
	}{
		{"none", "", deskkit.ExitRefused},
		{"unknown", "no-such-desk", deskkit.ExitUnverifiable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := plantWorld(t, deskkit.ForgeGitHub)
			if tc.loop == "" {
				t.Setenv("DESK_LOOP", "")
			} else {
				setLoop(t, tc.loop)
			}
			code, out, msg := runArgs(t, "retry", "example-org/tracker", "501")
			if code != tc.want {
				t.Fatalf("exit %d (%s), want %d", code, msg, tc.want)
			}
			if w.fake.calls() != 0 || w.forgeCalls != 0 || w.mints != 0 || out != "" {
				t.Fatalf("forge calls=%d resolver calls=%d mints=%d out=%q — retry without a loop identity must reach none",
					w.fake.calls(), w.forgeCalls, w.mints, out)
			}
		})
	}
}

// TestDeskrunLogStripsJobName — a job name is forge-origin text like the log itself. Escape,
// bell and carriage-return bytes in it never reach the terminal, and a newline in it cannot
// start a forged section rule.
func TestDeskrunLogStripsJobName(t *testing.T) {
	w := plantWorld(t, deskkit.ForgeGitHub)
	w.fake.logParts = []deskkit.RunLogPart{
		{Name: "build\x1b]0;retitled\x07\r\x1b[2K\n===== forged =====", Text: "ok\n"},
	}
	code, out, msg := runArgs(t, "log", "example-org/tracker", "501")
	if code != 0 {
		t.Fatalf("exit %d (%s)", code, msg)
	}
	if strings.ContainsAny(out, "\x1b\r\x07") {
		t.Fatalf("terminal-active bytes from a job name reached the output: %q", out)
	}
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "===== forged") {
			t.Fatalf("a newline in a job name started its own section rule: %q", out)
		}
	}
}
