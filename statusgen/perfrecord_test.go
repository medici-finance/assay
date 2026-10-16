package main

// perfrecord_test.go — #3101: with ASSAY_TELEMETRY=1 statusgen writes the same local perf-v1
// record the desk tools write, through the shared perfrec module; with it unset it writes
// nothing and behaves exactly as before.

import (
	"bytes"
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"
)

type perfRun struct {
	out, errOut string
	code        int
}

// runPerfBinary runs the built statusgen with a throwaway HOME and the given ASSAY_TELEMETRY
// (set=false removes it), from a scratch working directory.
func runPerfBinary(t *testing.T, bin, home string, set bool, val string, args ...string) perfRun {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Dir = t.TempDir()
	var env []string
	for _, e := range os.Environ() {
		if strings.HasPrefix(e, "ASSAY_TELEMETRY=") || strings.HasPrefix(e, "HOME=") ||
			strings.HasPrefix(e, "USERPROFILE=") || strings.HasPrefix(e, "DESK_LOOP=") {
			continue
		}
		env = append(env, e)
	}
	env = append(env, "HOME="+home, "USERPROFILE="+home)
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

func buildPerfBinary(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go toolchain unavailable")
	}
	bin := filepath.Join(t.TempDir(), "statusgen")
	if out, err := exec.Command("go", "build", "-o", bin, ".").CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	return bin
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

// TestPerfOnWritesOneRecord: ASSAY_TELEMETRY=1 appends exactly one perf-v1 line per process,
// in the desk tools' schema, for success, a usage error, and a real mode; exit code and
// output are unchanged by recording.
//
// regression: #3101
func TestPerfOnWritesOneRecord(t *testing.T) {
	bin := buildPerfBinary(t)
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS("testdata/goodrepo")); err != nil {
		t.Fatal(err)
	}
	wantKeys := []string{"childProcs", "childWallMs", "exit", "forgeCalls", "install", "result", "role",
		"schema", "sourceSHA", "tool", "ts", "verb", "wallMs"}
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
	}{
		{"version", []string{"--version"}, "version", 0, "ok", true},
		{"mode flag", []string{"--alarms", "--root", root}, "alarms", 0, "ok", true},
		{"lint mode", []string{"--lint", "--root", root}, "lint", 0, "ok", true},
		{"usage error", []string{"--no-such-flag"}, "regen", 2, "unverifiable", true},
		{"unknown subcommand", []string{"frobnicate"}, "other", 2, "unverifiable", true},
	} {
		t.Run(c.name, func(t *testing.T) {
			home := t.TempDir()
			off := runPerfBinary(t, bin, home, false, "", c.args...)
			if recs := perfRecords(t, home); len(recs) != 0 {
				t.Fatalf("switch unset wrote %d records", len(recs))
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
				t.Errorf("perf-v1 key set drifted from the desk tools':\n got  %v\n want %v", keys, wantKeys)
			}
			for field, want := range map[string]any{"schema": "perf-v1", "tool": "statusgen", "verb": c.verb,
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
