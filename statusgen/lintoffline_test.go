package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// lintoffline_test.go — forge-neutral/18 row 4 (TestLintOfflineMakesNoNetworkCall).
//
// A full `--lint`, with no `--forge`, run against a harness whose PATH contains NO `gh` and NO
// `deskread`, must complete with the same verdict as a networked run — because no forge-backed
// read this brief's offline default guards actually gets far enough to need either binary.
//
// WHY A HERMETIC PATH IS THE DIAL HOOK. statusgen makes no direct net/http call anywhere in the
// --lint path (the only net/http client in this module is the opt-in analytics telemetry
// client, gated behind BOTH --telemetry and ASSAY_TELEMETRY=1, neither of which this test
// sets — docs/telemetry.md). Every forge read — including the one THIS test proves still fires
// under plain --lint, dead-claim decay's `gh pr list` (claimdecay.go) — reaches the network
// exclusively by exec'ing a named binary ("gh" or "deskread") resolved off PATH. Go's os/exec
// resolves that name via LookPath before ever forking: when the name is not on PATH, Start/Run/
// Output return an error immediately and NO PROCESS IS CREATED — no fork, no exec, no socket.
// A PATH containing neither name is therefore not an approximation of "no network call was
// made" and "no forge process was started" — for THIS module's whole call shape, it is a
// datapoint. This is the fixture-repo test that made this refusal empirically true, in-tree.
//
// THE FIXTURE MUST ACTUALLY REACH THE GAP, NOT JUST AVOID IT. A tree with no .git (like
// testdata/goodrepo, used unmodified) never calls `git ls-remote` at all, so decayDeadClaims
// short-circuits on an EMPTY branch list (claimdecay.go: `if len(branches) == 0 { return
// branches, "" }`) and the gh call this row exists to catch is never attempted — a green run
// that proves nothing. This fixture adds a REAL "origin" remote (a local bare repo) carrying
// one branch, so `listRemoteBranches` returns non-empty and decayDeadClaims's `gh pr list`
// attempt is genuinely reached before the hermetic PATH stops it.
func TestLintOfflineMakesNoNetworkCall(t *testing.T) {
	gitBin, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git not on PATH — cannot build the fixture remote")
	}

	// A bare "remote" repo with one branch, so `git ls-remote --heads origin` (listRemoteBranches)
	// returns something non-empty and dead-claim decay is genuinely attempted.
	bareDir := t.TempDir()
	runGit(t, bareDir, "init", "--bare")
	seedDir := t.TempDir()
	gitInit(t, seedDir, "Seed Author", "seed@example.com")
	if err := os.WriteFile(filepath.Join(seedDir, "f.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, seedDir, "add", "f.txt")
	runGit(t, seedDir, "commit", "-m", "seed")
	runGit(t, seedDir, "branch", "-M", "fix/alpha-02-inflight")
	runGit(t, seedDir, "remote", "add", "origin", bareDir)
	runGit(t, seedDir, "push", "origin", "fix/alpha-02-inflight")

	// The fixture repo under test: the ordinary goodrepo docs/streams tree, made into a real git
	// repo whose OWN "origin" is the bare repo above — so it is the SAME "origin" listRemoteBranches
	// queries when run() calls resolveClaims on this root.
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS("testdata/goodrepo")); err != nil {
		t.Fatal(err)
	}
	gitInit(t, root, "Test Author", "test@example.com")
	runGit(t, root, "add", "-A")
	runGit(t, root, "commit", "-m", "initial")
	runGit(t, root, "remote", "add", "origin", bareDir)

	// The CONTROL PATH: git plus a FAKE-BUT-PRESENT "gh" that answers an empty PR list. This is
	// deliberately not a dependency on the real `gh` CLI being installed on whatever machine runs
	// this test (it may not be — see below): the only thing the control needs to prove is "when
	// `gh` IS resolvable, dead-claim decay actually invokes it," so a stub that succeeds cleanly
	// is a sharper, environment-independent proof than requiring the real CLI to be present.
	controlDir := t.TempDir()
	if err := os.Symlink(gitBin, filepath.Join(controlDir, "git")); err != nil {
		t.Fatal(err)
	}
	ghStub := "#!/bin/sh\necho '[]'\n"
	if err := os.WriteFile(filepath.Join(controlDir, "gh"), []byte(ghStub), 0o755); err != nil {
		t.Fatal(err)
	}

	// The HERMETIC PATH: git only — deliberately no "gh", no "deskread", nothing else.
	// exec.LookPath must fail closed for both.
	hermeticDir := t.TempDir()
	if err := os.Symlink(gitBin, filepath.Join(hermeticDir, "git")); err != nil {
		t.Fatal(err)
	}

	t.Run("control: gh present and resolvable — decay actually invokes it", func(t *testing.T) {
		t.Setenv("PATH", controlDir)
		if _, err := exec.LookPath("gh"); err != nil {
			t.Fatal("test setup broken: the stub `gh` is not resolvable on the control PATH")
		}
		var code int
		stderr := captureStderr(t, func() { code = run(root, "lint", nil, nil, "") })
		if code != 0 {
			t.Fatalf("lint exited %d against the control PATH", code)
		}
		if strings.Contains(stderr, "could-not-check: claims not decayed") {
			t.Fatalf("with a resolvable `gh` that answers cleanly, decay must NOT report could-not-check:\n%s", stderr)
		}
	})

	t.Setenv("PATH", hermeticDir)
	if _, err := exec.LookPath("gh"); err == nil {
		t.Fatal("test setup broken: `gh` is still resolvable on the hermetic PATH")
	}
	if _, err := exec.LookPath("deskread"); err == nil {
		t.Fatal("test setup broken: `deskread` is still resolvable on the hermetic PATH")
	}
	var offlineCode int
	offlineStderr := captureStderr(t, func() { offlineCode = run(root, "lint", nil, nil, "") })

	if offlineCode != 0 {
		t.Fatalf("hermetic-PATH lint exited %d, want 0 — the SAME verdict as the networked control (row 4)", offlineCode)
	}

	// Proof the gap was genuinely exercised (not a fixture that skipped decay entirely): the
	// hermetic run's stderr must show decay ATTEMPTED the gh read and failed CLOSED (fell back
	// to the full claim superset), naming an executable-not-found style reason — never silently
	// dropping the notice, and never treating the missing binary as "nothing to decay."
	if !strings.Contains(offlineStderr, "could-not-check: claims not decayed") {
		t.Fatalf("expected the dead-claim decay could-not-check NOTICE on stderr (proof the gh attempt was "+
			"genuinely reached and genuinely blocked by the hermetic PATH), got:\n%s", offlineStderr)
	}
	if !strings.Contains(offlineStderr, "executable file not found") {
		t.Errorf("expected the could-not-check reason to name an executable-not-found style failure "+
			"(proof no process was ever started, only a failed LOOKUP), got:\n%s", offlineStderr)
	}
}
