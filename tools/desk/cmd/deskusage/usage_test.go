package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/gitquiet/goquiet"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

func setupUsage(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("DESK_TOOLS_DISABLED", "")
	t.Setenv("DESK_LOOP", "")
	t.Setenv("ASSAY_TELEMETRY", "")
	t.Setenv("ASSAY_USAGE_DIR", filepath.Join(home, "counts"))
	deskkit.ReloadConfig()
	t.Cleanup(deskkit.ReloadConfig)
	return home
}
func invoke(args ...string) (int, string, string) {
	var out, err bytes.Buffer
	rc := run(args, &out, &err)
	return rc, out.String(), err.String()
}
func recordArgs(ref string) []string {
	return []string{"record", ref, "--repo", "example-org/project", "--item", "example/01", "--tier", "strong"}
}

const testRef = "assay--example--01@20261006T141502Z.3fa9c01b7d2e"

func storeBytes(t *testing.T) []byte {
	t.Helper()
	d, e := deskkit.UsageStoreDir()
	if e != nil {
		t.Fatal(e)
	}
	b, e := os.ReadFile(filepath.Join(d, deskkit.UsageFile))
	if e != nil && !os.IsNotExist(e) {
		t.Fatal(e)
	}
	return b
}

// regression: F-usage-transcript-input
func TestUsageRecord_RefusesTranscriptShapedInput(t *testing.T) {
	setupUsage(t)
	rc, _, e := invoke(recordArgs(testRef)...)
	if rc != 0 {
		t.Fatal(rc, e)
	}
	before := storeBytes(t)
	canary := `{"message":{"content":"USAGE_CANARY"}}`
	for _, extra := range [][]string{{"--input-tokens", canary}, {"--transcript", "x"}, {"--prompt", "x"}, {"--input-tokens", "-1"}, {"--input-tokens", "1.5"}, {"--tier", "vendor-model"}, {"extra"}} {
		rc, _, _ := invoke(append(recordArgs(testRef), extra...)...)
		if rc != 5 {
			t.Fatalf("accepted %v: exit %d", extra, rc)
		}
		if !bytes.Equal(before, storeBytes(t)) {
			t.Fatal("invalid input changed store")
		}
	}
	r, w, e2 := os.Pipe()
	if e2 != nil {
		t.Fatal(e2)
	}
	defer r.Close()
	if _, e2 = io.WriteString(w, canary); e2 != nil {
		t.Fatal(e2)
	}
	w.Close()
	old := os.Stdin
	os.Stdin = r
	t.Cleanup(func() { os.Stdin = old })
	rc, _, e = invoke(recordArgs(testRef)...)
	if rc != 0 {
		t.Fatal(rc, e)
	}
	remaining, e2 := io.ReadAll(r)
	if e2 != nil || string(remaining) != canary {
		t.Fatalf("stdin consumed: %q %v", remaining, e2)
	}
	if bytes.Contains(storeBytes(t), []byte("USAGE_CANARY")) {
		t.Fatal("transcript leaked")
	}
	// Positional and claim-key forms are mutually exclusive and missing refs refuse.
	for _, args := range [][]string{{"record"}, {"record", testRef, "--claim-key", "x"}} {
		rc, _, _ := invoke(args...)
		if rc != 5 {
			t.Fatal("invalid selector accepted")
		}
	}
}

// regression: F-usage-polluted-reader
func TestUsageSummary_RefusesPollutedStore(t *testing.T) {
	setupUsage(t)
	rc, _, e := invoke(recordArgs(testRef)...)
	if rc != 0 {
		t.Fatal(rc, e)
	}
	rc, out, e := invoke("summary", "--json")
	if rc != 0 || strings.Contains(out, "dispatch_ref") {
		t.Fatal(rc, out, e)
	}
	d, _ := deskkit.UsageStoreDir()
	f, e2 := os.OpenFile(filepath.Join(d, deskkit.UsageFile), os.O_APPEND|os.O_WRONLY, 0600)
	if e2 != nil {
		t.Fatal(e2)
	}
	polluted := bytes.TrimSpace(storeBytes(t))
	polluted = bytes.Replace(polluted, []byte(`"schema":`), []byte(`"prompt":"CANARY","schema":`), 1)
	if _, e2 = f.Write(append(polluted, '\n')); e2 != nil {
		t.Fatal(e2)
	}
	f.Close()
	rc, out, e = invoke("summary", "--json")
	if rc != 6 || out != "" || !strings.Contains(e, "line 2") {
		t.Fatalf("polluted store: %d %q %q", rc, out, e)
	}
}

