package main

// perfrecord_test.go — #3101: with ASSAY_TELEMETRY=1 statusgen writes the same local perf-v2
// record the desk tools write, through the shared perfrec module; with it unset it writes
// nothing and behaves exactly as before. Phases and the forge counter: perfphases_test.go,
// perfhttp_test.go.

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/gitquiet/goquiet"
	"github.com/medici-finance/assay/perfrec"
)

type perfRun struct {
	out, errOut string
	code        int
}

// perfTestEnv is the environment every child of these tests runs with: the caller's, minus
// the perf switch and context keys, HOME, the desk loop, every forge credential, the forge repo and gh/XDG config
// locations, and every GIT_* variable (so a child can neither reach a forge with a real token
// nor resolve a repository the test did not make), with HOME set to the throwaway one and git
// discovery stopped at ceiling.
func perfTestEnv(home, ceiling string) []string {
	var env []string
	for _, e := range os.Environ() {
		key, _, _ := strings.Cut(e, "=")
		switch k := strings.ToUpper(key); {
		case strings.HasPrefix(k, "GIT_"), strings.HasPrefix(k, "ASSAY_PERF_"),
			k == "ASSAY_TELEMETRY", k == "HOME", k == "USERPROFILE", k == "DESK_LOOP",
			k == "GH_TOKEN", k == "GITHUB_TOKEN", k == "GH_ENTERPRISE_TOKEN",
			k == "GITHUB_ENTERPRISE_TOKEN", k == "GITLAB_TOKEN", k == "STATUSGEN_GITLAB_TOKEN",
			k == "CI_JOB_TOKEN", k == "GITHUB_REPOSITORY", k == "GITHUB_GRAPHQL_URL", k == "GH_CONFIG_DIR", k == "XDG_CONFIG_HOME":
			continue
		}
		env = append(env, e)
	}
	return append(env, "HOME="+home, "USERPROFILE="+home, "GIT_CEILING_DIRECTORIES="+ceiling)
}

// runPerfBinary runs the built statusgen with a throwaway HOME and the given ASSAY_TELEMETRY
// (set=false removes it), from a scratch working directory, in perfTestEnv.
func runPerfBinary(t *testing.T, bin, home string, set bool, val string, args ...string) perfRun {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Dir = t.TempDir()
	env := perfTestEnv(home, filepath.Dir(cmd.Dir))
	if set {
		env = append(env, "ASSAY_TELEMETRY="+val)
	}
	cmd.Env = env
	var so, se bytes.Buffer
	cmd.Stdout, cmd.Stderr = &so, &se
	err := cmd.Run()
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatalf("run %v: %v", args, err)
	}
	return perfRun{so.String(), se.String(), code}
}

// buildPerfBinary builds statusgen with any extra `go build` flags. The plain form passes
// -buildvcs=false: a binary with no VCS information is the one whose record must stay
// "unpinned", whether or not the test runs inside a git checkout.
func buildPerfBinary(t *testing.T, flags ...string) string {
	t.Helper()
	return buildPerfBinaryWith(t, flags...)
}

// buildPerfBinaryWith builds statusgen with extra `go build` flags (an -ldflags stamp).
func buildPerfBinaryWith(t *testing.T, flags ...string) string {
	t.Helper()
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go toolchain unavailable")
	}
	if len(flags) == 0 {
		flags = []string{"-buildvcs=false"}
	}
	bin := filepath.Join(t.TempDir(), "statusgen")
	args := append(append([]string{"build"}, flags...), "-o", bin, ".")
	if out, err := goquiet.Cmd(exec.Command("go", args...)).CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	return bin
}

// A build stamp is free text set at link time. Only a commit-shaped stamp may reach the record:
// a stamp carrying a URL with a credential-looking query, or a builder's path, must not appear
// anywhere in the written line, and the record's sourceSHA reads "unpinned". The path stamp is
// relative because the public tree carries no absolute machine paths.
// regression: #3101
func TestPerfHostileStampDropped(t *testing.T) {
	for name, stamp := range map[string]string{
		"url":  "https://forge.invalid/rel?token=fake-stamp-token",
		"path": "build/src/assay",
	} {
		t.Run(name, func(t *testing.T) {
			bin := buildPerfBinaryWith(t, "-buildvcs=false", "-ldflags", "-X main.statusgenVersion="+stamp)
			run, _, home := perfFixtureRun(t, bin, true, "--lint", "--root", "{root}")
			if run.code != 0 {
				t.Fatalf("exit %d\nstdout: %s\nstderr: %s", run.code, run.out, run.errOut)
			}
			files, _ := filepath.Glob(filepath.Join(home, ".config", "assay", "perf", "*.jsonl"))
			var raw []byte
			for _, f := range files {
				b, err := os.ReadFile(f)
				if err != nil {
					t.Fatal(err)
				}
				raw = append(raw, b...)
			}
			if len(bytes.TrimSpace(raw)) == 0 {
				t.Fatal("switch on wrote no perf line")
			}
			for _, leak := range []string{stamp, "fake-stamp-token", "forge.invalid", "build/src"} {
				if bytes.Contains(raw, []byte(leak)) {
					t.Errorf("perf line carries %q from the build stamp: %s", leak, raw)
				}
			}
			recs := perfRecords(t, home)
			if len(recs) != 1 || recs[0]["sourceSHA"] != "unpinned" {
				t.Errorf("records = %v, want one with sourceSHA unpinned", recs)
			}
		})
	}
}

func perfRecords(t *testing.T, home string) []map[string]any {
	t.Helper()
	files, _ := filepath.Glob(filepath.Join(home, ".config", "assay", "perf", "*.jsonl"))
	var out []map[string]any
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
			if line == "" {
				continue
			}
			var rec map[string]any
			if err := json.Unmarshal([]byte(line), &rec); err != nil {
				t.Fatalf("perf line is not JSON: %q: %v", line, err)
			}
			out = append(out, rec)
		}
	}
	return out
}

// TestPerfOffWritesNothing: unset, "", "0", "true" and "yes" are all OFF — no state
// directory, no perf directory, no install id — and the run's output and exit code are the
// same as with the switch on.
//
// regression: #3101
func TestPerfOffWritesNothing(t *testing.T) {
	bin := buildPerfBinary(t)
	onHome := t.TempDir()
	on := runPerfBinary(t, bin, onHome, true, "1", "--version")
	if recs := perfRecords(t, onHome); len(recs) != 1 {
		t.Fatalf("ASSAY_TELEMETRY=1 wrote %d records for one run, want exactly 1", len(recs))
	}
	for _, c := range []struct {
		name string
		set  bool
		val  string
	}{{"unset", false, ""}, {"empty", true, ""}, {"zero", true, "0"}, {"true", true, "true"}, {"yes", true, "yes"}} {
		t.Run(c.name, func(t *testing.T) {
			home := t.TempDir()
			got := runPerfBinary(t, bin, home, c.set, c.val, "--version")
			if _, err := os.Stat(filepath.Join(home, ".config")); !os.IsNotExist(err) {
				entries, _ := os.ReadDir(filepath.Join(home, ".config"))
				t.Errorf("switch off but %s/.config exists (%v): nothing may be created", home, entries)
			}
			if got != on {
				t.Errorf("switch off changed behaviour: got %+v, with the switch on %+v", got, on)
			}
		})
	}
}

