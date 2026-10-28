package deskkit

import (
	"os"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

// The git floor for the verify-runner image (#4207).
//
// The admitted verify runner calls /usr/bin/git with a fixed PATH and renders a
// converted file with `git --attr-source`, which arrived in git 2.41. The
// runner image started FROM the Go bookworm image, whose git is 2.39.5, so every
// brief that ships an eol-converted file was refused at admission (#2318 fixed
// the same floor for containers/base only).
//
// images/verify-runner/Dockerfile now builds git in a `gitbuild` stage from the
// SAME pinned tarball and sha256 as containers/base/Dockerfile, installs it over
// /usr, and runs containers/scripts/git-floor-check.sh with the runner's own fixed
// PATH as the last layer. The PR CI runners have no Docker daemon and the image
// is built only on release, so this test proves what can be proven on every PR:
// the two Dockerfiles pin one git, the runner builds it with the same
// fail-closed download-check-unpack step, the distro git is out, and the floor
// check cannot be dropped, masked, or outrun.

const (
	verifyRunnerDockerfilePath = fixtureRepoRoot + "/images/verify-runner/Dockerfile"
	// runnerFixedPath is the PATH the admitted runner gives /usr/bin/git's caller.
	runnerFixedPath = "PATH=/usr/bin:/bin:/usr/sbin:/sbin"
	// runnerFloorCmd is the last command of the floor RUN: the shipped script,
	// under the runner's fixed PATH, bare so a failure fails the build.
	runnerFloorCmd = runnerFixedPath + " " + gitFloorScriptInImg
	// runnerOwnerGate fails the build on any file of the install tree that is
	// not root:root.
	runnerOwnerGate = `if find /usr/bin/git /usr/libexec/git-core /usr/share/git-core -xdev \( ! -user 0 -o ! -group 0 \) -print | grep .`
)

// verifyRunnerFloorProblem returns why the verify-runner Dockerfile does not
// hold the git floor, or "" when it does. base is containers/base/Dockerfile's
// text. It requires:
//   - the gitbuild stage downloads, checks and unpacks the tarball in one
//     fail-closed RUN, pins a reviewed (version, sha256) pair, and that pair is
//     the base image's own, so the fleet has one git and no second pin;
//   - the gitbuild stage and the final stage start FROM the same reference, so
//     git links against the libraries of the image it ships in;
//   - the final stage purges the distro git, never installs it, takes the built
//     one from gitbuild into /usr, and puts the check script in place;
//   - the floor RUN is the final stage's last RUN, COPY or ADD, is fail-closed,
//     tests the binaries, gates on ldd, and ends with the bare script under the
//     runner's fixed PATH.
func verifyRunnerFloorProblem(runner, base string) string {
	ins := parseDockerfile(runner)
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
	if ins[gitbuild].args[0] != ins[lastFrom].args[0] {
		return "gitbuild starts FROM " + ins[gitbuild].args[0] + " but the final stage from " + ins[lastFrom].args[0] + ", so git is built against other libraries than it runs on"
	}
	// Both stages share the one reference, so checking each is checking it; do
	// both anyway so a future split of the two cannot leave one unguarded.
	for _, i := range []int{gitbuild, lastFrom} {
		if p := runnerBaseProblem(ins, i); p != "" {
			return p
		}
	}
	args, problem := gitbuildStageProblem(ins, gitbuild)
	if problem != "" {
		return problem
	}
	if p := knownPairProblem(args); p != "" {
		return p
	}

	baseIns := parseDockerfile(base)
	baseGitbuild := -1
	for i, in := range baseIns {
		if in.op == "FROM" && len(in.args) >= 3 && in.args[len(in.args)-2] == "AS" && in.args[len(in.args)-1] == "gitbuild" {
			baseGitbuild = i
		}
	}
	if baseGitbuild < 0 {
		return "containers/base/Dockerfile has no `AS gitbuild` stage to compare the pin with"
	}
	baseArgs, bp := gitbuildStageProblem(baseIns, baseGitbuild)
	if bp != "" {
		return "containers/base/Dockerfile: " + bp
	}
	for _, k := range []string{"GIT_VERSION", "GIT_TARBALL_SHA256"} {
		if args[k] != baseArgs[k] {
			return "the gitbuild ARG " + k + "=" + args[k] + " differs from containers/base/Dockerfile's " + baseArgs[k] + ": the fleet pins one git"
		}
	}

	gitIn, copyCheck, purge, runCheck, lastLayer := -1, -1, -1, -1, -1
	for i := lastFrom + 1; i < len(ins); i++ {
		in := ins[i]
		if in.op == "RUN" || in.op == "COPY" || in.op == "ADD" {
			lastLayer = i
		}
		switch {
		case in.op == "RUN" && aptInstallsGit(in.args):
			return "the final stage apt-installs git (bookworm's is 2.39.5, below the floor)"
		case in.op == "RUN" && hasWord(in.args, "apt-get") && hasWord(in.args, "purge") && hasWord(in.args, "git"):
			purge = i
		case in.op == "COPY" && len(in.args) == 3 && in.args[0] == "--from=gitbuild" && in.args[1] == "/out/usr/" && in.args[2] == "/usr/":
			gitIn = i
		case in.op == "COPY" && hasWord(in.args, "--from=gitbuild"):
			return "the final stage copies from gitbuild somewhere other than `/out/usr/ /usr/`, so /usr/bin/git is not the built one"
		case in.op == "COPY" && len(in.args) == 2 && in.args[0] == "containers/scripts/git-floor-check.sh" && in.args[1] == gitFloorScriptInImg:
			copyCheck = i
		case in.op == "RUN" && hasWord(in.args, "git-floor-check") || in.op == "RUN" && hasWord(in.args, gitFloorScriptInImg):
			runCheck = i
		}
	}
	switch {
	case purge < 0:
		return "the final stage never purges the distro git, so its 2.39.5 files and package record stay in the image"
	case gitIn < 0:
		return "the final stage does not COPY git --from=gitbuild /out/usr/ /usr/"
	case purge > gitIn:
		return "the distro git is purged after the built git is copied in, which would delete the files just installed"
	case copyCheck < 0:
		return "the final stage does not COPY containers/scripts/git-floor-check.sh to " + gitFloorScriptInImg
	case runCheck < 0:
		return "the final stage never runs git-floor-check"
	case runCheck < gitIn || runCheck < copyCheck:
		return "git-floor-check runs before git or the script is in place"
	case runCheck != lastLayer:
		return "a RUN, COPY or ADD follows git-floor-check in the final stage, so its layer ships unchecked"
	}
	return runnerFloorRunProblem(ins[runCheck])
}

// runnerBaseRe is the one base the runner may start FROM: the fleet's Go
// toolchain image, by tag and by digest (#4207, F-runner-base-unguarded).
// TestGoSingleVersion reads the tag of an `ARG NAME=golang:…` line; this holds
// the digest, which that test does not, and that the FROM actually resolves to
// the ARG's value rather than to some other image.
var runnerBaseRe = regexp.MustCompile(`^golang:` + regexp.QuoteMeta(fleetGoToolchain) + `-[A-Za-z0-9._-]+@sha256:[0-9a-f]{64}$`)

// runnerBaseProblem returns why the FROM at ins[from], resolved through the
// ARGs that precede the first FROM, is not `golang:<fleetGoToolchain>-…@sha256:…`.
func runnerBaseProblem(ins []dfInstr, from int) string {
	globals := map[string]string{}
	for _, in := range ins {
		if in.op == "FROM" {
			break
		}
		if in.op == "ARG" && len(in.args) == 1 {
			if k, v, ok := strings.Cut(in.args[0], "="); ok {
				globals[k] = v
			}
		}
	}
	ref := ""
	for _, a := range ins[from].args {
		if !strings.HasPrefix(a, "--") {
			ref = a
			break
		}
	}
	shown := ref
	if strings.HasPrefix(ref, "${") && strings.HasSuffix(ref, "}") {
		ref = globals[strings.TrimSuffix(strings.TrimPrefix(ref, "${"), "}")]
	}
	if !runnerBaseRe.MatchString(ref) {
		return "FROM " + shown + " resolves to `" + ref + "`, not golang:" + fleetGoToolchain + "-<variant>@sha256:<64 hex>, the fleet's digest-pinned Go image"
	}
	return ""
}

// aptInstallsGit reports whether any `apt-get install` in a RUN's words lists
// the package git. The list ends at the next `&&`, `apt-get` or `rm`, so a
// `purge git` later in the same RUN is not an install.
func aptInstallsGit(args []string) bool {
	installing := false
	for i, a := range args {
		switch {
		case a == "install" && i > 0 && args[i-1] == "apt-get":
			installing = true
		case a == "&&" || a == "apt-get" || a == "rm":
			installing = false
		case installing && a == "git":
			return true
		}
	}
	return false
}

// runnerFloorRunProblem returns why the floor RUN does not fail the build on a
// missing binary, a missing shared library, or a git below the floor. The check
// must be the RUN's last command, bare, under the runner's fixed PATH.
func runnerFloorRunProblem(in dfInstr) string {
	if p := failClosedProblem(in.args); p != "" {
		return "the git-floor-check RUN " + p
	}
	segs := in.segs
	if last := segs[len(segs)-1]; last != runnerFloorCmd {
		return "the git-floor-check RUN does not end with the bare `" + runnerFloorCmd + "` (its last command is `" + last + "`)"
	}
	tg, tr := segAt(segs, "test -x /usr/bin/git"), segAt(segs, "test -x /usr/libexec/git-core/git-remote-http")
	if tg < 0 || tr < 0 {
		return "the git-floor-check RUN lacks `test -x /usr/bin/git` or `test -x /usr/libexec/git-core/git-remote-http` (ldd alone lets a missing binary pass)"
	}
	const lddGate = "if ldd /usr/bin/git /usr/libexec/git-core/git-remote-http | grep 'not found'"
	if l := segAt(segs, lddGate); l < 0 || l < tg || l < tr {
		return "the git-floor-check RUN lacks the ldd gate after the test -x lines"
	}
	if p := gateProblem(segs, lddGate, "ldd"); p != "" {
		return "the git-floor-check RUN " + p
	}
	// The owner gate keeps the install tree root-owned and the last test proves
	// the distro git's exec directory is gone (both asserted by the Dockerfile's
	// own comment); a dropped one must go red here too.
	if p := gateProblem(segs, runnerOwnerGate, "root-ownership"); p != "" {
		return "the git-floor-check RUN " + p
	}
	if segAt(segs, "test ! -e /usr/lib/git-core") < 0 {
		return "the git-floor-check RUN lacks `test ! -e /usr/lib/git-core` (the distro git's exec directory must be gone)"
	}
	return ""
}

// regression: #4207
func TestVerifyRunnerImageRunsGitFloor(t *testing.T) {
	if runtime.GOOS == "windows" {
		// The mutant anchors below are LF-terminated; a CRLF checkout would
		// miss them. The Dockerfiles are Linux build inputs.
		t.Skip("the Dockerfiles are Linux build inputs; their mutant anchors assume an LF checkout")
	}
	skipIfFixtureAbsent(t, verifyRunnerDockerfilePath, "images/ is not part of this repository's published file set")
	skipIfFixtureAbsent(t, baseDockerfilePath, "containers/ is not part of this repository's published file set")
	rawRunner, err := os.ReadFile(verifyRunnerDockerfilePath)
	if err != nil {
		t.Fatal(err)
	}
	rawBase, err := os.ReadFile(baseDockerfilePath)
	if err != nil {
		t.Fatal(err)
	}
	text, base := string(rawRunner), string(rawBase)
	if p := verifyRunnerFloorProblem(text, base); p != "" {
		t.Fatalf("images/verify-runner/Dockerfile: %s", p)
	}

	shaArg := "ARG GIT_TARBALL_SHA256=" + knownGitTarballs["2.56.0"] + "\n"
	verArg := "ARG GIT_VERSION=2.56.0\n"
	shaLine := `    echo "${GIT_TARBALL_SHA256}  /tmp/git.txz" | sha256sum -c -; \` + "\n"
	tarLine := "    tar -C /tmp -xJf /tmp/git.txz; \\\n"
	curlLine := "    curl -fsSL --proto '=https' --proto-redir '=https' \\\n"
	gitbuildFrom := "FROM ${RUNNER_BASE} AS gitbuild\n"
	finalFrom := "FROM ${RUNNER_BASE}\n"
	purgeLine := " && apt-get purge -y git git-man \\\n"
	gitCopy := "COPY --from=gitbuild /out/usr/ /usr/\n"
	checkCopy := "COPY containers/scripts/git-floor-check.sh " + gitFloorScriptInImg + "\n"
	statusgenCopy := "COPY .release-images/verify-runner/statusgen-linux-${TARGETARCH} /usr/local/bin/statusgen\n"
	testX := "    test -x /usr/bin/git; \\\n"
	testRemote := "    test -x /usr/libexec/git-core/git-remote-http; \\\n"
	lddGate := "    if ldd /usr/bin/git /usr/libexec/git-core/git-remote-http | grep 'not found'; then \\\n" +
		"        echo \"source-built git is missing a shared library\" >&2; \\\n        exit 1; \\\n    fi; \\\n"
	floorCmd := "    " + runnerFloorCmd + "\n"
	aptAnchor := "      python3-jsonschema=4.10.3-1 \\\n"
	// The runner's base reference lives in one pre-FROM ARG; the mutants below
	// derive from the committed line, so a legitimate bump keeps them valid.
	baseArgLines := regexp.MustCompile(`(?m)^ARG RUNNER_BASE=\S+\n`).FindAllString(text, -1)
	if len(baseArgLines) != 1 {
		t.Fatalf("the mutants below need exactly one `ARG RUNNER_BASE=` line in the Dockerfile, found %d", len(baseArgLines))
	}
	baseArg := baseArgLines[0]
	digestless := regexp.MustCompile(`@sha256:[0-9a-f]{64}`).ReplaceAllString(baseArg, "")
	ownerGate := "    " + runnerOwnerGate + "; then \\\n" +
		"        echo \"the source-built git install tree holds files not owned by root:root\" >&2; \\\n        exit 1; \\\n    fi; \\\n"
	execDirTest := "    test ! -e /usr/lib/git-core; \\\n"
	floorRun := "RUN set -eux; \\\n" + testX
	for _, anchor := range []string{shaArg, verArg, shaLine, tarLine, curlLine, gitbuildFrom, finalFrom, purgeLine, gitCopy,
		checkCopy, statusgenCopy, testX, testRemote, lddGate, floorCmd, aptAnchor, baseArg, ownerGate, execDirTest, floorRun} {
		if strings.Count(text, anchor) != 1 {
			t.Fatalf("the mutants below need %q exactly once in the Dockerfile", anchor)
		}
	}
	floorTail := func(last string) string { return strings.Replace(text, floorCmd, "    "+last+"\n", 1) }
	shaSwap := func(cmd string) string {
		return strings.Replace(text, shaLine, strings.Replace(shaLine, "sha256sum -c -;", cmd, 1), 1)
	}
	mutants := map[string]string{
		// The reported instance: the runner ships the distro git.
		"distro git left in place":       strings.Replace(strings.Replace(text, gitCopy, "", 1), purgeLine, "", 1),
		"built git not copied":           strings.Replace(text, gitCopy, "", 1),
		"built git copied elsewhere":     strings.Replace(text, gitCopy, "COPY --from=gitbuild /out/usr/ /usr/local/\n", 1),
		"distro git kept":                strings.Replace(text, purgeLine, "", 1),
		"distro git reinstalled":         strings.Replace(text, aptAnchor, aptAnchor+"      git \\\n", 1),
		"distro git purged after copy":   strings.Replace(strings.Replace(text, purgeLine, "", 1), gitCopy, gitCopy+"RUN apt-get purge -y git\n", 1),
		"check script not copied":        strings.Replace(text, checkCopy, "", 1),
		"floor RUN dropped":              strings.Replace(text, "RUN set -eux; \\\n"+testX, "RUN set -eux; \\\n    true; \\\n", 1),
		"later layer after check":        text + "RUN apt-get update\n",
		"later copy after check":         text + "COPY --from=gitbuild /out/usr/bin/git /usr/bin/git\n",
		"gitbuild stage dropped":         strings.Replace(text, gitbuildFrom, "FROM scratch AS other\n", 1),
		"stages from different bases":    strings.Replace(text, gitbuildFrom, "FROM debian:bookworm-slim AS gitbuild\n", 1),
		"tarball sha swapped":            strings.Replace(text, shaArg, "ARG GIT_TARBALL_SHA256="+strings.Repeat("0", 64)+"\n", 1),
		"git version swapped":            strings.Replace(text, verArg, "ARG GIT_VERSION=2.39.5\n", 1),
		"sha256 check dropped":           strings.Replace(text, shaLine, "", 1),
		"sha256 check masked":            shaSwap("sha256sum -c - || true;"),
		"hash checked after unpack":      strings.Replace(strings.Replace(text, tarLine, "", 1), shaLine, tarLine+shaLine, 1),
		"tarball replaced after check":   strings.Replace(text, shaLine, shaLine+"    cp /etc/hostname /tmp/git.txz; \\\n", 1),
		"curl https-only dropped":        strings.Replace(text, curlLine, "    curl -fsSL \\\n", 1),
		"floor check masked":             floorTail("PATH=/usr/bin:/bin:/usr/sbin:/sbin /usr/local/bin/git-floor-check || true"),
		"floor check piped":              floorTail(runnerFloorCmd + " | cat"),
		"floor check backgrounded":       floorTail(runnerFloorCmd + " &"),
		"floor check negated":            floorTail("! " + runnerFloorCmd),
		"floor check in an if":           floorTail("if " + runnerFloorCmd + "; then :; fi"),
		"floor check under image PATH":   floorTail(gitFloorScriptInImg),
		"floor check bare name":          floorTail("git-floor-check"),
		"floor check fixed PATH dropped": floorTail("PATH=/usr/bin " + gitFloorScriptInImg),
		"exit 0 before the check":        floorTail("exit 0; \\\n    " + runnerFloorCmd),
		"floor RUN set +e":               strings.Replace(text, testX, "    set +e; \\\n"+testX, 1),
		"test -x git dropped":            strings.Replace(text, testX, "", 1),
		"test -x remote-http dropped":    strings.Replace(text, testRemote, "", 1),
		"ldd gate dropped":               strings.Replace(text, lddGate, "", 1),
		// The runner base (F-runner-base-digest-unproven): each of these keeps the
		// two FROM tokens equal, so only runnerBaseProblem can catch it.
		"runner base digest dropped": strings.Replace(text, baseArg, digestless, 1),
		"runner base tag changed":    strings.Replace(text, baseArg, strings.Replace(baseArg, "golang:"+fleetGoToolchain+"-", "golang:1.26.0-", 1), 1),
		"runner base ARG dropped":    strings.Replace(text, baseArg, "", 1),
		"both FROMs literal, no digest": strings.Replace(strings.Replace(text, gitbuildFrom, "FROM golang:"+fleetGoToolchain+"-bookworm AS gitbuild\n", 1),
			finalFrom, "FROM golang:"+fleetGoToolchain+"-bookworm\n", 1),
		"runner base other image": strings.Replace(text, baseArg, "ARG RUNNER_BASE=debian:bookworm-slim@sha256:"+strings.Repeat("b", 64)+"\n", 1),
		// The last-layer gates (T-new-guards-never-red).
		"owner gate dropped":            strings.Replace(text, ownerGate, "", 1),
		"owner gate masked":             strings.Replace(text, ownerGate, strings.Replace(ownerGate, "exit 1; \\\n    fi;", "true; \\\n    fi;", 1), 1),
		"distro exec dir check dropped": strings.Replace(text, execDirTest, "", 1),
		"floor RUN removed whole":       text[:strings.Index(text, floorRun)],
	}
	// The rule each of the newer mutants must trip, so a mutant caught for some
	// other reason (an earlier check) cannot stand in for its own guard.
	wantRule := map[string]string{
		"runner base digest dropped":    "not golang:" + fleetGoToolchain,
		"runner base tag changed":       "not golang:" + fleetGoToolchain,
		"runner base ARG dropped":       "not golang:" + fleetGoToolchain,
		"both FROMs literal, no digest": "not golang:" + fleetGoToolchain,
		"runner base other image":       "not golang:" + fleetGoToolchain,
		"owner gate dropped":            "root-ownership",
		"owner gate masked":             "root-ownership",
		"distro exec dir check dropped": "test ! -e /usr/lib/git-core",
		"floor RUN removed whole":       "never runs git-floor-check",
	}
	for name, m := range mutants {
		if m == text {
			t.Fatalf("mutant %q did not change the Dockerfile", name)
		}
		p := verifyRunnerFloorProblem(m, base)
		if p == "" {
			t.Errorf("mutant %q: the broken floor was not caught", name)
			continue
		}
		t.Logf("mutant %q caught: %s", name, p)
		if want, ok := wantRule[name]; ok && !strings.Contains(p, want) {
			t.Errorf("mutant %q: caught, but not by its own rule (want %q in %q)", name, want, p)
		}
	}

	// The pin is the base image's: a base that moves to another git, with the
	// runner left behind, must go red here rather than leave two pins to drift.
	t.Run("runner pin drifts from base", func(t *testing.T) {
		baseShaArg := "ARG GIT_TARBALL_SHA256=" + knownGitTarballs["2.56.0"] + "\n"
		baseVerArg := "ARG GIT_VERSION=2.56.0\n"
		if strings.Count(base, baseShaArg) != 1 || strings.Count(base, baseVerArg) != 1 {
			t.Fatalf("the base-drift mutants need the base git ARGs exactly once in containers/base/Dockerfile")
		}
		for name, drifted := range map[string]string{
			"base version bumped": strings.Replace(base, baseVerArg, "ARG GIT_VERSION=2.57.0\n", 1),
			"base sha changed":    strings.Replace(base, baseShaArg, "ARG GIT_TARBALL_SHA256="+strings.Repeat("a", 64)+"\n", 1),
		} {
			if p := verifyRunnerFloorProblem(text, drifted); !strings.Contains(p, "differs from containers/base/Dockerfile") {
				t.Errorf("%s: want a pin-drift problem, got %q", name, p)
			}
		}
		if p := verifyRunnerFloorProblem(text, "FROM scratch\n"); !strings.Contains(p, "no `AS gitbuild` stage") {
			t.Errorf("a base with no gitbuild stage: want a problem naming it, got %q", p)
		}
	})
}
