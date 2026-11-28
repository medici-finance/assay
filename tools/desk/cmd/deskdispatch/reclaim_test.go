package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// regression: #3724
// Exercise both entry points with real claim writes to a local bare repository.
// The worker services use the dispatch fixtures, including attestation readback.
func TestReclaimedLeaseWorkerDispatch(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "deskclaim-ref.test")
	build := exec.Command("go", "test", "-c", "-o", binary, "../deskclaim-ref")
	build.Env = append(os.Environ(), "GOPROXY=off")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build claim fixture: %v: %s", err, output)
	}
	bare := t.TempDir()
	if output, err := exec.Command("git", "init", "--bare", "-q", bare).CombinedOutput(); err != nil {
		t.Fatalf("init claim repository: %v: %s", err, output)
	}
	claim := func(args ...string) *exec.Cmd {
		return exec.Command(binary, append([]string{"local-claim-fixture", bare}, args...)...)
	}
	s := &stub{}
	_, root := s.install(t)
	plantScripts(t, root)
	s.replies = happyReplies(t.TempDir())
	attest := stubWorkerAttest(t, nil)
	baseExec, baseLook := execCommand, lookPath
	execCommand = func(name string, args ...string) *exec.Cmd {
		if name == goClaimBinary {
			return claim(args...)
		}
		return baseExec(name, args...)
	}
	lookPath = func(name string) (string, error) {
		if name == goClaimBinary {
			return binary, nil
		}
		return baseLook(name)
	}
	prompt := filepath.Join(t.TempDir(), "prompt.md")
	args := attestArgs(root, prompt)
	id := args[0]
	runClaim := func(want int, verb string, flags ...string) {
		t.Helper()
		cmd := claim(append([]string{verb, id, "--repo", allowedRepo}, flags...)...)
		output, err := cmd.CombinedOutput()
		if got := exitCodeOf(err); got != want {
			t.Fatalf("%s: exit %d, want %d: %s", verb, got, want, output)
		}
	}
	runClaim(0, "acquire", "--owner", "former")
	t.Setenv("DESK_SESSION", "returning")
	runClaim(0, "steal", "--reason", "previous worker stopped")

	for _, session := range []string{"other", "returning"} {
		t.Setenv("DESK_SESSION", session)
		if session == "other" {
			runClaim(deskkit.ExitRefused, "release")
		}
		_, diagnostic, code := ddRunCapture(t, args)
		if code != deskkit.ExitRefused || !strings.Contains(diagnostic, "returning") {
			t.Fatalf("dispatch as %s before release: %d, %s", session, code, diagnostic)
		}
		if s.ran("deskwt add") || len(*attest) != 0 {
			t.Fatal("blocked dispatch created a worktree or requested attestation")
		}
		if _, err := os.Stat(emittedPathFor(prompt)); !os.IsNotExist(err) {
			t.Fatalf("blocked dispatch prompt: %v", err)
		}
	}

	runClaim(0, "release")
	_, diagnostic, code := ddRunCapture(t, args)
	if code != 0 || !s.ran("deskwt add") || len(*attest) != 1 {
		t.Fatalf("worker dispatch after release: rc=%d attestations=%d: %s", code, len(*attest), diagnostic)
	}
	body, err := os.ReadFile(emittedPathFor(prompt))
	if err != nil || !strings.Contains(string(body), workerReceiptFixture("gpt-6.1-sol").PRBinding()) {
		t.Fatalf("expected the attested worker assignment, read error: %v", err)
	}
	runClaim(deskkit.ExitRefused, "acquire")
}
