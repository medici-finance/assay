package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// A concrete, well-formed digest used across these tests — 64 lowercase hex.
const testHarnessDigest = "sha256:" +
	"1111111111111111111111111111111111111111111111111111111111111111"

func testPin() harnessPin {
	return harnessPin{
		Image:  "ghcr.io/medici-finance/assay/desk-tools",
		Tag:    "v1.0.6",
		Digest: testHarnessDigest,
	}
}

// ---------------------------------------------------------------------------
// validate — fail-closed on anything that is not a concrete digest pin
// ---------------------------------------------------------------------------

func TestVICPinValidate(t *testing.T) {
	cases := []struct {
		name    string
		pin     harnessPin
		wantErr bool
	}{
		{"good digest pin", testPin(), false},
		{"empty image", harnessPin{Image: "", Digest: testHarnessDigest}, true},
		{"latest tag on image", harnessPin{Image: "ghcr.io/x/desk-tools:latest", Digest: testHarnessDigest}, true},
		{"no digest", harnessPin{Image: "ghcr.io/x/desk-tools", Digest: ""}, true},
		{"placeholder digest", harnessPin{Image: "ghcr.io/x/desk-tools", Digest: "PENDING-HARVEST-x"}, true},
		{"truncated digest", harnessPin{Image: "ghcr.io/x/desk-tools", Digest: "sha256:abc123"}, true},
		{"uppercased digest", harnessPin{Image: "ghcr.io/x/desk-tools", Digest: "sha256:" + strings.Repeat("A", 64)}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.pin.validate()
			if tc.wantErr && err == nil {
				t.Fatalf("validate() = nil, want an error")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("validate() = %v, want nil", err)
			}
		})
	}
}

func TestVICRefDigest(t *testing.T) {
	got := testPin().ref()
	want := "ghcr.io/medici-finance/assay/desk-tools@" + testHarnessDigest
	if got != want {
		t.Fatalf("ref() = %q, want %q", got, want)
	}
	if strings.Contains(got, ":latest") {
		t.Fatalf("ref() must never carry a floating tag: %q", got)
	}
}

// ---------------------------------------------------------------------------
// composeDockerArgs — the pure argv the wrapper would hand docker
// ---------------------------------------------------------------------------

func TestVICComposePosix(t *testing.T) {
	inv := containerInvocation{
		pin:      testPin(),
		root:     "/home/me/checkout",
		briefRel: "docs/streams/windows-port/brief-10.md",
		envFile:  "/tmp/role.env",
		inner:    buildInnerCommand("docs/streams/windows-port/brief-10.md", false, false, false, ""),
		uid:      1000,
		gid:      1000,
	}
	argv := composeDockerArgs(inv)
	joined := strings.Join(argv, " ")

	// Digest-pinned image ref, never latest.
	if !strings.Contains(joined, testPin().ref()) {
		t.Errorf("argv missing digest-pinned image ref: %q", joined)
	}
	if strings.Contains(joined, ":latest") {
		t.Errorf("argv must never reference :latest: %q", joined)
	}
	// Bind mount + workdir.
	if !argvHasPair(argv, "-v", "/home/me/checkout:/work") {
		t.Errorf("argv missing bind mount -v /home/me/checkout:/work: %q", joined)
	}
	if !argvHasPair(argv, "-w", "/work") {
		t.Errorf("argv missing -w /work: %q", joined)
	}
	// --user maps container writes to the host user so Evidence lands host-owned.
	if !argvHasPair(argv, "--user", "1000:1000") {
		t.Errorf("argv missing --user 1000:1000: %q", joined)
	}
	// Credentials: only the env-file PATH is passed through.
	if !argvHasPair(argv, "--env-file", "/tmp/role.env") {
		t.Errorf("argv missing --env-file passthrough: %q", joined)
	}
	// The inner command runs verifyrun WITHOUT --in-container (no recursion) and
	// addresses the brief under /work.
	if !strings.Contains(joined, "statusgen verifyrun --brief docs/streams/windows-port/brief-10.md") {
		t.Errorf("argv missing inner verifyrun invocation: %q", joined)
	}
	if strings.Contains(joined, "--in-container") {
		t.Errorf("inner command must NOT re-pass --in-container (would recurse): %q", joined)
	}
	// The image ref must come BEFORE the inner command (docker run <ref> <cmd…>).
	refIdx, innerIdx := indexOf(argv, testPin().ref()), indexOf(argv, "statusgen")
	if refIdx < 0 || innerIdx < 0 || refIdx > innerIdx {
		t.Errorf("image ref must precede the inner command: ref@%d inner@%d in %q", refIdx, innerIdx, joined)
	}
}

