package deskkit

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/gitversion"
)

// The git floor for verifier homes (#2318).
//
// Verifier admission renders a converted file (an eol or encoding attribute)
// from the attested commit's attributes with `git --attr-source`, which arrived
// in git 2.41. On an older git it refuses the home, by design. This repository
// ships *.ps1 and *.psm1 with eol=crlf, so a desk-base image with an older git
// produces verifier homes that are refused for every brief here.
//
// containers/scripts/git-floor-check.sh is the check, and containers/base/
// Dockerfile runs it at build time. The PR CI runners have no Docker daemon and
// the image is built only on release, so these tests prove the two halves that
// can be proven on every PR:
//
//   - the SHIPPED script passes on a capable git and FAILS on each kind of
//     incapable one: too old, rejecting the option, and accepting the option
//     but ignoring it (TestGitFloorRefusesWeakGits is the positive control);
//   - the base Dockerfile still builds git from a sha256-checked tarball, keeps
//     bookworm's apt git out, and runs the script after git is in place in the
//     final stage (TestBaseImageRunsGitFloor), so a base change cannot drop the
//     build-time check without going red here.

const (
	gitFloorScriptPath  = fixtureRepoRoot + "/containers/scripts/git-floor-check.sh"
	baseDockerfilePath  = fixtureRepoRoot + "/containers/base/Dockerfile"
	gitFloorScriptInImg = "/usr/local/bin/git-floor-check"
)

// gitFloorTools resolves sh and the real git, skipping off-POSIX hosts and
// failing (never skipping) in CI when either is missing: a check that skips on
// the runner has proven nothing there.
func gitFloorTools(t *testing.T) (sh, git string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("git-floor-check.sh is a POSIX shell script for a Linux image")
	}
	skipIfFixtureAbsent(t, gitFloorScriptPath, "containers/ is not part of this repository's published file set")
	look := func(name string) string {
		p, err := exec.LookPath(name)
		if err == nil {
			return p
		}
		if os.Getenv("CI") != "" {
			t.Fatalf("%s is not on PATH, but CI is set: skipping would report the git floor as checked when it was not", name)
		}
		t.Skipf("%s not available (local run)", name)
		return ""
	}
	return look("sh"), look("git")
}

// runGitFloor runs the shipped script with pathDir prepended to PATH (or as
// the WHOLE of PATH when only is true) and returns its exit code and output.
func runGitFloor(t *testing.T, sh, pathDir string, only bool, args ...string) (int, string) {
	t.Helper()
	script, err := filepath.Abs(gitFloorScriptPath)
	if err != nil {
		t.Fatal(err)
	}
	path := pathDir + string(os.PathListSeparator) + os.Getenv("PATH")
	if only {
		path = pathDir
	}
	cmd := exec.Command(sh, append([]string{script}, args...)...)
	cmd.Env = append(envWithout(os.Environ(), "PATH"), "PATH="+path)
	out, err := cmd.CombinedOutput()
	var ee *exec.ExitError
	switch {
	case err == nil:
		return 0, string(out)
	case errors.As(err, &ee):
		return ee.ExitCode(), string(out)
	default:
		t.Fatalf("running git-floor-check.sh: %v\n%s", err, out)
		return -1, ""
	}
}

func envWithout(env []string, key string) []string {
	out := make([]string, 0, len(env))
	for _, kv := range env {
		if !strings.HasPrefix(kv, key+"=") {
			out = append(out, kv)
		}
	}
	return out
}

