package perfrec

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"
)

var testNow = time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC)

func newWriter() *Writer { return &Writer{Now: func() time.Time { return testNow }} }

func dayName(daysAgo int) string {
	return testNow.AddDate(0, 0, -daysAgo).Format("2006-01-02") + ".jsonl"
}

func sample() Record {
	return New("tool", "verb", RoleNone, 12*time.Millisecond, 0, "abcdef1", testNow)
}

func TestEnabledIsExactlyOne(t *testing.T) {
	for val, want := range map[string]bool{"1": true, "": false, "0": false, "true": false, "yes": false, " 1": false, "11": false} {
		t.Setenv(EnvVar, val)
		if got := Enabled(); got != want {
			t.Errorf("%s=%q: Enabled() = %v, want %v", EnvVar, val, got, want)
		}
	}
}

// TestAppendOffCreatesNothing: the package-level Append with the switch off creates no
// directory, day file or install id under the home it would resolve.
func TestAppendOffCreatesNothing(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	for _, v := range []string{"", "0", "true"} {
		t.Setenv(EnvVar, v)
		Append(sample())
	}
	if entries, _ := os.ReadDir(home); len(entries) != 0 {
		t.Fatalf("switch off, but the home gained %v", entries)
	}
	t.Setenv(EnvVar, "1")
	Append(sample())
	got, _ := filepath.Glob(filepath.Join(home, ".config", "assay", "perf", "*.jsonl"))
	if len(got) != 1 {
		t.Fatalf("switch on wrote %v, want one day file", got)
	}
}

func TestRecordGoldenShape(t *testing.T) {
	state := t.TempDir()
	w := newWriter()
	if err := w.Write(state, sample()); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(Dir(state), dayName(0)))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(raw), "\n") != 1 || !strings.HasSuffix(string(raw), "\n") {
		t.Fatalf("want exactly one newline-terminated line, got %q", raw)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	var keys []string
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	want := "childProcs,childWallMs,exit,forgeCalls,install,result,role,schema,sourceSHA,tool,ts,verb,wallMs"
	if strings.Join(keys, ",") != want {
		t.Errorf("key set drifted:\n got  %v\n want %s", keys, want)
	}
	for f, v := range map[string]any{"schema": "perf-v1", "ts": "2026-10-13T12:00:00Z", "tool": "tool", "verb": "verb",
		"role": "none", "wallMs": float64(12), "exit": float64(0), "result": "ok", "sourceSHA": "abcdef1"} {
		if m[f] != v {
			t.Errorf("%s = %v, want %v", f, m[f], v)
		}
	}
	if !InstallIDPattern.MatchString(m["install"].(string)) {
		t.Errorf("install %q is not 32 hex", m["install"])
	}

	// cpuMs is OMITTED when unmeasured and present (even as 0) when measured.
	if b, _ := json.Marshal(Record{}); strings.Contains(string(b), "cpuMs") {
		t.Errorf("an unmeasured record carries cpuMs: %s", b)
	}
	zero := int64(0)
	if b, _ := json.Marshal(Record{CPUMs: &zero}); !strings.Contains(string(b), `"cpuMs":0`) {
		t.Errorf("a measured zero lost its cpuMs: %s", b)
	}
	if ms, ok := CPUMillis(); runtime.GOOS != "windows" && (!ok || ms < 0) {
		t.Errorf("CPUMillis() = %d, %v on %s, want a measurement", ms, ok, runtime.GOOS)
	}
}

func TestMappersReturnConstants(t *testing.T) {
	results := map[string]bool{ResultOK: true, ResultDisabled: true, ResultRefused: true, ResultUnverifiable: true}
	for code := -1; code <= 300; code++ {
		if r := ResultForExit(code); !results[r] {
			t.Fatalf("ResultForExit(%d) = %q — not one of its own constants", code, r)
		}
	}
	if ResultForExit(0) != "ok" || ResultForExit(3) != "disabled" || ResultForExit(5) != "refused" || ResultForExit(6) != "unverifiable" {
		t.Error("the exit→result map drifted")
	}
	roster := []string{"create", "", "update"}
	for args, want := range map[string]string{"create": "create", "update": "update", "other-thing": Other, "": Other} {
		var a []string
		if args != "" {
			a = []string{args, "secret-arg"}
		}
		if got := Verb(roster, a); got != want {
			t.Errorf("Verb(%q) = %q, want %q", args, got, want)
		}
	}
	if Verb(roster, nil) != Other {
		t.Error("no args must map to other")
	}
	if SourceSHA("v1.2.3-secret") != Unpinned || SourceSHA("dev") != Unpinned || SourceSHA("0123abc") != "0123abc" {
		t.Error("SourceSHA must pass hex stamps and nothing else")
	}
}

