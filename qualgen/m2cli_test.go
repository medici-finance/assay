package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func m2Fixture(t *testing.T) (repo, inducing, fix string) {
	t.Helper()
	requireGit(t)
	repo = t.TempDir()
	szzGit(t, repo, "2020-01-01T00:00:00Z", "init", "-q", "-b", "main")
	inducing = commitFile(t, repo, "2020-01-01T00:00:00Z", "app.go", "package app\nreturn x / 0\ntail\n", "introduce behavior")
	fix = commitFile(t, repo, "2020-01-04T00:00:00Z", "app.go", "package app\nreturn x / y\ntail\n", "Correct divisor\n\nCloses #7")
	return
}

func m2Source(t *testing.T, tip, fix string) FixSource {
	t.Helper()
	date := mustTime(t, "2020-01-03T00:00:00Z")
	return FixSource{SchemaVersion: fixSourceSchema, Repository: "example/project", HeadSHA: tip, MergedPRCount: 2,
		Issues:       []FixSourceIssue{{Number: 7, Labels: []string{"bug"}, ReportedAt: &date}},
		PullRequests: []FixSourcePR{{Number: 2, MergeSHA: fix, Title: "Correct divisor"}}}
}

func sourceFile(t *testing.T, source FixSource) string {
	t.Helper()
	raw, err := json.Marshal(source)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "source.json")
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func cliOK(t *testing.T, args ...string) string {
	t.Helper()
	var out, errb bytes.Buffer
	if rc := dispatch(args, &out, &errb); rc != 0 {
		t.Fatalf("%v exit %d: %s", args, rc, errb.String())
	}
	return out.String()
}

// regression: #1589. This exercises real parsing, issue-label linkage, blame,
// persistence, rendering and the PR/check consumer boundary against a planted bug.
func TestM2CLIFlow(t *testing.T) {
	repo, inducing, fix := m2Fixture(t)
	out := t.TempDir()
	source := sourceFile(t, m2Source(t, fix, fix))
	cliOK(t, "mine", "--repo", repo, "--out", out, "--fix-source", source)
	store := NewStore(out)
	traces, err := store.ReadTraces()
	if err != nil || len(traces) != 1 {
		t.Fatalf("traces=%+v, err=%v", traces, err)
	}
	tr := traces[0]
	if tr.TraceState != TraceTraced || tr.EvidenceTier != Tier1 || !contains(tr.InducingCommits, inducing) || tr.FixPR != 2 {
		t.Fatalf("wrong trace: %+v", tr)
	}
	view := cliOK(t, "report", "--out", out)
	for _, want := range []string{"1 traced, 0 traced-none, 0 could-not-trace", "| defect_inducing_rate | 0.5 | 1 | 1 / 0 / 0 |", fix, inducing} {
		if !strings.Contains(view, want) {
			t.Fatalf("report lacks %q:\n%s", want, view)
		}
	}
	if strings.Contains(view, "populated by quality/06") {
		t.Fatal("M2 placeholder survived")
	}
	feed := cliOK(t, "pr", "2", "--repo", repo, "--out", out, "--head", fix, "--base", inducing)
	var features PRFeatureFeed
	if err := json.Unmarshal([]byte(feed), &features); err != nil {
		t.Fatal(err)
	}
	if len(features.Files) != 1 || features.Files[0].DefectDensity == nil || *features.Files[0].DefectDensity != 1 {
		t.Fatalf("defect density did not reach PR feed: %s", feed)
	}
	if check := cliOK(t, "check", "app.go", "--repo", repo, "--out", out); !strings.Contains(check, "add-coverage") {
		t.Fatalf("defect signal did not reach screen: %s", check)
	}
	path, _ := store.tablePath(KindDefect)
	before, _ := os.ReadFile(path)
	// Single-dash legacy forms also reach the new options. No duplicate facts.
	cliOK(t, "mine", "-repo", repo, "-out", out, "-fix-source", source)
	after, _ := os.ReadFile(path)
	if !bytes.Equal(before, after) {
		t.Fatal("repeat mining duplicated defect facts")
	}
	if status := gitCmd(t, repo, "status", "--porcelain"); status != "" {
		t.Fatalf("mined repo was modified: %s", status)
	}
}

