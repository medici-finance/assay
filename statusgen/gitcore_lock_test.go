package main

import (
	"testing"
	"time"
)

// regression: #2395
func TestGoGitSessionLock(t *testing.T) {
	root, _ := revisionFixture(t)
	end := beginGitReadSession()
	defer end()
	s := activeGitReads
	// Existing readers call typed adapters while holding the run memo lock.
	// The canonical repository cache must never recursively acquire that lock.
	s.mu.Lock()
	done := make(chan gitCoreRead, 1)
	go func() { done <- gitCoreFor(root) }()
	select {
	case read := <-done:
		s.mu.Unlock()
		if read.err != nil || read.native != "" || read.repo == nil {
			t.Fatalf("canonical repository cache did not open: %v %q", read.err, read.native)
		}
	case <-time.After(5 * time.Second):
		s.mu.Unlock()
		<-done // release the blocked reader before session cleanup, even on RED
		t.Fatal("canonical repository cache recursively acquired the run memo lock")
	}
}
