package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const humanStampSampleReadme = "---\nstream: test-stream\nstatus: active\npriority: P1\n---\n\n| # | Brief | Wave | Effort | Status | Verified | Reviewed |\n|---|-------|------|--------|--------|----------|----------|\n| 01 | [brief-01](brief-01.md) | 1 | S | todo | — | — |\n| 02 | [brief-02](brief-02.md) | 1 | M | verified | 2026-07-01 opus-verifier | — |\n| 03 | [brief-03](brief-03.md) | 1 | L | done | 2026-07-01 opus-verifier | 2026-07-02 human:alice |\n\n"

func humanStampFixture(t *testing.T) (root, readmePath string) {
	t.Helper()
	root = t.TempDir()
	gitRun(t, root, "init", "-q")
	gitRun(t, root, "checkout", "-q", "-b", "base")
	streamDir := filepath.Join(root, "docs", "streams", "test-stream")
	mustMkdirAll(t, streamDir)
	readmePath = filepath.Join(streamDir, "README.md")
	writeTemp(t, streamDir, "README.md", humanStampSampleReadme)
	writeTemp(t, streamDir, "brief-01.md", "---\nbrief: test-stream/01\ntitle: Brief 1\nstatus: todo\nwave: 1\neffort: S\ngate: model\nrisk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}\n---\n\nNo human stamp.\n")
	writeTemp(t, streamDir, "brief-02.md", "---\nbrief: test-stream/02\ntitle: Brief 2\nstatus: verified\nwave: 1\neffort: M\ngate: human\nrisk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}\n---\n\nVerified by opus.\n")
	writeTemp(t, streamDir, "brief-03.md", "---\nbrief: test-stream/03\ntitle: Brief 3\nstatus: done\nwave: 1\neffort: L\ngate: human\nrisk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}\n---\n\nDone, reviewed by human:alice.\n")
	gitRun(t, root, "add", "-A")
	gitRun(t, root, "commit", "-q", "-m", "base: stream README with briefs")
	gitRun(t, root, "update-ref", "refs/remotes/origin/main", "HEAD")
	gitRun(t, root, "checkout", "-q", "-b", "feature")
	// Add a dummy commit on the feature branch so HEAD != merge-base.
	// The arming condition requires HEAD to differ from the merge-base;
	// without this the rule is always disarmed in test fixtures.
	mustMkdirAll(t, filepath.Join(root, "dummy"))
	writeTemp(t, filepath.Join(root, "dummy"), "marker", "feature-branch\n")
	gitRun(t, root, "add", "-A")
	gitRun(t, root, "commit", "-q", "-m", "feature: marker commit")
	return root, readmePath
}
func readmeWithHumanInReviewed() string {
	return strings.Replace(humanStampSampleReadme,
		"| 02 | [brief-02](brief-02.md) | 1 | M | verified | 2026-07-01 opus-verifier | — |",
		"| 02 | [brief-02](brief-02.md) | 1 | M | verified | 2026-07-01 opus-verifier | 2026-08-01 human:alice |", 1)
}

func readmeWithHumanInVerified() string {
	return strings.Replace(humanStampSampleReadme,
		"| 01 | [brief-01](brief-01.md) | 1 | S | todo | — | — |",
		"| 01 | [brief-01](brief-01.md) | 1 | S | todo | 2026-08-01 human:alice | — |", 1)
}

func loadStreamsOrSkip(t *testing.T, root string) []*Stream {
	t.Helper()
	streams, _, err := loadStreams(root)
	if err != nil {
		t.Skipf("loadStreams error: %v", err)
	}
	attachPlaceholders(streams)
	return streams
}

func TestHumanStampCaseA_GainReviewedFires(t *testing.T) {
	root, path := humanStampFixture(t)
	gained := readmeWithHumanInReviewed()
	if err := os.WriteFile(path, []byte(gained), 0o644); err != nil {
		t.Fatal(err)
	}
	problems, _ := humanStampProblems(root, loadStreamsOrSkip(t, root))
	if len(problems) == 0 {
		t.Fatal("expected PROBLEM, got none")
	}
}

