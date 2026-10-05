package cellscratch

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func gitFixture(t *testing.T) (string, string) {
	t.Helper()
	dir := t.TempDir()
	git := func(args ...string) string {
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull)
		b, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git: %s: %v", b, err)
		}
		return strings.TrimSpace(string(b))
	}
	git("init", "-q")
	for name, body := range map[string]string{"tracked": "source", "hidden": "required", "test.sh": "#!/bin/sh\n", "link-target": "data", ".gitattributes": "hidden export-ignore\n", ".gitignore": "generated\n"} {
		mode := os.FileMode(0600)
		if name == "test.sh" {
			mode = 0700
		}
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), mode); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink("link-target", filepath.Join(dir, "link")); err != nil {
		t.Fatal(err)
	}
	git("add", "tracked", "hidden", "test.sh", "link-target", ".gitattributes", ".gitignore", "link")
	git("-c", "user.name=Fixture", "-c", "user.email=fixture@example.com", "commit", "-qm", "fixture")
	return dir, git("rev-parse", "HEAD")
}
func TestScratchSnapshot(t *testing.T) {
	dir, sha := gitFixture(t)
	s := scratchStore(t)
	r := scratchRun(t, s)
	r.Record.Revision = sha
	os.WriteFile(filepath.Join(dir, "generated"), []byte(strings.Repeat("g", 10000)), 0600)
	if err := r.Snapshot(context.Background(), dir, 1000); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"tracked", "hidden", "test.sh", "link"} {
		if !exists(t, filepath.Join(r.Work(), name)) {
			t.Fatal("required source omitted", name)
		}
	}
	if exists(t, filepath.Join(r.Work(), "generated")) || exists(t, filepath.Join(r.Work(), ".git")) {
		t.Fatal("imported untracked output or Git metadata")
	}
	if err := r.Inputs(dir, []string{"generated"}, 2000); err == nil {
		t.Fatal("unbounded declared input")
	}
	if err := r.Inputs(dir, []string{"generated"}, 20000); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(r.Work(), "generated"))
	if err != nil || len(b) != 10000 {
		t.Fatal("required extra input missing", err)
	}
	r2 := scratchRun(t, s)
	r2.Record.Revision = sha
	if err := r2.Snapshot(context.Background(), dir, 1); err == nil {
		t.Fatal("snapshot byte cap not enforced")
	}
	if err := r2.Inputs(dir, []string{"../outside"}, 20000); err == nil {
		t.Fatal("escaping input accepted")
	}
	if err := r2.Inputs(dir, []string{".git/config"}, 20000); err == nil {
		t.Fatal("credential-bearing metadata imported")
	}
	if err := r2.Inputs(s.Path, []string{filepath.Join(r.Record.ID, "work", "generated")}, 20000); err == nil {
		t.Fatal("output recursively copied")
	}
}
