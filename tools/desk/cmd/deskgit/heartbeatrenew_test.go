package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
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
