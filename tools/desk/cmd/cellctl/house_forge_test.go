package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// wantGhNA is the k8s gitlab arm's gh-config row, verbatim: a gitlab house cell must report the same.
const wantGhNA = "gh config link — not applicable on a gitlab cell (GitLab custody is the role token store)"

// houseForgeWorld scaffolds nothing: it points the process at a fresh HOME (with NO gh config),
// an operator config home holding a roster, a cells root and a git checkout carrying
// docs/streams, and returns the checkout and the cells root. The `new` and `check` calls below
// run in process against it (inproc), exactly as the binary would.
func houseForgeWorld(t *testing.T) (repo, cells string) {
	t.Helper()
	tmp := t.TempDir()
	home, cfg := filepath.Join(tmp, "home"), filepath.Join(tmp, "cfg")
	repo, cells = filepath.Join(tmp, "repo"), filepath.Join(tmp, "cells")
	for _, p := range []string{home, cfg, cells, filepath.Join(repo, "docs", "streams")} {
		must(t, os.MkdirAll(p, 0o755))
	}
	must(t, os.WriteFile(filepath.Join(cfg, "roster.env"), nil, 0o600))
	must(t, os.WriteFile(filepath.Join(home, ".gitconfig"), []byte("[user]\n"), 0o644))
	gitInit(t, repo)
	t.Setenv("HOME", home)
	t.Setenv("ASSAY_CONFIG_HOME", cfg)
	t.Setenv("CELLS_ROOT", cells)
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("GH_CONFIG_DIR", "")
	return repo, cells
}

func gitInit(t *testing.T, repo string) {
	t.Helper()
	if out, err := exec.Command("git", "init", "-q", repo).CombinedOutput(); err != nil {
		t.Skipf("git init: %v %s", err, out)
	}
}

// houseCheckOut scaffolds a github house cell, rewrites its CELL_FORGE to forge (so the check is
// proven independently of what `new` writes), and returns `cellctl check`'s stdout.
func houseCheckOut(t *testing.T, forge string) string {
	t.Helper()
	repo, cells := houseForgeWorld(t)
	if code := inproc("new", "h", "--kind", "house", "--repo", repo, "--roots", "o/r="+repo); code != 0 {
		t.Fatalf("new exited %d", code)
	}
	envfile := filepath.Join(cells, "h", "cell.env")
	raw, err := os.ReadFile(envfile)
	must(t, err)
	body := strings.Replace(string(raw), "CELL_FORGE=github\n", "CELL_FORGE="+forge+"\n", 1)
	must(t, os.WriteFile(envfile, []byte(body+"CELL_HARNESS=codex\n"), 0o600))
	return captureStdout(t, func() { _ = inproc("check", "h") })
}

// regression: #4817
func TestHouseCheckGitlabGhNA(t *testing.T) {
	out := houseCheckOut(t, "gitlab")
	if !strings.Contains(out, "forge=gitlab") {
		t.Fatalf("the cell did not load as gitlab:\n%s", out)
	}
	if strings.Contains(out, "MISS  gh config") {
		t.Errorf("a gitlab house cell reports the gh config link as a MISS:\n%s", out)
	}
	if !strings.Contains(out, "n/a   "+wantGhNA) {
		t.Errorf("a gitlab house cell lacks the gh-config n/a row:\n%s", out)
	}
}

// regression: #4817
func TestHouseCheckGithubGhMiss(t *testing.T) {
	out := houseCheckOut(t, "github")
	if !strings.Contains(out, "MISS  gh config linked:") {
		t.Errorf("a github house cell without a gh config must still MISS it:\n%s", out)
	}
	if strings.Contains(out, wantGhNA) {
		t.Errorf("a github house cell reports the gitlab n/a row:\n%s", out)
	}
}

// regression: #4817
func TestNewHouseGitlab(t *testing.T) {
	repo, cells := houseForgeWorld(t)
	// A gh config IS present on the host: a gitlab house cell must still not link it.
	must(t, os.MkdirAll(filepath.Join(os.Getenv("HOME"), ghConfigRelPath), 0o755))
	if code := inproc("new", "g", "--kind", "house", "--forge", "gitlab", "--group", "example-group",
		"--repo", repo, "--roots", "o/r="+repo); code != 0 {
		t.Fatalf("new exited %d", code)
	}
	d := filepath.Join(cells, "g")
	raw, err := os.ReadFile(filepath.Join(d, "cell.env"))
	must(t, err)
	env := string(raw)
	if strings.Contains(env, "CELL_FORGE=github") || !strings.Contains(env, "\nCELL_FORGE=gitlab\n") {
		t.Errorf("a gitlab house cell.env must carry CELL_FORGE=gitlab, not github:\n%s", env)
	}
	for _, want := range []string{"\nGITLAB_GROUP=example-group\n", "\nFORGE_API_BASE=https://gitlab.com/api/v4\n"} {
		if !strings.Contains(env, want) {
			t.Errorf("gitlab house cell.env lacks %q:\n%s", strings.TrimSpace(want), env)
		}
	}
	if _, err := os.Lstat(filepath.Join(d, "home", ghConfigRelPath)); err == nil {
		t.Error("a gitlab house cell linked the gh config")
	}
	if !exists(filepath.Join(d, "home", ".gitconfig")) {
		t.Error("a gitlab house cell must still link the gitconfig")
	}
}

// regression: #4817
func TestNewHouseGithubLinksGh(t *testing.T) {
	repo, cells := houseForgeWorld(t)
	must(t, os.MkdirAll(filepath.Join(os.Getenv("HOME"), ghConfigRelPath), 0o755))
	if code := inproc("new", "h", "--kind", "house", "--repo", repo, "--roots", "o/r="+repo); code != 0 {
		t.Fatalf("new exited %d", code)
	}
	if _, err := os.Lstat(filepath.Join(cells, "h", "home", ghConfigRelPath)); err != nil {
		t.Errorf("a github house cell must still link the gh config: %v", err)
	}
}

// regression: #4817
func TestNewHouseBadForge(t *testing.T) {
	repo, cells := houseForgeWorld(t)
	stderr := captureStderr(t, func() {
		if code := inproc("new", "b", "--kind", "house", "--forge", "bitbucket", "--repo", repo, "--roots", "o/r="+repo); code == 0 {
			t.Error("new accepted an unknown forge on a house cell")
		}
	})
	if !strings.Contains(stderr, "--forge must be github or gitlab") {
		t.Errorf("unknown-forge refusal does not name the flag: %q", stderr)
	}
	if exists(filepath.Join(cells, "b")) {
		t.Error("a refused house scaffold left a cell behind")
	}
}
