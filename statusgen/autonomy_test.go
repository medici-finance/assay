package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func autonomyWindow(t *testing.T) (since, until time.Time) {
	t.Helper()
	return mustTime(t, "2026-07-01T00:00:00Z"), mustTime(t, "2026-07-15T00:00:00Z")
}

// TestAutonomyClassifyAuthor: the three-way authorship bucket never guesses. An
// assay App or any other bot is loop-initiated; a roster-known individual is
// human; every other User account (the shared org account, an unknown login) is
// AMBIGUOUS.
func TestAutonomyClassifyAuthor(t *testing.T) {
	humans := map[string]bool{"realhuman": true}
	cases := []struct {
		login string
		isBot bool
		want  string
	}{
		{"assay-worker-app[bot]", true, bucketLoop},
		{"assay-reviewer-app", true, bucketLoop},
		{"github-actions[bot]", true, bucketLoop},
		{"realhuman", false, bucketHuman},
		{"the-org", false, bucketAmbiguous}, // shared account — never guessed
		{"some-unknown-user", false, bucketAmbiguous},
	}
	for _, c := range cases {
		if got := classifyAuthor(c.login, c.isBot, humans); got != c.want {
			t.Errorf("classifyAuthor(%q, bot=%v) = %q, want %q", c.login, c.isBot, got, c.want)
		}
	}
}

// TestAutonomyCIProxyBucketsAmbiguousSeparate: the CI-proxy variant computes the
// loop ratio over the loop/human/ambiguous split, and the ambiguous bucket is
// carried SEPARATELY — never folded into either side of the ratio.
func TestAutonomyCIProxyBucketsAmbiguousSeparate(t *testing.T) {
	since, until := autonomyWindow(t)
	in := autonomyInputs{
		Since: since, Until: until, Now: until,
		AuthorsOK:   true,
		HumanLogins: map[string]bool{"alice": true},
		MergedAuthors: []autonomyAuthor{
			{Login: "assay-worker-app[bot]", IsBot: true}, // loop
			{Login: "assay-worker-app[bot]", IsBot: true}, // loop
			{Login: "alice", IsBot: false},                // human
			{Login: "the-org", IsBot: false},              // ambiguous
		},
	}
	rep := computeAutonomy(in)

	if rep.Buckets.LoopInitiated != 2 || rep.Buckets.Human != 1 || rep.Buckets.Ambiguous != 1 || rep.Buckets.Total != 4 {
		t.Fatalf("buckets = %+v, want loop=2 human=1 ambiguous=1 total=4", rep.Buckets)
	}
	proxy := rep.Axes[0]
	if !proxy.Measured {
		t.Fatalf("ci-proxy axis not measured: %+v", proxy)
	}
	// 2 loop / 4 total = 50% — the ambiguous unit is in the DENOMINATOR but not
	// the loop numerator, and is never reassigned to human.
	if !strings.Contains(proxy.Value, "50% loop-initiated") {
		t.Errorf("ci-proxy value = %q, want 50%% loop-initiated", proxy.Value)
	}
	if !strings.Contains(proxy.Detail, "ambiguous=1") || !strings.Contains(proxy.Detail, "NOT folded") {
		t.Errorf("ci-proxy detail must carry the separate ambiguous bucket: %q", proxy.Detail)
	}
}

// TestAutonomyCIProxyUnmeasuredWhenGHDown: gh down → the CI-proxy axis is
// "unmeasured" with a reason, never a fabricated 0% loop-initiated.
func TestAutonomyCIProxyUnmeasuredWhenGHDown(t *testing.T) {
	since, until := autonomyWindow(t)
	rep := computeAutonomy(autonomyInputs{Since: since, Until: until, Now: until, AuthorsOK: false})
	proxy := rep.Axes[0]
	if proxy.Measured || proxy.Value != "unmeasured" || proxy.Reason == "" {
		t.Errorf("ci-proxy with gh down = %+v, want unmeasured + reason, never zero", proxy)
	}
	if strings.Contains(proxy.Value, "0%") {
		t.Errorf("ci-proxy fabricated a zero: %q", proxy.Value)
	}
}

