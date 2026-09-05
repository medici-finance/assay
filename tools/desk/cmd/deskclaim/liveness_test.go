package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// ---- fixtures ---------------------------------------------------------------

// initGitRepo makes a one-commit git repo in dir, on the default branch, with a hermetic
// env (no ambient user/global config). A branch never checked out here is simply absent from
// `git worktree list`, which is exactly the "no worktree holds it" signal the probe reads.
func initGitRepo(t *testing.T, dir string) {
	t.Helper()
	git := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null",
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.invalid",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.invalid",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	git("init", "-q")
	if err := os.WriteFile(filepath.Join(dir, "f"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	git("add", "-A")
	git("commit", "-qm", "init")
}

// gitCheckoutBranch leaves the repo's HEAD on `branch` (creating it), so `git worktree list`
// reports the worktree as holding refs/heads/<branch>.
func gitCheckoutBranch(t *testing.T, dir, branch string) {
	t.Helper()
	cmd := exec.Command("git", "checkout", "-qb", branch)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git checkout -b %s: %v\n%s", branch, err, out)
	}
}

func rosterDir(home string) string { return filepath.Join(home, ".config", "assay", "roster") }

// makeRosterDir creates an EMPTY roster dir. A readable roster dir with no beacon for a
// session is positive evidence the session is gone (beaconGone), the signal a reclaim needs.
func makeRosterDir(t *testing.T, home string) {
	t.Helper()
	if err := os.MkdirAll(rosterDir(home), 0o700); err != nil {
		t.Fatal(err)
	}
}

// writeBeacon plants a roster beacon for `owner` stamped at `updated`.
func writeBeacon(t *testing.T, home, owner string, updated time.Time) {
	t.Helper()
	makeRosterDir(t, home)
	body := fmt.Sprintf(`{"updated":%q}`, updated.UTC().Format(time.RFC3339))
	if err := os.WriteFile(filepath.Join(rosterDir(home), owner+".json"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

// writeClaim plants a canonical-shape claim for item and sets its mtime (the clock the
// staleness decision reads) via Chtimes — no sleeps.
func writeClaim(t *testing.T, home, item, owner, branch string, mtime time.Time) string {
	t.Helper()
	dir := filepath.Join(home, ".config", "assay", "claims")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	path := claimFile(home, item)
	body := fmt.Sprintf(`{"kind":"dispatch","item":%q,"owner":%q,"branch":%q,"ts":%q}`,
		item, owner, branch, mtime.UTC().Format(time.RFC3339))
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, mtime, mtime); err != nil {
		t.Fatal(err)
	}
	return path
}

// captureStdout runs fn with os.Stdout redirected to a pipe and returns what it printed.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	fn()
	_ = w.Close()
	os.Stdout = old
	b, _ := io.ReadAll(r)
	return string(b)
}

// lastAuditDetail returns the Detail of the last audit line matching verb (or "").
func lastAuditDetail(t *testing.T, home, verb string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(home, ".config", "assay", "audit.jsonl"))
	if err != nil {
		return ""
	}
	var detail string
	for _, ln := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		if ln == "" {
			continue
		}
		var e struct {
			Verb   string `json:"verb"`
			Detail string `json:"detail"`
		}
		if json.Unmarshal([]byte(ln), &e) == nil && e.Verb == verb {
			detail = e.Detail
		}
	}
	return detail
}

func aged(mins int) time.Time { return time.Now().Add(-time.Duration(mins) * time.Minute) }

// ---- Verify row 2 -----------------------------------------------------------

// TestStaleVerdictOldBranchCheckedOut: an aged --branch claim whose branch IS checked out in
// a worktree is LIVE. `stale` exits 5 with because=branch-checked-out:; `acquire` refuses;
// and the read-only `stale` leaves the claim file's bytes AND mtime untouched.
func TestStaleVerdictOldBranchCheckedOut(t *testing.T) {
	home := deskHome(t)
	repo := t.TempDir()
	initGitRepo(t, repo)
	gitCheckoutBranch(t, repo, "feat/x")
	makeRosterDir(t, home) // readable, but the worktree signal decides before the beacon

	path := writeClaim(t, home, "brief-a", "deadsession", "feat/x", aged(200))
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	fiBefore, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}

	var out string
	rc := 999
	out = captureStdout(t, func() {
		rc = run([]string{"stale", "--item", "brief-a", "--repo", repo})
	})
	if rc != deskkit.ExitRefused {
		t.Fatalf("stale rc = %d, want %d (live)", rc, deskkit.ExitRefused)
	}
	if !strings.Contains(out, "verdict=live") || !strings.Contains(out, "because=branch-checked-out:") {
		t.Fatalf("stale line = %q, want verdict=live because=branch-checked-out:", strings.TrimSpace(out))
	}

	// The read-only probe mutated nothing.
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatalf("stale changed the claim bytes: before=%q after=%q", before, after)
	}
	fiAfter, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if !fiBefore.ModTime().Equal(fiAfter.ModTime()) {
		t.Fatalf("stale changed the claim mtime: before=%s after=%s", fiBefore.ModTime(), fiAfter.ModTime())
	}

	// acquire must refuse the same live claim.
	if rc := run([]string{"acquire", "--kind", "dispatch", "--item", "brief-a", "--branch", "feat/x", "--repo", repo}); rc != deskkit.ExitRefused {
		t.Fatalf("acquire rc = %d, want %d (refused — branch is live)", rc, deskkit.ExitRefused)
	}
}