func TestHumanStampCaseA_GainVerifiedFires(t *testing.T) {
	root, path := humanStampFixture(t)
	gained := readmeWithHumanInVerified()
	if err := os.WriteFile(path, []byte(gained), 0o644); err != nil {
		t.Fatal(err)
	}
	problems, _ := humanStampProblems(root, loadStreamsOrSkip(t, root))
	if len(problems) == 0 {
		t.Fatal("expected PROBLEM, got none")
	}
}

func TestHumanStampCaseB_ReflowDoesNotFire(t *testing.T) {
	root, path := humanStampFixture(t)
	if err := os.WriteFile(path, []byte(humanStampSampleReadme), 0o644); err != nil {
		t.Fatal(err)
	}
	p, _ := humanStampProblems(root, loadStreamsOrSkip(t, root))
	if len(p) != 0 {
		t.Errorf("re-flow must not fire; got %v", p)
	}
}

func TestHumanStampCaseC_UnknownNameFires(t *testing.T) {
	root, path := humanStampFixture(t)
	c := strings.Replace(humanStampSampleReadme,
		"| 02 | [brief-02](brief-02.md) | 1 | M | verified | 2026-07-01 opus-verifier | — |",
		"| 02 | [brief-02](brief-02.md) | 1 | M | verified | 2026-07-01 opus-verifier | 2026-08-01 human:alice |", 1)
	if err := os.WriteFile(path, []byte(c), 0o644); err != nil {
		t.Fatal(err)
	}
	p, _ := humanStampProblems(root, loadStreamsOrSkip(t, root))
	if len(p) == 0 {
		t.Fatal("expected PROBLEM for unknown name, got none")
	}
}
func TestHumanStampCaseD_AuthorizedByBoundary(t *testing.T) {
	root, _ := humanStampFixture(t)
	fdir := filepath.Join(root, "docs", "streams", "findings")
	mustMkdirAll(t, fdir)
	writeTemp(t, fdir, "2026-07-17-boundary.md",
		"---\nid: F-boundary\ndate: \"2026-07-17\"\ntitle: Boundary test\naffects: [\"test-stream\"]\nresolved: false\nauthorized-by: human:alice\n---\n\nBody.\n")
	gitRun(t, root, "add", "-A")
	gitRun(t, root, "commit", "-q", "-m", "add boundary finding")
	gitRun(t, root, "update-ref", "refs/remotes/origin/main", "HEAD")
	gitRun(t, root, "checkout", "-q", "-b", "feature-boundary")
	writeTemp(t, fdir, "2026-07-17-boundary-2.md",
		"---\nid: F-boundary-2\ndate: \"2026-07-17\"\ntitle: Another\naffects: [\"test-stream\"]\nresolved: false\nauthorized-by: human:alice\n---\n\nBody.\n")
	p, _ := humanStampProblems(root, loadStreamsOrSkip(t, root))
	if len(p) != 0 {
		t.Errorf("authorized-by must NOT fire; got %v", p)
	}
}

func TestHumanStampCaseE_ArmedOffOnMain(t *testing.T) {
	root, path := humanStampFixture(t)
	gitRun(t, root, "update-ref", "refs/remotes/origin/main", "HEAD")
	gained := readmeWithHumanInReviewed()
	if err := os.WriteFile(path, []byte(gained), 0o644); err != nil {
		t.Fatal(err)
	}
	p, _ := humanStampProblems(root, loadStreamsOrSkip(t, root))
	if len(p) != 0 {
		t.Errorf("armed off must NOT fire; got %v", p)
	}
}

