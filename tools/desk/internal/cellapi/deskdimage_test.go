// Package cellapi holds tests for the deskd service packaging. This file is the
// static build contract for the deskd binary and image: it parses the
// real images/deskd/Dockerfile, its BuildKit ignore file and the root Makefile,
// and holds every property of the local, digest-pinned, credential-free,
// push-free build. The same checker then runs over one negative fixture per
// property under testdata/deskdimage/; each fixture plants exactly one
// violation into a copy of the real files and must be rejected for its named
// reason and for nothing else.
package cellapi

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// Real inputs, relative to this package directory (tools/desk/internal/cellapi).
const (
	repoRoot         = "../../../.."
	deskdDockerfile  = "images/deskd/Dockerfile"
	deskdIgnoreFile  = "images/deskd/Dockerfile.dockerignore"
	rootMakefile     = "Makefile"
	deskdFixturesDir = "testdata/deskdimage"
)

// Problem codes. Every code has a negative fixture named <code>.fixture, and may
// have more named <code>--<variant>.fixture; the test fails if a code has none,
// so a rule cannot exist untested.
var deskdProblemCodes = []string{
	// Dockerfile: pins and stage shape.
	"unpinned-frontend",
	"unpinned-from",
	"final-not-scratch",
	"final-user-root",
	"final-user-nonnumeric",
	// Dockerfile: build-stage environment.
	"gowork-not-off",
	"cgo-not-off",
	"toolchain-not-local",
	"modules-not-readonly",
	"build-not-only-cmd-deskd",
	"build-missing-trimpath",
	// Dockerfile: checksum verification stays on.
	"gosumdb-off",
	"gonosumdb",
	"gonosumcheck",
	"goprivate",
	"goinsecure",
	"mod-mod",
	"insecure-flag",
	// Dockerfile: what may be copied and mounted.
	"credential-copy",
	"out-of-context-copy",
	"url-source",
	"extra-final-copy",
	"secret-mount",
	"secret-named-env",
	"secret-named-arg",
	// Dockerfile: runtime surface.
	"entrypoint-not-binary",
	"baked-cmd",
	"baked-config-path",
	"non-loopback-bind",
	"missing-oci-label",
	// Ignore file.
	"ignore-missing-entry",
	// Makefile.
	"target-missing",
	"context-not-deskd",
	"missing-secret-scan",
	"step-not-fatal",
	"no-builder-check",
	"tag-not-local",
	"docker-push",
	"push-flag",
	"registry-login",
	"registry-exporter",
	"build-outside-dist",
	"build-not-gowork-off",
}

type deskdProblem struct{ code, detail string }

func (p deskdProblem) String() string { return p.code + ": " + p.detail }

// deskdBuildInputs is the text of the three files the contract reads.
type deskdBuildInputs struct {
	dockerfile, ignore, makefile string
}

// ---------------------------------------------------------------------------
// Dockerfile parsing

type dockerInstr struct {
	op    string   // upper-cased instruction
	flags []string // leading --flag[=value] tokens
	args  string   // the rest, raw
}

type dockerStage struct {
	base   string // FROM image reference
	name   string // AS alias, lower-cased
	instrs []dockerInstr
}

