package gitmaintenance

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/medici-finance/assay/gitcore"
)

func refGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-c", "gc.auto=0", "-c", "maintenance.auto=false", "-c", "core.hooksPath=" + os.DevNull, "-c", "user.name=fixture", "-c", "user.email=fixture@example.invalid"}, args...)...)
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v (%s)", args, err, out)
	} else {
		return strings.TrimSpace(string(out))
	}
	return ""
}

func refWorld(t *testing.T) (root, old string) {
	t.Helper()
	root = t.TempDir()
	refGit(t, root, "init", "-q", "-b", "main")
	refGit(t, root, "commit", "--allow-empty", "-qm", "first")
	old = refGit(t, root, "rev-parse", "HEAD")
	refGit(t, root, "commit", "--allow-empty", "-qm", "second")
	return root, old
}

func seedTemp(t *testing.T, root, oid string) *TempRef {
	t.Helper()
	tmp, err := BeginTempRef(root, "deskmerge")
	if err != nil {
		t.Fatal(err)
	}
	refGit(t, root, "update-ref", tmp.Name, oid)
	if got, err := tmp.Observe(); err != nil || got != oid {
		t.Fatalf("Observe=%s,%v", got, err)
	}
	return tmp
}

func assertRef(t *testing.T, root, name, want string) {
	t.Helper()
	gr, err := gitcore.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	got, exists, err := gr.LocalRefHash(name)
	if err != nil || (want == "" && exists) || (want != "" && (!exists || got != want)) {
		t.Fatalf("ref %s=%s,%v,%v want=%s", name, got, exists, err, want)
	}
}

// regression: #2513
func TestTempRefsIsolateRuns(t *testing.T) {
	root, old := refWorld(t)
	a, b := seedTemp(t, root, old), seedTemp(t, root, old)
	if a.Name == b.Name {
		t.Fatal("same-PR operation destinations alias")
	}
	ctx := context.Background()
	if row, err := a.Complete(ctx); err != nil || row.State != "deleted" {
		t.Fatalf("completion=%+v,%v", row, err)
	}
	assertRef(t, root, a.Name, "")
	rows, err := RecoverTempRefs(ctx, root, 256)
	if err != nil || len(rows) != 1 || rows[0].State != "held" {
		t.Fatalf("active recovery=%+v,%v", rows, err)
	}
	assertRef(t, root, b.Name, old)
	if _, err := b.Complete(ctx); err != nil {
		t.Fatal(err)
	}
	assertRef(t, root, b.Name, "")
}

// regression: #2513
func TestTempRecoveryProofs(t *testing.T) {
	root, old := refWorld(t)
	a := seedTemp(t, root, old)
	a.receipt.Complete = true
	if err := a.save(); err != nil {
		t.Fatal(err)
	}
	refGit(t, root, "update-ref", a.Name, "HEAD")
	head := refGit(t, root, "rev-parse", "HEAD")
	rows, err := RecoverTempRefs(context.Background(), root, 256)
	if err != nil || len(rows) != 1 || rows[0].State != "held" {
		t.Fatalf("moved recovery=%+v,%v", rows, err)
	}
	assertRef(t, root, a.Name, head)
	// Restore the observed hash; a dry run plans but does not touch either artifact.
	refGit(t, root, "update-ref", a.Name, old)
	rows, err = InspectTempRefs(context.Background(), root)
	if err != nil || len(rows) != 1 || rows[0].State != "planned" {
		t.Fatalf("inspect=%+v,%v", rows, err)
	}
	assertRef(t, root, a.Name, old)
	if _, err := os.Stat(a.path); err != nil {
		t.Fatal("inspection removed receipt")
	}
	rows, err = RecoverTempRefs(context.Background(), root, 256)
	if err != nil || len(rows) != 1 || rows[0].State != "deleted" {
		t.Fatalf("recovery=%+v,%v", rows, err)
	}
	assertRef(t, root, a.Name, "")
	if _, err := os.Stat(a.path); !os.IsNotExist(err) {
		t.Fatal("completed receipt was not drained")
	}
}

