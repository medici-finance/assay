package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeReference materialises plugins/assay/references/<name> under root with body.
func writeReference(t *testing.T, root, name, body string) {
	t.Helper()
	d := filepath.Join(root, "plugins", "assay", "references")
	if err := os.MkdirAll(d, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", d, err)
	}
	if err := os.WriteFile(filepath.Join(d, name), []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

func TestAmbientAuth_FlagsBareGhAuthToken(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, root, "worker-desk", "---\nname: worker-desk\ndescription: x\n---\n\nRun `GH_TOKEN=$(gh auth token) gh pr list`.\n")
	writeReference(t, root, "clean.md", "# clean\n")

	checked, issues, err := AmbientAuthIssues(root)
	if err != nil {
		t.Fatalf("unexpected could-not-check: %v", err)
	}
	if checked != 2 {
		t.Fatalf("checked = %d, want 2", checked)
	}
	if len(issues) != 1 || !strings.HasSuffix(issues[0].Path, "worker-desk/SKILL.md") || !strings.Contains(issues[0].Msg, "line 6:") {
		t.Fatalf("want one issue at worker-desk/SKILL.md line 6, got %+v", issues)
	}
}

func TestAmbientAuth_FlagsSkillOwnReferences(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, root, "pr-review-desk", "---\nname: pr-review-desk\ndescription: x\n---\n")
	d := filepath.Join(root, "plugins", "assay", "skills", "pr-review-desk", "references")
	if err := os.MkdirAll(d, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(d, "filing.md"), []byte("export GH_TOKEN=\"$(gh auth token)\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeReference(t, root, "clean.md", "# clean\n")

	_, issues, err := AmbientAuthIssues(root)
	if err != nil {
		t.Fatalf("unexpected could-not-check: %v", err)
	}
	if len(issues) != 1 || !strings.HasSuffix(issues[0].Path, "references/filing.md") {
		t.Fatalf("want one issue in the skill's own references/, got %+v", issues)
	}
}

func TestAmbientAuth_ExemptsCanonicalBlockAndDriverForm(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, root, "ask-decision", "---\nname: ask-decision\ndescription: x\n---\n\n"+
		"Driver runs: `t=\"$(gh auth token --user <driver-login>)\"`.\n")
	writeReference(t, root, "desk-shell.md", "# shell\n\n"+
		"<!-- BEGIN forge cli as the role -->\n"+
		"Never `gh auth token`: that is a human's login.\n"+
		"<!-- END forge cli as the role -->\n")

	_, issues, err := AmbientAuthIssues(root)
	if err != nil {
		t.Fatalf("unexpected could-not-check: %v", err)
	}
	if len(issues) != 0 {
		t.Fatalf("exempt forms flagged: %+v", issues)
	}
}

func TestAmbientAuth_DriverFormDoesNotLaunderASecondCall(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, root, "ask-decision", "---\nname: ask-decision\ndescription: x\n---\n\n"+
		"`gh auth token --user <driver-login>` then `gh auth token`\n")
	writeReference(t, root, "clean.md", "# clean\n")

	_, issues, err := AmbientAuthIssues(root)
	if err != nil {
		t.Fatalf("unexpected could-not-check: %v", err)
	}
	if len(issues) != 1 {
		t.Fatalf("a bare call beside the driver form must be flagged, got %+v", issues)
	}
}

func TestAmbientAuth_TextAfterCanonicalBlockIsChecked(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, root, "the-desk", "---\nname: the-desk\ndescription: x\n---\n")
	writeReference(t, root, "desk-shell.md",
		"<!-- BEGIN forge cli as the role -->\nok `gh auth token`\n<!-- END forge cli as the role -->\n"+
			"later: `gh auth token`\n")

	_, issues, err := AmbientAuthIssues(root)
	if err != nil {
		t.Fatalf("unexpected could-not-check: %v", err)
	}
	if len(issues) != 1 || !strings.Contains(issues[0].Msg, "line 4:") {
		t.Fatalf("want one issue at line 4 (after END), got %+v", issues)
	}
}

func TestAmbientAuth_UnterminatedBlockIsAFinding(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, root, "the-desk", "---\nname: the-desk\ndescription: x\n---\n")
	writeReference(t, root, "desk-shell.md", "<!-- BEGIN forge cli as the role -->\n`gh auth token`\n")

	_, issues, err := AmbientAuthIssues(root)
	if err != nil {
		t.Fatalf("unexpected could-not-check: %v", err)
	}
	if len(issues) != 1 || !strings.Contains(issues[0].Msg, "no matching") {
		t.Fatalf("want an unterminated-block finding, got %+v", issues)
	}
}

func TestAmbientAuth_EmptyOrMissingTreeIsCouldNotCheck(t *testing.T) {
	if _, _, err := AmbientAuthIssues(t.TempDir()); err == nil {
		t.Fatal("missing tree reported clean; want could-not-check")
	}
	root := t.TempDir()
	for _, d := range ambientAuthDirs {
		if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(d)), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if _, _, err := AmbientAuthIssues(root); err == nil {
		t.Fatal("empty tree reported clean; want could-not-check")
	}
}

// TestAmbientAuth_RealTreeClean runs the check over THIS repo's plugin tree
// (two directories up), so a regression fails `go test` and not only the CLI.
func TestAmbientAuth_RealTreeClean(t *testing.T) {
	const repoRoot = "../.."
	if _, err := os.Stat(filepath.Join(repoRoot, "plugins", "assay", "references", "desk-shell.md")); err != nil {
		t.Fatalf("could-not-check: desk-shell.md is missing (%v); a check that read nothing proved nothing", err)
	}
	checked, issues, err := AmbientAuthIssues(repoRoot)
	if err != nil {
		t.Fatalf("could-not-check: %v", err)
	}
	if checked == 0 {
		t.Fatal("checked zero files")
	}
	for _, is := range issues {
		t.Errorf("%s: %s", is.Path, is.Msg)
	}
}
