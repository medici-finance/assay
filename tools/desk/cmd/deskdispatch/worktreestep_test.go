package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// deskwt's stderr as a REAL run produces it: the effective-config echo, the unpinned-build
// warning, then the tool's own message. A step report that keeps only the first line keeps
// only the echo — which is how a stale-branch collision reached an operator as
// "worktree-create failed (assay-config: …)" and sent them chasing the claim instead of the
// stray ref.
const deskwtStderr = `assay-config: class=write source=config file /x/roster.env configured=true
assay-config: ASSAY_ALLOWED_REPOS=example-org/tracker:ci:private
desk-tools WARNING: running UNPINNED (go run / unstamped build) — sourceSHA/builtAt not embedded.
refused: branch feat/item-1 already exists and is CHECKED OUT in the worktree /private/tmp/tracker-item-1 — that worktree owns it, so it is not a leftover.`

func runCapturingStderr(t *testing.T, args []string) (int, string) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	old := os.Stderr
	os.Stderr = w
	rc := run(args)
	_ = w.Close()
	os.Stderr = old
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	return rc, buf.String()
}

// stepReport isolates the worktree-create failure report from the run's own stderr, which
// legitimately also carries this binary's config echo and unpinned warning.
func stepReport(t *testing.T, stderr string) string {
	t.Helper()
	i := strings.Index(stderr, "step "+stepWorktreeCreate+":")
	if i < 0 {
		t.Fatalf("no %s step report on stderr:\n%s", stepWorktreeCreate, stderr)
	}
	return stderr[i:]
}

// The worktree-create step reports what deskwt SAID, verbatim — not the first line of a
// stream whose first lines are always the config echo.
func TestWorktreeCreateSurfacesDeskwtsOwnMessageVerbatim(t *testing.T) {
	s := &stub{}
	_, root := s.install(t)
	plantScripts(t, root)
	s.replies = []reply{
		{match: "remote get-url origin", stdout: "git@github.com:medici-finance/assay.git"},
		{match: "deskwt add", stderr: deskwtStderr, code: deskkit.ExitRefused},
	}

	promptFile := filepath.Join(t.TempDir(), "p.md")
	rc, stderr := runCapturingStderr(t, []string{"item-1", "--root", root, "--prompt-file", promptFile})

	// deskwt REFUSED — a decision, not an unestablished state. Flattening it to 6 tells the
	// operator to retry something that can never succeed on its own.
	if rc != deskkit.ExitRefused {
		t.Errorf("rc = %d, want %d (deskwt's refusal passes through)", rc, deskkit.ExitRefused)
	}
	// Assert on the STEP REPORT, not on the whole stream: this binary's own config echo and
	// unpinned warning are legitimately on stderr, and the bug was quoting THEM as the cause.
	report := stepReport(t, stderr)
	want := "refused: branch feat/item-1 already exists and is CHECKED OUT in the worktree " +
		"/private/tmp/tracker-item-1 — that worktree owns it, so it is not a leftover."
	if !strings.Contains(report, want) {
		t.Errorf("the step report does not carry deskwt's own message:\nwant substring: %s\ngot:\n%s", want, report)
	}
	if strings.Contains(report, "assay-config: class=write") {
		t.Errorf("the step report quoted the config echo as the failure cause:\n%s", report)
	}
	if strings.Contains(report, "running UNPINNED") {
		t.Errorf("the step report quoted the unpinned-build warning as the failure cause:\n%s", report)
	}
	// The claim is still held and no agent was launched with no home.
	if !strings.Contains(report, "The claim is HELD") {
		t.Errorf("the step report dropped the held-claim instruction:\n%s", report)
	}
	if _, err := os.Stat(promptFile); err == nil {
		t.Error("a prompt was emitted for a dispatch with no worktree")
	}
}

// An unverifiable deskwt failure stays unverifiable: only a REFUSAL passes through as one.
func TestWorktreeCreateKeepsAnUnverifiableFailureUnverifiable(t *testing.T) {
	s := &stub{}
	_, root := s.install(t)
	plantScripts(t, root)
	s.replies = []reply{
		{match: "remote get-url origin", stdout: "git@github.com:medici-finance/assay.git"},
		{match: "deskwt add", stderr: "git worktree add failed: could not create branch feat/item-1", code: deskkit.ExitUnverifiable},
	}
	rc, stderr := runCapturingStderr(t, []string{"item-1", "--root", root,
		"--prompt-file", filepath.Join(t.TempDir(), "p.md")})
	if rc != deskkit.ExitUnverifiable {
		t.Errorf("rc = %d, want %d", rc, deskkit.ExitUnverifiable)
	}
	if report := stepReport(t, stderr); !strings.Contains(report, "could not create branch feat/item-1") {
		t.Errorf("the step report does not carry deskwt's message:\n%s", report)
	}
}

func TestToolMessageKeepsEverythingButTheKnownPreamble(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"config echo is preamble", deskwtStderr,
			"refused: branch feat/item-1 already exists and is CHECKED OUT in the worktree " +
				"/private/tmp/tracker-item-1 — that worktree owns it, so it is not a leftover."},
		{"multi-line message is kept whole", "assay-config: x=1\nline one\nline two",
			"line one\nline two"},
		// When the ROSTER is what refused, that line IS the message, not preamble.
		{"roster refusal is the message", "assay-config: class=write source=env\nassay-config: REFUSED — bad roster",
			"assay-config: REFUSED — bad roster"},
		{"nothing but preamble renders as no output", "assay-config: x=1", "(no output)"},
		{"empty renders as no output", "", "(no output)"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := toolMessage(c.in); got != c.want {
				t.Errorf("toolMessage() = %q, want %q", got, c.want)
			}
		})
	}
}