func TestVICComposeWin(t *testing.T) {
	// On Windows there is no POSIX uid (os.Getuid() == -1); --user is omitted and
	// Docker Desktop maps ownership to the host user itself.
	inv := containerInvocation{
		pin:      testPin(),
		root:     `C:\src\checkout`,
		briefRel: "docs/streams/windows-port/brief-10.md",
		inner:    buildInnerCommand("docs/streams/windows-port/brief-10.md", false, false, false, ""),
		uid:      -1,
		gid:      -1,
	}
	argv := composeDockerArgs(inv)
	if indexOf(argv, "--user") >= 0 {
		t.Errorf("argv must omit --user when there is no POSIX uid: %q", strings.Join(argv, " "))
	}
}

// TestVICComposeSafeDirectory pins the Windows-backend fix: the launcher marks
// EXACTLY the bind-mounted /work tree safe for the inner git, so attribution
// succeeds when Docker Desktop bind-mounts the checkout root-owned under the
// unprivileged `desk` USER. It must never widen that trust to `*` (a global
// safe.directory would trust every tree the container ever sees).
func TestVICComposeSafeDirectory(t *testing.T) {
	inv := containerInvocation{
		pin:      testPin(),
		root:     "/home/me/checkout",
		briefRel: "b.md",
		inner:    buildInnerCommand("b.md", false, false, false, ""),
		uid:      1000, gid: 1000,
	}
	argv := composeDockerArgs(inv)
	joined := strings.Join(argv, " ")

	// The three ephemeral git-config env pairs that mark exactly /work safe.
	if !argvHasPair(argv, "-e", "GIT_CONFIG_COUNT=1") {
		t.Errorf("argv missing -e GIT_CONFIG_COUNT=1: %q", joined)
	}
	if !argvHasPair(argv, "-e", "GIT_CONFIG_KEY_0=safe.directory") {
		t.Errorf("argv missing -e GIT_CONFIG_KEY_0=safe.directory: %q", joined)
	}
	if !argvHasPair(argv, "-e", "GIT_CONFIG_VALUE_0=/work") {
		t.Errorf("argv missing -e GIT_CONFIG_VALUE_0=/work: %q", joined)
	}
	// Never wider than /work: a global `safe.directory=*` is refused.
	if strings.Contains(joined, "safe.directory=*") || strings.Contains(joined, "GIT_CONFIG_VALUE_0=*") {
		t.Errorf("safe.directory must be exactly /work, never a wildcard: %q", joined)
	}
	// The safe-directory value is precisely the bind-mount target, nothing else.
	if !argvHasPair(argv, "-e", "GIT_CONFIG_VALUE_0="+containerWorkDir) {
		t.Errorf("safe.directory value must equal the bind-mount target %q: %q", containerWorkDir, joined)
	}
}

func TestVICComposeNoEnv(t *testing.T) {
	inv := containerInvocation{
		pin:      testPin(),
		root:     "/r",
		briefRel: "b.md",
		envFile:  "",
		inner:    buildInnerCommand("b.md", false, false, false, ""),
		uid:      1000, gid: 1000,
	}
	if indexOf(composeDockerArgs(inv), "--env-file") >= 0 {
		t.Errorf("argv must omit --env-file when none is supplied")
	}
}

// TestVICComposeAttribEnv pins the #2092 fix: the launcher forwards the host's
// attribution env (GITHUB_ACTIONS, GITHUB_ACTOR) into the container, NAME-ONLY, so
// the inner verifyrun can attribute a CI run whose checkout carries no git identity
// (before the fix it refused could-not-attribute, exit 2), and the value is never
// rendered in the argv the launcher prints.
func TestVICComposeAttribEnv(t *testing.T) {
	inv := containerInvocation{
		pin:      testPin(),
		root:     "/r",
		briefRel: "b.md",
		inner:    buildInnerCommand("b.md", false, false, false, ""),
		uid:      1000, gid: 1000,
	}
	argv := composeDockerArgs(inv)
	joined := strings.Join(argv, " ")
	for _, name := range []string{"GITHUB_ACTIONS", "GITHUB_ACTOR"} {
		if !argvHasPair(argv, "-e", name) {
			t.Errorf("argv missing name-only -e %s (the container cannot attribute a CI run without it): %q", name, joined)
		}
		if strings.Contains(joined, name+"=") {
			t.Errorf("-e %s must be name-only (docker copies the host value); a KEY=VALUE form renders the value in the printed argv: %q", name, joined)
		}
	}
	// Every forwarded name is a docker option, so it must precede the image ref.
	refIdx := indexOf(argv, testPin().ref())
	for i, a := range argv {
		if a == "GITHUB_ACTOR" && i > refIdx {
			t.Errorf("-e GITHUB_ACTOR lands after the image ref (it would reach the inner command, not docker): %q", joined)
		}
	}
}