func TestHumanStampCaseF_RenameWithinRepo(t *testing.T) {
	root, _ := humanStampFixture(t)
	oldDir := filepath.Join(root, "docs", "streams", "old-stream")
	mustMkdirAll(t, oldDir)
	oldReadme := strings.Replace(humanStampSampleReadme, "test-stream", "old-stream", 1)
	writeTemp(t, oldDir, "README.md", oldReadme)
	writeTemp(t, oldDir, "brief-01.md", "---\nbrief: old-stream/01\ntitle: Old Brief\nstatus: todo\nwave: 1\neffort: S\ngate: model\nrisk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}\n---\n\nBody.\n")
	writeTemp(t, oldDir, "brief-02.md", "---\nbrief: old-stream/02\ntitle: Old Brief 2\nstatus: verified\nwave: 1\neffort: M\ngate: human\nrisk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}\n---\n\nBody.\n")
	writeTemp(t, oldDir, "brief-03.md", "---\nbrief: old-stream/03\ntitle: Old Brief 3\nstatus: done\nwave: 1\neffort: L\ngate: human\nrisk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}\n---\n\nBody.\n")
	gitRun(t, root, "add", "-A")
	gitRun(t, root, "commit", "-q", "-m", "add old-stream")
	gitRun(t, root, "update-ref", "refs/remotes/origin/main", "HEAD")
	newDir := filepath.Join(root, "docs", "streams", "new-stream")
	if err := os.Rename(oldDir, newDir); err != nil {
		t.Fatal(err)
	}
	newContent := strings.Replace(oldReadme, "old-stream", "new-stream", 1)
	if err := os.WriteFile(filepath.Join(newDir, "README.md"), []byte(newContent), 0o644); err != nil {
		t.Fatal(err)
	}
	p, _ := humanStampProblems(root, loadStreamsOrSkip(t, root))
	if len(p) != 0 {
		t.Errorf("rename must not fire; got %v", p)
	}
}
func TestHumanStamp_NoOriginMainReturnsNil(t *testing.T) {
	root, path := humanStampFixture(t)
	gitRun(t, root, "update-ref", "-d", "refs/remotes/origin/main")
	gained := readmeWithHumanInReviewed()
	if err := os.WriteFile(path, []byte(gained), 0o644); err != nil {
		t.Fatal(err)
	}
	problems, notices := humanStampProblems(root, loadStreamsOrSkip(t, root))
	if len(problems) != 0 {
		t.Errorf("no origin must not hard-fail; got %v", problems)
	}
	if len(notices) == 0 {
		t.Error("should emit NOTICE")
	}
}

func TestHumanStampNewRowGainsStamp(t *testing.T) {
	root, path := humanStampFixture(t)
	newReadme := strings.TrimSuffix(humanStampSampleReadme, "\n") + "| 04 | [brief-04](brief-04.md) | 1 | M | verified | — | 2026-08-01 human:alice |\n"
	writeTemp(t, filepath.Join(root, "docs", "streams", "test-stream"), "brief-04.md",
		"---\nbrief: test-stream/04\ntitle: Brief 4\nstatus: verified\nwave: 1\neffort: M\ngate: human\nrisk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}\n---\n\nNew brief.\n")
	if err := os.WriteFile(path, []byte(newReadme), 0o644); err != nil {
		t.Fatal(err)
	}
	p, _ := humanStampProblems(root, loadStreamsOrSkip(t, root))
	if len(p) == 0 {
		t.Fatal("expected PROBLEM, got none")
	}
}

func TestHumanStamp_NoStreamsReturnsNil(t *testing.T) {
	root := t.TempDir()
	p, n := humanStampProblems(root, nil)
	if len(p) != 0 {
		t.Errorf("expected no problems without .git; got %v", p)
	}
	// .git absent is a degraded run — advisory NOTICE, never a hard problem.
	if len(n) == 0 {
		t.Error("expected NOTICE for missing .git")
	}
}

