package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestInitScaffoldsLintCleanTree verifies `statusgen init` creates a structure that
// statusgen itself accepts: after init, --lint (mode "lint") must return 0.
func TestInitScaffoldsLintCleanTree(t *testing.T) {
	dir := t.TempDir()

	if code := runInit(dir); code != 0 {
		t.Fatalf("runInit exit = %d, want 0", code)
	}

	// The files it promises must exist — the three append-only registers
	// (FINDINGS/INTAKE/RETRO), the streams tree + example stream, the channel-E
	// pin file, and the CI workflow.
	for _, rel := range []string{
		"docs/streams/README.md",
		"docs/streams/FINDINGS.md",
		"docs/streams/INTAKE.md",
		"docs/streams/RETRO.md",
		"docs/streams/example/README.md",
		"docs/streams/example/brief-01-first-brief.md",
		".assay-versions",
		".github/workflows/assay-statusgen.yml",
	} {
		if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(rel))); err != nil {
			t.Errorf("expected %s to exist: %v", rel, err)
		}
	}

	// The scaffolded CI workflow must be BOOTSTRAP-SAFE by construction: the regen
	// guard uses `git status --porcelain -- STATUS.md` (not `git diff`, which can't
	// see an untracked first STATUS.md), carries the [skip-status-regen] marker, and
	// documents that STATUS.md is generated. These are the exact greps
	// docs/adopting-assay.md § add-statusgen-ci verifies on.
	wf, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(".github/workflows/assay-statusgen.yml")))
	if err != nil {
		t.Fatalf("read workflow: %v", err)
	}
	for _, want := range []string{
		"git status --porcelain -- STATUS.md",
		"skip-status-regen",
		"STATUS.md is generated",
	} {
		if !strings.Contains(string(wf), want) {
			t.Errorf("scaffolded workflow missing bootstrap-safe marker %q", want)
		}
	}

	// The pin file must be in the channel-E shape and NOT ship a live/guessed
	// digest — a placeholder line the adopter fills, so a fresh repo can't silently
	// install an unpinned binary.
	pin, err := os.ReadFile(filepath.Join(dir, ".assay-versions"))
	if err != nil {
		t.Fatalf("read .assay-versions: %v", err)
	}
	for _, want := range []string{"statusgen-linux-amd64", "REPLACE_WITH_SHA256"} {
		if !strings.Contains(string(pin), want) {
			t.Errorf(".assay-versions missing %q; got:\n%s", want, pin)
		}
	}

	// The scaffolded tree must pass lint (no PROBLEMs) with default settings.
	if code := run(dir, "lint", nil, nil, ""); code != 0 {
		t.Errorf("lint of scaffolded tree exit = %d, want 0 (scaffold must be lint-clean)", code)
	}

	// End-to-end adopter flow: bare `statusgen --lint` (as wired through main())
	// must be green in the fresh repo — the auto-applied default budget must NOT
	// red-gate it just because there is no CLAUDE.md.
	specs := effectiveBudgetSpecs("lint", nil, dir)
	if len(specs) != 0 {
		t.Errorf("effectiveBudgetSpecs on a CLAUDE.md-less repo = %v, want none", specs)
	}
	if code := run(dir, "lint", specs, nil, ""); code != 0 {
		t.Errorf("bare --lint on a freshly-init'd repo exit = %d, want 0", code)
	}
}

// TestEffectiveBudgetSpecs covers the fresh-adopter carve-out: the auto default is
// dropped when CLAUDE.md is absent, kept when present, and explicit specs always win.
func TestEffectiveBudgetSpecs(t *testing.T) {
	empty := t.TempDir()
	if got := effectiveBudgetSpecs("lint", nil, empty); len(got) != 0 {
		t.Errorf("no CLAUDE.md: got %v, want none (default dropped)", got)
	}

	withClaude := t.TempDir()
	if err := os.WriteFile(filepath.Join(withClaude, "CLAUDE.md"), []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := effectiveBudgetSpecs("lint", nil, withClaude); len(got) != 1 || got[0] != defaultBudgetSpec {
		t.Errorf("with CLAUDE.md: got %v, want [%s]", got, defaultBudgetSpec)
	}

	// An explicit --budget is always honored, present or not.
	explicit := []string{"docs/x.md:10"}
	if got := effectiveBudgetSpecs("lint", explicit, empty); len(got) != 1 || got[0] != explicit[0] {
		t.Errorf("explicit spec: got %v, want %v", got, explicit)
	}
}

// TestInitIsIdempotentAndNeverClobbers verifies a second run creates nothing and an
// existing file is left byte-for-byte unchanged.
func TestInitIsIdempotentAndNeverClobbers(t *testing.T) {
	dir := t.TempDir()
	if code := runInit(dir); code != 0 {
		t.Fatalf("first runInit exit = %d, want 0", code)
	}

	// Tamper with one file; a re-run must NOT overwrite it.
	readme := filepath.Join(dir, "docs", "streams", "README.md")
	sentinel := []byte("EDITED BY USER — must survive re-init\n")
	if err := os.WriteFile(readme, sentinel, 0o644); err != nil {
		t.Fatal(err)
	}

	if code := runInit(dir); code != 0 {
		t.Fatalf("second runInit exit = %d, want 0", code)
	}
	got, err := os.ReadFile(readme)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(sentinel) {
		t.Errorf("re-init clobbered an existing file; README.md = %q, want the user edit preserved", got)
	}
}
