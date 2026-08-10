package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
