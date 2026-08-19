package main

import (
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// --- the CI rollup must be read IN FULL, and degrade CLOSED when it cannot be ---
//
// Before the fix, combinedStatusAt/checkRunsAt each issued ONE unpaginated request.
// GitHub's default page size is 30, so a head with 31 check runs whose 31st is a
// `failure` returned 30 successes → evalCI saw only greens → ciGreen → runReady FLIPPED a
// PR whose CI was actually RED. TotalCount was parsed and never used. These tests pin the
// two halves of the fix: (1) walk every page, (2) refuse when the pages we hold are
// shorter than the TotalCount GitHub reported.

// TestReadyRedCheckOnPageTwoRefuses is the paging repro, exactly as reported: 31 check runs
// at the head, the 31st a failure. The fake serves GitHub's default 30-per-page. The gate
// MUST see the page-2 failure and refuse (exit 5), with no flip.
func TestReadyRedCheckOnPageTwoRefuses(t *testing.T) {
	f, _ := setupFake(t)
	f.reviews = []reviewInfo{appReview("APPROVED", testHead, okReviewBody)}
	f.status = greenStatus()
	f.checks = manyChecks(31, 30) // 30 successes, then a failure at index 30 (the 31st)

	code := run(readyArgs(exampleRepo))
	if code != deskkit.ExitRefused {
		t.Fatalf("31-check-run rollup with a page-2 failure: exit = %d, want %d (refused) — "+
			"a green verdict here is the FAIL-OPEN", code, deskkit.ExitRefused)
	}
	if f.flips != 0 {
		t.Fatalf("flips = %d, want 0 — deskpost flipped a PR whose CI is RED", f.flips)
	}
}

// TestReadyRedCheckBeyondOnePerPageFetch — 130 check runs (> the 100-per-page max), the
// failure last. Correctness here REQUIRES a real page loop; a single per_page=100 request
// cannot see it.
func TestReadyRedCheckBeyondOnePerPageFetch(t *testing.T) {
	f, _ := setupFake(t)
	f.reviews = []reviewInfo{appReview("APPROVED", testHead, okReviewBody)}
	f.status = greenStatus()
	f.checks = manyChecks(130, 129)

	code := run(readyArgs(exampleRepo))
	if code != deskkit.ExitRefused {
		t.Fatalf("130-check-run rollup with the failure on page 2: exit = %d, want %d", code, deskkit.ExitRefused)
	}
	if f.flips != 0 {
		t.Fatalf("flips = %d, want 0", f.flips)
	}
}

// TestReadyAllGreenAcrossPagesFlips — the same >1-page shape, all green. Pagination must
// not turn a genuinely green rollup into a refusal (the fix degrades closed only when the
// rollup is short).
func TestReadyAllGreenAcrossPagesFlips(t *testing.T) {
	f, _ := setupFake(t)
	f.reviews = []reviewInfo{appReview("APPROVED", testHead, okReviewBody)}
	f.status = greenStatus()
	f.checks = manyChecks(130, -1)

	code := run(readyArgs(exampleRepo))
	if code != 0 {
		t.Fatalf("130 green check runs: exit = %d, want 0", code)
	}
	if f.flips != 1 {
		t.Fatalf("flips = %d, want 1", f.flips)
	}
}

// TestReadyTruncatedCheckRollupFailsClosed — the belt to pagination's braces: GitHub says
// total_count=31 but the pages only ever yield 30 items (a truncating/misbehaving API, a
// per_page it declined to honour, a page cap). We cannot positively read the head as
// green → unverifiable (exit 6), NOT green.
func TestReadyTruncatedCheckRollupFailsClosed(t *testing.T) {
	f, _ := setupFake(t)
	f.reviews = []reviewInfo{appReview("APPROVED", testHead, okReviewBody)}
	f.status = greenStatus()
	f.checks = manyChecks(30, -1) // 30 green runs...
	f.checks.TotalCount = 31      // ...but GitHub reports 31 exist at the head

	code := run(readyArgs(exampleRepo))
	if code != deskkit.ExitUnverifiable {
		t.Fatalf("short check rollup (30 of 31): exit = %d, want %d (unverifiable)", code, deskkit.ExitUnverifiable)
	}
	if f.flips != 0 {
		t.Fatalf("flips = %d, want 0 — a rollup we could not read in full is not green", f.flips)
	}
}

// TestReadyTruncatedStatusRollupFailsClosed — same, for the combined-status half.
func TestReadyTruncatedStatusRollupFailsClosed(t *testing.T) {
	f, _ := setupFake(t)
	f.reviews = []reviewInfo{appReview("APPROVED", testHead, okReviewBody)}
	f.status = greenStatus()
	f.status.TotalCount = 7 // one status returned, seven claimed

	code := run(readyArgs(exampleRepo))
	if code != deskkit.ExitUnverifiable {
		t.Fatalf("short status rollup (1 of 7): exit = %d, want %d (unverifiable)", code, deskkit.ExitUnverifiable)
	}
	if f.flips != 0 {
		t.Fatalf("flips = %d, want 0", f.flips)
	}
}

// TestCheckRunsAtPaginates — the client-side page walk, asserted directly: every page is
// requested until the rollup is exhausted, and the aggregate holds all 130 runs.
func TestCheckRunsAtPaginates(t *testing.T) {
	f, _ := setupFake(t)
	f.checks = manyChecks(130, -1)

	c, err := newGHClient("example-org", "tracker")
	if err != nil {
		t.Fatalf("client: %v", err)
	}
	cr, err := c.checkRunsAt(testHead)
	if err != nil {
		t.Fatalf("checkRunsAt: %v", err)
	}
	if len(cr.CheckRuns) != 130 {
		t.Fatalf("check runs fetched = %d, want 130 (the rollup was truncated)", len(cr.CheckRuns))
	}
	if cr.TotalCount != 130 {
		t.Fatalf("TotalCount = %d, want 130", cr.TotalCount)
	}
	if n := f.hitCount("GET", "/check-runs"); n < 2 {
		t.Fatalf("check-runs requests = %d, want >= 2 — a 130-item rollup cannot be read in one page", n)
	}
}

// --- evalCI unit table: the reconcile is part of the verdict, not an afterthought ---

func TestEvalCIShortRollupIsPending(t *testing.T) {
	cases := []struct {
		name string
		cs   combinedStatus
		cr   checkRunsResp
		want ciVerdict
	}{
		{
			name: "complete green rollup",
			cs:   greenStatus(),
			cr:   manyChecks(3, -1),
			want: ciGreen,
		},
		{
			name: "short check rollup → pending, not green",
			cs:   greenStatus(),
			cr:   func() checkRunsResp { c := manyChecks(3, -1); c.TotalCount = 4; return c }(),
			want: ciPending,
		},
		{
			name: "short status rollup → pending, not green",
			cs:   func() combinedStatus { s := greenStatus(); s.TotalCount = 2; return s }(),
			cr:   checkRunsResp{},
			want: ciPending,
		},
		{
			name: "short rollup + a visible failure → still RED (the stronger refusal wins)",
			cs:   combinedStatus{},
			cr:   func() checkRunsResp { c := manyChecks(3, 1); c.TotalCount = 9; return c }(),
			want: ciRed,
		},
		{
			name: "empty rollup, nothing claimed → empty (per-repo policy decides)",
			cs:   combinedStatus{},
			cr:   checkRunsResp{},
			want: ciEmpty,
		},
		{
			name: "empty rollup but checks claimed → pending, NOT empty",
			cs:   combinedStatus{},
			cr:   checkRunsResp{TotalCount: 5},
			want: ciPending,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := evalCI(&c.cs, &c.cr); got != c.want {
				t.Fatalf("evalCI = %v, want %v", got, c.want)
			}
		})
	}
}