func TestHumanStamp_NoGitReturnsNotice(t *testing.T) {
	root := t.TempDir()
	streamDir := filepath.Join(root, "docs", "streams", "test-stream")
	mustMkdirAll(t, streamDir)
	writeTemp(t, streamDir, "README.md", humanStampSampleReadme)
	streams, _, err := loadStreams(root)
	if err != nil {
		t.Skipf("loadStreams: %v", err)
	}
	p, n := humanStampProblems(root, streams)
	if len(p) != 0 {
		t.Errorf("no-git must not hard-fail; got %v", p)
	}
	if len(n) == 0 {
		t.Error("no-git must emit NOTICE")
	}
}

func TestHumanStamp_RowRemovedDoesNotFire(t *testing.T) {
	root, path := humanStampFixture(t)
	lines := strings.Split(humanStampSampleReadme, "\n")
	var kept []string
	for _, l := range lines {
		if strings.Contains(l, "| 03 |") {
			continue
		}
		kept = append(kept, l)
	}
	trimmed := strings.Join(kept, "\n")
	if err := os.WriteFile(path, []byte(trimmed), 0o644); err != nil {
		t.Fatal(err)
	}
	p, _ := humanStampProblems(root, loadStreamsOrSkip(t, root))
	if len(p) != 0 {
		t.Errorf("row removal must not fire; got %v", p)
	}
}

func TestHumanStamp_AckInProseInReadmeDoesNotFire(t *testing.T) {
	root, path := humanStampFixture(t)
	content := humanStampSampleReadme + "\n## Notes\n\nhuman:alice mentioned in prose.\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	p, _ := humanStampProblems(root, loadStreamsOrSkip(t, root))
	if len(p) != 0 {
		t.Errorf("prose must not fire; got %v", p)
	}
}

// TestHumanStampStampSwapFires: an already-stamped cell whose name changes
// on the branch (e.g. human:alice → human:mallory) must fire. This is reviewer
// case 4b — a key-existence-only check would miss it.
func TestHumanStampStampSwapFires(t *testing.T) {
	root, path := humanStampFixture(t)
	// Swap name in brief-03 Reviewed cell from human:alice to human:mallory.
	c := strings.Replace(humanStampSampleReadme,
		"| 03 | [brief-03](brief-03.md) | 1 | L | done | 2026-07-01 opus-verifier | 2026-07-02 human:alice |",
		"| 03 | [brief-03](brief-03.md) | 1 | L | done | 2026-07-01 opus-verifier | 2026-07-02 human:mallory |", 1)
	if err := os.WriteFile(path, []byte(c), 0o644); err != nil {
		t.Fatal(err)
	}
	p, _ := humanStampProblems(root, loadStreamsOrSkip(t, root))
	if len(p) == 0 {
		t.Fatal("stamp swap (name change) must fire; got none")
	}
}

// TestHumanStampStampAppendFires: appending a second human:<name> stamp
// to an already-stamped cell must fire. This is reviewer case 4c.
func TestHumanStampStampAppendFires(t *testing.T) {
	root, path := humanStampFixture(t)
	// Append human:bob to the already-stamped cell.
	c := strings.Replace(humanStampSampleReadme,
		"| 03 | [brief-03](brief-03.md) | 1 | L | done | 2026-07-01 opus-verifier | 2026-07-02 human:alice |",
		"| 03 | [brief-03](brief-03.md) | 1 | L | done | 2026-07-01 opus-verifier | 2026-07-02 human:alice, 2026-08-01 human:bob |", 1)
	if err := os.WriteFile(path, []byte(c), 0o644); err != nil {
		t.Fatal(err)
	}
	p, _ := humanStampProblems(root, loadStreamsOrSkip(t, root))
	if len(p) == 0 {
		t.Fatal("stamp append must fire; got none")
	}
}

