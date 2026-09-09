package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// goclaimfallback_test.go — issue 708. When a repo carries no tools/dispatch-claim.sh,
// deskdispatch falls back to the pure-Go claim binary (goClaimBinary, cmd/deskclaim-ref)
// resolved on PATH. That fallback is the ONLY claim path that runs on a native-Windows
// adopter: a bare executable CreateProcess resolves through PATH, with no shebang `.sh` and
// no bash association. These tests pin the fallback, the fail-closed when neither tool
// exists, and the Windows-viable exec shape (no script, no shell).

// withGoClaimOnPath makes goClaimBinary resolvable via the lookPath seam, the way the
// installer places it on PATH alongside the other desk-tools binaries.
func withGoClaimOnPath(t *testing.T) {
	t.Helper()
	old := lookPath
	lookPath = func(name string) (string, error) {
		if name == goClaimBinary {
			return "/opt/desk-tools/bin/" + goClaimBinary, nil
		}
		return "", exec.ErrNotFound
	}
	t.Cleanup(func() { lookPath = old })
}

// A green-field tree (no consumer script) dispatches through the Go claim binary, invoked by
// bare name — the exact shape that works under Windows CreateProcess.
func TestDispatchFallsBackToTheGoClaimBinaryWhenNoScriptIsPresent(t *testing.T) {
	s := &stub{}
	_, root := s.install(t) // plant NO scripts
	withGoClaimOnPath(t)
	s.replies = happyReplies("/private/tmp/worker-home")

	promptFile := filepath.Join(t.TempDir(), "p.md")
	rc := run([]string{"example--stream--07", "--root", root, "--repo", allowedRepo,
		"--prompt-file", promptFile})
	if rc != deskkit.ExitOK {
		t.Fatalf("green-field dispatch rc = %d, want 0 — the Go fallback did not carry the claim", rc)
	}
	if !s.ran(goClaimBinary + " acquire example--stream--07") {
		t.Errorf("the Go claim binary was not invoked for acquire; calls: %v", s.calls)
	}
	if s.ran("dispatch-claim.sh") {
		t.Error("a .sh claim script was invoked on a tree that carries none")
	}
	// The claim tool is a BARE executable name (PATH-resolved, CreateProcess-friendly) — never
	// a filesystem path, never a shell/script. This is the Windows-viable exec property.
	for _, c := range s.calls {
		if len(c) >= 2 && c[1] == "acquire" {
			if c[0] != goClaimBinary {
				t.Errorf("claim tool argv[0] = %q, want the bare binary %q (a path/script does not run under "+
					"Windows CreateProcess)", c[0], goClaimBinary)
			}
			if strings.Contains(c[0], "/") || strings.HasSuffix(c[0], ".sh") {
				t.Errorf("the claim tool was invoked as a path/script %q, not a bare Windows-native binary", c[0])
			}
		}
	}

	body, err := os.ReadFile(promptFile)
	if err != nil {
		t.Fatalf("prompt: %v", err)
	}
	prompt := string(body)
	if !strings.Contains(prompt, goClaimBinary+" release") {
		t.Errorf("the release hint does not name the Go claim binary:\n%s", prompt)
	}
	if strings.Contains(prompt, "dispatch-claim.sh") {
		t.Error("the prompt still references the .sh script the tree does not carry")
	}
}

// With NEITHER a script on disk NOR the Go binary on PATH, the dispatch fails closed (exit 6)
// and the refusal names BOTH missing tools — a claim this verb cannot place is never
// permission to proceed.
func TestNoScriptAndNoGoBinaryFailsClosedNamingBoth(t *testing.T) {
	s := &stub{}
	_, root := s.install(t) // no scripts; install's default lookPath finds no binary
	s.replies = happyReplies("/private/tmp/worker-home")

	err := cmdDispatch([]string{"item-1", "--root", root, "--repo", allowedRepo})
	if err == nil {
		t.Fatal("a dispatch with no claim tool at all returned nil")
	}
	if deskkit.ExitCodeOf(err) != deskkit.ExitUnverifiable {
		t.Fatalf("no-claim-tool rc = %d, want %d (unverifiable)", deskkit.ExitCodeOf(err), deskkit.ExitUnverifiable)
	}
	for _, want := range []string{"dispatch-claim.sh", goClaimBinary} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the fail-closed refusal does not name %q: %s", want, err.Error())
		}
	}
	if len(s.calls) != 0 {
		t.Fatalf("the refusal came after %d child process(es): %v — nothing may run without a claim tool", len(s.calls), s.calls)
	}
}

// The legacy script still WINS when present: a consumer mid-transition keeps its exact
// current behavior (the .sh is invoked, the release hint is the repo-relative path), and the
// Go binary is not consulted. This is what lets a Go and a bash dispatcher coexist without
// double-dispatching — they collide on the same refs/dispatch ref either way.
func TestLegacyScriptIsPreferredOverTheGoBinaryWhenBothExist(t *testing.T) {
	s := &stub{}
	_, root := s.install(t)
	plantScripts(t, root) // the tree DOES carry the .sh
	withGoClaimOnPath(t)  // and the Go binary is also available
	s.replies = happyReplies("/private/tmp/worker-home")

	promptFile := filepath.Join(t.TempDir(), "p.md")
	if rc := run([]string{"example--stream--07", "--root", root, "--repo", allowedRepo, "--prompt-file", promptFile}); rc != deskkit.ExitOK {
		t.Fatalf("rc = %d, want 0", rc)
	}
	if !s.ran("dispatch-claim.sh acquire example--stream--07") {
		t.Error("the legacy script was not preferred when present")
	}
	if s.ran(goClaimBinary + " acquire") {
		t.Error("the Go binary was invoked despite the legacy script being present")
	}
	body, _ := os.ReadFile(promptFile)
	if !strings.Contains(string(body), claimScriptRel+" release") {
		t.Errorf("the release hint is not the repo-relative script path:\n%s", string(body))
	}
}
