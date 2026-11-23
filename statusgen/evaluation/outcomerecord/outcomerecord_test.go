package outcomerecord

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/medici-finance/assay/statusgen/evaluation"
)

// --- records ---------------------------------------------------------------------------

const (
	recA = `{"brief":"alpha/01","ts":"2026-10-06T10:00:00Z","outcome":"verify-fail","sha":"aaa"}`
	recB = `{"brief":"alpha/01","ts":"2026-10-06T11:00:00Z","outcome":"verified","sha":"bbb"}`
	recC = `{"brief":"beta/02","ts":"2026-10-06T09:00:00Z","outcome":"blocked","sha":"ccc"}`
)

func recordFile(t *testing.T, line string) (string, string) {
	t.Helper()
	name, err := RecordName([]byte(line))
	if err != nil {
		t.Fatal(err)
	}
	return name, line + "\n"
}

// regression: F-outcome-protocol-copies
func TestOutcomeProtocolRecords(t *testing.T) {
	t.Run("name and digest are a pure function of the bytes", func(t *testing.T) {
		sum := sha256.Sum256([]byte(recA + "\n"))
		want := hex.EncodeToString(sum[:])[:12]
		if got := RecordDigest([]byte(recA)); got != want {
			t.Fatalf("digest = %s, want %s", got, want)
		}
		name, err := RecordName([]byte(recA + "\n"))
		if err != nil {
			t.Fatal(err)
		}
		if name != "docs/streams/verify-outcomes/alpha/01-20261006T100000Z-"+want+".json" {
			t.Fatalf("name = %s", name)
		}
		if string(CanonicalBytes([]byte(recA))) != recA+"\n" {
			t.Fatal("canonical bytes are the line plus one newline")
		}
	})

	t.Run("malformed keys and records are refused, never sanitised", func(t *testing.T) {
		for _, k := range []string{"../01", "alpha", "alpha/x1", "Alpha/01", "alpha/", "/01", "a/b/01"} {
			if _, _, err := evaluation.SplitBriefKey(k); err == nil {
				t.Errorf("evaluation.SplitBriefKey(%q) accepted a malformed key", k)
			}
		}
		for _, raw := range []string{"", "not json", `{"ts":"x"}`, `{"brief":" "}`} {
			if _, err := ParseRecord([]byte(raw)); err == nil {
				t.Errorf("ParseRecord(%q) accepted a malformed record", raw)
			}
		}
		if _, err := RecordName([]byte(`{"brief":"alpha/01","ts":"yesterday"}`)); err == nil {
			t.Error("RecordName accepted an unparsable ts")
		}
	})

	t.Run("reader unions both layouts, dedupes and tolerates a malformed legacy line", func(t *testing.T) {
		nameA, bytesA := recordFile(t, recA)
		nameB, bytesB := recordFile(t, recB)
		fsys := fstest.MapFS{
			nameA: {Data: []byte(bytesA)},
			nameB: {Data: []byte(bytesB)},
			// the legacy log re-states B (dedupes by digest) and adds C and a malformed line
			"docs/streams/verify-outcomes.jsonl": {Data: []byte(recB + "\nnot json\n" + recC + "\n\n")},
		}
		got, skipped, err := ReadRecords(fsys, "ROOT")
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 3 {
			t.Fatalf("records = %d, want 3 (A, B once, C)", len(got))
		}
		if len(skipped) != 1 || !strings.HasPrefix(skipped[0].Source, "legacy:ROOT/") || string(skipped[0].Line) != "not json" {
			t.Fatalf("skipped = %+v", skipped)
		}
		for _, r := range got {
			if r.Name == "" || r.Source == "" || r.Digest == "" {
				t.Errorf("record missing identity: %+v", r)
			}
			if strings.HasPrefix(r.Source, "record:") && (r.Path == "" || !strings.HasPrefix(r.Source, "record:ROOT/")) {
				t.Errorf("per-file record lost its path/label: %+v", r)
			}
			if strings.HasPrefix(r.Source, "legacy:") && r.Path != "" {
				t.Errorf("a legacy line has no file path: %+v", r)
			}
		}
		latest, future := LatestPerBrief(got, time.Date(2026, 10, 7, 0, 0, 0, 0, time.UTC))
		if len(future) != 0 || latest["alpha/01"].Digest != RecordDigest([]byte(recB)) || latest["beta/02"].Digest != RecordDigest([]byte(recC)) {
			t.Fatalf("latest = %+v future=%v", latest, future)
		}
	})

	t.Run("absent history is empty; an unreadable file is could-not-check", func(t *testing.T) {
		got, skipped, err := ReadRecords(fstest.MapFS{}, "ROOT")
		if err != nil || len(got) != 0 || len(skipped) != 0 {
			t.Fatalf("empty tree: %v %v %v", got, skipped, err)
		}
		nameA, bytesA := recordFile(t, recA)
		if _, _, err := ReadRecords(failingFS{MapFS: fstest.MapFS{nameA: {Data: []byte(bytesA)}}, failOn: nameA}, "ROOT"); err == nil || !strings.Contains(err.Error(), "could-not-check") {
			t.Fatalf("an unreadable record must be could-not-check, got %v", err)
		}
		bad := fstest.MapFS{"docs/streams/verify-outcomes/alpha/01-x.json": {Data: []byte("not json")}}
		if _, _, err := ReadRecords(bad, "ROOT"); err == nil || !strings.Contains(err.Error(), "malformed") {
			t.Fatalf("a malformed per-file record must be refused, got %v", err)
		}
	})

	t.Run("latest per brief: newest wins, name breaks ties, a future ts never wins", func(t *testing.T) {
		now := time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC)
		mk := func(ts, name string) Record {
			return Record{Brief: "alpha/01", TS: ts, Name: name, Digest: name}
		}
		got, future := LatestPerBrief([]Record{
			mk("2026-10-06T10:00:00Z", "a"), mk("2026-10-06T11:00:00Z", "b"), mk("2026-10-06T11:00:00Z", "c"),
			mk("2999-01-01T00:00:00Z", "z"), mk("garbage", "y"),
		}, now)
		if got["alpha/01"].Name != "c" {
			t.Fatalf("winner = %s, want c (newest ts, greatest name)", got["alpha/01"].Name)
		}
		if !future["alpha/01"] {
			t.Fatal("the future-dated record must be reported")
		}
		if FutureTS("2026-10-06T12:04:59Z", now) || !FutureTS("2026-10-06T12:05:01Z", now) || FutureTS("garbage", now) {
			t.Fatal("future-ts bound is MaxClockSkew")
		}
	})
}

// failingFS fails reading one named file, to prove an unreadable record is an error.
type failingFS struct {
	fstest.MapFS
	failOn string
}

func (f failingFS) ReadFile(name string) ([]byte, error) {
	if name == f.failOn {
		return nil, errors.New("permission denied")
	}
	return f.MapFS.ReadFile(name)
}

func (f failingFS) Open(name string) (fs.File, error) {
	if name == f.failOn {
		return nil, errors.New("permission denied")
	}
	return f.MapFS.Open(name)
}