func TestM2MissingEvidence(t *testing.T) {
	for _, mode := range []string{"missing-issue", "no-denominator", "addition-only"} {
		t.Run(mode, func(t *testing.T) {
			repo, _, fix := m2Fixture(t)
			source := m2Source(t, fix, fix)
			switch mode {
			case "missing-issue":
				source.Issues = nil
			case "no-denominator":
				source.MergedPRCount = 0
			case "addition-only":
				fix = commitFile(t, repo, "2020-01-05T00:00:00Z", "new.txt", "new guard\n", "Closes #8")
				source.HeadSHA = fix
				source.Issues = append(source.Issues, FixSourceIssue{Number: 8, Labels: []string{"bug"}})
			}
			out := t.TempDir()
			cliOK(t, "mine", "--repo", repo, "--out", out, "--fix-source", sourceFile(t, source))
			view := cliOK(t, "report", "--out", out)
			if mode != "addition-only" && !strings.Contains(view, "| defect_inducing_rate | not measured") {
				t.Fatalf("unavailable evidence rendered as a measured rate:\n%s", view)
			}
			if mode == "missing-issue" && !strings.Contains(view, "unresolved fix identification: 1") {
				t.Fatal(view)
			}
			if mode == "addition-only" && !strings.Contains(view, "could-not-trace") {
				t.Fatal(view)
			}
		})
	}
}

func TestM2CustomLabelsAndCorrection(t *testing.T) {
	repo, _, fix := m2Fixture(t)
	source := m2Source(t, fix, fix)
	source.Issues[0].Labels = []string{"regression"}
	out := t.TempDir()
	path := sourceFile(t, source)
	cliOK(t, "mine", "--repo", repo, "--out", out, "--fix-source", path, "--defect-labels", "regression")
	before, err := AssembleFileFeatures(NewStore(out), "app.go")
	if err != nil || before.DefectDensity.State != StateMeasured {
		t.Fatalf("custom labels not used: %+v %v", before, err)
	}
	// Corrected classification produces an empty current snapshot. The prior
	// density remains historical, but cannot leak into current PR advice.
	cliOK(t, "mine", "--repo", repo, "--out", out, "--fix-source", path)
	after, err := AssembleFileFeatures(NewStore(out), "app.go")
	if err != nil || after.DefectDensity.State != StateCouldNotMeasure {
		t.Fatalf("stale density survived correction: %+v %v", after, err)
	}
	view := cliOK(t, "report", "--out", out)
	if !strings.Contains(view, "Trace outcomes: 0 traced") {
		t.Fatal(view)
	}
}

func TestM2StaleSourceAndSnapshot(t *testing.T) {
	repo, _, fix := m2Fixture(t)
	source := sourceFile(t, m2Source(t, fix, fix))
	out := t.TempDir()
	cliOK(t, "mine", "--repo", repo, "--out", out, "--fix-source", source)
	commitFile(t, repo, "2020-01-06T00:00:00Z", "notes.txt", "notes\n", "add notes")
	badOut := filepath.Join(t.TempDir(), "must-not-exist")
	var stdout, stderr bytes.Buffer
	if rc := dispatch([]string{"mine", "--repo", repo, "--out", badOut, "--fix-source", source}, &stdout, &stderr); rc == 0 {
		t.Fatal("stale source accepted")
	}
	if _, err := os.Stat(badOut); !os.IsNotExist(err) {
		t.Fatalf("stale source created artifacts: %v", err)
	}
	cliOK(t, "mine", "--repo", repo, "--out", out)
	if view := cliOK(t, "report", "--out", out); !strings.Contains(view, "defect snapshot is stale") {
		t.Fatal(view)
	}
	f, err := AssembleFileFeatures(NewStore(out), "app.go")
	if err != nil || f.DefectDensity.State != StateCouldNotMeasure {
		t.Fatalf("stale snapshot used: %+v %v", f, err)
	}
}

