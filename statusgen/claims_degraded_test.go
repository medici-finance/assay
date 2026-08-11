package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The fail-open this file pins: a `git ls-remote` failure
// used to drop claim filtering silently, so the board reported MORE eligible
// briefs than were genuinely unclaimed with nothing to distinguish it from a
// clean run. Every test here forces the failure path and observes the
// behaviour — a fix to a fail-open that is never seen failing is not evidence.

// stubRemoteBranches substitutes the remote lister for one test.
func stubRemoteBranches(t *testing.T, branches []string, err error) {
	t.Helper()
	prev := listRemoteBranches
	listRemoteBranches = func(string) ([]string, error) { return branches, err }
	t.Cleanup(func() { listRemoteBranches = prev })
}

// goodRepoRoot copies the goodrepo fixture (stream `alpha`, brief 02 todo) into
// a temp root.
func goodRepoRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS("testdata/goodrepo")); err != nil {
		t.Fatal(err)
	}
	return root
}

func readStatus(t *testing.T, root string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, "STATUS.md"))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// nextUpSection returns STATUS.md's "## Next up" section.
func nextUpSection(t *testing.T, status string) string {
	t.Helper()
	_, after, ok := strings.Cut(status, "## Next up")
	if !ok {
		t.Fatalf("STATUS.md has no Next up section:\n%s", status)
	}
	if before, _, ok := strings.Cut(after, "\n## "); ok {
		return before
	}
	return after
}

// TestClaimReadFailureIsReportedNotSwallowed is the core regression test. With
// the remote read failing, the run must still produce a board (single-writer
// STATUS.md), but that board must SAY it is unfiltered — on stderr for the
// operator and inside the artifact for every later reader.
func TestClaimReadFailureIsReportedNotSwallowed(t *testing.T) {
	root := goodRepoRoot(t)
	stubRemoteBranches(t, nil, errors.New("`git ls-remote --heads origin` timed out after 10s (raise with STATUSGEN_REMOTE_TIMEOUT)"))

	var code int
	stderr := captureStderr(t, func() { code = run(root, "write", nil, nil, "") })
	if code != 0 {
		t.Fatalf("write run exited %d, want 0 — the board must still render (default is loud-degraded, not fail-closed)", code)
	}

	if !strings.Contains(stderr, "claim filtering UNAVAILABLE") {
		t.Errorf("stderr does not announce the degradation; got:\n%s", stderr)
	}
	if !strings.Contains(stderr, "SUPERSET") {
		t.Errorf("stderr does not say the eligible list is a superset; got:\n%s", stderr)
	}
	if !strings.Contains(stderr, "timed out after 10s") {
		t.Errorf("stderr does not name the cause of the failure; got:\n%s", stderr)
	}

	sec := nextUpSection(t, readStatus(t, root))
	if !strings.Contains(sec, "DEGRADED — claim filtering did not run") {
		t.Errorf("STATUS.md Next-up carries no degraded banner; got:\n%s", sec)
	}
	if !strings.Contains(sec, "unfiltered superset") {
		t.Errorf("degraded banner does not tell the reader the rows are a superset; got:\n%s", sec)
	}
	if !strings.Contains(sec, "timed out after 10s") {
		t.Errorf("degraded banner does not name the cause; got:\n%s", sec)
	}
}

// TestClaimFilteringIsSilentWhenItActuallyRan pins the other side: a successful
// read must NOT print the degraded banner, or the alarm becomes noise and gets
// ignored (the ISA-18.2 failure mode).
func TestClaimFilteringIsSilentWhenItActuallyRan(t *testing.T) {
	root := goodRepoRoot(t)
	stubRemoteBranches(t, []string{"main"}, nil)

	var code int
	stderr := captureStderr(t, func() { code = run(root, "write", nil, nil, "") })
	if code != 0 {
		t.Fatalf("write run exited %d, want 0", code)
	}
	if strings.Contains(stderr, "claim filtering UNAVAILABLE") {
		t.Errorf("clean run printed a degraded NOTICE; got:\n%s", stderr)
	}
	sec := nextUpSection(t, readStatus(t, root))
	if strings.Contains(sec, "DEGRADED") {
		t.Errorf("clean run wrote a degraded banner into STATUS.md; got:\n%s", sec)
	}
	if !strings.Contains(sec, "| alpha | 02") {
		t.Errorf("unclaimed brief alpha/02 missing from Next up; got:\n%s", sec)
	}
}

// TestDegradedBoardIsAGenuineSuperset demonstrates the harm the issue measured
// (19 eligible filtered vs 22 unfiltered): the SAME sources produce a Next-up
// row when the remote read fails that they correctly suppress when it succeeds.
// It is that difference — invisible before this fix — the banner now labels.
func TestDegradedBoardIsAGenuineSuperset(t *testing.T) {
	filteredRoot := goodRepoRoot(t)
	stubRemoteBranches(t, []string{"main", "feat/alpha-02-in-flight"}, nil)
	if code := run(filteredRoot, "write", nil, nil, ""); code != 0 {
		t.Fatalf("write run exited %d, want 0", code)
	}
	filtered := nextUpSection(t, readStatus(t, filteredRoot))
	if strings.Contains(filtered, "| alpha | 02") {
		t.Fatalf("sanity: claimed brief alpha/02 should be filtered out when the claim read works; got:\n%s", filtered)
	}

	degradedRoot := goodRepoRoot(t)
	stubRemoteBranches(t, nil, errors.New("`git ls-remote --heads origin` timed out after 10s"))
	if code := run(degradedRoot, "write", nil, nil, ""); code != 0 {
		t.Fatalf("write run exited %d, want 0", code)
	}
	degraded := nextUpSection(t, readStatus(t, degradedRoot))
	if !strings.Contains(degraded, "| alpha | 02") {
		t.Fatalf("expected the degraded board to be a superset containing alpha/02; got:\n%s", degraded)
	}
	if !strings.Contains(degraded, "DEGRADED") {
		t.Fatalf("the superset board must be labelled as such; got:\n%s", degraded)
	}
}

