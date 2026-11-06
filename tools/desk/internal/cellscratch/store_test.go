package cellscratch

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

func scratchStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "owned"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}
func scratchRun(t *testing.T, s *Store) *Run {
	t.Helper()
	r, err := s.Begin("session", "worker", "revision")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { r.Close() })
	if err = os.WriteFile(filepath.Join(r.Work(), "build"), []byte(strings.Repeat("x", 1000)), 0600); err != nil {
		t.Fatal(err)
	}
	return r
}
func acknowledge(t *testing.T, r *Run) {
	t.Helper()
	if err := r.Store.Acknowledge(r.Record.ID, "forge.example/pr/1#evidence", false); err != nil {
		t.Fatal(err)
	}
}
func finish(t *testing.T, r *Run, code int) {
	t.Helper()
	if err := r.Finish(code, true); err != nil {
		t.Fatal(err)
	}
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
}
func exists(t *testing.T, path string) bool {
	t.Helper()
	_, err := os.Lstat(path)
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	return err == nil
}
func sweep(t *testing.T, s *Store, p Policy, apply bool) Report {
	t.Helper()
	r, err := s.Sweep(p, apply)
	if err != nil {
		t.Fatal(err)
	}
	return r
}
func TestScratchHandoff(t *testing.T) {
	s := scratchStore(t)
	r := scratchRun(t, s)
	acknowledge(t, r)
	if got := sweep(t, s, DefaultPolicy(), true); got.Entries[0].Action != "keep" {
		t.Fatal("live task removed", got)
	}
	finish(t, r, 0)
	dry := sweep(t, s, DefaultPolicy(), false)
	if !exists(t, r.Work()) || dry.Entries[0].Action != "remove" {
		t.Fatal(dry)
	}
	applied := sweep(t, s, DefaultPolicy(), true)
	if dry.Entries[0].Bytes != applied.Reclaimed || exists(t, r.Work()) {
		t.Fatal("dry-run/apply disagree", dry, applied)
	}
	if got := sweep(t, s, DefaultPolicy(), true); len(got.Entries) != 0 || got.Reclaimed != 0 {
		t.Fatal("not idempotent", got)
	}
}
func TestScratchRetention(t *testing.T) {
	s := scratchStore(t)
	p := Policy{MaxAge: time.Hour, MaxBytes: 2 * TailLimit}
	for n := 0; n < 15; n++ {
		r := scratchRun(t, s)
		r.Record.Task = []string{"worker", "reviewer", "verifier"}[n%3]
		tail := r.Tail()
		if _, err := tail.Write([]byte(strings.Repeat("q", 4*TailLimit) + "END")); err != nil {
			t.Fatal(err)
		}
		acknowledge(t, r)
		finish(t, r, 23)
		got := sweep(t, s, p, true)
		if got.OverBudget || exists(t, r.Work()) {
			t.Fatal("retention grows or disposable tree retained", got)
		}
		record, err := s.record(r.Record.ID)
		if err != nil || record.ExitCode != 23 || record.State != "failed" {
			t.Fatalf("failure result lost: %+v %v", record, err)
		}
		b, err := s.read(r.Record.ID+"/diagnostic.txt", TailLimit)
		if err != nil || len(b) != TailLimit || !strings.HasSuffix(string(b), "END") {
			t.Fatal("diagnostic bound/selection", len(b), err)
		}
	}
	if got := sweep(t, s, Policy{MaxAge: 0, MaxBytes: 0}, true); got.OverBudget {
		t.Fatal(got)
	}
}
func TestScratchProtection(t *testing.T) {
	for _, kind := range []string{"pending", "resumable", "git", "foreign", "symlink", "unproved", "uncertain"} {
		t.Run(kind, func(t *testing.T) {
			s := scratchStore(t)
			r := scratchRun(t, s)
			finish(t, r, 0)
			if kind != "pending" {
				acknowledge(t, r)
			}
			outside := t.TempDir()
			sentinel := filepath.Join(outside, "sentinel")
			os.WriteFile(sentinel, []byte("keep"), 0600)
			switch kind {
			case "resumable":
				if err := s.Acknowledge(r.Record.ID, "evidence", true); err != nil {
					t.Fatal(err)
				}
			case "git":
				os.WriteFile(filepath.Join(r.Work(), ".git"), []byte("gitdir: somewhere"), 0600)
			case "foreign":
				r.Record.Owner = "another"
				s.save(r.Record)
			case "symlink":
				os.RemoveAll(r.Work())
				os.Symlink(outside, r.Work())
				r.Record.Reaped = false
				s.save(r.Record)
			case "unproved", "uncertain":
				r.Record.Reaped = false
				r.Record.ChildGroup = 0
				s.save(r.Record)
			}
			// The plan must keep it too: a later deletion recheck is not the only guard.
			if dry := sweep(t, s, Policy{}, false); dry.Entries[0].Action != "keep" {
				t.Fatal(kind, "planned for reclaim", dry)
			}
			got := sweep(t, s, Policy{}, true)
			if got.Entries[0].Action != "keep" || !exists(t, sentinel) || !exists(t, filepath.Join(s.Path, r.Record.ID)) {
				t.Fatal(kind, got)
			}
		})
	}
}
func TestScratchEscape(t *testing.T) {
	s := scratchStore(t)
	outside := t.TempDir()
	sentinel := filepath.Join(outside, "keep")
	os.WriteFile(sentinel, []byte("keep"), 0600)
	r := scratchRun(t, s)
	os.Symlink(outside, filepath.Join(r.Work(), "escape"))
	acknowledge(t, r)
	finish(t, r, 0)
	sweep(t, s, Policy{}, true)
	if !exists(t, sentinel) {
		t.Fatal("followed symlink out of scope")
	}
	os.Symlink(outside, filepath.Join(s.Path, "task-1234567890abcdef"))
	if got := sweep(t, s, Policy{}, true); got.Entries[0].Action != "keep" {
		t.Fatal(got)
	}
	if _, err := Open(filepath.Join(s.Path, "task-1234567890abcdef")); err == nil {
		t.Fatal("accepted symlink root")
	}
	if err := s.Acknowledge("../outside", "evidence", false); err == nil {
		t.Fatal("accepted escaping id")
	}
	legacy, err := Inventory(outside)
	if err != nil || len(legacy) != 1 || legacy[0].Action != "keep" {
		t.Fatal(legacy, err)
	}
}
func TestScratchPartial(t *testing.T) {
	s := scratchStore(t)
	r := scratchRun(t, s)
	acknowledge(t, r)
	finish(t, r, 0)
	original := s.remove
	s.remove = func(path string, seen pins) error {
		if err := s.root.Remove(path + "/build"); err != nil {
			return err
		}
		return errors.New("fixture deletion failed")
	}
	got, err := s.Sweep(Policy{}, true)
	if err == nil || !got.Partial || got.Reclaimed != 1000 || got.Entries[0].Action != "error" {
		t.Fatal("partial deletion not reported", got, err)
	}
	s.remove = original
	if got := sweep(t, s, Policy{}, true); got.Partial || exists(t, r.Work()) {
		t.Fatal(got)
	}
}
func TestScratchSweepLease(t *testing.T) {
	s := scratchStore(t)
	r := scratchRun(t, s)
	acknowledge(t, r)
	finish(t, r, 0)
	lock, err := s.lock("sweep.lock")
	if err != nil {
		t.Fatal(err)
	}
	s.lockWait = 100 * time.Millisecond
	if _, err = s.Sweep(Policy{}, true); err == nil {
		t.Fatal("overlapping sweep admitted")
	}
	if !exists(t, r.Work()) {
		t.Fatal("overlapping sweep deleted task")
	}
	lock.Close()
	sweep(t, s, Policy{}, true)
}
func deleteSites(src string) []string {
	f, err := parser.ParseFile(token.NewFileSet(), "fixture.go", src, 0)
	if err != nil {
		return []string{"parse-error"}
	}
	var sites []string
	ast.Inspect(f, func(n ast.Node) bool {
		sel, ok := n.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "RemoveAll" {
			return true
		}
		if id, ok := sel.X.(*ast.Ident); ok {
			sites = append(sites, id.Name+".RemoveAll")
		} else {
			sites = append(sites, "unknown.RemoveAll")
		}
		return true
	})
	return sites
}
func TestScratchDeleteGuard(t *testing.T) {
	// Positive control is the required planted SECOND raw deletion site.
	if got := deleteSites("package fixture; func planted(){ os.RemoveAll(outside) }"); !reflect.DeepEqual(got, []string{"os.RemoveAll"}) {
		t.Fatal("guard failed to see planted deletion", got)
	}
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	var sites []string
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		b, err := os.ReadFile(e.Name())
		if err != nil {
			t.Fatal(err)
		}
		for _, site := range deleteSites(string(b)) {
			sites = append(sites, e.Name()+":"+site)
		}
	}
	if !reflect.DeepEqual(sites, []string{"store.go:r.RemoveAll"}) {
		t.Fatal("unowned deletion site:", sites)
	}
}

