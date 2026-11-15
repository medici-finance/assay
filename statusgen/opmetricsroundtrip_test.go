package main

import (
	"testing"
)

// opmetricsRoundTripRoot holds a day-file emitted by the opmetrics PRODUCER
// (tools/desk/cmd/opmetrics) over its dispatch fixture. The producer side pins the
// same bytes (TestRoundTripGoldenMatchesProducer), so this file cannot drift from
// what the producer writes without one of the two tests failing.
const opmetricsRoundTripRoot = "testdata/opmetrics-roundtrip"

// TestOpmetricsRoundTripLadderStepsMeasured is the consumer half of the round trip:
// the producer's real output, read through loadOpDayFile, makes the ladder's rungs 3
// and 4 (and the two autonomy axes behind them) MEASURED. Before the producer wrote
// the dispatch and token_efficiency blocks, both rungs read dayfile-no-dispatch-split
// and dayfile-no-token-block off every real day-file.
// regression: #5470
func TestOpmetricsRoundTripLadderStepsMeasured(t *testing.T) {
	until := mustTime(t, "2026-07-25T00:00:00Z")
	df, date := loadOpDayFile(opmetricsRoundTripRoot, until)
	if df == nil {
		t.Fatalf("the producer's day-file under %s did not parse", opmetricsRoundTripRoot)
	}
	if date != "2026-07-22" || df.Schema != "opmetrics/2" {
		t.Fatalf("day-file date=%q schema=%q, want 2026-07-22 / opmetrics/2", date, df.Schema)
	}
	in := autonomyInputs{
		Since:       mustTime(t, "2026-07-01T00:00:00Z"),
		Until:       until,
		Now:         until,
		DayFile:     df,
		DayFileDate: date,
	}

	rungs := ladderRungsFromAutonomy(in)
	byStep := map[int]ladderRung{}
	for _, r := range rungs {
		byStep[r.Step] = r
	}
	r3, r4 := byStep[3], byStep[4]
	if !r3.Measured {
		t.Fatalf("rung 3 unmeasured (%s) off the producer's day-file", r3.Reason)
	}
	// 4 loop / (4 loop + 2 operator) = 67% ≥ the 60% threshold.
	if !r3.Pass {
		t.Fatalf("rung 3 = %q, want a pass at 67%% loop-initiated", r3.Detail)
	}
	if !r4.Measured {
		t.Fatalf("rung 4 unmeasured (%s) off the producer's day-file", r4.Reason)
	}
	// 2 no-op / 4 runs = 50% > the 20% cap: measured, and a fail.
	if r4.Pass {
		t.Fatalf("rung 4 = %q, want a measured fail at a 50%% no-op rate", r4.Detail)
	}

	rep := computeAutonomy(in)
	for _, key := range []string{"autonomy_dispatch", "token_efficiency"} {
		if ax := findAxis(t, rep, key); !ax.Measured {
			t.Errorf("axis %q unmeasured (%s) off the producer's day-file", key, ax.Reason)
		}
	}
	if ax := findAxis(t, rep, "token_efficiency"); ax.Value != "651 subagent tokens/merged PR; 50% no-op dispatch rate" {
		t.Errorf("token axis value = %q", ax.Value)
	}
}
