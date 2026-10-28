package terminal

import "time"

// follow.go — the READ-ONLY live follower the terminal stream's SSE stream is built on
// (the service contract).
//
// A [Follower] is a wakeup, not a queue. The registry's pump never hands a
// follower bytes: it writes them into the handle's ring exactly as before and
// then pokes every follower's one-slot wake channel without blocking. The
// follower reads the bytes back OUT OF THE RING by cursor ([Follower.ReadFrom]).
// Three properties fall out of that shape, and they are the reason it is not a
// per-subscriber channel of chunks the way serve.EventBus is:
//
//   - a slow client can never back the pump up, and can never make the pump
//     DROP a chunk for it either — there is nothing per-client to fill. The
//     worst a slow client can do is fall behind the ring's retained window,
//     and that is reported as eviction, never papered over;
//   - live delivery and resume are one code path (read from a cursor), so
//     there is no seam between "replay" and "live" for a byte to fall into or
//     be delivered twice across;
//   - a follower holds no reference to the driver's [Session], so closing one
//     cannot touch the desk (spec §2: attach, never host). It has no write,
//     resize or close path to the session at all.

// Follower is one read-only live follower of a desk's scrollback. Obtain it
// with [Registry.Follow]; Close it when the client goes away.
type Follower struct {
	h    *Handle
	wake chan struct{}
}

// Follow registers a read-only follower on a configured desk. ok is false only
// for a desk id that names no configured desk (the caller 404s that); a
// configured desk with no target, or an unreachable one, still yields a
// follower — it simply never wakes, and its state reads could-not-check
// (spec §5.2: a row, never a miss).
func (r *Registry) Follow(id string) (f *Follower, ok bool) {
	if r == nil {
		return nil, false
	}
	h := r.handle(id)
	if h == nil {
		return nil, false
	}
	f = &Follower{h: h, wake: make(chan struct{}, 1)}
	h.mu.Lock()
	if h.followers == nil {
		h.followers = map[*Follower]struct{}{}
	}
	h.followers[f] = struct{}{}
	h.mu.Unlock()
	return f, true
}

// Followers is the number of live followers on a desk — the instrument the
// stream's leak test reads: it must return to zero once every client has
// disconnected.
func (r *Registry) Followers(id string) int {
	h := r.handle(id)
	if h == nil {
		return 0
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.followers)
}

// notifyFollowersLocked pokes every follower without blocking. The wake channel
// holds one slot: a follower that has not yet consumed the previous poke needs
// no second one, because it re-reads the ring from its own cursor and so picks
// up everything written since — wakes coalesce, bytes never do. Called with
// h.mu held.
func (h *Handle) notifyFollowersLocked() {
	for f := range h.followers {
		select {
		case f.wake <- struct{}{}:
		default:
		}
	}
}

// Wake is signalled (coalesced) whenever new output has been written to the
// desk's ring since the follower last drained it.
func (f *Follower) Wake() <-chan struct{} { return f.wake }

// ReadFrom returns the scrollback bytes after cursor, and start — the cursor
// the returned bytes BEGIN at, so the caller's next cursor is
// start+len(b).
//
// reset is true when cursor cannot be honoured exactly:
//
//   - it is older than the ring's oldest retained byte ([ErrCursorEvicted] —
//     the resume point scrolled off), or
//   - it is BEYOND the latest cursor this ring has issued. A cursor from the
//     future names bytes this ring never wrote — in practice a client
//     resuming against a restarted service whose memory-only scrollback
//     started again from zero. Treating it as "nothing new yet" would hold the
//     client silent until the new stream happened to grow past the old
//     offset, skipping everything before it; so it is a reset, exactly like
//     eviction.
//
// On reset the bytes returned are the ring's whole retained window and start is
// its base, read under the SAME lock as the verdict — a caller announces the
// reset and continues from start, and nothing retained is skipped.
func (f *Follower) ReadFrom(cursor uint64) (b []byte, start uint64, reset bool) {
	return f.h.ring.readFrom(cursor)
}

// State is the desk's current derived state, OSC title and derivation time —
// the payload of a `terminal.state` frame. It refreshes the prompt/idle signal
// first, so a prompt that settled with no new output is observed.
func (f *Follower) State() (state AgentState, title string, at time.Time) {
	f.h.Refresh()
	f.h.mu.Lock()
	defer f.h.mu.Unlock()
	if f.h.asOf != nil {
		at = *f.h.asOf
	}
	return f.h.state(), f.h.title, at
}

// Close deregisters the follower. It is idempotent, and it touches nothing but
// the follower set: the desk's session keeps running (spec §2).
func (f *Follower) Close() {
	f.h.mu.Lock()
	delete(f.h.followers, f)
	f.h.mu.Unlock()
}
