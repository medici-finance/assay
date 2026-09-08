package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// loadTestVocab loads the fixture capability set; fails the test on error.
func loadTestVocab(t *testing.T) map[string]bool {
	t.Helper()
	v, err := loadVocabulary(filepath.Join("testdata", "vocab.md"))
	if err != nil {
		t.Fatalf("loadVocabulary(fixture): %v", err)
	}
	return v
}

// writeBody writes a single skill body into a fresh skills tree rooted at a
// temp dir and returns the skills dir.
func writeBody(t *testing.T, skill, body string) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, skill)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

const cleanBody = "# Fixture\n\nDispatch a worker agent (`capability:dispatch-worker`); resume it via " +
	"`capability:message-agent`. Its own workspace (`capability:isolate-workspace`), NEVER the shared checkout.\n"

func TestLoadVocabulary(t *testing.T) {
	v := loadTestVocab(t)
	want := []string{"dispatch-worker", "message-agent", "isolate-workspace", "invoke-skill", "session-notifications"}
	if len(v) != len(want) {
		t.Fatalf("vocab size = %d, want %d (%v)", len(v), len(want), v)
	}
	for _, w := range want {
		if !v[w] {
			t.Errorf("vocab missing %q", w)
		}
	}
}

func TestLoadVocabulary_ThreeState(t *testing.T) {
	// Absent marker → could-not-check (error), never an empty-but-passing set.
	if _, err := loadVocabulary(filepath.Join("testdata", "vocab-nomarker.md")); err == nil {
		t.Error("no-marker README returned nil error; want could-not-check")
	}
	// Empty block → could-not-check.
	if _, err := loadVocabulary(filepath.Join("testdata", "vocab-empty.md")); err == nil {
		t.Error("empty-block README returned nil error; want could-not-check")
	}
	// Unreadable file → could-not-check.
	if _, err := loadVocabulary(filepath.Join("testdata", "does-not-exist.md")); err == nil {
		t.Error("missing README returned nil error; want could-not-check")
	}
}

func TestLoadBanned(t *testing.T) {
	banned, err := loadBanned()
	if err != nil {
		t.Fatalf("loadBanned: %v", err)
	}
	if len(banned) == 0 {
		t.Fatal("loadBanned returned zero tokens")
	}
	// Every token must carry a reason — the config's contract.
	for _, bt := range banned {
		if bt.reason == "" {
			t.Errorf("banned token %q has no reason", bt.token)
		}
	}
}

func TestCheckBodies_ShippedFixtureIsClean(t *testing.T) {
	v := loadTestVocab(t)
	banned, err := loadBanned()
	if err != nil {
		t.Fatal(err)
	}
	violations, err := checkBodies(filepath.Join("testdata", "skills"), v, banned)
	if err != nil {
		t.Fatalf("could-not-check on clean fixture: %v", err)
	}
	if len(violations) != 0 {
		t.Errorf("clean fixture reported %d violation(s):\n%s", len(violations), strings.Join(violations, "\n"))
	}
}

// Each banned token class must go red individually.
func TestCheckBodies_EachBannedTokenRedIndividually(t *testing.T) {
	v := loadTestVocab(t)
	banned, err := loadBanned()
	if err != nil {
		t.Fatal(err)
	}
	for _, bt := range banned {
		t.Run(bt.token, func(t *testing.T) {
			body := cleanBody + "\nPlanted violation: use " + bt.token + " here.\n"
			skillsDir := writeBody(t, "adopt", body)
			violations, err := checkBodies(skillsDir, v, banned)
			if err != nil {
				t.Fatalf("could-not-check: %v", err)
			}
			if len(violations) == 0 {
				t.Fatalf("banned token %q was not caught", bt.token)
			}
			joined := strings.Join(violations, "\n")
			if !strings.Contains(joined, bt.token) {
				t.Errorf("violation output does not name token %q:\n%s", bt.token, joined)
			}
			if !strings.Contains(joined, "adopt") {
				t.Errorf("violation output does not name the skill file:\n%s", joined)
			}
		})
	}
}

func TestCheckBodies_UnknownCapabilityIsRed(t *testing.T) {
	v := loadTestVocab(t)
	banned, err := loadBanned()
	if err != nil {
		t.Fatal(err)
	}
	body := cleanBody + "\nBroken: fan out via `capability:broadcast-message` here.\n"
	skillsDir := writeBody(t, "worker-desk", body)
	violations, err := checkBodies(skillsDir, v, banned)
	if err != nil {
		t.Fatalf("could-not-check: %v", err)
	}
	joined := strings.Join(violations, "\n")
	if !strings.Contains(joined, "broadcast-message") {
		t.Errorf("unknown capability not caught / not named:\n%s", joined)
	}
}

func TestCheckBodies_PlainWordAgentIsNotBanned(t *testing.T) {
	// The other direction: ordinary prose using "agent"/"task" must not trip.
	v := loadTestVocab(t)
	banned, err := loadBanned()
	if err != nil {
		t.Fatal(err)
	}
	body := "# Fixture\n\nDispatch a worker agent for the task; the reviewer agent works read-only.\n" +
		"Use `capability:dispatch-worker`.\n"
	skillsDir := writeBody(t, "market-intelligence", body)
	violations, err := checkBodies(skillsDir, v, banned)
	if err != nil {
		t.Fatal(err)
	}
	if len(violations) != 0 {
		t.Errorf("plain-prose body flagged:\n%s", strings.Join(violations, "\n"))
	}
}

func TestCheckBodies_CouldNotCheck_EmptyDir(t *testing.T) {
	v := loadTestVocab(t)
	banned, _ := loadBanned()
	empty := t.TempDir()
	if _, err := checkBodies(empty, v, banned); err == nil {
		t.Error("empty skills dir returned nil error; want could-not-check")
	}
}

