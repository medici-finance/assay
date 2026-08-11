package main

import (
	"bufio"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func fixedNow(t *testing.T, ts string) {
	t.Helper()
	parsed, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		t.Fatal(err)
	}
	old := nowFunc
	nowFunc = func() time.Time { return parsed }
	t.Cleanup(func() { nowFunc = old })
}

func mkStreams(briefs map[string]string) []*Stream {
	// briefs keys are "<stream>/<NN>"; group them into Stream/Brief structs.
	byStream := map[string][]Brief{}
	for id, status := range briefs {
		parts := strings.SplitN(id, "/", 2)
		byStream[parts[0]] = append(byStream[parts[0]], Brief{Num: parts[1], Status: status})
	}
	var streams []*Stream
	for name, bs := range byStream {
		streams = append(streams, &Stream{Name: name, Briefs: bs})
	}
	return streams
}

func TestDiffHistorySeedsFirstRun(t *testing.T) {
	streams := mkStreams(map[string]string{"hist/01": "implemented"})
	entries := diffHistory(streams, map[string]string{}, "sha1", time.Unix(0, 0).UTC())
	if len(entries) != 1 {
		t.Fatalf("got %d entries, want 1", len(entries))
	}
	e := entries[0]
	if e.Brief != "hist/01" || e.From != "" || e.To != "implemented" || e.SHA != "sha1" {
		t.Errorf("wrong seed entry: %+v", e)
	}
}

func TestDiffHistoryNoChangesEmpty(t *testing.T) {
	streams := mkStreams(map[string]string{"hist/01": "implemented"})
	last := map[string]string{"hist/01": "implemented"}
	entries := diffHistory(streams, last, "sha1", time.Unix(0, 0).UTC())
	if len(entries) != 0 {
		t.Fatalf("got %d entries, want 0 (idempotent no-op)", len(entries))
	}
}

func TestDiffHistoryDetectsTransition(t *testing.T) {
	streams := mkStreams(map[string]string{"hist/01": "verified"})
	last := map[string]string{"hist/01": "implemented"}
	entries := diffHistory(streams, last, "shaNEW", time.Unix(0, 0).UTC())
	if len(entries) != 1 {
		t.Fatalf("got %d entries, want 1", len(entries))
	}
	e := entries[0]
	if e.From != "implemented" || e.To != "verified" || e.SHA != "shaNEW" {
		t.Errorf("wrong transition entry: %+v", e)
	}
}

func TestLoadHistoryMissingFileIsNotError(t *testing.T) {
	entries, err := LoadHistory(filepath.Join(t.TempDir(), "does-not-exist.jsonl"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entries != nil {
		t.Errorf("got %v, want nil", entries)
	}
}

func TestLoadHistoryRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".history.jsonl")
	body := `{"ts":"2026-07-01T00:00:00Z","brief":"hist/01","from":"","to":"implemented","sha":"a"}
{"ts":"2026-07-02T00:00:00Z","brief":"hist/01","from":"implemented","to":"verified","sha":"b"}
`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	entries, err := LoadHistory(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(entries))
	}
	last := LastRecordedStatus(entries)
	if last["hist/01"] != "verified" {
		t.Errorf("last status = %q, want verified (last line wins)", last["hist/01"])
	}
}

func TestLoadHistoryMalformedLineErrors(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".history.jsonl")
	if err := os.WriteFile(path, []byte("not json\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadHistory(path); err == nil {
		t.Error("expected an error on a malformed history line")
	}
}

func TestAppendHistoryNoopOnEmpty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", ".history.jsonl")
	if err := appendHistory(path, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("appendHistory(nil) must not create the file")
	}
}

func TestAppendHistoryAppendsLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".history.jsonl")
	if err := appendHistory(path, []HistoryEntry{{Ts: "t1", Brief: "hist/01", From: "", To: "todo", SHA: "s1"}}); err != nil {
		t.Fatal(err)
	}
	if err := appendHistory(path, []HistoryEntry{{Ts: "t2", Brief: "hist/01", From: "todo", To: "implemented", SHA: "s2"}}); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	lines := 0
	sc := bufio.NewScanner(bytes.NewReader(raw))
	for sc.Scan() {
		if strings.TrimSpace(sc.Text()) != "" {
			lines++
		}
	}
	if lines != 2 {
		t.Fatalf("got %d lines, want 2 (append, not overwrite)", lines)
	}
}

