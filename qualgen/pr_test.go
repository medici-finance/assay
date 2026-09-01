package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"
)

// --- `pr <n>` mode fixture harness: builds real git repositories with a
// genuine two-parent merge commit carrying the standard GitHub
// "Merge pull request #<n> from ..." message, so touchedFilesMergedPR is
// dereferenced against real history/refs, never a mock. Reuses
// requireGit/gitCmd/writeFile (mine_test.go), hotspotFixtureRepo/
// couplingFixtureRepo (check_test.go), and szzGit/commitFile/identifiedFix/
// openRepo (szz_test.go) — all in this package. ---

// mergePRBranch checks out base, merges branch into it --no-ff with the
// exact merge-commit message a non-squash GitHub merge writes for PR
// #prNum, and returns to base. Returns the merge commit's SHA.
func mergePRBranch(t *testing.T, dir string, prNum int, base, branch string) string {
	t.Helper()
	gitCmd(t, dir, "checkout", "-q", base)
	msg := fmt.Sprintf("Merge pull request #%d from fixture/%s\n\nfixture PR #%d", prNum, branch, prNum)
	gitCmd(t, dir, "merge", "--no-ff", "-q", "-m", msg, branch)
	return gitCmd(t, dir, "rev-parse", "HEAD")
}

// prFileOf returns the feed record for path, if present.
func prFileOf(feed PRFeatureFeed, path string) (PRFileFeature, bool) {
	for _, f := range feed.Files {
		if f.Path == path {
			return f, true
		}
	}
	return PRFileFeature{}, false
}

// TestPR_KnownHotspotFeatureValue is Verify row 3 (DEREFERENCING): a fixture
// PR touches both a churned, deeply-nested file (a planted hotspot) and a
// quiet one; the churned file's hotspot_percentile must strictly exceed the
// quiet file's, and both must carry measured (never could-not-measure —
// both files have mined history).
func TestPR_KnownHotspotFeatureValue(t *testing.T) {
	requireGit(t)
	dir := hotspotFixtureRepo(t)
	store := mineFixture(t, dir)

	gitCmd(t, dir, "checkout", "-q", "-b", "fixture/hotspot-pr")
	writeFile(t, dir, "hot.go", "package hot\n// touched again by the PR\n")
	gitCmd(t, dir, "add", "hot.go")
	gitCmd(t, dir, "commit", "-q", "-m", "PR: touch hot.go")
	writeFile(t, dir, "quiet.go", "y\n")
	gitCmd(t, dir, "add", "quiet.go")
	gitCmd(t, dir, "commit", "-q", "-m", "PR: touch quiet.go")
	mergePRBranch(t, dir, 101, "main", "fixture/hotspot-pr")

	repo := openRepo(t, dir)
	paths, err := touchedFilesMergedPR(repo, 101)
	if err != nil {
		t.Fatalf("touchedFilesMergedPR: %v", err)
	}
	if !contains(paths, "hot.go") || !contains(paths, "quiet.go") {
		t.Fatalf("expected the PR's file set to contain hot.go and quiet.go, got %v", paths)
	}

	feed, err := buildPRFeed(store, 101, paths)
	if err != nil {
		t.Fatalf("buildPRFeed: %v", err)
	}
	hot, ok := prFileOf(feed, "hot.go")
	if !ok {
		t.Fatalf("no feed record for hot.go, got %+v", feed.Files)
	}
	quiet, ok := prFileOf(feed, "quiet.go")
	if !ok {
		t.Fatalf("no feed record for quiet.go, got %+v", feed.Files)
	}
	if hot.Measured["hotspot_percentile"] != string(StateMeasured) {
		t.Fatalf("hot.go hotspot_percentile measured state = %q, want %q", hot.Measured["hotspot_percentile"], StateMeasured)
	}
	if quiet.Measured["hotspot_percentile"] != string(StateMeasured) {
		t.Fatalf("quiet.go hotspot_percentile measured state = %q, want %q", quiet.Measured["hotspot_percentile"], StateMeasured)
	}
	if hot.HotspotPercentile == nil || quiet.HotspotPercentile == nil {
		t.Fatalf("expected non-nil hotspot_percentile for both files, got hot=%v quiet=%v", hot.HotspotPercentile, quiet.HotspotPercentile)
	}
	if !(*hot.HotspotPercentile > *quiet.HotspotPercentile) {
		t.Fatalf("hot.go hotspot_percentile %.3f must strictly exceed quiet.go's %.3f", *hot.HotspotPercentile, *quiet.HotspotPercentile)
	}
}

