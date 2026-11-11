package main

import (
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// hintMint is the desktoken invocation a printed claim command carries.
var hintMint = regexp.MustCompile(`desktoken --no-rotate (\S+) --repo \S+?(?: --cross-role "([^"]*)")?\)`)

// admittedAsPrinted reports whether desktoken would admit the mint in hint from a shell on
// loop: the loop takes the role, or the hint carries a non-blank --cross-role reason. This is
// desktoken's own admission rule (#6069), read from the same deskkit check.
func admittedAsPrinted(t *testing.T, loop, hint string) (bool, string) {
	t.Helper()
	m := hintMint.FindStringSubmatch(hint)
	if m == nil {
		t.Fatalf("no desktoken mint in the printed hint: %q", hint)
	}
	t.Setenv("DESK_LOOP", loop)
	_, _, err := deskkit.RequireLoopRole("desktoken", m[1])
	if err == nil {
		return m[2] == "", "admitted by the loop; override present=" + boolText(m[2] != "")
	}
	if deskkit.ExitCodeOf(err) != deskkit.ExitRefused {
		return false, "could not check: " + err.Error()
	}
	return strings.TrimSpace(m[2]) != "", "refused by the loop; override=" + m[2]
}

func boolText(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

// Every printed claim credential runs as printed from the shell that reads it: the
// dispatching window for its own hints, the agent's loop for the release line. An override
// appears only when that loop does not take the role.
// regression: #6069
func TestClaimHintRunsAsPrinted(t *testing.T) {
	loops := []string{"the-desk", "worker-desk", "pr-review-desk", "verify-desk", "intake-desk"}
	for _, loop := range loops {
		for _, role := range []string{"desk", "worker", "reviewer", "verifier"} {
			hint := claimCredArg(goClaimBinary, role, allowedRepo, loop)
			if ok, why := admittedAsPrinted(t, loop, hint); !ok {
				t.Errorf("loop %s, role %s: printed hint does not run as printed (%s):\n%s", loop, role, why, hint)
			}
		}
	}
	// The legacy script reads the ambient credential: no flag, so nothing to admit.
	if got := claimCredArg(claimScriptRel, "desk", allowedRepo, "worker-desk"); got != "" {
		t.Errorf("legacy script hint carries a credential argument: %q", got)
	}
}

// A worker-desk window dispatches a worker and claims as the desk App. Its NOT LAUNCHED
// progress and release commands must run from that window as printed.
// regression: #6069
func TestWorkerDeskHintAdmitted(t *testing.T) {
	s := &stub{}
	_, root := s.install(t)
	withGoClaimOnPath(t)
	t.Setenv("DESK_LOOP", "worker-desk")
	s.replies = happyReplies(filepath.Join(t.TempDir(), "worker-home"))
	rc, stderr := runCapturingStderr(t, []string{"example-stream--07", "--root", root, "--repo", allowedRepo,
		"--prompt-file", filepath.Join(t.TempDir(), "p.md")})
	if rc != deskkit.ExitOK {
		t.Fatalf("dispatch rc = %d, want 0\n%s", rc, stderr)
	}
	line := lineWith(stderr, "NOT LAUNCHED")
	hints := hintMint.FindAllString(line, -1)
	if len(hints) != 2 {
		t.Fatalf("NOT LAUNCHED line carries %d desktoken mint(s), want 2:\n%s", len(hints), line)
	}
	for _, h := range hints {
		if ok, why := admittedAsPrinted(t, "worker-desk", h); !ok {
			t.Errorf("worker-desk NOT LAUNCHED hint refused as printed (%s): %s", why, h)
		}
	}
}
