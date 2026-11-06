package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// The five script adapters (desktools-v2/52) in a tree that carries the deskchangelog source,
// with either no Go at all or a Go whose build fails. A missing tool must never pass for an
// answer: every adapter exits 127, prints nothing on stdout and names the cause on stderr. For credit-identity.sh that
// means it never prints skip:unknown itself, and the release credits loop never reaches it,
// because the loop is fed by `aggregate.py credits`, which fails the same way first and leaves
// the highlights step to refuse the release.

// adapterCase is one adapter invocation, relative to the repository root.
type adapterCase struct {
	name string
	argv []string
}

func adapterCases(root string) []adapterCase {
	ch := filepath.Join(root, "tools", "changelog")
	return []adapterCase{
		{"credit-identity", []string{"bash", filepath.Join(ch, "credit-identity.sh"), "classify", "someone"}},
		{"credit-identity-body", []string{"bash", filepath.Join(ch, "credit-identity.sh"), "classify", "someone", filepath.Join(root, "CHANGELOG.md")}},
		{"credits-feeder", []string{"python3", filepath.Join(ch, "aggregate.py"), "credits", "changelog"}},
		{"highlights", []string{"python3", filepath.Join(ch, "aggregate.py"), "highlights", "changelog", "CHANGELOG.md"}},
		{"check", []string{"bash", filepath.Join(ch, "check.sh")}},
		{"spec-header", []string{"bash", filepath.Join(root, "tools", "release", "check-spec-header-version.sh")}},
		{"lint-decision", []string{"bash", filepath.Join(root, "tools", "lint-human-decision-defaults.sh"), root}},
	}
}

// toolsOnlyPATH is a directory holding links to exactly the named programs.
func toolsOnlyPATH(t *testing.T, names ...string) string {
	t.Helper()
	dir := t.TempDir()
	for _, n := range names {
		p, err := exec.LookPath(n)
		if err != nil {
			t.Skipf("could-not-check: %s is not installed here (%v); this is not a pass", n, err)
		}
		if err := os.Symlink(p, filepath.Join(dir, n)); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestAdapterMissingGo(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("could-not-check: the adapters are POSIX shell and python; this is not a pass")
	}
	root, err := filepath.Abs(filepath.Join("..", "..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	base := []string{"bash", "python3", "dirname", "mktemp", "rm"}

	noGo := toolsOnlyPATH(t, base...)
	brokenGo := toolsOnlyPATH(t, base...)
	stub := "#!/bin/sh\necho 'go: simulated build failure' >&2\nexit 1\n"
	if err := os.WriteFile(filepath.Join(brokenGo, "go"), []byte(stub), 0o755); err != nil {
		t.Fatal(err)
	}

	for _, env := range []struct {
		name, path, want string
	}{
		{"no-go", noGo, "go is not installed, so it cannot be built"},
		{"build-fails", brokenGo, "failed; nothing was"},
	} {
		for _, c := range adapterCases(root) {
			t.Run(env.name+"/"+c.name, func(t *testing.T) {
				r := execIn(t, filepath.Join(env.path, c.argv[0]), root,
					[]string{"PATH=" + env.path, "HOME=" + t.TempDir(), "TMPDIR=" + t.TempDir(),
						"ASSAY_TRUSTED_LOGINS=maintainer"},
					c.argv[1:]...)
				if r.code != 127 {
					t.Errorf("exit %d, want 127 (stderr %q)", r.code, r.stderr)
				}
				if r.stdout != "" {
					t.Errorf("stdout %q, want nothing: a missing tool must not print an answer", r.stdout)
				}
				if strings.Contains(r.stdout+r.stderr, "skip:unknown") {
					t.Errorf("printed skip:unknown: a missing tool must never pass for a credit decision")
				}
				if !strings.Contains(r.stderr, "error:") || !strings.Contains(r.stderr, env.want) {
					t.Errorf("stderr %q, want an error: line containing %q", r.stderr, env.want)
				}
			})
		}
	}
}

// TestAdapterStubOnPath: the tree under check decides the gate. In this tree, which carries
// the deskchangelog source, a stub deskchangelog sits first on PATH and would pass any gate.
// No adapter may run it: each builds the tree's own source instead, so the stub's marker file
// is never written and its verdict never reaches stdout.
func TestAdapterStubOnPath(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("could-not-check: the adapters are POSIX shell and python; this is not a pass")
	}
	if _, err := exec.LookPath("go"); err != nil {
		t.Skipf("could-not-check: go is not installed here (%v); this is not a pass", err)
	}
	root, err := filepath.Abs(filepath.Join("..", "..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	stubDir := t.TempDir()
	marker := filepath.Join(t.TempDir(), "stub-ran")
	stub := "#!/bin/sh\n: > \"$STUB_MARKER\"\necho stub-verdict\nexit 0\n"
	if err := os.WriteFile(filepath.Join(stubDir, "deskchangelog"), []byte(stub), 0o755); err != nil {
		t.Fatal(err)
	}
	var env []string
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(kv, "PATH=") {
			env = append(env, kv)
		}
	}
	env = append(env, "PATH="+stubDir+string(os.PathListSeparator)+os.Getenv("PATH"),
		"STUB_MARKER="+marker, "TMPDIR="+t.TempDir(), "ASSAY_TRUSTED_LOGINS=maintainer")
	for _, c := range adapterCases(root) {
		t.Run(c.name, func(t *testing.T) {
			bin, err := exec.LookPath(c.argv[0])
			if err != nil {
				t.Skipf("could-not-check: %s is not installed here (%v); this is not a pass", c.argv[0], err)
			}
			r := execIn(t, bin, root, env, c.argv[1:]...)
			if _, err := os.Stat(marker); err == nil {
				t.Fatalf("the deskchangelog on PATH ran (exit %d, stdout %q): the tree has the source, so its own build must decide", r.code, r.stdout)
			}
			if strings.Contains(r.stdout, "stub-verdict") {
				t.Errorf("stdout %q carries the PATH stub's verdict", r.stdout)
			}
			if r.code == 127 {
				t.Errorf("exit 127 (stderr %q): the tree's own build did not run", r.stderr)
			}
		})
	}
}
