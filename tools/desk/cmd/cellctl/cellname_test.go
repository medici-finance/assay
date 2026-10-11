package main

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// A cell is named by its directory under the cells root. These tests hold the refusal of every
// other name — in `new`, in the loader and in `set` — and that the names which ARE under the
// root, a nested one and a symlinked one included, load as they always did.

// tryRefusal runs fn and reports whether it refused (died), with what it printed on stderr. It
// is refusal without the fatal: a test that walks many names wants to report each one.
func tryRefusal(t *testing.T, fn func()) (msg string, refused bool) {
	t.Helper()
	rd, wr, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	prev := os.Stderr
	os.Stderr = wr
	done := make(chan string, 1)
	go func() { b, _ := io.ReadAll(rd); done <- string(b) }()
	func() {
		defer func() {
			os.Stderr = prev
			wr.Close()
			if r := recover(); r != nil {
				if _, ok := r.(exitCode); !ok {
					panic(r)
				}
				refused = true
			}
		}()
		fn()
	}()
	msg = <-done
	rd.Close()
	return msg, refused
}

// cellNameRoot is a cells root one level inside the test's own temporary directory, so a name
// that climbs out of the root still lands somewhere this test owns.
func cellNameRoot(t *testing.T) (tmp, root string) {
	t.Helper()
	tmp = defaultsRoot(t)
	if resolved, err := filepath.EvalSymlinks(tmp); err == nil {
		tmp = resolved
	}
	root = filepath.Join(tmp, "cells")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CELLS_ROOT", root)
	return tmp, root
}

// notLocalNames are names that do not name a directory under the cells root at `root`, whose
// parent is tmp.
func notLocalNames(tmp string) map[string]string {
	return map[string]string{
		"climbs out of the root":            "../other/x",
		"climbs out through a nested name":  "team/../../other/x",
		"the root's parent":                 "..",
		"the root itself":                   ".",
		"the root itself, by a longer path": "team/..",
		"an absolute path":                  filepath.Join(tmp, "other", "x"),
	}
}

func assertNotLocalRefusal(t *testing.T, what, msg, name, root string) {
	t.Helper()
	for _, want := range []string{"'" + name + "'", root, "is not a path under the cells root"} {
		if !strings.Contains(msg, want) {
			t.Errorf("%s: the refusal should name the cell name and the cells root, and say why (missing %q):\n%s", what, want, msg)
		}
	}
}

// TestCellNameNewRefusesANameOutsideTheRoot: `new` scaffolds nothing for a name that is not
// local to the cells root, and says which name and which root.
func TestCellNameNewRefusesANameOutsideTheRoot(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	repo := t.TempDir()
	if out, err := exec.Command("git", "init", "-q", repo).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, out)
	}
	tmp, root := cellNameRoot(t)
	scaffold := func(name string) func() {
		return func() {
			captureStdout(t, func() {
				cmdNew([]string{name, "--kind", "scrubbed", "--repo", repo, "--repo-slug", "o/r", "--roots", "o/r=" + repo})
			})
		}
	}
	for what, name := range notLocalNames(tmp) {
		msg, refused := tryRefusal(t, scaffold(name))
		if !refused {
			t.Errorf("new %q (%s) scaffolded a cell; want a refusal naming the name and the cells root %s", name, what, root)
			continue
		}
		assertNotLocalRefusal(t, "new "+name, msg, name, root)
		if !strings.Contains(msg, "cellctl: new: ") {
			t.Errorf("new %q: the refusal should be `new`'s own:\n%s", name, msg)
		}
	}
	// Nothing was made outside the root, nothing in it, and no defaults file was seeded by a
	// `new` that refused.
	if _, err := os.Lstat(filepath.Join(tmp, "other")); !os.IsNotExist(err) {
		t.Errorf("a refused `new` left %s behind (%v)", filepath.Join(tmp, "other"), err)
	}
	if left, err := os.ReadDir(root); err != nil || len(left) != 0 {
		var names []string
		for _, e := range left {
			names = append(names, e.Name())
		}
		t.Errorf("a refused `new` left %v in the cells root (%v)", names, err)
	}

	// The names that are under the root still scaffold: one level down, and nested.
	for _, name := range []string{"demo", "team/demo"} {
		if msg, refused := tryRefusal(t, scaffold(name)); refused {
			t.Fatalf("new %q is refused:\n%s", name, msg)
		}
		c, msg := tryLoadCell(t, name)
		if c == nil {
			t.Fatalf("the cell `new %s` scaffolded does not load:\n%s", name, msg)
		}
		if r, n := c.reenter(); r != root || n != filepath.FromSlash(name) {
			t.Errorf("%s re-enters as (%s, %s), want (%s, %s)", name, r, n, root, name)
		}
	}
}

