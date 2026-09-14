package deskkit

import (
	"bufio"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestLogAppendOnly proves the audit file is opened O_APPEND: write two lines and
// assert BOTH survive in order (the tools never truncate/rewrite).
func TestLogAppendOnly(t *testing.T) {
	setup(t)

	if err := Log(Entry{Tool: "deskpost", Verb: "comment", Result: ResultOK, Detail: "first"}); err != nil {
		t.Fatalf("Log #1: %v", err)
	}
	if err := Log(Entry{Tool: "deskpost", Verb: "comment", Result: ResultNoop, Detail: "second"}); err != nil {
		t.Fatalf("Log #2: %v", err)
	}

	entries, err := LoadEntries()
	if err != nil {
		t.Fatalf("LoadEntries: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2 (append must not overwrite)", len(entries))
	}
	if entries[0].Detail != "first" || entries[1].Detail != "second" {
		t.Fatalf("append order wrong: %q then %q", entries[0].Detail, entries[1].Detail)
	}
	// Defaults filled in.
	if entries[0].TS == "" || entries[0].SessionTag != "test-session" {
		t.Fatalf("defaults not filled: ts=%q session=%q", entries[0].TS, entries[0].SessionTag)
	}
	if entries[0].SourceSHA == "" || entries[0].BuiltAt == "" {
		t.Fatalf("version stamp not filled: %q %q", entries[0].SourceSHA, entries[0].BuiltAt)
	}
}

func TestLogCreatesDirAndFilePerms(t *testing.T) {
	dir := setup(t)
	if err := Log(Entry{Tool: "deskwt", Verb: "guard", Result: ResultOK}); err != nil {
		t.Fatalf("Log: %v", err)
	}
	di, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("stat dir: %v", err)
	}
	if perm := di.Mode().Perm(); perm != 0o700 {
		t.Errorf("dir perm = %o, want 700", perm)
	}
	fi, err := os.Stat(filepath.Join(dir, "audit.jsonl"))
	if err != nil {
		t.Fatalf("stat file: %v", err)
	}
	if perm := fi.Mode().Perm(); perm != 0o600 {
		t.Errorf("file perm = %o, want 600", perm)
	}
}

func TestLogRejectsMissingResult(t *testing.T) {
	setup(t)
	if err := Log(Entry{Tool: "deskpost", Verb: "comment"}); !IsUnverifiable(err) {
		t.Fatalf("Log with empty result = %v, want Unverifiable", err)
	}
}

// TestLoadEntriesMissingIsEmpty — a missing audit file is empty history, not an error
// (bootstrap).
func TestLoadEntriesMissingIsEmpty(t *testing.T) {
	setup(t)
	entries, err := LoadEntries()
	if err != nil {
		t.Fatalf("LoadEntries on missing file = %v, want nil", err)
	}
	if len(entries) != 0 {
		t.Fatalf("got %d entries, want 0", len(entries))
	}
}

// TestLoadEntriesCorruptLineUnverifiable — a malformed line is REFUSED (exit 6), not
// skipped. This is the core negative-path property downstream lookups
// inherit.
func TestLoadEntriesCorruptLineUnverifiable(t *testing.T) {
	dir := setup(t)
	appendEntry(t, dir, Entry{Tool: "deskpost", Verb: "comment", Result: ResultOK})
	appendLine(t, dir, `{"ts": this is not valid json`)

	entries, err := LoadEntries()
	if entries != nil {
		t.Fatalf("LoadEntries returned entries despite corruption: %v", entries)
	}
	if !IsUnverifiable(err) {
		t.Fatalf("LoadEntries on corrupt line = %v, want Unverifiable (exit 6)", err)
	}
	if ExitCodeOf(err) != ExitUnverifiable {
		t.Fatalf("ExitCodeOf = %d, want %d", ExitCodeOf(err), ExitUnverifiable)
	}
}