// --- concurrency-supersede: a CANCELLED twin with a SUCCESS sibling is green (#1283) ---

// checkRun is one (name, status, conclusion) triple for buildChecks.
type checkRun struct{ name, status, conclusion string }

// buildChecks assembles a checkRunsResp from explicit runs, with TotalCount = len(runs)
// (a complete rollup, so the reconcile does not itself force pending). It models the raw
// per-run shape the check-runs REST endpoint returns — including a name that appears on
// MORE than one run, which is exactly the closecheck concurrency-supersede twin.
func buildChecks(runs ...checkRun) checkRunsResp {
	cr := checkRunsResp{TotalCount: len(runs)}
	for _, r := range runs {
		cr.CheckRuns = append(cr.CheckRuns, struct {
			Name       string `json:"name"`
			Status     string `json:"status"`
			Conclusion string `json:"conclusion"`
		}{Name: r.name, Status: r.status, Conclusion: r.conclusion})
	}
	return cr
}

// closecheckTwin is the real shape observed on #1263 (#1283): the closecheck
// workflow emits, per push, a CANCELLED run AND a SUCCESS run of the SAME check name at
// the SAME head — a concurrency-supersede. GitHub's rollup reads the SUCCESS.
func closecheckTwin() []checkRun {
	return []checkRun{
		{name: "closecheck", status: "completed", conclusion: "cancelled"},
		{name: "closecheck", status: "completed", conclusion: "success"},
	}
}