// TestPerfOnWritesOneRecord: ASSAY_TELEMETRY=1 appends exactly one perf-v2 line per process,
// in the desk tools' schema, for success, a usage error, and a real mode; exit code and
// output are unchanged by recording.
//
// regression: #3101, #3327
func TestPerfOnWritesOneRecord(t *testing.T) {
	bin := buildPerfBinary(t)
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS("testdata/goodrepo")); err != nil {
		t.Fatal(err)
	}
	wantKeys := []string{"boundary", "counters", "cpu", "exit", "install", "phaseOverlap", "phases",
		"result", "role", "schema", "sourceSHA", "tool", "ts", "unavailable", "verb", "wallMs"}
	if runtime.GOOS != "windows" {
		wantKeys = append(wantKeys, "cpuMs")
	}
	sort.Strings(wantKeys)
	installRE := regexp.MustCompile(`^[0-9a-f]{32}$`)

	for _, c := range []struct {
		name     string
		args     []string
		verb     string
		exit     int
		result   string
		sameAsOn bool
		roster   bool
	}{
		{"version", []string{"--version"}, "version", 0, "ok", true, false},
		{"mode flag", []string{"--alarms", "--root", root}, "alarms", 0, "ok", true, false},
		{"lint mode", []string{"--lint", "--root", root}, "lint", 0, "ok", true, true},
		{"lint unconfigured", []string{"--lint", "--root", root}, "lint", 5, "refused", true, false},
		{"usage error", []string{"--no-such-flag"}, "regen", 2, "unverifiable", true, false},
		{"unknown subcommand", []string{"frobnicate"}, "other", 2, "unverifiable", true, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			home := t.TempDir()
			if c.roster {
				dir := filepath.Join(home, ".config", "assay")
				if err := os.MkdirAll(dir, 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, "roster.env"), []byte(fixtureRoster+darFixtureExtra()), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			off := runPerfBinary(t, bin, home, false, "", c.args...)
			if recs := perfRecords(t, home); len(recs) != 0 {
				t.Fatalf("switch unset wrote %d records", len(recs))
			}
			if c.name == "lint unconfigured" && !strings.Contains(off.out, "LINT: REFUSED configuration") {
				t.Errorf("unconfigured lint did not report the configuration refusal: %s", off.out)
			}
			on := runPerfBinary(t, bin, home, true, "1", c.args...)
			recs := perfRecords(t, home)
			if len(recs) != 1 {
				t.Fatalf("one run wrote %d records, want exactly 1", len(recs))
			}
			rec := recs[0]
			var keys []string
			for k := range rec {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			if strings.Join(keys, ",") != strings.Join(wantKeys, ",") {
				t.Errorf("perf-v2 key set drifted from the desk tools':\n got  %v\n want %v", keys, wantKeys)
			}
			for field, want := range map[string]any{"schema": "perf-v2", "tool": "statusgen", "verb": c.verb,
				"role": "none", "result": c.result, "exit": float64(c.exit), "sourceSHA": "unpinned"} {
				if rec[field] != want {
					t.Errorf("%s = %v, want %v", field, rec[field], want)
				}
			}
			if id, _ := rec["install"].(string); !installRE.MatchString(id) {
				t.Errorf("install = %q, want 32 lowercase hex", id)
			}
			if ts, _ := rec["ts"].(string); ts == "" {
				t.Errorf("ts missing")
			} else if _, err := time.Parse(time.RFC3339, ts); err != nil {
				t.Errorf("ts %q is not RFC3339: %v", ts, err)
			}
			if on.code != c.exit || off.code != c.exit {
				t.Errorf("exit codes: on %d, off %d, want %d", on.code, off.code, c.exit)
			}
			if c.sameAsOn && (on.out != off.out || on.code != off.code) {
				t.Errorf("recording changed stdout or the exit code:\n on  %+v\n off %+v", on, off)
			}
			if _, err := os.Stat(filepath.Join(home, ".config", "assay", "perf", "install-id")); err != nil {
				t.Errorf("install-id file missing: %v", err)
			}
		})
	}
}

// TestPerfRecordRevision: a statusgen built the way the release builds it — inside a git
// checkout, with only the release tag stamped (-X main.statusgenVersion) — records the commit
// it was built from as sourceSHA, not "unpinned". `--version` still prints the release tag.
// Both writers are held to it: the perf-v2 line and the perf-v3 line a selected profile writes.
// regression: F-release-binary-unpinned
func TestPerfRecordRevision(t *testing.T) {
	head, err := exec.Command("git", "rev-parse", "HEAD").Output()
	if err != nil {
		t.Skipf("not a git checkout (git rev-parse HEAD: %v) — no VCS information to embed", err)
	}
	want := strings.TrimSpace(string(head))[:12]
	bin := buildPerfBinary(t, "-buildvcs=true", "-ldflags", "-X main.statusgenVersion=v9.9.9")
	for _, c := range []struct{ name, schema, profile string }{
		{"perf-v2", "perf-v2", ""},
		{"perf-v3", "perf-v3", "tool"},
	} {
		t.Run(c.name, func(t *testing.T) {
			home := t.TempDir()
			cmd := exec.Command(bin, "--version")
			cmd.Dir = t.TempDir()
			cmd.Env = append(perfTestEnv(home, filepath.Dir(cmd.Dir)), "ASSAY_TELEMETRY=1")
			if c.profile != "" {
				cmd.Env = append(cmd.Env, "ASSAY_PERF_PROFILE="+c.profile)
			}
			out, err := cmd.Output()
			if err != nil {
				t.Fatalf("run --version: %v", err)
			}
			if strings.TrimSpace(string(out)) != "v9.9.9" {
				t.Errorf("--version = %q, want the stamped release tag v9.9.9", out)
			}
			recs := perfRecords(t, home)
			if len(recs) != 1 {
				t.Fatalf("one run wrote %d records, want exactly 1", len(recs))
			}
			if sch := recs[0]["schema"]; sch != c.schema {
				t.Fatalf("schema = %v, want %s (the test must exercise that writer)", sch, c.schema)
			}
			if sha := recs[0]["sourceSHA"]; sha != want {
				t.Errorf("sourceSHA = %v, want %s (the build's vcs.revision) — a release-tag-only stamp must not record unpinned", sha, want)
			}
		})
	}
}

// TestPerfVerbRoster pins the argv→verb mapping: always a roster member, never input text.
func TestPerfVerbRoster(t *testing.T) {
	for _, c := range []struct {
		args []string
		want string
	}{
		{nil, "regen"},
		{[]string{"--root", "/some/private/path"}, "regen"},
		{[]string{"--version"}, "version"},
		{[]string{"version"}, "version"},
		{[]string{"verifyrun", "docs/streams/x/01-secret.md"}, "verifyrun"},
		{[]string{"init", "--dry-run"}, "init"},
		{[]string{"--lint", "--root", "."}, "lint"},
		{[]string{"-lint"}, "lint"},
		{[]string{"--root", ".", "--check"}, "check"},
		{[]string{"--close-verify=stream/01"}, "close-verify"},
		{[]string{"--since", "2026-01-01", "--dora-json"}, "dora-json"},
		{[]string{"user-typed-free-text"}, "other"},
		{[]string{"--unknown-flag"}, "regen"},
	} {
		if got := perfVerb(c.args); got != c.want {
			t.Errorf("perfVerb(%v) = %q, want %q", c.args, got, c.want)
		}
	}
}

