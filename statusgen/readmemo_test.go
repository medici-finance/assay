package main

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// readMemoTestSetup clears the memo, and with settled=true moves the memo's clock
// an hour ahead, so a file this test has just written already counts as settled.
// Without that, nothing a test writes is ever served from the memo, because it was
// written inside the settle window.
func readMemoTestSetup(t *testing.T, settled bool) {
	t.Helper()
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("the stamp fast path reads ctime on darwin and linux only; elsewhere every call reads from disk")
	}
	resetReadMemo()
	prev := readMemoNow
	if settled {
		readMemoNow = func() time.Time { return time.Now().Add(time.Hour) }
	}
	t.Cleanup(func() {
		readMemoNow = prev
		resetReadMemo()
	})
}

func readMemoDiskReadCount() int {
	readMemoMu.Lock()
	defer readMemoMu.Unlock()
	return readMemoDiskReads
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// A settled, unchanged file is read from disk once, however often it is asked
// for, and every caller gets its own copy of the bytes.
func TestReadMemoServesSettledUnchangedFileFromOneRead(t *testing.T) {
	readMemoTestSetup(t, true)
	p := filepath.Join(t.TempDir(), "brief-01.md")
	writeTestFile(t, p, "# Brief 01\n")

	for i := 0; i < 19; i++ {
		got, err := readFileMemo(p)
		if err != nil || string(got) != "# Brief 01\n" {
			t.Fatalf("call %d: got %q, %v", i, got, err)
		}
		got[0] = 'X' // a caller scribbling on its slice must not reach the memo
	}
	if n := readMemoDiskReadCount(); n != 1 {
		t.Fatalf("19 reads of one settled, unchanged file went to disk %d times, want 1", n)
	}
}

// The parse memo sits on top of the read memo: the same 19 parses of one settled
// brief cost one disk read, and the parse result is unchanged.
func TestParseBriefFileReadsASettledBriefOnce(t *testing.T) {
	readMemoTestSetup(t, true)
	resetBriefParseMemo()
	t.Cleanup(resetBriefParseMemo)
	p := filepath.Join(t.TempDir(), "brief-01.md")
	writeTestFile(t, p, "# Brief 01\n\nno frontmatter, so legacy and exempt\n")

	for i := 0; i < 19; i++ {
		bf, found, err := parseBriefFile(p)
		if bf != nil || found || err != nil {
			t.Fatalf("call %d: got (%v, %v, %v), want the legacy-exempt (nil, false, nil)", i, bf, found, err)
		}
	}
	if n := readMemoDiskReadCount(); n != 1 {
		t.Fatalf("19 parses of one settled brief read it from disk %d times, want 1", n)
	}
}

// An edit that keeps the size and puts the old mtime back is still seen: the
// file's ctime moved, and ctime is part of the stamp.
func TestReadMemoRereadsSameSizeEditWithRestoredMtime(t *testing.T) {
	readMemoTestSetup(t, true)
	p := filepath.Join(t.TempDir(), "brief-01.md")
	writeTestFile(t, p, "gate: model\n")
	fi, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := readFileMemo(p); string(got) != "gate: model\n" {
		t.Fatalf("first read: %q", got)
	}

	time.Sleep(20 * time.Millisecond) // a ctime strictly after the first one
	writeTestFile(t, p, "gate: human\n")
	if err := os.Chtimes(p, fi.ModTime(), fi.ModTime()); err != nil {
		t.Fatal(err)
	}
	got, err := readFileMemo(p)
	if err != nil || string(got) != "gate: human\n" {
		t.Fatalf("after a same-size edit with the mtime restored: got %q, %v — the memo served bytes the file no longer holds", got, err)
	}
}

// A file whose times fall inside the settle window is read from disk on every
// call. This is the case where a coarse timestamp tick could hide a same-size edit
// (#1407), and the memo must never answer it.
func TestReadMemoNeverServesAnUnsettledFile(t *testing.T) {
	readMemoTestSetup(t, false) // the real clock: the file below was written just now
	p := filepath.Join(t.TempDir(), "brief-01.md")
	writeTestFile(t, p, "example-a/01\n")

	// Two reads with no edit between them. The stamp matches, so only the settle
	// guard can send the second read to disk. On a filesystem with nanosecond
	// timestamps a real same-size edit would also move the stamp, so an edit here
	// could not tell whether the guard ran.
	for i := 0; i < 2; i++ {
		if got, err := readFileMemo(p); err != nil || string(got) != "example-a/01\n" {
			t.Fatalf("read %d: got %q, %v", i, got, err)
		}
	}
	if n := readMemoDiskReadCount(); n != 2 {
		t.Fatalf("two reads of a file written inside the settle window went to disk %d times, want 2 — an unsettled stamp was trusted", n)
	}
	writeTestFile(t, p, "example-a/03\n")
	got, err := readFileMemo(p)
	if err != nil || string(got) != "example-a/03\n" {
		t.Fatalf("got %q, %v after a same-size rewrite", got, err)
	}
}

// A read error is never stored, and a file that disappears after it was stored is
// reported missing rather than served from memory.
func TestReadMemoNeverCachesOrHidesAMissingFile(t *testing.T) {
	readMemoTestSetup(t, true)
	p := filepath.Join(t.TempDir(), "brief-01.md")

	if _, err := readFileMemo(p); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("missing file: err %v, want not-exist", err)
	}
	writeTestFile(t, p, "now here\n")
	if got, err := readFileMemo(p); err != nil || string(got) != "now here\n" {
		t.Fatalf("after creating it: got %q, %v — the earlier error was cached", got, err)
	}
	if err := os.Remove(p); err != nil {
		t.Fatal(err)
	}
	if got, err := readFileMemo(p); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("after removing it: got %q, %v — the memo answered for a file that is gone", got, err)
	}
}

// A directory listing is served from memory only while the directory is
// unchanged: adding or removing an entry moves the directory's own times.
func TestReadDirMemoSeesAddedAndRemovedEntries(t *testing.T) {
	readMemoTestSetup(t, true)
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "brief-01.md"), "one\n")

	names := func() []byte {
		t.Helper()
		ents, err := readDirMemo(dir)
		if err != nil {
			t.Fatal(err)
		}
		var b bytes.Buffer
		for _, e := range ents {
			b.WriteString(e.Name() + ";")
		}
		return b.Bytes()
	}
	if got := names(); string(got) != "brief-01.md;" {
		t.Fatalf("first listing: %q", got)
	}
	if got := names(); string(got) != "brief-01.md;" {
		t.Fatalf("second listing: %q", got)
	}
	if n := readMemoDiskReadCount(); n != 1 {
		t.Fatalf("two listings of an unchanged settled directory went to disk %d times, want 1", n)
	}

	time.Sleep(20 * time.Millisecond)
	writeTestFile(t, filepath.Join(dir, "brief-02.md"), "two\n")
	if got := names(); string(got) != "brief-01.md;brief-02.md;" {
		t.Fatalf("after adding brief-02.md: %q — the memo served a stale listing", got)
	}
	time.Sleep(20 * time.Millisecond)
	if err := os.Remove(filepath.Join(dir, "brief-01.md")); err != nil {
		t.Fatal(err)
	}
	if got := names(); string(got) != "brief-02.md;" {
		t.Fatalf("after removing brief-01.md: %q — the memo served a stale listing", got)
	}
}