// TestHumanStamp_ReDateSameNameDoesNotFire pins a KNOWN RESIDUAL, not a
// desired property: reviewer case 4 — an already-stamped cell whose stamp is
// re-dated while the name stays the same. The comparison unit is the set of
// stamp NAMES per cell, so re-dating changes nothing the gate can see and the
// branch stays silent. Re-dating a stale sign-off to look fresh therefore
// remains possible on the PR path.
//
// It is left open on purpose rather than closed by comparing raw cell text: a
// text comparison fires on ordinary re-flow and re-wording, which
// TestHumanStampCaseB_ReflowDoesNotFire pins as must-not-fire, and a
// false-positive class is what made the first cut of this gate unadoptable.
// Closing it properly needs a (name, date) pairing rule, which is a separate
// change. This test exists so the limit is discovered here rather than
// re-discovered as an unknown in review.
func TestHumanStamp_ReDateSameNameDoesNotFire(t *testing.T) {
	root, path := humanStampFixture(t)
	c := strings.Replace(humanStampSampleReadme,
		"| 03 | [brief-03](brief-03.md) | 1 | L | done | 2026-07-01 opus-verifier | 2026-07-02 human:alice |",
		"| 03 | [brief-03](brief-03.md) | 1 | L | done | 2026-07-01 opus-verifier | 2026-08-01 human:alice |", 1)
	if err := os.WriteFile(path, []byte(c), 0o644); err != nil {
		t.Fatal(err)
	}
	p, _ := humanStampProblems(root, loadStreamsOrSkip(t, root))
	if len(p) != 0 {
		t.Fatalf("documented residual changed: re-dating an existing stamp now fires (%v).\n"+
			"If this is intentional, update the residual note in humanstamp.go and the PR/brief text.", p)
	}
}

// TestHumanStamp_AmbiguousSpaceAfterColonFires pins the fix for the review
// finding on PR #324: brieffile.go's hasHumanReviewer (strings.Fields +
// HasPrefix(tok, "human:")) is satisfied by the bare "human:" token alone, so
// "human: mallory" (a space after the colon) passed the done-gate while
// humanStampRe — this gate's own detector — required a name character
// immediately after the colon and so never saw it at all. A one-character
// edit on a PR branch could therefore change the recorded human from alice to
// mallory and stay invisible to --lint. The gate must now fire on the
// ambiguous form even though it cannot recover a name from it.
func TestHumanStamp_AmbiguousSpaceAfterColonFires(t *testing.T) {
	root, path := humanStampFixture(t)
	// Reviewer's exact row 2 construction: replace the canonical
	// "human:alice" stamp with the space-separated "human: mallory" bypass.
	c := strings.Replace(humanStampSampleReadme,
		"| 03 | [brief-03](brief-03.md) | 1 | L | done | 2026-07-01 opus-verifier | 2026-07-02 human:alice |",
		"| 03 | [brief-03](brief-03.md) | 1 | L | done | 2026-07-01 opus-verifier | 2026-08-07 human: mallory |", 1)
	if err := os.WriteFile(path, []byte(c), 0o644); err != nil {
		t.Fatal(err)
	}
	p, _ := humanStampProblems(root, loadStreamsOrSkip(t, root))
	if len(p) == 0 {
		t.Fatal("space-after-colon bypass (\"human: mallory\") must fire; got none")
	}
	found := false
	for _, problem := range p {
		if strings.Contains(problem, "ambiguous human:") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected an 'ambiguous human:' PROBLEM, got %v", p)
	}
}

// TestHumanStamp_BareColonNoNameFires covers the degenerate case of the same
// bypass class: a "human:" token with nothing after it at all (not even a
// later field), e.g. dropped in as "reviewed by human:". hasHumanReviewer
// still accepts this (HasPrefix(tok, "human:") on the token "human:" itself);
// the gate must still fire.
func TestHumanStamp_BareColonNoNameFires(t *testing.T) {
	root, path := humanStampFixture(t)
	c := strings.Replace(humanStampSampleReadme,
		"| 03 | [brief-03](brief-03.md) | 1 | L | done | 2026-07-01 opus-verifier | 2026-07-02 human:alice |",
		"| 03 | [brief-03](brief-03.md) | 1 | L | done | 2026-07-01 opus-verifier | 2026-08-07 human: |", 1)
	if err := os.WriteFile(path, []byte(c), 0o644); err != nil {
		t.Fatal(err)
	}
	p, _ := humanStampProblems(root, loadStreamsOrSkip(t, root))
	if len(p) == 0 {
		t.Fatal("bare \"human:\" token with no name at all must fire; got none")
	}
}