// TestPerfModeRosterIsRegistered: every flag in the mode roster is a flag main() really
// registers, so the roster cannot name a mode that was renamed away.
func TestPerfModeRosterIsRegistered(t *testing.T) {
	src, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	reg := map[string]bool{}
	for _, m := range regexp.MustCompile(`flag\.(?:Bool|String|Int)\("([a-z][a-z0-9-]*)"`).FindAllStringSubmatch(string(src), -1) {
		reg[m[1]] = true
	}
	for _, f := range perfModeFlags {
		if !reg[f] {
			t.Errorf("perfModeFlags names %q, which main.go does not register as a flag", f)
		}
	}
	seen := map[string]bool{}
	for _, v := range append(append([]string{}, perfSubcommands...), perfModeFlags...) {
		if v == "lint" || v == "regen" {
			continue // a sub-command and a flag by the same name
		}
		if seen[v] {
			t.Errorf("roster entry %q appears twice", v)
		}
		seen[v] = true
	}
}

// exitCallers returns the non-test files under dir whose AST calls os.Exit, and whether
// main.go calls flag.Parse or builds an ExitOnError flag set.
func exitCallers(t *testing.T, dir string) (callers []string, flagExits []string) {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(dir, "*.go"))
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range files {
		if strings.HasSuffix(p, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(token.NewFileSet(), p, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", p, err)
		}
		base := filepath.Base(p)
		ast.Inspect(f, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.SelectorExpr:
				id, ok := x.X.(*ast.Ident)
				if !ok {
					return true
				}
				if id.Name == "os" && x.Sel.Name == "Exit" {
					callers = append(callers, base)
				}
				if base == "main.go" && id.Name == "flag" && (x.Sel.Name == "Parse" || x.Sel.Name == "ExitOnError") {
					flagExits = append(flagExits, "flag."+x.Sel.Name)
				}
			}
			return true
		})
	}
	return callers, flagExits
}

// mainReturnExits parses the package's func main and returns the position of every return
// statement belonging to main itself (not to a nested function literal), plus whether the
// body's last statement is a call to exitProcess. A bare return ends the process with exit 0
// without passing through the recorder, the same blind spot as a direct os.Exit.
func mainReturnExits(t *testing.T, dir string) (returns []string, endsInExit bool, found bool) {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(dir, "*.go"))
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range files {
		if strings.HasSuffix(p, "_test.go") {
			continue
		}
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, p, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", p, err)
		}
		for _, d := range f.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok || fd.Recv != nil || fd.Name.Name != "main" || fd.Body == nil {
				continue
			}
			found = true
			ast.Inspect(fd.Body, func(n ast.Node) bool {
				switch x := n.(type) {
				case *ast.FuncLit:
					return false
				case *ast.ReturnStmt:
					returns = append(returns, fset.Position(x.Pos()).String())
				}
				return true
			})
			if n := len(fd.Body.List); n > 0 {
				if es, ok := fd.Body.List[n-1].(*ast.ExprStmt); ok {
					if call, ok := es.X.(*ast.CallExpr); ok {
						if id, ok := call.Fun.(*ast.Ident); ok && id.Name == "exitProcess" {
							endsInExit = true
						}
					}
				}
			}
		}
	}
	return returns, endsInExit, found
}

// TestMainExitsOnlyViaRecorder: main() ends the process only through exitProcess, so no exit
// path escapes the perf record. The only other os.Exit in the package is the check:ci sandbox
// helper child (netns.go), which must NOT record.
//
// regression: #3101
func TestMainExitsOnlyViaRecorder(t *testing.T) {
	// Negative control: a planted direct os.Exit is seen.
	plant := t.TempDir()
	if err := os.WriteFile(filepath.Join(plant, "x.go"), []byte("package main\nimport \"os\"\nfunc f() { os.Exit(1) }\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if c, _ := exitCallers(t, plant); len(c) != 1 {
		t.Fatalf("the scan missed a planted os.Exit: %v", c)
	}

	// Negative control: a planted bare return in main is seen, and a main that does not end
	// in exitProcess is reported as such.
	plantRet := t.TempDir()
	if err := os.WriteFile(filepath.Join(plantRet, "m.go"), []byte("package main\nfunc main() {\n\tif true {\n\t\treturn\n\t}\n\texitProcess(0)\n}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if r, ends, ok := mainReturnExits(t, plantRet); !ok || len(r) != 1 || !ends {
		t.Fatalf("the scan missed a planted bare return in main: returns=%v endsInExit=%v found=%v", r, ends, ok)
	}

	rets, endsInExit, found := mainReturnExits(t, ".")
	if !found {
		t.Fatal("func main not found in the package")
	}
	if len(rets) != 0 {
		t.Errorf("main() has a bare return that ends the process without recording: %v — use exitProcess(0)", rets)
	}
	if !endsInExit {
		t.Errorf("main() must end with a call to exitProcess so falling off the end cannot skip the record")
	}

	callers, flagExits := exitCallers(t, ".")
	allowed := map[string]int{"perfrecord.go": 1, "netns.go": 1}
	got := map[string]int{}
	for _, c := range callers {
		got[c]++
	}
	for file, n := range got {
		if allowed[file] != n {
			t.Errorf("%s calls os.Exit %d time(s); only the recorder's exitProcess (perfrecord.go) and the netns helper child may — route main() exits through exitProcess", file, n)
		}
	}
	for file, want := range allowed {
		if got[file] != want {
			t.Errorf("%s: %d os.Exit call(s), want %d", file, got[file], want)
		}
	}
	if len(flagExits) != 0 {
		t.Errorf("main.go parses flags with an exit the recorder cannot see: %v", flagExits)
	}
}

// --- perf-v3 (statusgen/26) ------------------------------------------------------------------

// perfShims are the online-capable commands the v3 flow tests replace on PATH: each one logs
// its call and refuses. deskread is the fake child: it also captures the perf context it was
// handed. git logs and forwards to the real git so remote subcommands are visible.
var perfShims = []string{"gh", "glab", "curl", "wget", "ssh", "nc", "docker"}

type perfLab struct {
	bin, root, home, path, log, capture, old string
	helpEnv                                  string // written only if the --help probe sees an inherited variable
}

// newPerfLab builds statusgen, copies the offline fixture, writes the test roster into a
// throwaway HOME and puts the recording shims first on PATH.
func newPerfLab(t *testing.T) perfLab {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("shell shims")
	}
	gitBin, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git not on PATH")
	}
	l := perfLab{bin: buildPerfBinary(t), root: t.TempDir(), home: t.TempDir()}
	if err := os.CopyFS(l.root, os.DirFS("testdata/goodrepo")); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(l.home, ".config", "assay")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "roster.env"), []byte(fixtureRoster+darFixtureExtra()), 0o600); err != nil {
		t.Fatal(err)
	}
	shims := t.TempDir()
	l.log = filepath.Join(t.TempDir(), "exec.log")
	l.capture = filepath.Join(t.TempDir(), "child.env")
	l.old = filepath.Join(t.TempDir(), "old-deskread")
	l.helpEnv = filepath.Join(t.TempDir(), "help-env")
	write := func(name, body string) {
		if err := os.WriteFile(filepath.Join(shims, name), []byte("#!/bin/sh\n"+body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, n := range perfShims {
		write(n, "printf '%s\\n' \""+n+" $*\" >> \""+l.log+"\"\nexit 1\n")
	}
	write("git", "printf '%s\\n' \"git $*\" >> \""+l.log+"\"\nexec \""+gitBin+"\" \"$@\"\n")
	// The fake deskread answers the version-gate probe (--help, run with an empty environment,
	// so builtins only) as a build that takes the --perf-* flags, or, while l.old exists, as one
	// that predates them and refuses an unknown flag the way the real parser does. A probe that
	// sees any variable every run sets (the switch, the desk loop, HOME) touches l.helpEnv.
	write("deskread", "printf '%s\\n' \"deskread $*\" >> \""+l.log+"\"\n"+
		"if [ \"$1\" = --help ]; then\n"+
		"  if [ -n \"${ASSAY_TELEMETRY-}${DESK_LOOP-}${HOME-}\" ]; then : > \""+l.helpEnv+"\"; fi\n"+
		"  if [ -f \""+l.old+"\" ]; then printf 'Usage: deskread <kind> [flags]\\n'; exit 0; fi\n"+
		"  for f in "+strings.Join(perfChildFlags, " ")+"; do printf '  --%s string\\n' \"$f\"; done; exit 0\n"+
		"fi\n"+
		"{ echo '--- child'; for a in \"$@\"; do printf 'arg %s\\n' \"$a\"; done\n"+
		"  env | grep -E '^(ASSAY_PERF_[A-Z_]*|ASSAY_TELEMETRY|DESK_LOOP)=' | sort; } >> \""+l.capture+"\"\n"+
		"if [ -f \""+l.old+"\" ]; then for a in \"$@\"; do case \"$a\" in --perf-*) echo \"deskread: unknown flag: ${a%%=*}\" >&2; exit 5;; esac; done; fi\n"+
		"exit 6\n")
	l.path = shims + string(os.PathListSeparator) + os.Getenv("PATH")
	return l
}

// run executes the binary with a scrubbed environment (no inherited telemetry, perf context,
// desk loop or forge credential) plus env.
func (l perfLab) run(t *testing.T, env []string, args ...string) perfRun {
	t.Helper()
	cmd := exec.Command(l.bin, args...)
	cmd.Dir = l.root
	for _, e := range os.Environ() {
		k, _, _ := strings.Cut(e, "=")
		switch {
		case strings.HasPrefix(k, "ASSAY_PERF_"), strings.HasSuffix(k, "_TOKEN"):
			continue
		}
		switch k {
		case "ASSAY_TELEMETRY", "HOME", "USERPROFILE", "DESK_LOOP", "PATH":
			continue
		}
		cmd.Env = append(cmd.Env, e)
	}
	cmd.Env = append(cmd.Env, "HOME="+l.home, "USERPROFILE="+l.home, "PATH="+l.path)
	cmd.Env = append(cmd.Env, env...)
	var so, se bytes.Buffer
	cmd.Stdout, cmd.Stderr = &so, &se
	err := cmd.Run()
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatalf("run %v: %v", args, err)
	}
	return perfRun{so.String(), se.String(), code}
}

// lines returns every perf day-file line written so far, in file order.
func (l perfLab) lines(t *testing.T) []string {
	t.Helper()
	files, _ := filepath.Glob(filepath.Join(l.home, ".config", "assay", "perf", "*.jsonl"))
	sort.Strings(files)
	var out []string
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
			if line != "" {
				out = append(out, line)
			}
		}
	}
	return out
}

