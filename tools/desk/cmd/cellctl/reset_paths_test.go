//go:build darwin || linux

package main

import (
	"encoding/json"
	"github.com/medici-finance/assay/tools/desk/internal/custodytest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func resetAlias(t *testing.T, target string) string {
	t.Helper()
	alias := filepath.Join(t.TempDir(), "alias")
	if err := os.Symlink(target, alias); err != nil {
		t.Fatal(err)
	}
	return alias
}

// regression: #2718
func TestResetPlanAliases(t *testing.T) {
	for _, aliasCell := range []bool{false, true} {
		t.Run(map[bool]string{false: "repository-alias", true: "cell-alias"}[aliasCell], func(t *testing.T) {
			base, err := filepath.EvalSymlinks(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			root := resetFixtureRepo(t, base, "repo")
			cell := filepath.Join(root, "cell")
			if err := os.Mkdir(cell, 0700); err != nil {
				t.Fatal(err)
			}
			alias := resetAlias(t, base)
			repoPath, cellPath := filepath.Join(alias, "repo"), cell
			if aliasCell {
				repoPath, cellPath = root, filepath.Join(alias, "repo", "cell")
			}
			c := &Cell{Name: "fixture", Dir: cellPath, Repo: repoPath, Env: envWith(map[string]string{})}
			if _, err := c.planReset(true); err == nil || !strings.Contains(err.Error(), "outside repository") {
				t.Fatalf("hard reset accepted cell nested through an alias: %v", err)
			}
			// A sibling whose name shares the prefix remains outside the repository.
			c.Dir = filepath.Join(alias, "repo-sibling")
			if err := os.Mkdir(c.Dir, 0700); err != nil {
				t.Fatal(err)
			}
			if _, err := c.planReset(true); err != nil {
				t.Fatalf("safe sibling refused: %v", err)
			}
		})
	}
}

// regression: #2718
func TestResetWorktreeAliases(t *testing.T) {
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	root := resetFixtureRepo(t, base, "repo")
	cell := filepath.Join(base, "cell")
	inside := filepath.Join(cell, "worktrees", "owned")
	outside := filepath.Join(cell, "worktrees-sibling", "foreign")
	resetFixtureGit(t, root, "worktree", "add", "-q", "-b", "owned", inside, "main")
	resetFixtureGit(t, root, "worktree", "add", "-q", "-b", "foreign", outside, "main")
	alias := resetAlias(t, base)
	for _, aliased := range []bool{false, true} {
		c := &Cell{Name: "fixture", Dir: cell, Repo: root, Env: envWith(map[string]string{})}
		if aliased {
			c.Dir = filepath.Join(alias, "cell")
			c.Repo = filepath.Join(alias, "repo")
		}
		for _, hard := range []bool{false, true} {
			p, err := c.planReset(hard)
			if err != nil {
				t.Fatal(err)
			}
			got := map[string]bool{}
			for _, wt := range p.Worktrees {
				got[wt.Branch] = true
			}
			if !got["owned"] || got["foreign"] != hard || len(got) != map[bool]int{false: 1, true: 2}[hard] {
				t.Fatalf("alias=%t hard=%t selected %v", aliased, hard, got)
			}
		}
	}
	// A linked checkout remains ineligible as a hard-reset primary through an alias.
	c := &Cell{Name: "fixture", Dir: cell, Repo: resetAlias(t, outside), Env: envWith(map[string]string{})}
	if _, err := c.planReset(true); err == nil || !strings.Contains(err.Error(), "primary checkout") {
		t.Fatalf("linked primary accepted: %v", err)
	}
}

// regression: #2718
// Fixtures built from t.TempDir() beneath a symlinked TMPDIR (macOS's /var ->
// /private/var) must plan exactly as they do beneath a resolved one, so a
// Linux runner sees the class a macOS default temp directory exposes.
func TestResetSymlinkedTMPDIR(t *testing.T) {
	real, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	alias := resetAlias(t, real)
	t.Setenv("TMPDIR", alias)
	fixture := func(t *testing.T) string {
		base := t.TempDir()
		if !strings.HasPrefix(base, alias+string(filepath.Separator)) {
			t.Fatalf("fixture not beneath symlinked TMPDIR %s: %s", alias, base)
		}
		return base
	}
	t.Run("hard-nested-cell", func(t *testing.T) {
		root := resetFixtureRepo(t, fixture(t), "one")
		c := &Cell{Name: "fixture", Dir: filepath.Join(root, "cell"), Repo: root, Env: envWith(map[string]string{"CELL_ROOTS": "example/one=" + root})}
		if err := os.Mkdir(c.Dir, 0700); err != nil {
			t.Fatal(err)
		}
		if _, err := c.planReset(true); err == nil || !strings.Contains(err.Error(), "outside repository") {
			t.Fatalf("hard reset accepted nested cell: %v", err)
		}
	})
	t.Run("soft-owned-worktree", func(t *testing.T) {
		base := fixture(t)
		root := resetFixtureRepo(t, base, "one")
		cell := filepath.Join(base, "cell")
		resetFixtureGit(t, root, "worktree", "add", "-q", "-b", "owned", filepath.Join(cell, "worktrees", "owned"), "main")
		c := &Cell{Name: "fixture", Dir: cell, Repo: root, Env: envWith(map[string]string{})}
		p, err := c.planReset(false)
		if err != nil {
			t.Fatal(err)
		}
		if len(p.Worktrees) != 1 || p.Worktrees[0].Branch != "owned" {
			t.Fatalf("soft reset selected %+v, want the cell-owned worktree", p.Worktrees)
		}
	})
}

// regression: #2718
// Execution must remove exactly the Git registrations the plan selected when
// the cell directory is spelled through a symlink; deleting the directories
// alone would strand locked registrations and their checked-out branches.
func TestResetExecAliasedCell(t *testing.T) {
	for _, hard := range []bool{false, true} {
		t.Run(map[bool]string{false: "soft", true: "hard"}[hard], func(t *testing.T) {
			c, _, _ := commsFixture(t)
			realCell, err := filepath.EvalSymlinks(filepath.Dir(c.Dir))
			if err != nil {
				t.Fatal(err)
			}
			realCell = filepath.Join(realCell, filepath.Base(c.Dir))
			c.Dir = filepath.Join(resetAlias(t, filepath.Dir(realCell)), filepath.Base(realCell))
			base, err := filepath.EvalSymlinks(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			root := resetFixtureRepo(t, base, "one")
			c.Repo = root
			c.Env.Put("CELL_ROOTS", "example/one="+root)
			owned := filepath.Join(realCell, "worktrees", "owned")
			foreign := filepath.Join(base, "foreign")
			resetFixtureGit(t, root, "worktree", "add", "-q", "-b", "owned", owned, "main")
			resetFixtureGit(t, root, "worktree", "lock", owned)
			resetFixtureGit(t, root, "worktree", "add", "-q", "-b", "foreign", foreign, "main")
			p, err := c.planReset(hard)
			if err != nil {
				t.Fatal(err)
			}
			if err = c.resetCell(p); err != nil {
				t.Fatal(err)
			}
			registered := map[string]bool{}
			for _, field := range strings.Split(resetFixtureGit(t, root, "worktree", "list", "--porcelain"), "\n") {
				if path, ok := strings.CutPrefix(field, "worktree "); ok {
					resolved, err := filepath.EvalSymlinks(path)
					if err != nil {
						resolved = path
					}
					registered[resolved] = true
				}
			}
			if registered[owned] {
				t.Fatalf("hard=%t left the cell-owned worktree registered: %v", hard, registered)
			}
			if registered[foreign] == hard {
				t.Fatalf("hard=%t foreign registration scope wrong: %v", hard, registered)
			}
		})
	}
}

// regression: #2718
func TestResetPlanPathErrors(t *testing.T) {
	base := t.TempDir()
	root := resetFixtureRepo(t, base, "repo")
	file := filepath.Join(base, "file")
	if err := os.WriteFile(file, []byte("sentinel"), 0600); err != nil {
		t.Fatal(err)
	}
	broken := resetAlias(t, filepath.Join(base, "absent"))
	loop := filepath.Join(base, "loop")
	if err := os.Symlink("loop", loop); err != nil {
		t.Fatal(err)
	}
	denied := filepath.Join(base, "denied")
	if err := os.Mkdir(denied, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(denied, "child"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(denied, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(denied, 0700) })
	for name, path := range map[string]string{"file": file, "file-child": filepath.Join(file, "child"), "broken-link": broken, "link-loop": loop, "unreadable": filepath.Join(denied, "child")} {
		t.Run(name, func(t *testing.T) {
			if name == "unreadable" {
				if _, err := os.Stat(path); err == nil {
					t.Skip("fixture permissions not enforced")
				}
			}
			c := &Cell{Name: "fixture", Dir: path, Repo: root, Env: envWith(map[string]string{})}
			for _, hard := range []bool{false, true} {
				if _, err := c.planReset(hard); err == nil {
					t.Fatalf("hard=%t accepted unusable cell path", hard)
				}
			}
		})
	}
	// The directory requirement holds on its own, not only through ENOTDIR on a child.
	if _, err := resetDirectoryPath(file); err == nil || !strings.Contains(err.Error(), "requires a directory") {
		t.Fatalf("non-directory accepted as reset directory: %v", err)
	}
	// Absent optional worktree storage is valid, including beneath an alias.
	cell := filepath.Join(base, "cell")
	if err := os.Mkdir(cell, 0700); err != nil {
		t.Fatal(err)
	}
	c := &Cell{Name: "fixture", Dir: resetAlias(t, cell), Repo: root, Env: envWith(map[string]string{})}
	if p, err := c.planReset(false); err != nil || len(p.Worktrees) != 0 {
		t.Fatalf("missing optional storage: %+v, %v", p, err)
	}
	if got, err := os.ReadFile(file); err != nil || string(got) != "sentinel" {
		t.Fatal("planning changed fixture", err)
	}
}

// regression: #2718
func TestResetPreflightAliases(t *testing.T) {
	for _, kind := range []string{"nested-repository", "nested-queue", "worktree-root-link"} {
		t.Run(kind, func(t *testing.T) {
			base, err := filepath.EvalSymlinks(custodytest.PrivateTempDir(t))
			if err != nil {
				t.Fatal(err)
			}
			root := resetFixtureRepo(t, base, "repo")
			c := &Cell{Name: "fixture", Kind: "house", Dir: filepath.Join(base, "cell"), Repo: root, Env: envWith(map[string]string{})}
			if err := os.Mkdir(c.Dir, 0700); err != nil {
				t.Fatal(err)
			}
			for _, key := range []string{"CODEX_HOME", "CLAUDE_CONFIG_DIR", "ASSAY_CONFIG_HOME", "GH_CONFIG_DIR", "CELLS_ROOT"} {
				c.Env.Put(key, filepath.Join(base, key))
			}
			want := "non-nested repository roots"
			switch kind {
			case "nested-repository":
				nested := resetFixtureRepo(t, root, "nested")
				c.Env.Put("CELL_ROOTS", "example/root="+root+",example/nested="+resetAlias(t, nested))
			case "nested-queue":
				key := filepath.Join(base, "key")
				if err := os.WriteFile(key, []byte("nonsecret fixture"), 0600); err != nil {
					t.Fatal(err)
				}
				cfg := &deskCommsConfig{Cell: c.Name, Mode: "interim", Repo: "example/repo", QueueDir: filepath.Join(resetAlias(t, root), "queue"), Socket: filepath.Join(base, "socket"), TrustStore: key, SigningKeys: map[string]string{}, Decider: json.RawMessage(`{"cmd":["fixture-reader"],"model":"fixture","pin":"1","isolate":true,"contained":true}`)}
				for _, role := range knownRoles {
					cfg.SigningKeys[role] = key
				}
				manifest := filepath.Join(base, "comms.json")
				writeCommsFixture(t, manifest, cfg)
				c.Env.Put("CELL_COMMS_CONFIG", manifest)
				want = "comms storage outside repository roots"
			case "worktree-root-link":
				if err := os.Symlink(root, filepath.Join(c.Dir, "worktrees")); err != nil {
					t.Fatal(err)
				}
				want = "worktree root must be a real directory"
			}
			plan, err := c.planReset(true)
			if err != nil {
				t.Fatal(err)
			}
			// Re-spell Git's canonical nested root to exercise this independent guard.
			if kind == "nested-repository" {
				plan.Repositories[1].Root = resetAlias(t, plan.Repositories[1].Root)
			}
			if _, err := c.preflightReset(plan); err == nil || !strings.Contains(err.Error(), want) {
				t.Fatalf("%s accepted: %v", kind, err)
			}
			if _, err := os.Stat(filepath.Join(c.Dir, ".reset-in-progress")); !os.IsNotExist(err) {
				t.Fatal("preflight changed cell", err)
			}
		})
	}
}
