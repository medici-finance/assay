package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	git "github.com/go-git/go-git/v5"
)

// --- B-SZZ fixture harness: stands up real repositories with controllable commit
// DATES and messages, so the trace engine is dereferenced against genuine git
// history and genuine blame — never a mock. Reuses requireGit / writeFile from
// mine_test.go (same package). ---

// szzGit runs git in dir with a fixed author+committer date (RFC3339), so a
// fixture can place a commit at a precise point on the timeline — the postdating
// refinement depends on the ordering being real, not ambient.
func szzGit(t *testing.T, dir, date string, args ...string) string {
	t.Helper()
	full := append([]string{
		"-c", "user.name=Fixture Author",
		"-c", "user.email=fixture@example.test",
		"-c", "commit.gpgsign=false",
		"-C", dir,
	}, args...)
	cmd := exec.Command("git", full...)
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_DATE="+date,
		"GIT_COMMITTER_DATE="+date,
	)
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	if err := cmd.Run(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, errb.String())
	}
	return strings.TrimSpace(out.String())
}

// commitFile writes name=content, stages it, commits at date with msg, and returns
// the new commit SHA.
func commitFile(t *testing.T, dir, date, name, content, msg string) string {
	t.Helper()
	writeFile(t, dir, name, content)
	szzGit(t, dir, date, "add", name)
	szzGit(t, dir, date, "commit", "-q", "-m", msg)
	return szzGit(t, dir, date, "rev-parse", "HEAD")
}

// identifiedFix builds an IDENTIFIED tier-1 DefectFix for a fix commit — the exact
// record quality/06 emits and this engine consumes.
func identifiedFix(fixSHA string, pr, issue int) DefectFix {
	f := DefectFix{
		FixCommitSHA: fixSHA,
		FixPRNumber:  pr,
		Tier:         Tier1,
		Identified:   Measured(true),
	}
	if issue != 0 {
		f.ClosedIssue = &IssueRef{Number: issue}
	}
	return f
}

// fixedReportDate is a test ReportDateResolver: a map from fix commit SHA to the
// defect-report date. A fix absent from the map has no resolvable report date.
type fixedReportDate map[string]time.Time

func (f fixedReportDate) ReportDate(fix DefectFix) (time.Time, bool) {
	d, ok := f[fix.FixCommitSHA]
	return d, ok
}

func openRepo(t *testing.T, dir string) *git.Repository {
	t.Helper()
	r, err := git.PlainOpen(dir)
	if err != nil {
		t.Fatalf("open repo %s: %v", dir, err)
	}
	return r
}

func traceOf(traces []DefectTrace, fixSHA string) (DefectTrace, bool) {
	for _, tr := range traces {
		if tr.FixCommit == fixSHA {
			return tr, true
		}
	}
	return DefectTrace{}, false
}

func contains(ss []string, want string) bool {
	for _, s := range ss {
		if s == want {
			return true
		}
	}
	return false
}

// TestSZZ_PlantedBugTracedToInducingCommit is Verify row 3 (DEREFERENCING): a
// planted bug is traced end-to-end to the commit that introduced it. Commit A
// introduces a defect on a line; B/C are unrelated; F rewrites A's defective line
// as a `fix:` closing a bug issue. The emitted trace must name A — and only A — and
// resolve `traced`.
func TestSZZ_PlantedBugTracedToInducingCommit(t *testing.T) {
	requireGit(t)
	dir := t.TempDir()
	szzGit(t, dir, "2020-01-01T00:00:00Z", "init", "-q", "-b", "main")

	a := commitFile(t, dir, "2020-01-01T00:00:00Z", "app.go", "package app\nreturn x / 0\ntail\n", "A: introduce divide-by-zero defect")
	b := commitFile(t, dir, "2020-01-02T00:00:00Z", "other1.txt", "unrelated one\n", "B: unrelated change")
	c := commitFile(t, dir, "2020-01-03T00:00:00Z", "other2.txt", "unrelated two\n", "C: another unrelated change")
	f := commitFile(t, dir, "2020-01-04T00:00:00Z", "app.go", "package app\nreturn x / y\ntail\n", "fix: guard the divisor (#7)")

	repo := openRepo(t, dir)
	corpus := TraceDefects(repo, []DefectFix{identifiedFix(f, 7, 7)}, nil)

	tr, ok := traceOf(corpus.Traces, f)
	if !ok {
		t.Fatalf("no trace emitted for fix %s", f)
	}
	if tr.TraceState != TraceTraced {
		t.Fatalf("expected traced, got %q (reason %q, conf %+v)", tr.TraceState, tr.CouldNotTraceReason, tr.Confidence)
	}
	if !contains(tr.InducingCommits, a) {
		t.Fatalf("inducing_commits %v must contain the inducing commit A=%s", tr.InducingCommits, a)
	}
	if contains(tr.InducingCommits, b) || contains(tr.InducingCommits, c) {
		t.Fatalf("inducing_commits %v must NOT contain the unrelated B=%s / C=%s", tr.InducingCommits, b, c)
	}
	if tr.EvidenceTier != Tier1 {
		t.Fatalf("evidence tier must carry through from the DefectFix (tier-1), got %q", tr.EvidenceTier)
	}
	if tr.Confidence.State != StateMeasured {
		t.Fatalf("a single-inducer trace must carry a measured confidence, got %+v", tr.Confidence)
	}
}