// one runs once and returns the single record line that run appended.
func (l perfLab) one(t *testing.T, env []string, args ...string) (perfRun, string) {
	t.Helper()
	before := len(l.lines(t))
	r := l.run(t, env, args...)
	after := l.lines(t)
	if len(after) != before+1 {
		t.Fatalf("%v wrote %d records, want exactly 1", args, len(after)-before)
	}
	return r, after[len(after)-1]
}

func (l perfLab) execLog(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile(l.log)
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	return string(raw)
}

// perfChild is one fake deskread start: its argv and the perf-relevant environment it inherited.
type perfChild struct {
	args []string
	env  map[string]string
}

// children parses the fake deskread's captures, one per child start (gate probes excluded).
func (l perfLab) children(t *testing.T) []perfChild {
	t.Helper()
	raw, err := os.ReadFile(l.capture)
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	var out []perfChild
	for _, line := range strings.Split(string(raw), "\n") {
		if line == "--- child" {
			out = append(out, perfChild{env: map[string]string{}})
			continue
		}
		if len(out) == 0 {
			continue
		}
		c := &out[len(out)-1]
		if a, ok := strings.CutPrefix(line, "arg "); ok {
			c.args = append(c.args, a)
		} else if k, v, ok := strings.Cut(line, "="); ok {
			c.env[k] = v
		}
	}
	return out
}

// perfArgs are the --perf-* words in the child's argv.
func (c perfChild) perfArgs() []string {
	var out []string
	for _, a := range c.args {
		if strings.HasPrefix(a, "--perf-") {
			out = append(out, a)
		}
	}
	return out
}

// resolve is what the child's own resolver selects at start: the --perf-* flags it was handed,
// then the environment it inherited (the order deskread's parser and perfrec apply).
func (c perfChild) resolve() perfrec.Context {
	in := perfrec.ContextInput{LookupEnv: func(k string) (string, bool) { v, ok := c.env[k]; return v, ok }}
	for _, a := range c.perfArgs() {
		name, v, _ := strings.Cut(strings.TrimPrefix(a, "--"), "=")
		val := perfrec.Value{Value: v, Present: true}
		switch name {
		case "perf-profile":
			in.Profile = val
		case "perf-session-id":
			in.SessionID = val
		case "perf-agent-id":
			in.AgentID = val
		case "perf-parent-agent-id":
			in.ParentAgentID = val
		case "perf-parent-invocation-id":
			in.ParentInvocationID = val
		case "perf-role":
			in.Role = val
		case "perf-work":
			in.Work, in.WorkPresent = append(in.Work, v), true
		}
	}
	return perfrec.ResolveContext(in)
}

func decodeV3(t *testing.T, line string) perfrec.Record {
	t.Helper()
	rec, err := perfrec.Decode([]byte(line))
	if err != nil {
		t.Fatalf("the shared reader refused statusgen's record: %v\n%s", err, line)
	}
	if rec.Schema != perfrec.SchemaV3 || rec.V3 == nil {
		t.Fatalf("schema %q, want %s: %s", rec.Schema, perfrec.SchemaV3, line)
	}
	if rec.Tool != "statusgen" || rec.V3.Kind != perfrec.ObservationProcess || rec.Boundary != perfrec.BoundaryProcess {
		t.Errorf("tool/kind/boundary = %s/%s/%s: %s", rec.Tool, rec.V3.Kind, rec.Boundary, line)
	}
	if rec.PhaseOverlap {
		t.Errorf("phases overlap — exclusive phases must never double count: %s", line)
	}
	return rec
}

func phaseNames(rec perfrec.Record) map[string]perfrec.PhaseStat {
	m := map[string]perfrec.PhaseStat{}
	for _, p := range rec.Phases {
		m[p.Name] = p
	}
	return m
}

// Fixture IDs: derived at test time per the driver's ruling; avoids a secret-shaped literal.
var (
	perfSession1 = perfFixtureID("perf-session-1")
	perfAgent1   = perfFixtureID("perf-agent-1")
	perfParent1  = perfFixtureID("perf-parent-1")
)

// perfFixtureID is the first 32 hex digits of a label's sha256: a valid perf-v3 opaque ID.
func perfFixtureID(label string) string {
	sum := sha256.Sum256([]byte(label))
	return hex.EncodeToString(sum[:16])
}