func dispatchLine(t *testing.T, key string, at time.Time) string {
	t.Helper()
	ref, e := deskkit.MintDispatchRef(key, at, bytes.NewReader(make([]byte, 6)))
	if e != nil {
		t.Fatal(e)
	}
	item, branch, session, tier, kit, stamp := "example/01", "feature/example", "unknown", "strong", "worker", "pending"
	attempt := 1
	r := deskkit.DispatchRecord{Schema: deskkit.DispatchRecordSchema, Event: deskkit.DispatchEventDispatched, TS: at.Format(time.RFC3339), DispatchRef: &ref, ClaimKey: key, Repo: "example-org/project", Item: &item, Branch: &branch, SessionTag: session, Tier: &tier, Kit: &kit, ModelStamp: &stamp, AttemptLocal: &attempt}
	if e = deskkit.AppendDispatchRecord(&r); e != nil {
		t.Fatal(e)
	}
	return ref
}

// regression: F1 usage-claimkey-dispatch-store-wedge
func TestUsageClaimKeyDispatchRecovery(t *testing.T) {
	for _, scenario := range []string{"torn-before", "obsolete-before", "added-field", "torn-after", "invalid-match-after", "recovered-match"} {
		t.Run(scenario, func(t *testing.T) {
			setupUsage(t)
			key := "assay--example--01"
			at := time.Date(2026, 10, 6, 14, 15, 2, 0, time.UTC)
			path, err := deskkit.DispatchRecordsPath()
			if err != nil {
				t.Fatal(err)
			}
			appendLine := func(line string) {
				t.Helper()
				f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0600)
				if err != nil {
					t.Fatal(err)
				}
				if _, err = f.WriteString(line + "\n"); err != nil {
					t.Fatal(err)
				}
				if err = f.Close(); err != nil {
					t.Fatal(err)
				}
			}
			dispatchLine(t, "assay--other--09", at)
			if scenario == "torn-before" {
				appendLine(`{"schema":"dispatch-record-v1","event":"dispa`)
			}
			if scenario == "obsolete-before" {
				appendLine(`{"schema":"old","event":"dispatched","claim_key":"assay--other--09"}`)
			}
			want := dispatchLine(t, key, at.Add(time.Second))
			if scenario == "added-field" {
				b, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				lines := bytes.Split(bytes.TrimSpace(b), []byte("\n"))
				appendLine(strings.TrimSuffix(string(lines[len(lines)-1]), "}") + `,"future_field":true}`)
			}
			if scenario == "torn-after" || scenario == "recovered-match" {
				appendLine(`{"event":"dispa`)
			}
			if scenario == "invalid-match-after" {
				appendLine(`{"schema":"old","event":"dispatched","claim_key":"` + key + `"}`)
			}
			if scenario == "recovered-match" {
				want = dispatchLine(t, key, at.Add(2*time.Second))
			}
			rc, _, stderr := invoke("record", "--claim-key", key, "--repo", "example-org/project", "--item", "example/01", "--tier", "strong", "--input-tokens", "10")
			if scenario == "torn-after" || scenario == "invalid-match-after" {
				if rc != 5 || len(storeBytes(t)) != 0 {
					t.Fatalf("ambiguous latest dispatch recorded stale counts: rc=%d stderr=%q", rc, stderr)
				}
				return
			}
			if rc != 0 {
				t.Fatalf("unrelated historical damage wedged usage: rc=%d stderr=%q", rc, stderr)
			}
			var got deskkit.UsageRecord
			if err = json.Unmarshal(bytes.TrimSpace(storeBytes(t)), &got); err != nil {
				t.Fatal(err)
			}
			if got.DispatchRef != want {
				t.Fatalf("joined %q, want latest %q", got.DispatchRef, want)
			}
		})
	}
}