// TestHumanStampModeGating pins reviewer mutation M7: with the `if mode ==
// "lint"` guard removed from run() in main.go, `go test ./...` stayed green
// because nothing asserted that check/write modes are silent on a gained
// stamp. This test runs the same gained-stamp fixture through run() in both
// "lint" and "check" mode and asserts the human-stamp PROBLEM appears only
// under lint.
func TestHumanStampModeGating(t *testing.T) {
	root, path := humanStampFixture(t)
	gained := readmeWithHumanInReviewed()
	if err := os.WriteFile(path, []byte(gained), 0o644); err != nil {
		t.Fatal(err)
	}

	lintOut := captureStderr(t, func() { run(root, "lint", nil, nil, "") })
	if !strings.Contains(lintOut, "gained human:") {
		t.Fatalf("lint mode must report the gained stamp; got:\n%s", lintOut)
	}

	checkOut := captureStderr(t, func() { run(root, "check", nil, nil, "") })
	if strings.Contains(checkOut, "gained human:") {
		t.Fatalf("check mode must NOT run the human-stamp gate (no PR branch to compare against); got:\n%s", checkOut)
	}
}

// TestHumanStamp_PrecedingTableDoesNotFire: a README with a markdown table
// before the brief table must NOT produce false positives. This is reviewer
// Cause 1 — the original parser's inTable latch grabbed the first |---|
// separator and never reached the brief table.
func TestHumanStamp_PrecedingTableDoesNotFire(t *testing.T) {
	root := t.TempDir()
	gitRun(t, root, "init", "-q")
	gitRun(t, root, "checkout", "-q", "-b", "base")
	streamDir := filepath.Join(root, "docs", "streams", "test-stream")
	mustMkdirAll(t, streamDir)
	// README with a preceding table (e.g. a merge-note detail table) before the brief table.
	readmeWithPrecedingTable := `---
stream: test-stream
status: active
priority: P1
---

## Merge notes

| Date | Source | Notes |
|------|--------|-------|
| 2026-08-01 | desk | Merged from a sibling stream |

## Briefs

| # | Brief | Wave | Effort | Status | Verified | Reviewed |
|---|-------|------|--------|--------|----------|----------|
| 01 | [brief-01](brief-01.md) | 1 | S | todo | — | — |
| 02 | [brief-02](brief-02.md) | 1 | M | verified | 2026-07-01 opus-verifier | — |
| 03 | [brief-03](brief-03.md) | 1 | L | done | 2026-07-01 opus-verifier | 2026-07-02 human:alice |

`
	readmePath := filepath.Join(streamDir, "README.md")
	writeTemp(t, streamDir, "README.md", readmeWithPrecedingTable)
	writeTemp(t, streamDir, "brief-01.md", "---\nbrief: test-stream/01\ntitle: Brief 1\nstatus: todo\nwave: 1\neffort: S\ngate: model\nrisk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}\n---\n\nBody.\n")
	writeTemp(t, streamDir, "brief-02.md", "---\nbrief: test-stream/02\ntitle: Brief 2\nstatus: verified\nwave: 1\neffort: M\ngate: human\nrisk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}\n---\n\nBody.\n")
	writeTemp(t, streamDir, "brief-03.md", "---\nbrief: test-stream/03\ntitle: Brief 3\nstatus: done\nwave: 1\neffort: L\ngate: human\nrisk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}\n---\n\nBody.\n")
	gitRun(t, root, "add", "-A")
	gitRun(t, root, "commit", "-q", "-m", "base: README with preceding table")
	gitRun(t, root, "update-ref", "refs/remotes/origin/main", "HEAD")
	gitRun(t, root, "checkout", "-q", "-b", "feature-preceding-table")
	mustMkdirAll(t, filepath.Join(root, "dummy"))
	writeTemp(t, filepath.Join(root, "dummy"), "marker", "feature\n")
	gitRun(t, root, "add", "-A")
	gitRun(t, root, "commit", "-q", "-m", "feature: marker commit")
	// Re-write the README identically (simulating an unrelated PR touching README)
	if err := os.WriteFile(readmePath, []byte(readmeWithPrecedingTable), 0o644); err != nil {
		t.Fatal(err)
	}
	p, _ := humanStampProblems(root, loadStreamsOrSkip(t, root))
	if len(p) != 0 {
		t.Errorf("preceding table must not cause false positives; got %v", p)
	}
}