// ---- Verify row 3 -----------------------------------------------------------

// TestAcquireReclaimsOldUnheldBranchClaim: an aged --branch claim whose branch is checked out
// in NO worktree and whose owner has no roster beacon is reclaimable. `stale` exits 0;
// `acquire` succeeds and its audit line says reclaimed with age= and prior-owner=.
func TestAcquireReclaimsOldUnheldBranchClaim(t *testing.T) {
	home := deskHome(t)
	repo := t.TempDir()
	initGitRepo(t, repo) // default branch; feat/x is checked out in NO worktree
	makeRosterDir(t, home)

	writeClaim(t, home, "brief-b", "deadsession", "feat/x", aged(150))

	var out string
	rc := 999
	out = captureStdout(t, func() { rc = run([]string{"stale", "--item", "brief-b", "--repo", repo}) })
	if rc != deskkit.ExitOK {
		t.Fatalf("stale rc = %d, want 0 (stale/reclaimable)", rc)
	}
	if !strings.Contains(out, "verdict=stale") || !strings.Contains(out, "because=old-no-live-signal") {
		t.Fatalf("stale line = %q, want verdict=stale because=old-no-live-signal", strings.TrimSpace(out))
	}

	rc = run([]string{"acquire", "--kind", "dispatch", "--item", "brief-b", "--branch", "feat/y", "--owner", "newsession", "--repo", repo})
	if rc != deskkit.ExitOK {
		t.Fatalf("acquire rc = %d, want 0 (reclaimed)", rc)
	}
	detail := lastAuditDetail(t, home, "acquire")
	if !strings.Contains(detail, "reclaimed age=") || !strings.Contains(detail, "prior-owner=deadsession") {
		t.Fatalf("acquire audit detail = %q, want 'reclaimed age=' and 'prior-owner=deadsession'", detail)
	}
}

// ---- Verify row 4 -----------------------------------------------------------

// TestYoungClaimIsLiveWhateverTheSignals: a claim inside its TTL is LIVE even with every
// liveness signal absent (no repo, no beacon). The age floor is consulted before the probe.
func TestYoungClaimIsLiveWhateverTheSignals(t *testing.T) {
	home := deskHome(t)
	nonRepo := t.TempDir() // not a git repo → the worktree signal is unreadable
	// no roster dir → the beacon signal is unreadable

	writeClaim(t, home, "brief-c", "someone", "feat/x", aged(5)) // 5m < 120m TTL

	var out string
	rc := 999
	out = captureStdout(t, func() { rc = run([]string{"stale", "--item", "brief-c", "--repo", nonRepo}) })
	if rc != deskkit.ExitRefused {
		t.Fatalf("stale rc = %d, want %d (live — inside TTL)", rc, deskkit.ExitRefused)
	}
	if !strings.Contains(out, "verdict=live") || !strings.Contains(out, "because=age-under-ttl") {
		t.Fatalf("stale line = %q, want verdict=live because=age-under-ttl", strings.TrimSpace(out))
	}
	// And acquire refuses (collision) — the young claim is not stolen.
	if rc := run([]string{"acquire", "--kind", "dispatch", "--item", "brief-c", "--branch", "feat/z", "--repo", nonRepo}); rc != deskkit.ExitRefused {
		t.Fatalf("acquire rc = %d, want %d (refused — young claim)", rc, deskkit.ExitRefused)
	}
}

// ---- Verify row 5 -----------------------------------------------------------

