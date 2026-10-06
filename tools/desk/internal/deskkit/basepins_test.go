package deskkit

import (
	"os"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

// Supply-chain pins on the base image (#2320, the pinning follow-ups to #2319).
//
// containers/base/Dockerfile names every base image by tag AND digest, and
// checks the Go, gh and Node tarballs with `sha256sum -c` against pinned ARGs
// before it unpacks them, the same way the gitbuild stage checks git (that half
// is pinned by TestBaseImageRunsGitFloor). The image is built only on release,
// so this static check is what makes dropping a pin or a check go red on a PR.

// toolTarball is the reviewed sha256 of one release's linux tarball for each
// architecture the base image builds for.
type toolTarball struct{ amd64, arm64 string }

// knownToolTarballs is the reviewed record of each Go, gh and Node release the
// base image may install, keyed by ARG prefix and then version. Each sha256 was
// read from the vendor's published checksums (go.dev/dl JSON, the gh release's
// checksums file, Node's SHASUMS256.txt) and matched against the downloaded
// tarball when it was added. The Dockerfile's version and both sha256 ARGs must
// be one of these rows, so changing any one of them alone goes red here. A bump
// adds a row in the same change as the Dockerfile (containers/README.md,
// "Pins and checksums").
var knownToolTarballs = map[string]map[string]toolTarball{
	"GO": {"1.25.0": {
		amd64: "2852af0cb20a13139b3448992e69b868e50ed0f8a1e5940ee1de9e19a123b613",
		arm64: "05de75d6994a2783699815ee553bd5a9327d8b79991de36e38b66862782f54ae",
	}},
	"GH": {"2.63.2": {
		amd64: "912fdb1ca29cb005fb746fc5d2b787a289078923a29d0f9ec19a0b00272ded00",
		arm64: "0f31e2a8549c64b5c1679f0b99ce5e0dac7c91da9e86f6246adb8805b0f0b4bb",
	}},
	"NODE": {"22.11.0": {
		amd64: "83bf07dd343002a26211cf1fcd46a9d9534219aad42ee02847816940bf610a72",
		arm64: "6031d04b98f59ff0f7cb98566f65b115ecd893d3b7870821171708cdbaf7ae6e",
	}},
}

// pinnedTool is one tarball the final stage downloads, checks and unpacks.
type pinnedTool struct {
	arg  string // ARG prefix: <arg>_VERSION, <arg>_SHA256_AMD64, <arg>_SHA256_ARM64
	file string // where the RUN downloads it
	tar  string // the command that unpacks it
}

var basePinnedTools = []pinnedTool{
	{"GO", "/tmp/go.tgz", "tar -C /usr/local --no-same-owner -xzf /tmp/go.tgz"},
	{"GH", "/tmp/gh.tgz", "tar -C /tmp -xzf /tmp/gh.tgz"},
	{"NODE", "/tmp/node.txz", "tar -C /usr/local --no-same-owner --strip-components=1 -xJf /tmp/node.txz"},
}

// digestRef is an image reference carrying both a tag and a sha256 digest.
var digestRef = regexp.MustCompile(`^[a-z0-9][a-z0-9._/-]*:[A-Za-z0-9_][A-Za-z0-9_.-]*@sha256:[0-9a-f]{64}$`)

// basePinProblem returns why a base Dockerfile does not hold its supply-chain
// pins, or "" when it does. It requires:
//   - every FROM (after resolving a pre-FROM ARG) names `<name>:<tag>@sha256:…`,
//     and every debian stage names the same reference, since the git built in
//     one stage links against the other's libraries;
//   - the final stage pins each tool's version and both per-arch sha256 ARGs
//     to a reviewed row in knownToolTarballs;
//   - each tool's RUN, fail-closed, maps TARGETARCH to that tool's own ARG,
//     downloads once over https only, runs a bare
//     `echo "${sha}  <file>" | sha256sum -c -`, and only then unpacks.
func basePinProblem(dockerfile string) string {
	ins := parseDockerfile(dockerfile)
	globals := map[string]string{}
	lastFrom, debianRef := -1, ""
	for i, in := range ins {
		if in.op == "ARG" && lastFrom < 0 && len(in.args) == 1 {
			if k, v, ok := strings.Cut(in.args[0], "="); ok {
				globals[k] = v
			}
		}
		if in.op != "FROM" {
			continue
		}
		lastFrom = i
		ref := ""
		for _, a := range in.args {
			if !strings.HasPrefix(a, "--") {
				ref = a
				break
			}
		}
		if strings.HasPrefix(ref, "${") && strings.HasSuffix(ref, "}") {
			ref = globals[strings.TrimSuffix(strings.TrimPrefix(ref, "${"), "}")]
		}
		if !digestRef.MatchString(ref) {
			return "FROM `" + strings.Join(in.args, " ") + "` resolves to `" + ref + "`, not a `<name>:<tag>@sha256:<digest>` reference"
		}
		if strings.HasPrefix(ref, "debian:") {
			if debianRef != "" && ref != debianRef {
				return "the debian stages name different references (" + debianRef + " and " + ref + ")"
			}
			debianRef = ref
		}
	}
	if lastFrom < 0 {
		return "no FROM"
	}
	args := map[string]string{}
	for _, in := range ins[lastFrom+1:] {
		if in.op == "ARG" && len(in.args) == 1 {
			if k, v, ok := strings.Cut(in.args[0], "="); ok {
				args[k] = v
			}
		}
	}
	for _, tl := range basePinnedTools {
		ver := args[tl.arg+"_VERSION"]
		amd, arm := args[tl.arg+"_SHA256_AMD64"], args[tl.arg+"_SHA256_ARM64"]
		if rec, ok := knownToolTarballs[tl.arg][ver]; !ok || amd != rec.amd64 || arm != rec.arm64 {
			return "the final stage pins " + tl.arg + "_VERSION=" + ver + " " + tl.arg + "_SHA256_AMD64=" + amd + " " +
				tl.arg + "_SHA256_ARM64=" + arm + ", which is not a reviewed row in knownToolTarballs"
		}
		var run *dfInstr
		for i := lastFrom + 1; i < len(ins); i++ {
			if ins[i].op != "RUN" {
				continue
			}
			for _, sg := range ins[i].segs {
				if strings.HasPrefix(sg, "curl ") && strings.HasSuffix(sg, " -o "+tl.file) {
					if run != nil && run != &ins[i] {
						return "more than one RUN downloads " + tl.file
					}
					run = &ins[i]
				}
			}
		}
		if run == nil {
			return "no final-stage RUN downloads " + tl.file
		}
		if p := toolRunProblem(*run, tl); p != "" {
			return "the " + tl.file + " RUN " + p
		}
	}
	return ""
}

// toolRunProblem returns why one tool's RUN could unpack a tarball it has not
// checked against that tool's pinned sha256.
func toolRunProblem(in dfInstr, tl pinnedTool) string {
	if p := failClosedProblem(in.args); p != "" {
		return p
	}
	amdWord := `sha="${` + tl.arg + `_SHA256_AMD64}"`
	armWord := `sha="${` + tl.arg + `_SHA256_ARM64}"`
	shaCmd := `echo "${sha} ` + tl.file + `" | sha256sum -c -`
	curl, amdArm, armArm, sha, tar := -1, -1, -1, -1, -1
	for i, sg := range in.segs {
		// The first case arm shares its command with `case "${TARGETARCH}" in`.
		arm := sg
		if strings.HasPrefix(arm, `case "${TARGETARCH}" in `) {
			arm = strings.TrimPrefix(arm, `case "${TARGETARCH}" in `)
		}
		words := strings.Fields(arm)
		setsSha := false
		for _, w := range words {
			setsSha = setsSha || strings.HasPrefix(w, "sha=")
		}
		switch {
		case strings.HasPrefix(sg, "curl "):
			if curl >= 0 {
				return "downloads more than once, so the file unpacked need not be the file checked"
			}
			if !strings.HasSuffix(sg, " -o "+tl.file) {
				return "runs a curl that does not write " + tl.file + " (`" + sg + "`)"
			}
			if !strings.Contains(sg, gitCurlHTTPSOnly) {
				return "downloads without " + gitCurlHTTPSOnly + ", so a redirect could leave https"
			}
			curl = i
		case strings.HasPrefix(arm, "amd64) "):
			if !hasWord(words, amdWord) {
				return "does not set " + amdWord + " for amd64 (`" + arm + "`)"
			}
			amdArm = i
		case strings.HasPrefix(arm, "arm64) "):
			if !hasWord(words, armWord) {
				return "does not set " + armWord + " for arm64 (`" + arm + "`)"
			}
			armArm = i
		case setsSha:
			return "sets sha outside the TARGETARCH case (`" + sg + "`), so the check need not use the pinned ARG"
		case sg == shaCmd:
			sha = i
		case sg == tl.tar:
			tar = i
		}
	}
	switch {
	case amdArm < 0 || armArm < 0:
		return "does not map amd64 to " + amdWord + " and arm64 to " + armWord
	case sha < 0:
		return "has no bare `" + shaCmd + "` command"
	case curl < 0 || tar < 0:
		return "does not both download " + tl.file + " and unpack it with `" + tl.tar + "`"
	case !(amdArm < sha && armArm < sha && curl < sha && sha < tar):
		return "does not choose the pinned sha256 and download, then check, then unpack"
	}
	return ""
}

// TestBaseImagePinsTarballs pins the supply-chain half of the base Dockerfile:
// the real file holds every pin and check, and each mutant that drops, masks,
// reorders or swaps one is caught.
func TestBaseImagePinsTarballs(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the base Dockerfile is a Linux build input; its mutant anchors assume an LF checkout")
	}
	skipIfFixtureAbsent(t, baseDockerfilePath, "containers/ is not part of this repository's published file set")
	raw, err := os.ReadFile(baseDockerfilePath)
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	if p := basePinProblem(text); p != "" {
		t.Fatalf("containers/base/Dockerfile: %s", p)
	}

	once := func(anchor string) string {
		t.Helper()
		if strings.Count(text, anchor) != 1 {
			t.Fatalf("the mutants below need %q exactly once in the Dockerfile", anchor)
		}
		return anchor
	}
	swap := func(old, repl string) string { return strings.Replace(text, once(old), repl, 1) }

	debianRef := regexp.MustCompile(`debian:bookworm-slim@sha256:[0-9a-f]{64}`).FindString(text)
	deskTools := regexp.MustCompile(`(?m)^ARG DESK_TOOLS_IMAGE=(\S+)@(sha256:[0-9a-f]{64})$`).FindStringSubmatch(text)
	if debianRef == "" || deskTools == nil {
		t.Fatal("the mutants below need a digest-pinned debian FROM and DESK_TOOLS_IMAGE ARG")
	}
	gitbuildFrom := once("FROM " + debianRef + " AS gitbuild\n")
	finalFrom := once("FROM " + debianRef + "\n")
	deskToolsArg := once(deskTools[0])
	otherDigest := "sha256:" + strings.Repeat("ab", 32)

	mutants := map[string]string{
		"desk-tools digest dropped": swap(deskToolsArg, "ARG DESK_TOOLS_IMAGE="+deskTools[1]),
		"desk-tools tag dropped":    swap(deskToolsArg, "ARG DESK_TOOLS_IMAGE="+strings.Split(deskTools[1], ":")[0]+"@"+deskTools[2]),
		"desk-tools digest short":   swap(deskToolsArg, deskToolsArg[:len(deskToolsArg)-1]),
		"gitbuild digest dropped":   swap(gitbuildFrom, "FROM debian:bookworm-slim AS gitbuild\n"),
		"final digest dropped":      swap(finalFrom, "FROM debian:bookworm-slim\n"),
		"final tag dropped":         swap(finalFrom, "FROM debian@"+strings.Split(debianRef, "@")[1]+"\n"),
		"debian stages diverge":     swap(gitbuildFrom, "FROM debian:bookworm-slim@"+otherDigest+" AS gitbuild\n"),
		"final FROM via bare ARG":   swap(finalFrom, "ARG FINAL_BASE=debian:bookworm-slim\nFROM ${FINAL_BASE}\n"),
	}
	for _, tl := range basePinnedTools {
		rec := knownToolTarballs[tl.arg]
		var ver string
		for v := range rec {
			ver = v
		}
		verArg := "ARG " + tl.arg + "_VERSION=" + ver + "\n"
		amdArg := "ARG " + tl.arg + "_SHA256_AMD64=" + rec[ver].amd64 + "\n"
		armArg := "ARG " + tl.arg + "_SHA256_ARM64=" + rec[ver].arm64 + "\n"
		shaLine := `    echo "${sha}  ` + tl.file + `" | sha256sum -c -; \` + "\n"
		tarLine := "    " + tl.tar + "; \\\n"
		amdWord := `sha="${` + tl.arg + `_SHA256_AMD64}"`
		armWord := `sha="${` + tl.arg + `_SHA256_ARM64}"`
		curlLine := "    curl -fsSL " + gitCurlHTTPSOnly + ` "$url" -o ` + tl.file + "; \\\n"
		shaCmd := func(repl string) string {
			return swap(shaLine, strings.Replace(shaLine, "sha256sum -c -;", repl, 1))
		}
		p := tl.arg + ": "
		for name, m := range map[string]string{
			"sha256 check dropped":      swap(shaLine, ""),
			"sha256 check masked":       shaCmd("sha256sum -c - || true;"),
			"sha256 check && true":      shaCmd("sha256sum -c - && true;"),
			"sha256 check piped":        shaCmd("sha256sum -c - | cat;"),
			"sha256 check backgrounded": shaCmd("sha256sum -c - & wait;"),
			"sha256 check negated":      swap(shaLine, strings.Replace(shaLine, "echo ", "! echo ", 1)),
			"sha256 check in an if":     swap(shaLine, strings.Replace(strings.Replace(shaLine, "echo ", "if echo ", 1), "sha256sum -c -;", "sha256sum -c -; then :; fi;", 1)),
			"exit 0 before sha256":      swap(shaLine, "    exit 0; \\\n"+shaLine),
			"hash checked after unpack": strings.Replace(swap(tarLine, ""), shaLine, tarLine+shaLine, 1),
			"sha self-computed":         swap(shaLine, `    sha="$(sha256sum `+tl.file+` | cut -d' ' -f1)"; \`+"\n"+shaLine),
			"second download after":     swap(shaLine, shaLine+curlLine),
			"curl https-only dropped":   swap(curlLine, "    curl -fsSL \"$url\" -o "+tl.file+"; \\\n"),
			"amd64 arm cross-wired":     swap(amdWord+" ;;", armWord+" ;;"),
			"arm64 arm cross-wired":     swap(armWord+" ;;", amdWord+" ;;"),
			"amd64 sha256 ARG swapped":  swap(amdArg, "ARG "+tl.arg+"_SHA256_AMD64="+strings.Repeat("0", 64)+"\n"),
			"arm64 sha256 ARG swapped":  swap(armArg, "ARG "+tl.arg+"_SHA256_ARM64="+strings.Repeat("0", 64)+"\n"),
			"version bumped alone":      swap(verArg, "ARG "+tl.arg+"_VERSION="+ver+"9\n"),
			"sha256 ARG dropped":        swap(amdArg, ""),
		} {
			mutants[p+name] = m
		}
	}
	for name, m := range mutants {
		if m == text {
			t.Fatalf("mutant %q did not change the Dockerfile", name)
		}
		if p := basePinProblem(m); p == "" {
			t.Errorf("mutant %q: the dropped pin was not caught", name)
		} else {
			t.Logf("mutant %q caught: %s", name, p)
		}
	}
}