// TestRecordAppendsTransitionAndIsIdempotent is the brief's Verify item 2:
// seed a testdata history, flip a brief implemented->verified, run
// `statusgen --record`, expect one new JSONL line; re-run appends nothing.
func TestRecordAppendsTransitionAndIsIdempotent(t *testing.T) {
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS("testdata/historyrepo")); err != nil {
		t.Fatal(err)
	}
	historyPath := filepath.Join(root, "docs", "streams", ".history.jsonl")

	before, err := os.ReadFile(historyPath)
	if err != nil {
		t.Fatal(err)
	}
	beforeLines := strings.Count(strings.TrimSpace(string(before)), "\n") + 1

	// Flip hist/01 implemented -> verified (Verified column must be filled —
	// check() requires it for status "verified").
	readme := filepath.Join(root, "docs", "streams", "hist", "README.md")
	raw, err := os.ReadFile(readme)
	if err != nil {
		t.Fatal(err)
	}
	flipped := strings.Replace(string(raw),
		"| 01 | [First](brief-01.md) | 0 | implemented | — | — |",
		"| 01 | [First](brief-01.md) | 0 | verified | 2026-07-09 test | — |", 1)
	if flipped == string(raw) {
		t.Fatal("fixture row not found — test setup is stale")
	}
	if err := os.WriteFile(readme, []byte(flipped), 0o644); err != nil {
		t.Fatal(err)
	}

	fixedNow(t, "2026-07-09T12:00:00Z")

	if code := run(root, "record", nil, nil, ""); code != 0 {
		t.Fatalf("record run exited %d, want 0", code)
	}

	after, err := os.ReadFile(historyPath)
	if err != nil {
		t.Fatal(err)
	}
	afterLines := strings.Count(strings.TrimSpace(string(after)), "\n") + 1
	if afterLines != beforeLines+1 {
		t.Fatalf("got %d lines after record, want %d (exactly one new line)", afterLines, beforeLines+1)
	}
	last := strings.TrimSpace(string(after))
	lastLine := last[strings.LastIndex(last, "\n")+1:]
	if !strings.Contains(lastLine, `"brief":"hist/01"`) ||
		!strings.Contains(lastLine, `"from":"implemented"`) ||
		!strings.Contains(lastLine, `"to":"verified"`) {
		t.Errorf("appended line missing expected fields: %s", lastLine)
	}

	// Re-run with no further source changes: must append nothing.
	if code := run(root, "record", nil, nil, ""); code != 0 {
		t.Fatalf("second record run exited %d, want 0", code)
	}
	again, err := os.ReadFile(historyPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(again) != string(after) {
		t.Error("re-running --record with no status changes must be a byte-identical no-op")
	}
}

// TestRecordFirstRunSeedsEveryBrief covers the "first run" case: no history
// log exists yet, so every current brief status is seeded with from="".
func TestRecordFirstRunSeedsEveryBrief(t *testing.T) {
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS("testdata/goodrepo")); err != nil {
		t.Fatal(err)
	}
	historyPath := filepath.Join(root, "docs", "streams", ".history.jsonl")
	if _, err := os.Stat(historyPath); !os.IsNotExist(err) {
		t.Fatal("fixture must not ship a history log — this test covers the very first run")
	}

	fixedNow(t, "2026-07-09T12:00:00Z")

	if code := run(root, "record", nil, nil, ""); code != 0 {
		t.Fatalf("record run exited %d, want 0", code)
	}
	entries, err := LoadHistory(historyPath)
	if err != nil {
		t.Fatal(err)
	}
	// goodrepo/alpha has brief-01 (done) and brief-02 (todo).
	if len(entries) != 2 {
		t.Fatalf("got %d seed entries, want 2", len(entries))
	}
	for _, e := range entries {
		if e.From != "" {
			t.Errorf("seed entry %+v should have from=\"\"", e)
		}
	}

	// --lint must never write the history log, even after a record run
	// created it — confirm lint doesn't touch it (methodology-metrics/01
	// Ground rules + Verify item 3).
	statBefore, err := os.Stat(historyPath)
	if err != nil {
		t.Fatal(err)
	}
	if code := run(root, "lint", nil, nil, ""); code != 0 {
		t.Fatalf("lint run exited %d, want 0", code)
	}
	statAfter, err := os.Stat(historyPath)
	if err != nil {
		t.Fatal(err)
	}
	if statBefore.ModTime() != statAfter.ModTime() || statBefore.Size() != statAfter.Size() {
		t.Error("--lint must not modify the history log")
	}
}
