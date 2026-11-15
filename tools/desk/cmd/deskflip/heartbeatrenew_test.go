package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// regression: #6510
func TestFlipRenewsOnlyRealFlip(t *testing.T) {
	yes, no := true, false
	for _, tc := range []struct {
		name  string
		o     flipOpts
		err   error
		renew bool
	}{
		{"real flip", flipOpts{pr: 1, repo: "o/r", flipped: &yes}, nil, true},
		{"dry run", flipOpts{pr: 1, repo: "o/r", dryRun: true, flipped: &no}, nil, false},
		{"already ready", flipOpts{pr: 1, repo: "o/r", flipped: &no}, nil, false},
		{"refused", flipOpts{pr: 1, repo: "o/r", flipped: &no}, errors.New("refused"), false},
		{"backstop revert", flipOpts{pr: 1, repo: "o/r", flipped: &yes}, errors.New("reverted"), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := renewsLease(tc.o, tc.err); got != tc.renew {
				t.Fatalf("renewsLease = %v, want %v", got, tc.renew)
			}
			home := t.TempDir()
			t.Setenv("HOME", home)
			dir := filepath.Join(home, ".config", "assay")
			if err := os.MkdirAll(dir, 0o700); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(dir, "HEARTBEAT")
			if err := os.WriteFile(path, []byte("ok"), 0o600); err != nil {
				t.Fatal(err)
			}
			old := time.Now().Add(-2 * time.Hour).Truncate(time.Second)
			if err := os.Chtimes(path, old, old); err != nil {
				t.Fatal(err)
			}
			audit(tc.o, tc.err)
			fi, err := os.Stat(path)
			if err != nil {
				t.Fatal(err)
			}
			if renewed := fi.ModTime().After(old); renewed != tc.renew {
				t.Fatalf("lease renewed=%v, want %v", renewed, tc.renew)
			}
		})
	}
}
