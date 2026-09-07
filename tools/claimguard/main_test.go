package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTemp(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestRun_UnresolvedClaim_ExitsNonZero(t *testing.T) {
	dir := t.TempDir()
	f := writeTemp(t, dir, "claim.md", "Our rival's VerdictCI has no equal, with SLAW support unmatched anywhere.\n")

	var out, errOut bytes.Buffer
	code := run([]string{f}, &out, &errOut)

	if code != 1 {
		t.Fatalf("expected exit 1, got %d; stdout=%s stderr=%s", code, out.String(), errOut.String())
	}
	if !strings.Contains(out.String(), "VerdictCI") {
		t.Errorf("expected VerdictCI in output, got: %s", out.String())
	}
	if !strings.Contains(out.String(), "SLAW") {
		t.Errorf("expected SLAW in output, got: %s", out.String())
	}
}

func TestRun_ResolvedClaim_ExitsZero(t *testing.T) {
	dir := t.TempDir()
	f := writeTemp(t, dir, "claim.md",
		"Our rival's [VerdictCI](https://get-verdict.example) has SLAW support, see https://vendor.example/slaw for the spec.\n")

	var out, errOut bytes.Buffer
	code := run([]string{f}, &out, &errOut)

	if code != 0 {
		t.Fatalf("expected exit 0, got %d; stdout=%s stderr=%s", code, out.String(), errOut.String())
	}
}

func TestRun_AllowFile_Suppresses(t *testing.T) {
	dir := t.TempDir()
	f := writeTemp(t, dir, "claim.md", "Built on OurCoreEngine with SLAW compliance baked in.\n")
	allowFile := writeTemp(t, dir, "allow.txt", "# house terms\nOurCoreEngine\nSLAW\n")

	var out, errOut bytes.Buffer
	code := run([]string{"--allow-file", allowFile, f}, &out, &errOut)

	if code != 0 {
		t.Fatalf("expected exit 0 once both terms are allowlisted, got %d; stdout=%s stderr=%s", code, out.String(), errOut.String())
	}
}

func TestRun_DirectoryWalk_ScansMarkdownOnly(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "nested")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTemp(t, sub, "claim.md", "VerdictCI has no citation here.\n")
	writeTemp(t, dir, "ignore.txt", "VerdictCI mentioned in a non-markdown file, ignored.\n")

	var out, errOut bytes.Buffer
	code := run([]string{dir}, &out, &errOut)

	if code != 1 {
		t.Fatalf("expected exit 1 from the nested .md file, got %d; stdout=%s stderr=%s", code, out.String(), errOut.String())
	}
	if strings.Contains(out.String(), "ignore.txt") {
		t.Errorf("expected the .txt file to be skipped, got: %s", out.String())
	}
}

func TestRun_NoPaths_UsageError(t *testing.T) {
	var out, errOut bytes.Buffer
	code := run(nil, &out, &errOut)
	if code != 2 {
		t.Fatalf("expected exit 2 for missing args, got %d", code)
	}
}
