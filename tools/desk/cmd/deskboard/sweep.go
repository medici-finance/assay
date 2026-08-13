package main

// sweep.go — the bounded-concurrency roster sweep shared by the board-wide verbs.
//
// WHY THIS EXISTS. Every board-wide verb (actions / prs / queue / health / stalled /
// policydrift) used to walk `deskkit.AllowedRepos()` in a plain serial `for` loop, and
// inside each repo it walked the repo's PRs serially, shelling out through `ghRun` once
// per read. With a 15-repo roster that is a serial chain of 50–150 `gh` subprocess
// round-trips, and the wall-clock is dominated almost entirely by subprocess/network
// latency, not compute (a measured `actions` run spent ~3.5s of CPU across ~98s of wall
// clock — 96% of it waiting on `gh`). The fix is to run the per-repo work concurrently
// under a BOUNDED worker pool.
//
// THE STRUCTURE: per-repo → product → full. sweepRepos runs one closure
// per repo concurrently and returns each repo's PARTIAL result — its own rows/tombstone
// inputs/externals, never shared mutable state touched by two goroutines. The caller then
// merges partials into PRODUCT-level groups (see product()) and merges those into the full
// report. Because the final report is re-sorted to a total order AFTER the merge (see the
// determinism note on each caller), the output is byte-identical to the old serial version
// regardless of the order goroutines finish in — which is exactly what the delta snapshots
// and the #79 truncation/emptyscope invariants depend on. The product layer is also what a
// future `--product <name>` partial board would filter on.
//
// THE INVARIANTS THIS MUST NOT BREAK (all reviewer-checked):
//   - ghRun stays the single exec choke point. Concurrency is plain `exec.Command`s
//     running at once; there is no second exec route. Nothing here calls exec itself.
//   - Fail-closed. A repo error must still fail the WHOLE run and still name the
//     repo — concurrency must never downgrade a hard error into a partial "clean" board.
//     sweepRepos collects every repo's error and returns the LOWEST-INDEX repo's error
//     (roster order), so the propagated error is deterministic too, and any repo failing
//     fails the run. The empty-scope guard (#79 / emptyscope_test.go) lives in dispatch,
//     upstream of here, and is untouched.
//   - Bounded concurrency. Exactly `limit` worker goroutines are ever alive (a fixed
//     worker pool, not one goroutine per repo blocking on a semaphore), so a large roster
//     cannot explode into hundreds of concurrent `gh` subprocesses and trip GitHub's
//     secondary rate limits. The per-PR reads inside a repo stay SERIAL within that repo's
//     worker, so the total number of concurrent `gh` processes never exceeds `limit`.

import (
	"sort"
	"sync"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// sweepConcurrency bounds how many repos are swept at once — and, because each repo's
// per-PR reads run serially inside its worker, it also bounds the total number of
// concurrent `gh` subprocesses to this value.
//
// 6 is a deliberate middle: high enough to turn a 15-repo serial chain into ~3 waves
// (a several-× wall-clock win), low enough to stay well clear of GitHub's secondary
// rate limits on a burst of REST reads. The desk's outward-WRITE budget lives in
// deskkit/ratelimit.go and is a separate control (it caps mutations, which this read-only
// sweep never makes); this cap is the READ-side burst bound. Raising it trades latency for
// a higher risk of a secondary-limit 403/429, which on this code fails the whole run
// closed — so the safe direction to be wrong in is LOW.
const sweepConcurrency = 6

// sweepConcurrent runs `work` for every item concurrently under a fixed pool of `limit`
// workers and returns each item's result in INPUT ORDER (results[i] is work(items[i])), so
// a caller merging the slice sees a deterministic order regardless of finish order. It is
// the shared core behind both the per-repo sweep and the per-PR fan-out within a repo.
//
// Fail-closed: if any item's work returns an error, sweepConcurrent returns the error of
// the LOWEST-INDEX item that failed (and no results) — the whole call fails, the error is
// the one work() already wrapped (which names the repo/PR), and which error surfaces does
// not depend on goroutine timing. It does not cancel in-flight peers; the input is small
// and the workers are bounded, so draining them is cheap and keeps the failure deterministic.
//
// Exactly min(limit, len(items)) worker GOROUTINES are created — never one per item — so
// this cannot spawn an unbounded number of goroutines. The number of concurrent `gh`
// SUBPROCESSES is bounded separately and authoritatively by ghRun's ghSem, so nesting one
// sweepConcurrent (per-PR) inside another (per-repo) still cannot exceed ghConcurrency
// live subprocesses no matter what pool sizes the two levels pass.
func sweepConcurrent[In, Out any](items []In, limit int, work func(In) (Out, error)) ([]Out, error) {
	results := make([]Out, len(items))
	errs := make([]error, len(items))
	if len(items) == 0 {
		return results, nil
	}
	if limit < 1 {
		limit = 1
	}
	if limit > len(items) {
		limit = len(items)
	}

	jobs := make(chan int)
	var wg sync.WaitGroup
	for w := 0; w < limit; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				results[i], errs[i] = work(items[i])
			}
		}()
	}
	for i := range items {
		jobs <- i
	}
	close(jobs)
	wg.Wait()

	// Deterministic fail-closed: the first item (in input order) that errored wins.
	for i := range errs {
		if errs[i] != nil {
			return nil, errs[i]
		}
	}
	return results, nil
}

// sweepRepos is the per-repo entry point: it sweeps the roster concurrently, one result
// per repo in roster order. It is a thin wrapper over sweepConcurrent so the repo-level and
// per-PR-level fan-outs share one implementation of the ordering and fail-closed rules.
func sweepRepos[T any](repos []string, limit int, work func(repo string) (T, error)) ([]T, error) {
	return sweepConcurrent(repos, limit, work)
}

// ---------------------------------------------------------------------------
// product grouping (per-repo → product → full)
// ---------------------------------------------------------------------------

// product maps an "owner/name" repo to the PRODUCT group it belongs to, realizing the
// per-repo → product → full hierarchy. It is a thin reading of the ONE shared
// deskkit resolver (deskkit.RepoProductGroup) so deskboard and issueboard group
// through a single mechanism: a GENERIC compiled default (product = the repo's
// owner) plus the adopter's ASSAY_REPO_ALIASES override, where the house product
// buckets now live instead of a switch compiled into this tree.
//
// The grouping is DELIBERATELY not load-bearing for correctness: the full report is
// re-sorted to a total order after the merge, so a repo landing in the "wrong" product
// cannot change the board's bytes. Its job is to give the merge a real intermediate
// structure and to be the axis a future `--product <name>` fast partial board filters on.
func product(repo string) string {
	return deskkit.RepoProductGroup(repo)
}

// groupByProduct partitions repos into product groups, each group's repos kept in the
// input (roster) order, and returns the group names sorted. It is the seam a `--product`
// filter would use; the board-wide merge does not depend on the grouping because it
// re-sorts the merged result to a total order.
func groupByProduct(repos []string) (names []string, groups map[string][]string) {
	groups = map[string][]string{}
	for _, r := range repos {
		p := product(r)
		groups[p] = append(groups[p], r)
	}
	for name := range groups {
		names = append(names, name)
	}
	sort.Strings(names)
	return names, groups
}
