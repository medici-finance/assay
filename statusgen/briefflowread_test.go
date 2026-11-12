package main

// briefflowread_test.go — forge-neutral/18: the merged-PR list and the per-PR review array the
// brief-flow metrics (--review-rework, --first-pass-yield, --assayscore's Quality dimension) read
// reach the forge through `deskread changes` and `deskread reviews`, never a `gh api` shell-out,
// and every read the verb could not serve in full is an error the callers report as
// could-not-check.
//
// The production default is the named var defaultBFPRSource, so these rows run the wiring the
// modes run. `gh` is stubbed to fail on every row: a read that still reached it errors.
//
// MUTATIONS that re-redden the rows on the fixed tree: drop the window-coverage test from
// MergedChanges so an incomplete list is always accepted (TestBFMergedUnserved/incomplete rows) or
// always refused (TestBFMergedIncompleteCovered); drop the `main` base filter in
// deskreadBFPRSource.MergedPRs (TestBFMergedViaDeskread); read the reviews with
// --issue instead of --change (TestBFReviewsViaDeskread).

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// stubBFVerbs puts a failing `gh` and a `deskread` that answers `changes` with changesBody and
// `reviews` with reviewsBody first on PATH, and returns the file the stub appends each call's argv
// to (one call a line).
func stubBFVerbs(t *testing.T, changesBody, reviewsBody string) string {
	t.Helper()
	bin := t.TempDir()
	argsFile := filepath.Join(t.TempDir(), "deskread.args")
	writeStubBin(t, bin, "gh", ghFails)
	writeStubBin(t, bin, "deskread", "#!/bin/sh\n"+
		"echo \"$*\" >> '"+argsFile+"'\n"+
		"case \"$1\" in\n"+
		"changes) cat <<'EOF'\n"+changesBody+"\nEOF\n;;\n"+
		"reviews) cat <<'EOF'\n"+reviewsBody+"\nEOF\n;;\n"+
		"*) exit 2;;\nesac\n")
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	return argsFile
}