// TestVICAttribEnvClass is the CLASS guard for #2092: an environment variable the
// witness-runner derivation (executingRunner, verifyrun.go) reads that the launcher
// does not carry into the container. It walks executingRunner's body for every
// os.Getenv / os.LookupEnv call and fails naming any variable composeDockerArgs does
// not forward by name — so the next env source added to the derivation is red here
// before it can make `--in-container` refuse a run the host would have attributed.
// A non-literal argument fails closed (the guard cannot resolve it).
func TestVICAttribEnvClass(t *testing.T) {
	// Positive control: the collector must flag a planted read, or a broken matcher
	// would report the real function clean.
	planted := "package main\nimport \"os\"\nfunc executingRunner(root string) (string, string, bool) {\n" +
		"\tif os.Getenv(\"GITHUB_ACTIONS\") == \"true\" { _ = os.Getenv(\"PLANTED_ACTOR\") }\n" +
		"\t_, _ = os.LookupEnv(\"PLANTED_LOOKUP\")\n\treturn \"\", \"\", false\n}\n"
	got, err := runnerEnvReads(t, []byte(planted))
	if err != nil {
		t.Fatalf("positive control: %v", err)
	}
	if strings.Join(got, ",") != "GITHUB_ACTIONS,PLANTED_ACTOR,PLANTED_LOOKUP" {
		t.Fatalf("positive control: collector found %v, want the three planted reads", got)
	}

	src, err := os.ReadFile("verifyrun.go")
	if err != nil {
		t.Fatalf("could-not-check: %v", err)
	}
	names, err := runnerEnvReads(t, src)
	if err != nil {
		t.Fatalf("could-not-check: %v", err)
	}
	if len(names) == 0 {
		t.Fatalf("could-not-check: no env read found in executingRunner — the guard is not looking at the derivation")
	}
	argv := composeDockerArgs(containerInvocation{pin: testPin(), root: "/r", briefRel: "b.md", uid: -1, gid: -1})
	for _, n := range names {
		if !argvHasPair(argv, "-e", n) {
			t.Errorf("executingRunner reads $%s but the launcher does not forward it into the container — add it to attributionEnvVars (verifyincontainer.go), or --in-container refuses a run the host would attribute (#2092)", n)
		}
	}

	// The other half of the class (#2097): a git-config key the derivation reads.
	// The container sees the checkout's local config but never the host's global
	// or system config, so every key read anywhere on executingRunner's in-package
	// call graph must be resolved on the host and carried across.
	plantedCfg := map[string]string{"p.go": "package main\n" +
		"func executingRunner(root string) (string, string, bool) {\n" +
		"\t_ = gitConfigValue(root, \"user.name\")\n\t_ = helperX(root)\n\treturn \"\", \"\", false\n}\n" +
		"func helperX(root string) string { return gitConfigValue(root, \"planted.key\") }\n"}
	gotCfg, err := runnerCfgReads(plantedCfg)
	if err != nil {
		t.Fatalf("positive control: %v", err)
	}
	if strings.Join(gotCfg, ",") != "planted.key,user.name" {
		t.Fatalf("positive control: collector found %v, want the direct and the transitive planted read", gotCfg)
	}
	srcs, err := packageSources()
	if err != nil {
		t.Fatalf("could-not-check: %v", err)
	}
	keys, err := runnerCfgReads(srcs)
	if err != nil {
		t.Fatalf("could-not-check: %v", err)
	}
	if len(keys) == 0 {
		t.Fatalf("could-not-check: no git-config read found on executingRunner's call graph — the guard is not looking at the derivation")
	}
	var full []identityPair
	for _, k := range identityConfigKeys {
		full = append(full, identityPair{key: k, value: "v"})
	}
	argv = composeDockerArgs(containerInvocation{pin: testPin(), root: "/r", briefRel: "b.md", uid: -1, gid: -1, identity: full})
	for _, k := range keys {
		carried := false
		for i := 0; i+1 < len(argv); i++ {
			if argv[i] == "-e" && strings.HasPrefix(argv[i+1], "GIT_CONFIG_KEY_") && strings.HasSuffix(argv[i+1], "="+k) {
				carried = true
			}
		}
		if !carried {
			t.Errorf("the witness-runner derivation reads git config %s but the launcher does not carry it into the container — add it to identityConfigKeys (verifyincontainer.go), or --in-container refuses a run the host would attribute (#2097)", k)
		}
	}
}