const (
	perfBriefRef = `{"kind":"brief","repo":"medici-finance/assay","brief":"assay:assay:statusgen:26","primary":true}`
	perfIssueRef = `{"kind":"issue","forge":"github","instance":"github.com","repo":"medici-finance/assay","issue":3745}`
)

// perfWorkflowEnv is a complete, valid workflow context in the environment.
func perfWorkflowEnv() []string {
	return []string{"ASSAY_TELEMETRY=1", "ASSAY_PERF_PROFILE=workflow",
		"ASSAY_PERF_SESSION_ID=" + perfSession1, "ASSAY_PERF_AGENT_ID=" + perfAgent1,
		"ASSAY_PERF_PARENT_AGENT_ID=" + perfParent1, "DESK_LOOP=worker-desk", "ASSAY_PERF_ROLE=implement",
		"ASSAY_PERF_WORK=[" + perfBriefRef + "," + perfIssueRef + "]"}
}

// perfModes are the four offline modes the brief names, with the phases each must show.
var perfModes = []struct {
	name, verb string
	args       []string
	phases     []string
}{
	{"lint", "lint", []string{"--lint", "--root", "."}, []string{"discover", "parse", "evaluate", "render"}},
	{"regen", "regen", []string{"--root", "."}, []string{"discover", "parse", "evaluate", "render", "write"}},
	{"gate-scores", "gate-scores", []string{"--gate-scores", "--root", "."}, []string{"discover", "parse", "evaluate", "render"}},
	{"next-up", "next-up", []string{"--next-up", "--root", "."}, []string{"discover", "parse", "evaluate", "render"}},
}