// standIn writes an executable POSIX script named name into a fresh directory
// and returns that directory.
func standIn(t *testing.T, name, body string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\n"+body), 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

// fakeVersionGit reports versionLine for `git version` and hands every other
// call to the real git. rejectAttr makes it refuse --attr-source the way a
// git older than 2.41 does (exit 129, unknown option).
func fakeVersionGit(realGit, versionLine string, rejectAttr bool) string {
	reject := ""
	if rejectAttr {
		reject = `for a in "$@"; do case "$a" in --attr-source*) echo "unknown option: $a" >&2; exit 129 ;; esac; done
`
	}
	return reject + `if [ "$#" -eq 1 ] && [ "$1" = version ]; then echo '` + versionLine + `'; exit 0; fi
exec '` + realGit + `' "$@"
`
}

// TestGitFloorPassesCapableGit: the real git on this host meets the floor, so
// the script passes. CI's git is well above 2.41; a developer host below it
// skips with the reason named.
func TestGitFloorPassesCapableGit(t *testing.T) {
	sh, _ := gitFloorTools(t)
	if os.Getenv("CI") == "" {
		gitversion.RequireGit(t, 2, 41, "git-floor-check.sh's passing case")
	} else if v := gitversion.InstalledGitVersion(t); !v.AtLeast(2, 41) {
		t.Fatalf("CI host git %s is below 2.41: the passing case cannot run here", v)
	}
	code, out := runGitFloor(t, sh, t.TempDir(), false)
	if code != 0 || !strings.Contains(out, "OK:") {
		t.Fatalf("capable git: exit %d, want 0 with an OK line\n%s", code, out)
	}
}

// TestGitFloorRefusesWeakGits is the positive control: every stand-in below is
// a git that cannot render a converted file from attested attributes, and the
// script must fail on each one for its own reason. The 2.41.0 case pins the
// boundary from the other side.
func TestGitFloorRefusesWeakGits(t *testing.T) {
	sh, git := gitFloorTools(t)
	gitversion.RequireGit(t, 2, 41, "the stand-ins that delegate to the real git")

	stripAttr := `n=$#; i=0
while [ "$i" -lt "$n" ]; do a=$1; shift; i=$((i+1)); case "$a" in --attr-source*) ;; *) set -- "$@" "$a" ;; esac; done
exec '` + git + `' "$@"
`
	cases := []struct {
		name     string
		body     string
		wantCode int
		wantText string
	}{
		{"bookworm 2.39.5", fakeVersionGit(git, "git version 2.39.5", true), 1, "is older than 2.41"},
		{"just below 2.40.1", fakeVersionGit(git, "git version 2.40.1", false), 1, "is older than 2.41"},
		{"claims 2.47 rejects option", fakeVersionGit(git, "git version 2.47.3", true), 1, "was rejected"},
		{"accepts option ignores it", stripAttr, 1, "accepted and ignored"},
		{"unreadable version", fakeVersionGit(git, "not a git", false), 2, "COULD-NOT-CHECK"},
		{"boundary 2.41.0 passes", fakeVersionGit(git, "git version 2.41.0", false), 0, "OK:"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			code, out := runGitFloor(t, sh, standIn(t, "git", tc.body), false)
			if code != tc.wantCode || !strings.Contains(out, tc.wantText) {
				t.Fatalf("exit %d, want %d with %q\n%s", code, tc.wantCode, tc.wantText, out)
			}
		})
	}

	t.Run("older git later on PATH", func(t *testing.T) {
		// The git found first is capable; a distro 2.39.5 sits behind it, where
		// a process with a different PATH order would reach it.
		first := standIn(t, "git", "exec '"+git+"' \"$@\"\n")
		behind := standIn(t, "git", fakeVersionGit(git, "git version 2.39.5", true))
		code, out := runGitFloor(t, sh, first+string(os.PathListSeparator)+behind, false)
		if code != 1 || !strings.Contains(out, behind) || !strings.Contains(out, "is older than 2.41") {
			t.Fatalf("older git behind a capable one: exit %d, want 1 naming %s\n%s", code, behind, out)
		}
	})

	t.Run("git absent", func(t *testing.T) {
		code, out := runGitFloor(t, sh, t.TempDir(), true)
		if code != 1 || !strings.Contains(out, "git is not on PATH") {
			t.Fatalf("no git: exit %d, want 1\n%s", code, out)
		}
	})
}

