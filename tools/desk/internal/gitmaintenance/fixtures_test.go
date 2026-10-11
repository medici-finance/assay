package gitmaintenance

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/gitquiet"
)

// regression: #2513
func TestMain(m *testing.M) {
	os.Exit(runFixtureMain(m))
}

// Own the package's Git config and runtime-guard state. A mutation replay must
// never depend on the operator's heartbeat or append to the operator's audit log.
func runFixtureMain(m *testing.M) int {
	home, err := os.MkdirTemp("", "gitmaintenance-fixture-home-")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer os.RemoveAll(home)
	for _, key := range []string{"HOME", "USERPROFILE", "XDG_CONFIG_HOME"} {
		if err := os.Setenv(key, home); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
	}
	return gitquiet.Run(m)
}

func fixtureGit(t testing.TB, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull,
		"GIT_AUTHOR_NAME=Fixture", "GIT_AUTHOR_EMAIL=fixture@example.invalid", "GIT_COMMITTER_NAME=Fixture", "GIT_COMMITTER_EMAIL=fixture@example.invalid")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("fixture git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func fixtureRepo(t testing.TB) string {
	t.Helper()
	// Git hook custody includes effective global config; each fixture owns its
	// global namespace as well as its repository and cannot inherit operator hooks.
	home := filepath.Join(t.TempDir(), "home")
	if err := os.Mkdir(home, 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_CONFIG_HOME", home)

	dir := filepath.Join(t.TempDir(), "repo")
	if err := os.Mkdir(dir, 0755); err != nil {
		t.Fatal(err)
	}
	fixtureGit(t, dir, "init", "-b", "main")
	fixtureCommit(t, dir, "initial")
	return dir
}

func fixtureCommit(t testing.TB, dir, text string) string {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "tracked"), []byte(text+"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	fixtureGit(t, dir, "add", "tracked")
	fixtureGit(t, dir, "commit", "-m", text)
	return fixtureGit(t, dir, "rev-parse", "HEAD")
}

// diskImage includes all files, symlink targets and modes, including otherwise invisible
// maintenance bookkeeping. Reading an inventory must not create a lock, stamp or config.
func diskImage(t testing.TB, dir string) map[string]string {
	t.Helper()
	image := map[string]string{}
	err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		value := fmt.Sprintf("%v", info.Mode())
		switch {
		case entry.Type()&os.ModeSymlink != 0:
			target, err := os.Readlink(path)
			if err != nil {
				return err
			}
			value += ":" + target
		case info.Mode().IsRegular():
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			value += fmt.Sprintf(":%x", sha256.Sum256(data))
		}
		image[rel] = value
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return image
}

// requireNativeHooks makes apply evidence capability-specific without accepting
// guessed hook state. The mandatory platform lane refuses a missing capability.
func requireNativeHooks(t testing.TB, repo string) {
	t.Helper()
	cmd := exec.Command("git", "hook", "list", "-z", "reference-transaction")
	cmd.Dir = repo
	cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull, "LC_ALL=C")
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	if e, ok := err.(*exec.ExitError); ok && e.ExitCode() == 1 && stdout.Len() == 0 && strings.TrimSpace(stderr.String()) == "warning: no hooks found for event 'reference-transaction'" {
		return
	}
	message := fmt.Sprintf("native git hook list required: %v stdout=%q stderr=%q", err, stdout.String(), stderr.String())
	if os.Getenv("ASSAY_REQUIRE_NATIVE_HOOK_LIST") == "1" {
		t.Fatal(message)
	}
	t.Skip(message)
}

// snapshotChanges preserves complete snapshots and names every mismatch; known
// coordinator bookkeeping must be seeded explicitly, never filtered out.
func snapshotChanges(before, after map[string]string) []string {
	keys := map[string]bool{}
	for key := range before {
		keys[key] = true
	}
	for key := range after {
		keys[key] = true
	}
	var changes []string
	for key := range keys {
		old, oldOK := before[key]
		new, newOK := after[key]
		if oldOK != newOK || old != new {
			changes = append(changes, fmt.Sprintf("%s: %q -> %q", key, old, new))
		}
	}
	sort.Strings(changes)
	return changes
}
