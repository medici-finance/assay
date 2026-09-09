package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestOpmetricsSchemaV2Readable is Verify row 8: the ladder/autonomy consumer view
// of the mm/40 day-file parses an opmetrics/2 file exactly as it parses opmetrics/1,
// with no metric change. opmetrics/2 ADDS operator.attention_families and keeps
// every v1 key; the consumer reads only the additive-tolerant schema/dispatch/token
// fields, so a v2 file is DayFile-present with schema "opmetrics/2" and the two
// day-file axes still honestly report "unmeasured" (the producer carries no
// dispatch-split or token block in either version).
func TestOpmetricsSchemaV2Readable(t *testing.T) {
	write := func(root, date, body string) {
		dir := filepath.Join(root, "docs", "reports", "daily", date)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "opmetrics.json"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	until := mustTime(t, "2026-07-25T00:00:00Z")

	// A realistic opmetrics/2 day-file: every v1 key present, plus the new
	// attention_families block. No dispatch/token blocks (the producer emits
	// neither in v1 or v2), so the consumer axes must read unmeasured, not error.
	v2body := `{
	  "schema": "opmetrics/2",
	  "date": "2026-07-22",
	  "classifierVersion": "opmetrics-relay/2",
	  "relay_ratio": 0.5,
	  "operator": {
	    "status": "ok",
	    "relay_families": {"sync":1,"state_echo":1,"poke":2,"lookup":2,"duplicate":1},
	    "attention_families": {"route":0,"status":2,"toil":0,"correction":3,"decision":0,"idea":0,"ack":2,"other":7}
	  }
	}`

	root := t.TempDir()
	write(root, "2026-07-22", v2body)

	df, date := loadOpDayFile(root, until)
	if df == nil {
		t.Fatal("an opmetrics/2 day-file did not parse — the consumer must tolerate the additive schema bump")
	}
	if df.Schema != "opmetrics/2" {
		t.Fatalf("parsed schema = %q, want opmetrics/2", df.Schema)
	}
	if date != "2026-07-22" {
		t.Fatalf("parsed date = %q, want 2026-07-22", date)
	}
	if df.Dispatch != nil || df.TokenEfficiency != nil {
		t.Fatalf("v2 file carried no dispatch/token block, but the consumer parsed one: %+v", df)
	}

	// The two day-file axes must degrade to unmeasured with their existing reason
	// codes — the SAME behaviour as v1, i.e. no metric change from the bump.
	rep := computeAutonomy(autonomyInputs{
		Since:       mustTime(t, "2026-07-01T00:00:00Z"),
		Until:       until,
		Now:         until,
		DayFile:     df,
		DayFileDate: date,
	})
	for _, key := range []string{"autonomy_dispatch", "token_efficiency"} {
		ax := findAxis(t, rep, key)
		if ax.Measured {
			t.Errorf("axis %q read as measured off a v2 day-file with no dispatch/token block — want unmeasured", key)
		}
	}

	// And an opmetrics/1 file still parses — the bump did not break the old shape.
	rootV1 := t.TempDir()
	write(rootV1, "2026-07-22", `{"schema":"opmetrics/1","dispatch":{"loop_initiated":9,"operator_initiated":1}}`)
	if df1, _ := loadOpDayFile(rootV1, until); df1 == nil || df1.Schema != "opmetrics/1" {
		t.Fatalf("an opmetrics/1 day-file no longer parses after the v2 change: %+v", df1)
	}
}

func findAxis(t *testing.T, rep AutonomyReport, key string) AutonomyAxis {
	t.Helper()
	for _, ax := range rep.Axes {
		if ax.Key == key {
			return ax
		}
	}
	t.Fatalf("axis %q not found in the autonomy report", key)
	return AutonomyAxis{}
}