// TestGitFloorImageForm drives the `<image-ref>` form through a stand-in
// docker. The stand-in runs what it is fed on stdin, which is exactly what
// `docker run -i ... /bin/sh -s` does inside the image, so the probe's own
// verdict must come back out, and a docker failure must read as
// could-not-check, never as a pass.
func TestGitFloorImageForm(t *testing.T) {
	sh, git := gitFloorTools(t)
	gitversion.RequireGit(t, 2, 41, "the image-form passing case")

	oldGit := standIn(t, "git", fakeVersionGit(git, "git version 2.39.5", true))
	cases := []struct {
		name     string
		docker   string
		wantCode int
		wantText string
	}{
		{"image meets floor", "exec '" + sh + "' -s\n", 0, "meets the git floor"},
		{"image below floor", "PATH='" + oldGit + "':$PATH exec '" + sh + "' -s\n", 1, "is older than 2.41"},
		{"image not runnable", "echo 'Unable to find image' >&2; exit 125\n", 2, "COULD-NOT-CHECK"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			code, out := runGitFloor(t, sh, standIn(t, "docker", tc.docker), false, "example/desk-base:test")
			if code != tc.wantCode || !strings.Contains(out, tc.wantText) {
				t.Fatalf("exit %d, want %d with %q\n%s", code, tc.wantCode, tc.wantText, out)
			}
		})
	}

	t.Run("docker absent", func(t *testing.T) {
		// PATH holds only a git: the image form must not fall back to the
		// local git when it cannot reach the image.
		code, out := runGitFloor(t, sh, standIn(t, "git", fakeVersionGit(git, "git version 2.47.3", false)), true, "example/desk-base:test")
		if code != 2 || !strings.Contains(out, "docker is not on PATH") {
			t.Fatalf("no docker: exit %d, want 2\n%s", code, out)
		}
	})
}

// dfInstr is one Dockerfile instruction with its continuation lines joined.
type dfInstr struct {
	op   string   // FROM, RUN, COPY, …
	args []string // whitespace-split words, with a trailing ';' trimmed
}