// parseDockerStages joins `\` continuations, drops comment and blank lines
// (parser directives are handled separately), and splits at FROM.
func parseDockerStages(text string) []dockerStage {
	var logical []string
	var cur strings.Builder
	for _, line := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		trim := strings.TrimSpace(line)
		if trim == "" || strings.HasPrefix(trim, "#") {
			continue
		}
		if strings.HasSuffix(trim, `\`) {
			cur.WriteString(strings.TrimSuffix(trim, `\`))
			cur.WriteString(" ")
			continue
		}
		cur.WriteString(trim)
		logical = append(logical, cur.String())
		cur.Reset()
	}
	if cur.Len() > 0 {
		logical = append(logical, cur.String())
	}

	var stages []dockerStage
	for _, l := range logical {
		fields := strings.Fields(l)
		in := dockerInstr{op: strings.ToUpper(fields[0])}
		rest := fields[1:]
		for len(rest) > 0 && strings.HasPrefix(rest[0], "--") {
			in.flags = append(in.flags, rest[0])
			rest = rest[1:]
		}
		in.args = strings.Join(rest, " ")
		if in.op == "FROM" {
			st := dockerStage{}
			if len(rest) > 0 {
				st.base = rest[0]
			}
			if len(rest) >= 3 && strings.EqualFold(rest[1], "AS") {
				st.name = strings.ToLower(rest[2])
			}
			stages = append(stages, st)
			continue
		}
		if len(stages) == 0 {
			// ARG before the first FROM: keep it in a synthetic global stage.
			stages = append(stages, dockerStage{base: "<global>"})
		}
		stages[len(stages)-1].instrs = append(stages[len(stages)-1].instrs, in)
	}
	return stages
}

// digestRef is a tag AND a sha256 digest: tag-only drifts, digest-only hides
// what was meant (the same shape tools/desk/internal/deskkit/basepins_test.go holds).
var digestRef = regexp.MustCompile(`^[a-z0-9][a-z0-9._/-]*:[A-Za-z0-9_][A-Za-z0-9_.-]*@sha256:[0-9a-f]{64}$`)

// envPairs reads both ENV forms: `ENV k=v k2="v 2"` and legacy `ENV k v`.
func envPairs(args string) map[string]string {
	out := map[string]string{}
	if !strings.Contains(strings.Fields(args + " x")[0], "=") {
		f := strings.Fields(args)
		if len(f) >= 1 {
			out[f[0]] = strings.TrimSpace(strings.TrimPrefix(args, f[0]))
		}
		return out
	}
	for _, tok := range shellishFields(args) {
		k, v, _ := strings.Cut(tok, "=")
		out[k] = v
	}
	return out
}

// shellishFields splits on whitespace while keeping double-quoted runs whole
// and dropping the quotes.
func shellishFields(s string) []string {
	var out []string
	var b strings.Builder
	inQ := false
	for _, r := range s {
		switch {
		case r == '"':
			inQ = !inQ
		case (r == ' ' || r == '\t') && !inQ:
			if b.Len() > 0 {
				out = append(out, b.String())
				b.Reset()
			}
		default:
			b.WriteRune(r)
		}
	}
	if b.Len() > 0 {
		out = append(out, b.String())
	}
	return out
}

// execForm parses a JSON-array instruction body; ok=false for shell form.
func execForm(args string) ([]string, bool) {
	s := strings.TrimSpace(args)
	if !strings.HasPrefix(s, "[") || !strings.HasSuffix(s, "]") {
		return nil, false
	}
	var out []string
	for _, part := range strings.Split(strings.TrimSuffix(strings.TrimPrefix(s, "["), "]"), ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		u, err := strconv.Unquote(part)
		if err != nil {
			return nil, false
		}
		out = append(out, u)
	}
	return out, true
}

func flagValue(flags []string, name string) (string, bool) {
	for _, f := range flags {
		if v, ok := strings.CutPrefix(f, "--"+name+"="); ok {
			return v, true
		}
	}
	return "", false
}

var (
	// A source path component that names key material or a credential store.
	credentialPath = regexp.MustCompile(`(?i)(^|/)(id_(rsa|dsa|ecdsa|ed25519)[^/]*|[^/]*\.(pem|key|p12|pfx|jks|kdbx)|\.netrc|\.npmrc|\.pypirc|\.git-credentials|\.ssh|\.gnupg|\.aws|\.docker|\.kube|kubeconfig|credentials?[^/]*|[^/]*(token|secret|password)[^/]*)(/|$)`)
	// An ENV/ARG name that carries a credential.
	secretName = regexp.MustCompile(`(?i)(token|secret|passw|pwd|credential|api_?key|private_?key|(^|_)key($|_)|(^|_)pat($|_)|auth)`)
	// Settings that switch Go module checksum verification off or around.
	checksumOff = []struct {
		code string
		re   *regexp.Regexp
	}{
		{"gonosumdb", regexp.MustCompile(`\bGONOSUMDB\b`)},
		{"gonosumcheck", regexp.MustCompile(`\bGONOSUMCHECK\b`)},
		{"goprivate", regexp.MustCompile(`\bGOPRIVATE\b`)},
		{"goinsecure", regexp.MustCompile(`\bGOINSECURE\b`)},
		{"mod-mod", regexp.MustCompile(`(^|\s|=)-mod=mod\b`)},
		{"insecure-flag", regexp.MustCompile(`(^|\s|=)-insecure\b`)},
	}
	gosumdbOff = regexp.MustCompile(`\bGOSUMDB(=|\s+)"?off\b`)
	// deskd flags that name a config, index, token or key path.
	pathFlag = regexp.MustCompile(`^--?(config|index|tokens-dir|intent-keys)(=|$)`)
	addrFlag = regexp.MustCompile(`^--?addr(=|$)`)
	// Loopback listen addresses: deskd's own default and its IPv6 twin.
	loopbackAddr = regexp.MustCompile(`^(127\.0\.0\.1|localhost|\[::1\]):[0-9]+$`)
)

// finalStageCopies is everything the final stage may carry: the binary, the CA
// bundle from the pinned build stage, and the passwd/group for the numeric user.
var finalStageCopies = map[string]bool{
	"/deskd":                             true,
	"/etc/ssl/certs/ca-certificates.crt": true,
	"/etc/passwd":                        true,
	"/etc/group":                         true,
}

var requiredOCILabels = map[string]string{
	"org.opencontainers.image.source":   "https://github.com/medici-finance/assay",
	"org.opencontainers.image.revision": "${REVISION}",
	"org.opencontainers.image.licenses": "Apache-2.0",
}

func checkDockerfile(text string) []deskdProblem {
	var ps []deskdProblem
	add := func(code, format string) { ps = append(ps, deskdProblem{code, format}) }

	// Parser directives: the leading `# key=value` lines before anything else.
	syntax := ""
	for _, line := range strings.Split(text, "\n") {
		m := regexp.MustCompile(`^#\s*([a-zA-Z]+)\s*=\s*(\S+)\s*$`).FindStringSubmatch(strings.TrimSpace(line))
		if m == nil {
			break
		}
		if strings.EqualFold(m[1], "syntax") {
			syntax = m[2]
		}
	}
	if !digestRef.MatchString(syntax) {
		add("unpinned-frontend", "the first line must be `# syntax=<frontend>:<tag>@sha256:<digest>`, got "+strconv.Quote(syntax))
	}

	stages := parseDockerStages(text)
	var real []dockerStage
	stageNames := map[string]bool{}
	for _, st := range stages {
		if st.base == "<global>" {
			continue
		}
		real = append(real, st)
	}
	if len(real) < 2 {
		add("final-not-scratch", "want a pinned build stage and a FROM scratch final stage")
		return ps
	}
	final := real[len(real)-1]

	for i, st := range real {
		switch {
		case i == len(real)-1 && st.base == "scratch":
		case stageNames[strings.ToLower(st.base)]:
		case digestRef.MatchString(st.base):
		default:
			add("unpinned-from", "FROM "+st.base+" is not pinned by tag and @sha256 digest")
		}
		if st.name != "" {
			stageNames[st.name] = true
		}
	}
	if final.base != "scratch" {
		add("final-not-scratch", "final stage is FROM "+final.base+"; it must be FROM scratch (no shell, no package manager)")
	}

	// Build-stage environment: the stage that runs `go build`.
	builds := 0
	for _, st := range real[:len(real)-1] {
		env := map[string]string{}
		hasBuild := false
		for _, in := range st.instrs {
			if in.op == "ENV" {
				for k, v := range envPairs(in.args) {
					env[k] = v
				}
			}
			if in.op != "RUN" {
				continue
			}
			for _, seg := range regexp.MustCompile(`&&|;|\|\|`).Split(in.args, -1) {
				f := strings.Fields(seg)
				if len(f) < 2 || f[0] != "go" || f[1] != "build" {
					continue
				}
				hasBuild = true
				builds++
				var pkgs []string
				trimpath := false
				for j := 2; j < len(f); j++ {
					switch {
					case f[j] == "-trimpath":
						trimpath = true
					case f[j] == "-o" || f[j] == "-ldflags" || f[j] == "-tags":
						j++
					case strings.HasPrefix(f[j], "-"):
					default:
						pkgs = append(pkgs, f[j])
					}
				}
				if len(pkgs) != 1 || pkgs[0] != "./cmd/deskd" {
					add("build-not-only-cmd-deskd", "go build packages "+strings.Join(pkgs, " ")+"; want exactly ./cmd/deskd")
				}
				if !trimpath {
					add("build-missing-trimpath", "go build without -trimpath embeds builder paths")
				}
			}
		}
		if !hasBuild {
			continue
		}
		if env["GOWORK"] != "off" {
			add("gowork-not-off", "build stage GOWORK="+strconv.Quote(env["GOWORK"])+"; want off")
		}
		if env["CGO_ENABLED"] != "0" {
			add("cgo-not-off", "build stage CGO_ENABLED="+strconv.Quote(env["CGO_ENABLED"])+"; want 0")
		}
		if env["GOTOOLCHAIN"] != "local" {
			add("toolchain-not-local", "build stage GOTOOLCHAIN="+strconv.Quote(env["GOTOOLCHAIN"])+"; want local")
		}
		if !regexp.MustCompile(`(^|\s)-mod=readonly(\s|$)`).MatchString(env["GOFLAGS"]) {
			add("modules-not-readonly", "build stage GOFLAGS="+strconv.Quote(env["GOFLAGS"])+"; want -mod=readonly")
		}
	}
	if builds != 1 {
		add("build-not-only-cmd-deskd", "want exactly one `go build` in the build stage, found "+strconv.Itoa(builds))
	}

	// Rules over every instruction of every stage.
	for si, st := range stages {
		isFinal := st.base == final.base && si == len(stages)-1
		for _, in := range st.instrs {
			body := in.op + " " + strings.Join(in.flags, " ") + " " + in.args
			if gosumdbOff.MatchString(body) {
				add("gosumdb-off", in.op+" switches the checksum database off")
			}
			for _, c := range checksumOff {
				if c.re.MatchString(body) {
					add(c.code, in.op+" carries "+c.re.String())
				}
			}
			switch in.op {
			case "RUN":
				for _, f := range in.flags {
					if regexp.MustCompile(`^--mount=.*type=(secret|ssh)`).MatchString(f) {
						add("secret-mount", "RUN "+f+" mounts a credential into the build")
					}
				}
			case "ENV":
				for k, v := range envPairs(in.args) {
					if secretName.MatchString(k) {
						add("secret-named-env", "ENV "+k+" names a credential")
					}
					if isFinal && strings.Contains(v, "0.0.0.0") {
						add("non-loopback-bind", "ENV "+k+"="+v+" widens the bind")
					}
				}
			case "ARG":
				name, _, _ := strings.Cut(in.args, "=")
				if secretName.MatchString(name) {
					add("secret-named-arg", "ARG "+name+" names a credential")
				}
			case "COPY", "ADD":
				from, hasFrom := flagValue(in.flags, "from")
				if hasFrom && !stageNames[strings.ToLower(from)] && !digestRef.MatchString(from) {
					add("unpinned-from", in.op+" --from="+from+" pulls an unpinned image")
				}
				parts := shellishFields(in.args)
				if j, ok := execForm(in.args); ok {
					parts = j
				}
				if len(parts) < 2 {
					add("extra-final-copy", "malformed "+in.op+" "+in.args)
					continue
				}
				srcs, dest := parts[:len(parts)-1], parts[len(parts)-1]
				for _, src := range srcs {
					switch {
					case strings.Contains(src, "://") || strings.HasPrefix(src, "git@"):
						add("url-source", in.op+" "+src+" fetches from outside the context")
					case credentialPath.MatchString(src):
						add("credential-copy", in.op+" "+src+" copies a credential-shaped path")
					case !hasFrom && (strings.HasPrefix(src, "/") || src == ".." ||
						strings.HasPrefix(src, "../") || strings.Contains(src, "/../") || strings.HasSuffix(src, "/..")):
						add("out-of-context-copy", in.op+" "+src+" reaches outside the deskd/ context")
					}
				}
				if isFinal && (!hasFrom || !stageNames[strings.ToLower(from)] || !finalStageCopies[dest] || len(srcs) != 1) {
					add("extra-final-copy", "final stage "+in.op+" "+strings.Join(in.flags, " ")+" "+in.args+" is not one of the binary, CA bundle, passwd or group")
				}
			}
		}
	}

	// Final stage: user, entrypoint, command and labels.
	user := ""
	var entry []string
	entryOK := false
	var cmdArgs []string
	hasCmd := false
	labels := map[string]string{}
	for _, in := range final.instrs {
		switch in.op {
		case "USER":
			user = strings.TrimSpace(in.args)
		case "ENTRYPOINT":
			entry, entryOK = execForm(in.args)
		case "CMD":
			hasCmd = true
			if j, ok := execForm(in.args); ok {
				cmdArgs = j
			} else {
				cmdArgs = strings.Fields(in.args)
			}
		case "LABEL":
			for k, v := range envPairs(in.args) {
				labels[k] = v
			}
		}
	}
	uid, gid, _ := strings.Cut(user, ":")
	numeric := regexp.MustCompile(`^[0-9]+$`)
	switch {
	case user == "" || uid == "root" || gid == "root" || uid == "0" || gid == "0" || gid == "":
		add("final-user-root", "final USER "+strconv.Quote(user)+" runs as root or leaves the group unset; want <uid>:<gid>, both non-zero")
	case !numeric.MatchString(uid) || !numeric.MatchString(gid):
		add("final-user-nonnumeric", "final USER "+strconv.Quote(user)+" is not numeric; a name cannot be proven non-root")
	case strings.TrimLeft(uid, "0") == "" || strings.TrimLeft(gid, "0") == "":
		add("final-user-root", "final USER "+user+" is uid or gid 0")
	}

	// Baked arguments: the named rules first, the generic rule only when a named
	// one has already REPORTED a problem for them. A compliant argument (a
	// loopback -addr) explains nothing, so it never excuses a CMD or an extra
	// ENTRYPOINT argument: any baked argument is red under one code or another.
	baked := append([]string{}, cmdArgs...)
	if entryOK && len(entry) > 1 {
		baked = append(baked, entry[1:]...)
	}
	named := false
	for i, a := range baked {
		if pathFlag.MatchString(a) {
			add("baked-config-path", "baked argument "+a+" names a config, index, token or key path")
			named = true
		}
		if addrFlag.MatchString(a) {
			v, has := strings.CutPrefix(a, strings.SplitN(a, "=", 2)[0]+"=")
			if !has && i+1 < len(baked) {
				v = baked[i+1]
			}
			if !loopbackAddr.MatchString(v) {
				add("non-loopback-bind", "baked argument "+a+" "+v+" widens deskd's loopback default")
				named = true
			}
		}
	}
	if !entryOK || len(entry) == 0 || entry[0] != "/deskd" || (len(entry) > 1 && !named) {
		add("entrypoint-not-binary", "ENTRYPOINT must be exactly [\"/deskd\"] in exec form")
	}
	if hasCmd && !named {
		add("baked-cmd", "final stage bakes a CMD; runtime configuration is the deployment profile's")
	}
	for k, want := range requiredOCILabels {
		if labels[k] != want {
			add("missing-oci-label", "LABEL "+k+"="+strconv.Quote(labels[k])+"; want "+strconv.Quote(want))
		}
	}
	return ps
}

// ---------------------------------------------------------------------------
// Ignore file

// requiredIgnores are context entries that must never reach the build: local
// build output and a SQLite index a developer may have run deskd against.
var requiredIgnores = []string{"dist", "*.db"}

func checkIgnore(text string) []deskdProblem {
	have := map[string]bool{}
	for _, line := range strings.Split(text, "\n") {
		l := strings.TrimSpace(line)
		if l == "" || strings.HasPrefix(l, "#") || strings.HasPrefix(l, "!") {
			continue
		}
		have[strings.TrimSuffix(strings.TrimPrefix(l, "/"), "/")] = true
	}
	var ps []deskdProblem
	for _, want := range requiredIgnores {
		if !have[want] {
			ps = append(ps, deskdProblem{"ignore-missing-entry", "ignore file does not exclude " + want})
		}
	}
	return ps
}

// ---------------------------------------------------------------------------
// Makefile

// makeRecipe returns target's recipe as logical shell lines (continuations
// joined, leading @/- markers dropped), and whether the rule exists.
func makeRecipe(makefile, target string) ([]string, bool) {
	lines := strings.Split(strings.ReplaceAll(makefile, "\r\n", "\n"), "\n")
	start := -1
	for i, l := range lines {
		if strings.HasPrefix(l, target+":") {
			start = i + 1
			break
		}
	}
	if start < 0 {
		return nil, false
	}
	var out []string
	var cur strings.Builder
	for _, l := range lines[start:] {
		if !strings.HasPrefix(l, "\t") {
			if cur.Len() == 0 {
				break
			}
		}
		body := strings.TrimSpace(l)
		if cur.Len() == 0 {
			body = strings.TrimLeft(body, "@-+")
		}
		if strings.HasSuffix(body, `\`) {
			cur.WriteString(strings.TrimSuffix(body, `\`))
			cur.WriteString(" ")
			continue
		}
		cur.WriteString(body)
		out = append(out, cur.String())
		cur.Reset()
	}
	return out, true
}

func phonyTargets(makefile string) map[string]bool {
	out := map[string]bool{}
	for _, l := range strings.Split(makefile, "\n") {
		if rest, ok := strings.CutPrefix(l, ".PHONY:"); ok {
			for _, t := range strings.Fields(rest) {
				out[t] = true
			}
		}
	}
	return out
}

// registryHostRef is an image reference whose first path component is a
// registry host (has a dot or a port, or is localhost).
var registryHostRef = regexp.MustCompile(`^([a-z0-9-]+\.[a-z0-9.-]+|[a-z0-9.-]+:[0-9]+|localhost)/`)

func checkMakefile(text string) []deskdProblem {
	var ps []deskdProblem
	add := func(code, d string) { ps = append(ps, deskdProblem{code, d}) }
	phony := phonyTargets(text)
	recipes := map[string][]string{}
	for _, t := range []string{"deskd-build", "deskd-image"} {
		r, ok := makeRecipe(text, t)
		if !phony[t] || !ok || len(r) == 0 {
			add("target-missing", t+" must be on .PHONY and have a recipe")
		}
		recipes[t] = r
	}

	// deskd-build: unprivileged, GOWORK=off, writes only under deskd/dist/.
	if len(recipes["deskd-build"]) > 0 {
		b := strings.Join(recipes["deskd-build"], "\n")
		gobuild := regexp.MustCompile(`cd deskd && GOWORK=off go build `).MatchString(b)
		if !gobuild {
			add("build-not-gowork-off", "deskd-build must run `cd deskd && GOWORK=off go build ...`")
		}
		outs := regexp.MustCompile(`(?:^|\s)-o\s+(\S+)`).FindAllStringSubmatch(b, -1)
		bad := len(outs) == 0
		for _, o := range outs {
			if !strings.HasPrefix(o[1], "dist/") || strings.Contains(o[1], "..") {
				bad = true
			}
		}
		if regexp.MustCompile(`(^|\s)(>|>>|cp|mv|install|ln|tee|rm)\s`).MatchString(b) {
			bad = true
		}
		if bad {
			add("build-outside-dist", "deskd-build may write only under deskd/dist/")
		}
	}

	// deskd-image: local docker build of the deskd context, then the layer scan.
	img := strings.Join(recipes["deskd-image"], "\n")
	if len(recipes["deskd-image"]) > 0 {
		var build []string
		for _, l := range recipes["deskd-image"] {
			for _, seg := range regexp.MustCompile(`&&|;|\|\|`).Split(l, -1) {
				if f := strings.Fields(seg); len(f) >= 2 && f[0] == "docker" && f[1] == "build" {
					build = f
				}
			}
		}
		if len(build) == 0 || build[len(build)-1] != "deskd" ||
			!regexp.MustCompile(`\s-f\s+images/deskd/Dockerfile(\s|$)`).MatchString(strings.Join(build, " ")) {
			add("context-not-deskd", "deskd-image must run `docker build -f images/deskd/Dockerfile ... deskd`, got "+strconv.Quote(strings.Join(build, " ")))
		}
		joined := strings.Join(build, " ")
		if !regexp.MustCompile(`\s-t\s+"?assay-deskd:`).MatchString(joined) {
			add("tag-not-local", "deskd-image must tag the local name assay-deskd:<sha>")
		}
		if !regexp.MustCompile(`--build-arg\s+"?REVISION=`).MatchString(joined) {
			add("missing-oci-label", "deskd-image must pass the source SHA as --build-arg REVISION=")
		}
		scan := regexp.MustCompile(`sh containers/scripts/layer-secret-scan\.sh\s+\S+`)
		scanOK := false
		for _, l := range recipes["deskd-image"] {
			if scan.MatchString(l) && !regexp.MustCompile(`\|\|\s*(true|:)`).MatchString(l) {
				scanOK = true
			}
		}
		if !scanOK {
			add("missing-secret-scan", "deskd-image must run containers/scripts/layer-secret-scan.sh on the built image, failing on any non-zero")
		}
		// make fails a recipe only on a logical line's LAST status. On the line that
		// carries the docker build or the scan, a later command (the closing echo)
		// would mask their failure unless the line runs under errexit or chains
		// every following command with &&.
		errexit := regexp.MustCompile(`^\s*set\s+(-[A-Za-z]*e[A-Za-z]*|-o\s+errexit)(\s|;)`)
		quoted := regexp.MustCompile(`"(?:[^"\\]|\\.)*"|'[^']*'`)
		buildCmd := regexp.MustCompile(`(^|[;&|]\s*)docker\s+build\s`)
		for _, l := range recipes["deskd-image"] {
			if errexit.MatchString(l) && !strings.Contains(l, "set +e") {
				continue
			}
			bare := quoted.ReplaceAllString(l, `""`)
			for _, at := range []*regexp.Regexp{buildCmd, scan} {
				loc := at.FindStringIndex(bare)
				if loc == nil {
					continue
				}
				if rest := bare[loc[1]:]; strings.Contains(rest, ";") || strings.Contains(rest, "||") {
					add("step-not-fatal", "deskd-image runs the build or the layer scan without errexit and before another command; a failure would not fail the target: "+strconv.Quote(l))
				}
			}
		}
		if !strings.Contains(img, "command -v docker") || !strings.Contains(img, "docker info") {
			add("no-builder-check", "deskd-image must refuse with a named could-not-check when docker or its daemon is missing")
		}
	}
	for _, l := range recipes["deskd-image"] {
		for _, seg := range regexp.MustCompile(`&&|;|\|\||\|`).Split(l, -1) {
			f := strings.Fields(seg)
			isDocker := len(f) >= 2 && f[0] == "docker"
			switch {
			case isDocker && (f[1] == "push" || (f[1] == "image" && len(f) > 2 && f[2] == "push")):
				add("docker-push", "deskd-image runs `"+strings.Join(f[:2], " ")+" push`")
			case isDocker && f[1] == "login":
				add("registry-login", "deskd-image logs in to a registry")
			case isDocker && f[1] == "tag" && len(f) >= 4 && registryHostRef.MatchString(f[3]):
				add("docker-push", "deskd-image tags to registry host "+f[3])
			}
			for i, tok := range f {
				switch {
				case tok == "--push":
					add("push-flag", "deskd-image builds with --push")
				case (tok == "--output" || tok == "-o") && i+1 < len(f) && regexp.MustCompile(`type=(registry|image)`).MatchString(f[i+1]),
					regexp.MustCompile(`^--output=.*type=(registry|image)`).MatchString(tok):
					add("registry-exporter", "deskd-image uses a registry exporter")
				case tok == "-t" && i+1 < len(f) && registryHostRef.MatchString(strings.Trim(f[i+1], `"`)):
					add("docker-push", "deskd-image tags to a registry host "+f[i+1])
				}
			}
		}
	}
	return ps
}

func checkDeskdBuild(in deskdBuildInputs) []deskdProblem {
	var ps []deskdProblem
	ps = append(ps, checkDockerfile(in.dockerfile)...)
	ps = append(ps, checkIgnore(in.ignore)...)
	ps = append(ps, checkMakefile(in.makefile)...)
	return ps
}

// ---------------------------------------------------------------------------
// Fixtures

// deskdFixture plants one violation into a copy of the real files.
// Format, one `key: value` per line, `#` comments allowed:
//
//	want: <problem code>            (the file's base name, or its <code>--<variant> prefix)
//	file: Dockerfile|ignore|Makefile
//	replace: "<Go-quoted text that occurs exactly once in the real file>"
//	with: "<Go-quoted replacement>"
type deskdFixture struct {
	name, want, file, replace, with string
}

func readDeskdFixtures(t *testing.T) []deskdFixture {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join(deskdFixturesDir, "*.fixture"))
	if err != nil {
		t.Fatal(err)
	}
	var out []deskdFixture
	for _, p := range paths {
		raw, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		fx := deskdFixture{name: filepath.Base(p)}
		for _, line := range strings.Split(string(raw), "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			k, v, ok := strings.Cut(line, ":")
			if !ok {
				t.Fatalf("%s: malformed line %q", p, line)
			}
			v = strings.TrimSpace(v)
			switch k {
			case "want":
				fx.want = v
			case "file":
				fx.file = v
			case "replace", "with":
				u, err := strconv.Unquote(v)
				if err != nil {
					t.Fatalf("%s: %s must be a Go-quoted string: %v", p, k, err)
				}
				if k == "replace" {
					fx.replace = u
				} else {
					fx.with = u
				}
			default:
				t.Fatalf("%s: unknown key %q", p, k)
			}
		}
		if base := strings.TrimSuffix(fx.name, ".fixture"); fx.want != base && !strings.HasPrefix(base, fx.want+"--") {
			t.Fatalf("%s: want %q must equal the file's base name, or prefix it as <code>--<variant>", p, fx.want)
		}
		if fx.replace == "" || fx.replace == fx.with {
			t.Fatalf("%s: a fixture must replace non-empty text with something different", p)
		}
		out = append(out, fx)
	}
	return out
}

