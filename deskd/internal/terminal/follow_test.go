package terminal

import (
	"bytes"
	"testing"
	"time"

	"github.com/medici-finance/assay/deskd/config/schema"
)

// TestRingReadFromBoundaries pins the cursor arithmetic the terminal stream's stream
// stands on, at every boundary where an off-by-one could hide: the base, the
// byte before it, the end, and one past the end.
func TestRingReadFromBoundaries(t *testing.T) {
	r := newRing(16, 100) // 16-byte cap: "aaaa\nbbbb\ncccc\ndddd\n" (20 B) evicts the first line
	r.Write([]byte("aaaa\nbbbb\ncccc\ndddd\n"))
	const base, total = 5, 20
	retained := []byte("bbbb\ncccc\ndddd\n")

	for _, tc := range []struct {
		name      string
		cursor    uint64
		wantBytes []byte
		wantStart uint64
		wantReset bool
	}{
		{"one byte before the base is evicted", base - 1, retained, base, true},
		{"cursor zero is evicted", 0, retained, base, true},
		{"exactly the base is honoured, whole window", base, retained, base, false},
		{"mid-window resumes strictly after the cursor", base + 5, retained[5:], base + 5, false},
		{"the last byte", total - 1, []byte("\n"), total - 1, false},
		{"caught up: nothing owed, no reset", total, []byte{}, total, false},
		{"one past the end was never issued: reset", total + 1, retained, base, true},
		{"far future (a restarted service): reset", 1 << 40, retained, base, true},
	} {
		b, start, reset := r.readFrom(tc.cursor)
		if !bytes.Equal(b, tc.wantBytes) || start != tc.wantStart || reset != tc.wantReset {
			t.Errorf("%s: readFrom(%d) = (%q, %d, %v), want (%q, %d, %v)",
				tc.name, tc.cursor, b, start, reset, tc.wantBytes, tc.wantStart, tc.wantReset)
		}
		// The invariant the stream relies on: start + len(bytes) is always the
		// ring's latest cursor — the read never stops short.
		if start+uint64(len(b)) != total {
			t.Errorf("%s: start+len = %d, want the latest cursor %d", tc.name, start+uint64(len(b)), total)
		}
	}
}

func followFixture(t *testing.T) *Registry {
	t.Helper()
	return NewRegistry(&schema.File{Cells: []schema.Cell{{
		Name:  "c",
		Loops: []schema.Loop{{Role: schema.RoleDispatch, App: "a"}},
	}}})
}

// TestFollowerWakesCoalesceBytesDoNot — a follower that is not draining gets
// ONE pending wake however many chunks arrive (so Ingest never blocks on it),
// and still reads every byte, because bytes come from the ring by cursor.
func TestFollowerWakesCoalesceBytesDoNot(t *testing.T) {
	reg := followFixture(t)
	f, ok := reg.Follow("c/dispatch")
	if !ok {
		t.Fatal("Follow refused a configured desk")
	}
	defer f.Close()
	h := reg.handle("c/dispatch")

	done := make(chan struct{})
	go func() {
		for i := 0; i < 1000; i++ {
			h.Ingest([]byte("0123456789"))
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		go func() { // unwedge the blocked Ingest so the deferred Close can take the lock
			for range f.Wake() {
			}
		}()
		t.Fatal("Ingest blocked behind a follower that was not draining its wake channel")
	}
	if n := len(f.Wake()); n != 1 {
		t.Fatalf("pending wakes = %d, want exactly 1 (coalesced)", n)
	}
	b, start, reset := f.ReadFrom(0)
	if reset || start != 0 || len(b) != 10000 {
		t.Fatalf("ReadFrom(0) = (%d bytes, start %d, reset %v), want all 10000 bytes from 0", len(b), start, reset)
	}
}

// TestFollowerLifecycle — Follow refuses only an unconfigured desk; Close is
// idempotent and returns the follower count to zero; a closed follower is no
// longer poked.
func TestFollowerLifecycle(t *testing.T) {
	reg := followFixture(t)
	if _, ok := reg.Follow("c/nope"); ok {
		t.Fatal("Follow accepted a desk id that names no configured desk")
	}
	if _, ok := (*Registry)(nil).Follow("c/dispatch"); ok {
		t.Fatal("Follow on a nil registry must refuse, not panic")
	}
	a, _ := reg.Follow("c/dispatch")
	b, _ := reg.Follow("c/dispatch")
	if n := reg.Followers("c/dispatch"); n != 2 {
		t.Fatalf("followers = %d, want 2", n)
	}
	a.Close()
	a.Close()
	if n := reg.Followers("c/dispatch"); n != 1 {
		t.Fatalf("followers after closing one (twice) = %d, want 1", n)
	}
	reg.handle("c/dispatch").Ingest([]byte("x"))
	if len(a.Wake()) != 0 || len(b.Wake()) != 1 {
		t.Fatalf("wakes: closed=%d live=%d, want 0 and 1", len(a.Wake()), len(b.Wake()))
	}
	b.Close()
	if n := reg.Followers("c/dispatch"); n != 0 {
		t.Fatalf("followers after closing all = %d, want 0", n)
	}
	// An unattached desk is could-not-check through a follower too.
	c, _ := reg.Follow("c/dispatch")
	defer c.Close()
	reg2 := followFixture(t)
	d, _ := reg2.Follow("c/dispatch")
	defer d.Close()
	if st, _, _ := d.State(); st != StateCouldNotCheck {
		t.Fatalf("unattached desk state via follower = %q, want could-not-check", st)
	}
}
