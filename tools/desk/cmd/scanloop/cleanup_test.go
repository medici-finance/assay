package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// cleanup_test.go — a PUBLISHED carrier survives a failed cleanup.
//
// The incident: a pass created the draft carrier PR, then `git worktree remove` refused the scan
// worktree ("contains modified or untracked files"), and the lane returned that as a dispatch
// error. The exit ledger then printed "nothing landed this pass" and the leak check named the
// source issue as having NO exit — while the carrier PR sat open on the forge. The class: any
// failure AFTER the outward write turned a landed exit into a reported leak.

// failingExec answers every command with success except the ones whose rendered "name args…"
// starts with a prefix in fail.
func failingExec(fail ...string) (Exec, *[]string) {
	var ran []string
	return func(_ string, name string, args ...string) (string, error) {
		line := name + " " + strings.Join(args, " ")
		ran = append(ran, line)
		for _, f := range fail {
			if strings.HasPrefix(line, f) {
				return "", errors.New("exit status 128: " + f + " refused")
			}
		}
		return "", nil
	}, &ran
}

const (
	failRemove = "git worktree remove"
	failDetach = "git --git-dir="
)

// TestScanCarrierLane_CleanupFailureAfterPublish is the table over the publish/cleanup boundary.
func TestScanCarrierLane_CleanupFailureAfterPublish(t *testing.T) {
	for _, tc := range []struct {
		name         string
		open         *OpenScanPR
		fail         []string
		wantErr      bool
		wantArtifact string
		wantWarnings []string // substrings, one per expected warning, in order
		wantRemove   bool     // the remove step was attempted
	}{
		{
			name:         "fresh: remove ok — unchanged",
			wantArtifact: "medici-finance/assay (new draft scan PR)",
			wantRemove:   true,
		},
		{
			name:         "coalesce: remove ok — unchanged",
			open:         openPR(3 * time.Minute),
			wantArtifact: "medici-finance/assay#42",
			wantRemove:   true,
		},
		{
			name:         "fresh: published, remove fails — exit stands, warning on its own line",
			fail:         []string{failRemove},
			wantArtifact: "medici-finance/assay (new draft scan PR)",
			wantWarnings: []string{"cleanup: could not remove worktree"},
			wantRemove:   true,
		},
		{
			name:         "coalesce: published, remove fails — exit stands, warning on its own line",
			open:         openPR(3 * time.Minute),
			fail:         []string{failRemove},
			wantArtifact: "medici-finance/assay#42",
			wantWarnings: []string{"cleanup: could not remove worktree"},
			wantRemove:   true,
		},
		{
			name:         "published, remove AND detach fail — both reported, still no error",
			fail:         []string{failRemove, failDetach},
			wantArtifact: "medici-finance/assay (new draft scan PR)",
			wantWarnings: []string{"cleanup: could not remove worktree", "cleanup: could not detach the retained worktree"},
			wantRemove:   true,
		},
		{
			name:    "fresh: publish fails — error as before, cleanup not reached",
			fail:    []string{"deskpr create"},
			wantErr: true,
		},
		{
			name:    "coalesce: publish fails — error as before, cleanup not reached",
			open:    openPR(3 * time.Minute),
			fail:    []string{"deskpr edit"},
			wantErr: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			x, ran := failingExec(tc.fail...)
			req := laneReq(t, scanItem("medici-finance/assay", string(LaneScanCarrierPR)), tc.open)
			req.DryRun = false
			if tc.open != nil {
				req.Branch = tc.open.Branch
			}
			out, err := scanCarrierPRLane{exec: x, write: writeTextFile}.Execute(req)

			removed := false
			for _, l := range *ran {
				if strings.HasPrefix(l, failRemove) {
					removed = true
				}
			}
			if removed != tc.wantRemove {
				t.Fatalf("remove attempted = %v, want %v:\n%s", removed, tc.wantRemove, strings.Join(*ran, "\n"))
			}
			if tc.wantErr {
				if err == nil {
					t.Fatalf("a failed publish produced no error — it must stay a dispatch error with no exit")
				}
				return
			}
			if err != nil {
				t.Fatalf("the lane failed after the carrier was published: %v — the ledger would report a leak for a PR that exists", err)
			}
			if out.Artifact != tc.wantArtifact {
				t.Fatalf("artifact = %q, want %q", out.Artifact, tc.wantArtifact)
			}
			if out.Exit != ExitPlaceholder {
				t.Fatalf("exit = %s, want %s", out.Exit, ExitPlaceholder)
			}
			if len(out.Warnings) != len(tc.wantWarnings) {
				t.Fatalf("warnings = %q, want %d", out.Warnings, len(tc.wantWarnings))
			}
			steps := strings.Join(out.Steps, "\n")
			for i, want := range tc.wantWarnings {
				if !strings.Contains(out.Warnings[i], want) {
					t.Fatalf("warning %d = %q, want it to contain %q", i, out.Warnings[i], want)
				}
				if !strings.Contains(steps, "# WARNING: "+want) {
					t.Fatalf("the warning is not on its own step line:\n%s", steps)
				}
			}
			if len(tc.wantWarnings) > 0 && !strings.Contains(out.Warnings[0], "worktree retained for diagnosis") {
				t.Fatalf("the cleanup warning does not say the worktree was retained: %q", out.Warnings[0])
			}
			for _, l := range *ran {
				if strings.Contains(l, "--force") {
					t.Fatalf("the lane forced a command after a cleanup failure: %s", l)
				}
			}
		})
	}
}

