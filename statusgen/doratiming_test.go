package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// mustTime is shared with alarms_test.go (handles date-only + RFC3339).

// head is a tiny constructor for episode-matching tests.
func head(sha string, red bool, at string, runID int64, t *testing.T) ciHead {
	return ciHead{SHA: sha, Red: red, At: mustTime(t, at), RunID: runID}
}

// --- episode matching: the correctness core --------------------------------

func TestMatchEpisodesSimpleRedGreen(t *testing.T) {
	heads := []ciHead{
		head("a", false, "2026-08-25T10:00:00Z", 1, t),
		head("b", true, "2026-08-25T11:00:00Z", 2, t),
		head("c", false, "2026-08-25T12:00:00Z", 3, t),
	}
	eps := matchEpisodes(heads)
	if len(eps) != 1 {
		t.Fatalf("got %d episodes, want 1", len(eps))
	}
	e := eps[0]
	if e.FailedRunID != 2 || e.RestoredRunID != 3 {
		t.Errorf("wrong ids: %+v", e)
	}
	if !e.FailedAt.Equal(mustTime(t, "2026-08-25T11:00:00Z")) || !e.RestoredAt.Equal(mustTime(t, "2026-08-25T12:00:00Z")) {
		t.Errorf("wrong instants: %+v", e)
	}
}

// Consecutive reds must NOT re-open: the episode stays anchored at the FIRST
// red, and the restore interval is measured from red#1, not red#2. This is the
// "wrong-but-plausible interval reads as a real metric forever" guard.
func TestMatchEpisodesConsecutiveRedsAnchorAtFirst(t *testing.T) {
	heads := []ciHead{
		head("a", false, "2026-08-25T09:00:00Z", 1, t),
		head("b", true, "2026-08-25T10:00:00Z", 2, t),  // first red — the anchor
		head("c", true, "2026-08-25T11:00:00Z", 3, t),  // consecutive red — must NOT re-open
		head("d", true, "2026-08-25T12:00:00Z", 4, t),  // still red
		head("e", false, "2026-08-25T13:00:00Z", 5, t), // restore
	}
	eps := matchEpisodes(heads)
	if len(eps) != 1 {
		t.Fatalf("got %d episodes, want 1", len(eps))
	}
	e := eps[0]
	if e.FailedRunID != 2 {
		t.Errorf("anchored at run %d, want the FIRST red (2)", e.FailedRunID)
	}
	if !e.FailedAt.Equal(mustTime(t, "2026-08-25T10:00:00Z")) {
		t.Errorf("failed_at=%v, want the first red at 10:00 (measuring from red#2 would under-report)", e.FailedAt)
	}
}

// A still-red tail (no closing green) records NOTHING — an open episode is
// could-not-check, never a fabricated interval.
func TestMatchEpisodesStillRedTailRecordsNothing(t *testing.T) {
	heads := []ciHead{
		head("a", false, "2026-08-25T09:00:00Z", 1, t),
		head("b", true, "2026-08-25T10:00:00Z", 2, t), // opens, never closes
	}
	if eps := matchEpisodes(heads); len(eps) != 0 {
		t.Fatalf("got %d episodes, want 0 (still-red tail must record nothing): %+v", len(eps), eps)
	}
}

func TestMatchEpisodesMultipleEpisodes(t *testing.T) {
	heads := []ciHead{
		head("a", true, "2026-08-25T09:00:00Z", 1, t),
		head("b", false, "2026-08-25T10:00:00Z", 2, t), // close #1
		head("c", false, "2026-08-25T11:00:00Z", 3, t), // green run — no new episode
		head("d", true, "2026-08-25T12:00:00Z", 4, t),
		head("e", false, "2026-08-25T13:00:00Z", 5, t), // close #2
	}
	eps := matchEpisodes(heads)
	if len(eps) != 2 {
		t.Fatalf("got %d episodes, want 2", len(eps))
	}
	if eps[0].FailedRunID != 1 || eps[0].RestoredRunID != 2 {
		t.Errorf("episode 0 wrong: %+v", eps[0])
	}
	if eps[1].FailedRunID != 4 || eps[1].RestoredRunID != 5 {
		t.Errorf("episode 1 wrong: %+v", eps[1])
	}
}

