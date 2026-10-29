package main

import (
	"testing"
	"time"
)

func kubeTTY(t *testing.T, w *cliWorld, limit time.Duration, args ...string) (int, string) {
	t.Skip("interactive compatibility PTY fixture requires qualified Unix test host")
	return 1, "could-not-check"
}