// TestSharedPhaseRecorderFlow (statusgen/26 Verify row 1): the compiled binary in its four
// offline modes and through the fake deskread child keeps output and exit with the switch off,
// on with the tool profile and on with the workflow profile; every record is a perf-v3 line
// the shared reader (the decoder and reducer deskperf report uses) accepts; flags override and
// clear the environment; the child receives this invocation as its parent; a mixed
// v1/v2/v3 day file reduces; and a copied writer's line fails parity with the shared one.
//
// regression: statusgen/26
func TestSharedPhaseRecorderFlow(t *testing.T) {
	l := newPerfLab(t)
	seen := map[string]bool{}
	var parentLine, offLine string

	// 1. Four offline modes: identical output/exit off, tool, workflow; correct phases.
	for _, m := range perfModes {
		t.Run(m.name, func(t *testing.T) {
			// DESK_LOOP is operational (next-up reads role claims under it), so every run
			// carries it; only the perf switch and context differ between the three.
			before := len(l.lines(t))
			off := l.run(t, []string{"DESK_LOOP=worker-desk"}, m.args...)
			if n := len(l.lines(t)); n != before {
				t.Fatalf("switch off wrote %d records", n-before)
			}
			tool, toolLine := l.one(t, []string{"ASSAY_TELEMETRY=1", "ASSAY_PERF_PROFILE=tool", "DESK_LOOP=worker-desk"}, m.args...)
			wf, wfLine := l.one(t, perfWorkflowEnv(), m.args...)
			for name, got := range map[string]perfRun{"tool": tool, "workflow": wf} {
				if got.out != off.out || got.code != off.code {
					t.Errorf("%s profile changed the result:\n on  %d %q\n off %d %q", name, got.code, got.out, off.code, off.out)
				}
			}
			for _, line := range []string{toolLine, wfLine} {
				rec := decodeV3(t, line)
				if rec.Verb != m.verb || rec.Exit != off.code {
					t.Errorf("verb/exit = %s/%d, want %s/%d", rec.Verb, rec.Exit, m.verb, off.code)
				}
				ph := phaseNames(rec)
				for _, want := range m.phases {
					if ph[want].N == 0 {
						t.Errorf("%s: phase %q not recorded: %v", m.name, want, rec.Phases)
					}
				}
				for name := range ph {
					if !slices.Contains(perfV3Phases, name) {
						t.Errorf("phase %q is not on statusgen's roster", name)
					}
				}
				if _, ok := ph["deskread"]; ok {
					t.Errorf("an offline mode started a deskread child: %v", rec.Phases)
				}
				id := rec.V3.Attribution.InvocationID
				if seen[id] {
					t.Errorf("invocation id %s reused — every process needs a fresh identity", id)
				}
				seen[id] = true
			}
			tr := decodeV3(t, toolLine)
			if tr.V3.Profile != perfrec.ProfileTool || tr.Role != "none" {
				t.Errorf("tool profile record: profile %s role %s", tr.V3.Profile, tr.Role)
			}
			w := decodeV3(t, wfLine)
			d := w.V3.Attribution
			if w.V3.Profile != perfrec.ProfileWorkflow || d.SessionID != perfSession1 || d.AgentID != perfAgent1 ||
				d.ParentAgentID != perfParent1 || d.DeskRole != "worker-desk" || d.Role != "implement" || w.Role != "implement" {
				t.Errorf("workflow attribution not carried: %+v (role %s)", d, w.Role)
			}
			if len(d.Work) != 2 || d.Work[0].Brief != "assay:assay:statusgen:26" || d.Work[1].Issue != 3745 ||
				d.Coverage[perfrec.FieldWork] != perfrec.CoverageMeasured {
				t.Errorf("two qualified work refs not carried: %+v %v", d.Work, d.Coverage)
			}
			if w.V3.RequestCoverage != perfrec.CoverageMissing {
				t.Errorf("request coverage %s: statusgen observes no request itself, so it must read missing", w.V3.RequestCoverage)
			}
		})
	}

	// 2. Flags bound through the real parser override and clear the environment.
	t.Run("override", func(t *testing.T) {
		args := []string{"--gate-scores", "--root", ".", "--perf-role=review", "--perf-work=", "--perf-session-id", perfParent1}
		_, line := l.one(t, perfWorkflowEnv(), args...)
		d := decodeV3(t, line).V3.Attribution
		if d.Role != "review" || d.SessionID != perfParent1 {
			t.Errorf("flags did not override the environment: %+v", d)
		}
		if len(d.Work) != 0 || d.Coverage[perfrec.FieldWork] != perfrec.CoverageMissing {
			t.Errorf("--perf-work= did not clear the inherited work: %+v %v", d.Work, d.Coverage)
		}
		_, line = l.one(t, perfWorkflowEnv(), "--gate-scores", "--root", ".", "--perf-work", perfIssueRef)
		if d := decodeV3(t, line).V3.Attribution; len(d.Work) != 1 || d.Work[0].Kind != "issue" {
			t.Errorf("a --perf-work flag must replace, not extend, the inherited work: %+v", d.Work)
		}
		_, line = l.one(t, perfWorkflowEnv(), "--gate-scores", "--root", ".", "--perf-profile=tool")
		if rec := decodeV3(t, line); rec.V3.Profile != perfrec.ProfileTool {
			t.Errorf("--perf-profile=tool did not override the environment's workflow: %s", line)
		}
		_, offLine = l.one(t, perfWorkflowEnv(), "--gate-scores", "--root", ".", "--perf-profile=off")
		if rec, err := perfrec.Decode([]byte(offLine)); err != nil || rec.Schema != perfrec.SchemaV2 || rec.Verb != "gate-scores" {
			t.Errorf("--perf-profile=off must keep the perf-v2 line: %v %s", err, offLine)
		}
	})

	// 3. The fake deskread child: same output/exit, a timed child phase, and the causal link
	// handed on the child's own argv (--perf-* flags after the read kind).
	t.Run("deskread", func(t *testing.T) {
		args := []string{"--lint", "--forge", "--root", "."}
		off := l.run(t, []string{"DESK_LOOP=worker-desk"}, args...)
		offChildren := l.children(t)
		if len(offChildren) == 0 {
			t.Fatalf("--lint --forge did not start the fake deskread; exec log:\n%s", l.execLog(t))
		}
		for _, c := range offChildren {
			if delete(c.env, "DESK_LOOP"); len(c.env) != 0 || len(c.perfArgs()) != 0 {
				t.Errorf("switch off: the child was handed perf context %v %v", c.env, c.perfArgs())
			}
		}
		on, line := l.one(t, perfWorkflowEnv(), args...)
		if on.out != off.out || on.code != off.code {
			t.Errorf("recording changed the --forge result:\n on  %d %q\n off %d %q", on.code, on.out, off.code, off.out)
		}
		parentLine = line
		if !strings.Contains(l.execLog(t), "deskread --help") {
			t.Errorf("the version gate never probed deskread; exec log:\n%s", l.execLog(t))
		}
		if _, err := os.Stat(l.helpEnv); err == nil {
			t.Errorf("the --help probe inherited the caller's environment; it must run with none")
		}
		parent := decodeV3(t, line)
		children := l.children(t)[len(offChildren):]
		if ph := phaseNames(parent)["deskread"]; ph.N != int64(len(children)) || len(children) == 0 {
			t.Errorf("deskread phase n = %d, child starts = %d", ph.N, len(children))
		}
		pd := parent.V3.Attribution
		for i, c := range children {
			if c.args[0] != offChildren[i%len(offChildren)].args[0] {
				t.Errorf("the read kind must stay first: %q", c.args)
			}
			// The child's own resolver (what deskread runs at start) sees the parent link and
			// mints a fresh invocation of its own.
			cc := c.resolve()
			cd := cc.Data()
			if cc.Profile() != perfrec.ProfileWorkflow || cd.ParentInvocationID != pd.InvocationID || cd.SessionID != pd.SessionID ||
				cd.AgentID != pd.AgentID || cd.Role != "implement" || cc.InvocationID() == pd.InvocationID || len(cd.Work) != 2 {
				t.Errorf("child context does not link to its parent %s: %+v (argv %q)", pd.InvocationID, cd, c.args)
			}
		}
		// Tool profile: the child gets the profile and nothing else, and resolves no workflow field.
		start := len(l.children(t))
		_, toolLine := l.one(t, append(perfWorkflowEnv(), "ASSAY_PERF_PROFILE=tool"), args...)
		decodeV3(t, toolLine)
		// A parent whose flag cleared the profile tells its child "off", whatever it inherited.
		_, clearedLine := l.one(t, perfWorkflowEnv(), append(args, "--perf-profile=off")...)
		if rec, err := perfrec.Decode([]byte(clearedLine)); err != nil || rec.Schema != perfrec.SchemaV2 {
			t.Errorf("--perf-profile=off: %v %s", err, clearedLine)
		}
		after := l.children(t)[start:]
		if len(after) != 2*len(offChildren) {
			t.Fatalf("child starts = %d, want %d", len(after), 2*len(offChildren))
		}
		for i, c := range after {
			want, profile := "--perf-profile=tool", perfrec.ProfileTool
			if i >= len(offChildren) {
				want, profile = "--perf-profile=off", ""
			}
			cc := c.resolve()
			if got := c.perfArgs(); len(got) != 1 || got[0] != want || cc.Profile() != profile || cc.Data().SessionID != "" || len(cc.Data().Work) != 0 {
				t.Errorf("child handed %q resolved %s %+v, want only %s", got, cc.Profile(), cc.Data(), want)
			}
		}
	})

	// 3b. Version gate: a deskread that predates the --perf-* flags (its help lists none, and
	// it refuses an unknown flag) is handed none, so the result is the switch-off result.
	t.Run("old deskread", func(t *testing.T) {
		if err := os.WriteFile(l.old, nil, 0o600); err != nil {
			t.Fatal(err)
		}
		defer os.Remove(l.old)
		args := []string{"--lint", "--forge", "--root", "."}
		start := len(l.children(t))
		off := l.run(t, []string{"DESK_LOOP=worker-desk"}, args...)
		envs := map[string][]string{"tool": append(perfWorkflowEnv(), "ASSAY_PERF_PROFILE=tool"), "workflow": perfWorkflowEnv()}
		for name, env := range envs {
			got, line := l.one(t, env, args...)
			if got.out != off.out || got.code != off.code || strings.Contains(got.errOut, "unknown flag") {
				t.Errorf("%s profile with an old deskread changed the result:\n on  %d %q %q\n off %d %q", name, got.code, got.out, got.errOut, off.code, off.out)
			}
			if ph := phaseNames(decodeV3(t, line))["deskread"]; ph.N == 0 {
				t.Errorf("%s: the deskread wait is still a phase with an old child: %v", name, line)
			}
		}
		for _, c := range l.children(t)[start:] {
			if got := c.perfArgs(); len(got) != 0 {
				t.Errorf("an old deskread was handed %q", got)
			}
		}
	})

	// 4. Mixed v1/v2/v3 in one day file: the shared reader decodes all and the v3 reducer
	// links parent and child, deduplicating a repeated line.
	t.Run("mixed", func(t *testing.T) {
		if parentLine == "" || offLine == "" {
			t.Skip("earlier subtests failed")
		}
		parent := decodeV3(t, parentLine)
		v1 := perfrec.New("deskboot", "run", "none", time.Millisecond, 0, "unpinned", time.Now())
		v1.Install = parent.Install
		v1Line, err := json.Marshal(v1)
		if err != nil {
			t.Fatal(err)
		}
		v2s := perfrec.Begin(perfrec.SystemProbe(), perfrec.SystemProbe().Mark(), nil)
		v2 := v2s.Finish("deskdispatch", "run", "none", 0, "unpinned")
		v2.Install = parent.Install
		v2Line, err := json.Marshal(v2)
		if err != nil {
			t.Fatal(err)
		}
		child := perfrec.BeginSelected(perfrec.SystemProbe(), perfrec.SystemProbe().Mark(), nil,
			perfrec.ResolveContext(perfrec.ContextInput{LookupEnv: func(k string) (string, bool) {
				env := map[string]string{perfrec.EnvVar: "1", "ASSAY_PERF_PROFILE": "workflow",
					"ASSAY_PERF_SESSION_ID": parent.V3.Attribution.SessionID, "ASSAY_PERF_AGENT_ID": parent.V3.Attribution.AgentID,
					"ASSAY_PERF_PARENT_INVOCATION_ID": parent.V3.Attribution.InvocationID, "ASSAY_PERF_ROLE": "implement"}
				v, ok := env[k]
				return v, ok
			}})).Finish("deskread", "issues", "implement", 6, "unpinned")
		child.Install = parent.Install
		childLine, err := json.Marshal(child)
		if err != nil {
			t.Fatal(err)
		}
		var recs []perfrec.Record
		schemas := map[string]int{}
		for _, line := range []string{string(v1Line), offLine, string(v2Line), parentLine, string(childLine), parentLine} {
			r, err := perfrec.Decode([]byte(line))
			if err != nil {
				t.Fatalf("mixed day file: %v: %s", err, line)
			}
			schemas[r.Schema]++
			recs = append(recs, r)
		}
		if schemas[perfrec.Schema] != 1 || schemas[perfrec.SchemaV2] != 2 || schemas[perfrec.SchemaV3] != 3 {
			t.Errorf("schemas = %v", schemas)
		}
		sum, err := perfrec.ReduceV3(recs, perfrec.ProfileWorkflow, "session", perfrec.V3Filter{})
		if err != nil {
			t.Fatal(err)
		}
		if sum.Records != 2 || sum.Duplicates != 1 || sum.Conflicts != 0 || len(sum.Groups) != 1 || sum.Groups[0].Records != 2 {
			t.Errorf("v3 reduction: %+v", sum)
		}
		if _, err := perfrec.ReduceV3(recs, perfrec.ProfileTool, "session", perfrec.V3Filter{}); err == nil {
			t.Error("a tool-permission reader grouped by session — workflow fields leaked past the profile")
		}
	})

	// 5. Copied writer: a statusgen-local copy of the v1 writer shape, or one that flattens the
	// attribution, is refused by the shared reader, while the production line round-trips byte
	// for byte; and perfrecord.go itself opens, encodes or writes nothing.
	t.Run("copied writer", func(t *testing.T) {
		if parentLine == "" {
			t.Skip("earlier subtests failed")
		}
		rec := decodeV3(t, parentLine)
		again, err := json.Marshal(rec)
		if err != nil || string(again) != parentLine {
			t.Errorf("production line does not round-trip through the shared writer:\n got  %s\n want %s", again, parentLine)
		}
		var fields map[string]any
		if err := json.Unmarshal([]byte(parentLine), &fields); err != nil {
			t.Fatal(err)
		}
		copies := map[string]func(map[string]any){
			"v1 counters kept": func(m map[string]any) { m["childProcs"], m["childWallMs"], m["forgeCalls"] = 0, 0, 0 },
			"flat attribution": func(m map[string]any) { v3 := m["v3"].(map[string]any); m["attribution"] = v3["attribution"] },
			"no coverage": func(m map[string]any) {
				delete(m["v3"].(map[string]any)["attribution"].(map[string]any), "coverage")
			},
		}
		for name, mutate := range copies {
			var m map[string]any
			_ = json.Unmarshal([]byte(parentLine), &m)
			mutate(m)
			b, _ := json.Marshal(m)
			if _, err := perfrec.Decode(b); err == nil {
				t.Errorf("copied writer (%s) passed the shared reader: %s", name, b)
			}
		}
		if hits := perfWriterCalls(t, "perfrecord.go", ""); len(hits) != 0 {
			t.Errorf("perfrecord.go writes records itself (%v) — perfrec.Append is the one writer", hits)
		}
		src, _ := os.ReadFile("perfrecord.go")
		if hits := perfWriterCalls(t, "", string(src)+"\nfunc copied() { f, _ := os.OpenFile(\"x\", 0, 0); _ = json.NewEncoder(f) }\n"); len(hits) != 2 {
			t.Errorf("negative control: a planted copied writer gave %v, want 2 hits", hits)
		}
	})
}