// TestCellNameLoaderRefusesANameOutsideTheRoot: a cell directory that exists, with a cell.env
// that would load, is still not loaded by a name that is not local to the cells root — by the
// loader or by `set`, which reads and writes the same cell.env.
func TestCellNameLoaderRefusesANameOutsideTheRoot(t *testing.T) {
	tmp, root := cellNameRoot(t)
	outside := strings.Replace(demoCell, "CELL=demo", "CELL=x", 1)
	envfile := writeCell(t, tmp, filepath.Join("other", "x"), outside)
	// The root itself made to look like a cell, so `.` and `team/..` have a cell.env to find.
	if err := os.WriteFile(filepath.Join(root, "cell.env"), []byte(demoCell), 0o600); err != nil {
		t.Fatal(err)
	}
	// And its parent, for `..`.
	if err := os.WriteFile(filepath.Join(tmp, "cell.env"), []byte(demoCell), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "team"), 0o700); err != nil {
		t.Fatal(err)
	}
	for what, name := range notLocalNames(tmp) {
		c, msg := tryLoadCell(t, name)
		if c != nil {
			t.Errorf("the loader loaded %q (%s) as the cell at %s; want a refusal naming the name and the cells root %s", name, what, c.Dir, root)
		} else {
			assertNotLocalRefusal(t, "load "+name, msg, name, root)
		}
		msg, refused := tryRefusal(t, func() { cmdSet(name, []string{"CELL_COCKPIT=tmux"}) })
		if !refused {
			t.Errorf("set %q (%s) wrote a cell.env; want a refusal naming the name and the cells root %s", name, what, root)
		} else {
			assertNotLocalRefusal(t, "set "+name, msg, name, root)
		}
	}
	if got, err := os.ReadFile(envfile); err != nil || string(got) != outside {
		t.Errorf("a refused `set` changed %s (%v):\n%s", envfile, err, got)
	}

	// A name that is under the root loads, however it is spelled, and re-enters by its path
	// under the root.
	writeCell(t, root, "demo", demoCell)
	writeCell(t, root, filepath.Join("team", "demo"), demoCell)
	for name, ref := range map[string]string{
		"demo":             "demo",
		"team/demo":        "team/demo",
		"team/../demo":     "demo",
		"./team//demo/":    "team/demo",
		"team/./x/../demo": "team/demo",
	} {
		c, msg := tryLoadCell(t, name)
		if c == nil {
			t.Errorf("%q, a name under the cells root, does not load:\n%s", name, msg)
			continue
		}
		if r, n := c.reenter(); r != root || n != filepath.FromSlash(ref) || c.Dir != filepath.Join(root, filepath.FromSlash(ref)) {
			t.Errorf("%q loads %s and re-enters as (%s, %s), want %s and (%s, %s)", name, c.Dir, r, n, filepath.Join(root, filepath.FromSlash(ref)), root, ref)
		}
	}

	// A cell directory that is a symlink to somewhere else is under the root by its NAME, which
	// is all this rule looks at: it loads, as it did before, and re-enters by that name.
	if runtime.GOOS != "windows" {
		if err := os.Symlink(filepath.Join(tmp, "other", "x"), filepath.Join(root, "linked")); err != nil {
			t.Fatal(err)
		}
		c, msg := tryLoadCell(t, "linked")
		if c == nil {
			t.Fatalf("a symlinked cell directory does not load:\n%s", msg)
		}
		if r, n := c.reenter(); r != root || n != "linked" {
			t.Errorf("a symlinked cell re-enters as (%s, %s), want (%s, linked)", r, n, root)
		}
	}
}