// TestDrainPass_PublishedCarrierWithFailedCleanupLandsItsExit — end to end through the pass: the
// item is recorded as exited, the leak check is clean, and the cleanup failure is its own report
// line. The pre-fix pass printed "nothing landed" and named the item as un-exited.
func TestDrainPass_PublishedCarrierWithFailedCleanupLandsItsExit(t *testing.T) {
	loop := passLoop(t, "INBOUND: medici-finance/assay#77 2026-08-24T11:50:00Z\n")
	x, _ := failingExec(failRemove)
	loop.Exec = x
	loop.Write = func(string, string) error { return nil }
	loop.DryRun = false

	var sb strings.Builder
	if err := drainPass(passConfig(t), loop, &sb); err != nil {
		t.Fatalf("a published carrier with a failed cleanup failed the pass: %v\n%s", err, sb.String())
	}
	recs := loop.Ledger().Records()
	if len(recs) != 1 || recs[0].Exit != ExitPlaceholder || recs[0].ItemID != "medici-finance/assay#77" {
		t.Fatalf("ledger = %+v, want the placeholder exit for #77", recs)
	}
	if leaked := loop.Ledger().Unexited(loop.exited()); len(leaked) != 0 {
		t.Fatalf("leak check = %v — the item left by its published carrier", leaked)
	}
	var rep strings.Builder
	renderPassReport(&rep, loop)
	report := rep.String()
	for _, want := range []string{"leak check: CLEAN", "WARNING: ", "cleanup: could not remove worktree"} {
		if !strings.Contains(report, want) {
			t.Fatalf("the pass report is missing %q:\n%s", want, report)
		}
	}
	if strings.Contains(report, "nothing landed") {
		t.Fatalf("the pass report says nothing landed for a published carrier:\n%s", report)
	}
}

