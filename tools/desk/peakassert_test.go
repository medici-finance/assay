package desk_test

// This is the SAMPLED-PEAK GUARD. It is a lexical tripwire for a defect class in this
// module's tests: proving
// that a pool runs work concurrently by letting each task sleep for a few milliseconds,
// recording the highest number of tasks seen in flight, and then asserting that peak was at
// least 2. The peak depends on the scheduler. On a loaded machine the tasks can finish one
// after another, so the assertion fails although the pool is correct, and a real run of
// `go test ./...` turns into a coin flip.
//
// The fix is a barrier: each task stays in flight until a second one is in flight beside it,
// with a bounded wait so a pool that never runs two at once fails instead of hanging.
// internal/loopengine's TestDrain is the worked example.
//
// The guard reads every _test.go file in the module for an `if <max|peak…> < 2` (or `<= 1`)
// lower bound on a sampled peak and fails on any file that is not on the allow-list below.
// The allow-list names the sites that still carry the old shape and are tracked for
// conversion; it may only shrink. An entry whose file no longer carries its count also
// fails, so a converted site cannot stay listed. A positive control proves the matcher
// still sees the shape, so a broken regex fails here rather than reporting clean.
//
// This is a MODULE-ROOT test (like TestLayout in layout_test.go) so it covers every package
// in the tools/desk module, not only the one that first hit the flake. It does not scan the
// repository's other Go modules; none of them carries the shape today.
//
// The match is LEXICAL, not semantic. It sees the common spellings (a plain variable, an
// atomic.LoadIntNN read, an atomic type's .Load() method) and misses rewrites such as
// `if got := …; got < 2` or `if 2 > maxSeen`, so a clean scan is evidence, not proof. It can
// also flag a non-concurrency variable (a retry limit named maxRetries compared below 2);
// rename that variable rather than adding an allow-list entry.

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// sampledPeakRe matches an assertion that a sampled maximum reached 2: an `if` over an
// identifier containing "max" or "peak" (optionally read through atomic.LoadIntNN or an
// atomic type's .Load() method) compared `< 2` or `<= 1`.
var sampledPeakRe = regexp.MustCompile(`(?i)\bif\s+(?:atomic\.Load\w*\(&?)?\w*(?:max|peak)\w*(?:\)|\.Load\(\))?\s*(?:<\s*2|<=\s*1)\b`)

// sampledPeakAllowed maps a module-relative test file to the number of sampled-peak
// assertions it may still carry. Both entries are tracked on #612 (convert them to a
// barrier). Remove an entry when its file is converted — never add one.
var sampledPeakAllowed = map[string]int{
	"cmd/deskboard/sweep_test.go": 1, // TestSweepRepos_BoundedConcurrency
	"cmd/muhar/harness_test.go":   1, // TestRun_ParallelActuallyOverlapsAndKeepsWorkspacesDisjoint
}

func TestNoSampledPeakAssert(t *testing.T) {
	// Positive control: the planted shape must match, or the scan below proves nothing.
	// Built by concatenation so this file does not match its own scan.
	for _, plant := range []string{
		"\tif max" + "InFlight < 2 {",
		"\tif atomic.LoadInt32(&" + "peak) <= 1 {",
		"\tif max" + "InFlight.Load() < 2 {",
	} {
		if !sampledPeakRe.MatchString(plant) {
			t.Fatalf("sampled-peak guard: the matcher no longer flags the planted line %q — the scan below would report clean on anything", plant)
		}
	}

	found := map[string]int{}
	err := filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == "testdata" || d.Name() == "vendor" || (path != "." && strings.HasPrefix(d.Name(), ".")) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, "_test.go") {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if n := len(sampledPeakRe.FindAllIndex(b, -1)); n > 0 {
			found[filepath.ToSlash(path)] = n
		}
		return nil
	})
	if err != nil {
		t.Fatalf("sampled-peak guard: could not walk the module: %v", err)
	}

	var problems []string
	for path, n := range found {
		if allowed := sampledPeakAllowed[path]; n > allowed {
			problems = append(problems, path+": asserts a sampled concurrency peak (`if max… < 2`) — the peak depends on the scheduler and fails under load; make each task wait on a bounded barrier until two are in flight instead (see internal/loopengine/drain_test.go TestDrain)")
		}
	}
	for path, allowed := range sampledPeakAllowed {
		if found[path] < allowed {
			problems = append(problems, path+": allow-listed for a sampled-peak assertion it no longer carries — remove the entry from sampledPeakAllowed")
		}
	}
	sort.Strings(problems)
	for _, p := range problems {
		t.Error("sampled-peak guard: " + p)
	}
}