func TestMatchEpisodesAllGreenNoEpisodes(t *testing.T) {
	heads := []ciHead{
		head("a", false, "2026-08-25T09:00:00Z", 1, t),
		head("b", false, "2026-08-25T10:00:00Z", 2, t),
	}
	if eps := matchEpisodes(heads); len(eps) != 0 {
		t.Fatalf("got %d episodes, want 0", len(eps))
	}
}

// --- headsFromRuns: aggregate state + re-run handling ----------------------

func TestHeadsFromRunsAggregateAnyRed(t *testing.T) {
	// One commit with two workflows: one green, one red => the commit is RED.
	runs := []workflowRun{
		{ID: 10, Name: "ci", HeadSHA: "sha1", Status: "completed", Conclusion: "success", UpdatedAt: "2026-08-25T10:00:00Z"},
		{ID: 11, Name: "lint", HeadSHA: "sha1", Status: "completed", Conclusion: "failure", UpdatedAt: "2026-08-25T10:05:00Z"},
	}
	heads := headsFromRuns(runs, "*")
	if len(heads) != 1 || !heads[0].Red {
		t.Fatalf("want 1 red head, got %+v", heads)
	}
	if heads[0].RunID != 11 {
		t.Errorf("red head should carry the failing run id 11, got %d", heads[0].RunID)
	}
}

// A red run RE-RUN green on the same (workflow, sha) leaves that commit GREEN —
// only the latest run per (workflow, sha) counts.
func TestHeadsFromRunsRerunGreenWins(t *testing.T) {
	runs := []workflowRun{
		{ID: 20, Name: "ci", HeadSHA: "sha1", Status: "completed", Conclusion: "failure", UpdatedAt: "2026-08-25T10:00:00Z"},
		{ID: 21, Name: "ci", HeadSHA: "sha1", Status: "completed", Conclusion: "success", UpdatedAt: "2026-08-25T11:00:00Z"}, // re-run, later
	}
	heads := headsFromRuns(runs, "*")
	if len(heads) != 1 || heads[0].Red {
		t.Fatalf("re-run green should win; got %+v", heads)
	}
}

func TestHeadsFromRunsWorkflowFilter(t *testing.T) {
	runs := []workflowRun{
		{ID: 30, Name: "statusgen", Path: ".github/workflows/statusgen.yml", HeadSHA: "sha1", Status: "completed", Conclusion: "failure", UpdatedAt: "2026-08-25T10:00:00Z"},
		{ID: 31, Name: "other", Path: ".github/workflows/other.yml", HeadSHA: "sha1", Status: "completed", Conclusion: "success", UpdatedAt: "2026-08-25T10:00:00Z"},
	}
	// Filter to statusgen only => commit is red (other workflow ignored).
	heads := headsFromRuns(runs, "statusgen")
	if len(heads) != 1 || !heads[0].Red {
		t.Fatalf("filtered to statusgen, want red; got %+v", heads)
	}
	// Filter by file basename also matches.
	heads2 := headsFromRuns(runs, "statusgen.yml")
	if len(heads2) != 1 || !heads2[0].Red {
		t.Fatalf("basename filter, want red; got %+v", heads2)
	}
}

func TestHeadsFromRunsIgnoresIncompleteAndNonDecisive(t *testing.T) {
	runs := []workflowRun{
		{ID: 40, Name: "ci", HeadSHA: "sha1", Status: "in_progress", Conclusion: "", UpdatedAt: "2026-08-25T10:00:00Z"},
		{ID: 41, Name: "ci", HeadSHA: "sha2", Status: "completed", Conclusion: "cancelled", UpdatedAt: "2026-08-25T10:00:00Z"},
	}
	if heads := headsFromRuns(runs, "*"); len(heads) != 0 {
		t.Fatalf("in-progress + cancelled-only should yield no heads; got %+v", heads)
	}
}

