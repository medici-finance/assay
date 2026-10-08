package main

import (
	"os"
	"sync"
	"time"
)

// A stamp-validated read memo for the files and directories the --lint checks
// read over and over (forge-neutral/18, Verify row 13).
//
// Measured before it, on a 420-brief tree: one offline --lint made about 11,200
// os.ReadFile calls and about 800 os.ReadDir calls. 8,020 of the reads came from
// parseBriefFile alone (19 per brief: the thirty-odd checks that walk the brief
// tree each read every brief again), and 700 of the directory reads came from
// briefFilePaths. The parse memo already skipped the PARSE on a repeat call, but
// not the read, because it keys on a hash of the content it has just read.
//
// What this memo adds: a repeat read of an UNCHANGED file is answered from memory
// after one stat call, instead of an open, a read and a close.
//
// "A cache must not outlive its subject." The memo never serves bytes the file no
// longer holds:
//
//   - Every call stats the path. An entry is served only when the stamp is
//     identical: size, modification time, change time, inode and device.
//   - Change time (ctime) is in the stamp because no ordinary write can set it back.
//     An edit that keeps the size and restores the old mtime still moves ctime.
//   - The stamp is taken before the read and again after it. If the two differ,
//     the file changed during the read and nothing is stored.
//   - "Racily clean" guard. An entry is served only when both of its times are at
//     least readMemoSettleWindow older than the moment the read began. A coarse
//     filesystem can record two writes under one timestamp tick, so a same-size
//     edit inside that tick leaves the stamp unchanged (#1407). Any write
//     AFTER the read began stamps a ctime no older than that moment, less one tick.
//     So once the stored times sit a full window before it, a later write always
//     changes the stamp. A file written within the window is re-read on every
//     call, which is how the memo behaves for anything a test or this process has
//     just written. This is the same reasoning git applies to "racily clean" index
//     entries.
//   - Platforms whose stat does not expose ctime (see readmemo_ctime_*.go) never
//     take the fast path. They read the file every time, exactly as before.
//
// The window assumes the filesystem's clock and this process's clock agree to
// within the window, less one timestamp tick. That holds for a local checkout.
//
// What it does not change: the bytes every caller receives, every error, and the
// parse memo above it. That memo still keys on a content hash, so Verify row 9's
// instrument and its mutation are untouched. A read error is never cached; the
// next call reads again.

// readMemoSettleWindow is how much older than the read an entry's times must be
// before a stat match may stand in for the read. It covers a 2 s timestamp tick
// (FAT, the coarsest in common use) with one more second of margin.
const readMemoSettleWindow = 3 * time.Second

// readMemoNow is the clock the settle guard reads. Tests replace it to put an
// entry past its window without sleeping. Production code never assigns it.
var readMemoNow = time.Now

// fileStamp is what a stat says about a path. ok is false when this platform's
// stat carries no ctime: such a stamp never validates an entry.
type fileStamp struct {
	size     int64
	mtime    time.Time
	ctime    time.Time
	ino, dev uint64
	dir      bool
	ok       bool
}

func statStamp(path string) (fileStamp, bool) {
	fi, err := os.Stat(path)
	if err != nil {
		return fileStamp{}, false
	}
	st := fileStamp{size: fi.Size(), mtime: fi.ModTime(), dir: fi.IsDir()}
	st.ctime, st.ino, st.dev, st.ok = statCtime(fi)
	return st, st.ok
}

func (a fileStamp) equal(b fileStamp) bool {
	return a.ok && b.ok && a.size == b.size && a.dir == b.dir && a.ino == b.ino && a.dev == b.dev &&
		a.mtime.Equal(b.mtime) && a.ctime.Equal(b.ctime)
}

// settledBefore reports whether both of the stamp's times are a full window
// older than t, the moment the read that produced the entry began.
func (a fileStamp) settledBefore(t time.Time) bool {
	cut := t.Add(-readMemoSettleWindow)
	return a.mtime.Before(cut) && a.ctime.Before(cut)
}

type readMemoEntry struct {
	stamp fileStamp
	began time.Time // the moment the read began, from readMemoNow
	data  []byte
	dir   []os.DirEntry
}

var (
	readMemoMu    sync.Mutex
	readMemoFiles = map[string]readMemoEntry{}
	readMemoDirs  = map[string]readMemoEntry{}
	// readMemoDiskReads counts reads that went to disk (a file read or a
	// directory listing). Tests assert against it to prove a hit really skipped
	// the read.
	readMemoDiskReads int
)

// resetReadMemo clears both memos and the disk-read counter. Tests use it so one
// test's entries cannot answer another's reads.
func resetReadMemo() {
	readMemoMu.Lock()
	readMemoFiles = map[string]readMemoEntry{}
	readMemoDirs = map[string]readMemoEntry{}
	readMemoDiskReads = 0
	readMemoMu.Unlock()
}

// readMemoLookup returns the entry for path when the current stamp validates it.
func readMemoLookup(m map[string]readMemoEntry, path string, now fileStamp) (readMemoEntry, bool) {
	readMemoMu.Lock()
	e, ok := m[path]
	readMemoMu.Unlock()
	if !ok || !e.stamp.equal(now) || !e.stamp.settledBefore(e.began) {
		return readMemoEntry{}, false
	}
	return e, true
}

// readFileMemo returns the same bytes and error os.ReadFile(path) would. The
// returned slice is the caller's own copy.
func readFileMemo(path string) ([]byte, error) {
	before, ok := statStamp(path)
	if ok && !before.dir {
		if e, hit := readMemoLookup(readMemoFiles, path, before); hit {
			return append([]byte(nil), e.data...), nil
		}
	}
	began := readMemoNow()
	raw, err := os.ReadFile(path)
	readMemoMu.Lock()
	readMemoDiskReads++
	readMemoMu.Unlock()
	if err != nil || !ok || before.dir {
		return raw, err
	}
	if after, ok2 := statStamp(path); ok2 && after.equal(before) && before.settledBefore(began) {
		readMemoMu.Lock()
		readMemoFiles[path] = readMemoEntry{stamp: before, began: began, data: append([]byte(nil), raw...)}
		readMemoMu.Unlock()
	}
	return raw, nil
}

// readDirMemo returns the same entries and error os.ReadDir(path) would. A
// directory's own mtime and ctime move when an entry is added, removed or
// renamed in it, so the stamp covers the listing.
func readDirMemo(path string) ([]os.DirEntry, error) {
	before, ok := statStamp(path)
	if ok && before.dir {
		if e, hit := readMemoLookup(readMemoDirs, path, before); hit {
			return append([]os.DirEntry(nil), e.dir...), nil
		}
	}
	began := readMemoNow()
	ents, err := os.ReadDir(path)
	readMemoMu.Lock()
	readMemoDiskReads++
	readMemoMu.Unlock()
	if err != nil || !ok || !before.dir {
		return ents, err
	}
	if after, ok2 := statStamp(path); ok2 && after.equal(before) && before.settledBefore(began) {
		readMemoMu.Lock()
		readMemoDirs[path] = readMemoEntry{stamp: before, began: began, dir: append([]os.DirEntry(nil), ents...)}
		readMemoMu.Unlock()
	}
	return ents, nil
}
