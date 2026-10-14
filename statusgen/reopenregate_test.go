package main

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// Reactivating a retired placeholder (the issue reopened, or lost its excluded
// label) re-derives `gate:` and `labels:` from the LIVE issue rather than
// carrying the retired file's stale frontmatter forward. Without it, an issue
// that reopens with a risk label stays `gate: model` and is dispatched past the
// human gate.

// reopenFixture copies goodrepo and seeds an issue-loop stream (README plus an
// empty done/ archive), returning the root and the stream dir.
func reopenFixture(t *testing.T) (root, dir string) {
	t.Helper()
	root = t.TempDir()
	if err := os.CopyFS(root, os.DirFS("testdata/goodrepo")); err != nil {
		t.Fatal(err)
	}
	dir = filepath.Join(root, "docs/streams/issue-loop")
	if err := os.MkdirAll(filepath.Join(dir, archiveDirName), 0o755); err != nil {
		t.Fatal(err)
	}
	writeTemp(t, dir, "README.md", scanStreamREADME)
	return root, dir
}

// assertRegated checks a reactivated placeholder at path: status todo, the
// wanted gate and labels, exactly one gate line, and no resolved marker.
func assertRegated(t *testing.T, path, wantGate string, wantLabels []string) {
	t.Helper()
	ph, ok, err := parsePlaceholderFile(path)
	if err != nil || !ok {
		t.Fatalf("reactivated file must parse: ok=%v err=%v", ok, err)
	}
	if ph.Status != "todo" {
		t.Errorf("status = %q, want todo", ph.Status)
	}
	if ph.Gate != wantGate {
		t.Errorf("gate = %q, want %q (re-derived from the live issue)", ph.Gate, wantGate)
	}
	if !reflect.DeepEqual(ph.Labels, wantLabels) {
		t.Errorf("labels = %q, want %q (the live issue's labels)", ph.Labels, wantLabels)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(string(raw), "\ngate:"); n != 1 {
		t.Errorf("want exactly one gate line, got %d:\n%s", n, raw)
	}
	if strings.Contains(string(raw), "resolved:") {
		t.Errorf("reactivated file must not keep a resolved marker:\n%s", raw)
	}
}

// TestReopenArchiveRegates — archived placeholder (stored gate: model, labels
// [bug]) whose issue reopens carrying risk:high comes back gate: human with the
// live label set.
// regression: F-scan-reopen-frontmatter
func TestReopenArchiveRegates(t *testing.T) {
	root, dir := reopenFixture(t)
	archived := writeTemp(t, filepath.Join(dir, archiveDirName), "issue-661.md",
		closedPlaceholderFile("issue-loop", 661, "bug", "gate: model"))

	data := map[string][]ghIssue{
		scanHomeRepo(): {{Number: 661, Title: "reopened bug", Labels: lbl("bug", "risk:high")}},
	}
	if code := runScanIssues(root, false, fixtureLister(data, ""), nilCommentLister, blessAll); code != 0 {
		t.Fatalf("scan exited %d, want 0", code)
	}
	if _, err := os.Stat(archived); !os.IsNotExist(err) {
		t.Fatalf("placeholder must move out of done/ (stat err=%v)", err)
	}
	assertRegated(t, filepath.Join(dir, "issue-661.md"), "human", []string{"bug", "risk:high"})
	if code := run(root, "lint", nil, nil, ""); code != 0 {
		t.Fatalf("lint after reactivate exited %d, want 0", code)
	}
}

// TestReopenInPlaceRegates — a retired placeholder still at the stream root
// with NO gate line (read as model from its stale labels) whose issue reopens
// with a security label gains gate: human and the live labels.
// regression: F-scan-reopen-frontmatter
func TestReopenInPlaceRegates(t *testing.T) {
	root, dir := reopenFixture(t)
	path := writeTemp(t, dir, "issue-662.md", closedPlaceholderFile("issue-loop", 662, "bug"))

	data := map[string][]ghIssue{
		scanHomeRepo(): {{Number: 662, Title: "reopened bug", Labels: lbl("bug", "security")}},
	}
	if code := runScanIssues(root, false, fixtureLister(data, ""), nilCommentLister, blessAll); code != 0 {
		t.Fatalf("scan exited %d, want 0", code)
	}
	assertRegated(t, path, "human", []string{"bug", "security"})
	if code := run(root, "lint", nil, nil, ""); code != 0 {
		t.Fatalf("lint after reactivate exited %d, want 0", code)
	}
}

// TestReopenKeepsHumanGate — the reversible default: a stored gate: human is
// never relaxed on reactivate (it may be a human override), while the labels
// still follow the live issue — a block-style list is replaced whole, and an
// issue with no labels left drops the line.
// regression: F-scan-reopen-frontmatter
func TestReopenKeepsHumanGate(t *testing.T) {
	root, dir := reopenFixture(t)
	body := "---\nschema: placeholder-v1\nbrief: issue-loop/issue-663\nissue: 663\n" +
		"repo: example-org/tracker\nwave: 0\nstatus: done\ngate: human\n" +
		"labels:\n  - bug\n  - funds\nresolved: issue-close\n---\nSee issue #663.\n"
	archived := writeTemp(t, filepath.Join(dir, archiveDirName), "issue-663.md", body)

	data := map[string][]ghIssue{
		scanHomeRepo(): {{Number: 663, Title: "reopened", Labels: nil}},
	}
	if code := runScanIssues(root, false, fixtureLister(data, ""), nilCommentLister, blessAll); code != 0 {
		t.Fatalf("scan exited %d, want 0", code)
	}
	if _, err := os.Stat(archived); !os.IsNotExist(err) {
		t.Fatalf("placeholder must move out of done/ (stat err=%v)", err)
	}
	got := filepath.Join(dir, "issue-663.md")
	assertRegated(t, got, "human", nil)
	raw, _ := os.ReadFile(got)
	if strings.Contains(string(raw), "labels:") || strings.Contains(string(raw), "- ") {
		t.Errorf("an issue with no live labels must leave no labels block:\n%s", raw)
	}
	if !strings.Contains(string(raw), "See issue #663.") {
		t.Errorf("body prose must survive reactivation:\n%s", raw)
	}
	if code := run(root, "lint", nil, nil, ""); code != 0 {
		t.Fatalf("lint after reactivate exited %d, want 0", code)
	}
}

// TestReopenKeepsHumanGateAnyYAMLForm — "stored is human" is decided from the
// value the reader decodes, not the raw line text: a quoted or commented human
// gate parses as human everywhere downstream, so reactivation must keep it, and
// a placeholder with no gate line whose stored labels derive human keeps that too.
// regression: F-scan-reopen-frontmatter
func TestReopenKeepsHumanGateAnyYAMLForm(t *testing.T) {
	cases := []struct {
		name   string
		gate   string // the frontmatter gate line ("" = none)
		labels string
	}{
		{"double-quoted", `gate: "human"`, "labels: [bug]\n"},
		{"single-quoted", `gate: 'human'`, "labels: [bug]\n"},
		{"trailing-comment", `gate: human  # override: reviewed by hand`, "labels: [bug]\n"},
		{"multi-line-flow-labels", "gate: human", "labels: [bug,\n  funds]\n"},
		{"no-gate-line-risky-stored-labels", "", "labels: [bug, funds]\n"},
	}
	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root, dir := reopenFixture(t)
			n := 700 + i
			gateLine := ""
			if tc.gate != "" {
				gateLine = tc.gate + "\n"
			}
			body := fmt.Sprintf("---\nschema: placeholder-v1\nbrief: issue-loop/issue-%d\nissue: %d\n"+
				"repo: example-org/tracker\nwave: 0\nstatus: done\n%s%sresolved: issue-close\n---\nSee issue #%d.\n",
				n, n, gateLine, tc.labels, n)
			archived := writeTemp(t, filepath.Join(dir, archiveDirName), fmt.Sprintf("issue-%d.md", n), body)
			data := map[string][]ghIssue{
				scanHomeRepo(): {{Number: n, Title: "reopened", Labels: lbl("bug")}},
			}
			if code := runScanIssues(root, false, fixtureLister(data, ""), nilCommentLister, blessAll); code != 0 {
				t.Fatalf("scan exited %d, want 0", code)
			}
			if _, err := os.Stat(archived); !os.IsNotExist(err) {
				t.Fatalf("placeholder must move out of done/ (stat err=%v)", err)
			}
			assertRegated(t, filepath.Join(dir, fmt.Sprintf("issue-%d.md", n)), "human", []string{"bug"})
			if code := run(root, "lint", nil, nil, ""); code != 0 {
				t.Fatalf("lint after reactivate exited %d, want 0", code)
			}
		})
	}
}

