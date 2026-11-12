package main

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// TestOutcomeRecordWrite is Verify row 3: the --outcome-record landing shape.
func TestOutcomeRecordWrite(t *testing.T) {
	t.Run("lands at the path RecordName gives", func(t *testing.T) {
		f, errBuf := setupFake(t)
		line := completeFail(t, `{"ts":"2026-09-07T02:00:00Z","brief":"example-stream/14","outcome":"verify-fail","sha":"0000002"}`)
		want, err := deskkit.RecordName([]byte(line))
		if err != nil {
			t.Fatal(err)
		}
		recFile := writeRepoFile(t, "record.json", line+"\n")

		code := run([]string{"example-org/tracker", "main", "--outcome-record", recFile})
		if code != deskkit.ExitOK {
			t.Fatalf("exit = %d, want 0 (stderr %q)", code, errBuf.String())
		}
		if len(f.writes) != 1 || f.writes[0].File != want {
			t.Fatalf("landed at %v, want exactly one write to %q", f.writes, want)
		}
	})

	t.Run("identical re-write is a noop", func(t *testing.T) {
		f, errBuf := setupFake(t)
		line := completeFail(t, `{"ts":"2026-09-07T02:00:00Z","brief":"example-stream/14","outcome":"verify-fail","sha":"0000002"}`)
		name, err := deskkit.RecordName([]byte(line))
		if err != nil {
			t.Fatal(err)
		}
		f.setFile(name, line+"\n")
		recFile := writeRepoFile(t, "record.json", line+"\n")

		code := run([]string{"example-org/tracker", "main", "--outcome-record", recFile})
		if code != deskkit.ExitOK {
			t.Fatalf("exit = %d, want 0 (stderr %q)", code, errBuf.String())
		}
		if f.putCalls != 0 {
			t.Fatalf("identical re-write must not land a write, got %d", f.putCalls)
		}
	})

	t.Run("a different-bytes write to an existing record path is refused (records are immutable)", func(t *testing.T) {
		f, errBuf := setupFake(t)
		line := completeFail(t, `{"ts":"2026-09-07T02:00:00Z","brief":"example-stream/14","outcome":"verify-fail","sha":"0000002","note":"a"}`)
		name, err := deskkit.RecordName([]byte(line))
		if err != nil {
			t.Fatal(err)
		}
		// Plant DIFFERENT bytes at the exact path this record's own name derives — a corrupted
		// or hand-edited remote copy, the one shape the immutability rule exists to refuse.
		f.setFile(name, `{"ts":"2026-09-07T02:00:00Z","brief":"example-stream/14","outcome":"verify-fail","sha":"9999999"}`+"\n")
		recFile := writeRepoFile(t, "record.json", line+"\n")

		code := run([]string{"example-org/tracker", "main", "--outcome-record", recFile})
		if code != deskkit.ExitRefused {
			t.Fatalf("exit = %d, want %d (refused, immutable) (stderr %q)", code, deskkit.ExitRefused, errBuf.String())
		}
		if f.putCalls != 0 {
			t.Fatalf("refusal must not write, got %d", f.putCalls)
		}
		if !strings.Contains(errBuf.String(), "immutable") {
			t.Fatalf("refusal message = %q, want it to name immutability", errBuf.String())
		}
	})

	t.Run("a verified record whose closure is not landed exits 5, exactly as a log line did", func(t *testing.T) {
		f, errBuf := setupFake(t)
		verifiedClosureCheckFn = func(root, brief string) (closureVerdict, string, error) {
			return closureNotAccepted, brief + ": NOT accepted", nil
		}
		line := `{"ts":"2026-09-07T02:00:00Z","brief":"example-stream/14","outcome":"verified","sha":"0000002"}`
		recFile := writeRepoFile(t, "record.json", line+"\n")

		code := run([]string{"example-org/tracker", "main", "--outcome-record", recFile})
		if code != deskkit.ExitRefused {
			t.Fatalf("exit = %d, want %d (stderr %q)", code, deskkit.ExitRefused, errBuf.String())
		}
		if f.putCalls != 0 {
			t.Fatalf("refusal must not write, got %d", f.putCalls)
		}
	})

	t.Run("a verify-fail record is not gated", func(t *testing.T) {
		f, errBuf := setupFake(t)
		verifiedClosureCheckFn = func(root, brief string) (closureVerdict, string, error) {
			t.Fatalf("closure check must NOT run for a verify-fail record (brief %q)", brief)
			return closureCouldNotErr, "", nil
		}
		line := completeFail(t, `{"ts":"2026-09-07T02:00:00Z","brief":"example-stream/14","outcome":"verify-fail","sha":"0000002"}`)
		recFile := writeRepoFile(t, "record.json", line+"\n")

		code := run([]string{"example-org/tracker", "main", "--outcome-record", recFile})
		if code != deskkit.ExitOK {
			t.Fatalf("exit = %d, want 0 (stderr %q)", code, errBuf.String())
		}
		if f.putCalls != 1 {
			t.Fatalf("expected exactly 1 write, got %d", f.putCalls)
		}
	})

	// #1803 SR-1803-2 writer half: a record whose ts is more than deskkit.MaxClockSkew ahead of
	// the writer's own clock is refused before it ever reaches the forge.
	t.Run("a ts more than the clock-skew tolerance ahead of now is refused", func(t *testing.T) {
		f, errBuf := setupFake(t)
		fixedNow := time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC)
		nowFn = func() time.Time { return fixedNow }
		defer func() { nowFn = time.Now }()

		futureTS := fixedNow.Add(deskkit.MaxClockSkew + time.Minute).Format(time.RFC3339)
		line := `{"ts":"` + futureTS + `","brief":"example-stream/14","outcome":"verified","sha":"0000002"}`
		recFile := writeRepoFile(t, "record.json", line+"\n")

		code := run([]string{"example-org/tracker", "main", "--outcome-record", recFile})
		if code != deskkit.ExitRefused {
			t.Fatalf("exit = %d, want %d (refused, future ts) (stderr %q)", code, deskkit.ExitRefused, errBuf.String())
		}
		if f.putCalls != 0 {
			t.Fatalf("refusal must not write, got %d", f.putCalls)
		}
		if !strings.Contains(errBuf.String(), "ts") {
			t.Fatalf("refusal message = %q, want it to name ts", errBuf.String())
		}
	})

	t.Run("a ts within the clock-skew tolerance is accepted", func(t *testing.T) {
		f, errBuf := setupFake(t)
		fixedNow := time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC)
		nowFn = func() time.Time { return fixedNow }
		defer func() { nowFn = time.Now }()

		withinSkewTS := fixedNow.Add(deskkit.MaxClockSkew - time.Minute).Format(time.RFC3339)
		line := completeFail(t, `{"ts":"`+withinSkewTS+`","brief":"example-stream/14","outcome":"verify-fail","sha":"0000002"}`)
		recFile := writeRepoFile(t, "record.json", line+"\n")

		code := run([]string{"example-org/tracker", "main", "--outcome-record", recFile})
		if code != deskkit.ExitOK {
			t.Fatalf("exit = %d, want 0 (stderr %q)", code, errBuf.String())
		}
		if f.putCalls != 1 {
			t.Fatalf("expected exactly 1 write, got %d", f.putCalls)
		}
	})

	// Issue #4163: the attempts list lands as validated, byte-for-byte; an invalid one is
	// exit 5 and nothing is written.
	attemptsLine := func(attempts string) string {
		return completeFail(t, `{"ts":"2026-09-07T02:00:00Z","brief":"example-stream/14","outcome":"blocked","sha":"0000002",`+
			`"started":"2026-09-07T01:00:00Z","finished":"2026-09-07T01:50:00Z","rows_passed":0,"rows_total":9,"attempts":`+attempts+`}`)
	}
	rawAttempts := func(t *testing.T, line []byte) string {
		t.Helper()
		var m map[string]json.RawMessage
		if err := json.Unmarshal(line, &m); err != nil {
			t.Fatalf("record is not a JSON object: %v", err)
		}
		return string(m["attempts"])
	}
	t.Run("a valid attempts list lands byte-for-byte", func(t *testing.T) {
		f, errBuf := setupFake(t)
		line := attemptsLine(`[` +
			`{"attempt":1,"disposition":"uncredited","reason":"local-build","started":"2026-09-07T01:00:00Z","finished":"2026-09-07T01:10:00Z","rows_run":9},` +
			`{"attempt":2,"disposition":"discarded","reason":"env-fault","cause_ref":"example-org/tracker#12","started":"2026-09-07T01:15:00Z","finished":"2026-09-07T01:25:00Z","rows_run":5},` +
			`{"attempt":3,"disposition":"credited","started":"2026-09-07T01:30:00Z","finished":"2026-09-07T01:45:00Z","rows_run":9}]`)
		recFile := writeRepoFile(t, "record.json", line+"\n")

		code := run([]string{"example-org/tracker", "main", "--outcome-record", recFile})
		if code != deskkit.ExitOK {
			t.Fatalf("exit = %d, want 0 (stderr %q)", code, errBuf.String())
		}
		if len(f.writes) != 1 {
			t.Fatalf("want exactly one write, got %d", len(f.writes))
		}
		want, got := rawAttempts(t, []byte(line)), rawAttempts(t, f.writes[0].Content)
		if want == "" || got != want {
			t.Fatalf("attempts did not land byte-for-byte:\nsubmitted %s\nlanded    %s", want, got)
		}
	})
	t.Run("an invalid attempts list exits 5 and writes nothing", func(t *testing.T) {
		f, errBuf := setupFake(t)
		line := attemptsLine(`[` +
			`{"attempt":1,"disposition":"credited","started":"2026-09-07T01:00:00Z","finished":"2026-09-07T01:10:00Z","rows_run":9},` +
			`{"attempt":2,"disposition":"uncredited","reason":"not-credited","started":"2026-09-07T01:15:00Z","finished":"2026-09-07T01:25:00Z","rows_run":9}]`)
		recFile := writeRepoFile(t, "record.json", line+"\n")

		code := run([]string{"example-org/tracker", "main", "--outcome-record", recFile})
		if code != deskkit.ExitRefused {
			t.Fatalf("exit = %d, want %d (stderr %q)", code, deskkit.ExitRefused, errBuf.String())
		}
		if f.putCalls != 0 || len(f.writes) != 0 {
			t.Fatalf("a refused attempts list must write nothing, got %d put(s)", f.putCalls)
		}
		if !strings.Contains(errBuf.String(), "attempt 1: rule credited-last") {
			t.Fatalf("refusal %q does not name the attempt and rule", errBuf.String())
		}
	})
}

