package reviewtest_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

func perfEntry(t *testing.T, bin, tool, home, consent string, args []string) (deskkit.PerfRecord, string, int, []byte) {
	t.Helper()
	dir := filepath.Join(home, ".config", "assay")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	roster := "ASSAY_BLESS_LOGIN=example-driver:2001\nASSAY_TRUSTED_LOGINS=example-driver:2001\nASSAY_ALLOWED_REPOS=example-org/tracker:ci:public\nASSAY_SCAN_REPOS=example-org/tracker\n"
	if err := os.WriteFile(filepath.Join(dir, "roster.env"), []byte(roster), 0600); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(args)
	cmd := exec.Command(filepath.Join(bin, tool+".test"), "-test.run=^TestPerfEntrypoint$")
	cmd.Dir = home
	cmd.Env = append(isolatedEnv(home), "ASSAY_TELEMETRY="+consent, "DESK_LOOP=worker-desk", "ASSAY_PERF_TEST_HOME="+home, "ASSAY_PERF_TEST_ARGS="+string(raw))
	out, _ := cmd.CombinedOutput()
	if cmd.ProcessState == nil {
		t.Fatalf("%s did not start", tool)
	}
	day := filepath.Join(dir, "perf", time.Now().UTC().Format("2006-01-02")+".jsonl")
	data, err := os.ReadFile(day)
	if err != nil && !os.IsNotExist(err) && !errors.Is(err, syscall.ENOTDIR) {
		t.Fatal(err)
	}
	var rec deskkit.PerfRecord
	if len(data) > 0 {
		if err := json.Unmarshal(bytes.TrimSpace(data), &rec); err != nil {
			t.Fatal(err)
		}
	}
	return rec, string(out), cmd.ProcessState.ExitCode(), data
}

// mutationBinary recompiles a real command against one planted production change.
// The source tree is untouched; Go's overlay binds the exact original file path.
func mutationBinary(t *testing.T, tool, source, before, after string) string {
	t.Helper()
	dir := t.TempDir()
	original := filepath.Join(moduleRoot(t), source)
	data, err := os.ReadFile(original)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(data), before) != 1 {
		t.Fatalf("mutation target %s is not unique", source)
	}
	replacement := filepath.Join(dir, "mutated.go")
	if err := os.WriteFile(replacement, []byte(strings.Replace(string(data), before, after, 1)), 0600); err != nil {
		t.Fatal(err)
	}
	overlay, _ := json.Marshal(map[string]any{"Replace": map[string]string{original: replacement}})
	overlayPath := filepath.Join(dir, "overlay.json")
	if err := os.WriteFile(overlayPath, overlay, 0600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "test", "-overlay", overlayPath, "-c", "-o", filepath.Join(dir, tool+".test"), "./cmd/"+tool)
	cmd.Dir = moduleRoot(t)
	cmd.Env = append(os.Environ(), "GOWORK=off", "GOPROXY=off", "GOSUMDB=off")
	if log, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("mutation build: %v\n%s", err, log)
	}
	return dir
}

