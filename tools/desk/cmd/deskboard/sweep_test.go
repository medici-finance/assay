package main

// sweep_test.go — proofs for the bounded-concurrency roster sweep (sweep.go):
//
//	(a) DETERMINISM  — the concurrent sweep returns results in the SAME order as a serial
//	                   reference, byte-identical, regardless of goroutine finish order; and
//	                   the whole `actions` board is byte-stable across repeated runs.
//	(b) FAIL-CLOSED  — a mid-sweep repo error fails the WHOLE run (exit 6) and names the
//	                   repo, at both the sweepRepos unit level and the `actions` verb level,
//	                   even when other repos succeed concurrently (must not degrade to a
//	                   partial "clean" board).
//	(c) BOUNDED      — the pool never runs more than `limit` work functions at once, so a
//	                   large roster cannot explode into unbounded concurrent gh subprocesses.

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// TestSweepRepos_OrderMatchesSerial proves the determinism guarantee at its source: for a
// fixed input, the concurrent sweep (limit=6) returns the identical ordered slice as a
// serial reference (limit=1), even though each unit of work sleeps a random amount so the
// goroutines finish out of order. If ordering leaked from finish order this fails.
func TestSweepRepos_OrderMatchesSerial(t *testing.T) {
	repos := make([]string, 40)
	for i := range repos {
		repos[i] = fmt.Sprintf("owner/repo-%02d", i)
	}
	work := func(repo string) (string, error) {
		time.Sleep(time.Duration(rand.Intn(3)) * time.Millisecond)
		return "seen:" + repo, nil
	}

	serial, err := sweepRepos(repos, 1, work)
	if err != nil {
		t.Fatalf("serial sweep: %v", err)
	}
	// Expected is exactly input order, one result per repo.
	for i, repo := range repos {
		if want := "seen:" + repo; serial[i] != want {
			t.Fatalf("serial[%d] = %q, want %q", i, serial[i], want)
		}
	}
	// Run the concurrent sweep many times; every run must equal the serial reference.
	for trial := 0; trial < 25; trial++ {
		got, err := sweepRepos(repos, sweepConcurrency, work)
		if err != nil {
			t.Fatalf("concurrent sweep trial %d: %v", trial, err)
		}
		for i := range repos {
			if got[i] != serial[i] {
				t.Fatalf("trial %d: got[%d]=%q != serial[%d]=%q — finish order leaked into result order",
					trial, i, got[i], i, serial[i])
			}
		}
	}
}

// TestSweepRepos_FailClosed proves a mid-sweep error fails the whole sweep, that the
// returned error is the LOWEST-INDEX failing repo (deterministic, not timing-dependent),
// and that no results are handed back to be mistaken for a partial board.
func TestSweepRepos_FailClosed(t *testing.T) {
	repos := []string{"o/a", "o/b", "o/c", "o/d", "o/e", "o/f"}
	// Two repos fail; the sweep must surface the FIRST one in roster order (o/c), never
	// whichever goroutine happened to error first.
	work := func(repo string) (string, error) {
		time.Sleep(time.Duration(rand.Intn(3)) * time.Millisecond)
		if repo == "o/c" || repo == "o/e" {
			return "", fmt.Errorf("boom on %s", repo)
		}
		return "ok:" + repo, nil
	}
	for trial := 0; trial < 25; trial++ {
		got, err := sweepRepos(repos, sweepConcurrency, work)
		if err == nil {
			t.Fatalf("trial %d: sweep returned nil error despite a failing repo — fail-open", trial)
		}
		if got != nil {
			t.Fatalf("trial %d: sweep returned results alongside an error — a partial board could leak", trial)
		}
		if err.Error() != "boom on o/c" {
			t.Fatalf("trial %d: error = %q, want the lowest-index failing repo %q (non-deterministic propagation)",
				trial, err.Error(), "boom on o/c")
		}
	}
}

