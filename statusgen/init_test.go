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

	stream := initStreamName(dir)

	// The files it promises must exist — the three append-only registers
	// (FINDINGS/INTAKE/RETRO), the streams tree + starter stream, the channel-E
	// pin file, and the CI workflow.
	for _, rel := range []string{
		"docs/streams/README.md",
		"docs/streams/FINDINGS.md",
		"docs/streams/INTAKE.md",
		"docs/streams/RETRO.md",
		"docs/streams/" + stream + "/README.md",
		"docs/streams/" + stream + "/brief-01-first-brief.md",
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

// TestInitLeavesNoUnsubstitutedPlaceholder guards the templating itself: the
// scaffold is written by substituting one token, so a template that grows a new
// occurrence of it in a file the substitution does not reach would ship the raw
// token to an adopter.
func TestInitLeavesNoUnsubstitutedPlaceholder(t *testing.T) {
	dir := t.TempDir()
	if code := runInit(dir); code != 0 {
		t.Fatalf("runInit exit = %d, want 0", code)
	}
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if strings.Contains(string(raw), initStreamPlaceholder) {
			t.Errorf("%s still contains the raw %s placeholder", path, initStreamPlaceholder)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// TestInitStreamNameIsDerivedAndSanitised covers the derivation table: the
// identity comes from the TARGET directory's name, lowercased and reduced to the
// [a-z0-9-] shape statusgen accepts (the stream name must equal its directory
// name, and brief ids are `<stream>/NN`). The historical literal survives only as
// the fallback for a name that sanitises to nothing — a name we cannot derive is
// better than one we invent.
func TestInitStreamNameIsDerivedAndSanitised(t *testing.T) {
	for _, tc := range []struct{ dir, want string }{
		{"payments-api", "payments-api"},
		{"Payments_API", "payments-api"},
		{"My Repo!!", "my-repo"},
		{"--weird--", "weird"},
		{"repo.v2", "repo-v2"},
		{"---", initFallbackStream},     // sanitises to empty → fallback
		{"findings", "findings-stream"}, // reserved register name → not a stream dir
		{"intake", "intake-stream"},
	} {
		base := t.TempDir()
		target := filepath.Join(base, tc.dir)
		if err := os.MkdirAll(target, 0o755); err != nil {
			t.Fatal(err)
		}
		if got := initStreamName(target); got != tc.want {
			t.Errorf("initStreamName(%q) = %q, want %q", tc.dir, got, tc.want)
		}
	}
}

// TestInitTwiceThenMultiRootDoesNotCollide is the regression this derivation
// exists for.
//
// Scaffolding every repo with the same literal stream identity makes any two
// freshly-init'd repos collide BY CONSTRUCTION — and a duplicate stream identity
// across roots is a hard error in a multi-root run, so the out-of-the-box
// scaffold broke the very feature it was scaffolding for, before an adopter had
// authored a single line. Init two repos, board them together, and the run must
// be clean.
func TestInitTwiceThenMultiRootDoesNotCollide(t *testing.T) {
	base := t.TempDir()
	a := filepath.Join(base, "payments-api")
	b := filepath.Join(base, "ledger-service")
	for _, dir := range []string{a, b} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if code := runInit(dir); code != 0 {
			t.Fatalf("runInit(%s) exit = %d, want 0", dir, code)
		}
	}

	if initStreamName(a) == initStreamName(b) {
		t.Fatalf("two freshly-init'd repos scaffolded the same stream identity %q", initStreamName(a))
	}

	problems, implicated := crossRootProblems([]string{a, b})
	if len(problems) != 0 {
		t.Errorf("two freshly-init'd repos collide across roots: %v", problems)
	}
	if len(implicated) != 0 {
		t.Errorf("two freshly-init'd repos were implicated in a collision: %v", implicated)
	}

	var code int
	stderr := captureStderr(t, func() { code = runRoots([]string{a, b}, "write", nil, nil, "") })
	if code != 0 {
		t.Fatalf("multi-root write over two freshly-init'd repos exited %d\n%s", code, stderr)
	}
	for _, dir := range []string{a, b} {
		board, err := os.ReadFile(filepath.Join(dir, "STATUS.md"))
		if err != nil {
			t.Fatalf("no board written for %s: %v", dir, err)
		}
		if !strings.Contains(string(board), initStreamName(dir)) {
			t.Errorf("%s's board does not carry its own stream %q", dir, initStreamName(dir))
		}
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
