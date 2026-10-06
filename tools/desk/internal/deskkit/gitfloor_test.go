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
//   - the base Dockerfile still builds git from a reviewed, sha256-checked
//     tarball, keeps bookworm's apt git out, and runs the script, fail-closed,
//     as the last layer-writing step of the final stage
//     (TestBaseImageRunsGitFloor), so a base change cannot drop, mask or
//     outrun the build-time check without going red here.

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
	path := pathDir + string(os.PathListSeparator) + os.Getenv("PATH")
	if only {
		path = pathDir
	}
	return runGitFloorIn(t, sh, "", path, nil, args...)
}

// runGitFloorIn runs the shipped script in dir (the test's own directory when
// empty) with exactly path as PATH and env overriding the inherited
// environment.
func runGitFloorIn(t *testing.T, sh, dir, path string, env map[string]string, args ...string) (int, string) {
	t.Helper()
	script, err := filepath.Abs(gitFloorScriptPath)
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(sh, append([]string{script}, args...)...)
	cmd.Dir = dir
	base := envWithout(os.Environ(), "PATH")
	for k := range env {
		base = envWithout(base, k)
	}
	cmd.Env = append(base, "PATH="+path)
	for k, v := range env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
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

	// A hostile user config must not reach the probe. Each setting below,
	// if read, breaks it: attr.tree makes the control render CRLF, and the
	// signing and hook settings make the probe's commit fail. Both end in
	// could-not-check, so a script that stops isolating the user's config
	// (drops GIT_CONFIG_GLOBAL=/dev/null) goes red here.
	t.Run("hostile user config ignored", func(t *testing.T) {
		home := t.TempDir()
		hooks := standIn(t, "pre-commit", "exit 1\n")
		hostile := "[attr]\n\ttree = HEAD\n[commit]\n\tgpgSign = true\n[gpg]\n\tprogram = false\n[core]\n\thooksPath = " + hooks + "\n"
		xdg := filepath.Join(home, "xdg")
		if err := os.MkdirAll(filepath.Join(xdg, "git"), 0o755); err != nil {
			t.Fatal(err)
		}
		for _, p := range []string{filepath.Join(home, ".gitconfig"), filepath.Join(xdg, "git", "config")} {
			if err := os.WriteFile(p, []byte(hostile), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		env := map[string]string{
			"HOME":              home,
			"XDG_CONFIG_HOME":   xdg,
			"GIT_CONFIG_GLOBAL": filepath.Join(home, ".gitconfig"),
			"GIT_ATTR_SOURCE":   "HEAD",
		}
		code, out := runGitFloorIn(t, sh, "", os.Getenv("PATH"), env)
		if code != 0 || !strings.Contains(out, "OK:") {
			t.Fatalf("capable git under a hostile user config: exit %d, want 0 with an OK line\n%s", code, out)
		}
	})
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
	crlfAlways := `case " $* " in *" cat-file "*)
  '` + git + `' "$@" | awk '{ sub(/\r$/, ""); printf "%s\r\n", $0 }'; exit 0 ;;
esac
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
		// Renders CRLF whatever --attr-source says. Only the plain-LF control
		// tells this apart from a working option, so it pins that control.
		{"renders CRLF without the option", crlfAlways, 2, "is not plain LF"},
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

	// An empty PATH entry means the current directory. An older git there is
	// reachable, so it must be swept, wherever the empty entry sits. Field
	// splitting drops a TRAILING empty entry, which is why that case is here.
	sep := string(os.PathListSeparator)
	for name, path := range map[string]func(first string) string{
		"older git in a trailing empty PATH entry": func(first string) string { return first + sep + os.Getenv("PATH") + sep },
		"older git in a doubled empty PATH entry":  func(first string) string { return first + sep + sep + os.Getenv("PATH") },
	} {
		t.Run(name, func(t *testing.T) {
			first := standIn(t, "git", "exec '"+git+"' \"$@\"\n")
			cwd := standIn(t, "git", fakeVersionGit(git, "git version 2.39.5", true))
			code, out := runGitFloorIn(t, sh, cwd, path(first), nil)
			if code != 1 || !strings.Contains(out, "./git: git version 2.39.5 is older than 2.41") {
				t.Fatalf("older git in the current directory, reached by an empty PATH entry: exit %d, want 1 naming ./git\n%s", code, out)
			}
		})
	}

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
	// The passing stand-in also pins the docker argv: no network, the boot
	// entrypoint overridden, and `--` before the ref so a ref cannot be read
	// as a docker flag.
	wantArgv := `[ "$*" = "run --rm -i --network none --entrypoint /bin/sh -- example/desk-base:test -s" ] || { echo "unexpected docker argv: $*" >&2; exit 3; }
`
	cases := []struct {
		name     string
		docker   string
		wantCode int
		wantText string
	}{
		{"image meets floor", wantArgv + "exec '" + sh + "' -s\n", 0, "meets the git floor"},
		{"image below floor", "PATH='" + oldGit + "':$PATH exec '" + sh + "' -s\n", 1, "is older than 2.41"},
		{"image not runnable", "echo 'Unable to find image' >&2; exit 125\n", 2, "COULD-NOT-CHECK"},
		// Exit 0 is not enough: whatever ran must be the check, and print its
		// OK line.
		{"image exits 0 without the check", "cat >/dev/null; exit 0\n", 2, "without the check's OK line"},
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
	// segs is the instruction text split on ';' into shell commands, each
	// with its whitespace collapsed to single spaces; empty ones are dropped.
	segs []string
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
		words := make([]string, 0, len(f)-1)
		for _, a := range f[1:] {
			if a != `\` {
				words = append(words, a)
			}
			if a = strings.TrimSuffix(a, ";"); a != "" && a != `\` {
				args = append(args, a)
			}
		}
		var segs []string
		for _, sg := range strings.Split(strings.Join(words, " "), ";") {
			if sg = strings.TrimSpace(sg); sg != "" {
				segs = append(segs, sg)
			}
		}
		out = append(out, dfInstr{op: strings.ToUpper(f[0]), args: args, segs: segs})
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
	return wordAt(args, w) >= 0
}

func wordAt(args []string, w string) int {
	for i, a := range args {
		if a == w {
			return i
		}
	}
	return -1
}

// knownGitTarballs is the reviewed record of each upstream git release the
// base image may build, each sha256 checked against the release signature
// when it was added. The Dockerfile's (GIT_VERSION, GIT_TARBALL_SHA256) pair
// must be one of these, so swapping either ARG alone goes red here. A bump
// adds a row in the same change as the Dockerfile (see its "To bump" note).
var knownGitTarballs = map[string]string{
	"2.56.0": "26c56c296b38c0695b26fa95f475f1d01704d2d38e73465ca30b0b2f5dc789d3",
}

// failClosedProblem returns why a RUN step could let a failing command pass:
// it does not start with `set -e…`, turns -e off, masks a status with ||, or
// can `exit` 0 (a bare `exit` or `exit 0`) before the commands after it run.
func failClosedProblem(args []string) string {
	if len(args) < 2 || args[0] != "set" || !strings.HasPrefix(args[1], "-") || !strings.Contains(args[1], "e") {
		return "does not start with `set -e`"
	}
	for i, a := range args {
		if strings.Contains(a, "||") {
			return "masks a status with ||"
		}
		if strings.HasPrefix(a, "+") && strings.Contains(a, "e") {
			return "turns `set -e` off"
		}
		if a == "exit" && (i+1 == len(args) || strings.Trim(args[i+1], "0123456789") != "" || strings.Trim(args[i+1], "0") == "") {
			return "can exit 0 before its later commands run"
		}
	}
	return ""
}

// segAt returns the index of the first command in segs equal to want, or -1.
func segAt(segs []string, want string) int {
	for i, sg := range segs {
		if sg == want {
			return i
		}
	}
	return -1
}

// gateProblem returns why segs lacks the build gate `<cond>; then …; exit 1;
// fi`, which fails the build whenever cond succeeds.
func gateProblem(segs []string, cond, name string) string {
	i := segAt(segs, cond)
	if i < 0 || i+1 >= len(segs) || !strings.HasPrefix(segs[i+1], "then ") {
		return "lacks the " + name + " gate"
	}
	for j := i + 1; j < len(segs) && segs[j] != "fi"; j++ {
		if segs[j] == "exit 1" || segs[j] == "then exit 1" {
			return ""
		}
	}
	return "has a " + name + " gate that does not exit 1"
}

// Commands the static check pins in the base Dockerfile. Each one is a
// hardening step the reviews asked for; dropping it must go red.
const (
	floorTestGit        = "test -x /usr/local/bin/git"
	floorTestRemoteHTTP = "test -x /usr/local/libexec/git-core/git-remote-http"
	floorLddGate        = "if ldd /usr/local/bin/git /usr/local/libexec/git-core/git-remote-http | grep 'not found'"
	floorOwnerGate      = `if find /usr/local -xdev \( ! -user 0 -o ! -group 0 \) -print | grep .`
	gitCurlHTTPSOnly    = "--proto '=https' --proto-redir '=https'"
)

// floorRunProblem returns why the final stage's floor RUN does not fail the
// build on a missing binary, a missing shared library, a non-root file under
// /usr/local, or a git below the floor. The check must be the RUN's last
// command and stand bare: `&& true`, `| cat`, `!`, `&` or an `if` around it
// would let a failing check pass.
func floorRunProblem(in dfInstr) string {
	if p := failClosedProblem(in.args); p != "" {
		return p
	}
	segs := in.segs
	if last := segs[len(segs)-1]; last != "git-floor-check" && last != gitFloorScriptInImg {
		return "does not end with a bare `git-floor-check` (its last command is `" + last + "`)"
	}
	tg, tr := segAt(segs, floorTestGit), segAt(segs, floorTestRemoteHTTP)
	if tg < 0 || tr < 0 {
		return "lacks `" + floorTestGit + "` or `" + floorTestRemoteHTTP + "` (ldd alone lets a missing binary pass)"
	}
	if l := segAt(segs, floorLddGate); l < 0 || l < tg || l < tr {
		return "lacks the ldd gate after the test -x lines"
	}
	if p := gateProblem(segs, floorLddGate, "ldd"); p != "" {
		return p
	}
	return gateProblem(segs, floorOwnerGate, "root-ownership")
}

// dockerfileFloorProblem returns why a base Dockerfile does not hold the git
// floor, or "" when it does. It requires:
//   - the gitbuild stage pins a known (version, sha256) pair and downloads,
//     then checks the sha256, then unpacks, in one fail-closed RUN;
//   - the final stage takes git from gitbuild, never from apt (bookworm's apt
//     git is 2.39.5);
//   - the final stage runs git-floor-check, fail-closed, after git and the
//     script are in place, as its last RUN, COPY or ADD, so no later layer can
//     bring in a git unchecked.
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
	args := map[string]string{}
	verified := ""
	for i := gitbuild + 1; i < len(ins) && ins[i].op != "FROM"; i++ {
		in := ins[i]
		if in.op == "ARG" && len(in.args) == 1 {
			if k, v, ok := strings.Cut(in.args[0], "="); ok {
				args[k] = v
			}
		}
		if in.op != "RUN" || !hasWord(in.args, "sha256sum") {
			continue
		}
		// The check's own command must be `echo "<sha>  <file>" | sha256sum
		// -c -` and nothing more: a pipeline's status is its last command's,
		// so a trailing `| cat`, `&& true` or `&`, a leading `!`, or an `if`
		// around it would let a mismatch pass.
		curl, sha, tar := -1, -1, segAt(in.segs, "tar -C /tmp -xJf /tmp/git.txz")
		for i, sg := range in.segs {
			switch {
			case strings.HasPrefix(sg, "curl ") && strings.Contains(sg, "git-${GIT_VERSION}.tar") && strings.HasSuffix(sg, "-o /tmp/git.txz"):
				curl = i
			case strings.HasPrefix(sg, `echo "${GIT_TARBALL_SHA256} /tmp/git.txz" |`) && strings.HasSuffix(sg, "| sha256sum -c -") && strings.Count(sg, "|") == 1:
				sha = i
			}
		}
		switch {
		case verified != "":
			return "the gitbuild stage has more than one sha256 RUN"
		case sha < 0:
			verified = "the gitbuild RUN has no bare `echo \"${GIT_TARBALL_SHA256}  /tmp/git.txz\" | sha256sum -c -` command"
		case curl < 0 || tar < 0 || !(curl < sha && sha < tar):
			verified = "the gitbuild RUN does not download git-${GIT_VERSION}.tar, then check the sha256, then unpack"
		case !strings.Contains(in.segs[curl], gitCurlHTTPSOnly):
			verified = "the gitbuild curl does not carry " + gitCurlHTTPSOnly + ", so a redirect could leave https"
		case failClosedProblem(in.args) != "":
			verified = "the gitbuild sha256 RUN " + failClosedProblem(in.args)
		default:
			verified = "ok"
		}
	}
	if verified == "" {
		return "the gitbuild stage never checks the tarball's sha256"
	}
	if verified != "ok" {
		return verified
	}
	if want, ok := knownGitTarballs[args["GIT_VERSION"]]; !ok || args["GIT_TARBALL_SHA256"] != want {
		return "the gitbuild ARGs pin GIT_VERSION=" + args["GIT_VERSION"] + " GIT_TARBALL_SHA256=" + args["GIT_TARBALL_SHA256"] + ", which is not a reviewed pair in knownGitTarballs"
	}
	gitIn, copyCheck, runCheck, lastLayer := -1, -1, -1, -1
	for i := lastFrom + 1; i < len(ins); i++ {
		in := ins[i]
		if in.op == "RUN" || in.op == "COPY" || in.op == "ADD" {
			lastLayer = i
		}
		switch {
		case in.op == "RUN" && hasWord(in.args, "apt-get") && hasWord(in.args, "install") && hasWord(in.args, "git"):
			return "the final stage apt-installs git (bookworm's is 2.39.5, below the floor)"
		case in.op == "COPY" && hasWord(in.args, "--from=gitbuild"):
			gitIn = i
		case in.op == "COPY" && len(in.args) == 2 && in.args[0] == "containers/scripts/git-floor-check.sh" && in.args[1] == gitFloorScriptInImg:
			copyCheck = i
		case in.op == "RUN" && (hasWord(in.args, "git-floor-check") || hasWord(in.args, gitFloorScriptInImg)):
			runCheck = i
		case in.op == "ENV" && runCheck >= 0 && (hasWord(in.args, "PATH") || strings.Contains(" "+strings.Join(in.args, " "), " PATH=")):
			return "an ENV after git-floor-check rewrites PATH, so the image can resolve a git the check never saw"
		}
		if in.op == "RUN" {
			for _, sg := range in.segs {
				if strings.HasPrefix(sg, "tar ") && strings.Contains(sg, "-C /usr/local ") && !strings.Contains(sg, "--no-same-owner") {
					return "a final-stage tar unpacks into /usr/local without --no-same-owner (`" + sg + "`)"
				}
			}
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
	case runCheck != lastLayer:
		return "a RUN, COPY or ADD follows git-floor-check in the final stage, so its layer ships unchecked"
	case floorRunProblem(ins[runCheck]) != "":
		return "the git-floor-check RUN " + floorRunProblem(ins[runCheck])
	}
	return ""
}

// TestBaseImageRunsGitFloor pins the build-time half: the real base Dockerfile
// holds the floor, and each mutant that drops or misplaces a piece of it is
// caught.
func TestBaseImageRunsGitFloor(t *testing.T) {
	if runtime.GOOS == "windows" {
		// The mutant anchors below are LF-terminated; a CRLF checkout would
		// miss them. The Dockerfile is built on Linux only.
		t.Skip("the base Dockerfile is a Linux build input; its mutant anchors assume an LF checkout")
	}
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
	tarLine := "    tar -C /tmp -xJf /tmp/git.txz; \\\n"
	shaArg := "ARG GIT_TARBALL_SHA256=" + knownGitTarballs["2.56.0"] + "\n"
	verArg := "ARG GIT_VERSION=2.56.0\n"
	testX := "    test -x /usr/local/bin/git; \\\n"
	aptAnchor := "        libcurl3-gnutls \\\n"
	volAnchor := "VOLUME /work\n"
	testRemote := "    test -x /usr/local/libexec/git-core/git-remote-http; \\\n"
	curlLine := "    curl -fsSL --proto '=https' --proto-redir '=https' \\\n"
	lddGate := "    if ldd /usr/local/bin/git /usr/local/libexec/git-core/git-remote-http | grep 'not found'; then \\\n" +
		"        echo \"source-built git is missing a shared library\" >&2; \\\n        exit 1; \\\n    fi; \\\n"
	ownerGate := "    if find /usr/local -xdev \\( ! -user 0 -o ! -group 0 \\) -print | grep .; then \\\n" +
		"        echo \"/usr/local holds files not owned by root:root\" >&2; \\\n        exit 1; \\\n"
	goTar := "tar -C /usr/local --no-same-owner -xzf"
	nodeTar := "tar -C /usr/local --no-same-owner --strip-components=1"
	for _, anchor := range []string{gitCopy, checkCopy, runTail, shaLine, tarLine, shaArg, verArg, testX, aptAnchor, volAnchor,
		testRemote, curlLine, lddGate, ownerGate, goTar, nodeTar} {
		if strings.Count(text, anchor) != 1 {
			t.Fatalf("the mutants below need %q exactly once in the Dockerfile", anchor)
		}
	}
	floorTail := func(last string) string { return strings.Replace(text, runTail, "    fi; \\\n    "+last+"\n", 1) }
	shaSwap := func(cmd string) string {
		return strings.Replace(text, shaLine, strings.Replace(shaLine, "sha256sum -c -;", cmd, 1), 1)
	}
	mutants := map[string]string{
		"run step dropped":          strings.Replace(text, runTail, "    fi\n", 1),
		"check copy dropped":        strings.Replace(text, checkCopy, "", 1),
		"git copy dropped":          strings.Replace(text, gitCopy, "", 1),
		"sha256 check dropped":      strings.Replace(text, shaLine, "", 1),
		"apt git reinstated":        strings.Replace(text, aptAnchor, "        git \\\n"+aptAnchor, 1),
		"git copied after test":     strings.Replace(strings.Replace(text, gitCopy, "", 1), volAnchor, gitCopy+volAnchor, 1),
		"check only in earlier":     text + "\nFROM scratch\n",
		"sha256 check masked":       strings.Replace(text, shaLine, strings.Replace(shaLine, "sha256sum -c -;", "sha256sum -c - || true;", 1), 1),
		"floor check masked":        strings.Replace(text, runTail, "    fi; \\\n    git-floor-check || true\n", 1),
		"floor RUN set +e":          strings.Replace(text, testX, "    set +e; \\\n"+testX, 1),
		"hash checked after unpack": strings.Replace(strings.Replace(text, tarLine, "", 1), shaLine, tarLine+shaLine, 1),
		"tarball sha swapped":       strings.Replace(text, shaArg, "ARG GIT_TARBALL_SHA256="+strings.Repeat("0", 64)+"\n", 1),
		"git version swapped":       strings.Replace(text, verArg, "ARG GIT_VERSION=2.39.5\n", 1),
		"later layer copies a git":  strings.Replace(text, volAnchor, "COPY --from=desktools /usr/local/bin/git /usr/local/bin/git\n"+volAnchor, 1),
		"later RUN after check":     strings.Replace(text, volAnchor, "RUN apt-get update\n"+volAnchor, 1),
		// #2320 item 1: shapes that keep the right words but stop a failing
		// check from failing the build.
		"floor check && true":       floorTail("git-floor-check && true"),
		"floor check piped":         floorTail("git-floor-check | cat"),
		"floor check negated":       floorTail("! git-floor-check"),
		"floor check backgrounded":  floorTail("git-floor-check &"),
		"floor check in an if":      floorTail("if git-floor-check; then :; fi"),
		"exit 0 before the check":   floorTail("exit 0; \\\n    git-floor-check"),
		"sha256 check && true":      shaSwap("sha256sum -c - && true;"),
		"sha256 check piped":        shaSwap("sha256sum -c - | cat;"),
		"sha256 check backgrounded": shaSwap("sha256sum -c - & wait;"),
		"sha256 check negated":      strings.Replace(text, shaLine, strings.Replace(shaLine, "echo ", "! echo ", 1), 1),
		"sha256 check in an if":     strings.Replace(text, shaLine, strings.Replace(strings.Replace(shaLine, "echo ", "if echo ", 1), "sha256sum -c -;", "sha256sum -c -; then :; fi;", 1), 1),
		"exit 0 before sha256":      strings.Replace(text, shaLine, "    exit 0; \\\n"+shaLine, 1),
		"ENV PATH after check":      strings.Replace(text, volAnchor, "ENV PATH=/opt/old-git/bin:$PATH\n"+volAnchor, 1),
		// #2320 item 2: hardening that must not go missing unnoticed.
		"curl https-only dropped":      strings.Replace(text, curlLine, "    curl -fsSL \\\n", 1),
		"test -x git dropped":          strings.Replace(text, testX, "", 1),
		"test -x remote-http dropped":  strings.Replace(text, testRemote, "", 1),
		"ldd gate dropped":             strings.Replace(text, lddGate, "", 1),
		"root-owner gate dropped":      strings.Replace(text, ownerGate+"    fi; \\\n", "", 1),
		"Go --no-same-owner dropped":   strings.Replace(text, goTar, "tar -C /usr/local -xzf", 1),
		"Node --no-same-owner dropped": strings.Replace(text, nodeTar, "tar -C /usr/local --strip-components=1", 1),
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
