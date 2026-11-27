package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// --- fixture helpers: real git repos via requireGit/gitCmd/mine (mine_test.go,
// instructionbrittle_test.go), so `check` is dereferenced against genuine
// mined history and a genuine HEAD tree, not a mock. ---

// nested and flat are the same complexity-proxy shapes hotspot_test.go uses.
var (
	nestedLines = []string{"        x", "        x", "        x"}
	flatLines   = []string{"x"}
)

func writeNested(t *testing.T, dir, name string, n int) {
	t.Helper()
	var b strings.Builder
	for i := 0; i < n; i++ {
		for _, l := range nestedLines {
			b.WriteString(l)
			b.WriteByte('\n')
		}
	}
	writeFile(t, dir, name, b.String())
}

// hotspotFixtureRepo builds a repo with hot.go — touched by several commits
// with deeply nested content — and quiet.go — touched once, flat content —
// so hot.go's decayed hotspot strictly outranks quiet.go's (hotspot_test.go's
// TestHotspotIsProductNotFactor pins the same product-of-both-factors shape).
func hotspotFixtureRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	gitCmd(t, dir, "init", "-q", "-b", "main")

	writeFile(t, dir, "quiet.go", strings.Join(flatLines, "\n")+"\n")
	gitCmd(t, dir, "add", "quiet.go")
	gitCmd(t, dir, "commit", "-q", "-m", "add quiet.go")

	for i := 0; i < 5; i++ {
		writeNested(t, dir, "hot.go", i+1)
		gitCmd(t, dir, "add", "hot.go")
		gitCmd(t, dir, "commit", "-q", "-m", "churn hot.go")
	}
	return dir
}

// couplingFixtureRepo builds a repo where a.go and b.go are co-changed in
// every one of three commits — a historically coupled pair well above
// DefaultCouplingParams' MinRatio/MinCoChanges.
func couplingFixtureRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	gitCmd(t, dir, "init", "-q", "-b", "main")
	for i := 0; i < 3; i++ {
		writeFile(t, dir, "a.go", strings.Repeat("a", i+1)+"\n")
		writeFile(t, dir, "b.go", strings.Repeat("b", i+1)+"\n")
		gitCmd(t, dir, "add", "a.go", "b.go")
		gitCmd(t, dir, "commit", "-q", "-m", "co-change a.go and b.go")
	}
	return dir
}

// findAdvisory reports whether files contains path with an advisory of kind.
func findAdvisory(files []FileScreenResult, path string, kind AdvisoryKind) (Advisory, bool) {
	for _, f := range files {
		if f.Path != path {
			continue
		}
		for _, a := range f.Advisories {
			if a.Kind == kind {
				return a, true
			}
		}
	}
	return Advisory{}, false
}

func resultFor(files []FileScreenResult, path string) (FileScreenResult, bool) {
	for _, f := range files {
		if f.Path == path {
			return f, true
		}
	}
	return FileScreenResult{}, false
}

// TestCheck_HotspotFileFlaggedStrongerTier is Verify row 3: the churned,
// deeply-nested file draws the stronger-tier advisory; the quiet file does
// not.
func TestCheck_HotspotFileFlaggedStrongerTier(t *testing.T) {
	requireGit(t)
	dir := hotspotFixtureRepo(t)
	store := mineFixture(t, dir)

	result, err := screenPaths(dir, store, []string{"hot.go", "quiet.go"})
	if err != nil {
		t.Fatalf("screenPaths: %v", err)
	}

	if _, ok := findAdvisory(result.Files, "hot.go", AdvisoryStrongerTier); !ok {
		t.Errorf("hot.go: expected a stronger-tier advisory, got %+v", result.Files)
	}
	if a, ok := findAdvisory(result.Files, "quiet.go", AdvisoryStrongerTier); ok {
		t.Errorf("quiet.go: expected NO stronger-tier advisory, got %+v", a)
	}
}

// TestCheck_CouplingPartnerNamed is Verify row 4: files A and B are
// historically coupled; `check A` (B not in the screened set) names B in
// the coupling-partner advisory.
func TestCheck_CouplingPartnerNamed(t *testing.T) {
	requireGit(t)
	dir := couplingFixtureRepo(t)
	store := mineFixture(t, dir)

	result, err := screenPaths(dir, store, []string{"a.go"})
	if err != nil {
		t.Fatalf("screenPaths: %v", err)
	}

	adv, ok := findAdvisory(result.Files, "a.go", AdvisoryCouplingPartner)
	if !ok {
		t.Fatalf("a.go: expected a coupling-partner advisory, got %+v", result.Files)
	}
	if !strings.Contains(adv.Detail, "b.go") {
		t.Errorf("coupling-partner advisory must name b.go specifically, got %q", adv.Detail)
	}
}

