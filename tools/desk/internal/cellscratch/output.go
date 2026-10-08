package cellscratch

import "sync"

// Tail writes a fixed-size diagnostic selection, never an append-only transcript.
// Atomic replacement cannot follow a planted output symlink or hard link.
type Tail struct {
	run   *Run
	mu    sync.Mutex
	bytes []byte
	err   error
}

func (r *Run) Tail() *Tail { return &Tail{run: r} }
func (t *Tail) Write(p []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	n := len(p)
	if len(p) > TailLimit {
		p = p[len(p)-TailLimit:]
	}
	if len(t.bytes)+len(p) > TailLimit {
		t.bytes = t.bytes[len(t.bytes)+len(p)-TailLimit:]
	}
	t.bytes = append(t.bytes, p...)
	if t.err == nil {
		t.err = t.run.Store.atomic(t.run.Record.ID+"/diagnostic.txt", t.bytes)
	}
	return n, t.err
}
func (t *Tail) Err() error { t.mu.Lock(); defer t.mu.Unlock(); return t.err }
