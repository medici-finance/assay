package deskkit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFileT(t *testing.T, p, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestRecordNameShape(t *testing.T) {
	name, err := RecordName([]byte(`{"ts":"2026-09-07T01:19:23Z","brief":"example-stream/20","outcome":"verified","sha":"67abbac"}`))
	if err != nil {
		t.Fatalf("RecordName: %v", err)
	}
	want := "docs/streams/verify-outcomes/example-stream/20-20260907T011923Z-"
	if !strings.HasPrefix(name, want) || !strings.HasSuffix(name, ".json") {
		t.Fatalf("RecordName = %q, want prefix %q and suffix .json", name, want)
	}
}

func TestRecordNameMalformedBriefRefused(t *testing.T) {
	for _, brief := range []string{"../x/01", "X/1", "no-slash", "a/", "/1"} {
		line := `{"ts":"2026-09-07T01:19:23Z","brief":"` + brief + `","outcome":"verified"}`
		if _, err := RecordName([]byte(line)); err == nil {
			t.Fatalf("RecordName(brief=%q): want refusal, got a name", brief)
		}
	}
}

func TestRecordNameDeterministic(t *testing.T) {
	line := []byte(`{"ts":"2026-09-07T01:19:23Z","brief":"example-stream/20","outcome":"verified"}`)
	n1, err1 := RecordName(line)
	n2, err2 := RecordName(line)
	if err1 != nil || err2 != nil {
		t.Fatalf("RecordName errors: %v, %v", err1, err2)
	}
	if n1 != n2 {
		t.Fatalf("RecordName not deterministic: %q vs %q", n1, n2)
	}
}

// TestVerifyOutcomes is Verify row 1: the fixtures ReadVerifyOutcomes + LatestPerBrief must
// handle. Sub-tests are independent scratch trees so each fixture is isolated.
func TestVerifyOutcomes(t *testing.T) {
	t.Run("records only", func(t *testing.T) {
		root := t.TempDir()
		writeFileT(t, filepath.Join(root, "docs/streams/verify-outcomes/example-stream/20-20260907T011923Z-aaaaaaaaaaaa.json"),
			`{"ts":"2026-09-07T01:19:23Z","brief":"example-stream/20","outcome":"verified"}`+"\n")
		recs, err := ReadVerifyOutcomes(root)
		if err != nil {
			t.Fatalf("ReadVerifyOutcomes: %v", err)
		}
		if len(recs) != 1 {
			t.Fatalf("got %d records, want 1", len(recs))
		}
		latest := LatestPerBrief(recs)
		if latest["example-stream/20"].Brief != "example-stream/20" {
			t.Fatalf("LatestPerBrief missing example-stream/20: %+v", latest)
		}
	})

	t.Run("legacy log only", func(t *testing.T) {
		root := t.TempDir()
		writeFileT(t, filepath.Join(root, "docs/streams/verify-outcomes.jsonl"),
			`{"ts":"2026-09-07T01:19:23Z","brief":"example-stream/20","outcome":"verified"}`+"\n"+
				`{"ts":"2026-09-07T01:36:11Z","brief":"example-stream/05","outcome":"verified"}`+"\n")
		recs, err := ReadVerifyOutcomes(root)
		if err != nil {
			t.Fatalf("ReadVerifyOutcomes: %v", err)
		}
		if len(recs) != 2 {
			t.Fatalf("got %d records, want 2", len(recs))
		}
	})

	t.Run("both layouts, every log line also present as a record: each outcome read once", func(t *testing.T) {
		root := t.TempDir()
		line := `{"ts":"2026-09-07T01:19:23Z","brief":"example-stream/20","outcome":"verified"}`
		writeFileT(t, filepath.Join(root, "docs/streams/verify-outcomes.jsonl"), line+"\n")
		name, err := RecordName([]byte(line))
		if err != nil {
			t.Fatal(err)
		}
		writeFileT(t, filepath.Join(root, filepath.FromSlash(name)), line+"\n")
		recs, err := ReadVerifyOutcomes(root)
		if err != nil {
			t.Fatalf("ReadVerifyOutcomes: %v", err)
		}
		if len(recs) != 1 {
			t.Fatalf("a record present in both layouts must be read ONCE: got %d records: %+v", len(recs), recs)
		}
	})

	t.Run("last log line for a brief carries an OLDER ts than an earlier line: LatestPerBrief returns the newer one", func(t *testing.T) {
		root := t.TempDir()
		// Deliberately out of ts order — a last-line-wins reader would return the OLDER row.
		writeFileT(t, filepath.Join(root, "docs/streams/verify-outcomes.jsonl"),
			`{"ts":"2026-09-27T10:00:00Z","brief":"example-stream/20","outcome":"verified"}`+"\n"+
				`{"ts":"2026-09-07T01:19:23Z","brief":"example-stream/20","outcome":"verify-fail"}`+"\n")
		recs, err := ReadVerifyOutcomes(root)
		if err != nil {
			t.Fatalf("ReadVerifyOutcomes: %v", err)
		}
		latest := LatestPerBrief(recs)
		got := latest["example-stream/20"]
		if got.TS != "2026-09-27T10:00:00Z" {
			t.Fatalf("LatestPerBrief returned %+v, want the newer (2026-09-27) row regardless of line position", got)
		}
	})

	t.Run("unreadable record file: error, not a skip", func(t *testing.T) {
		root := t.TempDir()
		// A dangling symlink is LISTED (os.ReadDir sees the entry, and it is not a directory,
		// so it is not skipped as a subdirectory the way a real directory would be) but fails
		// to READ — a real, non-not-exist-shaped failure, portable across CI (unlike a
		// chmod-based permission test, which a root-running CI user can simply bypass).
		dir := filepath.Join(root, "docs/streams/verify-outcomes/example-stream")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		badPath := filepath.Join(dir, "20-20260907T011923Z-aaaaaaaaaaaa.json")
		if err := os.Symlink(filepath.Join(dir, "does-not-exist"), badPath); err != nil {
			t.Fatal(err)
		}
		if _, err := ReadVerifyOutcomes(root); err == nil {
			t.Fatalf("ReadVerifyOutcomes over an unreadable record file: want an error, got nil (a skipped record is never acceptable here)")
		}
	})

	t.Run("neither layout present: empty, no error", func(t *testing.T) {
		root := t.TempDir()
		recs, err := ReadVerifyOutcomes(root)
		if err != nil {
			t.Fatalf("ReadVerifyOutcomes on a tree with no outcome history: %v", err)
		}
		if len(recs) != 0 {
			t.Fatalf("got %d records, want 0", len(recs))
		}
	})

	t.Run("malformed brief refused by RecordName", func(t *testing.T) {
		for _, brief := range []string{"../x/01", "X/1"} {
			line := `{"ts":"2026-09-07T01:19:23Z","brief":"` + brief + `"}`
			if _, err := RecordName([]byte(line)); err == nil {
				t.Fatalf("RecordName(brief=%q): want refusal", brief)
			}
		}
	})
}