// lastStubCall is the stub's last recorded argv. The reader first probes the verb's --help for the
// perf-context version gate, so the data call is the last line, not the only one.
func lastStubCall(t *testing.T, argsFile string) string {
	t.Helper()
	b, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatalf("the deskread stub was never called: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(b)), "\n")
	return lines[len(lines)-1]
}

const bfRepo = "example-org/alpha"

// regression: the merged-PR read shelled `gh api pulls?state=closed&base=main`, so it failed wherever gh was absent or unauthenticated
func TestBFMergedViaDeskread(t *testing.T) {
	argsFile := stubBFVerbs(t, `{"schema":1,"kind":"changes","repos":[{"repo":"`+bfRepo+`","changes":[`+
		`{"number":30,"state":"MERGED","body":"Brief: s/01","baseRef":"main","mergedAt":"2026-09-10T00:00:00Z"},`+
		`{"number":29,"state":"MERGED","body":"stacked","baseRef":"feat/other","mergedAt":"2026-09-09T00:00:00Z"},`+
		`{"number":28,"state":"MERGED","body":"old","baseRef":"main","mergedAt":"2026-07-01T00:00:00Z"}],`+
		`"incomplete":false,"pageCap":5}],"partial":[]}`, `{}`)

	got, err := defaultBFPRSource.MergedPRs(bfRepo, time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("defaultBFPRSource errored with a working deskread and no working gh: %v", err)
	}
	if len(got) != 1 || got[0].Number != 30 || got[0].Body != "Brief: s/01" || got[0].MergedAt != "2026-09-10T00:00:00Z" {
		t.Fatalf("merged PRs = %+v, want only #30 (#29 is not based on main, #28 merged before the window)", got)
	}
	if want := "changes --repo " + bfRepo + " --state merged"; lastStubCall(t, argsFile) != want {
		t.Errorf("deskread argv = %q, want %q", lastStubCall(t, argsFile), want)
	}
}

// regression: a window busier than gh's one 100-row page read as a complete one; an unservable list must be could-not-check, never short
func TestBFMergedUnserved(t *testing.T) {
	since := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	ok := `{"number":1,"state":"MERGED","baseRef":"main","mergedAt":"2026-09-10T00:00:00Z"}`
	okU := `{"number":1,"state":"MERGED","baseRef":"main","mergedAt":"2026-09-10T00:00:00Z","updatedAt":"2026-09-11T00:00:00Z"}`
	for _, tc := range []struct{ name, body string }{
		{"incomplete, no updatedAt to show the window covered", `{"schema":1,"kind":"changes","repos":[{"repo":"` + bfRepo + `","changes":[` + ok + `],"incomplete":true,"pageCap":5}],"partial":[]}`},
		{"incomplete, oldest row still inside the window", `{"schema":1,"kind":"changes","repos":[{"repo":"` + bfRepo + `","changes":[` + okU + `],"incomplete":true,"pageCap":5}],"partial":[]}`},
		{"incomplete, no rows", `{"schema":1,"kind":"changes","repos":[{"repo":"` + bfRepo + `","changes":[],"incomplete":true,"pageCap":5}],"partial":[]}`},
		{"no completeness field", `{"schema":1,"kind":"changes","repos":[{"repo":"` + bfRepo + `","changes":[` + ok + `]}],"partial":[]}`},
		{"no change list", `{"schema":1,"kind":"changes","repos":[{"repo":"` + bfRepo + `","incomplete":false}],"partial":[]}`},
		{"partial", `{"schema":1,"kind":"changes","repos":[],"partial":[{"repo":"` + bfRepo + `","reason":"the App installation cannot read it"}]}`},
		{"repo absent", `{"schema":1,"kind":"changes","repos":[],"partial":[]}`},
		{"wrong kind", `{"schema":1,"kind":"issues","repos":[{"repo":"` + bfRepo + `","changes":[],"incomplete":false}],"partial":[]}`},
		{"unknown schema", `{"schema":2,"kind":"changes","repos":[{"repo":"` + bfRepo + `","changes":[],"incomplete":false}],"partial":[]}`},
		{"merged with no merge time", `{"schema":1,"kind":"changes","repos":[{"repo":"` + bfRepo + `","changes":[{"number":1,"state":"MERGED","baseRef":"main"}],"incomplete":false}],"partial":[]}`},
		{"merged with no base", `{"schema":1,"kind":"changes","repos":[{"repo":"` + bfRepo + `","changes":[{"number":1,"state":"MERGED","mergedAt":"2026-09-10T00:00:00Z"}],"incomplete":false}],"partial":[]}`},
		{"no envelope", `not json`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stubBFVerbs(t, tc.body, `{}`)
			got, err := defaultBFPRSource.MergedPRs(bfRepo, since)
			if err == nil {
				t.Fatalf("defaultBFPRSource returned %+v and no error for an unserved list — it must be could-not-check", got)
			}
		})
	}
}

// regression: the verb's walk is bounded by pages, not by the window, so a repo with a long merged history always reports incomplete; that alone made every window could-not-check
func TestBFMergedIncompleteCovered(t *testing.T) {
	// Most-recently-updated first: #30 merged in the window, #28 last updated before it, so every
	// unlisted change is older than the window even though the walk stopped at its page bound.
	stubBFVerbs(t, `{"schema":1,"kind":"changes","repos":[{"repo":"`+bfRepo+`","changes":[`+
		`{"number":30,"state":"MERGED","baseRef":"main","mergedAt":"2026-09-10T00:00:00Z","updatedAt":"2026-09-11T00:00:00Z"},`+
		`{"number":28,"state":"MERGED","baseRef":"main","mergedAt":"2026-07-01T00:00:00Z","updatedAt":"2026-08-20T00:00:00Z"}],`+
		`"incomplete":true,"pageCap":5}],"partial":[]}`, `{}`)
	got, err := defaultBFPRSource.MergedPRs(bfRepo, time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("an incomplete walk that already passed the window errored: %v", err)
	}
	if len(got) != 1 || got[0].Number != 30 {
		t.Fatalf("merged PRs = %+v, want only #30", got)
	}
}

