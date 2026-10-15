package deskkit

import (
	"strings"
	"testing"
	"time"
)

func TestPRCreateSeparatesMaintenanceAndCountsCreatedNumbers(t *testing.T) {
	dir := setup(t)
	now := time.Now().UTC().Truncate(time.Second)
	for _, verb := range []string{"update", "edit"} {
		for i := 0; i < 20; i++ {
			n := 6000 + i
			appendEntry(t, dir, Entry{Tool: "deskpr", Repo: testRepo, PR: &n, Verb: verb, Result: ResultOK, TS: now.Format(time.RFC3339)})
		}
	}
	for i := 0; i < RateLimitPerPRPerHour-1; i++ {
		n := 7000 + i
		appendEntry(t, dir, Entry{Tool: "deskpr", Repo: testRepo, PR: &n, Verb: "create", Result: ResultOK, TS: now.Format(time.RFC3339)})
	}
	if err := AllowPRCreateAt(testRepo, now); err != nil {
		t.Fatalf("maintenance consumed create capacity: %v", err)
	}
	// A sent-but-unconfirmed create is still charged, including its unnumbered shape.
	appendEntry(t, dir, Entry{Tool: "deskpr", Repo: testRepo, Verb: "create", Result: ResultUnverifiable, TS: now.Format(time.RFC3339)})
	err := AllowPRCreateAt(testRepo, now)
	if !IsRateLimited(err) || !strings.Contains(err.Error(), "new-PR write budget") {
		t.Fatalf("20 creates must refuse: %v", err)
	}
	if got := RetryAfterOf(err); got != time.Hour+time.Second {
		t.Fatalf("retry-after = %v", got)
	}
	if err := AllowWriteAt("deskpr", testRepo, 9000, now); err != nil {
		t.Fatalf("create cap blocked a different PR update: %v", err)
	}
	if err := AllowPRCreateAt(testRepo, now.Add(time.Hour+time.Second)); err != nil {
		t.Fatalf("expired creates still counted: %v", err)
	}
}

func TestPRCreateRetainsAggregateAndConservativeLegacyLimits(t *testing.T) {
	for _, tc := range []struct {
		verb  string
		count int
		want  string
	}{
		{"update", RateLimitPerRepoPerHour, "per-repo"},
		{"", RateLimitPerPRPerHour, "new-PR"},
		{"pr-create", RateLimitPerPRPerHour, "new-PR"},
		{"future-write", RateLimitPerPRPerHour, "new-PR"},
	} {
		t.Run(tc.verb, func(t *testing.T) {
			dir := setup(t)
			now := time.Now()
			for i := 0; i < tc.count; i++ {
				n := 7000 + i
				appendEntry(t, dir, Entry{Tool: "deskpr", Repo: testRepo, PR: &n, Verb: tc.verb, Result: ResultOK})
			}
			err := AllowPRCreateAt(testRepo, now)
			if !IsRateLimited(err) || !strings.Contains(err.Error(), tc.want+" write budget") {
				t.Fatalf("got %v, want %s limit", err, tc.want)
			}
			if err := AllowPRCreateAt("example-org/other", now); err != nil {
				t.Fatalf("different repo blocked: %v", err)
			}
		})
	}
}