// A task's ownership and evidence must survive a partial workspace deletion and
// a store restart. Otherwise the next sweep can only inventory the residue.
func TestScratchPartialWorkspaceRestart(t *testing.T) {
	for _, code := range []int{0, 17} {
		t.Run(fmt.Sprint(code), func(t *testing.T) {
			s := scratchStore(t)
			r := scratchRun(t, s)
			acknowledge(t, r)
			finish(t, r, code)
			original := s.remove
			var calls []string
			s.remove = func(path string, seen pins) error {
				calls = append(calls, path)
				if path != r.Record.ID+"/work" {
					t.Fatalf("metadata retired before workspace: %s", path)
				}
				if err := s.root.Remove(path + "/build"); err != nil {
					return err
				}
				return errors.New("workspace removal interrupted")
			}
			got, err := s.Sweep(Policy{}, true)
			if err == nil || !got.Partial || got.Reclaimed != 1000 || len(calls) != 1 {
				t.Fatal(got, err, calls)
			}
			if _, err := s.record(r.Record.ID); err != nil {
				t.Fatal("ownership lost", err)
			}
			if _, err := s.receipt(r.Record.ID); err != nil {
				t.Fatal("evidence receipt lost", err)
			}
			s.remove = original
			root := s.Path
			s.Close()
			recovered, err := Open(root)
			if err != nil {
				t.Fatal(err)
			}
			defer recovered.Close()
			got = sweep(t, recovered, Policy{}, true)
			if got.Partial || exists(t, filepath.Join(root, r.Record.ID)) {
				t.Fatal("retry stranded task", got)
			}
			if got = sweep(t, recovered, Policy{}, true); len(got.Entries) != 0 {
				t.Fatal("not idempotent", got)
			}
		})
	}
}