func TestPerfCommandKit(t *testing.T) {
	bin := build(t)
	for _, tc := range []struct {
		tool string
		args []string
		want string
	}{
		{"deskboard", []string{"--version"}, ""},
		{"deskboard", []string{"--help"}, ""},
		{"deskboard", []string{"--json", "actions"}, "actions"},
		{"deskboard", []string{"next-up"}, "dispatch"},
		{"deskboard", []string{"--throughput"}, "throughput"},
		{"deskboard", []string{"private-argument"}, ""},
		{"issueboard", []string{"--version"}, "--version"},
		{"issueboard", []string{"--help"}, "--help"},
		{"issueboard", []string{"--root", ".", "intake"}, "intake"},
		{"issueboard", []string{"board", "--sla-days", "-1"}, "board"},
		{"issueboard", []string{"private-argument"}, "other"},
		{"deskdispatch", []string{"--kits"}, "--kits"},
		{"deskdispatch", []string{"--help"}, "--help"},
		{"deskdispatch", []string{"bad-item", "--kit", "review"}, "dispatch:review"},
		{"deskdispatch", []string{"bad-item", "--kit=verifier"}, "dispatch:verifier"},
		{"deskdispatch", []string{"bad-item"}, "dispatch:worker"},
		{"deskdispatch", []string{"bad-item", "--kit", "worker-objective"}, "dispatch:worker-objective"},
		{"deskdispatch", []string{"bad-item", "--kit", "private-argument"}, "other"},
		{"deskdispatch", []string{"--stamp-only", "--kit=review"}, "stamp-only:review"},
	} {
		t.Run(tc.tool+"/"+strings.Join(tc.args, "_"), func(t *testing.T) {
			home := t.TempDir()
			rec, _, code, raw := perfEntry(t, bin, tc.tool, home, "1", tc.args)
			if tc.want == "" {
				if len(raw) != 0 {
					t.Fatalf("help/version/parse refusal unexpectedly recorded: %s", raw)
				}
				return
			}
			if rec.Verb != tc.want || rec.Tool != tc.tool || rec.Exit != code || rec.Schema != "perf-v1" {
				t.Fatalf("record %+v, want verb=%s exit=%d", rec, tc.want, code)
			}
			if bytes.Contains(raw, []byte("private-argument")) || bytes.Contains(raw, []byte(home)) || bytes.Contains(raw, []byte("bad-item")) {
				t.Fatalf("private argv leaked: %s", raw)
			}
			// Existing JSON reader has no enum gate: the additive closed verb values
			// decode alongside unchanged legacy fixture records and remain non-steps.
			t.Setenv("HOME", home)
			history, err := deskkit.LoadPerfHistory(7)
			if err != nil || history.Skipped != 0 || len(history.Records) != 1 || history.Records[0].Verb != tc.want {
				t.Fatalf("existing history reader rejected new verb: %+v %v", history, err)
			}
			if deskkit.IsPerfStep(rec) {
				t.Fatal("operation became a boot step")
			}
		})
	}
	t.Run("mutation_actual_kit_assignment", func(t *testing.T) {
		mutated := mutationBinary(t, "deskdispatch", "cmd/deskdispatch/dispatch.go", `deskkit.PerfCommand(perfVerbs, perfKitVerb("dispatch", *kit))`, "")
		rec, _, _, _ := perfEntry(t, mutated, "deskdispatch", t.TempDir(), "1", []string{"bad-item", "--kit=review"})
		if rec.Verb == "dispatch:review" {
			t.Fatal("oracle accepted removed production kit attribution")
		}
		if rec.Verb != "other" {
			t.Fatalf("mutation did not reach wrong observation: %+v", rec)
		}
	})

	// Existing perf-v1 fixture: exact same type/reader, no schema migration.
	files, err := filepath.Glob("../deskkit/testdata/perf/*.jsonl")
	if err != nil || len(files) == 0 {
		t.Fatal("legacy fixtures missing", err)
	}
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		for _, line := range bytes.Split(bytes.TrimSpace(data), []byte("\n")) {
			var rec deskkit.PerfRecord
			if err := json.Unmarshal(line, &rec); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func TestPerfCoverageFlow(t *testing.T) {
	bin := build(t)
	// A real entrypoint's failing ordinary dispatch has identical behavior with
	// consent off and a broken optional sink. No audit or budget file is modified
	// by the performance writer; existing command auditing stays load-bearing.
	for _, tool := range []string{"deskboard", "issueboard", "deskdispatch"} {
		args := []string{"private-argument"}
		if tool == "deskdispatch" {
			args = []string{"bad-item", "--kit=private-argument"}
		}
		home := t.TempDir()
		_, off, offExit, raw := perfEntry(t, bin, tool, home, "0", args)
		if len(raw) != 0 {
			t.Fatal("consent-off wrote history")
		}
		state := filepath.Join(home, ".config", "assay")
		auditBefore, _ := os.ReadFile(filepath.Join(state, "audit.jsonl"))
		if err := os.WriteFile(filepath.Join(state, "perf"), []byte("broken sink"), 0600); err != nil {
			t.Fatal(err)
		}
		_, broken, brokenExit, raw := perfEntry(t, bin, tool, home, "1", args)
		auditAfter, _ := os.ReadFile(filepath.Join(state, "audit.jsonl"))
		if brokenExit != offExit || broken != off || len(raw) != 0 {
			t.Fatalf("%s optional sink changed behavior: exit %d/%d\n%s\n%s", tool, brokenExit, offExit, broken, off)
		}
		// Command audit timestamps may differ; performance data never replaces or
		// truncates those bytes. Compare complete prefix, not timestamp guesses.
		if !bytes.HasPrefix(auditAfter, auditBefore) {
			t.Fatal("performance recording changed prior audit")
		}
	}
	// Tests in the production packages own the unexported transport/refusal seams.
	// Run their named flow assertions through the already-built command test binary.
	cmd := exec.Command(filepath.Join(bin, "deskboard.test"), "-test.run=^TestPerf(LeafTransport|StatusgenChildren)$", "-test.v")
	cmd.Env = isolatedEnv(t.TempDir())
	log, err := cmd.CombinedOutput()
	if err != nil || !strings.Contains(string(log), "--- PASS: TestPerfLeafTransport ") || !strings.Contains(string(log), "--- PASS: TestPerfStatusgenChildren ") {
		t.Fatalf("production transport: %v\n%s", err, log)
	}

	t.Run("mutation_counts_local_refusal", func(t *testing.T) {
		mutated := mutationBinary(t, "deskboard", "internal/deskkit/forgetransport.go", `return &http.Client{Transport: countedTransport{next: githubTransportWrapper(countRoundTrips(http.DefaultTransport))}}`, `return &http.Client{Transport: githubTransportWrapper(http.DefaultTransport)}`)
		bad := exec.Command(filepath.Join(mutated, "deskboard.test"), "-test.run=^TestPerfLeafTransport$", "-test.v")
		bad.Env = isolatedEnv(t.TempDir())
		log, err := bad.CombinedOutput()
		if err == nil || !strings.Contains(string(log), "local refusal became HTTP work") {
			t.Fatalf("counter mutation not rejected: %v\n%s", err, log)
		}
	})
	cmd = exec.Command("go", "test", "-count=1", "-v", "-run", "^TestPerf(ProcessCoverage|AccountTransport)$", "./internal/deskkit")
	cmd.Dir = moduleRoot(t)
	cmd.Env = append(os.Environ(), "GOWORK=off", "GOPROXY=off", "GOSUMDB=off")
	log, err = cmd.CombinedOutput()
	if err != nil || !strings.Contains(string(log), "--- PASS: TestPerfProcessCoverage ") || !strings.Contains(string(log), "--- PASS: TestPerfAccountTransport ") {
		t.Fatalf("production process paths: %v\n%s", err, log)
	}
	fmt.Fprintln(os.Stdout, "counter scope: own-process CPU; descendant CPU and GraphQL points unknown; no perf-v1 expansion")
}
