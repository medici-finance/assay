package m4

import (
	"testing"

	"github.com/medici-finance/assay/qualgen/telemetry"
)

// stubSource is a minimal in-memory TelemetrySource for tests that don't
// need the real file adapter.
type stubSource struct {
	records map[telemetry.PRKey]telemetry.Measure[telemetry.TelemetryRecord]
}

func (s stubSource) Telemetry(key telemetry.PRKey) telemetry.Measure[telemetry.TelemetryRecord] {
	if m, ok := s.records[key]; ok {
		return m
	}
	return telemetry.CouldNotMeasure[telemetry.TelemetryRecord]("stub: no record for this key")
}

var _ telemetry.TelemetrySource = stubSource{}

// TestForensics_JoinDereferencesTelemetry dereferences the join against the
// REAL file reference adapter (Verify row 3): the fixture at
// testdata/sessions.jsonl carries PR 42 with retries=7; a fixture M1 outcome
// marks PR 42 high-churn. The emitted join row for PR 42 must carry retries=7
// pulled THROUGH the interface AND the churn outcome from M1 — proving the
// join resolved telemetry to the CORRECT PR, not merely that a row exists. A
// second, decoy PR (43) with its own distinct churn rate and no telemetry
// record in the fixture is joined alongside it, so a join that accidentally
// cross-wired the two rows (e.g. by position instead of by PRKey) would be
// caught by row 43 picking up PR 42's telemetry or vice versa.
func TestForensics_JoinDereferencesTelemetry(t *testing.T) {
	source := telemetry.NewFileAdapter("testdata/sessions.jsonl")
	if err := source.OpenError(); err != nil {
		t.Fatalf("unexpected open error: %v", err)
	}

	pr42 := telemetry.PRKey{PRNumber: 42, MergeSHA: "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef", StreamTaskID: "quality/13"}
	pr43 := telemetry.PRKey{PRNumber: 43, MergeSHA: "c0ffeec0ffeec0ffeec0ffeec0ffeec0ffeec0ff", StreamTaskID: "quality/13"}

	j := NewJoiner(source)
	rows := j.Join(
		[]ChurnOutcome{
			{Key: pr42, ChurnRate: telemetry.Measured(0.75)}, // "high-churn"
			{Key: pr43, ChurnRate: telemetry.Measured(0.05)}, // low-churn decoy
		},
		nil,
	)

	byKey := make(map[telemetry.PRKey]JoinedPR, len(rows))
	for _, r := range rows {
		byKey[r.Key] = r
	}

	row42, ok := byKey[pr42]
	if !ok {
		t.Fatalf("expected a join row for PR 42, got rows=%+v", rows)
	}
	if row42.Telemetry.State != telemetry.StateMeasured || row42.Telemetry.Value.Retries != 7 {
		t.Fatalf("expected PR 42's row to carry retries=7 pulled through the interface, got %+v", row42.Telemetry)
	}
	if row42.Churn.State != telemetry.StateMeasured || row42.Churn.Value != 0.75 {
		t.Fatalf("expected PR 42's row to carry its M1 churn outcome (0.75), got %+v", row42.Churn)
	}

	row43, ok := byKey[pr43]
	if !ok {
		t.Fatalf("expected a join row for decoy PR 43, got rows=%+v", rows)
	}
	if row43.Churn.State != telemetry.StateMeasured || row43.Churn.Value != 0.05 {
		t.Fatalf("expected PR 43's own churn outcome (0.05), not PR 42's — cross-wired join, got %+v", row43.Churn)
	}
	if row43.Telemetry.State == telemetry.StateMeasured && row43.Telemetry.Value.Retries == 7 {
		t.Fatalf("PR 43 has no telemetry record in the fixture; got PR 42's retries=7 instead — cross-wired join: %+v", row43.Telemetry)
	}
}

// TestForensics_MissingTelemetryIsCouldNotMeasure dereferences the
// three-state rule (Verify row 4): a fixture PR with no telemetry record
// (present in the M1/M2 corpus, absent from the telemetry source) emits
// could-not-measure for that PR, never a silent zero.
func TestForensics_MissingTelemetryIsCouldNotMeasure(t *testing.T) {
	source := telemetry.NewFileAdapter("testdata/sessions.jsonl")

	pr99 := telemetry.PRKey{PRNumber: 99, MergeSHA: "0000000000000000000000000000000000000000", StreamTaskID: "quality/13"}

	j := NewJoiner(source)
	rows := j.Join(
		[]ChurnOutcome{{Key: pr99, ChurnRate: telemetry.Measured(0.42)}},
		[]DefectOutcome{{Key: pr99, DefectInducing: telemetry.Measured(true)}},
	)
	if len(rows) != 1 {
		t.Fatalf("expected exactly one row, got %d: %+v", len(rows), rows)
	}
	row := rows[0]
	if row.Telemetry.State != telemetry.StateCouldNotMeasure {
		t.Fatalf("expected could-not-measure telemetry for a PR absent from the source, got %+v", row.Telemetry)
	}
	if row.Telemetry.Reason == "" {
		t.Fatal("could-not-measure requires a non-empty reason")
	}
	// Never a silent zero: a could-not-measure Telemetry.Value must not be
	// read as "zero retries" by a caller that skips the state check.
	if row.Telemetry.Value != (telemetry.TelemetryRecord{}) {
		t.Fatalf("could-not-measure must carry no meaningful value, got %+v", row.Telemetry.Value)
	}
	// The PR's own M1/M2 outcomes still came through untouched.
	if row.Churn.State != telemetry.StateMeasured || row.Churn.Value != 0.42 {
		t.Fatalf("expected the PR's own churn outcome to still be joined, got %+v", row.Churn)
	}
	if row.Defect.State != telemetry.StateMeasured || row.Defect.Value != true {
		t.Fatalf("expected the PR's own defect outcome to still be joined, got %+v", row.Defect)
	}
}

