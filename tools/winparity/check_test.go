package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeRoot materialises a repository root under t.TempDir() holding a Makefile
// with the given .PHONY targets and a scripts/build-windows.ps1 with the given
// declared parity targets. Passing a nil slice for either OMITS that file, to
// exercise the could-not-check paths. Everything lives under the temp dir, so
// this test reads nothing outside its own module.
func writeRoot(t *testing.T, makePhony, psTargets []string) string {
	t.Helper()
	root := t.TempDir()

	if makePhony != nil {
		mk := ".PHONY: " + strings.Join(makePhony, " ") + "\n\ndesk-build:\n\t@true\n"
		if err := os.WriteFile(filepath.Join(root, "Makefile"), []byte(mk), 0o644); err != nil {
			t.Fatalf("write Makefile: %v", err)
		}
	}

	if psTargets != nil {
		var b strings.Builder
		b.WriteString("param([string]$Target)\n")
		b.WriteString("# " + psBeginMarker + "\n")
		b.WriteString("$MakefileParityTargets = @(\n")
		for _, tgt := range psTargets {
			b.WriteString("    '" + tgt + "'\n")
		}
		b.WriteString(")\n")
		b.WriteString("# " + psEndMarker + "\n")
		dir := filepath.Join(root, "scripts")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir scripts: %v", err)
		}
		if err := os.WriteFile(filepath.Join(dir, "build-windows.ps1"), []byte(b.String()), 0o644); err != nil {
			t.Fatalf("write ps1: %v", err)
		}
	}
	return root
}

var canonicalTargets = []string{
	"desk-build", "desk-install", "desk-manifest", "desk-hook-install",
	"desk-test", "skillslint", "guardrail-sync", "paired-versions",
}

// TestCheck_InParityIsClean is the GREEN half of the fail-first pair: identical
// sets clear.
func TestCheck_InParityIsClean(t *testing.T) {
	root := writeRoot(t, canonicalTargets, canonicalTargets)
	var out bytes.Buffer
	if !Check(root, &out) {
		t.Fatalf("expected clean parity, got failure:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "OK") {
		t.Errorf("clean run did not report OK:\n%s", out.String())
	}
}

// TestCheck_MissingTargetIsDrift is the RED half: a target the Makefile declares
// but the ps1 omits — the exact "Windows fell behind" drift the guard exists for
// — must fail. Run against the in-parity fixture this same input is green
// (TestCheck_InParityIsClean), so the red is caused by the drift, not the setup.
func TestCheck_MissingTargetIsDrift(t *testing.T) {
	// ps1 omits "paired-versions" that the Makefile's .PHONY still names.
	psShort := canonicalTargets[:len(canonicalTargets)-1]
	root := writeRoot(t, canonicalTargets, psShort)
	var out bytes.Buffer
	if Check(root, &out) {
		t.Fatalf("expected drift failure for a target missing from the ps1, got clean:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "paired-versions") || !strings.Contains(out.String(), "MISSING") {
		t.Errorf("drift output did not name the missing target:\n%s", out.String())
	}
}

// TestCheck_ExtraTargetIsDrift: a target the ps1 declares but the Makefile lacks
// must also fail — parity is set EQUALITY, not subset.
func TestCheck_ExtraTargetIsDrift(t *testing.T) {
	psExtra := append(append([]string{}, canonicalTargets...), "windows-only-thing")
	root := writeRoot(t, canonicalTargets, psExtra)
	var out bytes.Buffer
	if Check(root, &out) {
		t.Fatalf("expected drift failure for an extra ps1 target, got clean:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "windows-only-thing") || !strings.Contains(out.String(), "ABSENT") {
		t.Errorf("drift output did not name the extra target:\n%s", out.String())
	}
}

// TestCheck_MissingMakefileIsCouldNotCheck: an unreadable source is reported as
// could-not-check and is NEVER rounded up to a pass.
func TestCheck_MissingMakefileIsCouldNotCheck(t *testing.T) {
	root := writeRoot(t, nil, canonicalTargets)
	var out bytes.Buffer
	if Check(root, &out) {
		t.Fatalf("expected could-not-check failure with no Makefile, got clean:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "could-not-check") {
		t.Errorf("missing Makefile was not reported as could-not-check:\n%s", out.String())
	}
}

// TestCheck_MissingPsIsCouldNotCheck: same, for the PowerShell side.
func TestCheck_MissingPsIsCouldNotCheck(t *testing.T) {
	root := writeRoot(t, canonicalTargets, nil)
	var out bytes.Buffer
	if Check(root, &out) {
		t.Fatalf("expected could-not-check failure with no ps1, got clean:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "could-not-check") {
		t.Errorf("missing ps1 was not reported as could-not-check:\n%s", out.String())
	}
}

// TestCheck_MissingMarkersIsCouldNotCheck: a ps1 present but with no parity
// marker block must not silently pass.
func TestCheck_MissingMarkersIsCouldNotCheck(t *testing.T) {
	root := t.TempDir()
	mk := ".PHONY: " + strings.Join(canonicalTargets, " ") + "\n"
	if err := os.WriteFile(filepath.Join(root, "Makefile"), []byte(mk), 0o644); err != nil {
		t.Fatalf("write Makefile: %v", err)
	}
	dir := filepath.Join(root, "scripts")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "build-windows.ps1"), []byte("param()\n# no markers here\n"), 0o644); err != nil {
		t.Fatalf("write ps1: %v", err)
	}
	var out bytes.Buffer
	if Check(root, &out) {
		t.Fatalf("expected could-not-check failure with no marker block, got clean:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "could-not-check") {
		t.Errorf("missing markers were not reported as could-not-check:\n%s", out.String())
	}
}

// TestMakefilePhony_HandlesLineContinuation: a `.PHONY` split across `\`
// continuations is parsed as one set.
func TestMakefilePhony_HandlesLineContinuation(t *testing.T) {
	root := t.TempDir()
	mk := ".PHONY: desk-build \\\n\tdesk-test \\\n\tskillslint\n"
	if err := os.WriteFile(filepath.Join(root, "Makefile"), []byte(mk), 0o644); err != nil {
		t.Fatalf("write Makefile: %v", err)
	}
	got, err := makefilePhonyTargets(filepath.Join(root, "Makefile"))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	for _, want := range []string{"desk-build", "desk-test", "skillslint"} {
		if !got[want] {
			t.Errorf("continuation target %q not parsed; got %v", want, got)
		}
	}
	if len(got) != 3 {
		t.Errorf("expected 3 targets across continuations, got %d: %v", len(got), got)
	}
}
