package deskkit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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
	for _, brief := range []string{
		"../x/01", "X/1", "no-slash", "a/", "/1",
		// #1803 SR-1803-3: number-side traversal/injection shapes. SplitBriefKey's
		// outcomeNumRe (^[0-9]+$) already refuses every one of these today; they are pinned
		// here so a future loosening of that regex (the exact mutation the security review
		// probed) is caught here FIRST, before it ever reaches UnderOutcomeRecordsDir's
		// independent check (see TestUnderOutcomeRecordsDirIndependentGuard).
		"x/01/../y", "x/../01", "x/01\x00", "x/1a",
	} {
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
		latest, _ := LatestPerBrief(recs)
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
		latest, _ := LatestPerBrief(recs)
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

// TestLatestPerBriefFutureTSCouldNotCheck is the reader half of #1803 SR-1803-2: a record whose
// `ts` is more than MaxClockSkew ahead of "now" must never win LatestPerBrief's newest-ts
// comparison, and its brief must be reported back (via the second return value) rather than
// silently folded into a normal result.
func TestLatestPerBriefFutureTSCouldNotCheck(t *testing.T) {
	fixedNow := time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC)

	t.Run("a lone future-ts record never wins: the brief is reported future, not silently trusted", func(t *testing.T) {
		future := OutcomeRecord{Brief: "example-stream/30", TS: fixedNow.Add(MaxClockSkew + time.Hour).Format(time.RFC3339), Raw: []byte(`{}`), Name: "z.json"}
		latest, futureByBrief := LatestPerBriefAt([]OutcomeRecord{future}, fixedNow)
		if _, ok := latest["example-stream/30"]; ok {
			t.Fatalf("a future-dated record with no non-future competitor must not appear as a normal winner: %+v", latest)
		}
		if !futureByBrief["example-stream/30"] {
			t.Fatalf("future-dated record must be reported in futureByBrief: %+v", futureByBrief)
		}
	})

	t.Run("a future-ts record never outranks a genuine, non-future record for the same brief", func(t *testing.T) {
		genuine := OutcomeRecord{Brief: "example-stream/31", TS: "2026-09-27T10:00:00Z", Raw: []byte(`{"outcome":"verify-fail"}`), Name: "a.json"}
		fabricated := OutcomeRecord{Brief: "example-stream/31", TS: "2099-01-01T00:00:00Z", Raw: []byte(`{"outcome":"verified"}`), Name: "b.json"}
		latest, futureByBrief := LatestPerBriefAt([]OutcomeRecord{genuine, fabricated}, fixedNow)
		got := latest["example-stream/31"]
		if got.TS != genuine.TS {
			t.Fatalf("LatestPerBriefAt returned %+v, want the genuine non-future record to win over the fabricated future one", got)
		}
		if !futureByBrief["example-stream/31"] {
			t.Fatalf("the excluded future record's brief must still be reported: %+v", futureByBrief)
		}
	})

	t.Run("a ts within the clock-skew tolerance is NOT future and wins normally", func(t *testing.T) {
		withinSkew := OutcomeRecord{Brief: "example-stream/32", TS: fixedNow.Add(MaxClockSkew - time.Minute).Format(time.RFC3339), Raw: []byte(`{}`), Name: "c.json"}
		latest, futureByBrief := LatestPerBriefAt([]OutcomeRecord{withinSkew}, fixedNow)
		if _, ok := latest["example-stream/32"]; !ok {
			t.Fatalf("a ts within MaxClockSkew must win normally: %+v", latest)
		}
		if futureByBrief["example-stream/32"] {
			t.Fatalf("a ts within MaxClockSkew must NOT be reported future")
		}
	})

	t.Run("FutureTS: unparsable ts is never future", func(t *testing.T) {
		if FutureTS("not-a-timestamp", fixedNow) {
			t.Fatalf("an unparsable ts must never be classified future")
		}
	})
}

// TestUnderOutcomeRecordsDirIndependentGuard is #1803 SR-1803-3's negative test: the writer's
// path-prefix guard is INDEPENDENT of RecordName/SplitBriefKey's own regex validation, so it must
// still refuse a traversal/absolute-path escape even when fed the exact shape a regression in
// THAT validation (for example outcomeNumRe loosened to admit a slash) would hand it. This test
// never calls RecordName — it feeds UnderOutcomeRecordsDir the resolved paths directly, so a
// regression in RecordName cannot also blind this check (the two share no code).
func TestUnderOutcomeRecordsDirIndependentGuard(t *testing.T) {
	for _, p := range []string{
		"docs/streams/verify-outcomes/example-stream/20-20260907T011923Z-aaaaaaaaaaaa.json",
	} {
		if err := UnderOutcomeRecordsDir(p); err != nil {
			t.Fatalf("UnderOutcomeRecordsDir(%q): want nil (a legitimate record path), got %v", p, err)
		}
	}

	for _, p := range []string{
		// The exact shape SR-1803-3's own probe produced from a brief key carrying traversal
		// segments in its number half, under a loosened outcomeNumRe: RecordName(brief
		// "x/01/../../../../.github/workflows/y") would resolve OUTSIDE docs/streams entirely.
		"docs/.github/workflows/y-20260907T011923Z-aaaaaaaaaaaa.json",
		"docs/streams/verify-outcomes/example-stream/../../../.github/workflows/evil.json",
		"docs/streams/verify-outcomes/../not-outcomes/x.json",
		"/etc/passwd",                                       // absolute-path injection
		"docs/streams/verify-outcomes/onlyonesegment.json",   // wrong depth: no stream segment
		"docs/streams/verify-outcomes/a/b/c.json",            // wrong depth: too deep
		"docs/other/verify-outcomes/example-stream/x.json",   // wrong root entirely
		"",
		// #1803 SR-1803-3 coverage gap: a short, unrelated two-segment relative path. Its shape
		// (<seg>/<seg>) mirrors a legitimate record's <stream>/<file>.json depth, so it is the
		// case that would slip through a naive HasPrefix(clean, OutcomeRecordsDir) check missing
		// the trailing "/" boundary (that check would treat "x/y.json" as no prefix match either
		// way, but a hypothetical "docs/streams/verify-outcomesX/y.json" WOULD false-positive
		// under such a bug — this pins the boundary-correct behavior directly). No production bug
		// found: the writer's `prefix := OutcomeRecordsDir + "/"` already includes the boundary
		// slash, so this refuses today; this case exists to keep it refusing.
		"x/y.json",
	} {
		if err := UnderOutcomeRecordsDir(p); err == nil {
			t.Fatalf("UnderOutcomeRecordsDir(%q): want refusal (path-traversal / prefix escape), got nil", p)
		}
	}
}