// packageSources returns every non-test Go source in this package, by name.
func packageSources() (map[string]string, error) {
	paths, err := filepath.Glob("*.go")
	if err != nil {
		return nil, err
	}
	out := map[string]string{}
	for _, p := range paths {
		if strings.HasSuffix(p, "_test.go") {
			continue
		}
		raw, err := os.ReadFile(p)
		if err != nil {
			return nil, err
		}
		out[p] = string(raw)
	}
	return out, nil
}

// runnerCfgReads returns, sorted and de-duplicated, the literal keys passed to
// gitConfigValue anywhere on executingRunner's in-package call graph (plain
// function calls followed transitively). A non-literal key fails closed.
func runnerCfgReads(srcs map[string]string) ([]string, error) {
	fset := token.NewFileSet()
	funcs := map[string]*ast.FuncDecl{}
	for name, src := range srcs {
		f, err := parser.ParseFile(fset, name, src, 0)
		if err != nil {
			return nil, err
		}
		for _, d := range f.Decls {
			if fd, ok := d.(*ast.FuncDecl); ok && fd.Recv == nil && fd.Body != nil {
				funcs[fd.Name.Name] = fd
			}
		}
	}
	if funcs["executingRunner"] == nil {
		return nil, fmt.Errorf("no executingRunner function in the source")
	}
	seen := map[string]bool{"executingRunner": true}
	queue := []string{"executingRunner"}
	keys := map[string]bool{}
	var bad error
	for len(queue) > 0 {
		fn := funcs[queue[0]]
		queue = queue[1:]
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			id, ok := call.Fun.(*ast.Ident)
			if !ok {
				return true
			}
			if id.Name == "gitConfigValue" && len(call.Args) == 2 {
				lit, ok := call.Args[1].(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					bad = fmt.Errorf("%s calls gitConfigValue with a non-literal key; extend this guard to resolve it", fn.Name.Name)
					return true
				}
				if v, err := strconv.Unquote(lit.Value); err == nil {
					keys[v] = true
				}
				return true
			}
			if funcs[id.Name] != nil && !seen[id.Name] && id.Name != "gitConfigValue" {
				seen[id.Name] = true
				queue = append(queue, id.Name)
			}
			return true
		})
	}
	out := make([]string, 0, len(keys))
	for k := range keys {
		out = append(out, k)
	}
	sort.Strings(out)
	return out, bad
}

// TestVICArgvNoIdent is the class guard for the attribution SURFACE: no identity
// value may be rendered in the docker argv the launcher prints. Every inline
// `-e KEY=VALUE` must be one of the fixed, non-identity config pairs; anything
// else (a `-e GIT_CONFIG_VALUE_1=<name>`, a `-e GIT_AUTHOR_NAME=<name>`) is red.
func TestVICArgvNoIdent(t *testing.T) {
	ident := []identityPair{{"user.name", "Quillon Vex"}, {"user.email", "qvex@example.invalid"}}
	argv := composeDockerArgs(containerInvocation{pin: testPin(), root: "/r", briefRel: "b.md", uid: 1000, gid: 1000, identity: ident})
	joined := strings.Join(argv, " ")
	inlineOK := regexp.MustCompile(`^(GIT_CONFIG_COUNT|GIT_CONFIG_KEY_[0-9]+|GIT_CONFIG_VALUE_0)$`)
	for i := 0; i+1 < len(argv); i++ {
		if argv[i] != "-e" {
			continue
		}
		if k, _, ok := strings.Cut(argv[i+1], "="); ok && !inlineOK.MatchString(k) {
			t.Errorf("-e %s renders a value in the printed argv; an identity value must ride name-only via the docker client env", argv[i+1])
		}
	}
	for _, p := range ident {
		if strings.Contains(joined, p.value) {
			t.Errorf("identity value %q is in the argv: %q", p.value, joined)
		}
	}
	for _, want := range [][2]string{
		{"-e", "GIT_CONFIG_COUNT=3"},
		{"-e", "GIT_CONFIG_KEY_1=user.name"}, {"-e", "GIT_CONFIG_VALUE_1"},
		{"-e", "GIT_CONFIG_KEY_2=user.email"}, {"-e", "GIT_CONFIG_VALUE_2"},
	} {
		if !argvHasPair(argv, want[0], want[1]) {
			t.Errorf("argv missing %s %s: %q", want[0], want[1], joined)
		}
	}
	if refIdx := indexOf(argv, testPin().ref()); indexOf(argv, "GIT_CONFIG_VALUE_2") > refIdx {
		t.Errorf("identity env lands after the image ref (it would reach the inner command, not docker): %q", joined)
	}
}