// regression: #2513
func TestTempRefsPreserveHolders(t *testing.T) {
	root, old := refWorld(t)
	a := seedTemp(t, root, old)
	wt := filepath.Join(t.TempDir(), "holder")
	refGit(t, root, "worktree", "add", "-q", "--detach", wt, old)
	row, err := a.Complete(context.Background())
	if err != nil || row.State != "held" {
		t.Fatalf("held=%+v,%v", row, err)
	}
	assertRef(t, root, a.Name, old)
	common, err := gitcore.CommonDir(root)
	if err != nil {
		t.Fatal(err)
	}
	// A damaged registered HEAD must hold rather than masquerade as no holder.
	entries, err := os.ReadDir(filepath.Join(common, "worktrees"))
	if err != nil || len(entries) != 1 {
		t.Fatal(err)
	}
	headPath := filepath.Join(common, "worktrees", entries[0].Name(), "HEAD")
	if err := os.WriteFile(headPath, []byte("broken\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := RecoverTempRefs(context.Background(), root, 256); err == nil {
		t.Fatal("unreadable holder returned clean")
	}
	assertRef(t, root, a.Name, old)
}

// regression: #2525
func TestTempUnbornHolder(t *testing.T) {
	root, old := refWorld(t)
	a := seedTemp(t, root, old)
	wt := filepath.Join(t.TempDir(), "orphan")
	refGit(t, root, "worktree", "add", "-q", "--orphan", "-b", "unborn", wt)
	// An unborn branch holds no object; it must not turn cleanup into could-not-check.
	row, err := a.Complete(context.Background())
	if err != nil || row.State != "deleted" {
		t.Fatalf("unborn holder=%+v,%v", row, err)
	}
	assertRef(t, root, a.Name, "")
	// Positive control: a branch file that exists but is malformed still holds.
	b := seedTemp(t, root, old)
	common, err := gitcore.CommonDir(root)
	if err != nil {
		t.Fatal(err)
	}
	branch := filepath.Join(common, "refs", "heads", "unborn")
	if err := os.WriteFile(branch, []byte("broken\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if row, err := b.Complete(context.Background()); err == nil || row.State != "held" {
		t.Fatalf("malformed branch holder=%+v,%v", row, err)
	}
	assertRef(t, root, b.Name, old)
}

// regression: #2513
func TestTempRefsKeepUnknown(t *testing.T) {
	root, old := refWorld(t)
	for _, name := range []string{"refs/deskmerge/pr-7", "refs/dispatch/item", "refs/heads/dispatch/item"} {
		refGit(t, root, "update-ref", name, old)
	}
	if _, err := BeginTempRef(root, "unregistered"); err == nil {
		t.Fatal("unregistered owner admitted")
	}
	a := seedTemp(t, root, old)
	a.receipt.Owner = "unregistered"
	a.receipt.Complete = true
	if err := a.save(); err != nil {
		t.Fatal(err)
	}
	rows, err := RecoverTempRefs(context.Background(), root, 256)
	if err != nil || len(rows) != 1 || rows[0].State != "held" {
		t.Fatalf("unknown=%+v,%v", rows, err)
	}
	for _, name := range []string{a.Name, "refs/deskmerge/pr-7", "refs/dispatch/item", "refs/heads/dispatch/item"} {
		assertRef(t, root, name, old)
	}
}

// regression: #2513
func TestTempRefsBoundRecovery(t *testing.T) {
	root, old := refWorld(t)
	a, b := seedTemp(t, root, old), seedTemp(t, root, old)
	for _, tmp := range []*TempRef{a, b} {
		tmp.receipt.Complete = true
		if err := tmp.save(); err != nil {
			t.Fatal(err)
		}
	}
	rows, err := RecoverTempRefs(context.Background(), root, 1)
	if err == nil || len(rows) != 0 {
		t.Fatalf("overflow not refused=%+v,%v", rows, err)
	}
	assertRef(t, root, a.Name, old)
	assertRef(t, root, b.Name, old)
	rows, err = RecoverTempRefs(context.Background(), root, 2)
	if err != nil || len(rows) != 2 || rows[0].State != "deleted" || rows[1].State != "deleted" {
		t.Fatalf("adequate bound=%+v,%v", rows, err)
	}
	remaining, err := InspectTempRefs(context.Background(), root)
	if err != nil || len(remaining) != 0 {
		t.Fatalf("remaining=%+v,%v", remaining, err)
	}
	seedTemp(t, root, old)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := RecoverTempRefs(ctx, root, 256); err == nil {
		t.Fatal("cancel ignored")
	}
}

// regression: #2513
func TestTempRefsDrainAbsent(t *testing.T) {
	root, _ := refWorld(t)
	tmp, err := BeginTempRef(root, "deskmerge")
	if err != nil {
		t.Fatal(err)
	}
	row, err := tmp.Complete(context.Background())
	if err != nil || row.State != "absent" {
		t.Fatalf("absent completion=%+v,%v", row, err)
	}
	if _, err := os.Stat(tmp.path); !os.IsNotExist(err) {
		t.Fatal("absent completed receipt was not drained")
	}
	// A symbolic alias must remain held instead of being treated as absent.
	tmp = seedTemp(t, root, refGit(t, root, "rev-parse", "HEAD^"))
	refGit(t, root, "symbolic-ref", tmp.Name, "refs/heads/main")
	gr, err := gitcore.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	// The reader must distinguish an alias from a malformed direct hash, even
	// when the cleanup layer independently holds both forms of uncertainty.
	if hash, exists, err := gr.LocalRefHash(tmp.Name); hash != "" || !exists || err == nil || !strings.Contains(err.Error(), "symbolic") {
		t.Fatalf("symbolic reader contract: hash=%q exists=%v err=%v", hash, exists, err)
	}
	if row, err := tmp.Complete(context.Background()); err == nil || row.State != "held" {
		t.Fatalf("symbolic completion=%+v,%v", row, err)
	}
	if got := refGit(t, root, "symbolic-ref", tmp.Name); got != "refs/heads/main" {
		t.Fatal("symbolic ref changed")
	}
}

// regression: #2513
func TestTempRefsConcurrent(t *testing.T) {
	root, old := refWorld(t)
	const count = 8
	names := make(chan string, count)
	errors := make(chan error, count)
	var wg sync.WaitGroup
	for i := 0; i < count; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			tmp, err := BeginTempRef(root, "deskmerge")
			if err != nil {
				errors <- err
				return
			}
			cmd := exec.Command("git", "-c", "core.hooksPath="+os.DevNull, "update-ref", tmp.Name, old)
			cmd.Dir = root
			if err = cmd.Run(); err != nil {
				errors <- err
				return
			}
			if _, err = tmp.Observe(); err != nil {
				errors <- err
				return
			}
			if _, err = tmp.Complete(context.Background()); err != nil {
				errors <- err
				return
			}
			names <- tmp.Name
		}()
	}
	wg.Wait()
	close(errors)
	close(names)
	for err := range errors {
		t.Error(err)
	}
	// Busy cleanup retains an already-completed receipt. A later bounded sweep
	// drains it after all producers finish; no active receipt is adopted by age.
	if _, err := RecoverTempRefs(context.Background(), root, tempRefLimit); err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for name := range names {
		if seen[name] {
			t.Error("concurrent fetch destinations alias")
		}
		seen[name] = true
		assertRef(t, root, name, "")
	}
	if len(seen) != count {
		t.Fatalf("completed %d of %d operations", len(seen), count)
	}
}
