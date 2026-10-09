package main

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// recordHome points HOME at a fresh directory for one test, so the released line lands in a
// store the test can read and nothing touches another test's (or the operator's) state.
func recordHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	deskkit.ReloadConfig()
	t.Cleanup(deskkit.ReloadConfig)
	return home
}

func readRecords(t *testing.T, home string) []map[string]any {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(home, ".config", "assay", deskkit.DispatchRecordsFile))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	var out []map[string]any
	for _, l := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		var m map[string]any
		if err := json.Unmarshal([]byte(l), &m); err != nil {
			t.Fatalf("record line does not parse: %v\n%s", err, l)
		}
		out = append(out, m)
	}
	return out
}

// inWorktreeWithRef makes a REAL git checkout whose worktree config carries assay.dispatchRef =
// ref (as deskdispatch writes it) and runs the rest of the test from inside it.
func inWorktreeWithRef(t *testing.T, ref string) {
	t.Helper()
	dir := t.TempDir()
	for _, args := range [][]string{
		{"init", "-q", dir},
		{"-C", dir, "config", "extensions.worktreeConfig", "true"},
		{"-C", dir, "config", "--worktree", "assay.dispatchRef", ref},
	} {
		if out, err := exec.Command("git", args...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	t.Chdir(dir)
}

// TestReleaseWritesReleasedRecord is Verify row 5.
func TestReleaseWritesReleasedRecord(t *testing.T) {
	const id = "at--stream--28"
	const repo = "medici-finance/assay"

	t.Run("matching ref is carried", func(t *testing.T) {
		home := recordHome(t)
		ref := id + "@20261006T141502Z.3fa9c01b7d2e"
		inWorktreeWithRef(t, ref)
		f := newStore()
		f.seedClaim(id, "sess-A", "dispatched", "feat/x", time.Minute)
		run, _, se := harness(t, f)
		if rc := run("release", id, "--repo", repo); rc != exitOK {
			t.Fatalf("release rc = %d; stderr=%s", rc, se.String())
		}
		recs := readRecords(t, home)
		if len(recs) != 1 {
			t.Fatalf("want one released line, got %d: %v", len(recs), recs)
		}
		r := recs[0]
		if r["event"] != "released" || r["claim_key"] != id || r["repo"] != repo || r["dispatch_ref"] != ref {
			t.Errorf("released line = %v, want event released, claim_key %s, repo %s, dispatch_ref %s", r, id, repo, ref)
		}
		for _, k := range []string{"item", "brief", "kit", "branch", "pr", "tier", "brief_exec_tier", "brief_effort", "model_stamp", "attempt_local"} {
			if v, ok := r[k]; !ok || v != nil {
				t.Errorf("released line field %s = %v (present=%v), want null", k, v, ok)
			}
		}
	})

	t.Run("other item's ref is null", func(t *testing.T) {
		home := recordHome(t)
		inWorktreeWithRef(t, "at--stream--99@20261006T141502Z.3fa9c01b7d2e")
		f := newStore()
		f.seedClaim(id, "sess-A", "dispatched", "feat/x", time.Minute)
		run, _, _ := harness(t, f)
		if rc := run("release", id, "--repo", repo); rc != exitOK {
			t.Fatalf("release rc = %d", rc)
		}
		recs := readRecords(t, home)
		if len(recs) != 1 {
			t.Fatalf("want one released line, got %d", len(recs))
		}
		if v, ok := recs[0]["dispatch_ref"]; !ok || v != nil {
			t.Errorf("a non-matching worktree ref was recorded: dispatch_ref = %v", v)
		}
	})

	// A worktree ref minted for THIS key but with a malformed suffix is a ref the record cannot
	// trust: the released line still lands, with a null ref, rather than being refused whole.
	t.Run("malformed matching ref is null", func(t *testing.T) {
		home := recordHome(t)
		inWorktreeWithRef(t, id+"@not-a-timestamp")
		f := newStore()
		f.seedClaim(id, "sess-A", "dispatched", "feat/x", time.Minute)
		run, _, se := harness(t, f)
		if rc := run("release", id, "--repo", repo); rc != exitOK {
			t.Fatalf("release rc = %d", rc)
		}
		recs := readRecords(t, home)
		if len(recs) != 1 {
			t.Fatalf("want one released line, got %d; stderr=%s", len(recs), se.String())
		}
		if v, ok := recs[0]["dispatch_ref"]; !ok || v != nil {
			t.Errorf("a malformed worktree ref was recorded: dispatch_ref = %v", v)
		}
	})

	t.Run("no-op release writes nothing", func(t *testing.T) {
		home := recordHome(t)
		inWorktreeWithRef(t, id+"@20261006T141502Z.3fa9c01b7d2e")
		run, so, _ := harness(t, newStore())
		if rc := run("release", id, "--repo", repo); rc != exitOK {
			t.Fatalf("no-op release rc = %d", rc)
		}
		if !strings.Contains(so.String(), "no-op") {
			t.Fatalf("expected the no-op release path, got %q", so.String())
		}
		if recs := readRecords(t, home); len(recs) != 0 {
			t.Fatalf("a no-op release wrote %d record lines", len(recs))
		}
	})

	t.Run("unwritable store keeps exit 0", func(t *testing.T) {
		home := recordHome(t)
		if err := os.MkdirAll(filepath.Join(home, ".config", "assay", deskkit.DispatchRecordsFile), 0o700); err != nil {
			t.Fatal(err)
		}
		inWorktreeWithRef(t, id+"@20261006T141502Z.3fa9c01b7d2e")
		f := newStore()
		f.seedClaim(id, "sess-A", "dispatched", "feat/x", time.Minute)
		run, _, se := harness(t, f)
		if rc := run("release", id, "--repo", repo); rc != exitOK {
			t.Fatalf("an unwritable record store changed the release exit code to %d", rc)
		}
		if !strings.Contains(se.String(), "WARNING: could not write dispatch record") {
			t.Errorf("no WARNING on stderr: %q", se.String())
		}
		if _, held := f.claims[id]; held {
			t.Error("the claim was not released")
		}
	})
}