// TestSweepRepos_BoundedConcurrency proves the pool never runs more than `limit` work
// functions at once (the secondary-rate-limit safety property) while still being
// genuinely concurrent (max > 1).
func TestSweepRepos_BoundedConcurrency(t *testing.T) {
	const limit = 3
	repos := make([]string, 30)
	for i := range repos {
		repos[i] = fmt.Sprintf("o/r%02d", i)
	}
	var mu sync.Mutex
	var live, maxLive int
	work := func(string) (int, error) {
		mu.Lock()
		live++
		if live > maxLive {
			maxLive = live
		}
		mu.Unlock()
		time.Sleep(5 * time.Millisecond) // widen the overlap window
		mu.Lock()
		live--
		mu.Unlock()
		return 0, nil
	}
	if _, err := sweepRepos(repos, limit, work); err != nil {
		t.Fatalf("sweep: %v", err)
	}
	if maxLive > limit {
		t.Fatalf("observed %d concurrent workers, limit was %d — the pool is unbounded", maxLive, limit)
	}
	if maxLive < 2 {
		t.Fatalf("observed max concurrency %d — the sweep did not actually run concurrently", maxLive)
	}
}

// TestSweepRepos_WorkerCountBounded proves the pool creates a BOUNDED number of workers,
// not one goroutine per repo: with 500 repos and limit=4, the work function is never
// entered by more than 4 goroutines at once.
func TestSweepRepos_WorkerCountBounded(t *testing.T) {
	repos := make([]string, 500)
	for i := range repos {
		repos[i] = fmt.Sprintf("o/big%03d", i)
	}
	var inFlight, peak int32
	work := func(string) (int, error) {
		n := atomic.AddInt32(&inFlight, 1)
		for {
			p := atomic.LoadInt32(&peak)
			if n <= p || atomic.CompareAndSwapInt32(&peak, p, n) {
				break
			}
		}
		atomic.AddInt32(&inFlight, -1)
		return 0, nil
	}
	if _, err := sweepRepos(repos, 4, work); err != nil {
		t.Fatalf("sweep: %v", err)
	}
	if peak > 4 {
		t.Fatalf("peak concurrent workers = %d over 500 repos, limit 4 — goroutine explosion", peak)
	}
}

// TestSweepRepos_EmptyAndSingle covers the degenerate inputs: an empty roster returns an
// empty non-nil slice with no error (the empty-scope refusal lives upstream in dispatch,
// #79 / emptyscope_test.go — sweepRepos must not itself panic or fabricate), and a
// single-repo roster still runs.
func TestSweepRepos_EmptyAndSingle(t *testing.T) {
	got, err := sweepRepos(nil, sweepConcurrency, func(string) (int, error) { return 1, nil })
	if err != nil || len(got) != 0 {
		t.Fatalf("empty sweep = (%v, %v), want ([], nil)", got, err)
	}
	got, err = sweepRepos([]string{"o/only"}, sweepConcurrency, func(string) (int, error) { return 7, nil })
	if err != nil || len(got) != 1 || got[0] != 7 {
		t.Fatalf("single sweep = (%v, %v), want ([7], nil)", got, err)
	}
}

// TestProductGrouping pins the per-repo → product → full grouping map: every roster repo
// lands in exactly one product, the group names come back sorted, and an unknown repo
// falls into the catch-all rather than being dropped (which would silently shrink scope).
func TestProductGrouping(t *testing.T) {
	repos := []string{
		"example-org/tracker",
		"example-org/example-reconciler",
		"medici-finance/assay",
		"example-org/example-k8s",
		"example-org/proposals", // catch-all
	}
	names, groups := groupByProduct(repos)
	// sorted
	for i := 1; i < len(names); i++ {
		if names[i-1] > names[i] {
			t.Fatalf("product names not sorted: %v", names)
		}
	}
	// no repo dropped
	total := 0
	for _, g := range groups {
		total += len(g)
	}
	if total != len(repos) {
		t.Fatalf("grouping dropped repos: %d grouped, %d input", total, len(repos))
	}
	if product("example-org/proposals") != "demo" {
		t.Fatalf("unknown repo should fall into the demo catch-all, got %q", product("example-org/proposals"))
	}
	if product("example-org/tracker") != "tracker" {
		t.Fatalf("tracker grouping wrong: %q", product("example-org/tracker"))
	}
}