// TestAutonomyGateShareSplit: the deterministic-gate share renders the split —
// code-gated vs model-judgment-only — and never collapses it.
func TestAutonomyGateShareSplit(t *testing.T) {
	since, until := autonomyWindow(t)
	in := autonomyInputs{
		Since: since, Until: until, Now: until,
		GateOK: true,
		GateData: []autonomyGatePR{
			{Number: 1, CheckNames: []string{"statusgen --lint", "go-test"}}, // deterministic
			{Number: 2, CheckNames: []string{"leak-sweep"}},                  // deterministic
			{Number: 3, CheckNames: []string{}},                              // model-judgment only
			{Number: 4, CheckNames: []string{"assay-reviewer-app"}},          // no code gate → model only
		},
	}
	gate := computeAutonomy(in).Axes[3]
	if !gate.Measured {
		t.Fatalf("gate-share not measured: %+v", gate)
	}
	// 2/4 = 50% deterministic-gated.
	if !strings.Contains(gate.Value, "50% deterministic-gated") {
		t.Errorf("gate-share value = %q, want 50%% deterministic-gated", gate.Value)
	}
	if !strings.Contains(gate.Detail, "2/4") || !strings.Contains(gate.Detail, "model-judgment") {
		t.Errorf("gate-share must render the split: %q", gate.Detail)
	}
}

// TestDeterministicGateCallout: the generic built-in set ships with no house
// gate name, and a house sets ASSAY_DETERMINISTIC_GATE_PATTERNS to MERGE its own
// gate name(s) on top. Unset = generic-only; set = generic + house patterns.
func TestDeterministicGateCallout(t *testing.T) {
	// Unset (the shipped/public posture): a house-specific gate name is NOT
	// classified deterministic, while a generic gate name still is.
	os.Unsetenv(EnvDeterministicGatePatterns)
	if hasDeterministicGate([]string{"house-ci"}) {
		t.Errorf("unset: a house gate name must not be a deterministic gate (generic-only default)")
	}
	if !hasDeterministicGate([]string{"go-test"}) {
		t.Errorf("unset: generic go-test must still be a deterministic gate")
	}
	if got := additionalDeterministicGatePatterns(); got != nil {
		t.Errorf("unset: additional patterns = %v, want nil", got)
	}

	// Set: the house value is honored (merged), generic patterns still apply.
	t.Setenv(EnvDeterministicGatePatterns, "house-ci")
	if !hasDeterministicGate([]string{"house-ci"}) {
		t.Errorf("set: the configured house gate name must be classified deterministic")
	}
	if !hasDeterministicGate([]string{"go-test"}) {
		t.Errorf("set: generic go-test must still be a deterministic gate")
	}
	if hasDeterministicGate([]string{"assay-reviewer-app"}) {
		t.Errorf("set: an unrelated model-judgment check must stay non-deterministic")
	}

	// Parsing tolerates comma/whitespace separation and lower-cases entries.
	t.Setenv(EnvDeterministicGatePatterns, " House-CI, other-gate ")
	got := additionalDeterministicGatePatterns()
	want := map[string]bool{"house-ci": true, "other-gate": true}
	if len(got) != len(want) {
		t.Fatalf("parsed = %v, want keys %v", got, want)
	}
	for _, g := range got {
		if !want[g] {
			t.Errorf("unexpected parsed pattern %q", g)
		}
	}
}

