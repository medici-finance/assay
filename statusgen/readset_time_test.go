package main

import (
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/medici-finance/assay/statusgen/evaluation"
)

// readset_time_test.go — build-less-brittle/15: the TIME an evaluation's
// read-set observations take follows briefs, not rows, and stops at one
// wall-clock limit (observeEvalTime), past which each observation is refused.

// countLaunches counts the git processes the coverage port starts while f
// runs.
func countLaunches(f func()) int64 {
	var n atomic.Int64
	coverageGitHook = func(string, []string) { n.Add(1) }
	defer func() { coverageGitHook = nil }()
	f()
	return n.Load()
}

// tickingClock makes src's clock read within its limit for the first
// trip-1 reads and past it from the trip-th on; trip 0 never trips. It
// returns the number of reads so far.
func tickingClock(src *readSetSource, trip int) *int {
	n := new(int)
	src.now = func() time.Time {
		*n++
		if trip > 0 && *n >= trip {
			return src.deadline.Add(time.Second)
		}
		return src.deadline.Add(-time.Hour)
	}
	return n
}

// regression: #2026
// TestCoverageObserveLaunches — the git processes one brief's observation
// starts follow briefs, not rows: a brief of hundreds of token rows whose
// read-set file exists starts the same few processes as a brief of two, the
// file's existence at every row's introduction read by one batch check.
func TestCoverageObserveLaunches(t *testing.T) {
	digest := strings.Repeat("d4", 32)
	const briefRel = "docs/streams/s/brief-01-x.md"
	launches := func(rows int) int64 {
		brief, ws := tokenRows(digest, rows)
		root := t.TempDir()
		writes(briefRel, brief, "docs/streams/s/reads/"+digest, "x\n")(t, root)
		mustGitInit(t, root)
		src := newReadSetSource(root)
		defer src.close()
		n := countLaunches(func() { src.observeBrief(filepath.Join(root, filepath.FromSlash(briefRel)), briefRel, ws) })
		for id, w := range ws {
			if o := w[0].ReadSet; o == nil || o.Problem != "" || !o.IntroFound || !o.IntroFile {
				t.Fatalf("%d rows, row %s: observation %s: want the file found at its introduction", rows, id, obsText(o))
			}
		}
		return n
	}
	few, many := launches(2), launches(400)
	t.Logf("git processes started: %d for 2 rows, %d for 400 rows", few, many)
	if many != few || many > 4 {
		t.Errorf("400 rows started %d git processes and 2 rows %d: want the same few, not one per row", many, few)
	}
}

// regression: #2026
// TestCoverageObserveDeadline — every step of the observations reads the
// evaluation's clock against observeEvalTime first: past it, a row, a history
// walk cut short and the brief's batch check each refuse the observation with
// the limit named, and nothing past the limit starts a git process.
func TestCoverageObserveDeadline(t *testing.T) {
	digest := strings.Repeat("e5", 32)
	const briefRel = "docs/streams/s/brief-01-x.md"
	fileRel := "docs/streams/s/reads/" + digest
	limit := observeEvalTime.String()
	timed := func(o *evaluation.ReadSetObservation) bool {
		return o != nil && strings.Contains(o.Problem, "time limit of "+limit)
	}
	committed := func(t *testing.T, rows int, file bool) (string, map[string][]evaluation.Witness) {
		brief, ws := tokenRows(digest, rows)
		root := t.TempDir()
		kv := []string{briefRel, brief}
		if file {
			kv = append(kv, fileRel, "x\n")
		}
		writes(kv...)(t, root)
		mustGitInit(t, root)
		return root, ws
	}
	observe := func(root string, ws map[string][]evaluation.Witness, trip int) (int, int64) {
		src := newReadSetSource(root)
		defer src.close()
		reads := tickingClock(src, trip)
		n := countLaunches(func() { src.observeBrief(filepath.Join(root, filepath.FromSlash(briefRel)), briefRel, ws) })
		return *reads, n
	}

	t.Run("past the limit from the start", func(t *testing.T) {
		root, ws := committed(t, 3, true)
		if _, n := observe(root, ws, 1); n != 0 {
			t.Errorf("%d git processes started past the limit, want none", n)
		}
		for id, w := range ws {
			if o := w[0].ReadSet; !timed(o) {
				t.Errorf("row %s: observation %s: want the time limit named", id, obsText(o))
			}
		}
	})
	t.Run("rows past the limit", func(t *testing.T) {
		// No row's file exists, so no history is walked: each row reads the
		// clock once, and the rows from the tenth on are refused.
		root, ws := committed(t, 50, false)
		observe(root, ws, 10)
		refused := 0
		for _, w := range ws {
			if timed(w[0].ReadSet) {
				refused++
			}
		}
		if refused != 41 {
			t.Errorf("%d rows refused past the limit, want 41", refused)
		}
	})
	t.Run("a history walk cut short", func(t *testing.T) {
		head, ws := tokenRows(digest, 1)
		root := t.TempDir()
		writes(briefRel, head, fileRel, "x\n")(t, root)
		mustGitInit(t, root)
		var commits []map[string]int
		for i := 0; i < 40; i++ {
			commits = append(commits, map[string]int{briefRel: i % 2})
		}
		importHistory(t, root, []string{historicRows(2, 3, 0), head}, commits)
		// The steps outside the walk read the clock five times; tripping at
		// the twentieth read falls inside the walk, one read per commit.
		observe(root, ws, 20)
		if o := ws["1"][0].ReadSet; !timed(o) {
			t.Errorf("observation %s: want the walk cut short at the limit", obsText(o))
		}
	})
	t.Run("the batch check past the limit", func(t *testing.T) {
		root, ws := committed(t, 3, true)
		reads, launched := observe(root, ws, 0)
		for id, w := range ws {
			if o := w[0].ReadSet; o == nil || o.Problem != "" || !o.IntroFile {
				t.Fatalf("row %s within the limit: observation %s: want the file found at its introduction", id, obsText(o))
			}
		}
		// The last read of the clock is the one before the batch check.
		root, ws = committed(t, 3, true)
		if _, n := observe(root, ws, reads); n != launched-1 {
			t.Errorf("%d git processes started, want %d: the batch check ran past the limit", n, launched-1)
		}
		for id, w := range ws {
			if o := w[0].ReadSet; !timed(o) || o.IntroFile {
				t.Errorf("row %s: observation %s: want the batch check refused at the limit", id, obsText(o))
			}
		}
	})
}