// TestEvalCICancelledTwinSuperseded pins the fix and its bound: a CANCELLED run with a
// SUCCESS sibling of the same name at head is superseded (green), but a CANCELLED with NO
// non-cancelled sibling still blocks, and a genuine failure of another name still blocks
// even when a superseded twin is present. The gate is relaxed for the supersede case ONLY.
func TestEvalCICancelledTwinSuperseded(t *testing.T) {
	cases := []struct {
		name string
		cs   combinedStatus
		cr   checkRunsResp
		want ciVerdict
	}{
		{
			name: "closecheck cancelled+success twin → GREEN (the #1263 live case)",
			cs:   combinedStatus{},
			cr:   buildChecks(closecheckTwin()...),
			want: ciGreen,
		},
		{
			name: "twin alongside a real green status rollup → GREEN",
			cs:   greenStatus(),
			cr:   buildChecks(closecheckTwin()...),
			want: ciGreen,
		},
		{
			name: "twin + an unrelated genuine failure → RED (failure still blocks)",
			cs:   combinedStatus{},
			cr: buildChecks(append(closecheckTwin(),
				checkRun{name: "build", status: "completed", conclusion: "failure"})...),
			want: ciRed,
		},
		{
			name: "CANCELLED with NO success sibling → RED (a real cancellation still blocks)",
			cs:   combinedStatus{},
			cr:   buildChecks(checkRun{name: "closecheck", status: "completed", conclusion: "cancelled"}),
			want: ciRed,
		},
		{
			name: "one name superseded, a DIFFERENT name cancelled-alone → RED",
			cs:   combinedStatus{},
			cr: buildChecks(append(closecheckTwin(),
				checkRun{name: "lint", status: "completed", conclusion: "cancelled"})...),
			want: ciRed,
		},
		{
			name: "the SUCCESS sibling does not license a same-name FAILURE → RED",
			cs:   combinedStatus{},
			cr: buildChecks(
				checkRun{name: "closecheck", status: "completed", conclusion: "cancelled"},
				checkRun{name: "closecheck", status: "completed", conclusion: "success"},
				checkRun{name: "closecheck", status: "completed", conclusion: "failure"}),
			want: ciRed,
		},
		{
			name: "twin + an unrelated real success → GREEN",
			cs:   combinedStatus{},
			cr: buildChecks(append(closecheckTwin(),
				checkRun{name: "build", status: "completed", conclusion: "success"})...),
			want: ciGreen,
		},
		{
			name: "cancelled superseded only by a still-PENDING rerun → not superseded, RED",
			cs:   combinedStatus{},
			cr: buildChecks(
				checkRun{name: "closecheck", status: "completed", conclusion: "cancelled"},
				checkRun{name: "closecheck", status: "in_progress", conclusion: ""}),
			want: ciRed,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := evalCI(&c.cs, &c.cr); got != c.want {
				t.Fatalf("evalCI = %v, want %v", got, c.want)
			}
		})
	}
}

// TestReadyCancelledTwinFlips is the end-to-end repro of #1263 through runReady: an
// open+draft PR, App APPROVED at head, and a CI rollup carrying the closecheck
// cancelled+success twin. Before the fix the CANCELLED run refused the flip with "CI is
// not green"; after it the head reads green (as GitHub itself does) and the PR flips.
func TestReadyCancelledTwinFlips(t *testing.T) {
	f, _ := setupFake(t)
	f.reviews = []reviewInfo{appReview("APPROVED", testHead, okReviewBody)}
	f.status = greenStatus()
	f.checks = buildChecks(closecheckTwin()...)

	code := run(readyArgs(exampleRepo))
	if code != 0 {
		t.Fatalf("cancelled+success twin: exit = %d, want 0 — GitHub reads the SUCCESS sibling "+
			"and reports the head green (#1283)", code)
	}
	if f.flips != 1 {
		t.Fatalf("flips = %d, want 1 — the superseded CANCELLED twin must not block the flip", f.flips)
	}
}