// TestActions_OrderStableAcrossRuns is the end-to-end determinism proof: over a fixed
// multi-repo, multi-PR fixture, `actions` produces the SAME ordered rows/external/tombstone
// sequence on every run even though the repos (and their branch-health probes) are swept
// concurrently. The comparison is on the ordered identity sequence (repo#number) — the one
// thing goroutine finish order could disturb — not on the raw bytes, because the header's
// asOf timestamp legitimately varies between runs. A differing sequence means concurrency
// leaked into ordering.
func TestActions_OrderStableAcrossRuns(t *testing.T) {
	// Every repo returns the SAME two PRs — identical scores and PR numbers across repos
	// stress the tie-break (score → number → repo), the one place a non-total sort would
	// let finish order decide.
	prList := `[` +
		`{"number":7,"title":"alpha","state":"OPEN","isDraft":true,"author":{"login":"shared-agent"},"headRefOid":"abc123","mergeStateStatus":"BLOCKED","statusCheckRollup":[]},` +
		`{"number":3,"title":"beta","state":"OPEN","isDraft":false,"author":{"login":"shared-agent"},"headRefOid":"def456","mergeStateStatus":"BLOCKED","statusCheckRollup":[]}` +
		`]`

	runOnce := func() string {
		installFakeGH(t)
		t.Setenv("DESKBOARD_GH_PRLIST_JSON", prList)
		var out, errb bytes.Buffer
		if code := run([]string{"actions"}, &out, &errb); code != deskkit.ExitOK {
			t.Fatalf("run(actions) = exit %d, stderr=%s", code, errb.String())
		}
		var rep actionsReport
		if err := json.Unmarshal(out.Bytes(), &rep); err != nil {
			t.Fatalf("parsing actions JSON: %v\n%s", err, out.String())
		}
		var seq bytes.Buffer
		for _, r := range rep.Rows {
			fmt.Fprintf(&seq, "row %s#%d score=%d act=%s|", r.Repo, r.Number, r.Score, r.Action)
		}
		for _, e := range rep.External {
			fmt.Fprintf(&seq, "ext %s#%d|", e.Repo, e.Number)
		}
		for _, ts := range rep.Tombstones {
			fmt.Fprintf(&seq, "tomb %s#%d|", ts.Repo, ts.Number)
		}
		// The health block rides on the header and is sorted by rank+repo — include its
		// order too, since assessBranchHealth is now swept concurrently.
		if rep.Header.MainHealth != nil {
			for _, hr := range rep.Header.MainHealth.Repos {
				fmt.Fprintf(&seq, "mh %s=%s|", hr.Repo, hr.State)
			}
		}
		return seq.String()
	}

	first := runOnce()
	if first == "" {
		t.Fatal("fixture produced no rows — determinism check is vacuous")
	}
	for trial := 0; trial < 8; trial++ {
		if got := runOnce(); got != first {
			t.Fatalf("actions ordering not stable on trial %d — concurrency leaked into ordering.\nfirst: %s\ngot:   %s",
				trial, first, got)
		}
	}
}

// TestActions_FailClosed_Concurrent is the fail-closed proof for the CONVERTED `actions` verb:
// one repo failing mid-sweep fails the whole run (exit 6) and names the repo, while the
// OTHER repos in the roster succeed concurrently. The concurrency must not let the
// succeeding repos assemble a clean board that hides the failure.
func TestActions_FailClosed_Concurrent(t *testing.T) {
	installFakeGH(t)
	// Give the succeeding repos real PRs so they produce rows a naive implementation might
	// emit as a "clean" board; the one failing repo must still sink the whole run.
	t.Setenv("DESKBOARD_GH_PRLIST_JSON",
		`[{"number":1,"title":"t","state":"OPEN","isDraft":true,"author":{"login":"shared-agent"},"headRefOid":"abc123","mergeStateStatus":"BLOCKED","statusCheckRollup":[]}]`)
	failing := "example-org/example-k8s"
	t.Setenv("DESKBOARD_GH_FAIL_REPO", failing)

	var out, errb bytes.Buffer
	code := run([]string{"actions"}, &out, &errb)
	if code != deskkit.ExitUnverifiable {
		t.Fatalf("run(actions) with a failing repo = exit %d, want %d (fail-closed)", code, deskkit.ExitUnverifiable)
	}
	if !bytes.Contains(errb.Bytes(), []byte(failing)) {
		t.Errorf("exit-6 message must name the failing repo %q; got: %s", failing, errb.String())
	}
	// No partial board on stdout — the actions report marshals a "rows" key.
	if bytes.Contains(out.Bytes(), []byte(`"rows"`)) {
		t.Errorf("a failed run must not emit a (partial) board on stdout; got: %s", out.String())
	}
}

