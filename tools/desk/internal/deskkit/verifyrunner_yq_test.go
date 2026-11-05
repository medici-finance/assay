package deskkit

import (
	"os"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

// The mikefarah yq in the verify-runner image (#5531).
//
// Rows run with no network, so a tool a row shells out to has to be in the
// image already. Rows that convert Job templates and fixtures call the
// mikefarah `yq` and refuse any other program of that name; the image had none,
// so every such row was could-not-check inside the admission bracket.
//
// images/verify-runner/Dockerfile now downloads the static yq binary for the
// build platform, checks it against a reviewed per-arch sha256 and installs it
// as /usr/local/bin/yq. The image is built only on release and the PR runners
// have no Docker daemon, so this static check is what makes dropping the binary,
// the pin or the check go red on a PR.

// yqBinary is the reviewed sha256 of one release's linux binary per arch.
type yqBinary struct{ amd64, arm64 string }

// knownYqBinaries is the reviewed record of each mikefarah yq release the
// runner may install, keyed by version. Each sha256 is the SHA-256 column of
// that release's `checksums` file (column order from `checksums_hashes_order`),
// matched against the downloaded asset when it was added. A bump adds a row in
// the same change as the Dockerfile.
var knownYqBinaries = map[string]yqBinary{
	"4.54.1": {
		amd64: "8e34fc298390875de416e6a4afcb8cabeceb25d9aa8506c1a2f9353cf702ea5f",
		arm64: "189088da0c6429ec5178dfaab1a114805f6cab0b61b165ab236efedf1d57a71b",
	},
}

const (
	yqDownload = "/tmp/yq"
	yqBin      = "/usr/local/bin/yq"
	// yqCheckCmd must stand bare: `|| true`, `| cat`, `!` or an `if` around
	// it would let a mismatching download pass. parseDockerfile collapses
	// whitespace, so the Dockerfile's two spaces read as one here.
	yqCheckCmd   = `echo "${yq_sha} /tmp/yq" | sha256sum -c -`
	yqInstallCmd = "install -o 0 -g 0 -m 0755 /tmp/yq " + yqBin
)

// yqCurlRe is the one download shape allowed: https only, redirects included,
// one output file, and the release URL for the build platform.
var yqCurlRe = regexp.MustCompile(`^curl -fsSL ` + regexp.QuoteMeta(gitCurlHTTPSOnly) +
	` --tlsv1\.2 -o /tmp/yq "https://github\.com/mikefarah/yq/releases/download/v([0-9]+\.[0-9]+\.[0-9]+)/yq_linux_\$\{TARGETARCH\}"$`)

// yqVersionCmd is the bare version gate the RUN ends with.
func yqVersionCmd(ver string) string {
	return `test "$(yq --version)" = 'yq (https://github.com/mikefarah/yq/) version v` + ver + `'`
}

// verifyRunnerYqProblem returns why the verify-runner Dockerfile does not
// install a checked mikefarah yq, or "" when it does. It requires exactly one
// final-stage RUN that:
//   - is fail-closed, maps amd64 and arm64 to the reviewed sha256 of the
//     version it downloads, and exits 1 on any other platform;
//   - downloads once, with the yqCurlRe shape, then runs the bare sha256 check,
//     then installs as the very next command, so nothing can replace the file
//     between the check and the install;
//   - ends with the bare version gate for the same version;
//
// and that nothing else in the final stage writes /usr/local/bin/yq or
// apt-installs the Debian `yq`, which is a different program.
func verifyRunnerYqProblem(dockerfile string) string {
	ins := parseDockerfile(dockerfile)
	lastFrom := -1
	for i, in := range ins {
		if in.op == "FROM" {
			lastFrom = i
		}
	}
	if lastFrom < 0 {
		return "no FROM"
	}
	run := -1
	for i := lastFrom + 1; i < len(ins); i++ {
		in := ins[i]
		names := strings.Contains(strings.Join(in.args, " "), yqDownload) || strings.Contains(strings.Join(in.args, " "), yqBin)
		if in.op == "RUN" && hasWord(in.args, "apt-get") && aptInstallsPkg(in.args, "yq") {
			return "the final stage apt-installs the Debian yq, which is not the mikefarah yq rows call"
		}
		if !names {
			continue
		}
		if in.op != "RUN" {
			return in.op + " `" + strings.Join(in.args, " ") + "` writes yq outside the checked RUN"
		}
		if run >= 0 {
			return "more than one RUN names " + yqDownload + " or " + yqBin + ", so the yq installed need not be the one checked"
		}
		run = i
	}
	if run < 0 {
		return "the final stage has no RUN that downloads, checks and installs yq (#5531), so rows that call it cannot run offline"
	}
	in := ins[run]
	if p := failClosedProblem(in.args); p != "" {
		return "the yq RUN " + p
	}
	ver, amdSha, armSha := "", "", ""
	curl, amd, arm, other, check, inst := -1, -1, -1, -1, -1, -1
	for i, sg := range in.segs {
		arm0 := strings.TrimPrefix(sg, `case "${TARGETARCH:-}" in `)
		words := strings.Fields(arm0)
		setsSha := false
		for _, w := range words {
			setsSha = setsSha || strings.HasPrefix(w, "yq_sha=")
		}
		switch {
		case fetchWords(sg) > 0:
			m := yqCurlRe.FindStringSubmatch(sg)
			if m == nil || fetchWords(sg) != 1 {
				return "the yq RUN fetches with something other than the one allowed curl shape (`" + sg + "`)"
			}
			if curl >= 0 {
				return "the yq RUN downloads more than once, so the file installed need not be the file checked"
			}
			curl, ver = i, m[1]
		case strings.HasPrefix(arm0, "amd64) ") && len(words) == 2 && setsSha:
			amd, amdSha = i, strings.TrimPrefix(words[1], "yq_sha=")
		case strings.HasPrefix(arm0, "arm64) ") && len(words) == 2 && setsSha:
			arm, armSha = i, strings.TrimPrefix(words[1], "yq_sha=")
		case strings.HasPrefix(arm0, "*) "):
			if i+1 >= len(in.segs) || in.segs[i+1] != "exit 1" {
				return "the yq RUN's other-platform arm does not exit 1, so an unpinned platform need not fail"
			}
			other = i
		case setsSha:
			return "the yq RUN sets yq_sha outside the amd64 and arm64 arms (`" + sg + "`), so the check need not use the reviewed pin"
		case sg == yqCheckCmd:
			check = i
		case sg == yqInstallCmd:
			inst = i
		}
	}
	rec, ok := knownYqBinaries[ver]
	last := in.segs[len(in.segs)-1]
	switch {
	case curl < 0:
		return "the yq RUN has no download"
	case !ok:
		return "the yq RUN downloads v" + ver + ", which is not a reviewed row in knownYqBinaries"
	case amd < 0 || arm < 0 || other < 0:
		return "the yq RUN does not map amd64 and arm64 to a pinned sha256 and fail every other platform"
	case amdSha != rec.amd64 || armSha != rec.arm64:
		return "the yq RUN pins amd64 " + amdSha + " and arm64 " + armSha + ", not the reviewed sha256s of v" + ver
	case check < 0:
		return "the yq RUN has no bare `" + yqCheckCmd + "` command"
	case inst < 0:
		return "the yq RUN does not install with `" + yqInstallCmd + "`"
	case !(amd < curl && arm < curl && other < curl && curl < check && check < inst):
		return "the yq RUN does not choose the pinned sha256 and download, then check, then install"
	case inst != check+1:
		return "the yq RUN runs `" + in.segs[check+1] + "` between the check and the install, so the file installed need not be the file checked"
	case last != yqVersionCmd(ver):
		return "the yq RUN does not end with the bare `" + yqVersionCmd(ver) + "` (its last command is `" + last + "`)"
	}
	return ""
}

// aptInstallsPkg reports whether an `apt-get install` in a RUN's words lists
// pkg, optionally with a `=version` pin. The list ends at the next `&&`,
// `apt-get` or `rm`.
func aptInstallsPkg(args []string, pkg string) bool {
	installing := false
	for i, a := range args {
		switch {
		case a == "install" && i > 0 && args[i-1] == "apt-get":
			installing = true
		case a == "&&" || a == "apt-get" || a == "rm":
			installing = false
		case installing && (a == pkg || strings.HasPrefix(a, pkg+"=")):
			return true
		}
	}
	return false
}

// regression: #5531
func TestVerifyRunnerImageInstallsYq(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the Dockerfile is a Linux build input; its mutant anchors assume an LF checkout")
	}
	skipIfFixtureAbsent(t, verifyRunnerDockerfilePath, "images/ is not part of this repository's published file set")
	raw, err := os.ReadFile(verifyRunnerDockerfilePath)
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	if p := verifyRunnerYqProblem(text); p != "" {
		t.Fatalf("images/verify-runner/Dockerfile: %s", p)
	}

	rec := knownYqBinaries["4.54.1"]
	runStart := "RUN set -eu; \\\n    case \"${TARGETARCH:-}\" in \\\n      amd64) yq_sha="
	amdLine := "      amd64) yq_sha=" + rec.amd64 + " ;; \\\n"
	armLine := "      arm64) yq_sha=" + rec.arm64 + " ;; \\\n"
	otherLine := "      *) echo \"verify-runner: no pinned yq checksum for '${TARGETARCH:-}'\" >&2; exit 1 ;; \\\n"
	curlLine := "    curl -fsSL --proto '=https' --proto-redir '=https' --tlsv1.2 -o /tmp/yq \\\n"
	urlLine := "      \"https://github.com/mikefarah/yq/releases/download/v4.54.1/yq_linux_${TARGETARCH}\"; \\\n"
	checkLine := "    echo \"${yq_sha}  /tmp/yq\" | sha256sum -c -; \\\n"
	installLine := "    install -o 0 -g 0 -m 0755 /tmp/yq /usr/local/bin/yq; \\\n"
	versionLine := "    " + yqVersionCmd("4.54.1") + "\n"
	aptAnchor := "      tmux=3.3a-3 \\\n"
	for _, anchor := range []string{runStart, amdLine, armLine, otherLine, curlLine, urlLine, checkLine, installLine, versionLine, aptAnchor} {
		if strings.Count(text, anchor) != 1 {
			t.Fatalf("the mutants below need %q exactly once in the Dockerfile", anchor)
		}
	}
	runEnd := strings.Index(text, versionLine) + len(versionLine)
	runText := text[strings.Index(text, runStart):runEnd]
	swap := func(old, repl string) string { return strings.Replace(text, old, repl, 1) }
	checkSwap := func(cmd string) string {
		return swap(checkLine, strings.Replace(checkLine, "sha256sum -c -;", cmd, 1))
	}
	zero := strings.Repeat("0", 64)

	mutants := map[string]string{
		// The reported instance: the recipe before this change, with no yq.
		"yq RUN removed (old recipe)": strings.Replace(text, runText, "", 1),
		// A wrong checksum must be refused, whichever arch it is on.
		"amd64 sha256 wrong":        swap(amdLine, "      amd64) yq_sha="+zero+" ;; \\\n"),
		"arm64 sha256 wrong":        swap(armLine, "      arm64) yq_sha="+zero+" ;; \\\n"),
		"amd64 and arm64 swapped":   swap(armLine, "      arm64) yq_sha="+rec.amd64+" ;; \\\n"),
		"version bumped alone":      swap(urlLine, strings.Replace(urlLine, "v4.54.1", "v4.54.2", 1)),
		"other arch arm dropped":    swap(otherLine, ""),
		"other arch exits 0":        swap(otherLine, strings.Replace(otherLine, "exit 1", "exit 0", 1)),
		"sha set after the case":    swap(curlLine, "    yq_sha="+zero+"; \\\n"+curlLine),
		"sha self-computed":         swap(checkLine, "    yq_sha=\"$(sha256sum /tmp/yq | cut -d' ' -f1)\"; \\\n"+checkLine),
		"set -e dropped":            swap(runStart, strings.Replace(runStart, "set -eu", "set -u", 1)),
		"set +e before check":       swap(checkLine, "    set +e; \\\n"+checkLine),
		"sha256 check dropped":      swap(checkLine, ""),
		"sha256 check masked":       checkSwap("sha256sum -c - || true;"),
		"sha256 check piped":        checkSwap("sha256sum -c - | cat;"),
		"sha256 check && true":      checkSwap("sha256sum -c - && true;"),
		"sha256 check negated":      swap(checkLine, strings.Replace(checkLine, "echo ", "! echo ", 1)),
		"sha256 check in an if":     swap(checkLine, strings.Replace(strings.Replace(checkLine, "echo ", "if echo ", 1), "sha256sum -c -;", "sha256sum -c -; then :; fi;", 1)),
		"install before check":      strings.Replace(swap(installLine, ""), checkLine, installLine+checkLine, 1),
		"file replaced after check": swap(checkLine, checkLine+"    cp /etc/hostname /tmp/yq; \\\n"),
		"refetch after check": swap(checkLine, checkLine+"    curl -fsSL --proto '=https' --proto-redir '=https' --tlsv1.2 -o /tmp/yq "+
			"\"https://github.com/mikefarah/yq/releases/download/v4.54.1/yq_linux_${TARGETARCH}\"; \\\n"),
		"https-only dropped":         swap(curlLine, "    curl -fsSL --tlsv1.2 -o /tmp/yq \\\n"),
		"other host":                 swap(urlLine, strings.Replace(urlLine, "github.com/mikefarah", "example.com/mikefarah", 1)),
		"install dropped":            swap(installLine, ""),
		"version gate dropped":       swap(versionLine, "    true\n"),
		"version gate masked":        swap(versionLine, "    "+yqVersionCmd("4.54.1")+" || true\n"),
		"version gate other version": swap(versionLine, "    "+yqVersionCmd("4.54.2")+"\n"),
		// The class: a second way into /usr/local/bin/yq, at a site this change
		// does not touch, must go red too.
		"plant: Debian yq via apt":      swap(aptAnchor, aptAnchor+"      yq \\\n"),
		"plant: yq copied over":         text + "COPY tools/yq /usr/local/bin/yq\n",
		"plant: second RUN replaces yq": text + "RUN set -eu; cp /bin/true /usr/local/bin/yq\n",
	}
	wantRule := map[string]string{
		"yq RUN removed (old recipe)":   "no RUN that downloads",
		"amd64 sha256 wrong":            "not the reviewed sha256s",
		"arm64 sha256 wrong":            "not the reviewed sha256s",
		"amd64 and arm64 swapped":       "not the reviewed sha256s",
		"version bumped alone":          "not a reviewed row",
		"plant: Debian yq via apt":      "Debian yq",
		"plant: yq copied over":         "outside the checked RUN",
		"plant: second RUN replaces yq": "more than one RUN",
	}
	for name, m := range mutants {
		if m == text {
			t.Fatalf("mutant %q did not change the Dockerfile", name)
		}
		p := verifyRunnerYqProblem(m)
		if p == "" {
			t.Errorf("mutant %q: the broken yq install was not caught", name)
			continue
		}
		t.Logf("mutant %q caught: %s", name, p)
		if want, ok := wantRule[name]; ok && !strings.Contains(p, want) {
			t.Errorf("mutant %q: caught, but not by its own rule (want %q in %q)", name, want, p)
		}
	}
}
