package main

import (
	"fmt"
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
		// the twentieth read falls inside the walk, which reads it before
		// each tree and at each visit.
		observe(root, ws, 20)
		if o := ws["1"][0].ReadSet; !timed(o) {
			t.Errorf("observation %s: want the walk cut short at the limit", obsText(o))
		}
	})
	t.Run("a merge of many parents", func(t *testing.T) {
		// One merge of 60 parents, each with its own root tree: the clock is
		// read before every tree the walk reads, the merge's parents
		// included, so the walk reads it at least once per commit and per
		// parent edge on top of once per visit, and a limit passed inside
		// the merge's parents ends the walk there.
		const parents = 60
		merged := func(t *testing.T) (string, map[string][]evaluation.Witness) {
			root, ws := committed(t, 1, true)
			ref := gitPipe(t, root, "", "symbolic-ref", "HEAD")
			from := gitFullHead(t, root)
			var b strings.Builder
			for i := 1; i <= parents; i++ {
				fmt.Fprintf(&b, "commit refs/heads/p%d\nmark :%d\ncommitter f <f@example.com> %d +0000\ndata 6\nchild\nfrom %s\nM 100644 inline p%d.txt\ndata 2\np\n\n", i, i, 1700000000+i, from, i)
			}
			fmt.Fprintf(&b, "commit %s\ncommitter f <f@example.com> %d +0000\ndata 6\nmerge\nfrom :1\n", ref, 1700000001+parents)
			for i := 2; i <= parents; i++ {
				fmt.Fprintf(&b, "merge :%d\n", i)
			}
			b.WriteString("\ndone\n")
			gitPipe(t, root, b.String(), "fast-import", "--quiet", "--done", "--force")
			return root, ws
		}
		root, ws := merged(t)
		reads, _ := observe(root, ws, 0)
		if o := ws["1"][0].ReadSet; o == nil || o.Problem != "" || !o.IntroFile {
			t.Fatalf("within the limit: observation %s: want the file found at its introduction", obsText(o))
		}
		commits, edges := parents+2, 2*parents
		if reads < 2*commits+edges {
			t.Errorf("the clock was read %d times over %d commits and %d parent edges; want at least %d, once per tree the walk reads and once per visit", reads, commits, edges, 2*commits+edges)
		}
		root, ws = merged(t)
		// The steps before the walk read the clock five times; the tenth read
		// falls among the merge's parents.
		observe(root, ws, 10)
		if o := ws["1"][0].ReadSet; !timed(o) {
			t.Errorf("observation %s: want the walk cut short inside the merge's parents", obsText(o))
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
