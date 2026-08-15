package main

// readers_test.go — the three input readers, each driven against the fixture tree.
//
// Every test here builds its day in time.UTC. The day boundary is a local-time
// question at runtime (--tz pins it), but a TEST whose answer depends on the machine's
// zone is a test that passes in one office and fails in another, which is the same
// defect as a test that reads the real home directory.

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

const (
	fxTranscripts = "testdata/transcripts"
	fxDeskTools   = "testdata/desk-tools"
	fxGH          = "testdata/gh.json"
)

func fixtureDay() time.Time { return time.Date(2026, 7, 22, 0, 0, 0, 0, time.UTC) }
func fixtureNow() time.Time { return time.Date(2026, 7, 22, 15, 0, 0, 0, time.UTC) }

// TestReadOperatorMessagesFiltersHarnessTraffic is the load-bearing filter test. A
// transcript's type:"user" records are mostly NOT the human — tool results, subagent
// traffic, injected meta context. Counting those would inflate the denominator by an
// order of magnitude and make the relay ratio meaningless.
func TestReadOperatorMessagesFiltersHarnessTraffic(t *testing.T) {
	got, err := ReadOperatorMessages(fxTranscripts, fixtureDay())
	if err != nil {
		t.Fatalf("ReadOperatorMessages: %v", err)
	}
	if got.Files != 2 {
		t.Fatalf("files read = %d, want 2", got.Files)
	}
	if len(got.Messages) != 15 {
		t.Fatalf("operator messages = %d, want 15 (the fixture carries a tool_result, a "+
			"sidechain turn, an isMeta turn, assistant turns and a previous-day turn that must "+
			"ALL be excluded)", len(got.Messages))
	}
	if got.Unparseable != 1 {
		t.Fatalf("unparseable lines = %d, want 1 — a half-read transcript must not read as a quiet day", got.Unparseable)
	}
	for _, m := range got.Messages {
		if m.At.Before(fixtureDay()) || !m.At.Before(fixtureDay().AddDate(0, 0, 1)) {
			t.Fatalf("message dated %s leaked past the day filter", m.At)
		}
	}
}

// TestReadOperatorMessagesUnreadableDirIsError is the three-state pin at the reader
// level: a directory that cannot be read is an ERROR, so Build can report
// could-not-check. Returning an empty result here is how a mis-pointed flag becomes a
// confident zero.
func TestReadOperatorMessagesUnreadableDirIsError(t *testing.T) {
	if _, err := ReadOperatorMessages(filepath.Join(t.TempDir(), "does-not-exist"), fixtureDay()); err == nil {
		t.Fatal("an unreadable transcripts dir returned no error — a blind read would be reported as zero")
	}
}

// TestSessionSpansCoverWholeFileNotJustTheDay pins that the >24h hygiene signal is
// measured over the session's WHOLE extent. Clipping the span to the reported day
// would make a 36-hour session look like a 14-hour one.
func TestSessionSpansCoverWholeFileNotJustTheDay(t *testing.T) {
	got, err := ReadOperatorMessages(fxTranscripts, fixtureDay())
	if err != nil {
		t.Fatalf("ReadOperatorMessages: %v", err)
	}
	if len(got.Spans) != 2 {
		t.Fatalf("spans = %d, want 2", len(got.Spans))
	}
	if n := countLongSessions(got.Spans); n != 1 {
		t.Fatalf("sessions over 24h = %d, want 1", n)
	}
}

// TestReadBeaconsZombieRequiresOpenWork pins the zombie definition. A stale beacon
// with NO open work is a session that finished and stopped beaconing — the normal end
// state, not a zombie. Counting it would make every completed session an alarm.
func TestReadBeaconsZombieRequiresOpenWork(t *testing.T) {
	got, err := ReadBeacons(filepath.Join(fxDeskTools, "roster"), fixtureNow())
	if err != nil {
		t.Fatalf("ReadBeacons: %v", err)
	}
	if got.Beacons != 5 {
		t.Fatalf("beacons = %d, want 5", got.Beacons)
	}
	if got.Zombies != 2 {
		t.Fatalf("zombies = %d, want 2 (stale WITH open work; the stale-but-idle beacon is not one)", got.Zombies)
	}
	if got.Unparseable != 1 {
		t.Fatalf("unparseable beacons = %d, want 1 (the one with no timestamp)", got.Unparseable)
	}
}