// TestVICClientEnv pins dockerClientEnv: the identity values land in the docker
// client env under the names the argv forwards, replacing any stale same-named
// entry the host already carried, and the base env is untouched without one.
func TestVICClientEnv(t *testing.T) {
	base := []string{"PATH=/bin", "GIT_CONFIG_VALUE_1=stale"}
	if got := dockerClientEnv(base, nil); strings.Join(got, "|") != strings.Join(base, "|") {
		t.Errorf("no identity must leave the env untouched: %q", got)
	}
	got := dockerClientEnv(base, []identityPair{{"user.name", "--x=y"}, {"user.email", "e@example.invalid"}})
	want := "PATH=/bin|GIT_CONFIG_VALUE_1=--x=y|GIT_CONFIG_VALUE_2=e@example.invalid"
	if strings.Join(got, "|") != want {
		t.Errorf("dockerClientEnv = %q, want %q", strings.Join(got, "|"), want)
	}
}

// runnerEnvReads returns, in source order, the literal names executingRunner passes
// to os.Getenv / os.LookupEnv in src.
func runnerEnvReads(t *testing.T, src []byte) ([]string, error) {
	t.Helper()
	f, err := parser.ParseFile(token.NewFileSet(), "src.go", src, 0)
	if err != nil {
		return nil, err
	}
	var fn *ast.FuncDecl
	for _, d := range f.Decls {
		if fd, ok := d.(*ast.FuncDecl); ok && fd.Recv == nil && fd.Name.Name == "executingRunner" {
			fn = fd
		}
	}
	if fn == nil || fn.Body == nil {
		return nil, fmt.Errorf("no executingRunner function in the source")
	}
	var names []string
	var bad error
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		pkg, ok := sel.X.(*ast.Ident)
		if !ok || pkg.Name != "os" || (sel.Sel.Name != "Getenv" && sel.Sel.Name != "LookupEnv") || len(call.Args) != 1 {
			return true
		}
		lit, ok := call.Args[0].(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			bad = fmt.Errorf("executingRunner calls os.%s with a non-literal argument; extend this guard to resolve it", sel.Sel.Name)
			return true
		}
		v, err := strconv.Unquote(lit.Value)
		if err != nil {
			bad = err
			return true
		}
		names = append(names, v)
		return true
	})
	return names, bad
}

func TestVICInnerFlags(t *testing.T) {
	inner := buildInnerCommand("b.md", true, true, true, "30s")
	joined := strings.Join(inner, " ")
	for _, want := range []string{"--check", "--dry-run", "--ci", "--timeout 30s"} {
		if !strings.Contains(joined, want) {
			t.Errorf("inner command missing %q: %q", want, joined)
		}
	}
}

// ---------------------------------------------------------------------------
// readHarnessPin — reads the harness: block from paired-versions.yaml
// ---------------------------------------------------------------------------

func TestVICReadPin(t *testing.T) {
	root := t.TempDir()
	writePairedVersions(t, root, "  digest: "+testHarnessDigest+"\n")
	pin, err := readHarnessPin(root)
	if err != nil {
		t.Fatalf("readHarnessPin: %v", err)
	}
	if pin.Image != "ghcr.io/medici-finance/assay/desk-tools" || pin.Tag != "v1.0.6" || pin.Digest != testHarnessDigest {
		t.Fatalf("readHarnessPin = %+v", pin)
	}
}

func TestVICReadPinMissing(t *testing.T) {
	root := t.TempDir()
	// A paired-versions.yaml with no harness: block at all.
	dir := filepath.Join(root, "plugins", "assay")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "paired-versions.yaml"), []byte("plugin: \"1.0.0\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := readHarnessPin(root); err == nil {
		t.Fatalf("readHarnessPin should error on a missing harness: block (could-not-check is a failure, not a default)")
	}
}

// ---------------------------------------------------------------------------
// End-to-end with a FAKE docker on PATH — no real container ever runs
// ---------------------------------------------------------------------------

