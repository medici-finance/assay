package reviewtest_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	rf "github.com/medici-finance/assay/tools/desk/internal/reviewflow"
	q "github.com/medici-finance/assay/tools/desk/internal/reviewtest"
)

// The script is the shipped prose/watcher recipe; reviewloop is the discrete
// bridge. Only the producer transport is substituted, using an artifact collected
// by the real deskboard collector against the qualification HTTP transport.
func TestReviewDriverParity(t *testing.T) {
	got := collectQueue(t, "throughput-normal", 3)
	if got.Exit != 0 || got.Facts["depth"] != "3" || got.Facts["depthError"] != "<nil>" || got.Facts["depthAdditionalHTTP"] != "0" {
		t.Fatal("depth reuse failed", got)
	}
	legacy := collectQueue(t, "legacy-cycle", 3)
	if legacy.Exit != 0 || len(got.Attempts) != 7 || len(legacy.Attempts) != 14 {
		t.Fatal("complete-cycle request saving not observed", len(legacy.Attempts), len(got.Attempts), legacy)
	}
	t.Logf("matched fixture: legacy event + actions + prs + review depth = %d HTTP requests; shared queue + offline plan + reused depth = %d", len(legacy.Attempts), len(got.Attempts))
	o, err := rf.DecodeObservation(got.Snapshot)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".config", "assay"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".config", "assay", "roster.env"), []byte("ASSAY_BLESS_LOGIN=ada:2001\nASSAY_TRUSTED_LOGINS=ada:2001\nASSAY_ALLOWED_REPOS=example-org/tracker:ci:private\n"), 0600); err != nil {
		t.Fatal(err)
	}
	fixture := filepath.Join(dir, "fixture.json")
	if err := os.WriteFile(fixture, got.Snapshot, 0600); err != nil {
		t.Fatal(err)
	}
	cli := filepath.Join(dir, "reviewloop-real")
	cmd := exec.Command("go", "build", "-o", cli, "./cmd/reviewloop")
	cmd.Dir = moduleRoot(t)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build: %v %s", err, out)
	}
	write := func(name, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\nset -eu\n"+body), 0700); err != nil {
			t.Fatal(err)
		}
	}
	write("gh", "echo legacy-GH-called >> \"$CALL_LOG\"; exit 99\n")
	write("deskboard", `if [ "${1:-}" = --version ]; then echo 'deskboard releaseTag=v1.0.59'; exit; fi
printf 'collect\n' >> "$CALL_LOG"
[ "$1" = review-queue ] && [ "$2" = --out ]
[ "${HELD:-0}" != 1 ] || exit 6
cp "$FIXTURE" "$3"
`)
	write("reviewloop", `if [ "${1:-}" = --version ]; then echo 'reviewloop releaseTag=v1.0.59'; exit; fi
exec "$REAL_PLANNER" "$@"
`)
	policy := rf.SelectionPolicy{Owner: "fixture-current-policy", Revision: "priority-r7", Evaluator: rf.SelectionEvaluator, Unmapped: rf.UseObservedScore, Priorities: []rf.PriorityEntry{{Subject: rf.PrioritySubject{Repository: o.Inputs.Requested[0], Number: 3}, Score: 5000}}}
	raw, _ := json.Marshal(policy)
	req := queueRequest(t, got.Snapshot, 1)
	var inv map[string]any
	_ = json.Unmarshal([]byte(req.Selection), &inv)
	inv["Policy"] = policy
	sel, _ := json.Marshal(inv)
	req.Selection = string(sel)
	expected, discrete := planQueue(t, req)
	if discrete.Exit != 0 {
		t.Fatal(discrete)
	}
	script := filepath.Join(moduleRoot(t), "..", "..", "plugins", "assay", "scripts", "pr-monitor.sh")
	dest := filepath.Join(dir, "current.json")
	log := filepath.Join(dir, "calls")
	run := func(held bool) (string, error) {
		c := exec.Command("bash", script, "example-org/tracker")
		h := "0"
		if held {
			h = "1"
		}
		c.Env = append(isolatedEnv(dir), "PATH="+dir+string(os.PathListSeparator)+os.Getenv("PATH"), "FIXTURE="+fixture, "REAL_PLANNER="+cli, "CALL_LOG="+log, "PR_MONITOR_STATE_DIR="+filepath.Join(dir, "state"), "PR_MONITOR_REVIEW_OBSERVATION="+dest, "PR_MONITOR_REVIEW_POLICY="+string(raw), "PR_MONITOR_REVIEW_CAPACITY=1", "HELD="+h)
		b, e := c.CombinedOutput()
		return string(b), e
	}
	output, e := run(false)
	if e != nil {
		t.Fatal("prose monitor failed", e, output)
	}
	var actual rf.SelectionPlan
	found := false
	for _, line := range strings.Split(output, "\n") {
		if strings.HasPrefix(line, "REVIEW-PLAN: ") {
			found = true
			raw, err := os.ReadFile(strings.TrimPrefix(line, "REVIEW-PLAN: "))
			if err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(raw, &actual); err != nil {
				t.Fatal(err)
			}
		}
	}
	if !found || !reflect.DeepEqual(actual.Candidates, expected.Candidates) || actual.Candidates[0].Row.Identity.Number != 3 || actual.Candidates[0].Disposition != rf.SelectionSelected || !reflect.DeepEqual(actual.Depths, expected.Depths) {
		t.Fatal("prose/discrete priority parity lost", output, expected)
	}
	// Feed the actual watcher artifact to the production preparation ceremony.
	handed, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	prepared, _ := probe(t, build(t), "deskdispatch", q.Request{Case: "caller-prepare", Snapshot: handed, Size: 3}, t.TempDir())
	if prepared.Exit != 0 || prepared.Facts["head"] != actual.Candidates[0].Row.Identity.Head || prepared.Facts["prReads"] != "2" || !strings.Contains(prepared.Output, "Packet:") {
		t.Fatal("watcher preparation handoff failed", prepared)
	}
	moved, _ := probe(t, build(t), "deskdispatch", q.Request{Case: "caller-prepare-moved", Snapshot: handed, Size: 3}, t.TempDir())
	if moved.Exit != 6 || moved.Facts["stamps"] != "0" || moved.Facts["queueLabels"] != "0" {
		t.Fatal("moved watcher head escaped final admission", moved)
	}
	// A failed whole-cycle collection cannot publish a stale valid observation or
	// advance the event baseline. No same-cycle legacy fallback is permitted.
	baseline := filepath.Join(dir, "state", "example-org__tracker.state")
	before, _ := os.ReadFile(baseline)
	output, e = run(true)
	if e == nil || !strings.Contains(output, "MONITOR-DEGRADED") {
		t.Fatal("failed watcher appeared healthy", e, output)
	}
	b, _ := os.ReadFile(dest)
	if _, err := rf.DecodeObservation(b); err == nil {
		t.Fatal("failed watcher retained reusable observation")
	}
	after, _ := os.ReadFile(baseline)
	if !reflect.DeepEqual(before, after) {
		t.Fatal("failed cycle advanced baseline")
	}
	output, e = run(false)
	if e != nil || !strings.Contains(output, "REVIEW-OBSERVATION:") {
		t.Fatal("recovery failed", e, output)
	}
	calls, _ := os.ReadFile(log)
	if string(calls) != "collect\ncollect\ncollect\n" {
		t.Fatal("duplicate or legacy collection", string(calls))
	}
	// Planner scope/time mismatch stays a hold, including explicit current scope.
	for _, kind := range []string{"scope", "stale"} {
		bad := req
		var v map[string]any
		_ = json.Unmarshal([]byte(req.Selection), &v)
		if kind == "scope" {
			v["Scope"] = []rf.Repository{}
		} else {
			v["Oldest"] = o.Inputs.Collection.End.Add(time.Second)
		}
		b, _ := json.Marshal(v)
		bad.Selection = string(b)
		_, r := planQueue(t, bad)
		if r.Exit == 0 {
			t.Fatal("caller invalidation became success", kind)
		}
	}
}

