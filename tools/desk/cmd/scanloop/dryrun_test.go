package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// A --dry-run pass must not advance the poller's per-repo baselines. A poll advances them, so a
// dry-run that polled the REAL state dir consumed the inbound delta it previewed, and the next real
// run found nothing to scan. These tests drive the whole `run` command against a stub poller that
// behaves like the real one in the one way that matters here: every poll rewrites every baseline.

// stubPoller writes a poller that (1) records the state dir it was handed and the baselines it
// found there, then (2) advances every repo's baseline, the way a real poll does. It prints nothing,
// so the pass has no inbound items and opens no network read.
func stubPoller(t *testing.T) (script, sawDir, sawContent string) {
	t.Helper()
	if _, err := os.Stat("/bin/bash"); err != nil {
		t.Skip("/bin/bash absent — execMonitor runs the poller through it")
	}
	dir := t.TempDir()
	sawDir = filepath.Join(dir, "saw-dir")
	sawContent = filepath.Join(dir, "saw-content")
	script = filepath.Join(dir, "inbound-monitor.sh")
	body := `#!/bin/bash
set -eu
d="${INBOUND_MONITOR_STATE_DIR:?}"
printf '%s' "$d" > "$STUB_SAW_DIR"
cat "$d"/*.state > "$STUB_SAW_CONTENT" 2>/dev/null || : > "$STUB_SAW_CONTENT"
mkdir -p "$d"
for slug in "$@"; do
  printf 'advanced\n' >> "$d/$(printf '%s' "$slug" | sed 's#/#__#g').state"
done
`
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("STUB_SAW_DIR", sawDir)
	t.Setenv("STUB_SAW_CONTENT", sawContent)
	// No App role: the poller is handed no token files, so its argv is exactly the scan scope.
	stubIdentity(t,
		func(string) (string, string, error) { return "", "", errors.New("no role in this test") },
		func(string, string) (string, string, error) {
			t.Fatal("a token was minted with no resolvable role")
			return "", "", nil
		})
	return script, sawDir, sawContent
}

// withDryRunTempBase points the throwaway copies at a dir the test owns, so a leftover copy is
// observable.
func withDryRunTempBase(t *testing.T) string {
	t.Helper()
	base := t.TempDir()
	old := dryRunTempBase
	dryRunTempBase = func() string { return base }
	t.Cleanup(func() { dryRunTempBase = old })
	return base
}

func runLivePass(t *testing.T, script, stateDir string, dryRun bool) error {
	t.Helper()
	args := []string{
		"--root", t.TempDir(),
		"--scan-target", "medici-finance/assay",
		"--worktree-base", t.TempDir(),
		"--state-dir", stateDir,
		"--monitor", script,
		"--now", "2026-08-24T12:00:00Z",
	}
	if dryRun {
		args = append(args, "--dry-run")
	}
	return cmdRun(args, &strings.Builder{})
}

// snapshotDir reads every file under dir, keyed by relative path. A missing dir snapshots as nil.
func snapshotDir(t *testing.T, dir string) map[string]string {
	t.Helper()
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil
	}
	out := map[string]string{}
	err := filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(dir, p)
		if d.Type()&os.ModeSymlink != 0 {
			target, lerr := os.Readlink(p)
			out[rel] = "symlink->" + target
			return lerr
		}
		b, rerr := os.ReadFile(p)
		out[rel] = string(b)
		return rerr
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func sameSnapshot(a, b map[string]string) bool {
	if (a == nil) != (b == nil) || len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if w, ok := b[k]; !ok || w != v {
			return false
		}
	}
	return true
}

// TestRunDryRun_LeavesTheRealBaselinesByteIdentical — the acceptance row. The dry-run poll still
// starts from the REAL baselines (the preview is live), but every write lands in a throwaway copy
// that is gone when the pass ends.
func TestRunDryRun_LeavesTheRealBaselinesByteIdentical(t *testing.T) {
	script, sawDir, sawContent := stubPoller(t)
	base := withDryRunTempBase(t)
	stateDir := armedStateDir(t)
	before := snapshotDir(t, stateDir)

	if err := runLivePass(t, script, stateDir, true); err != nil {
		t.Fatalf("dry-run pass: %v", err)
	}

	if after := snapshotDir(t, stateDir); !sameSnapshot(before, after) {
		t.Fatalf("a --dry-run pass changed the real baselines:\nbefore %v\nafter  %v", before, after)
	}
	polled, err := os.ReadFile(sawDir)
	if err != nil {
		t.Fatalf("the stub poller never ran — this test would pass for the wrong reason: %v", err)
	}
	if string(polled) == stateDir {
		t.Fatalf("the dry-run poller was handed the REAL state dir %s", stateDir)
	}
	seen, _ := os.ReadFile(sawContent)
	if want := strings.Repeat("x\n", len(before)); string(seen) != want {
		t.Fatalf("the throwaway copy did not carry the real baselines (the preview would not be live): got %q", seen)
	}
	if _, err := os.Stat(string(polled)); !os.IsNotExist(err) {
		t.Fatalf("the throwaway copy %s outlived the pass (stat err %v)", polled, err)
	}
	if left, _ := os.ReadDir(base); len(left) != 0 {
		t.Fatalf("throwaway copies left behind: %v", left)
	}
}

