//go:build !windows

package main

import (
	"os"
	"testing"
	"time"
)

// regression: F-muhar-budget
func TestBudgetExternalSignal(t *testing.T) {
	out := runShell("", "kill -KILL $$", os.Environ(), time.Second)
	if out.Err == nil || out.Failed || out.TimedOut {
		t.Fatalf("external signal became a verdict: %+v", out)
	}
}