// TestReadyCancelledNoSiblingRefuses is the negative control: a lone CANCELLED run with no
// non-cancelled sibling of its name is a genuine cancellation and must still refuse (exit
// 5), so the supersede fix does not blanket-weaken the gate.
func TestReadyCancelledNoSiblingRefuses(t *testing.T) {
	f, _ := setupFake(t)
	f.reviews = []reviewInfo{appReview("APPROVED", testHead, okReviewBody)}
	f.status = greenStatus()
	f.checks = buildChecks(checkRun{name: "closecheck", status: "completed", conclusion: "cancelled"})

	code := run(readyArgs(exampleRepo))
	if code != deskkit.ExitRefused {
		t.Fatalf("lone cancelled run: exit = %d, want %d (refused) — a cancellation with no "+
			"success sibling is a genuine failure", code, deskkit.ExitRefused)
	}
	if f.flips != 0 {
		t.Fatalf("flips = %d, want 0 — a genuinely cancelled check must still block the flip", f.flips)
	}
}

// --- the drift anti-pattern: the CI-required set derives from the allowed-repo set, so they cannot drift ---

// TestCIRequiredMatchesAllowedRepoPolicy — the CI policy is testable for
// every explicit repo and representative org-default entries. The org-default cannot be
// exhaustively enumerated (it covers all present and future medici-finance repos), so
// the test pins representative entries + the fail-closed property for outside-repos.
func TestCIRequiredMatchesAllowedRepoPolicy(t *testing.T) {
	// Every repo in the compiled-in census must have a CI policy stated HERE. This is
	// the drift anti-pattern pin: adding a repo to the census without deciding its CI answer fails.
	want := map[string]bool{
		"example-org/tracker":            true,  // full PR CI
		"example-org/agents":             true,  // PR CI
		"example-org/examples":           false, // no PR CI (builds on merge-to-main)
		"medici-finance/assay":           true,  // HAS .github/workflows
		"example-org/example-k8s":        true,  // active Validate workflow on PRs
		"example-org/example-reconciler": true,  // ci.yml, `on: pull_request: branches: [main]`; runs complete on PR heads
		"example-org/org-slides":         false, // render-slides.yml exists but is push-only (no pull_request trigger)
		"example-org/proposals":          false, // empty repo
		// Checked per repo against
		// `gh api repos/<repo>/contents/.github/workflows`, not copied across the group
		// — platform runs a pull_request-triggered lint workflow; the
		// three slides repos have no workflows at all.
		"example-org/platform":                  true,  // lint.yml, `on: [pull_request]`
		"example-org/demo-slides":               false, // no .github/workflows — 404
		"example-org/assay-slides":              false, // no .github/workflows — 404
		"example-org/example-reconciler-slides": false, // no .github/workflows — 404
		"example-org/console":                   true,  // lint.yml, `on: [pull_request]`
	}
	for _, r := range deskkit.AllowedRepos() {
		w, ok := want[r]
		if !ok {
			t.Fatalf("census repo %q has no CI policy in this table", r)
		}
		if got := ciRequired(r); got != w {
			t.Fatalf("ciRequired(%q) = %v, want %v", r, got, w)
		}
	}
	// Org-default entries OUTSIDE the census cannot be enumerated (the default covers
	// every present and future medici-finance repo), so pin the PROPERTY instead: they
	// are all CI-required. `false` is the fail-open answer (an empty rollup reads as
	// green and `ready` flips), and nothing forces a CI decision for a repo that needs
	// no code edit to enter scope — so the default must be the safe one.
	for _, r := range []string{
		"medici-finance/example-alerts",        // never CI-classified
		"medici-finance/repo-created-tomorrow", // synthetic: no code edit needed to enter scope
		"example-org/site-lint",
		"example-org/assay-site",
	} {
		if _, inCensus := want[r]; inCensus {
			t.Fatalf("%q is in the census — it is not an org-default-only case", r)
		}
		if !ciRequired(r) {
			t.Fatalf("ciRequired(%q) = false for an off-census org-default repo — must fail closed", r)
		}
	}
	// Fail closed for anything outside the set (belt: the repo gate refuses first).
	if !ciRequired("attacker/slides") {
		t.Fatal("ciRequired must be TRUE (fail closed) for a repo outside the allowed set")
	}
}

