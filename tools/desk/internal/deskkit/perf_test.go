package deskkit

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
)

// perfTestNow is the fixed clock every perf test runs on.
var perfTestNow = time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC)

// perfFixture points the state directory at a fresh temp dir, fixes the clock, arms the
// switch, clears DESK_LOOP, and resets the once-per-process prune memory. It returns the
// state dir (the perf dir is <state>/perf).
func perfFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	oldDir, oldNow := dirOverride, perfNow
	dirOverride = dir
	perfNow = func() time.Time { return perfTestNow }
	resetPerfPrune()
	t.Cleanup(func() {
		dirOverride, perfNow = oldDir, oldNow
		resetPerfPrune()
	})
	t.Setenv("ASSAY_TELEMETRY", "1")
	t.Setenv("DESK_LOOP", "")
	return dir
}

func resetPerfPrune() {
	perfPruneMu.Lock()
	perfPrunedDir = ""
	perfPruneMu.Unlock()
}

func perfDayName(daysAgo int) string {
	return perfTestNow.AddDate(0, 0, -daysAgo).Format("2006-01-02") + ".jsonl"
}

// perfLines returns today's day file's lines, each decoded as a generic object, plus the raw bytes.
func perfLines(t *testing.T, state string) ([]map[string]any, []byte) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(state, "perf", perfDayName(0)))
	if err != nil {
		t.Fatalf("read today's perf day file: %v", err)
	}
	var out []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		var m map[string]any
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			t.Fatalf("perf line is not JSON: %q: %v", line, err)
		}
		out = append(out, m)
	}
	return out, raw
}

func perfKeys(m map[string]any) []string {
	var ks []string
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}

