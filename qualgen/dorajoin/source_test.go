package dorajoin

import (
	"os"
	"path/filepath"
	"testing"
)

// stubSource is a second, non-file DeliveryMetricsSource implementation used
// to prove the interface is genuinely pluggable — no join logic (denominator,
// cfr, joinkeys) references FileSource, so a stub swaps in freely.
type stubSource struct {
	records []DeliveryRecord
	err     error
}

func (s stubSource) DeliveryRecords() ([]DeliveryRecord, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.records, nil
}

// TestDeliverySourcePluggable (Verify item 7): the file-based reference
// adapter satisfies DeliveryMetricsSource and a second stub source swaps in
// without touching join logic.
func TestDeliverySourcePluggable(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "delivery.jsonl")
	writeFile(t, path, ""+
		`{"key":{"pr_number":42,"merge_sha":"deadbeef"},"window":"2026-08","incident":true}`+"\n"+
		"\n"+ // blank line skipped
		`{"key":{"pr_number":7,"merge_sha":"c0ffee00"},"window":"2026-08"}`+"\n",
	)

	var file DeliveryMetricsSource = NewFileSource(path)
	fileRecs, err := file.DeliveryRecords()
	if err != nil {
		t.Fatalf("FileSource.DeliveryRecords: %v", err)
	}
	if len(fileRecs) != 2 {
		t.Fatalf("got %d records from file source, want 2", len(fileRecs))
	}
	if !fileRecs[0].Incident || fileRecs[0].Key.PRNumber != 42 {
		t.Fatalf("first record not parsed correctly: %+v", fileRecs[0])
	}

	// DefaultSource wires the same reference adapter.
	var def DeliveryMetricsSource = DefaultSource(path)
	if _, err := def.DeliveryRecords(); err != nil {
		t.Fatalf("DefaultSource.DeliveryRecords: %v", err)
	}

	// A second, non-file source satisfies the same interface: swap it in
	// wherever a DeliveryMetricsSource is consumed, no join-logic change.
	var stub DeliveryMetricsSource = stubSource{records: []DeliveryRecord{
		{Key: JoinKey{PRNumber: 99}, Window: "2026-09", Incident: false},
	}}
	stubRecs, err := stub.DeliveryRecords()
	if err != nil {
		t.Fatalf("stubSource.DeliveryRecords: %v", err)
	}
	if len(stubRecs) != 1 || stubRecs[0].Key.PRNumber != 99 {
		t.Fatalf("stub source did not swap in cleanly: %+v", stubRecs)
	}

	// Both sources feed the SAME join-key resolver unmodified: the join logic
	// takes a DeliveryMetricsSource, never a concrete source type.
	quality := []JoinKey{{PRNumber: 42, MergeSHA: "deadbeef"}}
	for _, src := range []DeliveryMetricsSource{file, stub} {
		recs, err := src.DeliveryRecords()
		if err != nil {
			t.Fatalf("DeliveryRecords: %v", err)
		}
		for _, r := range recs {
			_ = ResolveJoin(r.Key, quality, nil)
		}
	}
}

func TestFileSourceMalformedLine(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.jsonl")
	writeFile(t, path, "not json\n")
	_, err := NewFileSource(path).DeliveryRecords()
	if err == nil {
		t.Fatalf("expected a malformed line to fail the read")
	}
}

func TestFileSourceMissingFile(t *testing.T) {
	_, err := NewFileSource("/nonexistent/path/does-not-exist.jsonl").DeliveryRecords()
	if err == nil {
		t.Fatalf("expected a missing file to error, not silently return empty")
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing test fixture %q: %v", path, err)
	}
}
