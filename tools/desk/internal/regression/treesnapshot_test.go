package regression

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestTreeSnapshotDetectsEveryChange(t *testing.T) {
	for _, tc := range []struct {
		name, want string
		mutate     func(*testing.T, string)
	}{
		{"body", `changed ".git/config"`, func(t *testing.T, dir string) {
			writeSnapshotFile(t, dir, ".git/config", "different sensitive body")
		}},
		{"added-metadata", `added ".git/gc.log"`, func(t *testing.T, dir string) {
			writeSnapshotFile(t, dir, ".git/gc.log", "maintenance output")
		}},
		{"removed", `removed ".git/config"`, func(t *testing.T, dir string) {
			if err := os.Remove(filepath.Join(dir, ".git/config")); err != nil {
				t.Fatal(err)
			}
		}},
		{"mode", `changed ".git/config"`, func(t *testing.T, dir string) {
			if runtime.GOOS == "windows" {
				t.Skip("POSIX permission bits")
			}
			if err := os.Chmod(filepath.Join(dir, ".git/config"), 0700); err != nil {
				t.Fatal(err)
			}
		}},
		{"symlink-target", `changed "link"`, func(t *testing.T, dir string) {
			if err := os.Remove(filepath.Join(dir, "link")); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink("different-private-target", filepath.Join(dir, "link")); err != nil {
				t.Fatal(err)
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.Mkdir(filepath.Join(dir, ".git"), 0700); err != nil {
				t.Fatal(err)
			}
			writeSnapshotFile(t, dir, ".git/config", "original sensitive body")
			if tc.name == "symlink-target" {
				if runtime.GOOS == "windows" {
					t.Skip("symlink creation may require privileges")
				}
				if err := os.Symlink("original-private-target", filepath.Join(dir, "link")); err != nil {
					t.Fatal(err)
				}
			}
			before := SnapshotTree(t, dir)
			beforeDigest := TreeDigest(t, dir)
			if got := before.Changes(SnapshotTree(t, dir)); got != "" {
				t.Fatalf("unchanged tree: %s", got)
			}
			tc.mutate(t, dir)
			changes := before.Changes(SnapshotTree(t, dir))
			if !strings.Contains(changes, tc.want) {
				t.Fatalf("missed %s: %s", tc.name, changes)
			}
			if TreeDigest(t, dir) == beforeDigest {
				t.Fatalf("digest missed %s", tc.name)
			}
			for _, private := range []string{"sensitive body", "private-target", "maintenance output", dir} {
				if strings.Contains(changes, private) {
					t.Fatalf("diagnostic disclosed content or absolute path: %s", changes)
				}
			}
			if !strings.Contains(changes, "mode=") || !strings.Contains(changes, "sha256=") {
				t.Fatalf("missing metadata: %s", changes)
			}
		})
	}
}

func TestTreeSnapshotChangeDetails(t *testing.T) {
	dir := t.TempDir()
	writeSnapshotFile(t, dir, "body", "before")
	before := SnapshotTree(t, dir)
	writeSnapshotFile(t, dir, "body", "after")
	changes := before.Changes(SnapshotTree(t, dir))
	for _, body := range []string{"before", "after"} {
		if hash := fmt.Sprintf("%x", sha256.Sum256([]byte(body))); !strings.Contains(changes, hash) {
			t.Fatalf("missing %s hash: %s", body, changes)
		}
	}
	// Sorted diagnostics stay reproducible even though snapshots are maps.
	writeSnapshotFile(t, dir, "z", "last")
	writeSnapshotFile(t, dir, "a", "first")
	after := SnapshotTree(t, dir)
	for i := 0; i < 20; i++ {
		if got := before.Changes(after); got != before.Changes(after) {
			t.Fatal("unstable order")
		}
	}
}

func writeSnapshotFile(t *testing.T, dir, path, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, filepath.FromSlash(path)), []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
}

// Query Git itself through the exact setup command constructor. Each key must
// override ambient config, so deleting any setup option makes this test fail.
func TestVictimSetupDisablesAutomaticMaintenance(t *testing.T) {
	home := t.TempDir()
	t.Setenv("GIT_CONFIG_COUNT", "4")
	for i, key := range []string{"maintenance.auto", "gc.auto", "maintenance.autoDetach", "gc.autoDetach"} {
		t.Setenv(fmt.Sprintf("GIT_CONFIG_KEY_%d", i), key)
		t.Setenv(fmt.Sprintf("GIT_CONFIG_VALUE_%d", i), "99")
	}
	for key, want := range map[string]string{
		"maintenance.auto": "false", "gc.auto": "0",
		"maintenance.autoDetach": "false", "gc.autoDetach": "false",
	} {
		cmd := victimSetupGit(home, "config", "--get", key)
		out, err := cmd.CombinedOutput()
		if err != nil || strings.TrimSpace(string(out)) != want {
			t.Errorf("setup %s = %q, error %v; want %s", key, out, err, want)
		}
	}
}