func TestM2SourceValidation(t *testing.T) {
	_, _, fix := m2Fixture(t)
	valid := m2Source(t, fix, fix)
	for _, mode := range []string{"schema", "head", "labels", "duplicate-issue", "duplicate-pr", "denominator", "unknown-field", "trailing-json"} {
		t.Run(mode, func(t *testing.T) {
			source := valid
			switch mode {
			case "schema":
				source.SchemaVersion = "unknown"
			case "head":
				source.HeadSHA = "HEAD"
			case "labels":
				source.Issues = []FixSourceIssue{{Number: 7}}
			case "duplicate-issue":
				source.Issues = append(append([]FixSourceIssue{}, source.Issues...), source.Issues[0])
			case "duplicate-pr":
				source.PullRequests = append(append([]FixSourcePR{}, source.PullRequests...), source.PullRequests[0])
			case "denominator":
				source.MergedPRCount = -1
			}
			path := sourceFile(t, source)
			if mode == "unknown-field" {
				raw, _ := os.ReadFile(path)
				raw = append([]byte(`{"typo":true,`), raw[1:]...)
				os.WriteFile(path, raw, 0600)
			}
			if mode == "trailing-json" {
				raw, _ := os.ReadFile(path)
				os.WriteFile(path, append(raw, []byte(` {}`)...), 0600)
			}
			if _, err := loadFixSource(path); err == nil {
				t.Fatal("invalid source accepted")
			}
		})
	}
}

func TestM2LibraryArtifactsReport(t *testing.T) {
	repo, _, fix := m2Fixture(t)
	store := NewStore(t.TempDir())
	corpus := TraceDefects(openRepo(t, repo), []DefectFix{identifiedFix(fix, 2, 7)}, nil)
	if err := corpus.WriteTo(store, 2, mustTime(t, "2020-01-05T00:00:00Z")); err != nil {
		t.Fatal(err)
	}
	view := cliOK(t, "report", "--out", store.root)
	if !strings.Contains(view, "1 traced, 0 traced-none") || !strings.Contains(view, "| defect_inducing_rate | 0.5 |") {
		t.Fatal(view)
	}
}

func TestM2UnfinishedPassIgnored(t *testing.T) {
	repo, _, fix := m2Fixture(t)
	out := t.TempDir()
	cliOK(t, "mine", "--repo", repo, "--out", out, "--fix-source", sourceFile(t, m2Source(t, fix, fix)))
	store := NewStore(out)
	traces, err := store.ReadTraces()
	if err != nil {
		t.Fatal(err)
	}
	changed := traces[0]
	changed.TraceState = TraceCouldNotTrace
	changed.CouldNotTraceReason = ReasonBlameError
	changed.InducingCommits = []string{}
	changed.Confidence = CouldNotMeasure[float64]("injected interrupted pass")
	if err := store.Append(KindDefect, changed); err != nil {
		t.Fatal(err)
	}
	view := cliOK(t, "report", "--out", out)
	if !strings.Contains(view, "1 traced, 0 traced-none, 0 could-not-trace") || strings.Contains(view, "| could-not-trace |") {
		t.Fatal(view)
	}
}

func TestM2MergeFixCountedOnce(t *testing.T) {
	repo, _, _ := m2Fixture(t)
	// A second branch fix is visible both as a branch commit and as its merge.
	gitCmd(t, repo, "checkout", "-b", "fix/second")
	branchFix := commitFile(t, repo, "2020-01-06T00:00:00Z", "app.go", "package app\nreturn x / safeY\ntail\n", "Fixes #8")
	gitCmd(t, repo, "checkout", "main")
	szzGit(t, repo, "2020-01-07T00:00:00Z", "merge", "--no-ff", "fix/second", "-m", "Merge pull request #3 from example/fix/second")
	merge := gitCmd(t, repo, "rev-parse", "HEAD")
	source := FixSource{SchemaVersion: fixSourceSchema, Repository: "example/project", HeadSHA: merge,
		Issues:       []FixSourceIssue{{Number: 7, Labels: []string{"bug"}}, {Number: 8, Labels: []string{"bug"}}},
		PullRequests: []FixSourcePR{{Number: 3, MergeSHA: merge, Body: "Fixes #8"}}}
	out := t.TempDir()
	cliOK(t, "mine", "--repo", repo, "--out", out, "--fix-source", sourceFile(t, source))
	fixes, err := NewStore(out).ReadDefects()
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, f := range fixes {
		if f.FixCommitSHA == branchFix {
			t.Fatal("branch fix was counted in addition to its merge")
		}
		if f.FixPRNumber == 3 {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("merged PR has %d fix records", count)
	}
}