func TestReviewCallerFaults(t *testing.T) {
	for _, kind := range []string{"untrusted", "untrusted-public", "partial", "moved-head", "denied"} {
		t.Run(kind, func(t *testing.T) {
			held := collectQueue(t, kind, 2)
			if held.Exit != 6 || len(held.Snapshot) != 0 {
				t.Fatal("incomplete population escaped hold", kind, held)
			}
			recovered := collectQueue(t, "normal", 2)
			o, err := rf.DecodeObservation(recovered.Snapshot)
			if recovered.Exit != 0 || err != nil || o.Result.Depths.Total != 2 {
				t.Fatal("held population not fully recovered", recovered, err)
			}
		})
	}
	if os.Getenv("ASSAY_REVIEW_CONTRACT_MUTANT") != "1" {
		contractMutation(t, "cmd/deskboard/throughput.go", `if !reflect.DeepEqual(scope, o.Inputs.Requested) {`, `if false && !reflect.DeepEqual(scope, o.Inputs.Requested) {`, "^TestReviewDriverParity$", "invalid frozen review depth accepted")
		contractMutation(t, "cmd/deskboard/throughput.go", `Oldest: now.Add(-5 * time.Minute)`, `Oldest: now.Add(-5 * time.Hour)`, "^TestReviewDriverParity$", "invalid frozen review depth accepted")
		// A refusal from a DIFFERENT check (here the provenance cutoff) must not
		// satisfy the stale case: each applicability check is asserted by cause.
		contractMutation(t, "internal/reviewflow/observation.go", `if c.Start.Before(policy.Oldest) {`, `if false && c.Start.Before(policy.Oldest) {`, "^TestReviewDriverParity$", "wrong frozen depth refusal for stale")
		contractMutation(t, "internal/reviewflow/selection.go", `return a.Priority > b.Priority`, `return a.Priority < b.Priority`, "^TestReviewDriverParity$", "prose/discrete priority parity lost")
	}
}
