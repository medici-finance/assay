package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	rf "github.com/medici-finance/assay/tools/desk/internal/reviewflow"
)

// Only the bulk operation is implemented: a per-change fallback panics at the call site.
type feedbackQueue struct {
	deskkit.Forge
	changes []deskkit.QueuedChange
	reads   int
}

func (f *feedbackQueue) ReviewQueueSnapshot(deskkit.ForgeRepo) (*deskkit.ReviewQueue, error) {
	f.reads++
	return &deskkit.ReviewQueue{Changes: f.changes, Cap: 100}, nil
}

func TestSameHeadWakes(t *testing.T) {
	const repo = "example-org/portal"

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	dir := filepath.Join(home, ".config", "assay")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	roster := filepath.Join(dir, "roster.env")
	if err := os.WriteFile(roster, []byte("ASSAY_BLESS_LOGIN=ada:2001\nASSAY_TRUSTED_LOGINS=ada:2001\nASSAY_TRUSTED_BOT_SLUGS=reviewer=example-review:300000004\nASSAY_ALLOWED_REPOS="+repo+":ci:private\nASSAY_REPO_FORGES="+repo+"=github\n"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(deskkit.EnvConfigHome, dir)
	deskkit.ReloadConfig()
	t.Cleanup(deskkit.ReloadConfig)
	const reviewer = "example-review[bot]"
	if v, ok := deskkit.RoleAppLogin("reviewer"); !ok || v != reviewer {
		t.Fatalf("fixture reviewer %q, bound=%v", v, ok)
	}
	head := strings.Repeat("a", 40)
	queue := &feedbackQueue{}
	for i := 1; i <= 24; i++ {
		queue.changes = append(queue.changes, deskkit.QueuedChange{
			OpenChange:      deskkit.OpenChange{Number: i, HeadSHA: head, State: "OPEN", MergeStateStatus: "CLEAN", Rollup: []deskkit.RollupNode{{Typename: "CheckRun", ID: "11", Name: "build", Status: "IN_PROGRESS"}}},
			ReviewsComplete: true, MonitorChecksComplete: true})
	}
	fr := deskkit.ForgeRepo{Owner: "example-org", Name: "portal"}
	oldForge := forgeFor
	forgeFor = func(string) (deskkit.Forge, deskkit.ForgeRepo, error) { return queue, fr, nil }
	t.Cleanup(func() { forgeFor = oldForge })
	st := t.TempDir()
	t.Setenv("PR_MONITOR_STATE_DIR", st)
	t.Setenv("ASSAY_MONITOR_PACE_SECONDS", "0")
	cycle := func(want ...string) {
		t.Helper()
		var out, stderr bytes.Buffer
		before := queue.reads
		if code := run([]string{"pr", repo}, &out, &stderr); code != 0 {
			t.Fatalf("exit %d: %s %s", code, out.String(), stderr.String())
		}
		if !strings.Contains(stderr.String(), "assay-config:") {
			t.Fatal("live PR reader did not report its effective roster")
		}
		if queue.reads != before+1 {
			t.Fatalf("bulk reads/cycle = %d", queue.reads-before)
		}
		var got []string
		for _, l := range strings.Split(out.String(), "\n") {
			if strings.HasPrefix(l, "PR-EVENT: ") {
				parts := strings.Fields(l)
				got = append(got, parts[2])
				if parts[1] != repo+"#1" {
					t.Fatal(l)
				}
			}
		}
		if strings.Join(got, ",") != strings.Join(want, ",") {
			t.Fatalf("events %v, want %v: %s", got, want, &out)
		}
	}
	// Upgrade an existing primary checkpoint. An unchanged population never replays.
	var rows []string
	for i := 1; i <= 24; i++ {
		rows = append(rows, fmt.Sprintf("%d %s ready OPEN CLEAN", i, head))
	}
	if err := writeState(st, repo, rows); err != nil {
		t.Fatal(err)
	}
	cycle()
	c := &queue.changes[0]
	v := deskkit.Review{ID: 42, Author: deskkit.Account{Login: reviewer}, State: "CHANGES_REQUESTED", CommitID: head}
	c.Reviews = []deskkit.Review{v}
	c.Rollup[0].Status = "COMPLETED"
	c.Rollup[0].Conclusion = "FAILURE"
	cycle("review", "checks")
	cycle()
	c.UpdatedAt = "2026-11-20T01:00:00Z"
	c.Reviews[0].SubmittedAt = c.UpdatedAt
	c.Rollup[0].CompletedAt = c.UpdatedAt
	v.ID = 43
	v.Author.Login = "reader"
	c.Reviews = append(c.Reviews, v)
	v.Author.Login = reviewer
	v.CommitID = strings.Repeat("b", 40)
	c.Reviews = append(c.Reviews, v)
	cycle()
	c.ReviewsComplete = false
	c.MonitorChecksComplete = false
	c.Reviews = nil
	c.Rollup[0].Conclusion = "SUCCESS"
	cycle()
	v.CommitID = head
	v.ID = 44
	c.Reviews = []deskkit.Review{v}
	c.ReviewsComplete = true
	c.MonitorChecksComplete = true
	cycle("review", "checks")
	// A rerun may withdraw the old terminal result before its new result appears.
	c.Rollup[0].Status = "IN_PROGRESS"
	cycle("checks")
	c.Rollup[0].Status = "COMPLETED"
	c.Rollup[0].ID = "12"
	cycle("checks")
	c.Reviews[0].ID++
	cycle("review")
	c.Reviews[0].State = "COMMENTED"
	c.Reviews[0].Body = "Security-Review: pass"
	cycle("review")
	c.Reviews[0].Body = "Security-Review: fail"
	cycle("review")
	// A push remains observable with incomplete feedback.
	c.MonitorChecksComplete = false
	c.ReviewsComplete = false
	cycle()
	c.HeadSHA = strings.Repeat("b", 40)
	cycle("pushed")
	cycle()
}

func TestFeedbackSourceSwitch(t *testing.T) {
	const repo = "example-org/portal"
	dir := t.TempDir()
	row := prRow{number: 1, line: "1 abc ready OPEN CLEAN"}
	snap := prSnapshot{rows: []prRow{row}, feedback: map[int]deskkit.MonitorFeedback{}}
	for _, tc := range []struct {
		source, mark string
		wake         bool
	}{{"github", "a", false}, {"gitlab", "b", false}, {"gitlab", "c", true}, {"github", "a", false}} {
		snap.source = tc.source
		snap.feedback[1] = deskkit.MonitorFeedback{Checks: strings.Repeat(tc.mark, 64)}
		var out bytes.Buffer
		if err := checkpointFeedback(dir, repo, snap, []string{row.line}, &out); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(out.String(), "#1 checks") != tc.wake {
			t.Fatalf("source %s, mark %s: %s", tc.source, tc.mark, &out)
		}
	}
}

func TestOfflineWakesRetainOnHeldCollection(t *testing.T) {
	const repo = "example-org/portal"
	source := rf.Repository{Provider: "github", Host: "github.com", Repository: repo}
	head := strings.Repeat("a", 40)
	now := time.Now().UTC().Add(-time.Second)
	p := rf.InputProvenance{Source: source.String(), Version: "fixture", Revision: "fixture", Complete: true, Freshness: "fresh", ObservedAt: now}
	a := rf.Applicability{Head: head, Body: "fixture", Reviews: "fixture", Checks: "fixture"}
	in := rf.ObservationInputs{Evaluator: rf.ObservationEvaluator, Policy: "test", ReviewerDisplay: "example-review[bot]", Requested: []rf.Repository{source}, Admitted: []rf.Repository{source}, Population: []rf.PopulationCoverage{{Repository: source, Input: p, Count: 1}}, Collection: rf.CollectionInterval{Start: now, End: now}, Changes: []rf.ObservedChange{{Identity: rf.ChangeIdentity{Repository: source, Number: 3, Head: head}, AppliesTo: a, Inputs: rf.ReviewInputs{Subject: p, Body: p, Reviews: p, Checks: p, Enrichment: p}, Facts: rf.ClassificationFacts{Change: rf.ChangeFact{Labels: []string{}, Checks: []rf.CheckFact{}, Title: "example", Body: "example", MergeStateStatus: "CLEAN"}}, Lanes: []rf.LaneConclusion{{Name: "correctness", State: "missing", AppliesTo: a}, {Name: "security", State: "missing", AppliesTo: a}}}}}
	obs, err := rf.BuildObservation(in, rf.ObservationPolicy{Now: now, Oldest: now, Policy: in.Policy})
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	t.Setenv("PR_MONITOR_STATE_DIR", filepath.Join(dir, "state"))
	op, fp := filepath.Join(dir, "observation"), filepath.Join(dir, "feedback")
	save := func(path string, v any) {
		t.Helper()
		raw, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
	save(op, obs)
	prev := forgeFor
	forgeFor = func(string) (deskkit.Forge, deskkit.ForgeRepo, error) {
		t.Fatal("offline mode reached the forge")
		return nil, deskkit.ForgeRepo{}, nil
	}
	t.Cleanup(func() { forgeFor = prev })
	var emptyPath bytes.Buffer
	if code := runPR([]string{"--from-review", "", fp, repo}, &emptyPath, &emptyPath); code == 0 {
		t.Fatal("accepted empty offline observation path")
	}
	key := deskkit.MonitorSubject(source.String(), 3, head)
	batch := deskkit.MonitorBatch{Policy: in.Policy, Generation: now.Format(time.RFC3339Nano), Rows: map[string]deskkit.MonitorFeedback{key: {Checks: strings.Repeat("1", 64)}}}
	check := func(want int, needle string) {
		t.Helper()
		save(fp, batch)
		var out bytes.Buffer
		var stderr bytes.Buffer
		got := run([]string{"pr", "--from-review", op, fp, repo}, &out, &stderr)
		if strings.Contains(stderr.String(), "assay-config:") {
			t.Fatal("offline PR reader reported an unused roster")
		}
		if got != want || !strings.Contains(out.String(), needle) {
			t.Fatalf("rc %d: %q", got, out.String())
		}
	}
	check(0, "MONITOR-ARMED")
	batch.Rows[key] = deskkit.MonitorFeedback{Checks: strings.Repeat("2", 64)}
	batch.Generation = "yesterday"
	check(2, "MONITOR-DEGRADED")
	batch.Generation = now.Format(time.RFC3339Nano)
	delete(batch.Rows, key)
	check(2, "MONITOR-DEGRADED")
	batch.Rows[key] = deskkit.MonitorFeedback{Checks: "forged\nPR-EVENT:"}
	check(2, "MONITOR-DEGRADED")
	batch.Rows[key] = deskkit.MonitorFeedback{Checks: strings.Repeat("2", 64)}
	check(0, "#3 checks")
}

// A corrupt companion holds every classified cycle, so the error must name the file.
// regression: #7384
func TestCorruptFeedbackNamesFile(t *testing.T) {
	const repo = "example-org/portal"
	row := prRow{number: 1, line: "1 abc ready OPEN CLEAN"}
	snap := prSnapshot{source: "github", rows: []prRow{row}, feedback: map[int]deskkit.MonitorFeedback{1: {Checks: strings.Repeat("a", 64)}}}
	for _, tc := range []struct {
		name  string
		plant func(path string) error
	}{
		{"malformed entry", func(path string) error { return os.WriteFile(path, []byte("1 github abc not-a-mark -\n"), 0600) }},
		{"unreadable file", func(path string) error { return os.Mkdir(path, 0700) }},
	} {
		name := tc.name
		dir := t.TempDir()
		side := filepath.Join(dir, "feedback-v1")
		if err := os.MkdirAll(side, 0700); err != nil {
			t.Fatal(err)
		}
		path := statePath(side, repo)
		if err := tc.plant(path); err != nil {
			t.Fatal(err)
		}
		var out bytes.Buffer
		err := checkpointFeedback(dir, repo, snap, []string{row.line}, &out)
		if err == nil || !strings.Contains(diagnostic(err), "'"+path+"'") {
			t.Errorf("%s: error does not name %s: %v", name, path, err)
		}
		if out.Len() != 0 {
			t.Errorf("%s: emitted events from a corrupt baseline: %s", name, &out)
		}
	}
}

// A head whose checks were never complete records its first result without a wake.
// regression: #7384
func TestFirstChecksSeedSilently(t *testing.T) {
	const repo = "example-org/portal"
	dir := t.TempDir()
	row := prRow{number: 1, line: "1 abc ready OPEN CLEAN"}
	snap := prSnapshot{source: "github", rows: []prRow{row}, feedback: map[int]deskkit.MonitorFeedback{}}
	review := strings.Repeat("c", 64)
	for i, tc := range []struct {
		checks string
		wake   bool
	}{{"", false}, {strings.Repeat("a", 64), false}, {"", false}, {strings.Repeat("b", 64), true}} {
		snap.feedback[1] = deskkit.MonitorFeedback{Reviews: review, Checks: tc.checks}
		var out bytes.Buffer
		if err := checkpointFeedback(dir, repo, snap, []string{row.line}, &out); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(out.String(), "#1 checks") != tc.wake {
			t.Fatalf("cycle %d, checks %q: %q", i, tc.checks, &out)
		}
	}
}