func TestCheckBindings_ShippedFixtureIsClean(t *testing.T) {
	v := loadTestVocab(t)
	violations, err := checkBindings(filepath.Join("testdata", "refs"), filepath.Join("testdata", "skills"), v)
	if err != nil {
		t.Fatalf("could-not-check on clean fixture: %v", err)
	}
	if len(violations) != 0 {
		t.Errorf("clean bindings fixture reported %d violation(s):\n%s", len(violations), strings.Join(violations, "\n"))
	}
}

// copyRefs copies the reference fixtures into a temp dir, applying a per-file
// line transform, and returns the temp refs dir.
func copyRefs(t *testing.T, transform func(name, line string) (string, bool)) string {
	t.Helper()
	src := filepath.Join("testdata", "refs")
	dst := t.TempDir()
	entries, err := os.ReadDir(src)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		raw, err := os.ReadFile(filepath.Join(src, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		var kept []string
		for _, ln := range strings.Split(string(raw), "\n") {
			out, keep := transform(e.Name(), ln)
			if keep {
				kept = append(kept, out)
			}
		}
		if err := os.WriteFile(filepath.Join(dst, e.Name()), []byte(strings.Join(kept, "\n")), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dst
}

func TestCheckBindings_MissingCapabilityIsRed(t *testing.T) {
	v := loadTestVocab(t)
	// Drop the dispatch-worker binding line from codex.md only.
	refs := copyRefs(t, func(name, line string) (string, bool) {
		if name == "codex.md" && strings.Contains(line, "dispatch-worker") {
			return "", false
		}
		return line, true
	})
	violations, err := checkBindings(refs, filepath.Join("testdata", "skills"), v)
	if err != nil {
		t.Fatalf("could-not-check: %v", err)
	}
	joined := strings.Join(violations, "\n")
	if !strings.Contains(joined, "dispatch-worker") || !strings.Contains(joined, "codex.md") {
		t.Errorf("missing capability not caught / not named:\n%s", joined)
	}
}

func TestCheckBindings_MissingSkillCellIsRed(t *testing.T) {
	v := loadTestVocab(t)
	// Drop beta's degradation row from claude-code.md only.
	refs := copyRefs(t, func(name, line string) (string, bool) {
		if name == "claude-code.md" && strings.Contains(line, "`beta`") {
			return "", false
		}
		return line, true
	})
	violations, err := checkBindings(refs, filepath.Join("testdata", "skills"), v)
	if err != nil {
		t.Fatalf("could-not-check: %v", err)
	}
	joined := strings.Join(violations, "\n")
	if !strings.Contains(joined, "beta") || !strings.Contains(joined, "claude-code.md") {
		t.Errorf("missing skill cell not caught / not named:\n%s", joined)
	}
}

// Row-3a shape: references copied away from their sibling skills/, one
// capability stripped. Closure must still fail and name the capability rather
// than being masked by the absent roster.
func TestCheckBindings_ClosureFailsWithoutRoster(t *testing.T) {
	v := loadTestVocab(t)
	refs := copyRefs(t, func(name, line string) (string, bool) {
		if name == "codex.md" && strings.Contains(line, "dispatch-worker") {
			return "", false
		}
		return line, true
	})
	// Point skillsDir at a nonexistent sibling, as the /tmp copy would have.
	absentSkills := filepath.Join(refs, "..", "skills")
	violations, err := checkBindings(refs, absentSkills, v)
	if err != nil {
		t.Fatalf("closure failure was masked as could-not-check: %v", err)
	}
	joined := strings.Join(violations, "\n")
	if !strings.Contains(joined, "dispatch-worker") {
		t.Errorf("stripped capability not named without a roster:\n%s", joined)
	}
}

// With a clean closure but an unreadable roster, the cell dimension is
// could-not-check (exit 2), never a silent pass.
func TestCheckBindings_CleanClosureAbsentRoster_CouldNotCheck(t *testing.T) {
	v := loadTestVocab(t)
	refs := copyRefs(t, func(name, line string) (string, bool) { return line, true })
	absentSkills := filepath.Join(refs, "..", "skills")
	if _, err := checkBindings(refs, absentSkills, v); err == nil {
		t.Error("clean closure with absent roster returned nil error; want could-not-check")
	}
}

func TestCheckBindings_CouldNotCheck_EmptyRefsDir(t *testing.T) {
	v := loadTestVocab(t)
	empty := t.TempDir()
	if _, err := checkBindings(empty, filepath.Join("testdata", "skills"), v); err == nil {
		t.Error("empty refs dir returned nil error; want could-not-check")
	}
}

// The run() entry point returns the three documented exit codes.
func TestRun_ExitCodes(t *testing.T) {
	vocab := filepath.Join("testdata", "vocab.md")
	cases := []struct {
		name string
		args []string
		want int
	}{
		{"bodies-clean", []string{"--vocab", vocab, "bodies", filepath.Join("testdata", "skills")}, exitClean},
		{"bindings-clean", []string{"--vocab", vocab, "bindings", filepath.Join("testdata", "refs")}, exitClean},
		{"unknown-mode", []string{"--vocab", vocab, "sideways", "x"}, exitUsage},
		{"bad-vocab", []string{"--vocab", filepath.Join("testdata", "vocab-nomarker.md"), "bodies", filepath.Join("testdata", "skills")}, exitCannot},
		{"missing-args", []string{"--vocab", vocab, "bodies"}, exitUsage},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := run(tc.args); got != tc.want {
				t.Errorf("run(%v) = %d, want %d", tc.args, got, tc.want)
			}
		})
	}
}
