package main

// flow_50_test.go — the shipped prepare path binds the run's prerequisite
// record, and the landing admission consumes it (example-stream/50).
//
// The planner is bypassed: the run is prepared by the real deskdispatch verifier
// dispatch and admitted by the real Evidence admission, the two verbs a verifier
// session actually uses. A record that only the planner persisted would leave
// both of these blind to a moved input.

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

func world50Git(t *testing.T, dir string, args ...string) {
	t.Helper()
	c := exec.Command("git", append([]string{"-C", dir}, args...)...)
	if out, err := c.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %s %v", args, out, err)
	}
}

// world50Dispatch prepares one verifier run whose brief declares a sibling
// checkout and returns the verifier home and that sibling.
func world50Dispatch(t *testing.T, attest func(dispatchOpts, string, string) (string, error)) (home, sibling string) {
	t.Helper()
	home, sibling, rc, _ := world50Try(t, attest)
	if rc != 0 {
		t.Fatalf("verifier dispatch rc=%d", rc)
	}
	return home, sibling
}

// world50Try is world50Dispatch without the success requirement: it returns the
// dispatch's exit code and the prompt file it would have written.
func world50Try(t *testing.T, attest func(dispatchOpts, string, string) (string, error)) (home, sibling string, rc int, prompt string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("POSIX fixture")
	}
	s := &stub{}
	deskHome, root := s.install(t)
	plantScripts(t, root)
	pinFixtureForge(t, deskHome, allowedRepo)
	t.Setenv("DESK_LOOP", "verify-desk")
	t.Setenv("DESK_SESSION", "fixture-shared-claim-owner")
	attestVerifierDispatchFn = attest
	t.Cleanup(func() { attestVerifierDispatchFn = attestVerifierDispatch })

	parent := t.TempDir()
	home, sibling = filepath.Join(parent, "home"), filepath.Join(parent, "peer")
	for _, dir := range []string{home, sibling} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		world50Git(t, dir, "init")
		world50Git(t, dir, "config", "user.name", "fixture")
		world50Git(t, dir, "config", "user.email", "fixture@example.invalid")
	}
	world50Git(t, home, "remote", "add", "origin", "https://github.com/"+allowedRepo+".git")
	if err := os.WriteFile(filepath.Join(sibling, "data.txt"), []byte("one\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	world50Git(t, sibling, "add", "data.txt")
	world50Git(t, sibling, "commit", "-m", "peer")

	body := "# Fixture\n\nout-of-repo files: ../peer/data.txt\n\n## Verify\n\n| # | Command | Expect |\n|---|---|---|\n| 1 | `true` | exit 0 |\n\n## Evidence\n\nPending.\n"
	if err := os.WriteFile(filepath.Join(home, "spec.md"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	world50Git(t, home, "add", "spec.md")
	world50Git(t, home, "commit", "-m", "fixture")
	world50Git(t, home, "update-ref", "refs/remotes/origin/main", "HEAD")
	world50Git(t, home, "checkout", "--detach")

	s.replies = happyReplies(home)
	gh := installGHStamp(t)
	verifierGHFixture(t, gh)
	mintTokenFn = func(role, repo string) (string, string, error) {
		return stubMintedToken, stubMintedTokenPath(t, t.TempDir()), nil
	}
	prompt = filepath.Join(t.TempDir(), "prompt.md")
	rc = run([]string{"example/01", "--root", root, "--repo", allowedRepo, "--kit", "verifier", "--model", "gpt-6-astra", "--tier", "strong", "--brief", "spec.md", "--prompt-file", prompt})
	return home, sibling, rc, prompt
}

// regression: example-stream/50 — the shipped prepare path persisted no
// record, so the Evidence admission skipped its recheck and a moved sibling
// landed under a result produced against another world.
func TestFlowWorldPreparePath(t *testing.T) {
	t.Run("dispatch binds the declared sibling to the run", func(t *testing.T) {
		home, _ := world50Dispatch(t, attestVerifierDispatch)
		r, err := deskkit.CheckVerifierEvidence(home, allowedRepo, "spec.md")
		if err != nil {
			t.Fatalf("an unmoved world must land: %v", err)
		}
		if r.World == nil || r.World.Inputs["sibling:../peer"] == "" || r.World.Inputs[deskkit.WorldSource] != r.Binding.Source {
			t.Fatalf("prepare persisted no usable record: %+v", r.World)
		}
	})

	t.Run("landing refuses a sibling moved after dispatch", func(t *testing.T) {
		home, sibling := world50Dispatch(t, attestVerifierDispatch)
		if err := os.WriteFile(filepath.Join(sibling, "data.txt"), []byte("two\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		world50Git(t, sibling, "commit", "-am", "moved")
		_, err := deskkit.CheckVerifierEvidence(home, allowedRepo, "spec.md")
		if err == nil || !strings.Contains(err.Error(), "environment changed since admission: sibling:../peer") {
			t.Fatalf("a moved sibling landed: %v", err)
		}
		if _, err := deskkit.CheckVerifierAttestation(home, "spec.md"); err == nil {
			t.Fatal("a moved sibling was admitted for execution")
		}
	})

	t.Run("recovery of a half-prepared run binds the same record", func(t *testing.T) {
		// A dispatch that stopped after the stamp: prepared and recovered, never bound.
		home, sibling := world50Dispatch(t, func(o dispatchOpts, repo, home string) (string, error) {
			brief := o.brief
			if filepath.IsAbs(brief) && vpWithin(o.root, brief) {
				brief, _ = filepath.Rel(o.root, brief)
			}
			if err := deskkit.PrepareVerifierAttestation(home, repo, brief, o.model, o.tier); err != nil {
				return "", err
			}
			r, err := deskkit.RecoverVerifierAttestation(home)
			return "OK: " + r.EvidenceBinding(), err
		})
		if r, err := deskkit.CheckVerifierEvidence(home, allowedRepo, "spec.md"); err != nil || r.World != nil {
			t.Fatalf("fixture must start unbound: world=%v err=%v", r.World, err)
		}
		t.Setenv("DESK_LOOP", "the-desk") // recovery runs under the coordinator desk's custody
		if err := cmdVerifierAttestation("--attest-verifier", []string{"--root", home}); err != nil {
			t.Fatalf("recovery: %v", err)
		}
		world50Git(t, sibling, "commit", "--allow-empty", "-m", "moved")
		if _, err := deskkit.CheckVerifierEvidence(home, allowedRepo, "spec.md"); err == nil ||
			!strings.Contains(err.Error(), "environment changed since admission: sibling:../peer") {
			t.Fatalf("recovered run is not bound: %v", err)
		}
	})

	t.Run("a missing declared sibling stops the dispatch before a prompt", func(t *testing.T) {
		// The class, planted a second time: a run whose declared world cannot be read.
		_, _, rc, prompt := world50Try(t, func(o dispatchOpts, repo, home string) (string, error) {
			if err := os.RemoveAll(filepath.Join(filepath.Dir(home), "peer")); err != nil {
				return "", err
			}
			return attestVerifierDispatch(o, repo, home)
		})
		if rc != deskkit.ExitUnverifiable {
			t.Fatalf("an unreadable declared world did not stop the dispatch: rc=%d", rc)
		}
		if _, err := os.Stat(emittedPathFor(prompt)); err == nil {
			t.Fatal("a prompt was emitted for a world that could not be recorded")
		}
	})
}