// TestLastEntryMatchesLoadEntriesTail proves LastEntry's bounded tail read agrees with
// LoadEntries's last element across the shapes that matter — empty, single line,
// multiple lines, trailing blank lines, and a corrupt tail. This is the correctness
// safety net for LastEntry/readLastLine (assay#1035): a tail scan that disagreed with a
// full scan about which bytes make up "the last entry" would be a silent behaviour
// change wearing a performance fix's clothes.
func TestLastEntryMatchesLoadEntriesTail(t *testing.T) {
	t.Run("missing file", func(t *testing.T) {
		setup(t)
		e, err := LastEntry()
		if err != nil || e != nil {
			t.Fatalf("LastEntry on missing file = (%+v, %v), want (nil, nil)", e, err)
		}
	})

	t.Run("single entry", func(t *testing.T) {
		dir := setup(t)
		appendEntry(t, dir, Entry{Tool: "deskpost", Verb: "comment", Result: ResultOK, Detail: "only"})
		e, err := LastEntry()
		if err != nil {
			t.Fatalf("LastEntry: %v", err)
		}
		if e == nil || e.Detail != "only" {
			t.Fatalf("LastEntry = %+v, want Detail=only", e)
		}
	})

	t.Run("multiple entries", func(t *testing.T) {
		dir := setup(t)
		appendEntry(t, dir, Entry{Tool: "deskpost", Verb: "comment", Result: ResultOK, Detail: "first"})
		appendEntry(t, dir, Entry{Tool: "deskpost", Verb: "comment", Result: ResultNoop, Detail: "second"})
		appendEntry(t, dir, Entry{Tool: "deskpost", Verb: "comment", Result: ResultDisabled, Detail: "third"})

		want, werr := LoadEntries()
		if werr != nil {
			t.Fatalf("LoadEntries: %v", werr)
		}
		got, gerr := LastEntry()
		if gerr != nil {
			t.Fatalf("LastEntry: %v", gerr)
		}
		wantLast := want[len(want)-1]
		if got == nil || got.Detail != wantLast.Detail || got.Result != wantLast.Result {
			t.Fatalf("LastEntry = %+v, want last of LoadEntries = %+v", got, wantLast)
		}
	})

	t.Run("trailing blank and whitespace-only lines ignored, same as LoadEntries", func(t *testing.T) {
		dir := setup(t)
		appendEntry(t, dir, Entry{Tool: "deskpost", Verb: "comment", Result: ResultOK, Detail: "real"})
		appendLine(t, dir, "")    // blank line
		appendLine(t, dir, "   ") // whitespace-only line

		want, werr := LoadEntries()
		if werr != nil {
			t.Fatalf("LoadEntries: %v", werr)
		}
		got, gerr := LastEntry()
		if gerr != nil {
			t.Fatalf("LastEntry: %v", gerr)
		}
		wantLast := want[len(want)-1]
		if got == nil || got.Detail != wantLast.Detail {
			t.Fatalf("LastEntry = %+v, want %+v (trailing blanks ignored)", got, wantLast)
		}
	})

	t.Run("malformed last line is a refusal, same class as LoadEntries", func(t *testing.T) {
		dir := setup(t)
		appendEntry(t, dir, Entry{Tool: "deskpost", Verb: "comment", Result: ResultOK})
		appendLine(t, dir, `{"ts": this is not valid json`)

		e, err := LastEntry()
		if e != nil {
			t.Fatalf("LastEntry returned an entry despite corruption: %+v", e)
		}
		if !IsUnverifiable(err) {
			t.Fatalf("LastEntry on corrupt tail = %v, want Unverifiable", err)
		}
	})
}

// countingReaderAt wraps an io.ReaderAt and counts the total bytes requested across
// every ReadAt call, so a test can assert a reader touched a BOUNDED slice of a file
// regardless of the file's overall size.
type countingReaderAt struct {
	io.ReaderAt
	bytesRequested int64
}

func (c *countingReaderAt) ReadAt(p []byte, off int64) (int, error) {
	c.bytesRequested += int64(len(p))
	return c.ReaderAt.ReadAt(p, off)
}

// TestReadLastLineBoundedRead is the O(1) regression guard for assay#1035: it plants a
// small audit log and a 1,000,000-line one (as the issue's Verify section asks for),
// reads the last line of each through a countingReaderAt, and asserts the bytes
// REQUESTED stay within a small, size-INDEPENDENT bound — proving readLastLine's cost
// does not grow with the file behind it, which is exactly the property LoadEntries
// lacked (it unmarshals every line to hand the caller the last one).
func TestReadLastLineBoundedRead(t *testing.T) {
	const bound = 4 * tailReadChunk // generous slack; real audit lines are far smaller

	measure := func(t *testing.T, path string) (line string, bytesRead int64) {
		t.Helper()
		f, err := os.Open(path)
		if err != nil {
			t.Fatalf("open: %v", err)
		}
		defer f.Close()
		size, err := f.Seek(0, 2) // io.SeekEnd
		if err != nil {
			t.Fatalf("seek: %v", err)
		}
		cr := &countingReaderAt{ReaderAt: f}
		got, err := readLastLine(cr, size)
		if err != nil {
			t.Fatalf("readLastLine: %v", err)
		}
		return string(got), cr.bytesRequested
	}

	small := filepath.Join(t.TempDir(), "small.jsonl")
	if err := os.WriteFile(small, []byte(
		`{"tool":"deskpost","result":"ok","detail":"line-1"}`+"\n"+
			`{"tool":"deskpost","result":"noop","detail":"line-2"}`+"\n"), 0o600); err != nil {
		t.Fatalf("write small fixture: %v", err)
	}

	const bigLines = 1_000_000
	big := filepath.Join(t.TempDir(), "big.jsonl")
	f, err := os.Create(big)
	if err != nil {
		t.Fatalf("create big fixture: %v", err)
	}
	w := bufio.NewWriter(f)
	for i := 0; i < bigLines-1; i++ {
		if _, err := w.WriteString(`{"tool":"deskpost","result":"ok","detail":"filler"}` + "\n"); err != nil {
			t.Fatalf("write filler line %d: %v", i, err)
		}
	}
	wantDetail := "the-actual-last-entry"
	if _, err := w.WriteString(`{"tool":"deskpost","result":"disabled","detail":"` + wantDetail + `"}` + "\n"); err != nil {
		t.Fatalf("write last line: %v", err)
	}
	if err := w.Flush(); err != nil {
		t.Fatalf("flush big fixture: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close big fixture: %v", err)
	}
	if fi, statErr := os.Stat(big); statErr != nil || fi.Size() < 40_000_000 {
		t.Fatalf("fixture too small to be a meaningful O(1) proof: %+v err=%v", fi, statErr)
	}

	smallLine, smallBytes := measure(t, small)
	bigLine, bigBytes := measure(t, big)

	if !strings.Contains(smallLine, "line-2") {
		t.Fatalf("small fixture: got %q, want the last line (line-2)", smallLine)
	}
	if !strings.Contains(bigLine, wantDetail) {
		t.Fatalf("big (1M-line) fixture: got %q, want the actual last entry (%q) — read the "+
			"wrong line entirely, not just too much of it", bigLine, wantDetail)
	}
	if smallBytes > bound {
		t.Fatalf("small fixture: readLastLine requested %d bytes, want <= %d", smallBytes, bound)
	}
	if bigBytes > bound {
		t.Fatalf("1,000,000-line (%d-byte) fixture: readLastLine requested %d bytes, want <= %d "+
			"— cost must not scale with file size (this is the assay#1035 regression: "+
			"LoadEntries would have unmarshalled all %d lines to answer the same question)",
			mustSize(t, big), bigBytes, bound, bigLines)
	}
	// The whole point: reading the tail of a file ~1000x bigger costs about the same.
	if bigBytes > smallBytes*10+bound {
		t.Fatalf("bytes requested grew with file size: small=%d big=%d (not O(1))", smallBytes, bigBytes)
	}
}