// regression: F-usage-run-join-and-aggregates
func TestUsage_RecordToSummaryFlow(t *testing.T) {
	home := setupUsage(t)
	// Exercise config-file resolution with the environment override absent.
	os.Unsetenv("ASSAY_USAGE_DIR")
	dir := filepath.Join(home, "configured")
	cfg := filepath.Join(home, ".config", "assay")
	if e := os.MkdirAll(cfg, 0700); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(filepath.Join(cfg, "roster.env"), []byte("ASSAY_USAGE_DIR="+dir+"\n"), 0600); e != nil {
		t.Fatal(e)
	}
	deskkit.ReloadConfig()
	key := "assay--example--01"
	now := time.Date(2026, 10, 6, 14, 15, 2, 0, time.UTC)
	_ = dispatchLine(t, key, now)
	latest := dispatchLine(t, key, now.Add(time.Second))
	args := []string{"record", "--claim-key", key, "--repo", "example-org/project", "--item", "example/01", "--tier", "strong", "--input-tokens", "10", "--output-tokens", "0"}
	if rc, _, e := invoke(args...); rc != 0 {
		t.Fatal(rc, e)
	}
	second := recordArgs(testRef)
	second = append(second, "--input-tokens", "20", "--output-tokens", "could-not-check")
	if rc, _, e := invoke(second...); rc != 0 {
		t.Fatal(rc, e)
	}
	third := []string{"record", testRef, "--repo", "example-org/project", "--item", "example/02", "--tier", "any", "--input-tokens", "4"}
	if rc, _, e := invoke(third...); rc != 0 {
		t.Fatal(rc, e)
	}
	if _, e := os.Stat(filepath.Join(dir, deskkit.UsageFile)); e != nil {
		t.Fatal("roster key not applied", e)
	}
	rows, e := deskkit.ReadUsageRecords()
	if e != nil || len(rows) != 3 || rows[0].DispatchRef != latest {
		t.Fatalf("joined records %+v %v", rows, e)
	}
	before := storeBytes(t)
	for _, bad := range [][]string{{"record", "--claim-key", "absent", "--repo", "example-org/project", "--item", "example/01", "--tier", "any"}, {"record", testRef, "--claim-key", key}, {"record", "--claim-key", key, "--repo", "example-org/other", "--item", "example/01", "--tier", "any"}} {
		if rc, _, _ := invoke(bad...); rc != 5 {
			t.Fatalf("accepted bad claim %v", bad)
		}
		if !bytes.Equal(before, storeBytes(t)) {
			t.Fatal("refusal wrote counts")
		}
	}
	rc, out, diag := invoke("summary", "--by", "tier", "--json")
	if rc != 0 {
		t.Fatal(rc, diag)
	}
	var groups []usageGroup
	if e = json.Unmarshal([]byte(out), &groups); e != nil {
		t.Fatal(e)
	}
	if len(groups) != 2 || groups[1].Group != "strong" || groups[1].Records != 2 {
		t.Fatal(out)
	}
	s := groups[1].Counts["input_tokens"]
	if s.Measured != 2 || s.Sum != 30 || s.Median == nil || *s.Median != 15 {
		t.Fatal(out)
	}
	s = groups[1].Counts["output_tokens"]
	if s.Measured != 1 || s.Sum != 0 || s.CouldNotCheck != 1 || s.Null != 0 {
		t.Fatal(out)
	}
	if groups[1].Counts["wall_seconds"].Null != 2 || strings.Contains(out, "dispatch_ref") {
		t.Fatal(out)
	}
	rc, out, diag = invoke("summary", "--by", "item")
	if rc != 0 || !strings.Contains(out, "example/01: 2 records") {
		t.Fatal(rc, out, diag)
	}
	rc, _, _ = invoke("summary", "--by", "person")
	if rc != 5 {
		t.Fatal("arbitrary grouping admitted")
	}
}

