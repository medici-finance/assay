package gitcore

import (
	"fmt"
	"github.com/medici-finance/assay/gitcore/gittest"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestMetadataReuseAllocs(t *testing.T) {
	f := gittest.NewFixture(t)
	for i := 0; i < 41; i++ {
		f.CommitFile(t, "other", fmt.Sprint(i), "unrelated change")
	}
	r, err := Open(f.Dir)
	if err != nil {
		t.Fatal(err)
	}
	var queryErr error
	allocations := testing.AllocsPerRun(3, func() {
		row, err := r.LastPathCommit("HEAD", "seed.txt")
		if err != nil {
			queryErr = err
		}
		if row == nil || row.NativeReason != "" {
			queryErr = fmt.Errorf("metadata unavailable: %+v", row)
		}
	})
	if queryErr != nil {
		t.Fatal(queryErr)
	}
	t.Logf("repeated last-path query over42commits: %.0f allocations", allocations)
	if allocations > 1000 {
		t.Fatalf("repeated immutable history is decoded again: %.0f allocations exceeds broad1000 budget", allocations)
	}
}

func TestMetadataConcurrentQueriesAndFreshHead(t *testing.T) {
	f := gittest.NewFixture(t)
	f.CommitFile(t, "other", "one", "other")
	r, err := Open(f.Dir)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for n := 0; n < 8; n++ {
				for _, path := range []string{"seed.txt", "other", "absent"} {
					if _, err := r.LastPathCommit("HEAD", path); err != nil {
						t.Error(err)
					}
				}
			}
		}()
	}
	wg.Wait()
	f.CommitFile(t, "seed.txt", "changed", "new head")
	want := mustGitOutput(t, f, "rev-parse", "HEAD")
	got, err := r.LastPathCommit("HEAD", "seed.txt")
	if err != nil || got == nil || got.ID != want {
		t.Fatalf("immutable facts cached mutable HEAD: %+v %v want %s", got, err, want)
	}
}

func TestMetadataFailureIsNotCached(t *testing.T) {
	f := gittest.NewFixture(t)
	f.CommitFile(t, "other", "one", "other")
	parent := mustGitOutput(t, f, "rev-parse", "HEAD^")
	file := filepath.Join(f.Dir, ".git", "objects", parent[:2], parent[2:])
	raw, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(file); err != nil {
		t.Fatal(err)
	}
	r, err := Open(f.Dir)
	if err != nil {
		t.Fatal(err)
	}
	if row, err := r.LastPathCommit("HEAD", "seed.txt"); err == nil {
		t.Fatalf("gap became answer %+v", row)
	}
	if err := os.WriteFile(file, raw, 0444); err != nil {
		t.Fatal(err)
	}
	row, err := r.LastPathCommit("HEAD", "seed.txt")
	if err != nil || row == nil || row.ID != parent {
		t.Fatalf("failed read poisoned later query: %+v %v", row, err)
	}
}

// regression: #4846 A1, decoded tree retention unbounded
func TestMetadataTreeCacheBounded(t *testing.T) {
	f := gittest.NewFixture(t)
	for i := 0; i < 10; i++ {
		f.CommitFile(t, "other", fmt.Sprint(i), "unrelated change")
	}
	r, err := Open(f.Dir)
	if err != nil {
		t.Fatal(err)
	}
	r.treeLimit = 3
	want := mustGitOutput(t, f, "log", "-1", "--format=%H", "--", "seed.txt")
	for n := 0; n < 2; n++ {
		if row, err := r.LastPathCommit("HEAD", "absent"); err != nil || row != nil {
			t.Fatalf("absent: %+v %v", row, err)
		}
		row, err := r.LastPathCommit("HEAD", "seed.txt")
		if err != nil || row == nil || row.ID != want {
			t.Fatalf("seed.txt: %+v %v want %s", row, err, want)
		}
	}
	r.metadataMu.Lock()
	held := len(r.metadataTrees)
	r.metadataMu.Unlock()
	if held > 3 {
		t.Fatalf("retained %d decoded trees, limit 3", held)
	}
}