func mustSize(t *testing.T, path string) int64 {
	t.Helper()
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	return fi.Size()
}

func TestFirstTS(t *testing.T) {
	dir := setup(t)

	if ts, err := FirstTS(); err != nil || ts != "" {
		t.Fatalf("FirstTS on empty = (%q,%v), want (\"\",nil)", ts, err)
	}
	appendEntry(t, dir, Entry{Tool: "deskpost", Verb: "comment", Result: ResultOK, TS: "2020-01-01T00:00:00Z"})
	appendEntry(t, dir, Entry{Tool: "deskpost", Verb: "comment", Result: ResultOK, TS: "2021-01-01T00:00:00Z"})
	ts, err := FirstTS()
	if err != nil {
		t.Fatalf("FirstTS: %v", err)
	}
	if ts != "2020-01-01T00:00:00Z" {
		t.Fatalf("FirstTS = %q, want the earliest-appended ts", ts)
	}
}

func TestArgsDigestStable(t *testing.T) {
	a := ArgsDigest([]string{"review", "--head", "abc"})
	b := ArgsDigest([]string{"review", "--head", "abc"})
	if a != b {
		t.Fatalf("ArgsDigest not stable: %q vs %q", a, b)
	}
	if a == ArgsDigest([]string{"review", "--head", "def"}) {
		t.Fatalf("ArgsDigest collided across different args")
	}
	if len(a) != 64 {
		t.Fatalf("ArgsDigest length = %d, want 64 (sha256 hex)", len(a))
	}
}

func TestSessionTag(t *testing.T) {
	// FIRST: the desk tools' own per-agent session id. A dispatched agent is a child
	// process and inherits the harness ids of the session that dispatched it, so those
	// name the DISPATCHER; $DESK_SESSION is what distinguishes one agent from its
	// siblings, and it must outrank both.
	t.Setenv("DESK_SESSION", "  desk-sess  ") // and it is trimmed
	t.Setenv("CLAUDE_CODE_SESSION_ID", "code-sess")
	t.Setenv("CLAUDE_SESSION_ID", "legacy-sess")
	if got := SessionTag(); got != "desk-sess" {
		t.Fatalf("SessionTag = %q, want desk-sess (DESK_SESSION names the acting agent and takes precedence)", got)
	}
	// Then the variable the Claude Code harness actually exports.
	t.Setenv("DESK_SESSION", "")
	if got := SessionTag(); got != "code-sess" {
		t.Fatalf("SessionTag = %q, want code-sess (CLAUDE_CODE_SESSION_ID takes precedence)", got)
	}
	// Legacy fallback: when the primary is unset, the old variable is still honoured.
	t.Setenv("CLAUDE_CODE_SESSION_ID", "")
	if got := SessionTag(); got != "legacy-sess" {
		t.Fatalf("SessionTag = %q, want legacy-sess (legacy fallback)", got)
	}
	// None set → "unknown".
	t.Setenv("CLAUDE_SESSION_ID", "")
	if got := SessionTag(); got != "unknown" {
		t.Fatalf("SessionTag = %q, want unknown", got)
	}
}