// regression: #3353
func TestMaintenanceScanBound(t *testing.T) {
	s := scratchStore(t)
	small := scratchRun(t, s)
	finish(t, small, 0)
	acknowledge(t, small)
	r := scratchRun(t, s)
	finish(t, r, 0)
	acknowledge(t, r)
	for i := 0; i < 65; i++ {
		if err := os.WriteFile(filepath.Join(r.Work(), fmt.Sprintf("wide-%03d", i)), []byte("retain"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	// Any failed inspection other than the per-entry bound still withdraws
	// authority from the whole plan: the measured task beside it is not reclaimed.
	locked := filepath.Join(s.Path, "legacy-locked", "locked")
	must(t, os.MkdirAll(locked, 0700))
	must(t, os.Chmod(locked, 0))
	t.Cleanup(func() { _ = os.Chmod(locked, 0700) })
	rep, err := s.Sweep(DefaultPolicy(), true)
	if err == nil || !rep.Partial || rep.Reclaimed != 0 || !exists(t, small.Work()) {
		t.Fatalf("failed inspection did not fail the plan closed: %+v %v", rep, err)
	}
	must(t, os.Chmod(locked, 0700))
	must(t, os.RemoveAll(filepath.Dir(locked)))
	t.Setenv("ASSAY_MAINTENANCE_ENTRY_LIMIT", "32")
	// The over-limit task gains no authority; since #5560 it no longer withholds
	// authority from the fully measured task beside it (TestScanBoundPerEntry).
	rep, err = s.Sweep(DefaultPolicy(), true)
	if err == nil || !rep.Partial {
		t.Fatalf("incomplete scan reported complete: %+v %v", rep, err)
	}
	if _, err := os.Stat(filepath.Join(r.Work(), "build")); err != nil {
		t.Fatalf("partial inventory deleted acknowledged work: %v", err)
	}
	if exists(t, small.Work()) {
		t.Fatal("over-limit task withheld authority from a measured one")
	}
	for i := 0; i < 40; i++ {
		if err := os.Mkdir(filepath.Join(s.Path, fmt.Sprintf("wide-%03d", i)), 0700); err != nil {
			t.Fatal(err)
		}
	}
	rep, err = s.Sweep(DefaultPolicy(), true)
	if err == nil || !rep.Partial || len(rep.Entries) != 0 || rep.Reclaimed != 0 {
		t.Fatalf("partial root listing gained authority: %+v %v", rep, err)
	}
	t.Setenv("ASSAY_MAINTENANCE_ENTRY_LIMIT", "256")
	rep, err = s.Sweep(DefaultPolicy(), true)
	if err != nil || rep.Partial || rep.Reclaimed == 0 {
		t.Fatalf("finite raised-budget recovery failed: %+v %v", rep, err)
	}

}

// One entry too large to walk inside the inventory bound is kept and named, and the
// rest of the plan still runs: an ordinary reclaimable task beside it is reclaimed.
// The sweep stays partial, so the periodic stamp is not written.
//
// regression: #5560
func TestScanBoundPerEntry(t *testing.T) {
	for _, kind := range []string{"task", "unverified"} {
		t.Run(kind, func(t *testing.T) {
			s := scratchStore(t)
			ordinary := scratchRun(t, s)
			acknowledge(t, ordinary)
			finish(t, ordinary, 0)
			id, wide := "legacy-wide", filepath.Join(s.Path, "legacy-wide")
			if kind == "task" {
				r := scratchRun(t, s)
				acknowledge(t, r)
				finish(t, r, 0)
				id, wide = r.Record.ID, r.Work()
			} else if err := os.Mkdir(wide, 0700); err != nil {
				t.Fatal(err)
			}
			for i := 0; i < 65; i++ {
				if err := os.WriteFile(filepath.Join(wide, fmt.Sprintf("wide-%03d", i)), []byte("retain"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			t.Setenv("ASSAY_MAINTENANCE_ENTRY_LIMIT", "32")
			rep, ran, err := s.SweepIfDue(DefaultPolicy(), time.Hour, true)
			rows := map[string]Entry{}
			for _, e := range rep.Entries {
				rows[e.ID] = e
			}
			done := rows[ordinary.Record.ID]
			if !ran || exists(t, filepath.Join(s.Path, ordinary.Record.ID)) || done.Action != "remove" || rep.Reclaimed == 0 || rep.Reclaimed != done.Reclaimed {
				t.Fatalf("one over-limit entry blocked the rest: ran=%v err=%v %+v", ran, err, rep)
			}
			kept := rows[id]
			if !exists(t, filepath.Join(wide, "wide-000")) || kept.Action != "keep" || !strings.Contains(kept.Reason, "inventory limit") {
				t.Fatalf("over-limit entry not kept with its reason: %+v", kept)
			}
			if err == nil || !rep.Partial || !errors.Is(err, deskkit.ErrScanLimit) || !strings.Contains(err.Error(), id) {
				t.Fatalf("unmeasured entry not reported by name: err=%v %+v", err, rep)
			}
			if exists(t, filepath.Join(s.Path, maintenanceStampName)) || !exists(t, filepath.Join(s.Path, maintenanceAttemptName)) {
				t.Fatal("incomplete accounting stamped the interval as done")
			}
		})
	}
}
