package deskkit

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
	"time"
)

// regression: F-cold-double-traversal
func TestSnapshotColdReference(t *testing.T) {
	count := snapshotObserver(t)
	for _, size := range []int{4, 256} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			root := t.TempDir()
			if err := os.Mkdir(filepath.Join(root, ".git"), 0700); err != nil {
				t.Fatal(err)
			}
			for i := 0; i < size; i++ {
				snapshotFile(t, filepath.Join(root, fmt.Sprintf("d%04d", i), "leaf.txt"), "fixture\n")
			}
			writeManifest(t, root, "test/verbs", []string{"assay.desk.verbs"}, []string{"test.absent"}, nil)
			var reference, cold []time.Duration
			kinds := []string{"enumeration", "reference-directory", "reference-read", "parse", "validation-directory", "validation-read", "reuse"}
			snapshot := func() map[string]int {
				r := map[string]int{}
				for _, k := range kinds {
					r[k] = count(k)
				}
				return r
			}
			check := func(before map[string]int) {
				t.Helper()
				expected := map[string]int{"enumeration": 1, "reference-directory": size + 3, "reference-read": 1, "parse": 1}
				for _, k := range kinds {
					if count(k)-before[k] != expected[k] {
						t.Fatalf("cold reference-equivalent work %s=%d want=%d", k, count(k)-before[k], expected[k])
					}
				}
			}
			// Interleave on identical files. Reset only the candidate slot, never the OS
			// cache. Both paths use the same observer; these are owner timings, not binaries.
			for i := 0; i < 9; i++ {
				before := snapshot()
				start := time.Now()
				want, ws, we := DiscoverManifests(root)
				reference = append(reference, time.Since(start))
				check(before)
				activationIndex.Lock()
				activationIndex.slot = nil
				activationIndex.Unlock()
				before = snapshot()
				start = time.Now()
				got, gs, ge := activationManifests(root)
				cold = append(cold, time.Since(start))
				check(before)
				if !reflect.DeepEqual(got, want) || !reflect.DeepEqual(gs, ws) || fmt.Sprint(ge) != fmt.Sprint(we) {
					t.Fatal("cold candidate changed reference result")
				}
			}
			sort.Slice(reference, func(i, j int) bool { return reference[i] < reference[j] })
			sort.Slice(cold, func(i, j int) bool { return cold[i] < cold[j] })
			t.Logf("reference baseline dirs=%d manifests=1 n=9 p50=%s p95=%s; cold candidate p50=%s p95=%s; each reference=1 reference-read=1 parse=1 validation-dirs=0 validation-read=0 reuse=0", size+3, reference[4], reference[8], cold[4], cold[8])
		})
	}
}