// TestPR_MissingCouplingPartnerFlagged is Verify row 4 (DEREFERENCING): a.go
// and b.go are historically coupled; a fixture PR touches ONLY a.go — a.go's
// coupling_missing list must name b.go.
func TestPR_MissingCouplingPartnerFlagged(t *testing.T) {
	requireGit(t)
	dir := couplingFixtureRepo(t)
	store := mineFixture(t, dir)

	gitCmd(t, dir, "checkout", "-q", "-b", "fixture/coupling-pr")
	writeFile(t, dir, "a.go", "aaaa\n")
	gitCmd(t, dir, "add", "a.go")
	gitCmd(t, dir, "commit", "-q", "-m", "PR: touch a.go only")
	mergePRBranch(t, dir, 102, "main", "fixture/coupling-pr")

	repo := openRepo(t, dir)
	paths, err := touchedFilesMergedPR(repo, 102)
	if err != nil {
		t.Fatalf("touchedFilesMergedPR: %v", err)
	}
	if !contains(paths, "a.go") || contains(paths, "b.go") {
		t.Fatalf("expected the PR's file set to contain ONLY a.go (not b.go), got %v", paths)
	}

	feed, err := buildPRFeed(store, 102, paths)
	if err != nil {
		t.Fatalf("buildPRFeed: %v", err)
	}
	a, ok := prFileOf(feed, "a.go")
	if !ok {
		t.Fatalf("no feed record for a.go, got %+v", feed.Files)
	}
	if !contains(a.CouplingMissing, "b.go") {
		t.Fatalf("a.go coupling_missing must contain b.go, got %v", a.CouplingMissing)
	}
}

// TestPR_DefectDensityCarriesTraceRate is Verify row 5 (DEREFERENCING): a
// planted defect on app.go is traced via the brief-07 B-SZZ corpus and
// written to the store; a fixture PR (the fix itself, merged) touching
// app.go must carry a non-nil defect_density AND a non-nil
// defect_trace_rate beside it — density without its trace-rate fails the
// test (honest-claims discipline).
func TestPR_DefectDensityCarriesTraceRate(t *testing.T) {
	requireGit(t)
	dir := t.TempDir()
	szzGit(t, dir, "2020-01-01T00:00:00Z", "init", "-q", "-b", "main")
	_ = commitFile(t, dir, "2020-01-01T00:00:00Z", "app.go", "package app\nreturn x / 0\ntail\n", "A: introduce divide-by-zero defect")

	gitCmd(t, dir, "checkout", "-q", "-b", "fixture/fix-app")
	f := commitFile(t, dir, "2020-01-04T00:00:00Z", "app.go", "package app\nreturn x / y\ntail\n", "fix: guard the divisor (#20)")
	mergePRBranch(t, dir, 103, "main", "fixture/fix-app")

	repo := openRepo(t, dir)
	corpus := TraceDefects(repo, []DefectFix{identifiedFix(f, 20, 20)}, nil)

	out := t.TempDir()
	if err := mine(dir, out, &bytes.Buffer{}); err != nil {
		t.Fatalf("mine: %v", err)
	}
	store := NewStore(out)
	if err := corpus.WriteTo(store, 4, time.Now().UTC()); err != nil {
		t.Fatalf("write corpus: %v", err)
	}

	paths, err := touchedFilesMergedPR(repo, 103)
	if err != nil {
		t.Fatalf("touchedFilesMergedPR: %v", err)
	}
	if !contains(paths, "app.go") {
		t.Fatalf("expected the PR's file set to contain app.go, got %v", paths)
	}

	feed, err := buildPRFeed(store, 103, paths)
	if err != nil {
		t.Fatalf("buildPRFeed: %v", err)
	}
	app, ok := prFileOf(feed, "app.go")
	if !ok {
		t.Fatalf("no feed record for app.go, got %+v", feed.Files)
	}
	if app.DefectDensity == nil {
		t.Fatalf("app.go defect_density must be non-nil (a traced defect is planted on it), measured=%v", app.Measured)
	}
	if app.DefectTraceRate == nil {
		t.Fatalf("app.go defect_density is present but defect_trace_rate is nil — density must never travel without its trace-rate (honest-claims)")
	}
}