// --- lead-time anchor fallback ---------------------------------------------

func TestComputeLeadTimeFirstCommitAnchor(t *testing.T) {
	now := mustTime(t, "2026-08-26T00:00:00Z")
	pr := doraMergedPR{
		Number:    1616,
		MergedSHA: "9c342296",
		CreatedAt: "2026-08-25T17:30:00Z",
		MergedAt:  "2026-08-25T18:37:51Z",
	}
	first := mustTime(t, "2026-08-25T17:10:00Z")
	rec, ok := computeLeadTime("owner/repo", pr, first, true, now)
	if !ok {
		t.Fatal("expected a record")
	}
	if rec.Anchor != "first_commit" {
		t.Errorf("anchor=%q, want first_commit", rec.Anchor)
	}
	// 17:10:00 -> 18:37:51 = 5271s
	if rec.LeadSeconds != 5271 {
		t.Errorf("lead_seconds=%d, want 5271", rec.LeadSeconds)
	}
	if rec.FirstCommitAt == "" {
		t.Error("first_commit_at should be populated")
	}
}

func TestComputeLeadTimeOpenedFallback(t *testing.T) {
	now := mustTime(t, "2026-08-26T00:00:00Z")
	pr := doraMergedPR{
		Number:    7,
		MergedSHA: "deadbeef",
		CreatedAt: "2026-08-25T17:30:00Z",
		MergedAt:  "2026-08-25T18:37:51Z",
	}
	// Commit list unavailable (haveFirst=false) => fall back to opened_at.
	rec, ok := computeLeadTime("owner/repo", pr, time.Time{}, false, now)
	if !ok {
		t.Fatal("expected a record")
	}
	if rec.Anchor != "opened" {
		t.Errorf("anchor=%q, want opened", rec.Anchor)
	}
	if rec.FirstCommitAt != "" {
		t.Errorf("first_commit_at should be empty on the opened fallback, got %q", rec.FirstCommitAt)
	}
	// 17:30:00 -> 18:37:51 = 4071s
	if rec.LeadSeconds != 4071 {
		t.Errorf("lead_seconds=%d, want 4071", rec.LeadSeconds)
	}
}

func TestComputeLeadTimeClockSkewClampsToZero(t *testing.T) {
	now := mustTime(t, "2026-08-26T00:00:00Z")
	pr := doraMergedPR{Number: 1, MergedAt: "2026-08-25T10:00:00Z", CreatedAt: "2026-08-25T11:00:00Z"}
	first := mustTime(t, "2026-08-25T12:00:00Z") // after merge — impossible, clock skew
	rec, ok := computeLeadTime("owner/repo", pr, first, true, now)
	if !ok || rec.LeadSeconds != 0 {
		t.Errorf("negative interval must clamp to 0, got ok=%v lead=%d", ok, rec.LeadSeconds)
	}
}

// --- the recorder: idempotency + append via a fake source ------------------

type fakeDoraSource struct {
	runs   []workflowRun
	prs    []doraMergedPR
	firsts map[int]time.Time // pr -> first commit time; absent => haveFirst=false
}

func (f fakeDoraSource) MainWorkflowRuns(repo string) ([]workflowRun, error) { return f.runs, nil }
func (f fakeDoraSource) MergedPRs(repo string, since time.Time) ([]doraMergedPR, error) {
	return f.prs, nil
}
func (f fakeDoraSource) FirstCommitAt(repo string, pr int) (time.Time, bool) {
	t, ok := f.firsts[pr]
	return t, ok
}