// TestReadBeaconsUnparseableTimestampIsNotAZombie pins the conservative half of the
// same rule: a beacon whose freshness cannot be established is counted as
// unparseable, NOT invented into a zombie. Manufacturing an alarm is as wrong as
// missing one, and the unparseable count is what tells the reader the answer is partial.
func TestReadBeaconsUnparseableTimestampIsNotAZombie(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "s.json"),
		[]byte(`{"session":"s","updated":"not-a-time","open_work":[{"repo":"example-org/tracker","pr":1,"what":"x"}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := ReadBeacons(dir, fixtureNow())
	if err != nil {
		t.Fatalf("ReadBeacons: %v", err)
	}
	if got.Zombies != 0 || got.Unparseable != 1 {
		t.Fatalf("zombies=%d unparseable=%d, want 0 and 1", got.Zombies, got.Unparseable)
	}
}

// TestReadClaimsReadsEveryWriterShape is why the claim reader goes through deskkit's
// tolerant Claim: three writers have written claims over this project's life, and a
// collector that understood only the canonical shape would report a confident, wrong,
// low number for dispatch volume.
func TestReadClaimsReadsEveryWriterShape(t *testing.T) {
	got, err := ReadClaims(filepath.Join(fxDeskTools, "claims"), fixtureDay())
	if err != nil {
		t.Fatalf("ReadClaims: %v", err)
	}
	if got.Open != 3 {
		t.Fatalf("open claims = %d, want 3 (canonical + loopengine + legacy bash shapes)", got.Open)
	}
	if got.FiledOnDate != 2 {
		t.Fatalf("claims filed on the day = %d, want 2 (the third is the previous day)", got.FiledOnDate)
	}
	if got.Owners != 2 {
		t.Fatalf("distinct owners = %d, want 2", got.Owners)
	}
	if got.Unparseable != 1 {
		t.Fatalf("unparseable claims = %d, want 1", got.Unparseable)
	}
}

// TestReadGHCountsAndPercentiles drives the whole latency reader off the fixture,
// including the basis breakdown that says how much of the answer is an upper bound.
func TestReadGHCountsAndPercentiles(t *testing.T) {
	got, err := ReadGH(fxGH, fixtureDay())
	if err != nil {
		t.Fatalf("ReadGH: %v", err)
	}
	if got.MergedOnDate != 5 {
		t.Fatalf("merged on the day = %d, want 5 (the previous-day merge is excluded)", got.MergedOnDate)
	}
	if got.Samples != 5 {
		t.Fatalf("latency samples = %d, want 5", got.Samples)
	}
	// Durations are 30, 120, 360, 1440, 60 minutes → sorted 30 60 120 360 1440.
	// Nearest-rank P50 is element ceil(.5*5)=3 → 120; P90 is element 5 → 1440.
	if got.P50Minutes != 120 || got.P90Minutes != 1440 {
		t.Fatalf("p50=%d p90=%d, want 120 and 1440", got.P50Minutes, got.P90Minutes)
	}
	if got.BasisReady != 2 || got.BasisDecisionReq != 1 || got.BasisCreated != 2 {
		t.Fatalf("basis ready=%d decision=%d created=%d, want 2/1/2",
			got.BasisReady, got.BasisDecisionReq, got.BasisCreated)
	}
	if got.Unparseable != 1 {
		t.Fatalf("unparseable PR records = %d, want 1 (the one with an empty mergedAt)", got.Unparseable)
	}
}

// TestReadGHUnreadableIsError is the same three-state pin for the gh input.
func TestReadGHUnreadableIsError(t *testing.T) {
	if _, err := ReadGH(filepath.Join(t.TempDir(), "absent.json"), fixtureDay()); err == nil {
		t.Fatal("a missing gh export returned no error — it would be reported as zero merged PRs")
	}
	bad := filepath.Join(t.TempDir(), "bad.json")
	if err := os.WriteFile(bad, []byte(`{"not":"an array"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadGH(bad, fixtureDay()); err == nil {
		t.Fatal("a malformed gh export returned no error")
	}
}

// TestPercentileIsNearestRank pins the choice: every reported percentile is a value
// some PR actually had. Interpolation over four daily samples invents a number nobody
// observed and then puts it in a committed artifact.
func TestPercentileIsNearestRank(t *testing.T) {
	s := []float64{10, 20, 30, 40}
	cases := []struct{ k, want float64 }{{50, 20}, {90, 40}, {0, 10}, {100, 40}}
	for _, c := range cases {
		if got := percentile(s, c.k); got != c.want {
			t.Fatalf("percentile(%v, %v) = %v, want %v", s, c.k, got, c.want)
		}
	}
	if got := percentile(nil, 50); got != 0 {
		t.Fatalf("percentile of an empty sample = %v, want 0", got)
	}
}
