package main

// opmetrics_test.go — the assembled report and the CLI.
//
// HERMETIC BY CONSTRUCTION. Every test that reaches the CLI sets HOME to a temp
// directory, so no code path — not the transcripts default, not deskkit's state dir —
// can touch the real home. A test that only passes on the author's machine is not a
// test, and here it would also mean a test that reads a person's private transcripts.

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// isolateHome points HOME at a temp dir. Called by every test that runs the CLI.
func isolateHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	return home
}

// fixtureInputs is the standard Build input over the fixture tree.
func fixtureInputs(root string, trend int) Inputs {
	return Inputs{
		Date:           "2026-07-22",
		Day:            fixtureDay(),
		Now:            fixtureNow(),
		TranscriptsDir: fxTranscripts,
		DeskToolsDir:   fxDeskTools,
		GHJSON:         fxGH,
		Root:           root,
		Trend:          trend,
	}
}

func TestBuildOverFixtures(t *testing.T) {
	var stderr bytes.Buffer
	rep := Build(fixtureInputs(t.TempDir(), 0), &stderr)

	if rep.Schema != SchemaVersion || rep.ClassifierVersion != ClassifierVersion {
		t.Fatalf("schema=%q classifier=%q, want %q and %q", rep.Schema, rep.ClassifierVersion, SchemaVersion, ClassifierVersion)
	}

	// Relay ratio: 7 relays / 14 classified (one image-only turn is ClassEmpty and
	// stays OUT of the denominator).
	op := rep.Operator
	if op.Status != StatusOK {
		t.Fatalf("operator block status = %s (%s)", op.Status, op.Reason)
	}
	if *op.MessagesTotal != 15 || *op.MessagesClassified != 14 || *op.RelayMessages != 7 ||
		*op.SubstantiveMessages != 7 || *op.EmptyMessages != 1 {
		t.Fatalf("operator counts total=%d classified=%d relay=%d subst=%d empty=%d, want 15/14/7/7/1",
			*op.MessagesTotal, *op.MessagesClassified, *op.RelayMessages, *op.SubstantiveMessages, *op.EmptyMessages)
	}
	if *op.RelayRatio != 0.5 || rep.RelayRatio == nil || *rep.RelayRatio != 0.5 {
		t.Fatalf("relay ratio = %v (top-level %v), want 0.5 in both places", *op.RelayRatio, rep.RelayRatio)
	}
	want := RelayFamilies{Sync: 1, StateEcho: 1, Poke: 2, Lookup: 2, Duplicate: 1}
	if op.RelayFamilies != want {
		t.Fatalf("relay families = %+v, want %+v", op.RelayFamilies, want)
	}

	// Intervention: 15 operator messages over 5 merged PRs.
	iv := rep.Intervention
	if iv.Status != StatusOK || *iv.MergedPRs != 5 || *iv.MessagesPerMergedPR != 3 {
		t.Fatalf("intervention status=%s merged=%v rate=%v, want ok/5/3", iv.Status, iv.MergedPRs, iv.MessagesPerMergedPR)
	}

	dl := rep.DecisionLatency
	if dl.Status != StatusOK || *dl.P50Minutes != 120 || *dl.P90Minutes != 1440 {
		t.Fatalf("latency status=%s p50=%v p90=%v, want ok/120/1440", dl.Status, dl.P50Minutes, dl.P90Minutes)
	}
	if dl.Basis.CreatedAt != 2 {
		t.Fatalf("latency basis createdAt = %d, want 2 — the honesty field must survive assembly", dl.Basis.CreatedAt)
	}

	hy := rep.SessionHygiene
	if hy.Status != StatusOK || *hy.ZombieAgents != 2 || *hy.SessionsOver24h != 1 ||
		*hy.ClaimsFiled != 2 || *hy.SessionsActive != 2 || hy.UnparseableFiles != 2 {
		t.Fatalf("hygiene = %+v", hy)
	}

	cr := rep.CorrectionRecurrence
	if cr.Status != StatusOK || *cr.CorrectiveMessages != 3 || *cr.RecurrenceCandidate != 1 {
		t.Fatalf("corrections corrective=%v candidates=%v, want 3 and 1", cr.CorrectiveMessages, cr.RecurrenceCandidate)
	}

	if len(rep.Unmeasured) != 2 {
		t.Fatalf("unmeasured = %v, want the two declared codes", rep.Unmeasured)
	}
}