// TestProbeFailsClosedWithoutRepoOrBeaconDir: with the claim aged past TTL, an UNREADABLE
// liveness signal must answer ACTIVE (live, exit 5), never stale. Two arms:
//
//	(a) --repo is not a git repo → the worktree signal is unreadable → cannot prove;
//	(b) --repo is a real repo (branch not held) but the beacon dir is absent → cannot prove.
func TestProbeFailsClosedWithoutRepoOrBeaconDir(t *testing.T) {
	// (a) no readable repo signal.
	t.Run("no-repo", func(t *testing.T) {
		home := deskHome(t)
		nonRepo := t.TempDir()
		makeRosterDir(t, home) // beacon readable+absent, but the worktree signal fails first
		writeClaim(t, home, "brief-d", "deadsession", "feat/x", aged(200))

		var out string
		rc := 999
		out = captureStdout(t, func() { rc = run([]string{"stale", "--item", "brief-d", "--repo", nonRepo}) })
		if rc != deskkit.ExitRefused {
			t.Fatalf("stale rc = %d, want %d (live — cannot prove without a repo)", rc, deskkit.ExitRefused)
		}
		if !strings.Contains(out, "verdict=live") || !strings.Contains(out, "because=no-repo-cannot-prove") {
			t.Fatalf("stale line = %q, want verdict=live because=no-repo-cannot-prove", strings.TrimSpace(out))
		}
		if rc := run([]string{"acquire", "--kind", "dispatch", "--item", "brief-d", "--branch", "feat/x", "--repo", nonRepo}); rc != deskkit.ExitRefused {
			t.Fatalf("acquire rc = %d, want %d — must not reclaim when it cannot prove", rc, deskkit.ExitRefused)
		}
	})

	// (b) repo readable, branch not held, but the beacon signal is unreadable (no roster dir).
	t.Run("no-beacon-dir", func(t *testing.T) {
		home := deskHome(t)
		repo := t.TempDir()
		initGitRepo(t, repo) // feat/x checked out nowhere
		// deliberately NO roster dir → beacon signal unreadable
		writeClaim(t, home, "brief-e", "deadsession", "feat/x", aged(200))

		var out string
		rc := 999
		out = captureStdout(t, func() { rc = run([]string{"stale", "--item", "brief-e", "--repo", repo}) })
		if rc != deskkit.ExitRefused {
			t.Fatalf("stale rc = %d, want %d (live — beacon dir unreadable)", rc, deskkit.ExitRefused)
		}
		if !strings.Contains(out, "verdict=live") || !strings.Contains(out, "because=no-repo-cannot-prove") {
			t.Fatalf("stale line = %q, want verdict=live because=no-repo-cannot-prove (cannot prove via beacon)", strings.TrimSpace(out))
		}
	})
}

// ---- Verify row 6 -----------------------------------------------------------

// TestBeaconKeepsClaimLive: with the branch checked out in no worktree, a FRESH roster beacon
// for the owner session is on its own sufficient to keep an aged claim live (because=beacon-live).
func TestBeaconKeepsClaimLive(t *testing.T) {
	home := deskHome(t)
	repo := t.TempDir()
	initGitRepo(t, repo) // feat/x checked out nowhere → worktree signal says "not held"
	writeBeacon(t, home, "livesession", time.Now().Add(-5*time.Minute))
	writeClaim(t, home, "brief-f", "livesession", "feat/x", aged(200))

	var out string
	rc := 999
	out = captureStdout(t, func() { rc = run([]string{"stale", "--item", "brief-f", "--repo", repo}) })
	if rc != deskkit.ExitRefused {
		t.Fatalf("stale rc = %d, want %d (live — fresh beacon)", rc, deskkit.ExitRefused)
	}
	if !strings.Contains(out, "verdict=live") || !strings.Contains(out, "because=beacon-live") {
		t.Fatalf("stale line = %q, want verdict=live because=beacon-live", strings.TrimSpace(out))
	}
	if rc := run([]string{"acquire", "--kind", "dispatch", "--item", "brief-f", "--branch", "feat/x", "--repo", repo}); rc != deskkit.ExitRefused {
		t.Fatalf("acquire rc = %d, want %d — a live beacon must block the reclaim", rc, deskkit.ExitRefused)
	}
}

// ---- Verify row 7 -----------------------------------------------------------

// TestStaleMissingAndUnreadableAreSix: a missing claim and an unreadable (aged) claim both
// exit 6, and neither is reported stale.
func TestStaleMissingAndUnreadableAreSix(t *testing.T) {
	t.Run("missing", func(t *testing.T) {
		home := deskHome(t)
		_ = home
		var out string
		rc := 999
		out = captureStdout(t, func() { rc = run([]string{"stale", "--item", "nope"}) })
		if rc != deskkit.ExitUnverifiable {
			t.Fatalf("stale-missing rc = %d, want %d", rc, deskkit.ExitUnverifiable)
		}
		if strings.Contains(out, "verdict=stale") {
			t.Fatalf("missing claim reported stale: %q", strings.TrimSpace(out))
		}
		if !strings.Contains(out, "because=no-claim") {
			t.Fatalf("missing claim line = %q, want because=no-claim", strings.TrimSpace(out))
		}
	})

	t.Run("unreadable", func(t *testing.T) {
		if os.Geteuid() == 0 {
			t.Skip("running as root: 0000 perms do not block reads")
		}
		home := deskHome(t)
		repo := t.TempDir()
		initGitRepo(t, repo)
		// Aged past TTL so IsStale reaches the file READ (a young claim would short-circuit
		// on the age floor and never try to read), then mode 000 makes that read fail.
		path := writeClaim(t, home, "brief-g", "deadsession", "feat/x", aged(200))
		if err := os.Chmod(path, 0o000); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(path, 0o600) })

		var out string
		rc := 999
		out = captureStdout(t, func() { rc = run([]string{"stale", "--item", "brief-g", "--repo", repo}) })
		if rc != deskkit.ExitUnverifiable {
			t.Fatalf("stale-unreadable rc = %d, want %d", rc, deskkit.ExitUnverifiable)
		}
		if strings.Contains(out, "verdict=stale") {
			t.Fatalf("unreadable claim reported stale: %q", strings.TrimSpace(out))
		}
	})
}
