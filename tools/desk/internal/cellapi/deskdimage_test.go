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
	"slices"
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

// makeRule is a target's recipe as the checker reads it.
type makeRule struct {
	lines   []string // logical shell lines: continuations joined, leading @ - + markers dropped
	ignored []bool   // lines[i] carried make's - marker, so make ignores its exit status
	recipes int      // rules for the target that carry a recipe; make runs only the last
}

// makeRecipe returns target's first recipe and whether the rule exists. The -
// marker is recorded per line rather than dropped silently: a line make runs
// with its status ignored can fail without failing the target.
func makeRecipe(makefile, target string) (makeRule, bool) {
	lines := strings.Split(strings.ReplaceAll(makefile, "\r\n", "\n"), "\n")
	var r makeRule
	found := false
	for i, l := range lines {
		if !strings.HasPrefix(l, target+":") {
			continue
		}
		found = true
		body, ignored := recipeLines(lines[i+1:])
		if len(body) == 0 {
			continue
		}
		r.recipes++
		if r.recipes == 1 {
			r.lines, r.ignored = body, ignored
		}
	}
	return r, found
}

// recipeLines reads the tab-led recipe lines at the top of lines.
func recipeLines(lines []string) ([]string, []bool) {
	var out []string
	var ignored []bool
	var cur strings.Builder
	ign := false
	for _, l := range lines {
		if !strings.HasPrefix(l, "\t") {
			if cur.Len() == 0 {
				break
			}
		}
		body := strings.TrimSpace(l)
		if cur.Len() == 0 {
			// make reads any run of @, - and + (blanks between them allowed).
			marks := body[:len(body)-len(strings.TrimLeft(body, "@-+ \t"))]
			ign = strings.Contains(marks, "-")
			body = body[len(marks):]
		}
		if strings.HasSuffix(body, `\`) {
			cur.WriteString(strings.TrimSuffix(body, `\`))
			cur.WriteString(" ")
			continue
		}
		cur.WriteString(body)
		out = append(out, cur.String())
		ignored = append(ignored, ign)
		cur.Reset()
	}
	return out, ignored
}

// makeIgnoresErrors names each Makefile-level setting under which make does not
// fail a target on a failed recipe line, or reads the recipe other than one
// shell per line: .IGNORE (bare, naming target, or through a variable),
// MAKEFLAGS/MFLAGS gaining -i or --ignore-errors, and .ONESHELL.
func makeIgnoresErrors(makefile string, targets []string) []string {
	ignoreRule := regexp.MustCompile(`^\.IGNORE\s*:(.*)$`)
	flagsSet := regexp.MustCompile(`^\s*(?:(?:export|override)\s+)*(?:MAKEFLAGS|MFLAGS)\s*(?:\+|:|::|\?|!)?=(.*)$`)
	var why []string
	for _, l := range strings.Split(strings.ReplaceAll(makefile, "\r\n", "\n"), "\n") {
		if strings.HasPrefix(l, "\t") {
			continue
		}
		l, _, _ = strings.Cut(l, "#")
		if m := ignoreRule.FindStringSubmatch(l); m != nil {
			deps := strings.Fields(m[1])
			named := len(deps) == 0 || strings.Contains(m[1], "$")
			for _, d := range deps {
				for _, t := range targets {
					named = named || d == t
				}
			}
			if named {
				why = append(why, ".IGNORE makes make ignore failed recipe lines: "+strconv.Quote(strings.TrimSpace(l)))
			}
		}
		if m := flagsSet.FindStringSubmatch(l); m != nil && makeflagsIgnore(m[1]) {
			why = append(why, "MAKEFLAGS runs make with -i, ignoring failed recipe lines: "+strconv.Quote(strings.TrimSpace(l)))
		}
		if regexp.MustCompile(`^\.ONESHELL\s*:`).MatchString(l) {
			why = append(why, ".ONESHELL runs each recipe as one shell, so a later line decides the target's status")
		}
	}
	return why
}

// makeflagsIgnore reports whether a MAKEFLAGS value carries -i, alone or in a
// short-flag group (a letter that takes an argument ends the group), or
// --ignore-errors. A variable reference cannot be read, so it counts.
func makeflagsIgnore(v string) bool {
	if strings.Contains(v, "$") {
		return true
	}
	for _, w := range strings.Fields(v) {
		if w == "--ignore-errors" {
			return true
		}
		if strings.HasPrefix(w, "--") {
			continue
		}
		for _, c := range strings.TrimPrefix(w, "-") {
			if strings.ContainsRune("CfIjlOoWE", c) {
				break
			}
			if c == 'i' {
				return true
			}
		}
	}
	return false
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

// ---------------------------------------------------------------------------
// Recipe-line shell reading, for step-not-fatal.
//
// make fails a target only on a logical recipe line's LAST exit status, and
// each line is its own shell. A command whose failure must fail the target (a
// build, the layer scan) therefore has to be a plain command whose status
// reaches the end of its line: nothing after it, an unbroken && chain to the
// end, or ; under errexit. errexit does not stop the shell for a command on the
// left of && or ||, inside an if or after !, and without pipefail a pipeline
// reports only its last command, so those positions are refused too.
//
// On a line that carries a step, a trap (which can rewrite the exit status), an
// exit/return without a literal non-zero status or an exec before the step
// (which end the line first), and a function or alias named after the step's
// command (which shadows it) are refused as well. At the make level, a line
// marked -, .IGNORE, MAKEFLAGS with -i, .ONESHELL and a second recipe for the
// target are refused in checkMakefile. Out of this file's reach, and so not
// held here: a caller running make -i or exporting MAKEFLAGS, and a SHELL or
// .SHELLFLAGS that is not a POSIX sh -c.

// recipeStep is a command whose failure must fail its make target.
type recipeStep struct {
	name  string
	words []string       // the plain command's leading words
	at    *regexp.Regexp // any occurrence of it in quote-masked text
}

var imageSteps = []recipeStep{
	{"docker build", []string{"docker", "build"}, regexp.MustCompile(`(^|[^\w./-])docker\s+build(\s|$)`)},
	{"layer-secret scan", []string{"sh", "containers/scripts/layer-secret-scan.sh"}, regexp.MustCompile(`layer-secret-scan\.sh`)},
}

var goBuildSteps = []recipeStep{
	{"go build", []string{"go", "build"}, regexp.MustCompile(`(^|[^\w./-])go\s+build(\s|$)`)},
}

// maskQuotes replaces every byte inside a quoted string (and every escaped
// byte) with '_', keeping the quotes and the length, so operators and command
// names inside strings are not read as shell structure.
func maskQuotes(line string) string {
	b := []byte(line)
	var q byte
	for i := 0; i < len(b); i++ {
		c := b[i]
		switch {
		case q != '\'' && c == '\\' && i+1 < len(b):
			b[i+1] = '_'
			if q != 0 {
				b[i] = '_'
			}
			i++
		case q == 0 && (c == '"' || c == '\''):
			q = c
		case q != 0 && c == q:
			q = 0
		case q != 0:
			b[i] = '_'
		}
	}
	return string(b)
}

// shellSeg is one simple command of a recipe line and the operator after it
// ("" at the end of the line).
type shellSeg struct{ text, masked, next string }

// shellSegments splits a logical recipe line at ; && || | |& and &, outside
// quotes, leaving the & of a redirection (>&2, &>f) and the | of >| alone.
func shellSegments(line string) []shellSeg {
	m := maskQuotes(line)
	var out []shellSeg
	start := 0
	emit := func(end int, op string) {
		out = append(out, shellSeg{strings.TrimSpace(line[start:end]), strings.TrimSpace(m[start:end]), op})
	}
	for i := 0; i < len(m); i++ {
		next := byte(0)
		if i+1 < len(m) {
			next = m[i+1]
		}
		prev := byte(0)
		if i > 0 {
			prev = m[i-1]
		}
		var op string
		switch c := m[i]; {
		case c == ';':
			op = ";"
		case c == '&' && next == '&':
			op = "&&"
		case c == '|' && next == '|':
			op = "||"
		case c == '|' && next == '&':
			op = "|&"
		case c == '|' && prev != '>':
			op = "|"
		case c == '&' && prev != '>' && prev != '<' && next != '>':
			op = "&"
		default:
			continue
		}
		emit(i, op)
		i += len(op) - 1
		start = i + 1
	}
	emit(len(m), "")
	return out
}

// commandWords drops leading NAME=value assignments from a command's words.
func commandWords(masked string) []string {
	w := strings.Fields(masked)
	for len(w) > 0 && regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*=`).MatchString(w[0]) {
		w = w[1:]
	}
	return w
}

func hasWords(w []string, want ...string) bool {
	if len(w) < len(want) {
		return false
	}
	for i := range want {
		if w[i] != want[i] {
			return false
		}
	}
	return true
}

// applySet folds one `set` command's arguments into the errexit and pipefail
// state: -e/+e, -o/+o errexit and -o/+o pipefail, in any flag group.
func applySet(args []string, errexit, pipefail bool) (bool, bool) {
	for j := 0; j < len(args); j++ {
		a := args[j]
		if len(a) < 2 || (a[0] != '-' && a[0] != '+') {
			continue
		}
		on := a[0] == '-'
		for _, c := range a[1:] {
			switch c {
			case 'e':
				errexit = on
			case 'o':
				if j+1 < len(args) {
					j++
					switch args[j] {
					case "errexit":
						errexit = on
					case "pipefail":
						pipefail = on
					}
				}
			}
		}
	}
	return errexit, pipefail
}

// stepMasked says why the step that starts rest[0] would not decide the line's
// exit status, or "" when it does.
func stepMasked(rest []shellSeg, errexit, pipefail bool) string {
	following := func(j int) bool {
		for _, s := range rest[j+1:] {
			if s.masked != "" {
				return true
			}
		}
		return false
	}
	i, op := 0, rest[0].next
	for pipefail && (op == "|" || op == "|&") && i+1 < len(rest) {
		i++
		op = rest[i].next
	}
	switch {
	case op == "|" || op == "|&":
		return "is piped without pipefail, so the pipeline reports only its last command"
	case op == "||":
		return "is followed by ||, which handles its failure"
	case op == "&":
		return "is backgrounded"
	case op == "" || !following(i):
		return ""
	case op == "&&":
		for j := i + 1; j < len(rest); j++ {
			if n := rest[j].next; n == "&" || ((n == ";" || n == "||") && following(j)) {
				return "is &&-chained into a list that a later " + n + " ends, which errexit does not stop"
			}
		}
		return ""
	case op == ";" && errexit:
		return ""
	}
	return "is followed by another command without errexit, which then decides the line's status"
}

// unfatalSteps reports each step on one logical recipe line whose failure the
// rest of the line would mask, and each occurrence of a step that is not a
// plain command (inside if, after !, in a subshell or a substitution).
func unfatalSteps(line string, steps []recipeStep) []string {
	segs := shellSegments(line)
	var why []string
	errexit, pipefail := false, false
	plain := map[string]int{}
	for k, s := range segs {
		w := commandWords(s.masked)
		if len(w) > 0 && w[0] == "set" {
			errexit, pipefail = applySet(w[1:], errexit, pipefail)
			continue
		}
		for _, st := range steps {
			if !hasWords(w, st.words...) {
				continue
			}
			plain[st.name]++
			if m := stepMasked(segs[k:], errexit, pipefail); m != "" {
				why = append(why, st.name+" "+m)
			}
		}
	}
	masked := maskQuotes(line)
	carries := false
	for _, st := range steps {
		n := len(st.at.FindAllStringIndex(masked, -1))
		carries = carries || n > 0
		if n > plain[st.name] {
			why = append(why, st.name+" is not a plain command, so its status is not the line's")
		}
		cmd := regexp.QuoteMeta(st.words[0])
		if regexp.MustCompile(`(^|[\s;&|(!{])((function\s+)?` + cmd + `\s*\(\s*\)|alias\s+` + cmd + `=)`).MatchString(masked) {
			why = append(why, st.name+"'s command "+st.words[0]+" is redefined on the line, so the real one may not run")
		}
	}
	if !carries {
		return why
	}
	stepSeen := false
	for _, s := range segs {
		w := commandWords(s.masked)
		for _, st := range steps {
			stepSeen = stepSeen || hasWords(w, st.words...)
		}
		switch {
		case len(w) == 0:
		case w[0] == "trap":
			why = append(why, "line sets a trap, which can rewrite its exit status")
		case stepSeen:
		case w[0] == "exec",
			(w[0] == "exit" || w[0] == "return") && (len(w) < 2 || !regexp.MustCompile(`^0*[1-9][0-9]*$`).MatchString(w[1])):
			why = append(why, "line runs "+w[0]+" before its step, which ends it first with a status the step did not set")
		}
	}
	return why
}

// registryHostRef is an image reference whose first path component is a
// registry host (has a dot or a port, or is localhost).
var registryHostRef = regexp.MustCompile(`^([a-z0-9-]+\.[a-z0-9.-]+|[a-z0-9.-]+:[0-9]+|localhost)/`)

func checkMakefile(text string) []deskdProblem {
	var ps []deskdProblem
	add := func(code, d string) { ps = append(ps, deskdProblem{code, d}) }
	phony := phonyTargets(text)
	targets := []string{"deskd-build", "deskd-image"}
	recipes := map[string][]string{}
	for _, t := range targets {
		r, ok := makeRecipe(text, t)
		if !phony[t] || !ok || len(r.lines) == 0 {
			add("target-missing", t+" must be on .PHONY and have a recipe")
		}
		if r.recipes > 1 {
			add("target-missing", t+" must have exactly one recipe; make runs only the last of its "+strconv.Itoa(r.recipes)+", not the one held here")
		}
		for i, l := range r.lines {
			if r.ignored[i] {
				add("step-not-fatal", t+": the recipe line is marked -, so make ignores its exit status: "+strconv.Quote(l))
			}
		}
		recipes[t] = r.lines
	}
	for _, why := range makeIgnoresErrors(text, targets) {
		add("step-not-fatal", why)
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
		for _, l := range recipes["deskd-build"] {
			for _, why := range unfatalSteps(l, goBuildSteps) {
				add("step-not-fatal", "deskd-build: the "+why+"; a failure would not fail the target: "+strconv.Quote(l))
			}
		}
	}

	// deskd-image: local docker build of the deskd context, then the layer scan.
	img := strings.Join(recipes["deskd-image"], "\n")
	if len(recipes["deskd-image"]) > 0 {
		var build []string
		for _, l := range recipes["deskd-image"] {
			for _, s := range shellSegments(l) {
				if hasWords(commandWords(s.masked), "docker", "build") {
					build = strings.Fields(s.text)
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
		// Presence only, on quote-masked text: a scan named inside an echo string
		// never runs. Whether its failure fails the target is step-not-fatal's.
		// The scanned reference must be the one the build tagged (when that tag is
		// the local name; tag-not-local reports otherwise), read from the unmasked
		// text at the masked match's offsets.
		scan := regexp.MustCompile(`(^|[\s;&|(!])sh\s+containers/scripts/layer-secret-scan\.sh\s+([^\s;&|]+)`)
		tag := ""
		for i, f := range build {
			if f == "-t" && i+1 < len(build) && strings.HasPrefix(strings.Trim(build[i+1], `"`), "assay-deskd:") {
				tag = build[i+1]
			}
		}
		var scanned []string
		for _, l := range recipes["deskd-image"] {
			for _, m := range scan.FindAllStringSubmatchIndex(maskQuotes(l), -1) {
				scanned = append(scanned, l[m[4]:m[5]])
			}
		}
		switch {
		case len(scanned) == 0:
			add("missing-secret-scan", "deskd-image must run containers/scripts/layer-secret-scan.sh on the built image")
		case tag != "" && !slices.Contains(scanned, tag):
			add("missing-secret-scan", "deskd-image scans "+strings.Join(scanned, ", ")+", not the image its build tagged, "+tag)
		}
		for _, l := range recipes["deskd-image"] {
			for _, why := range unfatalSteps(l, imageSteps) {
				add("step-not-fatal", "deskd-image: the "+why+"; a failure would not fail the target: "+strconv.Quote(l))
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