// TestBlindInputsReportCouldNotCheckNotZero is the three-state acceptance test. Every
// block must distinguish "nothing happened" from "I could not look". A collector that
// reports 0 relays because it could not find the transcripts reads as a perfect day.
func TestBlindInputsReportCouldNotCheckNotZero(t *testing.T) {
	var stderr bytes.Buffer
	missing := filepath.Join(t.TempDir(), "nope")
	rep := Build(Inputs{
		Date:           "2026-07-22",
		Day:            fixtureDay(),
		Now:            fixtureNow(),
		TranscriptsDir: missing,
		DeskToolsDir:   missing,
		GHJSON:         "",
		Root:           t.TempDir(),
	}, &stderr)

	if rep.RelayRatio != nil {
		t.Fatalf("top-level relay_ratio = %v with no readable transcripts, want null", *rep.RelayRatio)
	}
	checks := []struct {
		name   string
		status Status
		reason Reason
	}{
		{"operator", rep.Operator.Status, rep.Operator.Reason},
		{"intervention", rep.Intervention.Status, rep.Intervention.Reason},
		{"decision_latency", rep.DecisionLatency.Status, rep.DecisionLatency.Reason},
		{"session_hygiene", rep.SessionHygiene.Status, rep.SessionHygiene.Reason},
		{"correction_recurrence", rep.CorrectionRecurrence.Status, rep.CorrectionRecurrence.Reason},
	}
	for _, c := range checks {
		if c.status != StatusCouldNotCheck {
			t.Errorf("%s status = %s with unreadable inputs, want could-not-check", c.name, c.status)
		}
		if c.reason == ReasonNone {
			t.Errorf("%s reported could-not-check with no reason code", c.name)
		}
	}
	if rep.Operator.RelayMessages != nil {
		t.Errorf("relay_messages = %d on a blind read, want null — 0 would read as 'no relays today'", *rep.Operator.RelayMessages)
	}
	// The human-readable detail (which is where a home path would leak) belongs on
	// stderr, and it must actually be there — a silent blind run is worse than a
	// noisy one.
	if stderr.Len() == 0 {
		t.Error("a fully blind run wrote nothing to stderr")
	}
}

// TestNoMergedPRsIsNotAnInfiniteRate pins the division-by-zero decision: a day with
// operator traffic and no merges has no intervention RATE, and reporting one would be
// arithmetic on an empty denominator.
func TestNoMergedPRsIsNotAnInfiniteRate(t *testing.T) {
	dir := t.TempDir()
	gh := filepath.Join(dir, "gh.json")
	if err := os.WriteFile(gh, []byte(`[]`), 0o644); err != nil {
		t.Fatal(err)
	}
	in := fixtureInputs(dir, 0)
	in.GHJSON = gh
	rep := Build(in, &bytes.Buffer{})
	if rep.Intervention.Status != StatusCouldNotCheck || rep.Intervention.Reason != ReasonNoMergedPRs {
		t.Fatalf("intervention with zero merges = %s/%s, want could-not-check/no-merged-prs",
			rep.Intervention.Status, rep.Intervention.Reason)
	}
	if rep.Intervention.MessagesPerMergedPR != nil {
		t.Fatalf("a rate was computed over zero merged PRs: %v", *rep.Intervention.MessagesPerMergedPR)
	}
}

