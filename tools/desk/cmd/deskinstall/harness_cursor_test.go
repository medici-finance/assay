package main

import (
	"bytes"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// mustWrite writes content to path, creating parent directories as needed.
func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// cursorFixtureBundle builds a temp plugins/assay-shaped tree: a coverage
// roster (packaged names + optional exclusions), a SKILL.md + component.yaml
// for every name in packaged and diskOnly (to prove whole-directory copy,
// not just SKILL.md), a references/*.md set (unless omitReferences), the
// shared AGENTS.md fragment, and (unless omitRule) a cursor/assay.mdc rule.
// Returns the bundle root.
func cursorFixtureBundle(t *testing.T, packaged, diskOnly []string, excluded map[string]string, omitRule, omitReferences bool) string {
	t.Helper()
	bundle := t.TempDir()

	var b strings.Builder
	b.WriteString(cursorPackagingMarker + "\n")
	for _, name := range packaged {
		b.WriteString(name + "\n")
	}
	for name, reason := range excluded {
		b.WriteString(name + " :: EXCLUDED: " + reason + "\n")
	}
	b.WriteString("-->\n")
	mustWrite(t, filepath.Join(bundle, "cursor", "packaging.md"), b.String())

	if !omitRule {
		mustWrite(t, filepath.Join(bundle, "cursor", "assay.mdc"), "---\nalwaysApply: true\n---\nfixture rule\n")
	}

	allDisk := append([]string{}, packaged...)
	allDisk = append(allDisk, diskOnly...)
	for _, name := range allDisk {
		mustWrite(t, filepath.Join(bundle, "skills", name, "SKILL.md"), "---\nname: "+name+"\n---\nfixture skill "+name+"\n")
		mustWrite(t, filepath.Join(bundle, "skills", name, "component.yaml"), "id: "+name+"\n")
	}

	if !omitReferences {
		mustWrite(t, filepath.Join(bundle, "references", "desk-shell.md"), "fixture desk-shell reference\n")
		mustWrite(t, filepath.Join(bundle, "references", "other.md"), "fixture other reference\n")
	}

	mustWrite(t, filepath.Join(bundle, "codex", "AGENTS-assay.md"), "## fixture bindings\n\n1. fixture rule one\n2. fixture rule two\n")

	return bundle
}

// treeSnapshot maps every regular file under root (relative path) to its
// content, for a whole-tree byte-identical comparison across two runs.
func treeSnapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	snap := map[string]string{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		content, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		snap[rel] = string(content)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return snap
}

// TestHarnessCursorPlaces — Verify row 3. Every packaged name in the roster
// lands at .cursor/skills/<name>/SKILL.md, and no other name does; the whole
// skill directory (not just SKILL.md) is copied.
func TestHarnessCursorPlaces(t *testing.T) {
	packaged := []string{"adopt", "install", "worker-desk"}
	bundle := cursorFixtureBundle(t, packaged, nil, nil, false, false)
	repo := t.TempDir()

	if err := HarnessCursorPlace(HarnessOptions{BundleDir: bundle, RepoRoot: repo, Forge: "github"}, io.Discard); err != nil {
		t.Fatalf("place: %v", err)
	}

	for _, name := range packaged {
		if _, err := os.Stat(filepath.Join(repo, ".cursor", "skills", name, "SKILL.md")); err != nil {
			t.Errorf("expected .cursor/skills/%s/SKILL.md: %v", name, err)
		}
		if _, err := os.Stat(filepath.Join(repo, ".cursor", "skills", name, "component.yaml")); err != nil {
			t.Errorf("expected component.yaml alongside SKILL.md for %s (whole-dir copy): %v", name, err)
		}
	}

	entries, err := os.ReadDir(filepath.Join(repo, ".cursor", "skills"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != len(packaged) {
		var got []string
		for _, e := range entries {
			got = append(got, e.Name())
		}
		t.Errorf("placed skill set %v, want exactly %v", got, packaged)
	}
}

// TestHarnessCursorReferencesResolve — Verify row 4. Resolves each
// ../../references/*.md include found in a placed SKILL.md against the
// placed tree and asserts the target file exists — the exact failure mode
// both source docs warn about (skills-only copies leave the include dead).
func TestHarnessCursorReferencesResolve(t *testing.T) {
	bundle := cursorFixtureBundle(t, []string{"adopt"}, nil, nil, false, false)
	mustWrite(t, filepath.Join(bundle, "skills", "adopt", "SKILL.md"),
		"---\nname: adopt\n---\nSee [desk-shell](../../references/desk-shell.md) for mechanics.\n")
	repo := t.TempDir()

	if err := HarnessCursorPlace(HarnessOptions{BundleDir: bundle, RepoRoot: repo, Forge: "github"}, io.Discard); err != nil {
		t.Fatalf("place: %v", err)
	}

	skillMD := filepath.Join(repo, ".cursor", "skills", "adopt", "SKILL.md")
	body, err := os.ReadFile(skillMD)
	if err != nil {
		t.Fatal(err)
	}

	resolvedAny := false
	for _, line := range strings.Split(string(body), "\n") {
		idx := strings.Index(line, "](../../references/")
		if idx == -1 {
			continue
		}
		rest := line[idx+len("]("):]
		end := strings.Index(rest, ")")
		if end == -1 {
			t.Fatalf("malformed include line: %q", line)
		}
		includePath := rest[:end]
		resolved := filepath.Join(filepath.Dir(skillMD), includePath)
		if _, statErr := os.Stat(resolved); statErr != nil {
			t.Errorf("include %q from %s does not resolve: %v (resolved to %s)", includePath, skillMD, statErr, resolved)
		}
		resolvedAny = true
	}
	if !resolvedAny {
		t.Fatal("fixture SKILL.md carried no ../../references/*.md include to resolve — test is not exercising the failure mode")
	}
}

// TestHarnessCursorIdempotent — Verify row 5. Running the mode twice into
// one scratch repo produces byte-identical trees and exactly ONE bindings
// block in AGENTS.md.
func TestHarnessCursorIdempotent(t *testing.T) {
	bundle := cursorFixtureBundle(t, []string{"adopt", "install"}, nil, nil, false, false)
	repo := t.TempDir()
	opts := HarnessOptions{BundleDir: bundle, RepoRoot: repo, Forge: "gitlab"}

	if err := HarnessCursorPlace(opts, io.Discard); err != nil {
		t.Fatalf("first place: %v", err)
	}
	first := treeSnapshot(t, repo)

	if err := HarnessCursorPlace(opts, io.Discard); err != nil {
		t.Fatalf("second place: %v", err)
	}
	second := treeSnapshot(t, repo)

	if !reflect.DeepEqual(first, second) {
		t.Fatalf("re-run changed the tree:\nfirst:  %v\nsecond: %v", first, second)
	}

	agents, err := os.ReadFile(filepath.Join(repo, "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(string(agents), bindingsBegin); n != 1 {
		t.Errorf("AGENTS.md has %d bindings-begin markers after two runs, want exactly 1:\n%s", n, agents)
	}
}

// TestHarnessCursorCheckDrift — Verify row 6. Place, mutate one placed file,
// assert --check exits with that path named and the mutated file UNCHANGED
// after --check ran (--check must never write).
func TestHarnessCursorCheckDrift(t *testing.T) {
	bundle := cursorFixtureBundle(t, []string{"adopt"}, nil, nil, false, false)
	repo := t.TempDir()
	opts := HarnessOptions{BundleDir: bundle, RepoRoot: repo, Forge: "github"}
	if err := HarnessCursorPlace(opts, io.Discard); err != nil {
		t.Fatalf("place: %v", err)
	}

	mutated := filepath.Join(repo, ".cursor", "skills", "adopt", "SKILL.md")
	before, err := os.ReadFile(mutated)
	if err != nil {
		t.Fatal(err)
	}
	tampered := append(append([]byte{}, before...), []byte("\nTAMPERED\n")...)
	if err := os.WriteFile(mutated, tampered, 0o644); err != nil {
		t.Fatal(err)
	}

	findings, err := HarnessCursorCheck(opts)
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	wantRel := filepath.ToSlash(filepath.Join(".cursor", "skills", "adopt", "SKILL.md"))
	var got *DriftFinding
	for i := range findings {
		if findings[i].Path == wantRel {
			got = &findings[i]
		}
	}
	if got == nil {
		t.Fatalf("check did not name %s among findings: %v", wantRel, findings)
	}
	if got.Kind != driftContentDiffers {
		t.Errorf("finding for %s has kind %q, want %q", wantRel, got.Kind, driftContentDiffers)
	}

	after, err := os.ReadFile(mutated)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(after, tampered) {
		t.Fatalf("--check modified the mutated file: got %q, want unchanged %q", after, tampered)
	}
}

// TestHarnessCursorCheckClean — Verify row 7. --check exits clean (no
// findings) right after an unmutated placement.
func TestHarnessCursorCheckClean(t *testing.T) {
	bundle := cursorFixtureBundle(t, []string{"adopt"}, nil, nil, false, false)
	repo := t.TempDir()
	opts := HarnessOptions{BundleDir: bundle, RepoRoot: repo, Forge: "github"}
	if err := HarnessCursorPlace(opts, io.Discard); err != nil {
		t.Fatalf("place: %v", err)
	}
	findings, err := HarnessCursorCheck(opts)
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("check on an unmutated placement reported drift: %v", findings)
	}
}

// TestHarnessCursorRosterExcludes — Verify row 9. The roster decides the
// packaged set, never a glob: an excluded-but-present skill is skipped
// (run still succeeds); a packaged-but-absent skill REFUSES naming it.
func TestHarnessCursorRosterExcludes(t *testing.T) {
	t.Run("excluded skill present on disk is skipped, run succeeds", func(t *testing.T) {
		bundle := cursorFixtureBundle(t, []string{"adopt"}, []string{"the-desk"},
			map[string]string{"the-desk": "fixture reason"}, false, false)
		repo := t.TempDir()
		if err := HarnessCursorPlace(HarnessOptions{BundleDir: bundle, RepoRoot: repo, Forge: "github"}, io.Discard); err != nil {
			t.Fatalf("expected success with an excluded-but-present skill, got: %v", err)
		}
		if _, err := os.Stat(filepath.Join(repo, ".cursor", "skills", "the-desk")); !os.IsNotExist(err) {
			t.Errorf("excluded skill 'the-desk' was placed anyway (stat err=%v)", err)
		}
		if _, err := os.Stat(filepath.Join(repo, ".cursor", "skills", "adopt", "SKILL.md")); err != nil {
			t.Errorf("packaged skill 'adopt' was not placed: %v", err)
		}
	})

	t.Run("packaged skill absent from disk refuses naming it", func(t *testing.T) {
		bundle := cursorFixtureBundle(t, []string{"adopt", "ghost-skill"}, nil, nil, false, false)
		if err := os.RemoveAll(filepath.Join(bundle, "skills", "ghost-skill")); err != nil {
			t.Fatal(err)
		}
		repo := t.TempDir()
		err := HarnessCursorPlace(HarnessOptions{BundleDir: bundle, RepoRoot: repo, Forge: "github"}, io.Discard)
		if err == nil {
			t.Fatal("expected a refusal for a packaged skill with nothing on disk")
		}
		if !strings.Contains(err.Error(), "ghost-skill") {
			t.Errorf("refusal did not name the absent skill: %v", err)
		}
		if _, statErr := os.Stat(filepath.Join(repo, ".cursor")); !os.IsNotExist(statErr) {
			t.Errorf("a refused run still wrote under %s/.cursor", repo)
		}
	})
}

// TestHarnessCursorRefusesEscape — Verify row 10. A roster entry resolving
// outside --repo is refused and nothing is written; a symlink in the bundle
// source tree is refused the same way.
func TestHarnessCursorRefusesEscape(t *testing.T) {
	t.Run("escaping roster name", func(t *testing.T) {
		bundle := t.TempDir()
		var b strings.Builder
		b.WriteString(cursorPackagingMarker + "\n")
		b.WriteString("../../evil\n")
		b.WriteString("-->\n")
		mustWrite(t, filepath.Join(bundle, "cursor", "packaging.md"), b.String())
		mustWrite(t, filepath.Join(bundle, "codex", "AGENTS-assay.md"), "fixture\n")
		mustWrite(t, filepath.Join(bundle, "references", "x.md"), "fixture ref\n")

		repo := t.TempDir()
		err := HarnessCursorPlace(HarnessOptions{BundleDir: bundle, RepoRoot: repo, Forge: "github"}, io.Discard)
		if err == nil {
			t.Fatal("expected refusal for a roster entry that would escape --repo")
		}
		if _, statErr := os.Stat(filepath.Join(repo, ".cursor")); !os.IsNotExist(statErr) {
			t.Errorf("a refused escape attempt still wrote under %s/.cursor", repo)
		}
	})

	t.Run("symlink in skill source refuses", func(t *testing.T) {
		bundle := cursorFixtureBundle(t, []string{"adopt"}, nil, nil, false, false)
		outside := t.TempDir()
		mustWrite(t, filepath.Join(outside, "secret.txt"), "outside content\n")
		if err := os.Symlink(filepath.Join(outside, "secret.txt"), filepath.Join(bundle, "skills", "adopt", "link.txt")); err != nil {
			t.Skipf("symlinks unsupported in this environment: %v", err)
		}
		repo := t.TempDir()
		err := HarnessCursorPlace(HarnessOptions{BundleDir: bundle, RepoRoot: repo, Forge: "github"}, io.Discard)
		if err == nil {
			t.Fatal("expected refusal for a symlink inside the bundle skill source tree")
		}
		if _, statErr := os.Stat(filepath.Join(repo, ".cursor")); !os.IsNotExist(statErr) {
			t.Errorf("a refused symlink escape still wrote under %s/.cursor", repo)
		}
	})
}

// TestHarnessCursorForgeBindings — Verify row 11. --forge gitlab produces an
// AGENTS.md block naming glab/--forge gitlab and never the bare `gh` token;
// --forge github is the converse.
func TestHarnessCursorForgeBindings(t *testing.T) {
	bundle := cursorFixtureBundle(t, []string{"adopt"}, nil, nil, false, false)

	for _, tc := range []struct {
		forge      string
		wantSubstr []string
		wantAbsent []string
	}{
		{forge: "gitlab", wantSubstr: []string{"glab", "--forge gitlab"}, wantAbsent: []string{"`gh`"}},
		{forge: "github", wantSubstr: []string{"`gh`"}, wantAbsent: []string{"glab", "--forge gitlab"}},
	} {
		t.Run(tc.forge, func(t *testing.T) {
			repo := t.TempDir()
			if err := HarnessCursorPlace(HarnessOptions{BundleDir: bundle, RepoRoot: repo, Forge: tc.forge}, io.Discard); err != nil {
				t.Fatalf("place: %v", err)
			}
			agents, err := os.ReadFile(filepath.Join(repo, "AGENTS.md"))
			if err != nil {
				t.Fatal(err)
			}
			body := string(agents)
			for _, want := range tc.wantSubstr {
				if !strings.Contains(body, want) {
					t.Errorf("--forge %s: AGENTS.md missing %q:\n%s", tc.forge, want, body)
				}
			}
			for _, absent := range tc.wantAbsent {
				if strings.Contains(body, absent) {
					t.Errorf("--forge %s: AGENTS.md unexpectedly contains %q:\n%s", tc.forge, absent, body)
				}
			}
		})
	}
}

// TestHarnessCursorRefusesWithManifest — Verify row 12. --harness cursor
// supplied together with --manifest/--dest refuses naming both modes.
func TestHarnessCursorRefusesWithManifest(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{
		"--harness", "cursor", "--forge", "github", "--repo", t.TempDir(),
		"--manifest", "paired-versions.yaml", "--dest", t.TempDir(),
	}, &stdout, &stderr)
	if code != deskkit.ExitRefused {
		t.Fatalf("got exit %d, want %d (refused)", code, deskkit.ExitRefused)
	}
	msg := stderr.String()
	if !strings.Contains(msg, "--harness") {
		t.Errorf("refusal did not name the harness mode: %q", msg)
	}
	if !strings.Contains(msg, "--manifest") && !strings.Contains(msg, "--dest") {
		t.Errorf("refusal did not name the acquire-verify-place mode: %q", msg)
	}
}