func readDeskdInputs(t *testing.T) deskdBuildInputs {
	t.Helper()
	read := func(rel string) string {
		raw, err := os.ReadFile(filepath.Join(repoRoot, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatalf("reading %s: %v", rel, err)
		}
		return string(raw)
	}
	return deskdBuildInputs{
		dockerfile: read(deskdDockerfile),
		ignore:     read(deskdIgnoreFile),
		makefile:   read(rootMakefile),
	}
}

func problemCodes(ps []deskdProblem) []string {
	seen := map[string]bool{}
	var out []string
	for _, p := range ps {
		if !seen[p.code] {
			seen[p.code] = true
			out = append(out, p.code)
		}
	}
	sort.Strings(out)
	return out
}

func TestDeskdImageBuildContract(t *testing.T) {
	real := readDeskdInputs(t)

	if ps := checkDeskdBuild(real); len(ps) != 0 {
		for _, p := range ps {
			t.Errorf("real build definition: %s", p)
		}
		t.FailNow()
	}

	fixtures := readDeskdFixtures(t)
	if len(fixtures) == 0 {
		t.Fatalf("read 0 fixtures from %s: the negative half proved nothing", deskdFixturesDir)
	}
	covered := map[string]bool{}
	known := map[string]bool{}
	for _, c := range deskdProblemCodes {
		known[c] = true
	}
	for _, fx := range fixtures {
		t.Run(strings.TrimSuffix(fx.name, ".fixture"), func(t *testing.T) {
			if !known[fx.want] {
				t.Fatalf("fixture names unknown problem code %q", fx.want)
			}
			in := real
			var target *string
			switch fx.file {
			case "Dockerfile":
				target = &in.dockerfile
			case "ignore":
				target = &in.ignore
			case "Makefile":
				target = &in.makefile
			default:
				t.Fatalf("file %q: want Dockerfile, ignore or Makefile", fx.file)
			}
			if n := strings.Count(*target, fx.replace); n != 1 {
				t.Fatalf("replace text occurs %d times in the real %s; a fixture must plant exactly one violation", n, fx.file)
			}
			*target = strings.Replace(*target, fx.replace, fx.with, 1)
			got := problemCodes(checkDeskdBuild(in))
			if len(got) != 1 || got[0] != fx.want {
				t.Fatalf("want rejection for exactly %q, got %v (%v)", fx.want, got, checkDeskdBuild(in))
			}
			covered[fx.want] = true
		})
	}
	for _, c := range deskdProblemCodes {
		if !covered[c] {
			t.Errorf("problem code %q has no passing negative fixture %s/%s.fixture", c, deskdFixturesDir, c)
		}
	}
}
