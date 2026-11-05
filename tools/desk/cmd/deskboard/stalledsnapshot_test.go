package main

import (
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// A complete empty snapshot is positive evidence, not a missing per-item read.
// Retaining both activity reads also prevents this optimization from changing
// the meaning of a blocked draft whose author has replied or whose head moved.
func TestStalledSnapshotComplete(t *testing.T) {
	for _, tc := range []struct {
		name, state, head string
		want              int
		recentReply       bool
	}{
		{"blocking", "CHANGES_REQUESTED", "abc123", 1, false},
		{"old head", "CHANGES_REQUESTED", "previous", 0, false},
		{"approved", "APPROVED", "abc123", 0, false},
		{"empty", "", "", 0, false},
		{"author replied", "CHANGES_REQUESTED", "abc123", 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stalledSetup(t)
			var fallback atomic.Int32
			forgeHooks.reviews = func(string, int) ([]deskkit.Review, error) {
				fallback.Add(1)
				return nil, errors.New("per-item read must not run")
			}
			forgeHooks.queueReviews = func(string, int) ([]deskkit.Review, error) {
				if tc.state == "" {
					return nil, nil
				}
				return []deskkit.Review{{Author: deskkit.Account{Login: "assay-reviewer-app[bot]"}, State: tc.state, CommitID: tc.head, SubmittedAt: "2026-08-01T00:00:00Z"}}, nil
			}
			if tc.recentReply {
				t.Setenv("DESKBOARD_GH_PRCOMMENTS_JSON", commentsJSON(stalledAuthorREST, time.Now()))
			}
			rep := runStalled(t)
			if got := fallback.Load(); got != 0 {
				t.Fatalf("complete snapshot reread reviews %d times", got)
			}
			if len(rep.Stalled) != tc.want || len(rep.Unassessable) != 0 {
				t.Fatalf("stalled=%+v unassessable=%+v", rep.Stalled, rep.Unassessable)
			}
		})
	}
}

func TestStalledSnapshotFallback(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(map[bool]string{false: "complete fallback", true: "unreadable fallback"}[fail], func(t *testing.T) {
			stalledSetup(t) // no complete queue review hook: must retain paginated fallback
			var reads atomic.Int32
			forgeHooks.reviews = func(string, int) ([]deskkit.Review, error) {
				reads.Add(1)
				if fail {
					return nil, errors.New("reviews unavailable")
				}
				return []deskkit.Review{{Author: deskkit.Account{Login: "assay-reviewer-app[bot]"}, State: "CHANGES_REQUESTED", CommitID: "abc123", SubmittedAt: "2026-08-01T00:00:00Z"}}, nil
			}
			rep := runStalled(t)
			if reads.Load() != 1 {
				t.Fatalf("missing snapshot must read reviews once, got %d", reads.Load())
			}
			if fail {
				if len(rep.Unassessable) != 1 || !strings.Contains(rep.Unassessable[0].Reason, "cannot read reviewer-App verdict") {
					t.Fatalf("failure hidden: %+v", rep)
				}
			} else if len(rep.Stalled) != 1 {
				t.Fatalf("fallback lost blocked draft: %+v", rep)
			}
		})
	}
}
