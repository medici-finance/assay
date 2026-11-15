package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// leaseFixture points the desk state dir at a temp HOME and plants a HEARTBEAT two hours old.
func leaseFixture(t *testing.T) (string, time.Time) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := filepath.Join(home, ".config", "assay")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "HEARTBEAT")
	if err := os.WriteFile(path, []byte("ok"), 0o600); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-2 * time.Hour).Truncate(time.Second)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatal(err)
	}
	return path, old
}

func mtimeOf(t *testing.T, path string) time.Time {
	t.Helper()
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	return fi.ModTime()
}

// regression: #6510
func TestPushRenewsFetchDoesNot(t *testing.T) {
	for _, tc := range []struct {
		verb  string
		renew bool
	}{{"fetch", false}, {"heads", false}, {"push", true}} {
		t.Run(tc.verb, func(t *testing.T) {
			path, old := leaseFixture(t)
			(&auditCtx{verb: tc.verb, repo: "o/r"}).log("ok", "done")
			renewed := mtimeOf(t, path).After(old)
			if renewed != tc.renew {
				t.Fatalf("deskgit %s: lease renewed=%v, want %v", tc.verb, renewed, tc.renew)
			}
		})
	}
}

// A push the remote already holds moves no ref: git exits 0 ("Everything up-to-date"), but
// the row is noop and the lease is left alone. The first push (which creates the ref) and a
// fast-forward on top are real progress and renew it.
// regression: #6510
func TestUpToDatePushNoRenew(t *testing.T) {
	work := newRepo(t, allowedSlug)
	onBranch(t, work, "feature-1")
	withEnv(t, work)
	asWorker(t)
	lease := func(t *testing.T) (string, time.Time) {
		t.Helper()
		dir := filepath.Join(os.Getenv("HOME"), ".config", "assay")
		path := filepath.Join(dir, "HEARTBEAT")
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("ok"), 0o600); err != nil {
			t.Fatal(err)
		}
		old := time.Now().Add(-2 * time.Hour).Truncate(time.Second)
		if err := os.Chtimes(path, old, old); err != nil {
			t.Fatal(err)
		}
		return path, old
	}
	verb := func(step, v, wantResult string, wantRenew bool, args ...string) {
		t.Helper()
		path, old := lease(t)
		if code := run(append([]string{v}, args...)); code != deskkit.ExitOK {
			t.Fatalf("%s: %s exit = %d, want 0", step, v, code)
		}
		entries, err := deskkit.LoadEntries()
		if err != nil || len(entries) == 0 {
			t.Fatalf("%s: audit rows: %v (n=%d)", step, err, len(entries))
		}
		row := entries[len(entries)-1]
		if row.Tool != "deskgit" || row.Verb != v || row.Result != wantResult {
			t.Fatalf("%s: last row %s %s %q, want deskgit %s %q (detail %q)",
				step, row.Tool, row.Verb, row.Result, v, wantResult, row.Detail)
		}
		if renewed := mtimeOf(t, path).After(old); renewed != wantRenew {
			t.Fatalf("%s: lease renewed=%v, want %v", step, renewed, wantRenew)
		}
	}
	push := func(step, wantResult string, wantRenew bool) {
		t.Helper()
		verb(step, "push", wantResult, wantRenew, "--as", "worker")
	}
	push("first push creates the ref", deskkit.ResultOK, true)
	push("second push, remote already up to date", deskkit.ResultNoop, false)
	// A real fetch is a read: its ok row never renews, run through the verb itself.
	verb("fetch after the push", "fetch", deskkit.ResultOK, false)
	mustGit(t, work, "commit", "--allow-empty", "-m", "next")
	push("fast-forward push", deskkit.ResultOK, true)
}
