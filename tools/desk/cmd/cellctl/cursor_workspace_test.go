package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func writeCursorWorkspace(t *testing.T, dir, role string) {
	t.Helper()
	for rel, body := range map[string]string{
		filepath.Join(".cursor", "skills", role, "SKILL.md"):       "fixture role skill",
		filepath.Join(".cursor", "references", "cursor.md"):        "fixture cursor binding",
		filepath.Join(".cursor", "references", "desk-shell.md"):    "fixture desk shell binding",
		filepath.Join(".cursor", "references", "tick-contract.md"): "fixture tick contract",
		"AGENTS.md": "existing user instructions\n<!-- assay:bindings:begin -->\nfixture resident rules\n<!-- assay:bindings:end -->\n",
	} {
		path := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
}

func TestCursorSourceInstallDoesNotAuthorizeRoleWorkspace(t *testing.T) {
	c := &Cell{Repo: t.TempDir(), Dir: t.TempDir(), Forge: "github", Roles: []string{"worker-desk"}}
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-c", "user.name=fixture", "-c", "user.email=fixture@example.invalid", "-c", "commit.gpgsign=false", "-C", c.Repo}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git fixture: %v: %s", err, out)
		}
	}
	git("init")
	git("commit", "--allow-empty", "-m", "fixture")
	writeCursorWorkspace(t, c.Repo, "worker-desk") // local installer output, uncommitted
	wt := filepath.Join(c.Dir, "worktrees", "worker-desk")
	git("worktree", "add", "--detach", wt, "HEAD")
	if err := cursorWorkspaceError(c.Repo, "worker-desk"); err != nil {
		t.Fatal(err)
	}
	if c.cursorSkillsDiscoverable() {
		t.Fatal("source-only installation passed actual-workspace discovery")
	}
	err := c.prepareCursorWorkspace("worker-desk", wt)
	if err == nil || !strings.Contains(err.Error(), "deskinstall --harness cursor --forge github --repo") || !strings.Contains(err.Error(), "worker-desk") {
		t.Fatalf("missing actual-workspace install did not provide remediation: %v", err)
	}
	writeCursorWorkspace(t, wt, "worker-desk")
	if err = c.prepareCursorWorkspace("worker-desk", wt); err != nil || !c.cursorSkillsDiscoverable() {
		t.Fatalf("actual-workspace install refused: %v", err)
	}
}

func TestCursorActualWorkspaceRequiresBindingsAndReferences(t *testing.T) {
	for _, rel := range []string{"AGENTS.md", filepath.Join(".cursor", "references", "cursor.md"), filepath.Join(".cursor", "references", "desk-shell.md"), filepath.Join(".cursor", "references", "tick-contract.md"), filepath.Join(".cursor", "skills", "worker-desk", "SKILL.md")} {
		t.Run(rel, func(t *testing.T) {
			wt := t.TempDir()
			writeCursorWorkspace(t, wt, "worker-desk")
			if err := os.Remove(filepath.Join(wt, rel)); err != nil {
				t.Fatal(err)
			}
			if err := cursorWorkspaceError(wt, "worker-desk"); err == nil {
				t.Fatal("incomplete workspace accepted")
			}
		})
	}
	for _, text := range []string{"user instructions without bindings", "<!-- assay:bindings:begin -->\n<!-- assay:bindings:end -->", "<!-- assay:bindings:end -->\nbody\n<!-- assay:bindings:begin -->"} {
		wt := t.TempDir()
		writeCursorWorkspace(t, wt, "worker-desk")
		if err := os.WriteFile(filepath.Join(wt, "AGENTS.md"), []byte(text), 0600); err != nil {
			t.Fatal(err)
		}
		if err := cursorWorkspaceError(wt, "worker-desk"); err == nil {
			t.Fatal("malformed bindings accepted")
		}
	}
}

func TestCursorCadenceKeepsSourceProjectDenials(t *testing.T) {
	c := &Cell{Repo: t.TempDir(), Dir: t.TempDir(), Cadence: &cadenceOptions{}}
	wt := filepath.Join(c.Dir, "worktrees", "worker-desk")
	writeCursorWorkspace(t, c.Repo, "worker-desk")
	writeCursorWorkspace(t, wt, "worker-desk")
	config := []byte(`{"permissions":{"allow":["Write(allowed.txt)"],"deny":["Write(denied.txt)","Shell(rm)"]}}`)
	if err := os.WriteFile(filepath.Join(c.Repo, ".cursor", "cli.json"), config, 0600); err != nil {
		t.Fatal(err)
	}
	actual := filepath.Join(wt, ".cursor", "cli.json")
	if c.prepareCursorWorkspace("worker-desk", wt) == nil {
		t.Fatal("source-local permission denials disappeared without refusal")
	}
	if err := os.WriteFile(actual, []byte(`{"permissions":{"deny":[]}}`), 0600); err != nil {
		t.Fatal(err)
	}
	if c.prepareCursorWorkspace("worker-desk", wt) == nil {
		t.Fatal("different project permission config accepted")
	}
	if err := os.WriteFile(actual, config, 0600); err != nil {
		t.Fatal(err)
	}
	if err := c.prepareCursorWorkspace("worker-desk", wt); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(actual); err != nil || string(got) != string(config) {
		t.Fatal("workspace preparation modified permissions")
	}
}
