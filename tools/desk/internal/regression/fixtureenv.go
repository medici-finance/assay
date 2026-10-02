// Package regression holds the desktools regression floor. This file is its only
// non-test source: the git-environment isolation every floor fixture that runs git
// (directly or through a child shell) must use, plus the hostile-environment control
// that proves it. Test-only helpers; no desk tool imports them.
package regression

import (
	"crypto/sha256"
	"encoding/hex"
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
		cmd := exec.Command("git", args...) // literal argv[0]: the forge-CLI ban resolves it
		cmd.Env = fixedGitEnv(base)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("victim git %v: %v\n%s", args, err, out)
		}
	}
	t.Setenv("GIT_DIR", filepath.Join(victim, ".git"))
	t.Setenv("GIT_WORK_TREE", victim)
	t.Setenv("GIT_INDEX_FILE", filepath.Join(victim, ".git", "index"))
	return victim
}

// TreeDigest hashes every path, mode, symlink target and file body under dir, so
// two equal digests mean the tree is byte-unchanged.
func TreeDigest(t testing.TB, dir string) string {
	t.Helper()
	var rows []string
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
		row := filepath.ToSlash(rel) + "\x00" + info.Mode().String()
		switch {
		case d.Type()&fs.ModeSymlink != 0:
			target, err := os.Readlink(path)
			if err != nil {
				return err
			}
			row += "\x00" + target
		case d.Type().IsRegular():
			body, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			sum := sha256.Sum256(body)
			row += "\x00" + hex.EncodeToString(sum[:])
		}
		rows = append(rows, row)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(rows)
	sum := sha256.Sum256([]byte(strings.Join(rows, "\n")))
	return hex.EncodeToString(sum[:])
}