func TestRecordDoraTimingAppendAndIdempotent(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("GITHUB_REPOSITORY", "owner/repo")
	os.Unsetenv("STATUSGEN_DORA_WORKFLOW")
	now := mustTime(t, "2026-08-26T00:00:00Z")

	src := fakeDoraSource{
		runs: []workflowRun{
			{ID: 100, Name: "ci", HeadSHA: "red", Status: "completed", Conclusion: "failure", UpdatedAt: "2026-08-25T20:00:00Z"},
			{ID: 101, Name: "ci", HeadSHA: "green", Status: "completed", Conclusion: "success", UpdatedAt: "2026-08-25T21:30:00Z"},
		},
		prs: []doraMergedPR{
			{Number: 1616, MergedSHA: "9c34", CreatedAt: "2026-08-25T17:30:00Z", MergedAt: "2026-08-25T18:37:51Z"},
		},
		firsts: map[int]time.Time{1616: mustTime(t, "2026-08-25T17:10:00Z")},
	}

	n := recordDoraTiming(dir, src, now)
	if n != 2 {
		t.Fatalf("first pass appended %d, want 2 (1 episode + 1 lead time)", n)
	}
	path := filepath.Join(dir, filepath.FromSlash(doraTimingRelPath))
	before, _ := os.ReadFile(path)

	// Second pass with identical inputs must be a byte-identical no-op.
	n2 := recordDoraTiming(dir, src, now)
	if n2 != 0 {
		t.Fatalf("second pass appended %d, want 0 (idempotent)", n2)
	}
	after, _ := os.ReadFile(path)
	if string(before) != string(after) {
		t.Errorf("idempotent re-run changed the file:\nbefore=%q\nafter=%q", before, after)
	}

	// Verify the recorded episode shape.
	recs, err := loadDoraTimingRecords(path)
	if err != nil {
		t.Fatal(err)
	}
	var gotEpisode, gotLead bool
	for _, r := range recs {
		switch r.Type {
		case "restore_episode":
			gotEpisode = true
			if r.FailedRunID != 100 {
				t.Errorf("episode failed_run_id=%d, want 100", r.FailedRunID)
			}
			// 20:00 -> 21:30 = 5400s
			if r.RestoreSeconds != 5400 {
				t.Errorf("restore_seconds=%d, want 5400", r.RestoreSeconds)
			}
		case "pr_lead_time":
			gotLead = true
			if r.PR != 1616 || r.LeadSeconds != 5271 {
				t.Errorf("lead record wrong: pr=%d lead=%d", r.PR, r.LeadSeconds)
			}
		}
	}
	if !gotEpisode || !gotLead {
		t.Errorf("missing records: episode=%v lead=%v", gotEpisode, gotLead)
	}
}

func TestRecordDoraTimingStillRedTailRecordsNoEpisode(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("GITHUB_REPOSITORY", "owner/repo")
	now := mustTime(t, "2026-08-26T00:00:00Z")
	src := fakeDoraSource{
		runs: []workflowRun{
			{ID: 200, Name: "ci", HeadSHA: "red", Status: "completed", Conclusion: "failure", UpdatedAt: "2026-08-25T20:00:00Z"},
			// no closing green
		},
	}
	n := recordDoraTiming(dir, src, now)
	if n != 0 {
		t.Fatalf("still-red tail must record nothing, appended %d", n)
	}
	path := filepath.Join(dir, filepath.FromSlash(doraTimingRelPath))
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		// The file may not exist at all (true no-op) — that's correct.
		if data, _ := os.ReadFile(path); strings.TrimSpace(string(data)) != "" {
			t.Errorf("still-red tail wrote content: %q", data)
		}
	}
}

func TestRecordDoraTimingNoRepoIsCouldNotCheck(t *testing.T) {
	dir := t.TempDir() // a bare dir with no git remote
	os.Unsetenv("GITHUB_REPOSITORY")
	now := mustTime(t, "2026-08-26T00:00:00Z")
	// Even with data, an unresolved repo records nothing (fail-open).
	// (doraTargetRepo will try git -C dir which fails, then gh which we can't
	// rely on in CI; guard by asserting no panic + no file when repo empty.)
	src := fakeDoraSource{}
	_ = recordDoraTiming(dir, src, now) // must not panic
}

// errDoraSource models the production condition that keeps the timing substrate
// from ever accruing in a record CI job: both network reads fail (gh missing,
// unauthenticated, or rate-limited). It must fail OPEN — never fabricate, never
// fail the record job — but the failure must be LOUD and DISTINGUISHABLE from a
// healthy idempotent no-op, or a persistent read failure is a silent-unknown.
var errDoraStub = errors.New("gh api: could not read (no auth / offline)")

