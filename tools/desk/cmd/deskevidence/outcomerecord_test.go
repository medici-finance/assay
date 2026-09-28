package main

import (
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// TestOutcomeRecordWrite is Verify row 3: the --outcome-record landing shape.
func TestOutcomeRecordWrite(t *testing.T) {
	t.Run("lands at the path RecordName gives", func(t *testing.T) {
		f, errBuf := setupFake(t)
		line := `{"ts":"2026-09-07T02:00:00Z","brief":"example-stream/14","outcome":"verify-fail","sha":"0000002"}`
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
		line := `{"ts":"2026-09-07T02:00:00Z","brief":"example-stream/14","outcome":"verify-fail","sha":"0000002"}`
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
		line := `{"ts":"2026-09-07T02:00:00Z","brief":"example-stream/14","outcome":"verify-fail","sha":"0000002","note":"a"}`
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
		line := `{"ts":"2026-09-07T02:00:00Z","brief":"example-stream/14","outcome":"verify-fail","sha":"0000002"}`
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
		line := `{"ts":"` + withinSkewTS + `","brief":"example-stream/14","outcome":"verify-fail","sha":"0000002"}`
		recFile := writeRepoFile(t, "record.json", line+"\n")

		code := run([]string{"example-org/tracker", "main", "--outcome-record", recFile})
		if code != deskkit.ExitOK {
			t.Fatalf("exit = %d, want 0 (stderr %q)", code, errBuf.String())
		}
		if f.putCalls != 1 {
			t.Fatalf("expected exactly 1 write, got %d", f.putCalls)
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