// TestSZZ_ExcludesInducerPostdatingReport is Verify row 4 (DEREFERENCING): the
// blamed line was last touched by a commit dated AFTER the defect-report date, so
// that candidate cannot have induced the bug. It must be filtered and the trace
// must resolve `traced-none` — NOT `traced` on the postdating commit.
func TestSZZ_ExcludesInducerPostdatingReport(t *testing.T) {
	requireGit(t)
	dir := t.TempDir()
	szzGit(t, dir, "2020-01-01T00:00:00Z", "init", "-q", "-b", "main")

	_ = commitFile(t, dir, "2020-01-01T00:00:00Z", "svc.go", "package svc\nold logic here\ntail\n", "A: original logic")
	// The last touch of the blamed line happens AFTER the bug was reported.
	postdate := commitFile(t, dir, "2020-03-01T00:00:00Z", "svc.go", "package svc\nchanged logic here\ntail\n", "C: rewrite the logic line")
	f := commitFile(t, dir, "2020-06-01T00:00:00Z", "svc.go", "package svc\nfixed logic here\ntail\n", "fix: correct the logic (#8)")

	repo := openRepo(t, dir)
	// Bug reported 2020-02-01 — BEFORE the postdating commit (2020-03-01).
	report := fixedReportDate{f: mustTime(t, "2020-02-01T00:00:00Z")}
	corpus := TraceDefects(repo, []DefectFix{identifiedFix(f, 8, 8)}, report)

	tr, ok := traceOf(corpus.Traces, f)
	if !ok {
		t.Fatalf("no trace emitted for fix %s", f)
	}
	if tr.TraceState != TraceTracedNone {
		t.Fatalf("expected traced-none (postdating candidate filtered), got %q with inducers %v", tr.TraceState, tr.InducingCommits)
	}
	if contains(tr.InducingCommits, postdate) {
		t.Fatalf("the postdating commit %s must be filtered, not recorded as an inducer: %v", postdate, tr.InducingCommits)
	}
	if len(tr.InducingCommits) != 0 {
		t.Fatalf("traced-none must carry no inducers, got %v", tr.InducingCommits)
	}
	if tr.Confidence.State != StateMeasuredZero {
		t.Fatalf("traced-none confidence must be measured-zero, got %+v", tr.Confidence)
	}
	// A traced-none fix IS a real, measured zero: it belongs in the per-file
	// traceable denominator (contrast with a could-not-trace fix, which does not).
	if len(corpus.perFileFixes) == 0 {
		t.Fatal("a traced-none fix must count in the per-file traceable denominator (perFileFixes non-empty)")
	}
}