// TestAutonomyTokenEffMeasuredFromDayFile: when the day-file carries a token
// block, token efficiency is computed (tokens/merged PR + no-op rate).
func TestAutonomyTokenEffMeasuredFromDayFile(t *testing.T) {
	since, until := autonomyWindow(t)
	subTokens, mergedPRs := 30000, 3
	noop, total := 1, 5
	df := &opDayFile{
		Schema: "opmetrics/1",
		TokenEfficiency: &struct {
			SubagentTokens  *int `json:"subagent_tokens"`
			MergedPRs       *int `json:"merged_prs"`
			NoopDispatches  *int `json:"noop_dispatches"`
			TotalDispatches *int `json:"total_dispatches"`
		}{SubagentTokens: &subTokens, MergedPRs: &mergedPRs, NoopDispatches: &noop, TotalDispatches: &total},
	}
	in := autonomyInputs{Since: since, Until: until, Now: until, DayFile: df, DayFileDate: "2026-07-14"}
	tok := computeAutonomy(in).Axes[2]
	if !tok.Measured {
		t.Fatalf("token efficiency not measured: %+v", tok)
	}
	if !strings.Contains(tok.Value, "10000 subagent tokens/merged PR") {
		t.Errorf("token value = %q, want 10000 tokens/merged PR", tok.Value)
	}
	if !strings.Contains(tok.Value, "20% no-op dispatch rate") {
		t.Errorf("token value = %q, want 20%% no-op rate", tok.Value)
	}
}

// TestAutonomyDayFileAxesUnmeasuredWhenAbsent: with NO day-file, both day-file
// axes (dispatch-level autonomy + token efficiency) degrade to "unmeasured" —
// never a fabricated zero. This is the mm/41 honest-degrade contract.
func TestAutonomyDayFileAxesUnmeasuredWhenAbsent(t *testing.T) {
	since, until := autonomyWindow(t)
	in := autonomyInputs{Since: since, Until: until, Now: until, DayFile: nil}
	rep := computeAutonomy(in)
	for _, i := range []int{1, 2} { // dispatch autonomy, token efficiency
		a := rep.Axes[i]
		if a.Measured || a.Value != "unmeasured" || a.Reason == "" {
			t.Errorf("axis %q with no day-file = %+v, want unmeasured + reason", a.Key, a)
		}
		if strings.Contains(a.Value, "0") {
			t.Errorf("axis %q fabricated a zero: %q", a.Key, a.Value)
		}
	}
}

// TestAutonomyDispatchMeasuredFromDayFile: a day-file carrying the routine/operator
// dispatch split yields the dispatch-level autonomy ratio.
func TestAutonomyDispatchMeasuredFromDayFile(t *testing.T) {
	since, until := autonomyWindow(t)
	loop, op := 8, 2
	df := &opDayFile{
		Schema: "opmetrics/1",
		Dispatch: &struct {
			LoopInitiated     *int `json:"loop_initiated"`
			OperatorInitiated *int `json:"operator_initiated"`
		}{LoopInitiated: &loop, OperatorInitiated: &op},
	}
	in := autonomyInputs{Since: since, Until: until, Now: until, DayFile: df, DayFileDate: "2026-07-14"}
	disp := computeAutonomy(in).Axes[1]
	if !disp.Measured || !strings.Contains(disp.Value, "80% loop-initiated") {
		t.Errorf("dispatch autonomy = %+v, want 80%% loop-initiated", disp)
	}
}

// TestAutonomyExactlyFourAxes: the report always carries exactly the four gauges,
// in order, each self-citing its source.
func TestAutonomyExactlyFourAxes(t *testing.T) {
	since, until := autonomyWindow(t)
	rep := computeAutonomy(autonomyInputs{Since: since, Until: until, Now: until})
	if len(rep.Axes) != 4 {
		t.Fatalf("got %d axes, want exactly 4", len(rep.Axes))
	}
	wantKeys := []string{"autonomy_ci_proxy", "autonomy_dispatch", "token_efficiency", "deterministic_gate_share"}
	for i, k := range wantKeys {
		if rep.Axes[i].Key != k {
			t.Errorf("axis %d = %q, want %q", i, rep.Axes[i].Key, k)
		}
		if rep.Axes[i].Source == "" {
			t.Errorf("axis %q is not self-citing (empty source)", k)
		}
	}
	if !strings.Contains(rep.Note, "Goodhart") {
		t.Error("report note missing the anti-gaming Goodhart clause")
	}
}