// TestRunReal_StillAdvancesTheBaselines — the control. Without --dry-run the poller runs against
// the real state dir and advances it, so the test above cannot be passing because the stub never
// wrote anything.
func TestRunReal_StillAdvancesTheBaselines(t *testing.T) {
	script, sawDir, _ := stubPoller(t)
	withDryRunTempBase(t)
	stateDir := armedStateDir(t)
	before := snapshotDir(t, stateDir)

	if err := runLivePass(t, script, stateDir, false); err != nil {
		t.Fatalf("real pass: %v", err)
	}

	if polled, _ := os.ReadFile(sawDir); string(polled) != stateDir {
		t.Fatalf("the real pass polled %q, want the real state dir %s", polled, stateDir)
	}
	after := snapshotDir(t, stateDir)
	if sameSnapshot(before, after) {
		t.Fatal("a real pass did not advance the baselines — the stub is not exercising the write")
	}
	for name, body := range after {
		if body != "x\nadvanced\n" {
			t.Fatalf("baseline %s = %q, want it advanced", name, body)
		}
	}
}

// TestRunDryRun_NeverArmsAnUnarmedStateDir — a dry-run against a state dir that does not exist
// yet seeds a copy, not the real dir. The real dir stays absent, which is still "never armed".
func TestRunDryRun_NeverArmsAnUnarmedStateDir(t *testing.T) {
	script, sawDir, _ := stubPoller(t)
	withDryRunTempBase(t)
	stateDir := filepath.Join(t.TempDir(), "never-armed")

	if err := runLivePass(t, script, stateDir, true); err != nil {
		t.Fatalf("dry-run pass: %v", err)
	}
	if _, err := os.ReadFile(sawDir); err != nil {
		t.Fatalf("the stub poller never ran: %v", err)
	}
	if _, err := os.Stat(stateDir); !os.IsNotExist(err) {
		t.Fatalf("a --dry-run pass created the real state dir %s (stat err %v)", stateDir, err)
	}
}

// TestRunDryRun_UncopyableStateRefusesAndNeverPolls — a copy that cannot be made faithfully is a
// refusal (exit 5), and the poller never runs, so there is no fallback to the real dir. A symlinked
// baseline is the case: copied as a link, the poller would write through it to the real file.
func TestRunDryRun_UncopyableStateRefusesAndNeverPolls(t *testing.T) {
	script, sawDir, _ := stubPoller(t)
	base := withDryRunTempBase(t)
	stateDir := armedStateDir(t)
	elsewhere := filepath.Join(t.TempDir(), "linked.state")
	if err := os.WriteFile(elsewhere, []byte("x\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(elsewhere, filepath.Join(stateDir, "example-org__linked.state")); err != nil {
		t.Skipf("cannot create a symlink here: %v", err)
	}
	before := snapshotDir(t, stateDir)

	err := runLivePass(t, script, stateDir, true)
	if err == nil {
		t.Fatal("a dry-run whose state copy could not be made ran anyway")
	}
	if got := deskkit.ExitCodeOf(err); got != deskkit.ExitRefused {
		t.Fatalf("exit = %d, want %d (refused): %v", got, deskkit.ExitRefused, err)
	}
	if !strings.Contains(err.Error(), "never falls back to the real dir") {
		t.Fatalf("the refusal does not say it will not fall back: %v", err)
	}
	if _, serr := os.Stat(sawDir); !os.IsNotExist(serr) {
		t.Fatal("the poller ran after the copy failed — that is the fallback to the real dir")
	}
	if after := snapshotDir(t, stateDir); !sameSnapshot(before, after) {
		t.Fatalf("the refused pass changed the real baselines:\nbefore %v\nafter  %v", before, after)
	}
	if b, _ := os.ReadFile(elsewhere); string(b) != "x\n" {
		t.Fatalf("the symlink target was written: %q", b)
	}
	if left, _ := os.ReadDir(base); len(left) != 0 {
		t.Fatalf("a partial throwaway copy was left behind: %v", left)
	}
}

// TestDryRunStateCopy_RefusesACopyInsideTheStateDir — a throwaway dir created inside the tree it
// copies would copy itself, and removing it would be a write inside the real state dir.
func TestDryRunStateCopy_RefusesACopyInsideTheStateDir(t *testing.T) {
	stateDir := armedStateDir(t)
	old := dryRunTempBase
	dryRunTempBase = func() string { return stateDir }
	t.Cleanup(func() { dryRunTempBase = old })
	before := snapshotDir(t, stateDir)

	_, _, err := dryRunStateCopy(stateDir)
	if err == nil || deskkit.ExitCodeOf(err) != deskkit.ExitRefused {
		t.Fatalf("a copy inside the state dir was not refused: %v", err)
	}
	// Assert the guard's OWN reason, not just that something refused: without this guard, the
	// walk still refuses (it recurses into the throwaway dir it is filling), for the unrelated
	// reason "an entry could not be copied" — so a looser assertion here would pass even with the
	// guard at dryrun.go deleted, and could never catch its removal.
	if !strings.Contains(err.Error(), "would sit inside the state dir") {
		t.Fatalf("the refusal is not the inside-the-state-dir guard's own reason: %v", err)
	}
	if after := snapshotDir(t, stateDir); !sameSnapshot(before, after) {
		t.Fatalf("the refused copy left something in the state dir:\nbefore %v\nafter  %v", before, after)
	}
}