// TestHumanStamp_ProseStreamWordDoesNotFire: a README whose prose contains a
// line beginning "stream:" must NOT produce false positives. This is reviewer
// Cause 2 — the original parser read the stream name from ANY line starting
// with "stream:", including wrapped English prose.
func TestHumanStamp_ProseStreamWordDoesNotFire(t *testing.T) {
	root := t.TempDir()
	gitRun(t, root, "init", "-q")
	gitRun(t, root, "checkout", "-q", "-b", "base")
	streamDir := filepath.Join(root, "docs", "streams", "test-stream")
	mustMkdirAll(t, streamDir)
	// README with prose containing "stream:" on a regular line (not frontmatter).
	readmeWithStreamProse := `---
stream: test-stream
status: active
priority: P1
---

Sharpen the initiative-streams system itself and publish it. Three sub-goals, one
stream: (a) adopt the five research-verified mechanisms from the design spec §11

| # | Brief | Wave | Effort | Status | Verified | Reviewed |
|---|-------|------|--------|--------|----------|----------|
| 01 | [brief-01](brief-01.md) | 1 | S | todo | — | — |
| 02 | [brief-02](brief-02.md) | 1 | M | verified | 2026-07-01 opus-verifier | — |
| 03 | [brief-03](brief-03.md) | 1 | L | done | 2026-07-01 opus-verifier | 2026-07-02 human:alice |

`
	readmePath := filepath.Join(streamDir, "README.md")
	writeTemp(t, streamDir, "README.md", readmeWithStreamProse)
	writeTemp(t, streamDir, "brief-01.md", "---\nbrief: test-stream/01\ntitle: Brief 1\nstatus: todo\nwave: 1\neffort: S\ngate: model\nrisk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}\n---\n\nBody.\n")
	writeTemp(t, streamDir, "brief-02.md", "---\nbrief: test-stream/02\ntitle: Brief 2\nstatus: verified\nwave: 1\neffort: M\ngate: human\nrisk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}\n---\n\nBody.\n")
	writeTemp(t, streamDir, "brief-03.md", "---\nbrief: test-stream/03\ntitle: Brief 3\nstatus: done\nwave: 1\neffort: L\ngate: human\nrisk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}\n---\n\nBody.\n")
	gitRun(t, root, "add", "-A")
	gitRun(t, root, "commit", "-q", "-m", "base: README with stream: in prose")
	gitRun(t, root, "update-ref", "refs/remotes/origin/main", "HEAD")
	gitRun(t, root, "checkout", "-q", "-b", "feature-stream-prose")
	mustMkdirAll(t, filepath.Join(root, "dummy"))
	writeTemp(t, filepath.Join(root, "dummy"), "marker", "feature\n")
	gitRun(t, root, "add", "-A")
	gitRun(t, root, "commit", "-q", "-m", "feature: marker commit")
	// Re-write the README identically
	if err := os.WriteFile(readmePath, []byte(readmeWithStreamProse), 0o644); err != nil {
		t.Fatal(err)
	}
	p, _ := humanStampProblems(root, loadStreamsOrSkip(t, root))
	if len(p) != 0 {
		t.Errorf("prose containing 'stream:' must not cause false positives; got %v", p)
	}
}