// TestLoadOpDayFilePicksMostRecent: the day-file loader picks the most recent
// day dir on or before `until` and parses it.
func TestLoadOpDayFilePicksMostRecent(t *testing.T) {
	root := t.TempDir()
	write := func(date, body string) {
		dir := filepath.Join(root, "docs", "reports", "daily", date)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "opmetrics.json"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("2026-07-10", `{"schema":"opmetrics/1","dispatch":{"loop_initiated":1,"operator_initiated":1}}`)
	write("2026-07-14", `{"schema":"opmetrics/1","dispatch":{"loop_initiated":9,"operator_initiated":1}}`)
	write("2026-07-20", `{"schema":"opmetrics/1"}`) // after `until`, must be ignored

	df, date := loadOpDayFile(root, mustTime(t, "2026-07-15T00:00:00Z"))
	if df == nil {
		t.Fatal("expected a day-file, got nil")
	}
	if date != "2026-07-14" {
		t.Errorf("picked date %q, want 2026-07-14 (most recent ≤ until)", date)
	}
	if df.Dispatch == nil || *df.Dispatch.LoopInitiated != 9 {
		t.Errorf("parsed wrong day-file: %+v", df.Dispatch)
	}
}

// TestRunAutonomyExitsZeroAndNeverSilentZero is Verify item 3: `--autonomy`
// exits 0, its output contains "autonomy", and a missing source reads
// "unmeasured" — never a silent zero.
func TestRunAutonomyExitsZeroAndNeverSilentZero(t *testing.T) {
	// Force every source offline/absent so the whole report degrades.
	oa, og := autonomyMergedAuthors, autonomyGates
	autonomyMergedAuthors = func(root string, since, until time.Time) ([]autonomyAuthor, bool) { return nil, false }
	autonomyGates = func(root string, since, until time.Time) ([]autonomyGatePR, bool) { return nil, false }
	t.Cleanup(func() { autonomyMergedAuthors, autonomyGates = oa, og })
	fixedNow(t, "2026-07-15T00:00:00Z")

	out := captureStdout(t, func() {
		if code := runAutonomy(t.TempDir(), "2026-07-01", false); code != 0 {
			t.Fatalf("runAutonomy exited %d, want 0", code)
		}
	})
	if !strings.Contains(strings.ToLower(out), "autonomy") {
		t.Errorf("output missing 'autonomy':\n%s", out)
	}
	if !strings.Contains(out, "unmeasured") {
		t.Errorf("all-sources-absent output must say 'unmeasured', not a silent zero:\n%s", out)
	}
}

// TestRunAutonomyJSONShape: `--autonomy --json` emits valid JSON with four axes
// and the anti-gaming note.
func TestRunAutonomyJSONShape(t *testing.T) {
	oa, og := autonomyMergedAuthors, autonomyGates
	autonomyMergedAuthors = func(root string, since, until time.Time) ([]autonomyAuthor, bool) {
		return []autonomyAuthor{{Login: "assay-worker-app[bot]", IsBot: true}}, true
	}
	autonomyGates = func(root string, since, until time.Time) ([]autonomyGatePR, bool) {
		return []autonomyGatePR{{Number: 1, CheckNames: []string{"go-test"}}}, true
	}
	t.Cleanup(func() { autonomyMergedAuthors, autonomyGates = oa, og })
	fixedNow(t, "2026-07-15T00:00:00Z")

	out := captureStdout(t, func() {
		if code := runAutonomy(t.TempDir(), "2026-07-01", true); code != 0 {
			t.Fatalf("runAutonomy exited %d, want 0", code)
		}
	})
	var rep AutonomyReport
	if err := json.Unmarshal([]byte(out), &rep); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, out)
	}
	if len(rep.Axes) != 4 {
		t.Errorf("JSON has %d axes, want 4", len(rep.Axes))
	}
	if rep.Note == "" {
		t.Error("JSON missing anti-gaming note")
	}
}
