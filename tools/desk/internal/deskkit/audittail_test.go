package deskkit

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"
)

// plantLedger writes n synthetic audit rows into dir/<name>, returning the file's size.
// The rows are the real Entry shape so they parse exactly as production rows do.
func plantLedger(t *testing.T, dir, name string, n int, detail string) int64 {
	t.Helper()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	p := filepath.Join(dir, name)
	f, err := os.OpenFile(p, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatalf("open %s: %v", p, err)
	}
	base := time.Now().UTC().Add(-time.Duration(n) * time.Second)
	for i := 0; i < n; i++ {
		pr := i
		line, merr := json.Marshal(Entry{
			TS: base.Add(time.Duration(i) * time.Second).Format(time.RFC3339),
			Tool: "deskpost", Verb: "comment", Repo: "example-org/repo", PR: &pr,
			Result: ResultOK, Detail: fmt.Sprintf("%s %d", detail, i),
			ArgsDigest: "0000", SourceSHA: "dev", BuiltAt: "dev", SessionTag: "test-session",
		})
		if merr != nil {
			t.Fatalf("marshal: %v", merr)
		}
		if _, werr := f.Write(append(line, '\n')); werr != nil {
			t.Fatalf("write: %v", werr)
		}
	}
	if cerr := f.Close(); cerr != nil {
		t.Fatalf("close: %v", cerr)
	}
	fi, serr := os.Stat(p)
	if serr != nil {
		t.Fatalf("stat: %v", serr)
	}
	return fi.Size()
}

// TestLastEntryReadsBoundedBytes is the counting-reader row (#1035). The assertion is
// EQUAL-AND-BOUNDED, not merely "smaller": a tail read whose cost still grew with the
// ledger would pass a "smaller than the full parse" check while reproducing the defect.
func TestLastEntryReadsBoundedBytes(t *testing.T) {
	measure := func(rows int) (tailBytesUsed int64, fileSize int64) {
		dir := filepath.Join(t.TempDir(), "assay")
		old := dirOverride
		dirOverride = dir
		defer func() { dirOverride = old }()

		size := plantLedger(t, dir, "audit.jsonl", rows, "row")
		ResetTailBytesRead()
		e, ok := LastEntry()
		if !ok {
			t.Fatalf("LastEntry on a %d-row ledger: ok=false", rows)
		}
		if want := fmt.Sprintf("row %d", rows-1); e.Detail != want {
			t.Fatalf("LastEntry detail = %q, want %q", e.Detail, want)
		}
		return TailBytesRead(), size
	}

	smallBytes, smallSize := measure(1_000)
	bigBytes, bigSize := measure(100_000)

	if bigSize < smallSize*50 {
		t.Fatalf("fixture too weak: big ledger %d bytes vs small %d — the sizes must differ by orders of magnitude for this assertion to mean anything", bigSize, smallSize)
	}
	if smallBytes > 128*1024 || bigBytes > 128*1024 {
		t.Fatalf("tail read not bounded: %d bytes on the small ledger, %d on the big one (cap 128 KiB)", smallBytes, bigBytes)
	}
	if bigBytes != smallBytes {
		t.Fatalf("tail read cost differs with ledger size: %d vs %d bytes — it must be flat, not merely small", smallBytes, bigBytes)
	}

	// The control: the whole-file reader DOES pay for the size, which is what makes the
	// numbers above a finding about the new reader rather than about the fixture.
	if bigSize <= int64(bigBytes) {
		t.Fatalf("the big ledger (%d bytes) is not bigger than one tail read (%d) — fixture is degenerate", bigSize, bigBytes)
	}
}

// TestLastEntryThreeStates pins the best-effort contract lastResultWas has always
// documented, plus the day-boundary case rotation introduces.
func TestLastEntryThreeStates(t *testing.T) {
	t.Run("missing ledger", func(t *testing.T) {
		setup(t)
		if _, ok := LastEntry(); ok {
			t.Fatal("a missing ledger must be ok=false")
		}
		if lastResultWas(ResultDisabled) {
			t.Fatal("lastResultWas on a missing ledger must be false")
		}
	})

	t.Run("empty ledger", func(t *testing.T) {
		dir := setup(t)
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(filepath.Join(dir, "audit.jsonl"), nil, 0o600); err != nil {
			t.Fatalf("write: %v", err)
		}
		if _, ok := LastEntry(); ok {
			t.Fatal("a wholly empty ledger must be ok=false")
		}
	})

	t.Run("malformed final line", func(t *testing.T) {
		dir := setup(t)
		if err := Log(Entry{Tool: "deskpost", Verb: "comment", Result: ResultDisabled}); err != nil {
			t.Fatalf("Log: %v", err)
		}
		appendLine(t, dir, "{not json")
		if _, ok := LastEntry(); ok {
			t.Fatal("an unparseable final line must be ok=false, never the line before it")
		}
		if lastResultWas(ResultDisabled) {
			t.Fatal("a malformed final line must not let the PREVIOUS line answer the guard")
		}
	})

	t.Run("empty live file after rotation reads the newest segment", func(t *testing.T) {
		dir := setup(t)
		plantLedger(t, dir, "audit.jsonl."+time.Now().UTC().AddDate(0, 0, -1).Format("2006-01-02"), 3, "segment")
		if err := os.WriteFile(filepath.Join(dir, "audit.jsonl"), nil, 0o600); err != nil {
			t.Fatalf("write live: %v", err)
		}
		e, ok := LastEntry()
		if !ok {
			t.Fatal("an empty live file must not blind the disarm check — the newest segment answers")
		}
		if e.Detail != "segment 2" {
			t.Fatalf("LastEntry after rotation = %q, want the newest row of the newest segment", e.Detail)
		}
	})
}

// TestGuardOnHundredMegabyteLedgerMedianUnderFiftyMilliseconds is the benchmark row. It
// gates on the BOUNDED median only; the whole-parse median is printed for the record, so a
// faster machine cannot make the row vacuous by making the old path fast too.
func TestGuardOnHundredMegabyteLedgerMedianUnderFiftyMilliseconds(t *testing.T) {
	if testing.Short() {
		t.Skip("plants a 100 MB ledger")
	}
	dir := setup(t)

	// ~100 MB at ~230 bytes a row.
	const rows = 460_000
	size := plantLedger(t, dir, "audit.jsonl", rows, "bench")
	if size < 100<<20 {
		t.Fatalf("planted ledger is %d bytes, want at least 100 MB — the row asserts nothing on a small file", size)
	}

	median := func(f func()) time.Duration {
		const runs = 20
		ds := make([]time.Duration, 0, runs)
		for i := 0; i < runs; i++ {
			start := time.Now()
			f()
			ds = append(ds, time.Since(start))
		}
		sort.Slice(ds, func(i, j int) bool { return ds[i] < ds[j] })
		return ds[len(ds)/2]
	}

	bounded := median(func() { _ = lastResultWas(ResultDisabled) })
	whole := median(func() {
		entries, err := LoadEntries()
		if err != nil || len(entries) == 0 {
			t.Fatalf("whole-parse control failed: err=%v n=%d", err, len(entries))
		}
		_ = entries[len(entries)-1].Result == ResultDisabled
	})

	t.Logf("ledger %d bytes / %d rows: guard p50 bounded=%v whole-parse=%v", size, rows, bounded, whole)
	if bounded > 50*time.Millisecond {
		t.Fatalf("guard p50 = %v against a %d-byte ledger, want under 50ms", bounded, size)
	}
}