func TestVICRunDocker(t *testing.T) {
	root := t.TempDir()
	writePairedVersions(t, root, "  digest: "+testHarnessDigest+"\n")
	briefRel := filepath.Join("docs", "streams", "windows-port", "brief-10.md")
	writeFileWP10(t, filepath.Join(root, briefRel), "# brief\n")

	// A fake `docker` that records its argv and exits 0 — the wrapper's argv is the
	// unit under test, not a real container run (the offline envelope forbids one).
	argvFile := filepath.Join(root, "docker-argv.txt")
	bin := t.TempDir()
	writeStubBinWP10(t, bin, "docker", "#!/bin/sh\nprintf '%s\\n' \"$@\" > '"+argvFile+"'\nexit 0\n")
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	code := runInContainer(filepath.Join(root, briefRel), root, "", false, false, false, "", os.Stdout, os.Stderr)
	if code != verifyrunExitPass {
		t.Fatalf("runInContainer exit = %d, want %d", code, verifyrunExitPass)
	}
	got, err := os.ReadFile(argvFile)
	if err != nil {
		t.Fatalf("fake docker did not record its argv: %v", err)
	}
	recorded := string(got)
	for _, want := range []string{
		"run", "--rm", "-w", "/work",
		testPin().ref(),
		"statusgen", "verifyrun", "--brief", filepath.ToSlash(briefRel),
	} {
		if !strings.Contains(recorded, want) {
			t.Errorf("fake docker argv missing %q:\n%s", want, recorded)
		}
	}
	// The bind mount names this run's absolute root.
	absRoot, _ := filepath.Abs(root)
	if !strings.Contains(recorded, absRoot+":/work") {
		t.Errorf("fake docker argv missing bind mount %s:/work:\n%s", absRoot, recorded)
	}
}

func TestVICRunRefusePlaceholder(t *testing.T) {
	root := t.TempDir()
	writePairedVersions(t, root, "  digest: PENDING-HARVEST-x\n")
	writeFileWP10(t, filepath.Join(root, "b.md"), "# b\n")
	// A fake docker that WOULD succeed if it were ever called — proving the refusal
	// happens before any docker invocation.
	bin := t.TempDir()
	writeStubBinWP10(t, bin, "docker", "#!/bin/sh\nexit 0\n")
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	code := runInContainer(filepath.Join(root, "b.md"), root, "", false, false, false, "", os.Stdout, os.Stderr)
	if code == verifyrunExitPass {
		t.Fatalf("runInContainer must REFUSE a placeholder digest, got pass")
	}
}

func TestVICRunRefuseOutside(t *testing.T) {
	root := t.TempDir()
	writePairedVersions(t, root, "  digest: "+testHarnessDigest+"\n")
	other := t.TempDir()
	writeFileWP10(t, filepath.Join(other, "b.md"), "# b\n")
	bin := t.TempDir()
	writeStubBinWP10(t, bin, "docker", "#!/bin/sh\nexit 0\n")
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	code := runInContainer(filepath.Join(other, "b.md"), root, "", false, false, false, "", os.Stdout, os.Stderr)
	if code == verifyrunExitPass {
		t.Fatalf("a brief outside the bind-mounted root must be refused, got pass")
	}
}

// ---------------------------------------------------------------------------
// Host git identity across the container boundary (#2097)
// ---------------------------------------------------------------------------