// regression: F-usage-audit-guard
func TestUsageAuditAndStop(t *testing.T) {
	home := setupUsage(t)
	for _, args := range [][]string{recordArgs(testRef), {"summary", "--json"}, {"record", "--prompt", "CANARY"}} {
		if rc, _, _ := invoke(args...); rc != 0 && rc != 5 {
			t.Fatal(rc)
		}
	}
	b, e := os.ReadFile(filepath.Join(home, ".config", "assay", "audit.jsonl"))
	if e != nil {
		t.Fatal(e)
	}
	lines := bytes.Split(bytes.TrimSpace(b), []byte("\n"))
	if len(lines) != 3 {
		t.Fatalf("wanted one audit line per action: %s", b)
	}
	for _, line := range lines {
		var a deskkit.Entry
		if e = json.Unmarshal(line, &a); e != nil {
			t.Fatal(e)
		}
		if a.Tool != "deskusage" || a.ArgsDigest != "" || a.BodyDigest != "" || a.Detail != "local usage counts" {
			t.Fatalf("unexpected audit fields %s", line)
		}
	}
	if bytes.Contains(b, []byte("CANARY")) || bytes.Contains(b, []byte(testRef)) {
		t.Fatal("usage input leaked into audit")
	}
	for _, args := range [][]string{{"--help"}, {"record", "--help"}, {"--version"}} {
		rc, _, diag := invoke(args...)
		if rc != 0 {
			t.Fatal(rc, diag)
		}
	}
	after, e := os.ReadFile(filepath.Join(home, ".config", "assay", "audit.jsonl"))
	if e != nil || !bytes.Equal(after, b) {
		t.Fatal("help/version audit effect")
	}
	before := storeBytes(t)
	t.Setenv("DESK_TOOLS_DISABLED", "1")
	if rc, _, _ := invoke(recordArgs(testRef)...); rc != 3 {
		t.Fatalf("kill switch exit %d", rc)
	}
	if !bytes.Equal(before, storeBytes(t)) {
		t.Fatal("disabled write")
	}
	after, e = os.ReadFile(filepath.Join(home, ".config", "assay", "audit.jsonl"))
	if e != nil || len(bytes.Split(bytes.TrimSpace(after), []byte("\n"))) != 4 {
		t.Fatalf("disabled audit: %s %v", after, e)
	}
}

// regression: F-usage-perf-wrapper
func TestUsagePerformance(t *testing.T) {
	// Build before moving HOME: module caches must not land in the scratch usage home.
	binary := filepath.Join(t.TempDir(), "deskusage")
	if os.PathSeparator == '\\' {
		binary += ".exe"
	}
	build := exec.Command("go", "build", "-o", binary, ".")
	if b, e := goquiet.Cmd(build).CombinedOutput(); e != nil {
		t.Fatalf("build: %v %s", e, b)
	}
	home := setupUsage(t)
	// Even opted-in help must return before operational audit/config/performance effects.
	help := exec.Command(binary, "--help")
	help.Env = append(os.Environ(), "ASSAY_TELEMETRY=1")
	if b, e := help.CombinedOutput(); e != nil {
		t.Fatalf("help: %v %s", e, b)
	}
	if _, e := os.Stat(filepath.Join(home, ".config", "assay")); !os.IsNotExist(e) {
		t.Fatal("help created operational state")
	}
	var prior []byte
	for _, enabled := range []string{"0", "1"} {
		cmd := exec.Command(binary, "summary", "--json")
		cmd.Env = append(os.Environ(), "ASSAY_TELEMETRY="+enabled)
		b, e := cmd.CombinedOutput()
		if e != nil {
			t.Fatalf("summary: %v %s", e, b)
		}
		if prior != nil && !bytes.Equal(prior, b) {
			t.Fatal("optional performance recording changed command output")
		}
		prior = b
	}
	path := filepath.Join(home, ".config", "assay", "perf", time.Now().UTC().Format("2006-01-02")+".jsonl")
	b, e := os.ReadFile(path)
	if e != nil {
		t.Fatal("missing opted-in performance sample", e)
	}
	lines := bytes.Split(bytes.TrimSpace(b), []byte("\n"))
	if len(lines) != 1 {
		t.Fatalf("expected exactly one opted-in sample: %s", b)
	}
	var record deskkit.PerfRecord
	if e = json.Unmarshal(lines[0], &record); e != nil {
		t.Fatal(e)
	}
	if record.Tool != "deskusage" {
		t.Fatalf("wrong tool attribution: %s", b)
	}
	if _, e = os.Stat(filepath.Join(home, "go", "pkg", "mod")); !os.IsNotExist(e) {
		t.Fatal("Go module cache was created inside scratch usage HOME")
	}
	if _, e = os.Stat(filepath.Join(home, "counts", deskkit.UsageFile)); !os.IsNotExist(e) {
		t.Fatal("help created a worker usage record")
	}
}