// TestTrendComparesOnlyLikeClassifiers is the reason ClassifierVersion exists. A prior
// day-file written by a different classifier is EXCLUDED from the mean and counted, so
// a heuristic change cannot show up as a behaviour change.
func TestTrendComparesOnlyLikeClassifiers(t *testing.T) {
	root := t.TempDir()
	writePrior := func(date, classifier string, relay, rate float64) {
		t.Helper()
		p := DayFilePath(root, date)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		body := `{"classifierVersion":"` + classifier + `","relay_ratio":` +
			trimFloat(relay) + `,"intervention":{"messages_per_merged_pr":` + trimFloat(rate) + `}}`
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	writePrior("2026-07-21", ClassifierVersion, 0.40, 2.0)
	writePrior("2026-07-20", ClassifierVersion, 0.60, 4.0)
	writePrior("2026-07-19", "opmetrics-relay/0", 0.10, 9.0) // a different ruler

	today := 0.5
	rate := 3.0
	tb := BuildTrend(root, "2026-07-22", 7, &today, &rate)
	if tb.Status != StatusOK {
		t.Fatalf("trend status = %s, want ok", tb.Status)
	}
	if tb.PriorFilesFound != 2 || tb.ClassifierMismatch != 1 {
		t.Fatalf("prior files = %d, other-classifier = %d, want 2 and 1", tb.PriorFilesFound, tb.ClassifierMismatch)
	}
	if *tb.RelayRatio.PriorMean != 0.5 || *tb.RelayRatio.Delta != 0 {
		t.Fatalf("relay trend mean=%v delta=%v, want 0.5 and 0 — the /0 file must not drag the mean",
			*tb.RelayRatio.PriorMean, *tb.RelayRatio.Delta)
	}
	if *tb.MessagesPerMergedPR.PriorMean != 3 || *tb.MessagesPerMergedPR.Delta != 0 {
		t.Fatalf("intervention trend mean=%v delta=%v, want 3 and 0",
			*tb.MessagesPerMergedPR.PriorMean, *tb.MessagesPerMergedPR.Delta)
	}
}

// TestTrendWithNoPriorFilesIsNotSteady pins the third state on the trend block: no
// comparison is "no-prior-data", never a delta of zero, which would read as "steady".
func TestTrendWithNoPriorFilesIsNotSteady(t *testing.T) {
	today := 0.5
	tb := BuildTrend(t.TempDir(), "2026-07-22", 7, &today, nil)
	if tb.Status != StatusNoPriorData || tb.Reason != ReasonNoPriorDayFiles {
		t.Fatalf("trend = %s/%s, want no-prior-data/no-prior-day-files", tb.Status, tb.Reason)
	}
	if tb.RelayRatio.Delta != nil {
		t.Fatalf("a delta of %v was reported with nothing to compare against", *tb.RelayRatio.Delta)
	}
}

func trimFloat(f float64) string {
	b, _ := json.Marshal(f)
	return string(b)
}

// TestRunWritesTheDayFile drives the CLI end to end — the same path Verify row 4 runs.
func TestRunWritesTheDayFile(t *testing.T) {
	isolateHome(t)
	root := t.TempDir()
	var stdout, stderr bytes.Buffer
	code := run([]string{
		"--transcripts", fxTranscripts,
		"--desk-tools", fxDeskTools,
		"--gh-fixture", fxGH,
		"--root", root,
		"--date", "2026-07-22",
		"--now", "2026-07-22T15:00:00Z",
		"--tz", "UTC",
	}, &stdout, &stderr)
	if code != deskkit.ExitOK {
		t.Fatalf("run exited %d: %s", code, stderr.String())
	}
	path := DayFilePath(root, "2026-07-22")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("day-file was not written: %v", err)
	}
	if !strings.Contains(stdout.String(), path) {
		t.Fatalf("run printed %q, want the day-file path", stdout.String())
	}
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("day-file is not valid JSON: %v", err)
	}
	if got["classifierVersion"] != ClassifierVersion {
		t.Fatalf("classifierVersion = %v, want %s", got["classifierVersion"], ClassifierVersion)
	}
	if got["relay_ratio"] != 0.5 {
		t.Fatalf("relay_ratio = %v, want 0.5", got["relay_ratio"])
	}
}

