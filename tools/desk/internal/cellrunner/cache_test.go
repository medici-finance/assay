//go:build darwin || linux

package cellrunner

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/cellcache"
)

func TestCacheProcessCustody(t *testing.T) {
	cell, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	p, err := cellcache.Resolve(cell, func(k string) string {
		if k == "CELL_GO_CACHE" {
			return "on"
		}
		if k == "CELL_GO_CACHE_MIN_FREE" {
			return "0"
		}
		return ""
	})
	if err != nil {
		t.Fatal(err)
	}
	// The runner calls Finish(!uncertain) after the child; loopadmin/process
	// TestRunnerCustodyBoundary pins that mapping. Here the desk custodian must
	// hold the lease while admitted and keep it when cleanup was uncertain.
	for _, uncertain := range []bool{false, true} {
		admission, err := cacheCustody{}.Admit(p.Env())
		if err != nil {
			t.Fatal(err)
		}
		r, err := cellcache.Check(*p, true)
		if err != nil || len(r.SkippedActive) != 1 {
			t.Fatalf("child would run without custody: %+v %v", r, err)
		}
		if err := admission.Finish(!uncertain); err != nil {
			t.Fatal(err)
		}
		r, err = cellcache.Check(*p, true)
		if err != nil {
			t.Fatal(err)
		}
		want := 0
		if uncertain {
			want = 1
		}
		if len(r.SkippedActive) != want {
			t.Fatal("lost uncertain cache custody", r)
		}
		if uncertain {
			if err = cellcache.Recover(*p, true); err != nil {
				t.Fatal(err)
			}
		}
	}
	p.Floor = ^uint64(0)
	called := false
	_, _, err = RunObserved(t.Context(), []string{"/bin/sh", "-c", "exit 0"}, p.Env(), "", nil, nil, func(int) error { called = true; return nil })
	var held *cellcache.Deferred
	if called || !errors.As(err, &held) {
		t.Fatal("storage pressure launched a child", err)
	}
}
