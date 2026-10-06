package cellscratch

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func sourceGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("fixture git: %v: %s", err, out)
	}
	return strings.TrimSpace(string(out))
}

func TestScratchSourceAdmission(t *testing.T) {
	seed, sha := gitFixture(t)
	for _, name := range []string{"mirror", "project.git"} {
		t.Run(name, func(t *testing.T) {
			source := filepath.Join(t.TempDir(), name)
			sourceGit(t, seed, "clone", "--bare", "--no-hardlinks", seed, source)
			if _, _, err := SourceRevision(source, "HEAD"); err == nil {
				t.Error("non-worktree source admitted by SourceRevision")
			}
			r := scratchRun(t, scratchStore(t))
			r.Record.Revision = sha
			if err := r.Inputs(source, []string{"config"}, 4096); err == nil {
				t.Error("non-worktree source admitted by Inputs")
			}
			if err := r.Snapshot(context.Background(), source, 4096); err == nil {
				t.Error("non-worktree source admitted by Snapshot")
			}
		})
	}
	t.Run("subdirectory", func(t *testing.T) {
		sub := filepath.Join(seed, "generated")
		if err := os.Mkdir(sub, 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(sub, "data"), []byte("fixture"), 0600); err != nil {
			t.Fatal(err)
		}
		r := scratchRun(t, scratchStore(t))
		r.Record.Revision = sha
		if err := r.Inputs(sub, []string{"data"}, 4096); err == nil {
			t.Error("source subdirectory admitted by Inputs")
		}
		if err := r.Snapshot(context.Background(), sub, 4096); err == nil {
			t.Error("source subdirectory admitted by Snapshot")
		}
	})
}

func TestScratchInputRepoBoundary(t *testing.T) {
	source, _ := gitFixture(t)
	sourceGit(t, source, "clone", "--bare", "--no-hardlinks", source, filepath.Join(source, "mirror"))
	r := scratchRun(t, scratchStore(t))
	if err := r.Inputs(source, []string{"mirror/config"}, 4096); err == nil {
		t.Fatal("input crossed repository boundary")
	}
}

func TestScratchSourcePositive(t *testing.T) {
	seed, sha := gitFixture(t)
	linked := filepath.Join(t.TempDir(), "linked")
	sourceGit(t, seed, "worktree", "add", "--detach", linked, "HEAD")
	for _, source := range []string{seed, linked} {
		canonical, got, err := SourceRevision(source, "HEAD")
		if err != nil || got != sha {
			t.Fatalf("source revision: %s %v", got, err)
		}
		info, err := os.Stat(canonical)
		if err != nil {
			t.Fatal(err)
		}
		want, err := os.Stat(source)
		if err != nil || !os.SameFile(info, want) {
			t.Fatal("wrong source root", err)
		}
		r := scratchRun(t, scratchStore(t))
		r.Record.Revision = sha
		if err := r.Inputs(source, []string{"tracked"}, 4096); err != nil {
			t.Fatal("required input refused", err)
		}
		r2 := scratchRun(t, scratchStore(t))
		r2.Record.Revision = sha
		if err := r2.Snapshot(context.Background(), source, 4096); err != nil {
			t.Fatal("snapshot refused", err)
		}
	}
}

func TestScratchSourceEnv(t *testing.T) {
	seed, sha := gitFixture(t)
	bare := filepath.Join(t.TempDir(), "mirror")
	sourceGit(t, seed, "clone", "--bare", "--no-hardlinks", seed, bare)
	t.Setenv("GIT_DIR", filepath.Join(seed, ".git"))
	t.Setenv("GIT_WORK_TREE", bare)
	if _, _, err := SourceRevision(bare, "HEAD"); err == nil {
		t.Fatal("ambient Git settings admitted wrong source")
	}
	t.Setenv("GIT_DIR", filepath.Join(t.TempDir(), "missing"))
	_, got, err := SourceRevision(seed, "HEAD")
	if err != nil || got != sha {
		t.Fatal("ambient settings changed revision", err)
	}
	r := scratchRun(t, scratchStore(t))
	r.Record.Revision = sha
	if err := r.Snapshot(context.Background(), seed, 4096); err != nil {
		t.Fatal("ambient settings changed snapshot", err)
	}
}

func TestScratchSeparateGitDir(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	if err := os.Mkdir(source, 0700); err != nil {
		t.Fatal(err)
	}
	// A metadata store can have an ordinary name inside the working tree.
	sourceGit(t, source, "init", "--separate-git-dir", filepath.Join(source, "metadata"))
	if err := os.WriteFile(filepath.Join(source, "tracked"), []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	sourceGit(t, source, "add", "tracked")
	sourceGit(t, source, "-c", "user.name=Fixture", "-c", "user.email=fixture@example.com", "commit", "-qm", "fixture")
	r := scratchRun(t, scratchStore(t))
	if err := r.Inputs(source, []string{"metadata/config"}, 4096); err == nil {
		t.Fatal("separate metadata store admitted as input")
	}
	if err := r.Inputs(source, []string{"tracked"}, 4096); err != nil {
		t.Fatal("ordinary input refused", err)
	}
}

func TestScratchOutputInput(t *testing.T) {
	source, _ := gitFixture(t)
	s, err := Open(filepath.Join(source, "owned"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	first := scratchRun(t, s)
	if err := os.WriteFile(filepath.Join(first.Work(), "data"), []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	second := scratchRun(t, s)
	path := filepath.Join("owned", first.Record.ID, "work", "data")
	if err := second.Inputs(source, []string{path}, 4096); err == nil {
		t.Fatal("managed output imported through valid source")
	}
}