// TestVICHostIdent pins the #2097 fix end to end: a host whose git identity lives
// ONLY in global config (the common Windows setup) launches --in-container, and the
// inner derivation — executingRunner, run in an emulated container that sees no
// host global/system config, only the env the launcher forwarded — attributes the
// run exactly as the host would. Before the fix the launcher forwarded no git
// identity and the inner run refused could-not-attribute (exit 2).
//
// Every case also pins the attribution surface: the identity VALUES never appear in
// the docker argv, in the line the launcher prints, or on its stderr — they ride
// only in the docker client's environment, copied name-only (`-e NAME`).
func TestVICHostIdent(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the fake docker is a POSIX sh stub")
	}
	cases := []struct {
		name      string
		user      string // the [user] section body of the host's GLOBAL config
		wantExit  int
		wantRun   string // inner runner; "" = the inner must refuse (no identity)
		wantFwd   map[string]string
		secrets   []string // must never surface in argv / stdout / stderr
		wantInErr string   // refusal must name this (never the value)
	}{
		{
			name: "global name and email", user: "\tname = Quillon Vex\n\temail = qvex@example.invalid\n",
			wantRun: "human:quillon",
			wantFwd: map[string]string{"user.name": "Quillon Vex", "user.email": "qvex@example.invalid"},
			secrets: []string{"Quillon", "qvex@example.invalid"},
		},
		{
			name: "only email", user: "\temail = qonly@example.invalid\n",
			wantRun: "human:qonly",
			wantFwd: map[string]string{"user.email": "qonly@example.invalid"},
			secrets: []string{"qonly@example.invalid"},
		},
		{
			name: "empty name falls to email", user: "\tname = \"\"\n\temail = qempty@example.invalid\n",
			wantRun: "human:qempty",
			wantFwd: map[string]string{"user.email": "qempty@example.invalid"},
			secrets: []string{"qempty@example.invalid"},
		},
		{
			name: "equals sign rides verbatim", user: "\tname = \"qa=qb Vex\"\n",
			wantRun: "human:qaqb",
			wantFwd: map[string]string{"user.name": "qa=qb Vex"},
			secrets: []string{"qa=qb"},
		},
		{
			// A value shaped like a docker option must never reach the argv, where
			// it would be parsed as one.
			name: "leading dash never reaches argv", user: "\tname = \"--privileged\"\n",
			wantRun: "human:privileged",
			wantFwd: map[string]string{"user.name": "--privileged"},
			secrets: []string{"--privileged"},
		},
		{
			// Nothing to forward: the launcher invents nothing, and the inner run
			// still fails closed.
			name: "no identity at all", user: "",
			wantRun: "",
			wantFwd: map[string]string{},
		},
		{
			// A control character cannot cross intact: refuse on the host, before
			// docker, naming the key and never the value.
			name: "newline refused on host", user: "\tname = \"Qline\\nqevil\"\n\temail = qnl@example.invalid\n",
			wantExit: verifyrunExitCouldNot, wantInErr: "user.name",
			secrets: []string{"qevil", "qnl@example.invalid"},
		},
		{
			name: "backspace refused", user: "\temail = \"qbs@example.invalid\\bqevil\"\n",
			wantExit: verifyrunExitCouldNot, wantInErr: "user.email",
			secrets: []string{"qevil", "qbs@example.invalid"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := vicHostIdentRoot(t, tc.user)
			argvFile, envFile := filepath.Join(t.TempDir(), "argv"), filepath.Join(t.TempDir(), "env")
			vicFakeDocker(t, argvFile, envFile)
			out, errOut := vicCapture(t), vicCapture(t)

			code := runInContainer(filepath.Join(root, "b.md"), root, "", false, false, false, "", out, errOut)
			stdout, stderr := vicRead(t, out), vicRead(t, errOut)
			if code != tc.wantExit {
				t.Fatalf("runInContainer exit = %d, want %d\nstdout: %s\nstderr: %s", code, tc.wantExit, stdout, stderr)
			}
			rawArgv, _ := os.ReadFile(argvFile)
			for _, s := range tc.secrets {
				for where, text := range map[string]string{"docker argv": string(rawArgv), "stdout": stdout, "stderr": stderr} {
					if strings.Contains(text, s) {
						t.Errorf("identity value %q surfaced in the %s:\n%s", s, where, text)
					}
				}
			}
			if tc.wantExit != verifyrunExitPass {
				if len(rawArgv) != 0 {
					t.Errorf("refusal must happen before docker runs; docker was invoked with:\n%s", rawArgv)
				}
				if !strings.Contains(stderr, tc.wantInErr) {
					t.Errorf("refusal must name %s: %q", tc.wantInErr, stderr)
				}
				return
			}
			argv := strings.Split(strings.TrimRight(string(rawArgv), "\n"), "\n")
			fwd := vicForwardedEnv(t, envFile)
			if got := vicForwardedIdent(argv, fwd); !mapsEqual(got, tc.wantFwd) {
				t.Errorf("forwarded identity = %v, want %v\nargv: %q", got, tc.wantFwd, argv)
			}
			runner, _, ok := vicInnerRunner(t, root, argv, fwd)
			switch {
			case tc.wantRun == "" && ok:
				t.Errorf("inner run attributed %q with no host identity — the launcher invented one", runner)
			case tc.wantRun != "" && (!ok || runner != tc.wantRun):
				t.Errorf("inner runner = (%q, %v), want %q — the container could not attribute a run the host would", runner, ok, tc.wantRun)
			}
		})
	}
}

// vicHostIdentRoot builds a checkout with NO local identity on a host whose only
// identity is the given global [user] section, with CI env and system config cleared.
func vicHostIdentRoot(t *testing.T, user string) string {
	t.Helper()
	t.Setenv("GITHUB_ACTIONS", "")
	t.Setenv("GITHUB_ACTOR", "")
	for _, k := range []string{"GIT_CONFIG_COUNT", "GIT_CONFIG_PARAMETERS"} {
		t.Setenv(k, "")
		os.Unsetenv(k)
	}
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("GIT_CONFIG_SYSTEM", filepath.Join(t.TempDir(), "absent"))
	global := filepath.Join(t.TempDir(), "gitconfig")
	body := ""
	if user != "" {
		body = "[user]\n" + user
	}
	writeFileWP10(t, global, body)
	t.Setenv("GIT_CONFIG_GLOBAL", global)
	root := t.TempDir()
	runGit(t, root, "init", "-q")
	writePairedVersions(t, root, "  digest: "+testHarnessDigest+"\n")
	writeFileWP10(t, filepath.Join(root, "b.md"), "# b\n")
	return root
}

