package reviewtest_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

func perfEntry(t *testing.T, bin, tool, home, consent string, args []string, extraEnv ...string) (deskkit.PerfRecord, string, int, []byte) {
	t.Helper()
	dir := filepath.Join(home, ".config", "assay")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	roster := "ASSAY_BLESS_LOGIN=example-driver:2001\nASSAY_TRUSTED_LOGINS=example-driver:2001\nASSAY_TRUSTED_BOT_SLUGS=desk=example-dispatcher:300000001,worker=example-worker:300000002\nASSAY_ALLOWED_REPOS=example-org/tracker:ci:public\nASSAY_SCAN_REPOS=example-org/tracker\nASSAY_REPO_FORGES=example-org/tracker=github\n"
	if err := os.WriteFile(filepath.Join(dir, "roster.env"), []byte(roster), 0600); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(args)
	cmd := exec.Command(filepath.Join(bin, tool+".test"), "-test.run=^TestPerfEntrypoint$")
	cmd.Dir = home
	cmd.Env = append(isolatedEnv(home), "ASSAY_TELEMETRY="+consent, "DESK_LOOP=worker-desk", "DESK_SESSION=example-perf-preservation", "ASSAY_PERF_TEST_HOME="+home, "ASSAY_PERF_TEST_ARGS="+string(raw))
	cmd.Env = append(cmd.Env, extraEnv...)
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

// All three invoke main and perform successful ordinary work. The two Forge
// fixtures replace only the far side of the HTTP transport; intake reads a real
// entry. No caller, guard, logger, rate-limit reader or perf writer is stubbed.
func preservationCase(t *testing.T, bin, tool string) error {
	t.Helper()
	home := t.TempDir()
	state := filepath.Join(home, ".config", "assay")
	if err := os.MkdirAll(state, 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	deskkit.ReloadConfig()
	t.Cleanup(deskkit.ReloadConfig)
	args := []string{"files", "example-org/tracker", "77"}
	verb, repo, pr := "files", "", 0
	extra := []string{"ASSAY_PERF_TEST_WORK=1"}
	switch tool {
	case "issueboard":
		verb = "intake"
		root := filepath.Join(home, "root")
		dir := filepath.Join(root, "docs", "streams", "intake")
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
		entry := "---\nid: example-intake\ntitle: Example preservation work\ndisposition: new\ndate: " + time.Now().UTC().Format("2006-01-02") + "\n---\n"
		if err := os.WriteFile(filepath.Join(dir, "example.md"), []byte(entry), 0600); err != nil {
			t.Fatal(err)
		}
		args = []string{"intake", "--root", root}
	case "deskdispatch":
		verb, repo, pr = "stamp-only", "example-org/tracker", 77
		args = []string{"--stamp-only", "--repo", repo, "--pr", "77", "--model", "example-model", "--tier", "strong"}
		extra = append(extra, "DESK_LOOP=the-desk")
	}
	// Budget state lives in the audit ledger, not a separate budget file. Seed
	// one remaining charge for this tool, a full unrelated bucket, and an open
	// breaker. Query the real reader before and after each production invocation.
	now := time.Now().UTC()
	seed := []byte{}
	add := func(key, target, result string, number, count int) {
		for i := 0; i < count; i++ {
			var n *int
			if number != 0 {
				v := number
				n = &v
			}
			row, err := json.Marshal(deskkit.Entry{TS: now.Add(-time.Minute).Format(time.RFC3339), Tool: key, Verb: "example-write", Repo: target, PR: n, Result: result, Detail: "seeded preservation history"})
			if err != nil {
				t.Fatal(err)
			}
			seed = append(seed, append(row, '\n')...)
		}
	}
	add(tool, repo, deskkit.ResultOK, pr, deskkit.RateLimitPerPRPerHour-1)
	add("deskpost", "example-org/tracker", deskkit.ResultOK, 78, deskkit.RateLimitPerPRPerHour)
	add("deskpost", "example-org/tracker", deskkit.ResultRefused, 79, deskkit.BreakerTrip)
	budget := func() []int {
		at := time.Now().Add(time.Second)
		return []int{
			deskkit.ExitCodeOf(deskkit.AllowWriteAt(tool, repo, pr, at)),
			deskkit.ExitCodeOf(deskkit.AllowWriteAt("deskpost", "example-org/tracker", 78, at)),
			deskkit.ExitCodeOf(deskkit.AllowWriteAt("deskpost", "example-org/tracker", 79, at)),
		}
	}
	type outcome struct {
		Output string
		Audit  deskkit.Entry
		Work   string
		Budget []int
	}
	var reference outcome
	for i, consent := range []string{"0", "1"} {
		auditPath := filepath.Join(state, "audit.jsonl")
		if err := os.WriteFile(auditPath, seed, 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.Remove(filepath.Join(home, "work.jsonl")); err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
		if got := budget(); !reflect.DeepEqual(got, []int{0, 4, 4}) {
			return fmt.Errorf("seeded budget/breaker not established: %v", got)
		}
		if consent == "1" {
			if err := os.WriteFile(filepath.Join(state, "perf"), []byte("broken optional sink"), 0600); err != nil {
				t.Fatal(err)
			}
		}
		_, output, code, raw := perfEntry(t, bin, tool, home, consent, args, extra...)
		if code != 0 || len(raw) != 0 {
			return fmt.Errorf("ordinary %s failed: exit=%d perf=%s\n%s", tool, code, raw, output)
		}
		var work []byte
		if tool == "issueboard" {
			if !strings.Contains(output, "example-intake") || !strings.Contains(output, "Example preservation work") {
				return fmt.Errorf("ordinary intake work missing: %s", output)
			}
			work = []byte("example-intake rendered")
		} else {
			var err error
			work, err = os.ReadFile(filepath.Join(home, "work.jsonl"))
			if err != nil || len(work) == 0 {
				return fmt.Errorf("ordinary %s work missing: %v", tool, err)
			}
			if tool == "deskboard" && (!strings.Contains(output, "example.go") || !bytes.Contains(work, []byte("/pulls/77/files"))) {
				return fmt.Errorf("ordinary files work missing: %s %s", output, work)
			}
			if tool == "deskdispatch" && (!strings.Contains(output, "OK:") || !bytes.Contains(work, []byte("authored-model:example-model")) || !bytes.Contains(work, []byte("authored-tier:strong")) || !bytes.Contains(work, []byte(`"Method":"POST"`))) {
				return fmt.Errorf("ordinary stamp work missing: %s %s", output, work)
			}
		}
		after, err := os.ReadFile(auditPath)
		if err != nil {
			return fmt.Errorf("audit read failed: %w", err)
		}
		if !bytes.HasPrefix(after, seed) {
			return fmt.Errorf("seeded audit history changed")
		}
		lines := bytes.Split(bytes.TrimSpace(after[len(seed):]), []byte("\n"))
		if len(lines) != 1 || len(lines[0]) == 0 {
			return fmt.Errorf("ordinary audit append missing/duplicated: %s", after[len(seed):])
		}
		var row deskkit.Entry
		if err := json.Unmarshal(lines[0], &row); err != nil {
			return fmt.Errorf("audit append malformed: %w", err)
		}
		if row.Tool != tool || row.Verb != verb || row.Result != deskkit.ResultOK {
			return fmt.Errorf("ordinary audit result changed: %+v", row)
		}
		row.TS = "" // Only the actual execution timestamp may differ.
		got := outcome{Output: regexp.MustCompile(`\d{4}-\d\d-\d\dT\d\d:\d\d:\d\dZ`).ReplaceAllString(output, "<timestamp>"), Audit: row, Work: string(work), Budget: budget()}
		if !reflect.DeepEqual(got.Budget, []int{4, 4, 4}) {
			return fmt.Errorf("ordinary audit/budget charge lost: %v", got.Budget)
		}
		if i == 0 {
			reference = got
		} else if !reflect.DeepEqual(got, reference) {
			return fmt.Errorf("optional sink changed ordinary effects:\n got %+v\nwant %+v", got, reference)
		}
	}
	return nil
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
		mutated := mutationBinary(t, "deskdispatch", "cmd/deskdispatch/cli.go", `deskkit.PerfCommand(perfVerbs, perfKitVerb("dispatch", v.String("kit")))`, "")
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
	for _, tool := range []string{"deskboard", "issueboard", "deskdispatch"} {
		t.Run("ordinary_"+tool, func(t *testing.T) {
			if err := preservationCase(t, bin, tool); err != nil {
				t.Fatal(err)
			}
		})
	}
	t.Run("mutation_skips_ordinary_work", func(t *testing.T) {
		mutated := mutationBinary(t, "issueboard", "internal/deskkit/perf.go", "code := run(args)", "code := 0")
		if err := preservationCase(t, mutated, "issueboard"); err == nil || !strings.Contains(err.Error(), "ordinary intake work missing") {
			t.Fatalf("skipped production work accepted: %v", err)
		}
	})
	t.Run("mutation_erases_budget_history", func(t *testing.T) {
		mutated := mutationBinary(t, "deskdispatch", "internal/deskkit/perf.go", "\tperfAppend(rec)\n\treturn code", "\tif p, err := auditPath(); err == nil { _ = os.Truncate(p, 0) }\n\tperfAppend(rec)\n\treturn code")
		if err := preservationCase(t, mutated, "deskdispatch"); err == nil || !strings.Contains(err.Error(), "seeded audit history changed") {
			t.Fatalf("erased production budget history accepted: %v", err)
		}
	})
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
