package main

import (
	"os"
	"path/filepath"
	"testing"
)

// regression: #5684 — the coverage entry points bypass run(), which normally
// owns the Git reader. Reopening the same repository for each evaluator question
// loses the existing invocation-scoped object cache.
func TestCoverageGitReadLifetime(t *testing.T) {
	installStatusgenFixture(t)
	root := bulkV2Fixture(t, 1, 1, 1)
	stream := &Stream{Name: gv2Stream, Dir: filepath.Join(root, "docs", "streams", gv2Stream), Root: root}
	if activeGitReads != nil {
		t.Fatal("fixture leaked a Git reader")
	}
	var previous *gitReadSession
	for _, nested := range []bool{false, false, true} {
		var parent *gitReadSession
		var closeParent func()
		if nested {
			closeParent = beginGitReadSession()
			parent = activeGitReads
		}
		var observed *gitReadSession
		calls := 0
		coverageGitHook = func(string, []string) {
			calls++
			session := activeGitReads
			if session == nil {
				t.Error("coverage read has no invocation owner")
				return
			}
			if observed != nil && session != observed {
				t.Error("reader changed during one evaluation")
			}
			observed = session
			if nested && session != parent {
				t.Error("coverage replaced its caller's reader")
			}
		}
		coverage := evaluateCoverage(root, []*Stream{stream}, coverageOptions{})
		coverageGitHook = nil
		if calls == 0 {
			t.Fatal("fixture made no coverage Git calls")
		}
		c, ok := coverage[gv2Stream+"/01"]
		if !ok || !c.Released || len(c.Claims) != 1 {
			t.Fatalf("fixture coverage: %+v", coverage)
		}
		if observed == nil {
			t.Fatal("no owned reader observed")
		}
		if previous == observed {
			t.Error("reader escaped its evaluation boundary")
		}
		previous = observed
		if read := observed.core[root]; read.err != nil || read.repo == nil {
			t.Errorf("evaluation did not retain its canonical repository: %v", read.err)
		}
		if nested {
			if activeGitReads != parent {
				t.Error("coverage closed its caller's reader")
			}
			// The caller can still reuse its canonical reader after evaluation returns.
			if read := gitCoreFor(root); read.repo != observed.core[root].repo {
				t.Error("caller reader lost its repository")
			}
			closeParent()
		}
		if activeGitReads != nil {
			t.Fatal("coverage reader outlived its owner")
		}
	}
}

// A reused reader must still observe a newly corrupt history profile; restoring
// that metadata must permit the original valid evidence again.
func TestCoverageGitReadMetadataChanges(t *testing.T) {
	installStatusgenFixture(t)
	root := bulkV2Fixture(t, 1, 1, 1)
	stream := &Stream{Name: gv2Stream, Dir: filepath.Join(root, "docs", "streams", gv2Stream), Root: root}
	defer beginGitReadSession()()
	check := func(wantReleased bool) {
		t.Helper()
		c, ok := evaluateCoverage(root, []*Stream{stream}, coverageOptions{})[gv2Stream+"/01"]
		if !ok || len(c.Claims) != 1 || c.Released != wantReleased {
			t.Fatalf("released=%v want %v: %+v", c.Released, wantReleased, c)
		}
	}
	check(true)
	file := filepath.Join(root, ".git", "shallow")
	if err := os.WriteFile(file, []byte("not-a-commit\n"), 0644); err != nil {
		t.Fatal(err)
	}
	check(false)
	if err := os.Remove(file); err != nil {
		t.Fatal(err)
	}
	check(true)
	// A second source with the same brief IDs must open its own repository.
	other := bulkV2Fixture(t, 1, 1, 1)
	otherStream := &Stream{Name: gv2Stream, Dir: filepath.Join(other, "docs", "streams", gv2Stream), Root: other}
	c := evaluateCoverage(other, []*Stream{otherStream}, coverageOptions{})[gv2Stream+"/01"]
	if !c.Released || len(c.Claims) != 1 {
		t.Fatalf("second root did not release: %+v", c)
	}
	a, b := activeGitReads.core[root], activeGitReads.core[other]
	if a.repo == nil || b.repo == nil || a.repo == b.repo {
		t.Fatal("different roots shared a canonical repository")
	}
}