// vicFakeDocker installs a docker stub that records its argv (one token per line)
// and, for every name-only `-e NAME`, the value docker would copy from its own
// environment — exactly what a real `docker run -e NAME` hands the container.
func vicFakeDocker(t *testing.T, argvFile, envFile string) {
	t.Helper()
	bin := t.TempDir()
	writeStubBinWP10(t, bin, "docker", "#!/bin/sh\n"+
		"printf '%s\\n' \"$@\" > '"+argvFile+"'\n"+
		": > '"+envFile+"'\nprev=\n"+
		"for a in \"$@\"; do\n"+
		"  if [ \"$prev\" = -e ]; then case \"$a\" in *=*) ;; *)\n"+
		"    if v=$(printenv \"$a\"); then printf '%s=%s\\n' \"$a\" \"$v\" >> '"+envFile+"'; fi ;; esac; fi\n"+
		"  prev=$a\ndone\nexit 0\n")
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func vicForwardedEnv(t *testing.T, envFile string) map[string]string {
	t.Helper()
	raw, err := os.ReadFile(envFile)
	if err != nil {
		t.Fatalf("fake docker recorded no env: %v", err)
	}
	m := map[string]string{}
	for _, line := range strings.Split(strings.TrimRight(string(raw), "\n"), "\n") {
		if k, v, ok := strings.Cut(line, "="); ok {
			m[k] = v
		}
	}
	return m
}

// vicForwardedIdent pairs each `-e GIT_CONFIG_KEY_<n>=<key>` in argv with the value
// the container receives for GIT_CONFIG_VALUE_<n>, skipping the safe.directory pair.
func vicForwardedIdent(argv []string, fwd map[string]string) map[string]string {
	got := map[string]string{}
	for i := 0; i+1 < len(argv); i++ {
		if argv[i] != "-e" || !strings.HasPrefix(argv[i+1], "GIT_CONFIG_KEY_") {
			continue
		}
		k, key, _ := strings.Cut(argv[i+1], "=")
		if key == "safe.directory" {
			continue
		}
		got[key] = fwd["GIT_CONFIG_VALUE_"+strings.TrimPrefix(k, "GIT_CONFIG_KEY_")]
	}
	return got
}

// vicInnerRunner emulates the container: no host global or system git config, and
// exactly the environment the docker argv establishes (inline KEY=VALUE pairs plus
// name-only names resolved from the docker client's env). It then runs the REAL
// inner derivation, executingRunner, against the checkout.
func vicInnerRunner(t *testing.T, root string, argv []string, fwd map[string]string) (string, string, bool) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(t.TempDir(), "absent"))
	for i := 0; i+1 < len(argv); i++ {
		if argv[i] != "-e" {
			continue
		}
		if k, v, ok := strings.Cut(argv[i+1], "="); ok {
			t.Setenv(k, v)
		} else if v, ok := fwd[k]; ok {
			t.Setenv(k, v)
		} else {
			t.Setenv(k, "")
			os.Unsetenv(k)
		}
	}
	return executingRunner(root)
}

func vicCapture(t *testing.T) *os.File {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "out")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { f.Close() })
	return f
}

func vicRead(t *testing.T, f *os.File) string {
	t.Helper()
	raw, err := os.ReadFile(f.Name())
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func mapsEqual(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if bv, ok := b[k]; !ok || bv != v {
			return false
		}
	}
	return true
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func writePairedVersions(t *testing.T, root, digestLine string) {
	t.Helper()
	body := "plugin: \"1.0.0\"\n" +
		"harness:\n" +
		"  release_home: medici-finance/assay\n" +
		"  image: ghcr.io/medici-finance/assay/desk-tools\n" +
		"  tag: v1.0.6\n" +
		digestLine
	writeFileWP10(t, filepath.Join(root, "plugins", "assay", "paired-versions.yaml"), body)
}

func writeFileWP10(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeStubBinWP10(t *testing.T, dir, name, script string) {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
}

func argvHasPair(argv []string, flag, val string) bool {
	for i := 0; i+1 < len(argv); i++ {
		if argv[i] == flag && argv[i+1] == val {
			return true
		}
	}
	return false
}

func indexOf(argv []string, s string) int {
	for i, a := range argv {
		if a == s {
			return i
		}
	}
	return -1
}