// TestReadyEmptyCIReportReposGreen — the drift anti-pattern's operability half: repos with NO PR CI at
// all get an empty rollup that reads as green, so the flip goes through instead of the
// exit-6 dead end the desk hit before.
// example-org/example-reconciler moved OUT of this list: it grew ci.yml
// (`on: pull_request: branches: [main]`), so an empty rollup there now means "not
// reported yet" — see TestReadyEmptyCIUnverifiable below, which covers it alongside
// assay.
// example-org/proposals is CI-less TOO, but it is PUBLIC, so under the public-repo risk rule, it is
// risk-classed unconditionally and its flip needs a Security-Review: pass — see
// TestReadyPublicCIlessRepoStillNeedsSecurityReview below. Its CI half is unchanged.
//
// example-org/example-reconciler was in this list earlier: it gained `ci.yml`
// (`on: pull_request`) after the census reading, so an empty rollup there is
// "checks have not registered yet", not "green" (#310). It now belongs to
// TestReadyEmptyCICIRepoUnverifiable below.
func TestReadyEmptyCIReportReposGreen(t *testing.T) {
	for _, repo := range []string{
		"example-org/org-slides",
	} {
		t.Run(repo, func(t *testing.T) {
			f, _ := setupFake(t)
			f.reviews = []reviewInfo{appReview("APPROVED", testHead, okReviewBody)}
			// empty status + empty check rollup — this repo has no PR-triggered workflows

			code := run(readyArgs(repo))
			if code != 0 {
				t.Fatalf("%s empty-CI exit = %d, want 0 (CI-less repo → green)", repo, code)
			}
			if f.flips != 1 {
				t.Fatalf("flips = %d, want 1", f.flips)
			}
		})
	}
}

// TestReadyEmptyCIUnverifiable — repos that DO run PR CI must not treat an empty rollup
// as green: the drift anti-pattern's fix must not blanket-exempt the medici-finance org, and it must not
// exempt a repo (example-reconciler) the moment it grows CI either. assay has run PR CI
// throughout; example-reconciler joined it later (ci.yml, `on: pull_request`) — closing the gap where the compiled-in table still
// called example-reconciler CI-less.
func TestReadyEmptyCIUnverifiable(t *testing.T) {
	for _, repo := range []string{
		"medici-finance/assay",
		"example-org/example-reconciler",
	} {
		t.Run(repo, func(t *testing.T) {
			f, _ := setupFake(t)
			f.reviews = []reviewInfo{appReview("APPROVED", testHead, okReviewBody)}

			code := run(readyArgs(repo))
			if code != deskkit.ExitUnverifiable {
				t.Fatalf("%s empty-CI exit = %d, want %d — the repo has PR CI, so no rollup means "+
					"'not reported yet', not 'green'", repo, code, deskkit.ExitUnverifiable)
			}
			if f.flips != 0 {
				t.Fatalf("flips = %d, want 0", f.flips)
			}
		})
	}
}

// TestReadyEmptyCIRepoWithPRCIUnverifiable — the operability half, stated
// as behaviour rather than as a table value. Every one of these repos runs a workflow
// on `pull_request` (verified against the API), so an empty rollup means the
// checks have not registered YET. The flip must refuse, never flip on an empty read.
//
//	example-reconciler  ci.yml   on: push[main] + pull_request[main]
//	console             lint.yml on: [pull_request]
//	platform            lint.yml on: [pull_request]
//
// Each is a CI-required census repo, so an empty rollup means "not reported yet", not
// "green", and the flip must refuse. Setting CIRequired's default back to false reopens
// the drift anti-pattern here.
func TestReadyEmptyCIRepoWithPRCIUnverifiable(t *testing.T) {
	for _, repo := range []string{
		"example-org/example-reconciler",
		"example-org/console",
		"example-org/platform",
	} {
		t.Run(repo, func(t *testing.T) {
			f, _ := setupFake(t)
			f.reviews = []reviewInfo{appReview("APPROVED", testHead, okReviewBody)}
			// empty status + empty check rollup — checks have not registered yet

			code := run(readyArgs(repo))
			if code != deskkit.ExitUnverifiable {
				t.Fatalf("%s empty-CI exit = %d, want %d — the repo runs PR CI, so no rollup means "+
					"'not reported yet', not 'green'", repo, code, deskkit.ExitUnverifiable)
			}
			if f.flips != 0 {
				t.Fatalf("flips = %d, want 0 — an empty rollup must never flip a PR-CI repo", f.flips)
			}
		})
	}
}