// TestAppendedLogWriteRefused is Verify row 4: the desk-supervision-class-guard #882.
func TestAppendedLogWriteRefused(t *testing.T) {
	t.Run("a write to docs/streams/verify-outcomes.jsonl exits 5 naming #882", func(t *testing.T) {
		f, errBuf := setupFake(t)
		evidencePath := "docs/streams/verify-outcomes.jsonl"
		root := rootWithFile(t, evidencePath, `{"ts":"2026-09-07T01:19:23Z","brief":"example-stream/1","outcome":"verified"}`+"\n")

		code := run([]string{"example-org/tracker", "main", "--evidence-file", evidencePath, "--root", root})
		if code != deskkit.ExitRefused {
			t.Fatalf("exit = %d, want %d (stderr %q)", code, deskkit.ExitRefused, errBuf.String())
		}
		if len(f.hits) != 0 {
			t.Fatalf("class-guard refusal must not reach the forge: %v", f.hits)
		}
		if !strings.Contains(errBuf.String(), "#882") {
			t.Fatalf("refusal message = %q, want it to name #882", errBuf.String())
		}
	})

	t.Run("a write to a planted second log docs/streams/example-log.jsonl also refuses", func(t *testing.T) {
		f, errBuf := setupFake(t)
		evidencePath := "docs/streams/example-log.jsonl"
		root := rootWithFile(t, evidencePath, `{"anything":"goes"}`+"\n")

		code := run([]string{"example-org/tracker", "main", "--evidence-file", evidencePath, "--root", root})
		if code != deskkit.ExitRefused {
			t.Fatalf("exit = %d, want %d (stderr %q)", code, deskkit.ExitRefused, errBuf.String())
		}
		if len(f.hits) != 0 {
			t.Fatalf("class-guard refusal must not reach the forge: %v", f.hits)
		}
	})
}
