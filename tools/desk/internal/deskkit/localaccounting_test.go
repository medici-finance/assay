package deskkit

import (
	"testing"
	"time"
)

func TestScanOverrideReceiptsDoNotConsumeWriteBudget(t *testing.T) {
	dir := setup(t)
	now := time.Now()
	for i := 0; i < 64; i++ {
		appendEntry(t, dir, Entry{Tool: "deskpr", Repo: testRepo, Verb: ScanOverrideVerb, Result: ResultOK})
		if err := LogScanOverride(ScanOverride{Tool: "deskpr", Verb: "create", Repo: testRepo, Reason: "synthetic fixture needs an audited override", Surface: "fixture", Content: []byte("fixture")}); err != nil {
			t.Fatal(err)
		}
	}
	entries, err := LoadEntries()
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 128 {
		t.Fatalf("override receipts lost: %d", len(entries))
	}
	for i, e := range entries {
		if i%2 == 1 && e.Result != ResultLocal {
			t.Fatalf("new receipt result=%s", e.Result)
		}
	}
	for _, reader := range []struct {
		name string
		read func() []auditPoint
	}{
		{"whole", func() []auditPoint { return wholeParsePointsFor(t, "deskpr") }},
		{"bounded", func() []auditPoint {
			p, ok := boundedPointsFor("deskpr", now)
			if !ok {
				t.Fatal("bounded reader could not settle")
			}
			return p
		}},
	} {
		p := reader.read()
		if err := checkRepoWideBudget("deskpr", testRepo, now, p); err != nil {
			t.Fatalf("%s charged local receipts: %v", reader.name, err)
		}
	}
	for i := 0; i < RateLimitPerPRPerHour; i++ {
		appendEntry(t, dir, Entry{Tool: "deskpr", Repo: testRepo, Verb: "create", Result: ResultUnverifiable})
	}
	if !IsRateLimited(AllowWriteRepoWideAt("deskpr", testRepo, time.Now())) {
		t.Fatal("uncertain remote creates escaped budget")
	}
}

func TestLocalReceiptsCannotResetFailureBreaker(t *testing.T) {
	dir := setup(t)
	for i := 0; i < BreakerTrip; i++ {
		appendEntry(t, dir, Entry{Tool: "deskpost", Repo: testRepo, PR: testPRPtr, Verb: "comment", Result: ResultRefused})
		appendEntry(t, dir, Entry{Tool: "deskpost", Repo: testRepo, PR: testPRPtr, Verb: ScanOverrideVerb, Result: ResultOK})
		appendEntry(t, dir, Entry{Tool: "deskpost", Repo: testRepo, PR: testPRPtr, Verb: ScanOverrideVerb, Result: ResultLocal})
	}
	if got := meterOf(AllowWrite("deskpost", testRepo, testPR)); got != "breaker" {
		t.Fatalf("local receipts reset failure run: %s", got)
	}
	if !chargesBudget("unknown-result") {
		t.Fatal("unclassified result must remain charged")
	}
	if meterResult(Entry{Verb: ScanOverrideVerb, Result: ResultUnverifiable}) != ResultUnverifiable {
		t.Fatal("ambiguous override result was silently exempted")
	}
}