// TestScanCarrierLane_RetainedWorktreeThenCoalesceRealGit — the retry half, against real git.
// Pass 1 publishes the carrier, then its cleanup fails on an untracked file and the worktree is
// retained. Pass 2 runs inside the coalesce window with the open carrier read fresh: it must push
// onto THAT carrier (one `deskpr create` across both passes), and the retained worktree must still
// hold the file that blocked its removal. Real git refuses to re-cut a branch that is still checked
// out in another worktree, so without the detach the retry fails before it publishes and the item
// leaks on every tick of the window.
func TestScanCarrierLane_RetainedWorktreeThenCoalesceRealGit(t *testing.T) {
	base := t.TempDir()
	origin := filepath.Join(base, "origin.git")
	root := filepath.Join(base, "target")
	scanGit(t, base, "init", "--bare", "-b", "main", origin)
	scanGit(t, base, "clone", origin, root)
	scanGit(t, root, "checkout", "-b", "main")
	if err := os.WriteFile(filepath.Join(root, "seed.txt"), []byte("seed\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	scanGit(t, root, "add", "seed.txt")
	scanGit(t, root, "commit", "-m", "seed")
	scanGit(t, root, "push", "origin", "main")

	const branch = "chore/intake-scan-2026-08-24-1140"
	const stray = "stray-untracked.txt"
	n, creates, updates := 0, 0, 0
	plantStray := true
	x := Exec(func(dir, name string, args ...string) (string, error) {
		switch name {
		case "git":
			return gitErr(t, dir, args...)
		case "statusgen":
			n++
			d := filepath.Join(dir, deskkit.ScanDir)
			if err := os.MkdirAll(d, 0o755); err != nil {
				return "", err
			}
			if plantStray {
				// Something the commit does not carry: it makes the final remove refuse.
				if err := os.WriteFile(filepath.Join(dir, stray), []byte("x\n"), 0o600); err != nil {
					return "", err
				}
			}
			return "", os.WriteFile(filepath.Join(d, fmt.Sprintf("delta-%d.md", n)), []byte("x\n"), 0o600)
		case "deskscanbody":
			return "derived\n", nil
		case "deskpr":
			switch args[0] {
			case "create":
				creates++
			case "update":
				updates++
			}
			if args[0] == "create" || args[0] == "update" {
				cur, err := gitErr(t, dir, "branch", "--show-current")
				if err != nil {
					return "", err
				}
				return gitErr(t, dir, "push", "origin", "HEAD:refs/heads/"+strings.TrimSpace(cur))
			}
			return "", nil
		}
		return "", fmt.Errorf("unexpected command %s", name)
	})
	lane := scanCarrierPRLane{exec: x, write: func(string, string) error { return nil }}
	it := scanItem("medici-finance/assay", string(LaneScanCarrierPR))
	req := func(wt string, open *OpenScanPR) LaneRequest {
		r := laneReq(t, it, open)
		r.Root, r.Worktree, r.DryRun, r.Branch = root, filepath.Join(base, wt), false, branch
		return r
	}

	out, err := lane.Execute(req("wt-pass-1", nil))
	if err != nil {
		t.Fatalf("pass 1 published the carrier and still failed: %v", err)
	}
	if len(out.Warnings) != 1 || !strings.Contains(out.Warnings[0], "could not remove worktree") {
		t.Fatalf("pass 1 warnings = %q, want exactly the cleanup warning (the detach must succeed)", out.Warnings)
	}
	retained := filepath.Join(base, "wt-pass-1")
	if _, err := os.Stat(filepath.Join(retained, stray)); err != nil {
		t.Fatalf("the retained worktree lost the file that blocked its removal: %v", err)
	}

	plantStray = false
	out, err = lane.Execute(req("wt-pass-2", openPR(3*time.Minute)))
	if err != nil {
		t.Fatalf("the retry after a retained worktree failed instead of coalescing: %v\n%s", err, strings.Join(out.Steps, "\n"))
	}
	if out.Decision != CoalesceInto || out.Artifact != "medici-finance/assay#42" {
		t.Fatalf("retry decision = %s artifact = %q, want COALESCE onto #42", out.Decision, out.Artifact)
	}
	if creates != 1 || updates != 1 {
		t.Fatalf("deskpr create = %d, update = %d — want one carrier created and the retry pushed onto it", creates, updates)
	}
	if got, _ := gitErr(t, origin, "ls-tree", "-r", "--name-only", branch); strings.Count(got, "delta-") != 2 {
		t.Fatalf("the carrier branch should hold both passes' deltas:\n%s", got)
	}
	if _, err := os.Stat(filepath.Join(retained, stray)); err != nil {
		t.Fatalf("the retry touched the retained worktree: %v", err)
	}
}

// gitErr runs real git like scanGit, but returns the failure instead of failing the test: the
// remove refusal is the event under test.
func gitErr(t *testing.T, dir string, args ...string) (string, error) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "HOME="+t.TempDir(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null",
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.test", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.test")
	b, err := cmd.CombinedOutput()
	if err != nil {
		return string(b), fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(b)))
	}
	return string(b), nil
}