// perfWriterCalls lists the file-writing and encoding calls in one Go source (by path, or
// the src text when path is empty).
func perfWriterCalls(t *testing.T, path, src string) []string {
	t.Helper()
	var text any
	if path == "" {
		text = src
	}
	f, err := parser.ParseFile(token.NewFileSet(), path, text, 0)
	if err != nil {
		t.Fatal(err)
	}
	banned := map[string]bool{"os.OpenFile": true, "os.Create": true, "os.WriteFile": true, "json.Marshal": true,
		"json.NewEncoder": true, "os.MkdirAll": true}
	var hits []string
	ast.Inspect(f, func(n ast.Node) bool {
		if s, ok := n.(*ast.SelectorExpr); ok {
			if id, ok := s.X.(*ast.Ident); ok && banned[id.Name+"."+s.Sel.Name] {
				hits = append(hits, id.Name+"."+s.Sel.Name)
			}
		}
		return true
	})
	return hits
}

// perfForbiddenDeps are the import-path fragments statusgen's dependency closure must never
// contain: the desk tools, their kit, the forge adapters (which hold credentials) and any
// custody or token-minting code.
var perfForbiddenDeps = []string{"/tools/desk", "deskkit", "forgeread/adapters", "custody", "desktoken"}

func perfForbiddenIn(deps string) []string {
	var hits []string
	for _, p := range strings.Fields(deps) {
		for _, bad := range perfForbiddenDeps {
			if strings.Contains(p, bad) {
				hits = append(hits, p)
			}
		}
	}
	return hits
}

// perfCredentialReads lists the credential-shaped reads in one Go source (by path, or src
// when path is empty): any direct environment lookup, and any string literal naming a token,
// secret, password, credential helper, `gh auth` or desktoken. Measurement code needs none.
func perfCredentialReads(t *testing.T, path, src string) []string {
	t.Helper()
	var text any
	if path == "" {
		text = src
	}
	f, err := parser.ParseFile(token.NewFileSet(), path, text, 0)
	if err != nil {
		t.Fatal(err)
	}
	lit := regexp.MustCompile(`(?i)token|secret|password|credential|gh auth|desktoken|keychain`)
	var hits []string
	ast.Inspect(f, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.SelectorExpr:
			if id, ok := n.X.(*ast.Ident); ok && id.Name == "os" && (n.Sel.Name == "Getenv" || n.Sel.Name == "LookupEnv") {
				hits = append(hits, "os."+n.Sel.Name)
			}
		case *ast.BasicLit:
			if n.Kind == token.STRING && lit.MatchString(n.Value) {
				hits = append(hits, n.Value)
			}
		}
		return true
	})
	return hits
}