// TestRequireClaimsFailsClosed covers the opt-in fail-closed mode: a caller
// that dispatches work from the board can refuse an unfiltered one outright.
// Nothing is written, so no degraded artifact is left behind either.
func TestRequireClaimsFailsClosed(t *testing.T) {
	root := goodRepoRoot(t)
	stubRemoteBranches(t, nil, errors.New("`git ls-remote --heads origin` failed: fatal: 'origin' does not appear to be a git repository"))
	requireClaims = true
	t.Cleanup(func() { requireClaims = false })

	var code int
	stderr := captureStderr(t, func() { code = run(root, "write", nil, nil, "") })
	if code != 1 {
		t.Fatalf("--require-claims run exited %d, want 1", code)
	}
	if !strings.Contains(stderr, "PROBLEM:") || !strings.Contains(stderr, "--require-claims") {
		t.Errorf("fail-closed run did not report the reason as a PROBLEM; got:\n%s", stderr)
	}
	if _, err := os.Stat(filepath.Join(root, "STATUS.md")); !os.IsNotExist(err) {
		t.Errorf("--require-claims wrote a board anyway (stat err = %v)", err)
	}
}

// TestRequireClaimsPassesWhenClaimsAreKnown: the fail-closed flag must not
// change any behaviour on a healthy run.
func TestRequireClaimsPassesWhenClaimsAreKnown(t *testing.T) {
	root := goodRepoRoot(t)
	stubRemoteBranches(t, []string{"main"}, nil)
	requireClaims = true
	t.Cleanup(func() { requireClaims = false })
	if code := run(root, "write", nil, nil, ""); code != 0 {
		t.Fatalf("--require-claims run with a working remote exited %d, want 0", code)
	}
}

// TestResolveClaimsNeverConfusesFailureWithNoClaims pins the type-level
// property the whole fix rests on: "read the remote, found no claims" and
// "could not read the remote" produce the same empty map, so they MUST be
// distinguishable by the ClaimSource beside it.
func TestResolveClaimsNeverConfusesFailureWithNoClaims(t *testing.T) {
	streams := []*Stream{mkStream("alpha", "active", "P0", Brief{Num: "02", Title: "Second", Status: "todo"})}

	stubRemoteBranches(t, nil, errors.New("boom"))
	claimed, src := resolveClaims(t.TempDir(), streams)
	if len(claimed) != 0 {
		t.Errorf("failed read produced claims %v, want none", claimed)
	}
	if src.Known {
		t.Error("failed read reported Known=true — this is the fail-open")
	}
	if src.Reason != "boom" {
		t.Errorf("ClaimSource.Reason = %q, want the underlying cause", src.Reason)
	}

	stubRemoteBranches(t, []string{"main"}, nil)
	claimed, src = resolveClaims(t.TempDir(), streams)
	if len(claimed) != 0 {
		t.Errorf("clean read of an unclaimed repo produced claims %v, want none", claimed)
	}
	if !src.Known {
		t.Error("clean read reported Known=false")
	}
	if src.Reason != "" {
		t.Errorf("clean read carried a reason %q", src.Reason)
	}
}

// TestClaimSourceZeroValueIsUnknown: a NextUp nobody told about claims is
// unfiltered and must present itself that way. A zero value that meant "known"
// would reintroduce the defect at every future call site.
func TestClaimSourceZeroValueIsUnknown(t *testing.T) {
	var c ClaimSource
	if c.Known {
		t.Fatal("zero-value ClaimSource is Known — the fail-open default")
	}
	if c.Banner() == "" || c.Notice(3) == "" {
		t.Fatal("zero-value ClaimSource renders no degradation notice")
	}
	if !strings.Contains(c.Banner(), "never read") {
		t.Errorf("zero-value banner should name its empty reason; got %q", c.Banner())
	}
	known := ClaimSource{Known: true}
	if known.Banner() != "" || known.Notice(3) != "" {
		t.Errorf("a known ClaimSource must render nothing; got %q / %q", known.Banner(), known.Notice(3))
	}
}

// TestDegradedBoardLabelsTheEligibleCount: the overflow line quotes an
// eligible count, and a reader takes it at face value. Under degradation it
// must carry the unfiltered qualifier.
func TestDegradedBoardLabelsTheEligibleCount(t *testing.T) {
	s := mkStream("alpha", "active", "P0",
		Brief{Num: "01", Title: "One", Status: "todo"},
		Brief{Num: "02", Title: "Two", Status: "todo"},
	)
	nu := nextUp([]*Stream{s}, nil, nil)
	nu.Threshold = 1 // force the overflow line
	nu.Claims = ClaimSource{Reason: "ls-remote timed out"}
	out := emit([]*Stream{s}, nil, nu, nil, IntakeAlarmResult{}, nil, "")
	sec := nextUpSection(t, out)
	if !strings.Contains(sec, "UNFILTERED") {
		t.Errorf("overflow line does not qualify its eligible count under degradation; got:\n%s", sec)
	}

	nu.Claims = ClaimSource{Known: true}
	sec = nextUpSection(t, emit([]*Stream{s}, nil, nu, nil, IntakeAlarmResult{}, nil, ""))
	if strings.Contains(sec, "UNFILTERED") {
		t.Errorf("overflow line qualified a filtered count; got:\n%s", sec)
	}
}
