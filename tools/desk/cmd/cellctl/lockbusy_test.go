package main

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/cellcadence"
	"github.com/medici-finance/assay/tools/desk/internal/cellscratch"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// Two role launches in one cell share the cell's scratch sweep lock. A sibling
// that is mid-sweep holds it for a moment; a launch that meets it then must wait
// its turn, not fail with a bare "advisory lock is held by another process".
//
// regression: #2709
func TestTwoLaunchesOneCell(t *testing.T) {
	c := &Cell{Name: "fixture", Dir: t.TempDir(), Config: t.TempDir()}
	roles := []string{"worker-desk", "pr-review-desk"}
	for _, role := range roles {
		if err := os.MkdirAll(c.cadenceDir(role), 0700); err != nil {
			t.Fatal(err)
		}
	}
	root := filepath.Join(c.Dir, "run", "scratch")
	s, err := cellscratch.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	s.Close()
	// The sibling: holds the sweep lock the way a launch's sweep does, briefly.
	f, err := os.OpenFile(filepath.Join(root, "sweep.lock"), os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err = deskkit.TryLockExclusive(f); err != nil {
		t.Fatal(err)
	}
	released := make(chan struct{})
	time.AfterFunc(400*time.Millisecond, func() {
		deskkit.UnlockFile(f)
		f.Close()
		close(released)
	})
	results := make([]cellcadence.Result, len(roles))
	var wg sync.WaitGroup
	for i, role := range roles {
		wg.Add(1)
		go func() {
			defer wg.Done()
			evidence := filepath.Join(t.TempDir(), "result")
			env := append(os.Environ(), "CELL_SCRATCH_FIXTURE=1", "ASSAY_SOURCE_REVISION=fixture-revision", "DESK_LOOP="+role, "SCRATCH_EVIDENCE="+evidence)
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			results[i] = c.executeCadencePass(ctx, role, "claude", []string{os.Args[0], "--fixture-prompt"}, env, t.TempDir())
		}()
	}
	wg.Wait()
	<-released
	for i, r := range results {
		if r.Err != nil || r.ExitCode != 0 || r.Outcome != "ok" {
			t.Fatalf("%s launch failed while a sibling held the sweep lock: %+v", roles[i], r)
		}
	}
}
