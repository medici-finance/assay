package terminal

import (
	"bytes"
	"errors"
	"sync"
)

// Bound the ring by bytes to absorb pathologically wide lines, and by lines
// to bound the visible scrollback. These compatibility defaults are retained.
const (
	// DefaultRingBytes caps buffered bytes at 1 MiB.
	DefaultRingBytes = 1 << 20
	// DefaultRingLines caps scrollback at 10,000 lines.
	DefaultRingLines = 10000
)

// ErrCursorEvicted is returned by [ring.Since] when the requested cursor is
// older than the ring's oldest retained byte — the resume point has scrolled
// off. It is a TYPED error, never a silent truncation: the terminal stream's SSE stream
// turns it into a `resync` event so a late client re-reads from the base rather
// than resuming from a hole (spec §5.1). Silent truncation is exactly the
// off-by-one the spike's per-byte cursor was chosen to make impossible.
var ErrCursorEvicted = errors.New("terminal scrollback cursor evicted (older than the ring base)")

// ring is the in-memory, cursor-indexed scrollback for one handle. The cursor
// is a CUMULATIVE BYTE OFFSET (spike ANSWER 1): each write returns the ending
// offset of its bytes in the whole session stream, and [ring.Since] replays the
// bytes from a given offset to the end. Per-byte granularity is what makes
// resume exact regardless of how the driver chunked its writes.
//
// Memory only — no field, method, or helper here touches the filesystem
// (norest_test.go audits the package).
type ring struct {
	mu       sync.Mutex
	buf      []byte
	total    uint64 // cumulative bytes ever written; also the latest cursor
	maxBytes int
	maxLines int
}

func newRing(maxBytes, maxLines int) *ring {
	if maxBytes <= 0 {
		maxBytes = DefaultRingBytes
	}
	if maxLines <= 0 {
		maxLines = DefaultRingLines
	}
	return &ring{maxBytes: maxBytes, maxLines: maxLines}
}

// Write appends p to the scrollback, evicts down to the caps, and returns the
// new cursor — the cumulative byte offset at the END of p. A stream client that
// remembers this value resumes exactly after p.
func (r *ring) Write(p []byte) uint64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.buf = append(r.buf, p...)
	r.total += uint64(len(p))
	r.evict()
	return r.total
}

// evict drops the oldest bytes until BOTH caps hold. It drops whole lines where
// it can (up to and including the next newline) so a resumed client never sees
// a byte-severed first line; if a single line exceeds the byte cap it drops a
// raw prefix rather than growing without bound.
func (r *ring) evict() {
	for len(r.buf) > r.maxBytes {
		if !r.dropOldestLine() {
			// One line longer than the whole byte cap: drop a raw prefix.
			drop := len(r.buf) - r.maxBytes
			r.buf = append([]byte(nil), r.buf[drop:]...)
		}
	}
	for bytes.Count(r.buf, []byte{'\n'}) > r.maxLines {
		if !r.dropOldestLine() {
			break
		}
	}
}

// dropOldestLine removes bytes up to and including the first newline. It
// reports whether it dropped anything (false when there is no newline to drop
// at). It re-slices into a fresh backing array so the retained window does not
// pin the whole grown buffer.
func (r *ring) dropOldestLine() bool {
	i := bytes.IndexByte(r.buf, '\n')
	if i < 0 {
		return false
	}
	r.buf = append([]byte(nil), r.buf[i+1:]...)
	return true
}

// Since returns a copy of the bytes written after cursor c, or [ErrCursorEvicted]
// if c is older than the ring's oldest retained byte. A cursor at or beyond the
// current end returns an empty slice and no error (nothing new yet).
func (r *ring) Since(c uint64) ([]byte, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	base := r.total - uint64(len(r.buf))
	if c < base {
		return nil, ErrCursorEvicted
	}
	if c >= r.total {
		return []byte{}, nil
	}
	off := c - base
	return append([]byte(nil), r.buf[off:]...), nil
}

// readFrom is [ring.Since] for a live follower (follow.go): it returns the
// bytes after cursor c together with start, the cursor those bytes begin at.
// When c cannot be honoured exactly — older than the retained window
// ([ErrCursorEvicted]) or beyond anything this ring has issued — reset is true
// and the bytes are the WHOLE retained window from start = base. Verdict and
// bytes are read under one lock, so a caller that announces the reset and
// continues from start can neither skip a retained byte nor race a concurrent
// eviction between "how far back can I go" and "give me from there".
func (r *ring) readFrom(c uint64) (b []byte, start uint64, reset bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	base := r.total - uint64(len(r.buf))
	if c < base || c > r.total {
		return append([]byte(nil), r.buf...), base, true
	}
	return append([]byte(nil), r.buf[c-base:]...), c, false
}

// Cursor is the latest cursor (the cumulative byte total).
func (r *ring) Cursor() uint64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.total
}

// tail returns a copy of up to n most-recent bytes, for prompt/idle detection
// and OSC-title scanning.
func (r *ring) tail(n int) []byte {
	r.mu.Lock()
	defer r.mu.Unlock()
	if n >= len(r.buf) {
		return append([]byte(nil), r.buf...)
	}
	return append([]byte(nil), r.buf[len(r.buf)-n:]...)
}
