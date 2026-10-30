package gitcore

import (
	"fmt"
	"sync"
	"testing"

	"github.com/medici-finance/assay/gitcore/gittest"
)

// Query allocation must not grow by copying every ancestor's decoded record.
// The wide budget permits platform/runtime variation and counts no fixture work.
func TestCommitRangeRunIndexReuse(t *testing.T) {
	f := gittest.NewFixture(t)
	for i := 0; i < 40; i++ {
		f.CommitFile(t, "index.txt", fmt.Sprintf("%d\n", i), fmt.Sprintf("history %d", i))
	}
	id := mustGitOutput(t, f, "rev-parse", "HEAD")
	r, err := Open(f.Dir)
	if err != nil {
		t.Fatal(err)
	}
	if rows, err := r.CommitRange(id, id); err != nil || len(rows) != 0 {
		t.Fatalf("warmup %v %v", rows, err)
	}
	allocs := testing.AllocsPerRun(20, func() {
		rows, err := r.CommitRange(id, id)
		if err != nil || len(rows) != 0 {
			panic("immutable range changed")
		}
	})
	t.Logf("allocations per repeated empty range over 41 commits: %.0f", allocs)
	if allocs > 40 {
		t.Fatalf("range rebuilds ancestor records: %.0f allocations, budget 40", allocs)
	}
}

func TestCommitRangeConcurrentQueries(t *testing.T) {
	f := gittest.NewFixture(t)
	base := mustGitOutput(t, f, "rev-parse", "HEAD")
	f.CommitFile(t, "parallel.txt", "changed\n", "parallel")
	tip := mustGitOutput(t, f, "rev-parse", "HEAD")
	r, err := Open(f.Dir)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for worker := 0; worker < 8; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for query := 0; query < 10; query++ {
				rows, err := r.CommitRange(base, tip)
				if err != nil || len(rows) != 1 || len(rows[0].Parents) != 1 || rows[0].Parents[0] != base {
					t.Errorf("parallel query changed history: %v %v", rows, err)
					return
				}
				rows[0].Parents[0] = "caller-owned copy"
			}
		}()
	}
	wg.Wait()
}