// TestSZZ_UnreachableBlameIsCouldNotTrace is Verify row 5 (DEREFERENCING —
// three-state, no silent zero): a fix whose pre-image is unreachable by blame (a
// shallow/squash history floor cut its parent off) must resolve `could-not-trace`
// with reason `squash-floor`, and must be EXCLUDED from the numerator/denominator
// of the defect-inducing rate — never counted as a zero-inducer traced fix.
func TestSZZ_UnreachableBlameIsCouldNotTrace(t *testing.T) {
	requireGit(t)
	base := t.TempDir()
	src := filepath.Join(base, "src")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	szzGit(t, src, "2020-01-01T00:00:00Z", "init", "-q", "-b", "main")
	_ = commitFile(t, src, "2020-01-01T00:00:00Z", "app.go", "package app\nbuggy line\ntail\n", "A: introduce defect")
	_ = commitFile(t, src, "2020-01-02T00:00:00Z", "other.txt", "unrelated\n", "B: unrelated")
	f := commitFile(t, src, "2020-01-03T00:00:00Z", "app.go", "package app\nfixed line\ntail\n", "fix: repair the defect (#9)")

	// A depth-1 clone makes F a shallow boundary: its recorded parent object is
	// absent, so the pre-image the fix modified is unreachable — the history floor a
	// squash merge also produces.
	shallow := filepath.Join(base, "shallow")
	cloneShallow(t, src, shallow)

	repo := openRepo(t, shallow)
	corpus := TraceDefects(repo, []DefectFix{identifiedFix(f, 9, 9)}, nil)

	tr, ok := traceOf(corpus.Traces, f)
	if !ok {
		t.Fatalf("no trace emitted for fix %s", f)
	}
	if tr.TraceState != TraceCouldNotTrace {
		t.Fatalf("expected could-not-trace, got %q", tr.TraceState)
	}
	if tr.CouldNotTraceReason != ReasonSquashFloor {
		t.Fatalf("expected reason squash-floor, got %q", tr.CouldNotTraceReason)
	}
	if tr.Confidence.State != StateCouldNotMeasure || tr.Confidence.Reason == "" {
		t.Fatalf("a could-not-trace confidence must be could-not-measure with a reason, got %+v", tr.Confidence)
	}

	// EXCLUDED from the inducing numerator: it contributes no inducer.
	if got := corpus.DistinctInducingCommits(); got != 0 {
		t.Fatalf("a could-not-trace fix must contribute 0 to the inducing numerator, got %d", got)
	}
	// EXCLUDED from every per-file denominator: it touched nothing for aggregation.
	if len(corpus.perFileFixes) != 0 {
		t.Fatalf("a could-not-trace fix must be excluded from the per-file traceable denominator, got %v", corpus.perFileFixes)
	}
	traced, tracedNone, cnt := corpus.Partition()
	if traced != 0 || tracedNone != 0 || cnt != 1 {
		t.Fatalf("partition must be traced=0 tracedNone=0 couldNotTrace=1, got %d/%d/%d", traced, tracedNone, cnt)
	}
	// It IS in the trace-rate denominator, honestly lowering the rate (0 traced / 1
	// total) — the ~40%-unreachable disclosure, not a silent drop.
	if got := corpus.TraceRate(); got.State != StateMeasuredZero {
		t.Fatalf("trace-rate over one could-not-trace fix must be measured-zero (0/1), got %+v", got)
	}
	// The defect-inducing rate is a genuine measured-zero (0 inducers / 5 PRs), not a
	// could-not-measure error and not an inflated number.
	if got := corpus.DefectInducingRate(5); got.State != StateMeasuredZero {
		t.Fatalf("defect-inducing rate must be measured-zero, got %+v", got)
	}
}

// TestSZZ_DerivedMetricsCarryTraceRate is Verify row 6 (honest-claims): EVERY
// derived-metric record serializes a non-empty trace_rate and tier_composition
// beside the number — a record with a bare rate fails.
func TestSZZ_DerivedMetricsCarryTraceRate(t *testing.T) {
	requireGit(t)
	dir := t.TempDir()
	szzGit(t, dir, "2020-01-01T00:00:00Z", "init", "-q", "-b", "main")
	a := commitFile(t, dir, "2020-01-01T00:00:00Z", "app.go", "package app\nreturn x / 0\ntail\n", "A: introduce defect")
	f := commitFile(t, dir, "2020-01-05T00:00:00Z", "app.go", "package app\nreturn x / y\ntail\n", "fix: guard divisor (#10)")
	_ = a

	repo := openRepo(t, dir)
	corpus := TraceDefects(repo, []DefectFix{identifiedFix(f, 10, 10)}, nil)

	recs := corpus.DerivedMetrics(4, mustTime(t, "2026-08-30T00:00:00Z"))
	if len(recs) == 0 {
		t.Fatal("expected derived-metric records")
	}
	wantMetrics := map[string]bool{
		MetricDefectInducingRate: false,
		MetricDefectDensity:      false,
		MetricTracedCFRInput:     false,
	}
	for _, rec := range recs {
		// Every record's trace_rate is a real three-state Measure, never absent.
		if rec.TraceRate.State == "" {
			t.Fatalf("record %q has an EMPTY trace_rate — a bare rate is a bug (spec §10)", rec.Metric)
		}
		// Serialize and assert the honest-claims fields are physically present.
		raw, err := json.Marshal(rec)
		if err != nil {
			t.Fatalf("marshal %q: %v", rec.Metric, err)
		}
		if !strings.Contains(string(raw), `"trace_rate"`) {
			t.Fatalf("record %q serialized without a trace_rate field: %s", rec.Metric, raw)
		}
		if !strings.Contains(string(raw), `"tier_composition"`) {
			t.Fatalf("record %q serialized without a tier_composition field: %s", rec.Metric, raw)
		}
		if _, tracked := wantMetrics[rec.Metric]; tracked {
			wantMetrics[rec.Metric] = true
		}
	}
	for m, seen := range wantMetrics {
		if !seen {
			t.Fatalf("expected a %q derived-metric record carrying its trace-rate, none emitted", m)
		}
	}
}

