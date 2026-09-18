package heartbeat

import (
	"testing"
	"time"
)

func TestIsStaleAtExactGap(t *testing.T) {
	now := time.Now()
	last := now.Add(-20 * time.Minute)
	if !IsStale(last, now, 20*time.Minute) {
		t.Fatal("a heartbeat exactly one gap old must be reported stale")
	}
}

func TestIsStaleInsideGap(t *testing.T) {
	now := time.Now()
	last := now.Add(-19 * time.Minute)
	if IsStale(last, now, 20*time.Minute) {
		t.Fatal("a heartbeat inside the gap must not be reported stale")
	}
}

func TestIsStaleWellPastGap(t *testing.T) {
	now := time.Now()
	last := now.Add(-90 * time.Minute)
	if !IsStale(last, now, 20*time.Minute) {
		t.Fatal("a heartbeat well past the gap must be reported stale")
	}
}
