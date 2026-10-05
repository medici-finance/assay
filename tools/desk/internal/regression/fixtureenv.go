// Package regression holds the desktools regression floor. This file is its only
// non-test source: the git-environment isolation every floor fixture that runs git
// (directly or through a child shell) must use, plus the hostile-environment control
// that proves it. Test-only helpers; no desk tool imports them.
package regression

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// gitVar reports whether an environment key belongs to git's own namespace. Every
// GIT_* variable is dropped, not a list of known ones: GIT_DIR, GIT_WORK_TREE,
// GIT_INDEX_FILE, GIT_COMMON_DIR and GIT_OBJECT_DIRECTORY redirect the repository,
// and GIT_CONFIG_COUNT/KEY_n/VALUE_n and GIT_CONFIG_PARAMETERS inject config. Keys
// compare case-insensitively because Windows environment names do.
func gitVar(key string) bool { return strings.HasPrefix(strings.ToUpper(key), "GIT_") }

// FixtureEnv is the environment a floor fixture hands its git or shell child: the
// caller's environment with every GIT_* variable removed, followed by extra. A git
// hook, or any git command that runs the suite on a caller's behalf, exports GIT_DIR;
// inherited unchanged, it points every fixture git call at the caller's repository
// instead of the fixture's temporary one.
func FixtureEnv(extra ...string) []string {
	var env []string
	for _, kv := range os.Environ() {
		if key, _, _ := strings.Cut(kv, "="); !gitVar(key) {
			env = append(env, kv)
		}
	}
	return append(env, extra...)
}

// IsolateGit removes every GIT_* variable from the test process until t ends, so
// production code the fixture calls in-process (and any child it starts with the
// inherited environment) cannot be redirected either. Values are restored at cleanup.
func IsolateGit(t testing.TB) {
	t.Helper()
	for _, kv := range os.Environ() {
		if key, _, _ := strings.Cut(kv, "="); gitVar(key) {
			t.Setenv(key, "") // registers the restore
			if err := os.Unsetenv(key); err != nil {
				t.Fatal(err)
			}
		}
	}
}

// fixedGitEnv is the environment HostileGitDir builds its victim with: a literal
// list naming only PATH, a private HOME and null git config, never derived from the
// caller's environment or from FixtureEnv. Neither an exported GIT_* variable nor a
// mutation of FixtureEnv (mutate_guard.py gitenv) can redirect the victim's own
// setup commit into another repository.
func fixedGitEnv(home string) []string {
	env := []string{"PATH=" + os.Getenv("PATH"), "HOME=" + home,
		"GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=" + os.DevNull}
	if root := os.Getenv("SYSTEMROOT"); root != "" {
		env = append(env, "SYSTEMROOT="+root) // Windows git needs it to start
	}
	return env
}

// victimSetupGit cannot leave automatic maintenance running after the setup
// command returns. The options and literal environment apply only to setup,
// never to a fixture's Git invocation or the repository's persistent config.
func victimSetupGit(home string, args ...string) *exec.Cmd {
	args = append([]string{"-c", "maintenance.auto=false", "-c", "gc.auto=0",
		"-c", "maintenance.autoDetach=false", "-c", "gc.autoDetach=false"}, args...)
	cmd := exec.Command("git", args...) // literal argv[0]: the forge-CLI ban resolves it
	cmd.Env = fixedGitEnv(home)
	return cmd
}

// HostileGitDir builds a committed repository outside the fixture under test and
// exports GIT_DIR, GIT_WORK_TREE and GIT_INDEX_FILE naming it for the rest of t. It
// returns the repository's directory for TreeDigest. A fixture that leaks the
// inherited environment to git writes there.
func HostileGitDir(t testing.TB) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	base := t.TempDir()
	victim := filepath.Join(base, "victim")
	for _, args := range [][]string{
		{"init", "-q", "-b", "main", victim},
		{"-C", victim, "-c", "user.name=Victim", "-c", "user.email=victim@example.invalid",
			"-c", "commit.gpgsign=false", "commit", "-q", "--allow-empty", "-m", "victim"},
	} {
		cmd := victimSetupGit(base, args...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("victim git %v: %v\n%s", args, err, out)
		}
	}
	t.Setenv("GIT_DIR", filepath.Join(victim, ".git"))
	t.Setenv("GIT_WORK_TREE", victim)
	t.Setenv("GIT_INDEX_FILE", filepath.Join(victim, ".git", "index"))
	return victim
}

// TreeSnapshot records every path, mode, symlink target hash and file body hash
// under a directory, including every .git entry. No metadata is excluded.
// Hashes, rather than file bodies or symlink targets, are safe failure diagnostics.
type TreeSnapshot map[string]treeEntry

type treeEntry struct {
	Mode fs.FileMode
	Hash string
}

func SnapshotTree(t testing.TB, dir string) TreeSnapshot {
	t.Helper()
	entries := TreeSnapshot{}
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		entry := treeEntry{Mode: info.Mode()}
		var body []byte
		switch {
		case d.Type()&fs.ModeSymlink != 0:
			target, err := os.Readlink(path)
			if err != nil {
				return err
			}
			body = []byte(target)
		case d.Type().IsRegular():
			body, err = os.ReadFile(path)
			if err != nil {
				return err
			}
		}
		if d.Type()&fs.ModeSymlink != 0 || d.Type().IsRegular() {
			sum := sha256.Sum256(body)
			entry.Hash = hex.EncodeToString(sum[:])
		}
		entries[filepath.ToSlash(rel)] = entry
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return entries
}

// Changes reports sorted added, removed and changed paths, modes and hashes.
// A nonempty result always fails the isolation assertion, including .git changes.
func (before TreeSnapshot) Changes(after TreeSnapshot) string {
	paths := make(map[string]bool, len(before)+len(after))
	for path := range before {
		paths[path] = true
	}
	for path := range after {
		paths[path] = true
	}
	var changes []string
	for path := range paths {
		old, had := before[path]
		next, has := after[path]
		switch {
		case !had:
			changes = append(changes, fmt.Sprintf("added %q: mode=%s sha256=%s", path, next.Mode, next.Hash))
		case !has:
			changes = append(changes, fmt.Sprintf("removed %q: mode=%s sha256=%s", path, old.Mode, old.Hash))
		case old != next:
			changes = append(changes, fmt.Sprintf("changed %q: mode=%s sha256=%s -> mode=%s sha256=%s", path, old.Mode, old.Hash, next.Mode, next.Hash))
		}
	}
	sort.Strings(changes)
	return strings.Join(changes, "\n")
}

// TreeDigest hashes every path, mode, symlink target and file body under dir, so
// two equal digests mean the tree is byte-unchanged.
func TreeDigest(t testing.TB, dir string) string {
	t.Helper()
	snapshot := SnapshotTree(t, dir)
	var rows []string
	for path, entry := range snapshot {
		rows = append(rows, path+"\x00"+entry.Mode.String()+"\x00"+entry.Hash)
	}
	sort.Strings(rows)
	sum := sha256.Sum256([]byte(strings.Join(rows, "\n")))
	return hex.EncodeToString(sum[:])
}
