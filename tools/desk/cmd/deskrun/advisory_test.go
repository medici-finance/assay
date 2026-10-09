package main

// advisory_test.go — pins controls a review found unpinned (review advisories on PR #2382):
// retry's loop-identity check, the control-byte strip on a job NAME in `log` output (the strip
// on the log TEXT is pinned in readpath_test.go), retry's own audit lines, the stage label on a
// status failure, and a read that panics being recorded as could-not-check rather than ok.
// Removing any of them turns its test red; each is in cmd/deskrun/mutations.json.

import (
	"bufio"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
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

// ledgerLine is one audit line, as much of it as these pins read.
type ledgerLine struct{ Tool, Verb, Result, Detail string }

// ledgerFor returns this test's audit lines for one verb.
func ledgerFor(t *testing.T, verb string) []ledgerLine {
	t.Helper()
	f, err := os.Open(filepath.Join(os.Getenv("HOME"), ".config", "assay", "audit.jsonl"))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		t.Fatalf("open ledger: %v", err)
	}
	defer f.Close()
	var got []ledgerLine
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var e ledgerLine
		if err := json.Unmarshal(sc.Bytes(), &e); err != nil {
			t.Fatalf("ledger line %q: %v", sc.Text(), err)
		}
		if e.Verb == verb {
			got = append(got, e)
		}
	}
	return got
}

// TestDeskrunRetryAuditLines — retry, a write, records exactly one line under the write verbs'
// key for each outcome: ok when the forge accepted it, the failure's class when it did not, and
// dryrun (never ok) for --dry-run.
func TestDeskrunRetryAuditLines(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		fail error
		want string
	}{
		{"ok", []string{"retry", "example-org/tracker", "501"}, nil, deskkit.ResultOK},
		{"refused", []string{"retry", "example-org/tracker", "501"}, deskkit.Refused("refused: the forge answered 403"), deskkit.ResultRefused},
		{"unverifiable", []string{"retry", "example-org/tracker", "501"}, errors.New("the forge answered 502"), deskkit.ResultUnverifiable},
		{"dry-run", []string{"retry", "--dry-run", "example-org/tracker", "501"}, nil, deskkit.ResultDryRun},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := plantWorld(t, deskkit.ForgeGitHub)
			w.fake.retryFail = tc.fail
			runArgs(t, tc.args...)
			got := ledgerFor(t, "retry")
			if len(got) != 1 || got[0].Tool != toolName || got[0].Result != tc.want {
				t.Fatalf("retry %s wrote %+v — want exactly one %q line under %q", tc.name, got, tc.want, toolName)
			}
		})
	}
}

// TestDeskrunStatusStageLabel — a status read the forge fails is recorded with the step it
// failed at, so the audit trail says the forge read failed and not that the arguments did.
func TestDeskrunStatusStageLabel(t *testing.T) {
	w := plantWorld(t, deskkit.ForgeGitHub)
	w.fake.fail = errors.New("the forge answered 502")
	runArgs(t, "status", "example-org/tracker", "501")
	got := ledgerFor(t, "status")
	if len(got) != 1 || !strings.HasPrefix(got[0].Detail, "status: ") {
		t.Fatalf("status forge failure wrote %+v — want one line whose detail starts %q", got, "status: ")
	}
}

// TestDeskrunReadPanicNotOK — a read that panics after its trail started is recorded as
// could-not-check, never as ok (the deferred finish runs while the panic unwinds, with no error).
func TestDeskrunReadPanicNotOK(t *testing.T) {
	for _, verb := range []string{"log", "status"} {
		t.Run(verb, func(t *testing.T) {
			w := plantWorld(t, deskkit.ForgeGitHub)
			w.fake.panics = true
			func() {
				defer func() {
					if recover() == nil {
						t.Fatalf("control: the fake forge's %s read did not panic", verb)
					}
				}()
				runArgs(t, verb, "example-org/tracker", "501")
			}()
			got := ledgerFor(t, verb)
			if len(got) != 1 || got[0].Result != deskkit.ResultUnverifiable {
				t.Fatalf("%s that panicked wrote %+v — want exactly one %q line", verb, got, deskkit.ResultUnverifiable)
			}
		})
	}
}
