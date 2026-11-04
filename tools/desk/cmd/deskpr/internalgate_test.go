package main

import (
	"io"
	"os"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// internalgate_test.go — #5436. A GitLab `internal` project still listed `:public` in the
// allowed-repos set is refused by the public-repo gate at the live create (correctly: the
// configured claim must name the live value). These tests pin that the mismatch is visible
// BEFORE the live create: offline --check names --check-remote as the preflight that runs
// the gate, and --check-remote runs the REAL gate and refuses with the live create's text.

// captureStdoutPR runs fn with os.Stdout redirected and returns what it printed.
func captureStdoutPR(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stdout = w
	done := make(chan string, 1)
	go func() {
		var b strings.Builder
		_, _ = io.Copy(&b, r)
		done <- b.String()
	}()
	defer func() { os.Stdout = old }()
	fn()
	_ = w.Close()
	return <-done
}

// internalVisFetcher reports every repo live-internal, as GitLab does for an internal project.
type internalVisFetcher struct{}

func (internalVisFetcher) RepoVisibility(owner, repo string) (string, error) { return "internal", nil }

// regression: #5436
func TestCheckNamesRemoteGate(t *testing.T) {
	work := newBaseFixture(t)
	calls := withEnv(t, work)
	var rc int
	out := captureStdoutPR(t, func() {
		rc = run([]string{"create", "--title", "check gate line", "--body-min", "Issue: #5436\nbody", "--check"})
	})
	if rc != deskkit.ExitOK {
		t.Fatalf("create --check rc = %d, want 0\n%s", rc, out)
	}
	if !strings.Contains(out, "the public-repo authorization gate") || !strings.Contains(out, "--check-remote runs it") {
		t.Fatalf("--check must say the public-repo gate is not checked AND that --check-remote runs it; got:\n%s", out)
	}
	// --check stays offline: no visibility read, no push, no create.
	if curForge.visibilityCalls != 0 || anyPush(*calls) || curForge.createCalls != 0 {
		t.Fatalf("--check reached the forge (visibility reads %d, creates %d) or pushed", curForge.visibilityCalls, curForge.createCalls)
	}
}

// regression: #5436
func TestRemoteCheckStalePublicEntry(t *testing.T) {
	const repo = "example-org/tracker"
	for _, tc := range []struct {
		name, entry string
		want        int
	}{
		{"public entry, live internal", repo + ":ci:public", deskkit.ExitRefused},
		{"internal entry, live internal", repo + ":ci:internal", deskkit.ExitOK},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, calls, _ := evidenceFixture(t)
			installGateRoster(t, tc.entry)
			publicRepoGateFn = productionGateFn // the REAL gate, not withEnv's no-op stub
			curForge.visibility = "internal"

			// The refusal the gate itself produces for this (live, configured) pair.
			wantErr := deskkit.PublicRepoGate(internalVisFetcher{}, "example-org", "tracker")

			var rc int
			stderr := captureStderr(t, func() {
				rc = run([]string{"create", "--title", "remote gate", "--body-min", "Issue: #5436\nbody", "--check-remote"})
			})
			if rc != tc.want {
				t.Fatalf("--check-remote rc = %d, want %d\n%s", rc, tc.want, stderr)
			}
			if curForge.visibilityCalls != 1 {
				t.Fatalf("--check-remote read live visibility %d times, want 1", curForge.visibilityCalls)
			}
			if anyPush(*calls) || curForge.created != nil {
				t.Fatal("--check-remote caused an outward write")
			}
			if tc.want == deskkit.ExitOK {
				if wantErr != nil {
					t.Fatalf("the gate refused a :internal entry on a live-internal repo: %v", wantErr)
				}
				return
			}
			if wantErr == nil || !strings.Contains(stderr, wantErr.Error()) {
				t.Fatalf("--check-remote refusal is not the gate's own text\nwant: %v\ngot:  %s", wantErr, stderr)
			}
			if !strings.Contains(stderr, repo+":internal") || !strings.Contains(stderr, "if this :public entry was written before it existed") {
				t.Fatalf("refusal must name the :internal replacement and the entry's history; got:\n%s", stderr)
			}

			// The live create refuses with the SAME text, before any push.
			var liveRC int
			liveErr := captureStderr(t, func() {
				liveRC = run([]string{"create", "--title", "remote gate", "--body-min", "Issue: #5436\nbody"})
			})
			if liveRC != deskkit.ExitRefused || !strings.Contains(liveErr, wantErr.Error()) {
				t.Fatalf("live create rc = %d, want 5 with the same refusal; got:\n%s", liveRC, liveErr)
			}
			if anyPush(*calls) || curForge.created != nil {
				t.Fatal("a refused live create pushed or opened a PR")
			}
		})
	}
}
