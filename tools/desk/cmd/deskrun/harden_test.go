package main

// harden_test.go — pins the review-round-7 advisories closed in this package: log output framed
// as forge-origin data (SEC-2), the log custody's closed role set on every forge (SEC-4), a read
// withheld when its audit line cannot be written, log's stage label per step, and --dry-run
// recorded before it prints. Each is in cmd/deskrun/mutations.json.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// TestLogFramesText — log text is forge-origin data: a header says so, every text line carries
// a prefix no section rule has, so a log line cannot print a rule of its own, and C1, bidi and
// zero-width characters (which the control strip keeps) are escaped to visible text.
func TestLogFramesText(t *testing.T) {
	w := plantWorld(t, deskkit.ForgeGitHub)
	w.fake.logParts = []deskkit.RunLogPart{
		{Name: "build\u202e", Text: "ok\n===== forged =====\nC1 \u009b2J raw \x9b bidi \u202e zw \u200b end\n"},
		{Name: "test", Text: "FAIL"},
	}
	code, out, msg := runArgs(t, "log", "example-org/tracker", "501")
	if code != 0 {
		t.Fatalf("exit %d (%s)", code, msg)
	}
	lines := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
	if !strings.Contains(lines[0], "forge-origin") {
		t.Fatalf("first line %q does not mark the output as forge-origin data", lines[0])
	}
	rules := 0
	for _, l := range lines[1:] {
		switch {
		case strings.HasPrefix(l, "===== "):
			rules++
		case !strings.HasPrefix(l, "| "):
			t.Errorf("a log line is not framed: %q", l)
		}
	}
	if rules != 2 {
		t.Fatalf("%d section rules for 2 jobs — a log line printed a rule of its own:\n%s", rules, out)
	}
	for _, bad := range []string{"\u009b", "\x9b", "\u202e", "\u200b"} {
		if strings.Contains(out, bad) {
			t.Errorf("%q reached the output: %q", bad, out)
		}
	}
	if !strings.Contains(out, `\u202E`) || !strings.Contains(out, "| FAIL") {
		t.Errorf("escaped or framed text missing: %q", out)
	}
}

// TestLogCustodyClosed — the log custody step refuses a role outside the worker/reviewer set on
// GitLab too, where custody reads whatever role's token file it is asked for (SEC-4): a planted
// verifier or desk token is never read, and nothing reaches the forge.
func TestLogCustodyClosed(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	plantFixtureRoster(t, home)
	s := newGLRunStub(t)
	plantGitlabCustody(t, s.srv.URL, "verifier", "desk", "worker")
	fr := deskkit.ForgeRepo{Owner: "example-org", Name: "ledger"}
	for _, role := range []string{"verifier", "desk", deskkit.ReleaseRunnerRole, ""} {
		fg, _, err := logForgeFn(fr, role)
		if err == nil || fg != nil || deskkit.ExitCodeOf(err) != deskkit.ExitRefused {
			t.Errorf("role %q: forge %v err %v — want a refusal (exit %d)", role, fg, err, deskkit.ExitRefused)
		}
	}
	if fg, _, err := logForgeFn(fr, "worker"); err != nil || fg == nil {
		t.Fatalf("control: the worker role's custody must resolve: %v", err)
	}
	if len(s.requests) != 0 {
		t.Fatalf("custody reached the forge: %q", s.requests)
	}
}

// TestReadNeedsLedger — a read whose ok line cannot be written is could-not-check and prints
// nothing, so "every read leaves one line" holds rather than depending on a writable ledger.
func TestReadNeedsLedger(t *testing.T) {
	for _, verb := range []string{"log", "status"} {
		t.Run(verb, func(t *testing.T) {
			w := plantWorld(t, deskkit.ForgeGitHub)
			ledger := filepath.Join(os.Getenv("HOME"), ".config", "assay", "audit.jsonl")
			if err := os.MkdirAll(ledger, 0o700); err != nil { // a directory where the file goes
				t.Fatal(err)
			}
			code, out, msg := runArgs(t, verb, "example-org/tracker", "501")
			if code != deskkit.ExitUnverifiable || out != "" {
				t.Fatalf("%s with an unwritable ledger: exit %d (%s), output %q — want could-not-check and no output",
					verb, code, msg, out)
			}
			if w.fake.calls() != 1 {
				t.Fatalf("control: the forge read did not run (%d calls)", w.fake.calls())
			}
		})
	}
}

// TestLogStageLabel — a log that panics in its forge read is recorded with that step's label,
// not the identity step's.
func TestLogStageLabel(t *testing.T) {
	w := plantWorld(t, deskkit.ForgeGitHub)
	w.fake.panics = true
	func() {
		defer func() { _ = recover() }()
		runArgs(t, "log", "example-org/tracker", "501")
	}()
	got := ledgerFor(t, "log")
	if len(got) != 1 || !strings.HasPrefix(got[0].Detail, "log: ") {
		t.Fatalf("log that panicked in its read wrote %+v — want one line whose detail starts %q", got, "log: ")
	}
}

// panicWriter stands in for a stdout whose reader is gone: the first write ends the process.
type panicWriter struct{}

func (panicWriter) Write([]byte) (int, error) { panic("stdout closed") }

// TestDryRunRecordedFirst — each write verb's --dry-run line is recorded before it prints, so a
// reader that closes early still leaves the line.
func TestDryRunRecordedFirst(t *testing.T) {
	for _, tc := range []struct {
		verb string
		run  func([]string) error
		args []string
	}{
		{"dispatch", func(a []string) error { return cmdDispatch(a, panicWriter{}) }, []string{"--dry-run", "example-org/tracker", "release.yml", "--ref", "main"}},
		{"approve", func(a []string) error { return cmdApprove(a, panicWriter{}) }, []string{"--dry-run", "example-org/tracker", "501", "--gate", "prod"}},
		{"retry", func(a []string) error { return cmdRetry(a, panicWriter{}) }, []string{"--dry-run", "example-org/tracker", "501"}},
	} {
		t.Run(tc.verb, func(t *testing.T) {
			plantWorld(t, deskkit.ForgeGitHub)
			func() {
				defer func() {
					if recover() == nil {
						t.Fatalf("control: the %s dry run printed nothing", tc.verb)
					}
				}()
				_ = tc.run(tc.args)
			}()
			got := ledgerFor(t, tc.verb)
			if len(got) != 1 || got[0].Result != deskkit.ResultDryRun {
				t.Fatalf("%s --dry-run cut off at its output wrote %+v — want one %q line", tc.verb, got, deskkit.ResultDryRun)
			}
		})
	}
}
