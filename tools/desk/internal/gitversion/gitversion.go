// Package gitversion is the shared git-version guard for tests that depend on a git feature
// newer than the oldest git a supported host (or a Linux verify witness image) may ship.
//
// A test that needs, say, `git --attr-source` (git 2.40) or the empty-entry multi-valued
// config reset (git 2.46) fails on an older git for the WRONG reason: a usage error or a
// silently-ignored config entry, not the behaviour under test. RequireGit turns that into a
// SKIP that names the feature, the version it needs and the version installed, so a skip is
// never mistaken for a pass and never hides. On a git that meets the floor it does nothing:
// the test runs in full.
//
// It lives in its own stdlib-only package (not gittest) so any test binary can import it
// without inheriting gittest's `-update` flag, which would collide with packages that
// define their own.
package gitversion

import (
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// GitVersion is a parsed `git version` triple.
type GitVersion struct {
	Major, Minor, Patch int
	Raw                 string // the full `git version` line, trimmed
}

func (v GitVersion) String() string {
	return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
}

// AtLeast reports whether v is major.minor or later.
func (v GitVersion) AtLeast(major, minor int) bool {
	if v.Major != major {
		return v.Major > major
	}
	return v.Minor >= minor
}

var gitVersionRE = regexp.MustCompile(`^git version (\d+)\.(\d+)(?:\.(\d+))?`)

// ParseGitVersion parses the output of `git version`, e.g. "git version 2.39.5",
// "git version 2.47.3", "git version 2.50.1 (Apple Git-155)" or "git version
// 2.45.2.windows.1". It returns an error for anything it cannot read as a version.
func ParseGitVersion(out string) (GitVersion, error) {
	line := strings.TrimSpace(out)
	m := gitVersionRE.FindStringSubmatch(line)
	if m == nil {
		return GitVersion{}, fmt.Errorf("unrecognised `git version` output %q", line)
	}
	v := GitVersion{Raw: line}
	v.Major, _ = strconv.Atoi(m[1])
	v.Minor, _ = strconv.Atoi(m[2])
	if m[3] != "" {
		v.Patch, _ = strconv.Atoi(m[3])
	}
	return v, nil
}

// InstalledGitVersion runs `git version` on PATH and parses it. A missing git or an
// unparseable version is a test FAILURE, not a skip: a guard that skipped on "could not
// tell" would quietly turn every version-gated test off on a broken host.
func InstalledGitVersion(t testing.TB) GitVersion {
	t.Helper()
	out, err := exec.Command("git", "version").Output()
	if err != nil {
		t.Fatalf("gitversion: cannot run `git version`: %v", err)
	}
	v, err := ParseGitVersion(string(out))
	if err != nil {
		t.Fatalf("gitversion: %v", err)
	}
	return v
}

// RequireGit skips the test, with a named reason, when the installed git is older than
// major.minor. feature names what the test needs that version for, e.g.
// "merge-tree --attr-source". The skip message reads
// "needs git >= 2.40 for merge-tree --attr-source; have 2.39.5".
func RequireGit(t testing.TB, major, minor int, feature string) {
	t.Helper()
	v := InstalledGitVersion(t)
	if !v.AtLeast(major, minor) {
		t.Skipf("SKIP (could-not-check, not a pass): needs git >= %d.%d for %s; have %s",
			major, minor, feature, v)
	}
}