// regression: an empty merged list read through a verb that would not run must not read as "no PRs merged"
func TestBFMergedVerbFails(t *testing.T) {
	bin := t.TempDir()
	writeStubBin(t, bin, "gh", ghFails)
	writeStubBin(t, bin, "deskread", "#!/bin/sh\necho 'deskread: stubbed to fail' >&2\nexit 1\n")
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	if got, err := defaultBFPRSource.MergedPRs(bfRepo, time.Time{}); err == nil {
		t.Fatalf("a failing deskread read as %+v and no error", got)
	}
}

// regression: the reviews read shelled `gh api pulls/N/reviews`, so it failed wherever gh was absent or unauthenticated
func TestBFReviewsViaDeskread(t *testing.T) {
	argsFile := stubBFVerbs(t, `{}`, `{"schema":1,"kind":"reviews","items":[{"repo":"`+bfRepo+`","number":30,"reviews":[`+
		`{"authorLogin":"reviewer-app[bot]","state":"CHANGES_REQUESTED","commitId":"a1","submittedAt":"2026-09-02T00:00:00Z"},`+
		`{"authorLogin":"reviewer-app[bot]","state":"APPROVED","commitId":"b2","submittedAt":"2026-09-03T00:00:00Z"}]}],"partial":[]}`)

	got, err := defaultBFPRSource.Reviews(bfRepo, 30)
	if err != nil {
		t.Fatalf("defaultBFPRSource.Reviews errored with a working deskread and no working gh: %v", err)
	}
	if len(got) != 2 || got[0].Author.Login != "reviewer-app[bot]" || got[0].State != "CHANGES_REQUESTED" || got[1].State != "APPROVED" {
		t.Fatalf("reviews = %+v, want both submissions, oldest first — the un-laundered array", got)
	}
	if want := "reviews --change " + bfRepo + "#30"; lastStubCall(t, argsFile) != want {
		t.Errorf("deskread argv = %q, want %q", lastStubCall(t, argsFile), want)
	}
}

// regression: a review list the verb could not serve must be could-not-check, never an empty list that scores a PR as never reworked
func TestBFReviewsUnserved(t *testing.T) {
	for _, tc := range []struct{ name, body string }{
		{"partial", `{"schema":1,"kind":"reviews","items":[],"partial":[{"repo":"` + bfRepo + `","number":30,"reason":"cannot read it"}]}`},
		{"no review list", `{"schema":1,"kind":"reviews","items":[{"repo":"` + bfRepo + `","number":30}],"partial":[]}`},
		{"item absent", `{"schema":1,"kind":"reviews","items":[],"partial":[]}`},
		{"no envelope", `not json`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stubBFVerbs(t, `{}`, tc.body)
			if got, err := defaultBFPRSource.Reviews(bfRepo, 30); err == nil {
				t.Fatalf("defaultBFPRSource.Reviews returned %+v and no error for an unserved review list", got)
			}
		})
	}
}

// An empty review list is a real answer — nobody has reviewed — and stays distinct from a failure.
func TestBFReviewsEmptyList(t *testing.T) {
	stubBFVerbs(t, `{}`, `{"schema":1,"kind":"reviews","items":[{"repo":"`+bfRepo+`","number":30,"reviews":[]}],"partial":[]}`)
	got, err := defaultBFPRSource.Reviews(bfRepo, 30)
	if err != nil || len(got) != 0 {
		t.Fatalf("reviews = %+v, err = %v, want an empty list and no error", got, err)
	}
}