func TestPruneKeepsSevenDaysAndOnlyDayFiles(t *testing.T) {
	state := t.TempDir()
	dir := Dir(state)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	keep := []string{dayName(0), dayName(6), "install-id", "notes.txt", dayName(9) + ".bak", "x" + dayName(9)}
	drop := []string{dayName(7), dayName(8), dayName(30)}
	for _, n := range append(append([]string{}, keep...), drop...) {
		if err := os.WriteFile(filepath.Join(dir, n), []byte("x\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(dir, dayName(20)), 0o700); err != nil { // a directory with a day name
		t.Fatal(err)
	}
	if err := newWriter().Write(state, sample()); err != nil {
		t.Fatal(err)
	}
	for _, n := range keep {
		if _, err := os.Stat(filepath.Join(dir, n)); err != nil {
			t.Errorf("%s must survive the prune: %v", n, err)
		}
	}
	for _, n := range drop {
		if _, err := os.Stat(filepath.Join(dir, n)); !os.IsNotExist(err) {
			t.Errorf("%s is older than the 7-day window and must be pruned (err=%v)", n, err)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, dayName(20))); err != nil {
		t.Errorf("a directory named like a day file is not a regular file and must survive: %v", err)
	}
	if got := Cutoff(testNow); got != testNow.AddDate(0, 0, -6).Truncate(24*time.Hour) {
		t.Errorf("Cutoff = %v", got)
	}
}

func TestPruneRunsOncePerDirectory(t *testing.T) {
	state := t.TempDir()
	w := newWriter()
	if err := w.Write(state, sample()); err != nil {
		t.Fatal(err)
	}
	old := filepath.Join(Dir(state), dayName(30))
	if err := os.WriteFile(old, []byte("x\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := w.Write(state, sample()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(old); err != nil {
		t.Errorf("the second write in a process must not prune again: %v", err)
	}
	w.Reset()
	if err := w.Write(state, sample()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Errorf("after Reset the next write prunes (err=%v)", err)
	}
}

func TestInstallIDStableAndReplacedWhenMalformed(t *testing.T) {
	dir := t.TempDir()
	a, err := InstallID(dir)
	if err != nil || !InstallIDPattern.MatchString(a) {
		t.Fatalf("InstallID = %q, %v", a, err)
	}
	if b, _ := InstallID(dir); b != a {
		t.Errorf("install id changed between calls: %q then %q", a, b)
	}
	if err := os.WriteFile(filepath.Join(dir, "install-id"), []byte("not-hex\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	c, err := InstallID(dir)
	if err != nil || !InstallIDPattern.MatchString(c) || c == "not-hex" {
		t.Errorf("a malformed id must be replaced: %q, %v", c, err)
	}
	if st, err := os.Stat(filepath.Join(dir, "install-id")); err != nil || st.Mode().Perm() != 0o600 && runtime.GOOS != "windows" {
		t.Errorf("install-id mode/err: %v %v", st, err)
	}
}

func TestWriteFailureIsSwallowed(t *testing.T) {
	state := t.TempDir()
	// perf exists as a FILE, so the directory cannot be created.
	if err := os.WriteFile(filepath.Join(state, "perf"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	w := newWriter()
	if err := w.Write(state, sample()); err == nil {
		t.Error("Write into an unusable directory must report the error to tests")
	}
	w.Append(state, sample()) // must not panic or return anything
}

// TestNeverTouchesTheAuditLedger: no string literal in the package names the audit ledger.
func TestNeverTouchesTheAuditLedger(t *testing.T) {
	files, _ := filepath.Glob("*.go")
	for _, src := range files {
		if strings.HasSuffix(src, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(token.NewFileSet(), src, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(f, func(n ast.Node) bool {
			if lit, ok := n.(*ast.BasicLit); ok && lit.Kind == token.STRING && strings.Contains(strings.ToLower(lit.Value), "audit") {
				t.Errorf("%s: string literal %s names the audit ledger", src, lit.Value)
			}
			return true
		})
	}
}
