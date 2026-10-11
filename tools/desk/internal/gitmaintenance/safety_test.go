package gitmaintenance

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// regression: #2513
func TestDryRunLeavesNoWrites(t *testing.T) {
	repo := fixtureRepo(t)
	requireNativeHooks(t, repo)
	linked := filepath.Join(t.TempDir(), "linked")
	fixtureGit(t, repo, "worktree", "add", "-b", "active", linked)
	before := diskImage(t, repo)
	linkedBefore := diskImage(t, linked)
	if _, err := Inspect(context.Background(), linked); err != nil {
		t.Fatal(err)
	}
	report, err := Run(context.Background(), linked, Options{DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if !report.DryRun {
		t.Fatal("dry-run report did not identify its mode")
	}
	if got := diskImage(t, repo); !reflect.DeepEqual(before, got) {
		t.Fatal("dry-run changed repository files or bookkeeping")
	}
	if got := diskImage(t, linked); !reflect.DeepEqual(linkedBefore, got) {
		t.Fatal("dry-run changed linked worktree")
	}
}

// regression: #2513
func TestCommonDirCadence(t *testing.T) {
	repo := fixtureRepo(t)
	requireNativeHooks(t, repo)
	linked := filepath.Join(t.TempDir(), "linked")
	fixtureGit(t, repo, "worktree", "add", "-b", "active", linked)
	first, err := Run(context.Background(), repo, Options{MinInterval: time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	if first.Skipped != "" || len(first.Tasks) == 0 {
		t.Fatalf("first run did no work: %+v", first)
	}
	if first.Before.LastSuccess != nil || first.After == nil || first.After.LastSuccess == nil || first.After.LastAttempt == nil || first.After.LastSuccess.Before(*first.After.LastAttempt) {
		t.Fatal("completed report omitted success or confused attempted work with completed work")
	}
	stamp := diskImage(t, repo)
	next, err := Run(context.Background(), linked, Options{MinInterval: time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	if next.Skipped == "" || len(next.Tasks) != 0 {
		t.Fatalf("linked worktree repeated shared maintenance: %+v", next)
	}
	if first.Before.CommonDir != next.Before.CommonDir {
		t.Fatal("linked worktree did not resolve the same coordinator")
	}
	if next.Before.LastSuccess == nil || !next.Before.LastSuccess.Equal(*first.After.LastSuccess) {
		t.Fatal("completed report does not match the committed success timestamp")
	}
	if !reflect.DeepEqual(stamp, diskImage(t, repo)) {
		t.Fatal("cadence skip rewrote repository state")
	}
}

// regression: #2513
func TestPrimaryLockHeld(t *testing.T) {
	repo := fixtureRepo(t)
	requireNativeHooks(t, repo)
	// The shipped index coordinator holds only this primary lock. A separate
	// temp-ref coordinator cannot stand in for honoring that owner's custody.
	primary, err := OpenLock(filepath.Join(repo, ".git", "assay-maintenance.lock"))
	if err != nil {
		t.Fatal(err)
	}
	defer primary.Close()
	if err := deskkit.TryLockExclusive(primary); err != nil {
		t.Fatal(err)
	}
	defer deskkit.UnlockFile(primary)
	if _, err := os.Stat(filepath.Join(repo, ".git", "assay-maintenance-temprefs.lock")); !os.IsNotExist(err) {
		t.Fatal("positive control: temp-ref lock must be absent and free")
	}
	before := diskImage(t, repo)
	report, err := Run(context.Background(), repo, Options{})
	if err != nil || report.Skipped != "another coordinator is running" || len(report.Tasks) != 0 || report.After != nil || report.Before.LastAttempt != nil {
		t.Fatalf("primary-only coordinator custody bypassed: %#v %v", report, err)
	}
	if !reflect.DeepEqual(before, diskImage(t, repo)) {
		t.Fatalf("busy primary coordinator caused writes: %v", snapshotChanges(before, diskImage(t, repo)))
	}
}

// regression: #2513
func TestConcurrentSharedRun(t *testing.T) {
	repo := fixtureRepo(t)
	requireNativeHooks(t, repo)
	linked := filepath.Join(t.TempDir(), "linked")
	fixtureGit(t, repo, "worktree", "add", "-b", "active", linked)
	var wg sync.WaitGroup
	start := make(chan struct{})
	results := make(chan Report, 4)
	errs := make(chan error, 4)
	for _, root := range []string{repo, linked, repo, linked} {
		wg.Add(1)
		go func(root string) {
			defer wg.Done()
			<-start
			report, err := Run(context.Background(), root, Options{MinInterval: time.Hour})
			results <- report
			errs <- err
		}(root)
	}
	close(start)
	wg.Wait()
	close(results)
	close(errs)
	runs := 0
	for report := range results {
		if len(report.Tasks) != 0 {
			runs++
		}
	}
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if runs != 1 {
		t.Fatalf("%d concurrent callers launched work, want one", runs)
	}
}

// regression: #2513
func TestActiveWorkRetained(t *testing.T) {
	repo := fixtureRepo(t)
	requireNativeHooks(t, repo)
	linked := filepath.Join(t.TempDir(), "linked")
	fixtureGit(t, repo, "worktree", "add", "-b", "unpublished", linked)
	head := fixtureCommit(t, linked, "unpublished changes")
	if err := os.WriteFile(filepath.Join(linked, "tracked"), []byte("dirty changes\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(linked, "untracked"), []byte("untracked content\n"), 0644); err != nil {
		t.Fatal(err)
	}
	fixtureGit(t, repo, "update-ref", "refs/dispatch/live", head)
	fixtureGit(t, repo, "update-ref", "refs/heads/dispatch/live", head)
	refs := fixtureGit(t, repo, "for-each-ref", "--format=%(refname) %(objectname)")
	work := diskImage(t, linked)
	if _, err := Run(context.Background(), repo, Options{}); err != nil {
		t.Fatal(err)
	}
	if got := fixtureGit(t, repo, "for-each-ref", "--format=%(refname) %(objectname)"); got != refs {
		t.Fatalf("maintenance changed live refs\n%s\nwant\n%s", got, refs)
	}
	if got := fixtureGit(t, linked, "rev-parse", "HEAD"); got != head {
		t.Fatal("unpublished head changed")
	}
	if !reflect.DeepEqual(work, diskImage(t, linked)) {
		t.Fatal("dirty or untracked work changed")
	}
	fixtureGit(t, linked, "cat-file", "-e", head+"^{commit}")
}

// regression: #2513
func TestBorrowerObjectsRetained(t *testing.T) {
	repo := fixtureRepo(t)
	requireNativeHooks(t, repo)
	initial := fixtureGit(t, repo, "rev-parse", "HEAD")
	fixtureGit(t, repo, "checkout", "-b", "borrowed")
	borrowed := fixtureCommit(t, repo, "borrowed only")
	fixtureGit(t, repo, "repack", "-a")
	borrower := filepath.Join(t.TempDir(), "borrower")
	fixtureGit(t, repo, "clone", "--shared", "--branch", "borrowed", repo, borrower)
	// Borrower history is not bounded by its local counts. Both modes must hold
	// without bookkeeping writes and expose the independently owned object store.
	fixtureCommit(t, borrower, "borrower local work")
	sourceBefore := diskImage(t, repo)
	borrowerBefore := diskImage(t, borrower)
	borrowerInv, err := Inspect(context.Background(), borrower)
	if err != nil {
		t.Fatal(err)
	}
	sourceObjects, err := filepath.EvalSymlinks(filepath.Join(repo, ".git", "objects"))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(borrowerInv.Alternates, []string{sourceObjects}) {
		t.Fatalf("declared object store omitted from inventory: %v", borrowerInv.Alternates)
	}
	for _, dry := range []bool{true, false} {
		report, err := Run(context.Background(), borrower, Options{DryRun: dry})
		if err != nil || report.Skipped != "borrowed object stores require a separate maintenance window" || len(report.Tasks) != 0 || report.After != nil || report.Before.LastAttempt != nil || report.Before.LastSuccess != nil {
			t.Fatalf("unbounded borrower launched work or bookkeeping: %#v %v", report, err)
		}
		if !reflect.DeepEqual(report.Before.Alternates, []string{sourceObjects}) {
			t.Fatal("held borrower omitted declared store from inventory")
		}
		if !reflect.DeepEqual(sourceBefore, diskImage(t, repo)) {
			t.Fatalf("held borrower changed provider: %v", snapshotChanges(sourceBefore, diskImage(t, repo)))
		}
		if !reflect.DeepEqual(borrowerBefore, diskImage(t, borrower)) {
			t.Fatalf("held borrower changed own repository: %v", snapshotChanges(borrowerBefore, diskImage(t, borrower)))
		}
	}
	fixtureGit(t, borrower, "cat-file", "-e", borrowed+"^{commit}")

	fixtureGit(t, repo, "checkout", "main")
	for i := 0; i < 3; i++ {
		fixtureCommit(t, repo, fmt.Sprintf("provider %d", i))
		fixtureGit(t, repo, "repack", "-a")
	}
	fixtureGit(t, repo, "branch", "-D", "borrowed")
	// Fixture-only expiry deliberately leaves the provider unaware of the borrower.
	// Maintenance must preserve the object even without a source ref or reflog root.
	fixtureGit(t, repo, "reflog", "expire", "--expire=now", "--all")
	all := fixtureGit(t, repo, "cat-file", "--batch-all-objects", "--batch-check=%(objectname)", "--unordered")
	before, err := Inspect(context.Background(), repo)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		stamp := filepath.Join(before.CommonDir, "assay-maintenance.json")
		if i != 0 {
			if err := os.Remove(stamp); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := Run(context.Background(), repo, Options{MinInterval: time.Nanosecond}); err != nil {
			t.Fatal(err)
		}
		got := fixtureGit(t, repo, "cat-file", "--batch-all-objects", "--batch-check=%(objectname)", "--unordered")
		if !sameObjectSet(all, got) {
			t.Fatal("maintenance discarded an existing object ID")
		}
		fixtureGit(t, borrower, "cat-file", "-e", borrowed+"^{commit}")
		fixtureGit(t, borrower, "cat-file", "-e", initial+"^{commit}")
		fixtureGit(t, borrower, "fsck", "--connectivity-only")
	}
	after, err := Inspect(context.Background(), repo)
	if err != nil {
		t.Fatal(err)
	}
	if after.Objects.Packs >= before.Objects.Packs {
		t.Fatalf("redundant packs did not decrease: %d -> %d", before.Objects.Packs, after.Objects.Packs)
	}
}

func sameObjectSet(a, b string) bool {
	left, right := map[string]bool{}, map[string]bool{}
	for _, line := range strings.Fields(a) {
		left[line] = true
	}
	for _, line := range strings.Fields(b) {
		right[line] = true
	}
	return reflect.DeepEqual(left, right)
}