// TestActions_FailClosed_PerPRRead is the fail-closed proof for the PER-PR fan-out specifically:
// a single PR's /reviews read failing mid-sweep must still fail the WHOLE run (exit 6),
// even though every repo's pr-list read and every other PR succeed concurrently. This
// guards the finer-grained concurrency the per-PR fan-out introduced — a failed leaf read
// must never be swallowed into a partial clean board.
func TestActions_FailClosed_PerPRRead(t *testing.T) {
	installFakeGH(t)
	t.Setenv("DESKBOARD_GH_PRLIST_JSON",
		`[{"number":1,"title":"t","state":"OPEN","isDraft":true,"author":{"login":"shared-agent"},"headRefOid":"abc123","mergeStateStatus":"BLOCKED","statusCheckRollup":[]}]`)
	// Fail only the per-PR reviews read (leaves pr-list and everything else answering).
	t.Setenv("DESKBOARD_GH_FAIL_PATH", "/reviews")

	var out, errb bytes.Buffer
	code := run([]string{"actions"}, &out, &errb)
	if code != deskkit.ExitUnverifiable {
		t.Fatalf("run(actions) with a failing per-PR read = exit %d, want %d (fail-closed)", code, deskkit.ExitUnverifiable)
	}
	if bytes.Contains(out.Bytes(), []byte(`"rows"`)) {
		t.Errorf("a failed per-PR read must not emit a (partial) board on stdout; got: %s", out.String())
	}
}

// TestActions_WedgedRead_TimesOut is the #594 proof the other fail-closed tests could not
// give: a `gh` subprocess that WEDGES — blocks far longer than the per-unit budget, the way
// a blocking auth/token-refresh did in #594 — must be KILLED by ghRun's context deadline
// and become a terminable error, so the run fails CLOSED (exit 6) instead of stalling the
// whole concurrent sweep forever. A fast exit-1 (the DESKBOARD_GH_FAIL_PATH tests) never
// exercised the hang; before the per-unit ghTimeout this run had no output and never
// returned. It uses the real exec path (PATH shim) so the deadline and the subprocess kill
// are genuinely exercised, not stubbed, and it is wrapped in a wall-clock guard: if the run
// does not return, the deadline did not fire and that is the #594 regression.
func TestActions_WedgedRead_TimesOut(t *testing.T) {
	installFakeGH(t)
	// Wedge every repo's open-PR read (now a `gh api graphql`, #2024); each wedged
	// subprocess must be killed at ghTimeout.
	t.Setenv("DESKBOARD_GH_HANG_PATH", "pullRequests(states:OPEN")

	// Shrink the per-unit budget so the test is fast; the shim sleeps 60s, far beyond it.
	prev := ghTimeout
	ghTimeout = 300 * time.Millisecond
	t.Cleanup(func() { ghTimeout = prev })

	var out, errb bytes.Buffer
	done := make(chan int, 1)
	go func() { done <- run([]string{"actions"}, &out, &errb) }()

	select {
	case code := <-done:
		if code != deskkit.ExitUnverifiable {
			t.Fatalf("a wedged read must fail closed: exit %d, want %d", code, deskkit.ExitUnverifiable)
		}
		if bytes.Contains(out.Bytes(), []byte(`"rows"`)) {
			t.Errorf("a wedged run must not emit a (partial) board on stdout; got: %s", out.String())
		}
		if !bytes.Contains(errb.Bytes(), []byte("timed out")) {
			t.Errorf("the exit-6 message should report the timeout; got: %s", errb.String())
		}
	case <-time.After(20 * time.Second):
		t.Fatal("run(actions) hung on a wedged gh subprocess — ghRun's per-unit deadline did not fire (#594 regression)")
	}
}

// TestSweepRepos_AllErrorPaths is a guard that a NON-first failure still fails: even if the
// only failing repo is the LAST one swept, the sweep reports it. Pairs with the fail-closed
// test above by removing any "first repo is special" assumption.
func TestSweepRepos_LastRepoError(t *testing.T) {
	repos := []string{"o/a", "o/b", "o/c", "o/d"}
	sentinel := errors.New("boom on o/d")
	work := func(repo string) (string, error) {
		if repo == "o/d" {
			return "", sentinel
		}
		return "ok", nil
	}
	_, err := sweepRepos(repos, sweepConcurrency, work)
	if !errors.Is(err, sentinel) {
		t.Fatalf("a last-repo failure must propagate; got %v", err)
	}
}
