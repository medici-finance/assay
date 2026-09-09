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
	violations, skipped, err := checkBindings(filepath.Join("testdata", "refs"), filepath.Join("testdata", "skills"), v)
	if err != nil {
		t.Fatalf("could-not-check on clean fixture: %v", err)
	}
	if len(violations) != 0 {
		t.Errorf("clean bindings fixture reported %d violation(s):\n%s", len(violations), strings.Join(violations, "\n"))
	}
	// The fixture carries one declared non-matrix reference; the clean verdict
	// must be accompanied by the announcement, never a silent omission.
	if len(skipped) != 1 || !strings.Contains(strings.Join(skipped, "\n"), "neutral-mechanics.md") {
		t.Errorf("declared non-matrix reference not announced as skipped: %v", skipped)
	}
}

// writeRef drops one extra .md into a copy of the reference fixtures and returns
// the temp refs dir.
func refsPlus(t *testing.T, name, body string) string {
	t.Helper()
	refs := copyRefs(t, func(_, line string) (string, bool) { return line, true })
	if err := os.WriteFile(filepath.Join(refs, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return refs
}

const declaredNonMatrix = "# Neutral mechanics\n\n" +
	"<!-- assay:harnesslint non-matrix-reference — harness-neutral shell mechanics, not a per-harness capability binding -->\n\n" +
	"No capability resolves here and no skill has a cell here, on purpose.\n"

const undeclaredJunk = "# Undeclared junk reference\n\nno capability bindings here\n"

// (i) A reference that DECLARES itself non-matrix is excluded from both the
// closure and the degradation-cell dimension, and is announced.
//
// Fail-first: on the pre-change tool this file is eight violations (five
// unresolved capabilities in the fixture vocabulary + two missing skill cells,
// plus whatever the roster grows to) and checkBindings returns non-empty. See
// the PR body's `## Fail-first` for the red run.
func TestCheckBindings_DeclaredNonMatrixReferenceIsSkipped(t *testing.T) {
	v := loadTestVocab(t)
	refs := refsPlus(t, "zzz-neutral.md", declaredNonMatrix)
	violations, skipped, err := checkBindings(refs, filepath.Join("testdata", "skills"), v)
	if err != nil {
		t.Fatalf("could-not-check: %v", err)
	}
	if joined := strings.Join(violations, "\n"); strings.Contains(joined, "zzz-neutral.md") {
		t.Errorf("declared non-matrix reference still produced violations:\n%s", joined)
	}
	if len(violations) != 0 {
		t.Errorf("expected a clean run, got:\n%s", strings.Join(violations, "\n"))
	}
	joinedSkips := strings.Join(skipped, "\n")
	if !strings.Contains(joinedSkips, "zzz-neutral.md") {
		t.Errorf("skip was not announced: %v", skipped)
	}
	// The announcement carries the declared reason — a skip nobody can read the
	// justification for is the silent skip in a longer coat.
	if !strings.Contains(joinedSkips, "harness-neutral shell mechanics") {
		t.Errorf("skip announcement does not carry the declared reason: %v", skipped)
	}
}

// (ii) NARROWNESS — the guarantee that (i) did not broaden the guard. An
// UNdeclared reference in the same directory, with the same absence of bindings,
// is still fully checked and still red. The mutation that reddens this test is
// widening the skip (keying it on "has no capability bindings", or on a filename
// the tool knows) instead of on the explicit declaration.
func TestCheckBindings_UndeclaredReferenceStillChecked(t *testing.T) {
	v := loadTestVocab(t)
	refs := refsPlus(t, "zzz-undeclared.md", undeclaredJunk)
	violations, skipped, err := checkBindings(refs, filepath.Join("testdata", "skills"), v)
	if err != nil {
		t.Fatalf("could-not-check: %v", err)
	}
	joined := strings.Join(violations, "\n")
	if !strings.Contains(joined, "zzz-undeclared.md") {
		t.Fatalf("undeclared reference was not checked — the skip is too broad:\n%s", joined)
	}
	// Both dimensions must still fire on it, not just one.
	if !strings.Contains(joined, "does not resolve") {
		t.Errorf("closure dimension did not fire on the undeclared reference:\n%s", joined)
	}
	if !strings.Contains(joined, "no degradation cell") {
		t.Errorf("cell dimension did not fire on the undeclared reference:\n%s", joined)
	}
	if strings.Contains(strings.Join(skipped, "\n"), "zzz-undeclared.md") {
		t.Errorf("undeclared reference was announced as skipped: %v", skipped)
	}
}

// The two files side by side: declaring one out must not take the other with it.
func TestCheckBindings_DeclaredSkipDoesNotCoverItsNeighbour(t *testing.T) {
	v := loadTestVocab(t)
	refs := refsPlus(t, "zzz-neutral.md", declaredNonMatrix)
	if err := os.WriteFile(filepath.Join(refs, "zzz-undeclared.md"), []byte(undeclaredJunk), 0o644); err != nil {
		t.Fatal(err)
	}
	violations, skipped, err := checkBindings(refs, filepath.Join("testdata", "skills"), v)
	if err != nil {
		t.Fatalf("could-not-check: %v", err)
	}
	joined := strings.Join(violations, "\n")
	if strings.Contains(joined, "zzz-neutral.md") {
		t.Errorf("declared file was checked anyway:\n%s", joined)
	}
	if !strings.Contains(joined, "zzz-undeclared.md") {
		t.Errorf("undeclared neighbour was silenced by the declaration:\n%s", joined)
	}
	if len(skipped) != 2 { // the fixture's own neutral-mechanics.md plus zzz-neutral.md
		t.Errorf("skip announcement count = %d, want 2: %v", len(skipped), skipped)
	}
}

// A bare marker with no reason is could-not-check — the cheapest way to switch
// the guard off for a file must not be the one that works.
func TestCheckBindings_NonMatrixDeclarationRequiresReason(t *testing.T) {
	v := loadTestVocab(t)
	for _, tc := range []struct{ name, body string }{
		{"bare", "# X\n\n<!-- assay:harnesslint non-matrix-reference -->\n"},
		{"punctuation-only", "# X\n\n<!-- assay:harnesslint non-matrix-reference — -->\n"},
		{"unterminated", "# X\n\n<!-- assay:harnesslint non-matrix-reference — no close\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			refs := refsPlus(t, "zzz-bad-decl.md", tc.body)
			if _, _, err := checkBindings(refs, filepath.Join("testdata", "skills"), v); err == nil {
				t.Error("malformed declaration returned nil error; want could-not-check")
			}
		})
	}
}

// Declaring EVERY reference out leaves no matrix to check, which is
// could-not-check, never a clean sweep.
func TestCheckBindings_AllDeclaredIsCouldNotCheck(t *testing.T) {
	v := loadTestVocab(t)
	dir := t.TempDir()
	for _, n := range []string{"a.md", "b.md"} {
		if err := os.WriteFile(filepath.Join(dir, n), []byte(declaredNonMatrix), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if _, _, err := checkBindings(dir, filepath.Join("testdata", "skills"), v); err == nil {
		t.Error("all-declared refs dir returned nil error; want could-not-check")
	}
}

func TestNonMatrixDeclaration(t *testing.T) {
	if _, declared, err := nonMatrixDeclaration("# plain\n\nnothing here\n"); declared || err != nil {
		t.Errorf("undeclared body: declared=%v err=%v, want false/nil", declared, err)
	}
	reason, declared, err := nonMatrixDeclaration(declaredNonMatrix)
	if err != nil || !declared {
		t.Fatalf("declared body: declared=%v err=%v", declared, err)
	}
	if reason != "harness-neutral shell mechanics, not a per-harness capability binding" {
		t.Errorf("reason = %q", reason)
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
	violations, _, err := checkBindings(refs, filepath.Join("testdata", "skills"), v)
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
	violations, _, err := checkBindings(refs, filepath.Join("testdata", "skills"), v)
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
	violations, _, err := checkBindings(refs, absentSkills, v)
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
	if _, _, err := checkBindings(refs, absentSkills, v); err == nil {
		t.Error("clean closure with absent roster returned nil error; want could-not-check")
	}
}

func TestCheckBindings_CouldNotCheck_EmptyRefsDir(t *testing.T) {
	v := loadTestVocab(t)
	empty := t.TempDir()
	if _, _, err := checkBindings(empty, filepath.Join("testdata", "skills"), v); err == nil {
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