// TestReopenTranscribeRegates — the same-repo transcription lane has its own
// two reactivate sites (in place and from done/); both re-derive from live state.
// regression: F-scan-reopen-frontmatter
func TestReopenTranscribeRegates(t *testing.T) {
	scanWithRoster(t, transcribeRoster())
	root, dir := reopenFixture(t)
	writeRulings(t, root, r7Armed)
	inPlace := writeTemp(t, dir, "issue-664.md", closedPlaceholderFile("issue-loop", 664, "bug", "gate: model"))
	writeTemp(t, filepath.Join(dir, archiveDirName), "issue-665.md",
		closedPlaceholderFile("issue-loop", 665, "bug", "gate: model"))

	list := fixtureLister(map[string][]ghIssue{
		"example-org/tracker": {
			{Number: 664, Title: "reopened bug", Labels: lbl("bug", "funds")},
			{Number: 665, Title: "auth bypass on reopen", Labels: lbl("bug")},
		},
	}, "")
	out := captureRun(t, func() int {
		return runTranscribeScan(root, false, list, nilCommentLister,
			fixtureAuthorResolver(nil), blessNone, blessAuthorityResolver)
	})
	if out.code != 0 {
		t.Fatalf("transcribe-scan exited %d, want 0:\n%s\n%s", out.code, out.log, out.err)
	}
	assertRegated(t, inPlace, "human", []string{"bug", "funds"})
	assertRegated(t, filepath.Join(dir, "issue-665.md"), "human", []string{"bug"})
}

// TestReactivateNeedsLiveGate — the choke point: applyCloseOut refuses a
// reactivate plan built without the live issue (no Gate), leaving the file
// untouched, so a future reactivate site that bypasses reactivatePlan fails
// loudly instead of re-surfacing stale frontmatter.
// regression: F-scan-reopen-frontmatter
func TestReactivateNeedsLiveGate(t *testing.T) {
	_, dir := reopenFixture(t)
	before := closedPlaceholderFile("issue-loop", 666, "bug", "gate: model")
	path := writeTemp(t, dir, "issue-666.md", before)

	err := applyCloseOut(closeOutPlan{Repo: "example-org/tracker", Issue: 666,
		Path: path, Rel: "issue-666.md", Action: "reactivate"})
	if err == nil || !strings.Contains(err.Error(), "no live gate") {
		t.Fatalf("reactivate without a live gate must be refused, got err=%v", err)
	}
	if raw, _ := os.ReadFile(path); string(raw) != before {
		t.Errorf("a refused reactivate must not touch the file:\n%s", raw)
	}
}