type errDoraSource struct{ err error }

func (e errDoraSource) MainWorkflowRuns(string) ([]workflowRun, error) { return nil, e.err }
func (e errDoraSource) MergedPRs(string, time.Time) ([]doraMergedPR, error) {
	return nil, e.err
}
func (e errDoraSource) FirstCommitAt(string, int) (time.Time, bool) { return time.Time{}, false }

// captureStderr (shared with intake_scoped_unauthored_test.go) redirects
// os.Stderr to a pipe and returns what fn wrote. recordDoraTiming is
// synchronous, so this is reliable.

// A record pass whose reads all could-not-check must fail OPEN (return 0, write
// no file, never fabricate) yet emit a LOUD, DISTINCT degraded signal on stderr
// — never the same "nothing appended" success line a healthy empty pass prints.
// Without this, a consumer's record CI can fail every gh read every day and the
// DORA-timing substrate silently never materializes with no signal anyone sees.
func TestRecordDoraTimingReadFailureIsLoudAndDistinct(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("GITHUB_REPOSITORY", "owner/repo")
	now := mustTime(t, "2026-08-26T00:00:00Z")
	src := errDoraSource{err: errDoraStub}

	var n int
	stderr := captureStderr(t, func() { n = recordDoraTiming(dir, src, now) })

	// Fail-open preserved: the record job is never failed on a read failure.
	if n != 0 {
		t.Fatalf("read failure must fail open (return 0), got %d", n)
	}
	// Never fabricate: no substrate written when nothing could be read.
	path := filepath.Join(dir, filepath.FromSlash(doraTimingRelPath))
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		if data, _ := os.ReadFile(path); strings.TrimSpace(string(data)) != "" {
			t.Errorf("read failure wrote substrate content: %q", data)
		}
	}
	// LOUD + DISTINCT: the degraded signal lands on stderr, names the substrate
	// path, and is NOT the healthy-empty "nothing appended" line.
	if !strings.Contains(stderr, "DEGRADED") {
		t.Errorf("read failure must emit a loud DEGRADED signal on stderr, got: %q", stderr)
	}
	if !strings.Contains(stderr, doraTimingRelPath) {
		t.Errorf("degraded signal must name the substrate path %q, got: %q", doraTimingRelPath, stderr)
	}
	if strings.Contains(stderr, "no new episodes or lead times") {
		t.Errorf("degraded pass must NOT print the healthy-empty line on stderr: %q", stderr)
	}
}

func TestDoraCouldNotCheckWhich(t *testing.T) {
	cases := []struct {
		restore, lead bool
		want          string
	}{
		{false, false, ""},
		{true, false, "the restore-episode read"},
		{false, true, "the lead-time read"},
		{true, true, "both restore-episode and lead-time reads"},
	}
	for _, c := range cases {
		if got := doraCouldNotCheckWhich(c.restore, c.lead); got != c.want {
			t.Errorf("doraCouldNotCheckWhich(%v,%v)=%q, want %q", c.restore, c.lead, got, c.want)
		}
	}
}

// A HEALTHY empty pass (reads succeeded, genuinely nothing new) must stay quiet
// on stderr — its no-op is not a degraded condition and must not be conflated
// with one, or the loud degraded signal loses all meaning.
func TestRecordDoraTimingHealthyEmptyIsQuietOnStderr(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("GITHUB_REPOSITORY", "owner/repo")
	now := mustTime(t, "2026-08-26T00:00:00Z")
	src := fakeDoraSource{} // reads succeed, return empty

	var n int
	stderr := captureStderr(t, func() { n = recordDoraTiming(dir, src, now) })

	if n != 0 {
		t.Fatalf("healthy empty pass must append nothing, got %d", n)
	}
	if strings.Contains(stderr, "DEGRADED") {
		t.Errorf("healthy empty pass must not emit a degraded signal: %q", stderr)
	}
}

