package deskkit

// audittail.go — the bounded REVERSE reader over the audit ledger (#1035).
//
// THE DEFECT THIS CLOSES. Every question the desk tools ask of `audit.jsonl` used to be
// answered by `LoadEntries()`, which parses the whole file. Two of those questions do not
// need the whole file:
//
//   - `Guard`'s disarm-transition check reads ONE field of the LAST line (killswitch.go's
//     lastResultWas), and paid a full parse for it on every invocation of every desk verb —
//     ~0.6 s against a 105 MB ledger, read-only verbs included;
//   - the rate limiter's meters discard everything older than `rateWindow` and stop their
//     breaker walk at the first in-scope progress entry, so they too read far less than they
//     were handed.
//
// This file is the reader those two now use: it walks a file backwards in fixed blocks and
// hands complete lines to a callback newest-first, stopping the moment the caller says the
// answer is determined.
//
// THE RULE THAT GOVERNS IT. A bounded read returns an answer ONLY when that answer is
// provably identical to the one the full parse would give. Where determinacy cannot be
// established the caller falls back to `LoadEntries()` — it never narrows a meter. That
// direction matters: a reader that stops too early returns a SMALLER history, and a smaller
// history means a lower charged count, a shorter consecutive-refusal run and a "not already
// done" answer, every one of which reads as PERMISSION.
//
// The reader is deliberately not a `Scanner`: `bufio.Scanner` only moves forwards, and
// seeking to EOF and walking back is the whole point.

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
)

const (
	// tailBlockSize is the fixed block the reverse reader pulls per step. One block is
	// enough for the last entry of any real ledger (audit lines measure in hundreds of
	// bytes), which is what makes LastEntry O(1) in the ledger's size.
	tailBlockSize = 64 * 1024
	// tailMaxLine mirrors LoadEntries' own 4 MiB line cap, so the two readers refuse the
	// same pathological line rather than one of them silently truncating it.
	tailMaxLine = 4 * 1024 * 1024
)

// tailBytes counts the bytes the reverse reader has read in this process. It exists so a
// test can ASSERT the read is bounded rather than take the claim on trust: the same
// assertion against a small and a huge ledger must come back equal-and-bounded, not merely
// "smaller". It is never read on a production path.
var tailBytes int64

// TailBytesRead reports the bytes read by the reverse reader since the last reset. Test
// instrumentation; it has no production caller.
func TailBytesRead() int64 { return atomic.LoadInt64(&tailBytes) }

// ResetTailBytesRead zeroes the reverse reader's byte counter. Test instrumentation.
func ResetTailBytesRead() { atomic.StoreInt64(&tailBytes, 0) }

// TailLines returns the newest n raw ledger lines, OLDEST FIRST among those returned, read
// backwards across the daily segments. The lines are copies of what is on disk, byte for
// byte: no reserialisation, so what an operator reads is what a meter read.
//
// present reports whether any ledger file exists at all — an absent ledger is empty history
// (it looked, and there is nothing), not a fault, and the caller says so distinctly rather
// than printing the same empty output a genuinely empty file would produce. An existing file
// it cannot read IS an error.
func TailLines(n int) (lines []string, present bool, err error) {
	if n <= 0 {
		return nil, false, Refused("deskaudit tail: N must be a positive integer")
	}
	paths, perr := segmentPaths()
	if perr != nil {
		return nil, false, perr
	}
	var newestFirst []string
	for i := len(paths) - 1; i >= 0 && len(newestFirst) < n; i-- {
		serr := scanBackwards(paths[i], func(line []byte) bool {
			newestFirst = append(newestFirst, string(line))
			return len(newestFirst) < n
		})
		if serr != nil {
			if os.IsNotExist(serr) {
				continue
			}
			return nil, present, Unverifiable("cannot read audit file "+filepath.Base(paths[i]), serr)
		}
		present = true
	}
	for i := len(newestFirst) - 1; i >= 0; i-- {
		lines = append(lines, newestFirst[i])
	}
	return lines, present, nil
}

// scanBackwards calls fn with each non-blank line of path, NEWEST FIRST, until fn returns
// false or the start of the file is reached.
//
// The slice handed to fn aliases an internal buffer that the next step reuses: a callback
// that keeps the bytes past its own return must copy them.
//
// Errors: the file's own open/read errors verbatim (os.IsNotExist is the caller's to
// interpret — a missing ledger is empty history, not a fault), and a refusal for a line
// over tailMaxLine, which is the same bound LoadEntries enforces.
func scanBackwards(path string, fn func(line []byte) bool) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		return err
	}
	pos := fi.Size()
	buf := make([]byte, tailBlockSize)
	// carry holds the bytes of the line that straddles the block boundary — the front of
	// the block just consumed, which the NEXT (older) block completes.
	var carry []byte

	for pos > 0 {
		n := int64(tailBlockSize)
		if pos < n {
			n = pos
		}
		pos -= n
		if _, rerr := f.ReadAt(buf[:n], pos); rerr != nil {
			return rerr
		}
		atomic.AddInt64(&tailBytes, n)

		chunk := make([]byte, 0, int(n)+len(carry))
		chunk = append(chunk, buf[:n]...)
		chunk = append(chunk, carry...)
		carry = nil

		for {
			i := bytes.LastIndexByte(chunk, '\n')
			if i < 0 {
				break
			}
			line := chunk[i+1:]
			if len(line) > tailMaxLine {
				return Unverifiable(fmt.Sprintf("audit line in %s exceeds %d bytes — run `deskaudit recover`", path, tailMaxLine), nil)
			}
			if len(bytes.TrimSpace(line)) > 0 && !fn(line) {
				return nil
			}
			chunk = chunk[:i]
		}
		if len(chunk) > tailMaxLine {
			return Unverifiable(fmt.Sprintf("audit line in %s exceeds %d bytes — run `deskaudit recover`", path, tailMaxLine), nil)
		}
		carry = chunk
	}

	if len(bytes.TrimSpace(carry)) > 0 {
		fn(carry)
	}
	return nil
}
