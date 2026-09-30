package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const outcomesSplitFixtureLog = `{"ts":"2026-09-07T01:19:23Z","brief":"example-stream/20","outcome":"verified","sha":"67abbac"}
{"ts":"2026-09-07T01:19:23Z","brief":"example-stream/09","outcome":"verify-fail","sha":"67abbac"}
{"ts":"2026-09-07T01:36:11Z","brief":"example-stream/05","outcome":"verified","sha":"67abbac"}
`

func writeOutcomesSplitFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "docs", "streams")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "verify-outcomes.jsonl"), []byte(outcomesSplitFixtureLog), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

// TestOutcomesSplit is Verify row 7.
func TestOutcomesSplit(t *testing.T) {
	t.Run("writes one file per line with bytes equal to the line plus newline", func(t *testing.T) {
		root := writeOutcomesSplitFixture(t)
		var stdout, stderr bytes.Buffer
		code := runOutcomes([]string{"split", "--root", root}, &stdout, &stderr)
		if code != 0 {
			t.Fatalf("split exit = %d (stderr %q)", code, stderr.String())
		}

		lines := splitFixtureLines(t)
		for _, line := range lines {
			name, err := outcomeRecordName([]byte(line))
			if err != nil {
				t.Fatal(err)
			}
			got, rerr := os.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
			if rerr != nil {
				t.Fatalf("record for %q not written: %v", line, rerr)
			}
			want := line + "\n"
			if string(got) != want {
				t.Fatalf("record bytes = %q, want %q", got, want)
			}
		}
	})

	t.Run("a second run writes nothing", func(t *testing.T) {
		root := writeOutcomesSplitFixture(t)
		var out1, err1, out2, err2 bytes.Buffer
		if code := runOutcomes([]string{"split", "--root", root}, &out1, &err1); code != 0 {
			t.Fatalf("first split exit = %d (stderr %q)", code, err1.String())
		}
		if code := runOutcomes([]string{"split", "--root", root}, &out2, &err2); code != 0 {
			t.Fatalf("second split exit = %d (stderr %q)", code, err2.String())
		}
		if out2.String() != "outcomes split: wrote 0 new record file(s)\n" {
			t.Fatalf("second run output = %q, want it to report zero new files", out2.String())
		}
	})

	t.Run("--check exits 1 naming a line whose record was deleted", func(t *testing.T) {
		root := writeOutcomesSplitFixture(t)
		var out1, err1 bytes.Buffer
		if code := runOutcomes([]string{"split", "--root", root}, &out1, &err1); code != 0 {
			t.Fatalf("split exit = %d (stderr %q)", code, err1.String())
		}

		lines := splitFixtureLines(t)
		deletedLine := lines[1] // "example-stream/09" verify-fail row
		name, err := outcomeRecordName([]byte(deletedLine))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Remove(filepath.Join(root, filepath.FromSlash(name))); err != nil {
			t.Fatal(err)
		}

		var out2, err2 bytes.Buffer
		code := runOutcomes([]string{"split", "--root", root, "--check"}, &out2, &err2)
		if code != 1 {
			t.Fatalf("--check exit = %d, want 1 (stdout %q, stderr %q)", code, out2.String(), err2.String())
		}
		if !strings.Contains(out2.String(), "example-stream/09") {
			t.Fatalf("--check output = %q, want it to name the deleted line's brief", out2.String())
		}
	})
}

func splitFixtureLines(t *testing.T) []string {
	t.Helper()
	return []string{
		`{"ts":"2026-09-07T01:19:23Z","brief":"example-stream/20","outcome":"verified","sha":"67abbac"}`,
		`{"ts":"2026-09-07T01:19:23Z","brief":"example-stream/09","outcome":"verify-fail","sha":"67abbac"}`,
		`{"ts":"2026-09-07T01:36:11Z","brief":"example-stream/05","outcome":"verified","sha":"67abbac"}`,
	}
}