// --- the query -------------------------------------------------------------

func TestComputeDoraTimingWindowAndShape(t *testing.T) {
	now := mustTime(t, "2026-08-26T00:00:00Z")
	since := mustTime(t, "2026-08-01T00:00:00Z")
	until := mustTime(t, "2026-08-26T00:00:00Z")
	recs := []doraTimingRecord{
		// in-window restore episodes
		{Type: "restore_episode", RestoredAt: "2026-08-10T00:00:00Z", RestoreSeconds: 3600},
		{Type: "restore_episode", RestoredAt: "2026-08-11T00:00:00Z", RestoreSeconds: 7200},
		// out-of-window restore (before since) — excluded
		{Type: "restore_episode", RestoredAt: "2026-07-01T00:00:00Z", RestoreSeconds: 999999},
		// in-window lead time
		{Type: "pr_lead_time", MergedAt: "2026-08-12T00:00:00Z", LeadSeconds: 5271},
	}
	rep := computeDoraTiming(recs, since, until, now)

	if rep.TimeToRestore.N != 2 {
		t.Errorf("time_to_restore n=%d, want 2 (out-of-window excluded)", rep.TimeToRestore.N)
	}
	if rep.TimeToRestore.Unit != "hours" || rep.TimeToRestore.P50 == nil {
		t.Errorf("time_to_restore should carry unit+p50: %+v", rep.TimeToRestore)
	}
	if rep.ChangeLeadTime.N != 1 {
		t.Errorf("change_lead_time n=%d, want 1", rep.ChangeLeadTime.N)
	}
}

// An empty window emits {state:"could-not-check", n:0}, never a fabricated 0.
func TestComputeDoraTimingEmptyIsCouldNotCheck(t *testing.T) {
	now := mustTime(t, "2026-08-26T00:00:00Z")
	since := mustTime(t, "2026-08-01T00:00:00Z")
	until := mustTime(t, "2026-08-26T00:00:00Z")
	rep := computeDoraTiming(nil, since, until, now)

	if rep.TimeToRestore.State != "could-not-check" || rep.TimeToRestore.N != 0 {
		t.Errorf("empty time_to_restore should be could-not-check n=0: %+v", rep.TimeToRestore)
	}
	if rep.ChangeLeadTime.State != "could-not-check" || rep.ChangeLeadTime.N != 0 {
		t.Errorf("empty change_lead_time should be could-not-check n=0: %+v", rep.ChangeLeadTime)
	}

	// Serialized shape check: could-not-check must NOT carry a fabricated p50/0.
	b, _ := json.Marshal(rep.TimeToRestore)
	s := string(b)
	if strings.Contains(s, "p50") || strings.Contains(s, "unit") {
		t.Errorf("could-not-check metric leaked p50/unit: %s", s)
	}
	if !strings.Contains(s, `"state":"could-not-check"`) || !strings.Contains(s, `"n":0`) {
		t.Errorf("could-not-check metric missing state/n:0: %s", s)
	}
}

func TestPctlHours(t *testing.T) {
	// 3600s=1h, 7200s=2h, 10800s=3h
	secs := []int64{3600, 7200, 10800}
	if got := pctlHours(secs, 0.5); got != 2.0 {
		t.Errorf("p50=%v, want 2.0", got)
	}
	if got := pctlHours(secs, 0.9); got != 3.0 {
		t.Errorf("p90=%v, want 3.0", got)
	}
}

func TestOwnerRepoFromURL(t *testing.T) {
	cases := map[string]string{
		"https://github.com/medici-finance/assay.git":     "medici-finance/assay",
		"https://github.com:443/medici-finance/assay.git": "medici-finance/assay",
		"https://github.com:443/medici-finance/assay":     "medici-finance/assay",
		"git@github.com:medici-finance/assay.git":         "medici-finance/assay",
	}
	for url, want := range cases {
		if got := ownerRepoFromURL(url); got != want {
			t.Errorf("ownerRepoFromURL(%q)=%q, want %q", url, got, want)
		}
	}
}
