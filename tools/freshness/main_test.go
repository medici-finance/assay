package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// writeManifest writes a minimal freshness.yaml with two artifacts: "fresh.md"
// (no upstreams, reviewed long ago but under its max-age) and "stale.md" (past
// its max-age deadline as of --as-of 2026-08-08). Neither has upstreams, so the
// git-log path is not exercised here — that mechanic is covered by the existing
// STALE-on-upstream-commit behavior and is unchanged by this test's additions.
func writeManifest(t *testing.T, root string) {
	t.Helper()
	manifest := `artifacts:
  - path: fresh.md
    last-reviewed: "2026-08-01"
    max-age-days: 30
    upstreams: []
  - path: stale.md
    last-reviewed: "2026-01-01"
    max-age-days: 30
    upstreams: []
`
	if err := os.WriteFile(filepath.Join(root, "freshness.yaml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestRun_AllArtifacts_ExitsNonZeroWhenAnyStale(t *testing.T) {
	root := t.TempDir()
	writeManifest(t, root)

	var stdout, stderr bytes.Buffer
	code := run([]string{"--root", root, "--as-of", "2026-08-08"}, &stdout, &stderr)

	if code != 1 {
		t.Fatalf("exit code = %d, want 1 (stale.md should fail the whole manifest); stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "STALE  stale.md") {
		t.Errorf("stdout missing STALE stale.md line: %q", stdout.String())
	}
	if !strings.Contains(stdout.String(), "FRESH  fresh.md") {
		t.Errorf("stdout missing FRESH fresh.md line: %q", stdout.String())
	}
}

func TestRun_Only_ScopesCheckAndExitCode(t *testing.T) {
	root := t.TempDir()
	writeManifest(t, root)

	// Scoping to the fresh artifact alone must succeed even though the
	// manifest as a whole has a stale artifact — this is the fix for #514:
	// a caller verifying its own artifact should not inherit unrelated
	// staleness elsewhere in the manifest.
	var stdout, stderr bytes.Buffer
	code := run([]string{"--root", root, "--as-of", "2026-08-08", "--only", "fresh.md"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("--only fresh.md: exit code = %d, want 0; stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	if strings.Contains(stdout.String(), "stale.md") {
		t.Errorf("--only fresh.md: stdout should not mention stale.md, got %q", stdout.String())
	}

	// Scoping to the stale artifact alone must still fail.
	stdout.Reset()
	stderr.Reset()
	code = run([]string{"--root", root, "--as-of", "2026-08-08", "--only", "stale.md"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("--only stale.md: exit code = %d, want 1; stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	if strings.Contains(stdout.String(), "fresh.md") {
		t.Errorf("--only stale.md: stdout should not mention fresh.md, got %q", stdout.String())
	}
}

func TestRun_Only_CommaSeparatedAndRepeated(t *testing.T) {
	root := t.TempDir()
	writeManifest(t, root)

	var stdout, stderr bytes.Buffer
	code := run([]string{"--root", root, "--as-of", "2026-08-08", "--only", "fresh.md,stale.md"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("comma-separated --only: exit code = %d, want 1; stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "fresh.md") || !strings.Contains(stdout.String(), "stale.md") {
		t.Errorf("comma-separated --only should check both artifacts, got %q", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = run([]string{"--root", root, "--as-of", "2026-08-08", "--only", "fresh.md", "--only", "stale.md"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("repeated --only: exit code = %d, want 1; stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "fresh.md") || !strings.Contains(stdout.String(), "stale.md") {
		t.Errorf("repeated --only should check both artifacts, got %q", stdout.String())
	}
}

func TestRun_Only_UnknownPathFailsLoudly(t *testing.T) {
	root := t.TempDir()
	writeManifest(t, root)

	var stdout, stderr bytes.Buffer
	code := run([]string{"--root", root, "--as-of", "2026-08-08", "--only", "nope.md"}, &stdout, &stderr)

	if code != 2 {
		t.Fatalf("exit code = %d, want 2 (usage/config error) for an unknown --only path", code)
	}
	if !strings.Contains(stderr.String(), "nope.md") {
		t.Errorf("stderr should name the offending path, got %q", stderr.String())
	}
	if stdout.String() != "" {
		t.Errorf("stdout should be empty on a config error, got %q", stdout.String())
	}
}

func TestSelectOnly_PreservesManifestOrderAndDedupes(t *testing.T) {
	artifacts := []artifact{
		{Path: "a.md"},
		{Path: "b.md"},
		{Path: "c.md"},
	}

	got, err := selectOnly(artifacts, []string{"c.md", "a.md", "c.md"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d artifacts, want 2 (deduped): %+v", len(got), got)
	}
	if got[0].Path != "c.md" || got[1].Path != "a.md" {
		t.Errorf("selectOnly should preserve --only's own order, got %+v", got)
	}
}

// --- Claims: claim-staleness reproduction --------------------------
//
// A normative audit document ("example-audit/audit-2026-07-14.md") issued a
// CUT verdict — "no such artifact exists" — that was true when written. An
// upstream repo (example-service#16) then merged exactly the artifact the verdict
// said didn't exist. The whole audit document's OTHER content was still
// accurate, so nothing about the document as a whole looked wrong, and the
// stale verdict kept instructing its removal. Nothing in the pipeline
// flagged it — that is the bug. These tests build the same shape (a doc with
// a negative claim, an upstream repo that later adds the very thing the
// claim denies) and show the per-claim check now catches it.

// runGit runs a git command in dir, failing the test on error. It sets a
// local commit identity and deterministic author/committer dates so the
// "commit landed after last-reviewed" check is reproducible rather than
// dependent on wall-clock test-run time.
func runGit(t *testing.T, dir string, when time.Time, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=freshness-test",
		"GIT_AUTHOR_EMAIL=freshness-test@example.invalid",
		"GIT_COMMITTER_NAME=freshness-test",
		"GIT_COMMITTER_EMAIL=freshness-test@example.invalid",
		"GIT_AUTHOR_DATE="+when.Format(time.RFC3339),
		"GIT_COMMITTER_DATE="+when.Format(time.RFC3339),
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
}

// newUpstreamRepo creates a bare-ish local git repo (the stand-in for
// example-org/example-service) at <root>/<name>, seeded with an initial commit
// dated seedDate so the repo has SOME history before the artifact under test
// is ever reviewed.
func newUpstreamRepo(t *testing.T, root, name string, seedDate time.Time) string {
	t.Helper()
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, seedDate, "init", "-q")
	runGit(t, dir, seedDate, "config", "user.email", "freshness-test@example.invalid")
	runGit(t, dir, seedDate, "config", "user.name", "freshness-test")
	seedFile := filepath.Join(dir, "README.md")
	if err := os.WriteFile(seedFile, []byte("seed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, seedDate, "add", "README.md")
	runGit(t, dir, seedDate, "commit", "-q", "-m", "seed")
	return dir
}

// commitFile writes relPath under dir with the given content and commits it
// at the given time, simulating an upstream PR merging after the artifact's
// last-reviewed date.
func commitFile(t *testing.T, dir, relPath, content string, when time.Time) {
	t.Helper()
	full := filepath.Join(dir, relPath)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, when, "add", relPath)
	runGit(t, dir, when, "commit", "-q", "-m", "add "+relPath)
}

const exampleAuditAnchor = `THE CENTRAL CLAIM. No such artifact exists.`

func writeAuditDoc(t *testing.T, root, relPath string) {
	t.Helper()
	full := filepath.Join(root, relPath)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	doc := "# Example audit — item 9\n\n" + exampleAuditAnchor + "\n\n" +
		"`grep -ri 'balance invariant'` across the modules returns zero hits. CUT the sentence.\n"
	if err := os.WriteFile(full, []byte(doc), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestCheckClaim_FreshBeforeUpstreamArtifactLands(t *testing.T) {
	root := t.TempDir()
	reviewedAt := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)
	upstreamDir := newUpstreamRepo(t, root, "example-service", reviewedAt.AddDate(0, 0, -30))

	auditPath := "example-audit/audit-2026-07-14.md"
	writeAuditDoc(t, root, auditPath)

	a := artifact{Path: auditPath}
	c := claim{
		Anchor:       exampleAuditAnchor,
		LastReviewed: "2026-07-14",
		Upstreams: []upstream{
			{Repo: "example-service", Globs: []string{"spike/example/**"}},
		},
	}

	today := reviewedAt.AddDate(0, 0, 1)
	stale, reason := checkClaim(a, c, today, root)
	if stale {
		t.Fatalf("claim should still be FRESH before the upstream artifact lands, got STALE: %s (upstream dir %s)", reason, upstreamDir)
	}
}

func TestCheckClaim_StaleAfterUpstreamArtifactLands(t *testing.T) {
	root := t.TempDir()
	reviewedAt := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)
	upstreamRepoDir := newUpstreamRepo(t, root, "example-service", reviewedAt.AddDate(0, 0, -30))

	auditPath := "example-audit/audit-2026-07-14.md"
	writeAuditDoc(t, root, auditPath)

	a := artifact{Path: auditPath}
	c := claim{
		Anchor:       exampleAuditAnchor,
		LastReviewed: "2026-07-14",
		Upstreams: []upstream{
			{Repo: "example-service", Globs: []string{"spike/example/**"}},
		},
	}

	// example-service#16 merges the day after the audit was reviewed, landing
	// exactly the artifact the CUT verdict said did not exist.
	mergedAt := reviewedAt.AddDate(0, 0, 2)
	commitFile(t, upstreamRepoDir, "spike/example/invariant_check.txt", "the balance invariant\n", mergedAt)

	today := mergedAt.AddDate(0, 0, 1)
	stale, reason := checkClaim(a, c, today, root)
	if !stale {
		t.Fatalf("claim should be STALE once the upstream artifact lands, got FRESH")
	}
	if !strings.Contains(reason, "invariant_check.txt") {
		t.Errorf("reason should name the upstream file that invalidated the claim, got %q", reason)
	}
}

func TestCheckClaim_MissingAnchorReportedNotSkipped(t *testing.T) {
	root := t.TempDir()
	auditPath := "example-audit/audit-2026-07-14.md"
	// Doc has been edited and no longer contains the exact verdict sentence.
	full := filepath.Join(root, auditPath)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte("# Example audit — item 9\n\nRewritten content.\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	a := artifact{Path: auditPath}
	c := claim{Anchor: exampleAuditAnchor, LastReviewed: "2026-07-14"}

	stale, reason := checkClaim(a, c, time.Now(), root)
	if !stale {
		t.Fatal("claim with a missing anchor must be reported STALE, not silently skipped")
	}
	if !strings.Contains(reason, "anchor not found") {
		t.Errorf("reason should say the anchor was not found, got %q", reason)
	}
}

// TestRun_EndToEnd_ClaimStaleness exercises the full run() path (manifest →
// stdout → exit code) with a claims-bearing artifact, matching how a real
// freshness.yaml entry for a normative document like the example audit
// would be checked in CI.
func TestRun_EndToEnd_ClaimStaleness(t *testing.T) {
	root := t.TempDir()
	reviewedAt := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)
	newUpstreamRepo(t, root, "example-service", reviewedAt.AddDate(0, 0, -30))
	upstreamDir := filepath.Join(root, "example-service")

	auditPath := "example-audit/audit-2026-07-14.md"
	writeAuditDoc(t, root, auditPath)

	manifest := `artifacts:
  - path: example-audit/audit-2026-07-14.md
    last-reviewed: "2026-07-14"
    max-age-days: 0
    upstreams: []
    claims:
      - anchor: "THE CENTRAL CLAIM. No such artifact exists."
        last-reviewed: "2026-07-14"
        upstreams:
          - repo: "example-service"
            globs:
              - "spike/example/**"
`
	if err := os.WriteFile(filepath.Join(root, "freshness.yaml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}

	// Before example-service#16 merges: the manifest as a whole is FRESH.
	var stdout, stderr bytes.Buffer
	code := run([]string{"--root", root, "--as-of", "2026-07-15"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("before upstream merge: exit code = %d, want 0; stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "FRESH  claim") {
		t.Errorf("expected a FRESH claim line before the merge, got %q", stdout.String())
	}

	// example-service#16 merges, landing the artifact the CUT verdict denied.
	mergedAt := reviewedAt.AddDate(0, 0, 2)
	commitFile(t, upstreamDir, "spike/example/invariant_check.txt", "the balance invariant\n", mergedAt)

	stdout.Reset()
	stderr.Reset()
	code = run([]string{"--root", root, "--as-of", "2026-07-20"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("after upstream merge: exit code = %d, want 1 (claim staleness must fail the run); stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "STALE  claim") {
		t.Errorf("expected a STALE claim line after the merge, got %q", stdout.String())
	}
	if !strings.Contains(stdout.String(), "invariant_check.txt") {
		t.Errorf("expected the stale claim line to name the invalidating upstream file, got %q", stdout.String())
	}
	// The whole-document line stays FRESH — this is exactly the failure mode
	// #50 describes: the document's OWN review date says nothing moved, only
	// the claim-level check catches that one verdict inside it went stale.
	if !strings.Contains(stdout.String(), "FRESH  "+auditPath) {
		t.Errorf("whole-document line should still read FRESH (only the claim went stale), got %q", stdout.String())
	}
}