// TestCheck_AdvisoryPostureExitsZeroOnFlag is Verify row 5: `check` over a
// fixture hotspot file flags AND still returns exit 0 — advisory NOTICE
// posture, never a gate (spec §9.2).
func TestCheck_AdvisoryPostureExitsZeroOnFlag(t *testing.T) {
	requireGit(t)
	dir := hotspotFixtureRepo(t)
	store := mineFixture(t, dir)

	var stdout, stderr bytes.Buffer
	rc := runCheck([]string{"hot.go", "--out", store.root, "--repo", dir}, &stdout, &stderr)

	if rc != 0 {
		t.Fatalf("runCheck returned %d, want 0 (advisory posture never fails on a flag); stderr=%s", rc, stderr.String())
	}
	if !strings.Contains(stdout.String(), string(AdvisoryStrongerTier)) {
		t.Errorf("stdout must contain the stronger-tier advisory, got:\n%s", stdout.String())
	}
}

// TestCheck_NoHistoryIsCouldNotScreen is Verify row 6: a path with no mined
// history (never committed) screens as could-not-screen, never an implied
// all-clear.
func TestCheck_NoHistoryIsCouldNotScreen(t *testing.T) {
	requireGit(t)
	dir := t.TempDir()
	gitCmd(t, dir, "init", "-q", "-b", "main")
	writeFile(t, dir, "readme.txt", "hello\n")
	gitCmd(t, dir, "add", "readme.txt")
	gitCmd(t, dir, "commit", "-q", "-m", "add readme")
	store := mineFixture(t, dir)

	result, err := screenPaths(dir, store, []string{"never/committed.go"})
	if err != nil {
		t.Fatalf("screenPaths: %v", err)
	}
	fr, ok := resultFor(result.Files, "never/committed.go")
	if !ok {
		t.Fatalf("expected a result for the screened path, got %+v", result.Files)
	}
	if !fr.CouldNotScreen {
		t.Errorf("expected CouldNotScreen=true for a never-mined path, got %+v", fr)
	}
	if fr.Reason == "" {
		t.Errorf("could-not-screen must carry a reason, got empty")
	}
	if len(fr.Advisories) != 0 {
		t.Errorf("a could-not-screen file must raise no advisory (nothing measured to flag on), got %+v", fr.Advisories)
	}
}

// Exercise the public CLI with a real mined snapshot and an independent JSON
// consumer. In particular, a diff path can contain glob syntax and newlines.
func testCheckOutputFormats(t *testing.T) {
	requireGit(t)
	dir := couplingFixtureRepo(t)
	store := mineFixture(t, dir)
	head := strings.TrimSpace(gitCmd(t, dir, "rev-parse", "HEAD"))
	const unusualPath = "-missing[1]: path\n.go"

	var stdout, stderr bytes.Buffer
	textArgs := []string{"check", "--out", store.root, "--repo", dir, "*.go"}
	if rc := dispatch(textArgs, &stdout, &stderr); rc != 0 || stderr.Len() != 0 {
		t.Fatalf("check: exit %d, stderr %q", rc, stderr.String())
	}
	const wantText = "a.go: no advisory signal in this screen\nb.go: no advisory signal in this screen\n"
	if stdout.String() != wantText {
		t.Fatalf("default check output = %q, want %q", stdout.String(), wantText)
	}
	stdout.Reset()

	args := []string{"check", "--json", "--literal-paths", "--out", store.root, "--repo", dir,
		"--", "a.go", unusualPath, "a.go"}
	if rc := dispatch(args, &stdout, &stderr); rc != 0 || stderr.Len() != 0 {
		t.Fatalf("check --json: exit %d, stderr %q", rc, stderr.String())
	}

	var report struct {
		SchemaVersion string `json:"schema_version"`
		RepoHead      string `json:"repo_head"`
		Files         []struct {
			Path           string `json:"path"`
			CouldNotScreen *bool  `json:"could_not_screen"`
			Reason         string `json:"reason"`
			Advisories     []struct {
				Kind   string `json:"kind"`
				Detail string `json:"detail"`
			} `json:"advisories"`
		} `json:"files"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("expected one JSON document, got %q: %v", stdout.String(), err)
	}
	if report.SchemaVersion != "qualgen-check-v1" || report.RepoHead != head || len(report.Files) != 2 {
		t.Fatalf("unexpected screen envelope: %+v", report)
	}
	unmeasured, advised := report.Files[0], report.Files[1]
	if unmeasured.Path != unusualPath || unmeasured.CouldNotScreen == nil || !*unmeasured.CouldNotScreen ||
		unmeasured.Reason == "" || unmeasured.Advisories == nil || len(unmeasured.Advisories) != 0 {
		t.Errorf("unexpected unmeasured row: %+v", unmeasured)
	}
	if advised.Path != "a.go" || advised.CouldNotScreen == nil || *advised.CouldNotScreen || advised.Reason != "" ||
		len(advised.Advisories) != 1 || advised.Advisories[0].Kind != "coupling-partner" ||
		!strings.Contains(advised.Advisories[0].Detail, "b.go") {
		t.Errorf("unexpected advisory row: %+v", advised)
	}
}
