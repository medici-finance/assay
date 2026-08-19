package main

import (
	"crypto/sha256"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// TestUnknownSubcommandFailsClosed pins #1075: an unrecognised positional
// subcommand must fail closed — a diagnostic naming the unknown token plus the
// valid set to stderr, a non-zero exit, and NO writes — rather than silently
// falling through to the default regenerate (a WRITE mode) and reporting success.
//
// This is a main()-level dispatch (it calls os.Exit before flag parsing), so it
// is exercised through a built binary as a subprocess, matching version_test.go.
// The `run` helper the other tests use never sees the unknown token — the guard
// sits in main() ahead of it.
func TestUnknownSubcommandFailsClosed(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go toolchain unavailable")
	}
	bin := filepath.Join(t.TempDir(), "statusgen")
	if out, err := exec.Command("go", "build", "-o", bin, ".").CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}

	// A VALID stream root, so a fall-through regenerate genuinely WOULD write
	// (STATUS.md + register views). If the tree is untouched after the run, the
	// default mode demonstrably did not execute.
	freshRoot := func(t *testing.T) string {
		root := t.TempDir()
		if err := os.CopyFS(root, os.DirFS("testdata/goodrepo")); err != nil {
			t.Fatal(err)
		}
		return root
	}

	// snapshot returns a stable map of every file path -> content hash under root,
	// so a comparison catches a created STATUS.md/INTAKE.md AND an overwritten
	// FINDINGS.md alike.
	snapshot := func(t *testing.T, root string) map[string]string {
		m := map[string]string{}
		err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			b, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			rel, err := filepath.Rel(root, p)
			if err != nil {
				return err
			}
			m[rel] = string(sha256Sum(b))
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		return m
	}

	diff := func(before, after map[string]string) []string {
		var changed []string
		for k, v := range after {
			if before[k] != v {
				changed = append(changed, k)
			}
		}
		for k := range before {
			if _, ok := after[k]; !ok {
				changed = append(changed, k+" (removed)")
			}
		}
		sort.Strings(changed)
		return changed
	}

	t.Run("bogus subcommand: exit non-zero, diagnostic, no writes", func(t *testing.T) {
		root := freshRoot(t)
		before := snapshot(t, root)

		cmd := exec.Command(bin, "bogussubcmd")
		cmd.Dir = root
		var stderr strings.Builder
		cmd.Stderr = &stderr
		err := cmd.Run()

		ee, ok := err.(*exec.ExitError)
		if !ok {
			t.Fatalf("expected a non-zero exit, got err=%v (a bogus subcommand must not exit 0)", err)
		}
		if ee.ExitCode() == 0 {
			t.Fatalf("bogus subcommand exited 0 — it silently fell through to regenerate (#1075)")
		}
		if got := stderr.String(); !strings.Contains(got, "unknown subcommand") ||
			!strings.Contains(got, "bogussubcmd") {
			t.Errorf("stderr must name the unknown token; got:\n%s", got)
		}
		if !strings.Contains(stderr.String(), "mergecheck") {
			t.Errorf("stderr should list the valid subcommand set; got:\n%s", stderr.String())
		}
		if _, err := os.Stat(filepath.Join(root, "STATUS.md")); !os.IsNotExist(err) {
			t.Errorf("STATUS.md must not be written for an unknown subcommand (stat err = %v)", err)
		}
		if changed := diff(before, snapshot(t, root)); len(changed) != 0 {
			t.Errorf("tree must stay clean; these files changed: %v", changed)
		}
	})

	t.Run("typo of a real subcommand is rejected, not run", func(t *testing.T) {
		root := freshRoot(t)
		before := snapshot(t, root)

		cmd := exec.Command(bin, "mergcheck") // missing 'e' in mergecheck
		cmd.Dir = root
		var stderr strings.Builder
		cmd.Stderr = &stderr
		err := cmd.Run()

		if ee, ok := err.(*exec.ExitError); !ok || ee.ExitCode() == 0 {
			t.Fatalf("a misspelled subcommand must exit non-zero; err=%v", err)
		}
		if !strings.Contains(stderr.String(), "unknown subcommand") {
			t.Errorf("a typo must be diagnosed as unknown, not silently regenerated; got:\n%s", stderr.String())
		}
		if changed := diff(before, snapshot(t, root)); len(changed) != 0 {
			t.Errorf("tree must stay clean; these files changed: %v", changed)
		}
	})

	t.Run("a real subcommand is NOT misjudged as unknown", func(t *testing.T) {
		// mergecheck may still exit non-zero for its own reasons (it shells out to
		// git), but it must never be rejected by the unknown-subcommand guard.
		cmd := exec.Command(bin, "mergecheck", "--help")
		cmd.Dir = freshRoot(t)
		out, _ := cmd.CombinedOutput()
		if strings.Contains(string(out), "unknown subcommand") {
			t.Errorf("mergecheck must dispatch, not be rejected as unknown; got:\n%s", out)
		}
	})

	t.Run("flags-only regenerate still works (guard did not break the default)", func(t *testing.T) {
		root := freshRoot(t)
		cmd := exec.Command(bin, "--root", ".")
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("flags-only regenerate exited non-zero: %v\n%s", err, out)
		}
		if _, err := os.Stat(filepath.Join(root, "STATUS.md")); err != nil {
			t.Errorf("flags-only regenerate must still write STATUS.md; stat err = %v", err)
		}
	})

	t.Run("a leading flag is never treated as an unknown subcommand", func(t *testing.T) {
		// --version is handled at the top of main; the guard must not intercept a
		// token that begins with "-".
		cmd := exec.Command(bin, "--version")
		cmd.Dir = freshRoot(t)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("--version exited non-zero: %v\n%s", err, out)
		}
		if strings.Contains(string(out), "unknown subcommand") {
			t.Errorf("--version must not be rejected as an unknown subcommand; got:\n%s", out)
		}
	})
}

func sha256Sum(b []byte) []byte {
	h := sha256.Sum256(b)
	return h[:]
}