// TestJoin_UnionOfKeys proves a PR present in only one of churn/defects
// still gets a row, with the other family reported could-not-measure rather
// than the PR being dropped.
func TestJoin_UnionOfKeys(t *testing.T) {
	churnOnly := telemetry.PRKey{PRNumber: 1, MergeSHA: "a", StreamTaskID: "quality/13"}
	defectOnly := telemetry.PRKey{PRNumber: 2, MergeSHA: "b", StreamTaskID: "quality/13"}

	j := NewJoiner(stubSource{})
	rows := j.Join(
		[]ChurnOutcome{{Key: churnOnly, ChurnRate: telemetry.Measured(0.1)}},
		[]DefectOutcome{{Key: defectOnly, DefectInducing: telemetry.Measured(false)}},
	)
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows (union of the two key sets), got %d: %+v", len(rows), rows)
	}
	for _, r := range rows {
		switch r.Key {
		case churnOnly:
			if r.Churn.State != telemetry.StateMeasured {
				t.Fatalf("churn-only PR should keep its measured churn, got %+v", r.Churn)
			}
			if r.Defect.State != telemetry.StateCouldNotMeasure {
				t.Fatalf("churn-only PR should report could-not-measure defect, not drop the row: got %+v", r.Defect)
			}
		case defectOnly:
			if r.Defect.State != telemetry.StateMeasured {
				t.Fatalf("defect-only PR should keep its measured defect, got %+v", r.Defect)
			}
			if r.Churn.State != telemetry.StateCouldNotMeasure {
				t.Fatalf("defect-only PR should report could-not-measure churn, not drop the row: got %+v", r.Churn)
			}
		default:
			t.Fatalf("unexpected key in rows: %+v", r.Key)
		}
	}
}

// TestForensics_BandsReportThreeStateCoverage proves the correlation output
// carries coverage beside every average (honest-claims discipline, spec
// §10), and that an empty band is could-not-measure rather than a bare zero.
func TestForensics_BandsReportThreeStateCoverage(t *testing.T) {
	highRetries := telemetry.PRKey{PRNumber: 1, MergeSHA: "a", StreamTaskID: "quality/13"}
	lowRetries := telemetry.PRKey{PRNumber: 2, MergeSHA: "b", StreamTaskID: "quality/13"}
	noOutcome := telemetry.PRKey{PRNumber: 3, MergeSHA: "c", StreamTaskID: "quality/13"}

	source := stubSource{records: map[telemetry.PRKey]telemetry.Measure[telemetry.TelemetryRecord]{
		highRetries: telemetry.Measured(telemetry.TelemetryRecord{Retries: 8}),
		lowRetries:  telemetry.Measured(telemetry.TelemetryRecord{Retries: 1}),
		noOutcome:   telemetry.Measured(telemetry.TelemetryRecord{Retries: 8}),
	}}

	j := NewJoiner(source)
	report := j.Forensics(
		[]ChurnOutcome{
			{Key: highRetries, ChurnRate: telemetry.Measured(0.9)},
			{Key: lowRetries, ChurnRate: telemetry.Measured(0.1)},
		},
		// noOutcome is present in the M1/M2 corpus only via defects (no
		// churn outcome at all), so it still gets a join row (union-of-keys)
		// but contributes to the 6+ band's PRCount without a measured churn
		// outcome.
		[]DefectOutcome{
			{Key: noOutcome, DefectInducing: telemetry.Measured(false)},
		},
	)

	var band6plus, band0 *Band
	for i := range report.RetriesChurn {
		switch report.RetriesChurn[i].Label {
		case "6+":
			band6plus = &report.RetriesChurn[i]
		case "0":
			band0 = &report.RetriesChurn[i]
		}
	}
	if band6plus == nil {
		t.Fatal("expected a 6+ band")
	}
	// highRetries (0.9) and noOutcome (no churn outcome) both fall in 6+:
	// PRCount=2, but only ONE has a measured outcome.
	if band6plus.PRCount != 2 {
		t.Fatalf("expected 2 PRs (measured telemetry) in the 6+ band, got %d", band6plus.PRCount)
	}
	if band6plus.OutcomeAvg.State != telemetry.StateMeasured || band6plus.OutcomeAvg.Value != 0.9 {
		t.Fatalf("expected the 6+ band average to be 0.9 (the one PR with a measured outcome), got %+v", band6plus.OutcomeAvg)
	}
	if band6plus.Coverage.State != telemetry.StateMeasured || band6plus.Coverage.Value != 0.5 {
		t.Fatalf("expected 6+ band coverage 1/2=0.5, got %+v", band6plus.Coverage)
	}
	if band0 == nil {
		t.Fatal("expected a 0 band even though no PR fell in it")
	}
	if band0.PRCount != 0 || band0.OutcomeAvg.State != telemetry.StateCouldNotMeasure || band0.Coverage.State != telemetry.StateCouldNotMeasure {
		t.Fatalf("expected an empty band to be could-not-measure, never a bare zero: got %+v", band0)
	}
}