// TestSZZ_BlamelessOmissionIsCouldNotTrace covers the omission-bug three-state
// path: a fix that only ADDS lines has no deleted/modified pre-image to blame, so
// it resolves could-not-trace/blameless — never a silent traced-with-nothing.
func TestSZZ_BlamelessOmissionIsCouldNotTrace(t *testing.T) {
	requireGit(t)
	dir := t.TempDir()
	szzGit(t, dir, "2020-01-01T00:00:00Z", "init", "-b", "main", "-q")
	_ = commitFile(t, dir, "2020-01-01T00:00:00Z", "guard.go", "package guard\nline one\n", "A: base")
	// The fix only APPENDS a missing nil-check — a pure addition (omission bug).
	f := commitFile(t, dir, "2020-02-01T00:00:00Z", "guard.go", "package guard\nline one\nif x == nil { return }\n", "fix: add missing nil guard (#11)")

	repo := openRepo(t, dir)
	corpus := TraceDefects(repo, []DefectFix{identifiedFix(f, 11, 11)}, nil)
	tr, _ := traceOf(corpus.Traces, f)
	if tr.TraceState != TraceCouldNotTrace || tr.CouldNotTraceReason != ReasonBlameless {
		t.Fatalf("an addition-only fix must be could-not-trace/blameless, got %q/%q", tr.TraceState, tr.CouldNotTraceReason)
	}
}

// TestSZZ_StoreRoundTrip proves the corpus writes DefectTrace records to
// defects.jsonl and derived metrics to metrics.jsonl, and that the trace rows read
// back through StreamTraces WITHOUT corrupting a DefectFix reader on the same
// heterogeneous table (spec §9.4).
func TestSZZ_StoreRoundTrip(t *testing.T) {
	requireGit(t)
	dir := t.TempDir()
	szzGit(t, dir, "2020-01-01T00:00:00Z", "init", "-b", "main", "-q")
	_ = commitFile(t, dir, "2020-01-01T00:00:00Z", "app.go", "package app\nreturn x / 0\n", "A: defect")
	f := commitFile(t, dir, "2020-01-05T00:00:00Z", "app.go", "package app\nreturn x / y\n", "fix: guard (#12)")

	repo := openRepo(t, dir)
	corpus := TraceDefects(repo, []DefectFix{identifiedFix(f, 12, 12)}, nil)

	root := t.TempDir()
	store := NewStore(root)
	// A prior quality/06 DefectFix row already sits in the SAME defects table.
	if err := store.Append(KindDefect, identifiedFix(f, 12, 12)); err != nil {
		t.Fatalf("seed DefectFix: %v", err)
	}
	if err := corpus.WriteTo(store, 4, time.Now().UTC()); err != nil {
		t.Fatalf("write corpus: %v", err)
	}

	traces, err := store.ReadTraces()
	if err != nil {
		t.Fatalf("read traces: %v", err)
	}
	if len(traces) != 1 || traces[0].FixCommit != f {
		t.Fatalf("expected exactly the one DefectTrace back, got %+v", traces)
	}
	// The DefectFix reader must still see ONLY the seeded fix, not the trace row.
	fixes, err := store.ReadDefects()
	if err != nil {
		t.Fatalf("read defects: %v", err)
	}
	if len(fixes) != 1 || fixes[0].FixCommitSHA != f {
		t.Fatalf("DefectFix reader must return only the DefectFix row, got %+v", fixes)
	}
	// Derived metrics landed on the metrics table.
	metrics, err := store.ReadMetrics()
	if err != nil {
		t.Fatalf("read metrics: %v", err)
	}
	sawTraceRate := false
	for _, m := range metrics {
		if m.Metric == MetricDefectTraceRate {
			sawTraceRate = true
		}
	}
	if !sawTraceRate {
		t.Fatal("expected a defect_trace_rate record on the metrics table")
	}
}

// cloneShallow makes a depth-1 clone of src at dst, so the cloned HEAD is a shallow
// boundary whose parent object is absent.
func cloneShallow(t *testing.T, src, dst string) {
	t.Helper()
	cmd := exec.Command("git", "clone", "-q", "--depth=1", "file://"+src, dst)
	var errb bytes.Buffer
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		t.Fatalf("shallow clone: %v\n%s", err, errb.String())
	}
}

func mustTime(t *testing.T, rfc3339 string) time.Time {
	t.Helper()
	tm, err := time.Parse(time.RFC3339, rfc3339)
	if err != nil {
		t.Fatalf("parse time %q: %v", rfc3339, err)
	}
	return tm.UTC()
}