// TestPerfRecordGoldenShape pins the perf-v1 key set exactly, and feeds each of the four
// mapped fields an UNREGISTERED input to prove the mapper answers with one of its own
// constants — never the input.
// regression: F-dt23-perf-record
func TestPerfRecordGoldenShape(t *testing.T) {
	state := perfFixture(t)
	t.Setenv("DESK_LOOP", "worker-desk")

	if rc := PerfRun("deskpr", []string{"create", "update"}, []string{"create", "--flag"}, func([]string) int { return 0 }); rc != 0 {
		t.Fatalf("PerfRun rc = %d, want 0", rc)
	}
	recs, _ := perfLines(t, state)
	if len(recs) != 1 {
		t.Fatalf("one wrapped run wrote %d records, want exactly 1", len(recs))
	}
	got := recs[0]
	want := []string{"childProcs", "childWallMs", "exit", "forgeCalls", "install", "result", "role",
		"schema", "sourceSHA", "tool", "ts", "verb", "wallMs"}
	if _, measurable := processCPUMillis(); measurable {
		want = append(want, "cpuMs")
	}
	sort.Strings(want)
	if k := perfKeys(got); strings.Join(k, ",") != strings.Join(want, ",") {
		t.Fatalf("perf-v1 key set drifted:\n got  %v\n want %v", k, want)
	}
	for field, val := range map[string]any{
		"schema": "perf-v1", "tool": "deskpr", "verb": "create", "role": "worker-desk", "result": "ok", "exit": float64(0),
	} {
		if got[field] != val {
			t.Errorf("%s = %v, want %v", field, got[field], val)
		}
	}
	if ts, _ := got["ts"].(string); !regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z$`).MatchString(ts) {
		t.Errorf("ts = %q, want RFC3339 UTC at second resolution", ts)
	}
	if id, _ := got["install"].(string); !installIDPattern.MatchString(id) {
		t.Errorf("install = %q, want 32 lowercase hex characters", id)
	}
	if sha, _ := got["sourceSHA"].(string); sha != perfUnpinned && !shaPattern.MatchString(sha) {
		t.Errorf("sourceSHA = %q, want a hex build stamp or %q", sha, perfUnpinned)
	}

	// cpuMs is OMITTED when unmeasurable, and present (even as 0) when measured.
	unmeasured, _ := json.Marshal(PerfRecord{})
	if bytes.Contains(unmeasured, []byte("cpuMs")) {
		t.Errorf("an unmeasured record carries cpuMs: %s — a zero would claim a measurement", unmeasured)
	}
	zero := int64(0)
	measured, _ := json.Marshal(PerfRecord{CPUMs: &zero})
	if !bytes.Contains(measured, []byte(`"cpuMs":0`)) {
		t.Errorf("a measured zero lost its cpuMs: %s", measured)
	}

	// The mappers, each fed an unregistered input.
	t.Setenv("DESK_LOOP", "not-a-loop")
	PerfRun("zz-unregistered-tool", []string{"create"}, []string{"zz-unknown-sub"}, func([]string) int { return 42 })
	PerfStep("deskboot", "zz-unknown-step", time.Millisecond, 0)
	recs, _ = perfLines(t, state)
	if len(recs) != 3 {
		t.Fatalf("want 3 records after the unregistered runs, got %d", len(recs))
	}
	odd := recs[1]
	for field, val := range map[string]string{"tool": "other", "verb": "other", "role": "none", "result": ResultUnverifiable} {
		if odd[field] != val {
			t.Errorf("unregistered input: %s = %v, want the constant %q", field, odd[field], val)
		}
	}
	if odd["exit"] != float64(42) {
		t.Errorf("exit = %v, want the verb's own 42", odd["exit"])
	}
	if recs[2]["verb"] != "step:other" {
		t.Errorf("an unknown boot step recorded verb %v, want step:other", recs[2]["verb"])
	}
	results := map[string]bool{ResultOK: true, ResultDisabled: true, ResultRefused: true, ResultUnverifiable: true}
	for code := -1; code <= 300; code++ {
		r := resultForExit(code)
		if !results[r] || r == strconv.Itoa(code) {
			t.Fatalf("resultForExit(%d) = %q — not one of its own constants", code, r)
		}
	}
}

// TestPerfOffByDefaultCreatesNothing: unset, "0", "true" and "" are all OFF, and off means no
// directory, no day file and no install id — while the wrapped exit code still comes back.
// regression: F-dt23-perf-record
func TestPerfOffByDefaultCreatesNothing(t *testing.T) {
	for _, tc := range []struct {
		name  string
		value string
		unset bool
	}{
		{"unset", "", true},
		{"zero", "0", false},
		{"true", "true", false},
		{"empty", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			state := perfFixture(t)
			t.Setenv("ASSAY_TELEMETRY", tc.value)
			if tc.unset {
				if err := os.Unsetenv("ASSAY_TELEMETRY"); err != nil {
					t.Fatal(err)
				}
			}
			if PerfEnabled() {
				t.Fatalf("PerfEnabled() = true for ASSAY_TELEMETRY %s", tc.name)
			}
			ran := false
			if rc := PerfRun("deskpr", []string{"create"}, []string{"create"}, func([]string) int { ran = true; return 5 }); rc != 5 || !ran {
				t.Fatalf("PerfRun rc = %d ran = %v, want the wrapped verb's 5", rc, ran)
			}
			PerfStep("deskboot", "token-mint", time.Second, 0)
			entries, err := os.ReadDir(state)
			if err != nil {
				t.Fatal(err)
			}
			if len(entries) != 0 {
				t.Fatalf("telemetry %s created %v in the state dir — off must create nothing", tc.name, entries)
			}
		})
	}
}

// TestPerfPruneKeepsSevenDaysAndOnlyDayFiles: 12 day files, install-id and decoys; one append
// leaves today plus the six prior days, removes the eighth day and older, and leaves every
// non-day-file name (including a day-named DIRECTORY and a day-prefixed decoy) untouched.
// regression: F-dt23-perf-record
func TestPerfPruneKeepsSevenDaysAndOnlyDayFiles(t *testing.T) {
	state := perfFixture(t)
	pdir := filepath.Join(state, "perf")
	if err := os.MkdirAll(pdir, 0o700); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 12; i++ {
		if err := os.WriteFile(filepath.Join(pdir, perfDayName(i)), []byte("{}\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	installID := strings.Repeat("ab", 16)
	decoys := map[string]string{
		"install-id":            installID + "\n",
		"notes.txt":             "keep me\n",
		"2026-09-01.jsonl.keep": "a day-PREFIXED name is not a day file\n",
	}
	for name, body := range decoys {
		if err := os.WriteFile(filepath.Join(pdir, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(pdir, "2026-09-02.jsonl"), 0o700); err != nil {
		t.Fatal(err)
	}

	PerfRun("deskpr", []string{"create"}, []string{"create"}, func([]string) int { return 0 })

	var days []string
	entries, err := os.ReadDir(pdir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if _, ok := perfDayOf(e.Name()); ok && e.Type().IsRegular() {
			days = append(days, e.Name())
		}
	}
	var want []string
	for i := 6; i >= 0; i-- {
		want = append(want, perfDayName(i))
	}
	if strings.Join(days, ",") != strings.Join(want, ",") {
		t.Fatalf("day files after one append:\n got  %v\n want %v (today and the six prior)", days, want)
	}
	if _, err := os.Stat(filepath.Join(pdir, perfDayName(7))); !os.IsNotExist(err) {
		t.Fatalf("the day-8 file survived the prune (stat err %v)", err)
	}
	for name, body := range decoys {
		got, err := os.ReadFile(filepath.Join(pdir, name))
		if err != nil || string(got) != body {
			t.Fatalf("non-day-file %q was touched: %q, %v", name, got, err)
		}
	}
	if st, err := os.Stat(filepath.Join(pdir, "2026-09-02.jsonl")); err != nil || !st.IsDir() {
		t.Fatalf("a day-named DIRECTORY was removed: %v", err)
	}
	recs, _ := perfLines(t, state)
	if len(recs) != 2 || recs[1]["install"] != installID {
		t.Fatalf("today's file should hold the seed line plus one record under the existing install id; got %v", recs)
	}
}

// captureStd runs fn with os.Stdout and os.Stderr redirected, returning what each received.
func captureStd(t *testing.T, fn func()) (stdout, stderr string) {
	t.Helper()
	oldOut, oldErr := os.Stdout, os.Stderr
	ro, wo, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	re, we, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout, os.Stderr = wo, we
	outC, errC := make(chan string), make(chan string)
	go func() { b, _ := io.ReadAll(ro); outC <- string(b) }()
	go func() { b, _ := io.ReadAll(re); errC <- string(b) }()
	func() {
		defer func() { os.Stdout, os.Stderr = oldOut, oldErr }()
		fn()
	}()
	if err := wo.Close(); err != nil {
		t.Fatal(err)
	}
	if err := we.Close(); err != nil {
		t.Fatal(err)
	}
	return <-outC, <-errC
}

// TestPerfWriteFailureNeverChangesExitCode is the NEGATIVE control: with the perf directory
// forced unwritable (a regular file squats on its path), every wrapped exit code comes back
// unchanged and the run prints exactly what it prints with perf disabled.
// regression: F-dt23-perf-record
func TestPerfWriteFailureNeverChangesExitCode(t *testing.T) {
	state := perfFixture(t)
	if err := os.WriteFile(filepath.Join(state, "perf"), []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := perfWrite(newPerfRecord("deskpr", "create", time.Millisecond, 0)); err == nil {
		t.Fatal("the forced failure did not happen: perfWrite succeeded with a file on the perf path")
	}
	for _, code := range []int{ExitOK, ExitDisabled, ExitRefused, ExitUnverifiable, 42} {
		verb := func([]string) int {
			fmt.Fprintln(os.Stdout, "verb stdout", code)
			fmt.Fprintln(os.Stderr, "verb stderr", code)
			return code
		}
		var armedRC, offRC int
		t.Setenv("ASSAY_TELEMETRY", "1")
		armedOut, armedErr := captureStd(t, func() { armedRC = PerfRun("deskpr", []string{"create"}, []string{"create"}, verb) })
		t.Setenv("ASSAY_TELEMETRY", "0")
		offOut, offErr := captureStd(t, func() { offRC = PerfRun("deskpr", []string{"create"}, []string{"create"}, verb) })
		if armedRC != code || offRC != code {
			t.Fatalf("exit %d: armed rc = %d, off rc = %d — a perf write failure changed the exit code", code, armedRC, offRC)
		}
		if armedOut != offOut || armedErr != offErr {
			t.Fatalf("exit %d: the failing perf write changed the output:\n armed %q / %q\n off   %q / %q", code, armedOut, armedErr, offOut, offErr)
		}
	}
}

// TestPerfNeverTouchesTheAuditLedger: with the switch armed, a wrapped run, a boot step, a
// prune and a read leave audit.jsonl AND its rotated day segment byte- and mtime-identical,
// and create nothing in the state dir but perf/. Statically, the perf sources name no audit
// path and call no ledger function.
// regression: F-dt23-perf-record
func TestPerfNeverTouchesTheAuditLedger(t *testing.T) {
	state := perfFixture(t)
	ledger := map[string][]byte{
		"audit.jsonl":            []byte(`{"ts":"2026-10-13T00:00:00Z","tool":"deskpr","verb":"create","result":"ok"}` + "\n"),
		"audit.jsonl.2026-10-12": []byte(`{"ts":"2026-10-12T00:00:00Z","tool":"deskpr","verb":"create","result":"ok"}` + "\n"),
	}
	stamp := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	for name, body := range ledger {
		p := filepath.Join(state, name)
		if err := os.WriteFile(p, body, 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(p, stamp, stamp); err != nil {
			t.Fatal(err)
		}
	}

	PerfRun("deskpr", []string{"create"}, []string{"create"}, func([]string) int { return 0 })
	PerfStep("deskboot", "token-mint", time.Millisecond, 0)
	perfPrune(filepath.Join(state, "perf"))
	if recs, _, err := LoadPerfDays(7); err != nil || len(recs) != 2 {
		t.Fatalf("LoadPerfDays = %d records, %v; want the 2 just written", len(recs), err)
	}

	for name, body := range ledger {
		p := filepath.Join(state, name)
		got, err := os.ReadFile(p)
		if err != nil || !bytes.Equal(got, body) {
			t.Fatalf("%s changed: %q, %v", name, got, err)
		}
		st, err := os.Stat(p)
		if err != nil || !st.ModTime().Equal(stamp) {
			t.Fatalf("%s mtime moved: %v, %v", name, st.ModTime(), err)
		}
	}
	entries, err := os.ReadDir(state)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	if strings.Join(names, ",") != "audit.jsonl,audit.jsonl.2026-10-12,perf" {
		t.Fatalf("state dir after a perf run = %v; want only the ledger files plus perf/", names)
	}

	// No perf code path can open the ledger: no string literal naming it, no ledger function.
	ledgerFuncs := map[string]bool{"auditPath": true, "segmentPaths": true, "Log": true, "LoadEntries": true,
		"loadEntriesFrom": true, "LastEntry": true, "FirstTS": true, "rotateIfNeeded": true}
	for _, src := range []string{"perf.go", "perfread.go", "perfcpu_unix.go", "perfcpu_windows.go"} {
		f, err := parser.ParseFile(token.NewFileSet(), src, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", src, err)
		}
		ast.Inspect(f, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.BasicLit:
				if x.Kind == token.STRING && strings.Contains(strings.ToLower(x.Value), "audit") {
					t.Errorf("%s: string literal %s names the audit ledger", src, x.Value)
				}
			case *ast.Ident:
				if ledgerFuncs[x.Name] {
					t.Errorf("%s: references ledger function %s", src, x.Name)
				}
			}
			return true
		})
	}
}

// TestPerfAppendMedianUnderOneMillisecond writes 1000 records to a temp dir and GATES on the
// measured median append being under 1 ms.
func TestPerfAppendMedianUnderOneMillisecond(t *testing.T) {
	perfFixture(t)
	rec := newPerfRecord("deskpr", "create", 12*time.Millisecond, 0)
	durs := make([]time.Duration, 1000)
	for i := range durs {
		began := time.Now()
		perfAppend(rec)
		durs[i] = time.Since(began)
	}
	sort.Slice(durs, func(i, j int) bool { return durs[i] < durs[j] })
	median := durs[len(durs)/2]
	t.Logf("perf append over %d records: p50 %v, p90 %v, max %v", len(durs), median, durs[len(durs)*9/10], durs[len(durs)-1])
	if median >= time.Millisecond {
		t.Fatalf("median perf append = %v, want < 1ms", median)
	}
}

// BenchmarkPerfAppend measures one best-effort record append.
func BenchmarkPerfAppend(b *testing.B) {
	dir := b.TempDir()
	oldDir := dirOverride
	dirOverride = dir
	resetPerfPrune()
	b.Cleanup(func() { dirOverride = oldDir; resetPerfPrune() })
	rec := newPerfRecord("deskpr", "create", 12*time.Millisecond, 0)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		perfAppend(rec)
	}
}