// TestPerformanceKeepsCredentialBoundary (statusgen/26 Verify row 3): measurement adds no
// credential path. The dependency closure has no desk, custody or adapter import (a planted
// one is caught); with the switch off nothing is created whatever profile context is set;
// the tool profile never records or forwards inherited workflow fields; malformed optional
// context never changes output or exit; offline modes reach no forge tool or remote git and
// --forge reaches only deskread; and the recorder reads no credential (a planted getter is
// caught).
//
// regression: statusgen/26
func TestPerformanceKeepsCredentialBoundary(t *testing.T) {
	t.Run("dependency closure", func(t *testing.T) {
		if hits := perfForbiddenIn(goList(t, ".", "list", "-deps", ".")); len(hits) != 0 {
			t.Errorf("statusgen depends on credential-holding packages: %v", hits)
		}
		// Negative control: the same check sees a planted adapter import through an overlay
		// (relative key, so the overlay names no machine path).
		dir := t.TempDir()
		src := filepath.Join(dir, "planted.go")
		if err := os.WriteFile(src, []byte("package main\n\nimport _ \"github.com/medici-finance/assay/forgeread/adapters/github\"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		ov, _ := json.Marshal(map[string]any{"Replace": map[string]string{"zz_perf_planted.go": src}})
		ovPath := filepath.Join(dir, "overlay.json")
		if err := os.WriteFile(ovPath, ov, 0o600); err != nil {
			t.Fatal(err)
		}
		if hits := perfForbiddenIn(goList(t, ".", "list", "-e", "-deps", "-overlay", ovPath, ".")); len(hits) == 0 {
			t.Error("negative control: a planted adapter import was not seen in the closure")
		}
	})

	l := newPerfLab(t)
	modes := append(perfModes[:len(perfModes):len(perfModes)], struct {
		name, verb string
		args       []string
		phases     []string
	}{"forge", "lint", []string{"--lint", "--forge", "--root", "."}, nil})

	t.Run("off creates nothing", func(t *testing.T) {
		env := append(perfWorkflowEnv()[1:], "ASSAY_PERF_PROFILE=workflow")
		for _, m := range modes {
			l.run(t, env, append(m.args, "--perf-profile=workflow", "--perf-role=review", "--perf-work", perfIssueRef)...)
		}
		if _, err := os.Stat(filepath.Join(l.home, ".config", "assay", "perf")); !os.IsNotExist(err) {
			t.Errorf("switch off but profile context set: the perf directory exists (%v)", err)
		}
	})

	t.Run("tool profile is workflow-blind", func(t *testing.T) {
		start := len(l.children(t))
		env := append(perfWorkflowEnv(), "ASSAY_PERF_PROFILE=tool")
		for _, m := range modes {
			_, line := l.one(t, env, append(m.args, "--perf-role=review", "--perf-work", perfIssueRef, "--perf-session-id", perfParent1)...)
			rec := decodeV3(t, line)
			d := rec.V3.Attribution
			if rec.V3.Profile != perfrec.ProfileTool || rec.Role != perfrec.RoleNone || d.SessionID != "" || d.AgentID != "" ||
				d.ParentAgentID != "" || d.ParentInvocationID != "" || d.DeskRole != "" || d.Role != "" || len(d.Work) != 0 {
				t.Errorf("%s: tool profile recorded workflow fields: %+v role %s", m.name, d, rec.Role)
			}
			for _, v := range []string{perfSession1, perfAgent1, perfParent1, "worker-desk", "implement", "review", "3745", "statusgen:26"} {
				if strings.Contains(line, v) {
					t.Errorf("%s: tool record carries inherited workflow value %q: %s", m.name, v, line)
				}
			}
		}
		children := l.children(t)[start:]
		if len(children) == 0 {
			t.Fatal("--forge started no deskread child")
		}
		for _, c := range children {
			cc := c.resolve()
			d := cc.Data()
			if got := c.perfArgs(); len(got) != 1 || got[0] != "--perf-profile=tool" || cc.Profile() != perfrec.ProfileTool ||
				d.SessionID != "" || d.AgentID != "" || d.ParentInvocationID != "" || d.Role != "" || len(d.Work) != 0 {
				t.Errorf("tool profile forwarded workflow context to its child: %q -> %+v", got, d)
			}
		}
	})

	t.Run("malformed context is inert", func(t *testing.T) {
		bad := [][]string{
			{"ASSAY_PERF_WORK=not json"},
			{"ASSAY_PERF_WORK=[{\"kind\":\"brief\",\"repo\":\"/abs\",\"brief\":\"x\"}]"},
			{"ASSAY_PERF_SESSION_ID=XYZ", "ASSAY_PERF_AGENT_ID=short"},
			{"ASSAY_PERF_PARENT_INVOCATION_ID=../../etc"},
			{"ASSAY_PERF_PROFILE=everything"},
			{"ASSAY_PERF_ROLE=Bad Role", "DESK_LOOP=not-a-desk"},
		}
		for _, m := range modes {
			for _, b := range bad {
				// DESK_LOOP is operational, so the switch-off baseline carries the same loop.
				loop := "DESK_LOOP=worker-desk"
				for _, kv := range b {
					if strings.HasPrefix(kv, "DESK_LOOP=") {
						loop = kv
					}
				}
				off := l.run(t, []string{loop}, m.args...)
				got := l.run(t, append(append(perfWorkflowEnv(), loop), b...), m.args...)
				if got.out != off.out || got.code != off.code {
					t.Errorf("%s %v changed the result:\n got %d %q\n off %d %q", m.name, b, got.code, got.out, off.code, off.out)
				}
			}
			off := l.run(t, []string{"DESK_LOOP=worker-desk"}, m.args...)
			bogus := l.run(t, []string{"DESK_LOOP=worker-desk", "ASSAY_TELEMETRY=1"}, append(m.args, "--perf-work=nope", "--perf-session-id=bad", "--perf-profile=loud")...)
			if bogus.out != off.out || bogus.code != off.code {
				t.Errorf("%s: malformed perf flags changed the result:\n got %d %q\n off %d %q", m.name, bogus.code, bogus.out, off.code, off.out)
			}
		}
	})

	// Recording adds no route: per mode, the commands a recording run starts are exactly
	// the ones the switch-off run starts; offline modes start no forge tool and no remote git
	// (next-up's pre-existing claim-ref read is its only remote git, unchanged by the
	// switch); --forge reaches the network only through deskread.
	t.Run("deskread is the sole online route", func(t *testing.T) {
		calls := func(env []string, args []string) []string {
			_ = os.Remove(l.log)
			l.run(t, env, args...)
			var out []string
			for _, line := range strings.Split(strings.TrimSpace(l.execLog(t)), "\n") {
				if line != "" {
					out = append(out, line)
				}
			}
			sort.Strings(out)
			return out
		}
		for _, m := range modes {
			off := calls([]string{"DESK_LOOP=worker-desk"}, m.args)
			on := calls(perfWorkflowEnv(), m.args)
			// The hand-off adds only the version gate's offline probe (once, and only where a
			// deskread child starts) and --perf-* words on the child's own argv.
			probes := 0
			var norm []string
			for _, line := range on {
				if line == "deskread --help" {
					probes++
					continue
				}
				if strings.HasPrefix(line, "deskread ") {
					line = strings.Join(slices.DeleteFunc(strings.Fields(line), func(f string) bool { return strings.HasPrefix(f, "--perf-") }), " ")
				}
				norm = append(norm, line)
			}
			sort.Strings(norm)
			if want := map[bool]int{true: 1, false: 0}[m.name == "forge"]; probes != want {
				t.Errorf("%s: %d version-gate probes, want %d", m.name, probes, want)
			}
			if !slices.Equal(off, norm) {
				t.Errorf("%s: recording changed what the process starts:\n on  %q\n off %q", m.name, on, off)
			}
			for _, line := range on {
				fields := strings.Fields(line)
				switch {
				case fields[0] == "deskread" && m.name == "forge":
				case fields[0] == "git":
					for _, f := range fields[1:] {
						if gitRemoteSubcmds[f] && !(m.name == "next-up" && f == "ls-remote") {
							t.Errorf("%s: remote git call: %s", m.name, line)
						}
					}
				default:
					t.Errorf("%s: online tool reached: %s", m.name, line)
				}
			}
		}
	})

	t.Run("no credential getter", func(t *testing.T) {
		if hits := perfCredentialReads(t, "perfrecord.go", ""); len(hits) != 0 {
			t.Errorf("perfrecord.go reads credential-shaped values: %v", hits)
		}
		src, err := os.ReadFile("perfrecord.go")
		if err != nil {
			t.Fatal(err)
		}
		planted := string(src) + "\nfunc perfPlanted() string { return os.Getenv(\"GH_TOKEN\") }\n"
		if hits := perfCredentialReads(t, "", planted); len(hits) != 2 {
			t.Errorf("negative control: a planted credential getter gave %v, want the call and the literal", hits)
		}
	})
}