// TestRunRefusesBadInvocation pins the refusal codes. Exit 5 is a caller mistake, not
// a partial answer: a day-file computed from a misunderstood flag is worse than none.
func TestRunRefusesBadInvocation(t *testing.T) {
	isolateHome(t)
	cases := []struct {
		name string
		args []string
	}{
		{"no root", []string{"--date", "2026-07-22"}},
		{"bad date", []string{"--root", t.TempDir(), "--date", "22-07-2026"}},
		{"bad now", []string{"--root", t.TempDir(), "--now", "yesterday"}},
		{"bad tz", []string{"--root", t.TempDir(), "--tz", "Mars/Olympus"}},
		{"negative trend", []string{"--root", t.TempDir(), "--trend", "-1"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if code := run(c.args, &stdout, &stderr); code != deskkit.ExitRefused {
				t.Fatalf("run(%v) exited %d, want %d (refused)", c.args, code, deskkit.ExitRefused)
			}
		})
	}
}

// TestRunStdoutDoesNotWriteAnything pins that --stdout is a pure read: it must not
// create a docs/reports tree as a side effect.
func TestRunStdoutDoesNotWriteAnything(t *testing.T) {
	isolateHome(t)
	root := t.TempDir()
	var stdout, stderr bytes.Buffer
	code := run([]string{
		"--transcripts", fxTranscripts, "--desk-tools", fxDeskTools, "--gh-fixture", fxGH,
		"--root", root, "--date", "2026-07-22", "--now", "2026-07-22T15:00:00Z", "--tz", "UTC", "--stdout",
	}, &stdout, &stderr)
	if code != deskkit.ExitOK {
		t.Fatalf("run exited %d: %s", code, stderr.String())
	}
	if _, err := os.Stat(filepath.Join(root, "docs")); !os.IsNotExist(err) {
		t.Fatal("--stdout created a docs/ tree; it must be a pure read")
	}
	if !strings.Contains(stdout.String(), `"relay_ratio": 0.5`) {
		t.Fatalf("--stdout did not print the report: %q", stdout.String())
	}
}

// TestDefaultTranscriptsPathIsTheHomeTree documents the production default WITHOUT
// any test depending on it: the resolver is asked with an isolated HOME, so the
// assertion is about the shape of the path, never about the machine's real contents.
func TestDefaultTranscriptsPathIsTheHomeTree(t *testing.T) {
	home := isolateHome(t)
	got, err := resolveTranscripts("")
	if err != nil {
		t.Fatalf("resolveTranscripts: %v", err)
	}
	if got != filepath.Join(home, ".claude", "projects") {
		t.Fatalf("default transcripts dir = %q, want <home>/.claude/projects", got)
	}
	if !strings.HasPrefix(got, home) {
		t.Fatalf("the default escaped the isolated HOME: %q", got)
	}
}

// TestDefaultDeskToolsPathIsTheStateDir records the PATH CORRECTION. The brief places
// beacons and claims under ~/.claude/desk-tools/; on this tree they live under
// deskkit.StateDir() (~/.config/assay), which is what deskroster actually writes. The
// default follows the CODE.
func TestDefaultDeskToolsPathIsTheStateDir(t *testing.T) {
	home := isolateHome(t)
	got, err := resolveDeskTools("")
	if err != nil {
		t.Fatalf("resolveDeskTools: %v", err)
	}
	want := filepath.Join(home, ".config", "assay")
	if got != want {
		t.Fatalf("default desk-tools dir = %q, want %q (deskkit.StateDir, where deskroster writes)", got, want)
	}
}

// TestGeneratedAtIsStableUnderNow pins that --now drives the stamp, so two runs of the
// same command over the same day produce byte-identical output. A day-file that
// differs on every run turns a diff into noise.
func TestGeneratedAtIsStableUnderNow(t *testing.T) {
	in := fixtureInputs(t.TempDir(), 0)
	a := Build(in, &bytes.Buffer{})
	b := Build(in, &bytes.Buffer{})
	ra, err := marshalReport(a)
	if err != nil {
		t.Fatal(err)
	}
	rb, err := marshalReport(b)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(ra, rb) {
		t.Fatal("two builds of the same inputs differ")
	}
	if a.GeneratedAt != fixtureNow().UTC().Format(time.RFC3339) {
		t.Fatalf("generatedAt = %q, want the --now instant", a.GeneratedAt)
	}
}