// TestPR_NewFileIsCouldNotMeasure is Verify row 6 (three-state): a fixture PR
// adds a brand-new file with no prior mined history — its
// measured.hotspot_percentile must be could-not-measure, and
// hotspot_percentile itself must stay nil, never a fabricated 0 percentile.
func TestPR_NewFileIsCouldNotMeasure(t *testing.T) {
	requireGit(t)
	dir := t.TempDir()
	gitCmd(t, dir, "init", "-q", "-b", "main")
	writeFile(t, dir, "existing.go", "package x\n")
	gitCmd(t, dir, "add", "existing.go")
	gitCmd(t, dir, "commit", "-q", "-m", "add existing.go")
	store := mineFixture(t, dir)

	gitCmd(t, dir, "checkout", "-q", "-b", "fixture/newfile-pr")
	writeFile(t, dir, "brandnew.go", "package x\nfunc New() {}\n")
	gitCmd(t, dir, "add", "brandnew.go")
	gitCmd(t, dir, "commit", "-q", "-m", "PR: add brand-new file")
	mergePRBranch(t, dir, 104, "main", "fixture/newfile-pr")

	repo := openRepo(t, dir)
	paths, err := touchedFilesMergedPR(repo, 104)
	if err != nil {
		t.Fatalf("touchedFilesMergedPR: %v", err)
	}
	if !contains(paths, "brandnew.go") {
		t.Fatalf("expected the PR's file set to contain brandnew.go, got %v", paths)
	}

	feed, err := buildPRFeed(store, 104, paths)
	if err != nil {
		t.Fatalf("buildPRFeed: %v", err)
	}
	nf, ok := prFileOf(feed, "brandnew.go")
	if !ok {
		t.Fatalf("no feed record for brandnew.go, got %+v", feed.Files)
	}
	if nf.Measured["hotspot_percentile"] != string(StateCouldNotMeasure) {
		t.Fatalf("brandnew.go hotspot_percentile measured state = %q, want %q", nf.Measured["hotspot_percentile"], StateCouldNotMeasure)
	}
	if nf.HotspotPercentile != nil {
		t.Fatalf("brandnew.go hotspot_percentile must be nil (could-not-measure), got a fabricated value %v", *nf.HotspotPercentile)
	}
}

// TestPR_NoThresholdOrVerdictLeaked is Verify row 7: this mode emits
// features only — no weighting, no combined outcome, no gating decision.
// Grepped structurally here too (not only via the shell Verify row) so a
// regression fails `go test` directly.
func TestPR_NoThresholdOrVerdictLeaked(t *testing.T) {
	feed := PRFeatureFeed{PR: 1, Files: []PRFileFeature{{Path: "x.go", Measured: map[string]string{}}}}
	rawBytes, err := json.MarshalIndent(feed, "", "  ")
	if err != nil {
		t.Fatalf("marshal feed: %v", err)
	}
	raw := string(rawBytes)
	lower := strings.ToLower(raw)
	for _, bad := range []string{"weight", "threshold"} {
		if strings.Contains(lower, bad) {
			t.Fatalf("feed JSON must never carry a %q field/value, got:\n%s", bad, raw)
		}
	}
}

// TestPR_CLIEmitsJSONFeed exercises runPR end-to-end (the mode entry point,
// not just the lower-level helpers exercised above): a fixture PR emits a
// well-formed JSON feed on stdout and exits 0.
func TestPR_CLIEmitsJSONFeed(t *testing.T) {
	requireGit(t)
	dir := hotspotFixtureRepo(t)
	store := mineFixture(t, dir)

	gitCmd(t, dir, "checkout", "-q", "-b", "fixture/cli-pr")
	writeFile(t, dir, "hot.go", "package hot\n// touched again\n")
	gitCmd(t, dir, "add", "hot.go")
	gitCmd(t, dir, "commit", "-q", "-m", "PR: touch hot.go")
	mergePRBranch(t, dir, 105, "main", "fixture/cli-pr")

	var stdout, stderr bytes.Buffer
	rc := runPR([]string{"105", "--out", store.root, "--repo", dir}, &stdout, &stderr)
	if rc != 0 {
		t.Fatalf("runPR returned %d, want 0; stderr=%s", rc, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"pr": 105`) {
		t.Fatalf("stdout must carry the PR number, got:\n%s", stdout.String())
	}
	if !strings.Contains(stdout.String(), `"hot.go"`) {
		t.Fatalf("stdout must carry hot.go's feature record, got:\n%s", stdout.String())
	}
}