func parseDockerfile(text string) []dfInstr {
	var out []dfInstr
	var cur strings.Builder
	flush := func() {
		f := strings.Fields(cur.String())
		cur.Reset()
		if len(f) == 0 {
			return
		}
		args := make([]string, 0, len(f)-1)
		for _, a := range f[1:] {
			if a = strings.TrimSuffix(a, ";"); a != "" && a != `\` {
				args = append(args, a)
			}
		}
		out = append(out, dfInstr{op: strings.ToUpper(f[0]), args: args})
	}
	for _, l := range strings.Split(text, "\n") {
		t := strings.TrimSpace(l)
		if strings.HasPrefix(t, "#") {
			continue
		}
		if strings.HasSuffix(t, `\`) {
			cur.WriteString(strings.TrimSuffix(t, `\`) + " ")
			continue
		}
		cur.WriteString(t)
		flush()
	}
	flush()
	return out
}

func hasWord(args []string, w string) bool {
	for _, a := range args {
		if a == w {
			return true
		}
	}
	return false
}

// dockerfileFloorProblem returns why a base Dockerfile does not hold the git
// floor, or "" when it does. It requires: the gitbuild stage checks the
// tarball's sha256; the final stage takes git from gitbuild, never from apt
// (bookworm's apt git is 2.39.5); and the final stage runs git-floor-check
// after both git and the script are in place.
func dockerfileFloorProblem(dockerfile string) string {
	ins := parseDockerfile(dockerfile)
	lastFrom, gitbuild := -1, -1
	for i, in := range ins {
		if in.op != "FROM" {
			continue
		}
		lastFrom = i
		if len(in.args) >= 3 && in.args[len(in.args)-2] == "AS" && in.args[len(in.args)-1] == "gitbuild" {
			gitbuild = i
		}
	}
	if gitbuild < 0 {
		return "no `AS gitbuild` stage"
	}
	if gitbuild == lastFrom {
		return "gitbuild is the final stage"
	}
	verified := false
	for i := gitbuild + 1; i < len(ins) && ins[i].op != "FROM"; i++ {
		if ins[i].op == "RUN" && hasWord(ins[i].args, "sha256sum") && hasWord(ins[i].args, "-c") {
			verified = true
		}
	}
	if !verified {
		return "the gitbuild stage never checks the tarball's sha256"
	}
	gitIn, copyCheck, runCheck := -1, -1, -1
	for i := lastFrom + 1; i < len(ins); i++ {
		in := ins[i]
		switch {
		case in.op == "RUN" && hasWord(in.args, "apt-get") && hasWord(in.args, "install") && hasWord(in.args, "git"):
			return "the final stage apt-installs git (bookworm's is 2.39.5, below the floor)"
		case in.op == "COPY" && hasWord(in.args, "--from=gitbuild"):
			gitIn = i
		case in.op == "COPY" && len(in.args) == 2 && in.args[0] == "containers/scripts/git-floor-check.sh" && in.args[1] == gitFloorScriptInImg:
			copyCheck = i
		case in.op == "RUN" && (hasWord(in.args, "git-floor-check") || hasWord(in.args, gitFloorScriptInImg)):
			runCheck = i
		}
	}
	switch {
	case gitIn < 0:
		return "the final stage does not COPY git --from=gitbuild"
	case copyCheck < 0:
		return "the final stage does not COPY containers/scripts/git-floor-check.sh to " + gitFloorScriptInImg
	case runCheck < 0:
		return "the final stage never runs git-floor-check"
	case runCheck < gitIn || runCheck < copyCheck:
		return "git-floor-check runs before git or the script is in place"
	}
	return ""
}

// TestBaseImageRunsGitFloor pins the build-time half: the real base Dockerfile
// holds the floor, and each mutant that drops or misplaces a piece of it is
// caught.
func TestBaseImageRunsGitFloor(t *testing.T) {
	skipIfFixtureAbsent(t, baseDockerfilePath, "containers/ is not part of this repository's published file set")
	raw, err := os.ReadFile(baseDockerfilePath)
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	if p := dockerfileFloorProblem(text); p != "" {
		t.Fatalf("containers/base/Dockerfile: %s", p)
	}

	gitCopy := "COPY --from=gitbuild /out/usr/local/ /usr/local/\n"
	checkCopy := "COPY containers/scripts/git-floor-check.sh " + gitFloorScriptInImg + "\n"
	runTail := "    fi; \\\n    git-floor-check\n"
	shaLine := `    echo "${GIT_TARBALL_SHA256}  /tmp/git.txz" | sha256sum -c -; \` + "\n"
	aptAnchor := "        libcurl3-gnutls \\\n"
	goAnchor := "# Go toolchain, pinned"
	for _, anchor := range []string{gitCopy, checkCopy, runTail, shaLine, aptAnchor, goAnchor} {
		if strings.Count(text, anchor) != 1 {
			t.Fatalf("the mutants below need %q exactly once in the Dockerfile", anchor)
		}
	}
	mutants := map[string]string{
		"run step dropped":      strings.Replace(text, runTail, "    fi\n", 1),
		"check copy dropped":    strings.Replace(text, checkCopy, "", 1),
		"git copy dropped":      strings.Replace(text, gitCopy, "", 1),
		"sha256 check dropped":  strings.Replace(text, shaLine, "", 1),
		"apt git reinstated":    strings.Replace(text, aptAnchor, "        git \\\n"+aptAnchor, 1),
		"git copied after test": strings.Replace(strings.Replace(text, gitCopy, "", 1), goAnchor, gitCopy+goAnchor, 1),
		"check only in earlier": text + "\nFROM scratch\n",
	}
	for name, m := range mutants {
		if m == text {
			t.Fatalf("mutant %q did not change the Dockerfile", name)
		}
		if p := dockerfileFloorProblem(m); p == "" {
			t.Errorf("mutant %q: the broken floor was not caught", name)
		} else {
			t.Logf("mutant %q caught: %s", name, p)
		}
	}
}
