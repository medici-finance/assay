package loopengine

import (
	"errors"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

func mustParse(t *testing.T, s string) time.Time {
	t.Helper()
	ts, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatalf("bad test timestamp %q: %v", s, err)
	}
	return ts
}

// --- AuditProbe ---

func TestAuditProbe_MatchesBySessionTag(t *testing.T) {
	load := func() ([]AuditEntry, error) {
		return []AuditEntry{
			{TS: "2026-09-02T10:00:00Z", SessionTag: "other-session"},
			{TS: "2026-09-02T11:00:00Z", SessionTag: "sess-abc"},
			{TS: "2026-09-02T09:00:00Z", SessionTag: "sess-abc"}, // older — before `since`
		}, nil
	}
	probe := NewAuditProbe(load)
	it := Item{ID: "x/01", Payload: map[string]string{PayloadSessionTag: "sess-abc"}}
	since := mustParse(t, "2026-09-02T09:30:00Z")

	obs, err := probe(it, since)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := mustParse(t, "2026-09-02T11:00:00Z")
	if !obs.At.Equal(want) {
		t.Fatalf("At = %v, want %v", obs.At, want)
	}
	if obs.What != "audit line" {
		t.Fatalf("What = %q", obs.What)
	}
}

func TestAuditProbe_MatchesByRepoAndPR(t *testing.T) {
	pr := 42
	load := func() ([]AuditEntry, error) {
		return []AuditEntry{
			{TS: "2026-09-02T11:00:00Z", Repo: "medici-finance/assay", PR: &pr},
		}, nil
	}
	probe := NewAuditProbe(load)
	it := Item{ID: "x/01", Payload: map[string]string{PayloadRepo: "medici-finance/assay", PayloadPR: "42"}}
	obs, err := probe(it, mustParse(t, "2026-09-02T00:00:00Z"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if obs.At.IsZero() {
		t.Fatal("expected a positive observation, got zero")
	}
}

func TestAuditProbe_NoMatchIsCleanlySilent(t *testing.T) {
	load := func() ([]AuditEntry, error) {
		return []AuditEntry{{TS: "2026-09-02T11:00:00Z", SessionTag: "someone-else"}}, nil
	}
	probe := NewAuditProbe(load)
	it := Item{ID: "x/01", Payload: map[string]string{PayloadSessionTag: "sess-abc"}}
	obs, err := probe(it, mustParse(t, "2026-09-02T00:00:00Z"))
	if err != nil {
		t.Fatalf("no match must be clean, not an error: %v", err)
	}
	if !obs.At.IsZero() {
		t.Fatalf("expected zero Observation, got %v", obs)
	}
}

func TestAuditProbe_NoAttributionKeysIsCleanlySilent(t *testing.T) {
	called := false
	load := func() ([]AuditEntry, error) {
		called = true
		return nil, nil
	}
	probe := NewAuditProbe(load)
	it := Item{ID: "x/01"} // no Payload at all
	obs, err := probe(it, time.Time{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !obs.At.IsZero() {
		t.Fatalf("expected zero Observation, got %v", obs)
	}
	if called {
		t.Fatal("an item with no attribution keys must not even read the audit trail")
	}
}

// TestAuditProbe_UnreadableIsCouldNotCheck is the fail-first guard: an unreadable audit
// source must be an ERROR, never a clean silence — a clean silence would be misread by the
// engine as "no life", the exact eager-reclaim failure the conservative rule exists to stop.
func TestAuditProbe_UnreadableIsCouldNotCheck(t *testing.T) {
	load := func() ([]AuditEntry, error) { return nil, errors.New("disk full") }
	probe := NewAuditProbe(load)
	it := Item{ID: "x/01", Payload: map[string]string{PayloadSessionTag: "sess-abc"}}
	obs, err := probe(it, time.Time{})
	if err == nil {
		t.Fatal("an unreadable audit trail must be could-not-check, not a clean (nil) result")
	}
	if !obs.At.IsZero() {
		t.Fatalf("an errored probe must not also report a positive observation, got %v", obs)
	}
}

// --- BranchProbe ---

func TestBranchProbe_FirstSightingIsFresh(t *testing.T) {
	list := func(string) (map[string]string, error) {
		return map[string]string{"refs/heads/feat/x": "sha1"}, nil
	}
	probe := NewBranchProbe(list, NewMemBranchSHAStore())
	it := Item{ID: "x/01", Payload: map[string]string{PayloadRepo: "o/r", PayloadBranch: "feat/x"}}
	obs, err := probe(it, time.Time{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if obs.At.IsZero() {
		t.Fatal("a first-ever sighting of a branch must be reported as a fresh observation")
	}
}

func TestBranchProbe_UnchangedShaIsCleanlySilent(t *testing.T) {
	store := NewMemBranchSHAStore()
	list := func(string) (map[string]string, error) {
		return map[string]string{"refs/heads/feat/x": "sha1"}, nil
	}
	probe := NewBranchProbe(list, store)
	it := Item{ID: "x/01", Payload: map[string]string{PayloadRepo: "o/r", PayloadBranch: "feat/x"}}

	if _, err := probe(it, time.Time{}); err != nil {
		t.Fatalf("first call: %v", err)
	}
	obs, err := probe(it, time.Time{})
	if err != nil {
		t.Fatalf("second call: %v", err)
	}
	if !obs.At.IsZero() {
		t.Fatalf("an unchanged SHA must be clean silence (no NEW sign of life), got %v", obs)
	}
}

func TestBranchProbe_ChangedShaIsFreshAgain(t *testing.T) {
	store := NewMemBranchSHAStore()
	sha := "sha1"
	list := func(string) (map[string]string, error) {
		return map[string]string{"refs/heads/feat/x": sha}, nil
	}
	probe := NewBranchProbe(list, store)
	it := Item{ID: "x/01", Payload: map[string]string{PayloadRepo: "o/r", PayloadBranch: "feat/x"}}

	if _, err := probe(it, time.Time{}); err != nil {
		t.Fatalf("first call: %v", err)
	}
	sha = "sha2" // a new push
	obs, err := probe(it, time.Time{})
	if err != nil {
		t.Fatalf("second call: %v", err)
	}
	if obs.At.IsZero() {
		t.Fatal("a SHA change must be reported as a fresh observation")
	}
}

func TestBranchProbe_MissingBranchIsCleanlySilent(t *testing.T) {
	list := func(string) (map[string]string, error) { return map[string]string{}, nil }
	probe := NewBranchProbe(list, NewMemBranchSHAStore())
	it := Item{ID: "x/01", Payload: map[string]string{PayloadRepo: "o/r", PayloadBranch: "feat/x"}}
	obs, err := probe(it, time.Time{})
	if err != nil {
		t.Fatalf("a not-yet-existing branch must not be an error: %v", err)
	}
	if !obs.At.IsZero() {
		t.Fatalf("expected zero Observation, got %v", obs)
	}
}

func TestBranchProbe_ListerErrorIsCouldNotCheck(t *testing.T) {
	list := func(string) (map[string]string, error) { return nil, errors.New("network unreachable") }
	probe := NewBranchProbe(list, NewMemBranchSHAStore())
	it := Item{ID: "x/01", Payload: map[string]string{PayloadRepo: "o/r", PayloadBranch: "feat/x"}}
	if _, err := probe(it, time.Time{}); err == nil {
		t.Fatal("a lister failure must be could-not-check, never a clean result")
	}
}

// --- PRProbe ---

func TestPRProbe_UpdatedAfterSinceIsFresh(t *testing.T) {
	read := func(repo string, number int) (time.Time, bool, error) {
		return mustParse(t, "2026-09-02T11:00:00Z"), true, nil
	}
	probe := NewPRProbe(read)
	it := Item{ID: "x/01", Payload: map[string]string{PayloadRepo: "o/r", PayloadPR: "7"}}
	obs, err := probe(it, mustParse(t, "2026-09-02T10:00:00Z"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if obs.At.IsZero() {
		t.Fatal("a PR updated after `since` must be a fresh observation")
	}
}

func TestPRProbe_NotUpdatedSinceIsCleanlySilent(t *testing.T) {
	read := func(repo string, number int) (time.Time, bool, error) {
		return mustParse(t, "2026-09-02T09:00:00Z"), true, nil
	}
	probe := NewPRProbe(read)
	it := Item{ID: "x/01", Payload: map[string]string{PayloadRepo: "o/r", PayloadPR: "7"}}
	obs, err := probe(it, mustParse(t, "2026-09-02T10:00:00Z"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !obs.At.IsZero() {
		t.Fatalf("expected zero Observation, got %v", obs)
	}
}

func TestPRProbe_NotFoundIsCleanlySilent(t *testing.T) {
	read := func(repo string, number int) (time.Time, bool, error) { return time.Time{}, false, nil }
	probe := NewPRProbe(read)
	it := Item{ID: "x/01", Payload: map[string]string{PayloadRepo: "o/r", PayloadPR: "7"}}
	obs, err := probe(it, time.Time{})
	if err != nil {
		t.Fatalf("a not-found PR must not be an error: %v", err)
	}
	if !obs.At.IsZero() {
		t.Fatalf("expected zero Observation, got %v", obs)
	}
}

// TestPRProbe_404OnRecordedPRIsCouldNotCheck pins the brief's own wording: "404 on a PR that
// was recorded ⇒ error, never no-life" is a READER-level policy, not a housePRReader-level
// one — housePRReader itself treats not-found as `found=false` (see the doc on PRReader);
// callers that recorded a PR and then see 404 must decide from their OWN claim record
// whether a recorded PR going missing is itself suspicious. NewPRProbe, given only `it`,
// has no independent way to know "a PR number was recorded elsewhere" beyond Payload
// carrying one at all — which it already requires. This test pins the actual behavior
// (not-found is silent, per PRReader's contract) so a future reviewer sees the deliberate
// choice rather than an oversight.
func TestPRProbe_ReaderErrorIsCouldNotCheck(t *testing.T) {
	read := func(repo string, number int) (time.Time, bool, error) {
		return time.Time{}, false, errors.New("HTTP 500")
	}
	probe := NewPRProbe(read)
	it := Item{ID: "x/01", Payload: map[string]string{PayloadRepo: "o/r", PayloadPR: "7"}}
	if _, err := probe(it, time.Time{}); err == nil {
		t.Fatal("a reader failure must be could-not-check, never a clean result")
	}
}

// --- ObservableProbes.Latest negative controls (Task item 3) ---

// TestLatest_LifeBeatsCouldNotCheck is the negative control the brief names: "a
// could-not-check on one probe with a clean sign of life on another yields ALIVE" —
// Latest() is existing liveness.go behavior; this test exercises it through the house
// probe SHAPE (two real ObservableProbe closures) rather than through hand-rolled funcs,
// so a regression in either probe's error/observation plumbing would also trip it.
func TestLatest_LifeBeatsCouldNotCheck(t *testing.T) {
	auditErr := NewAuditProbe(func() ([]AuditEntry, error) { return nil, errors.New("unreadable") })
	branchAlive := NewBranchProbe(
		func(string) (map[string]string, error) { return map[string]string{"refs/heads/b": "sha1"}, nil },
		NewMemBranchSHAStore(),
	)
	probes := &ObservableProbes{AuditScan: auditErr, BranchMoved: branchAlive}
	it := Item{ID: "x/01", Payload: map[string]string{PayloadSessionTag: "sess", PayloadRepo: "o/r", PayloadBranch: "b"}}

	obs, err := probes.Latest(it, time.Time{})
	if err != nil {
		t.Fatalf("a positive observation from one source must outrank a could-not-check on another: %v", err)
	}
	if obs.At.IsZero() {
		t.Fatal("expected a positive observation")
	}
}

// TestLatest_AllErroredIsBlind is the second negative control: "all-probes-could-not-check
// yields BLIND" (an error from Latest, which the CLI renders as COULD-NOT-CHECK / action
// BLIND and releases nothing — see cmd/desksupervise).
func TestLatest_AllErroredIsBlind(t *testing.T) {
	auditErr := NewAuditProbe(func() ([]AuditEntry, error) { return nil, errors.New("unreadable") })
	branchErr := NewBranchProbe(func(string) (map[string]string, error) {
		return nil, errors.New("network down")
	}, NewMemBranchSHAStore())
	prErr := NewPRProbe(func(string, int) (time.Time, bool, error) { return time.Time{}, false, errors.New("403") })
	probes := &ObservableProbes{AuditScan: auditErr, BranchMoved: branchErr, PRActivity: prErr}
	it := Item{ID: "x/01", Payload: map[string]string{
		PayloadSessionTag: "sess", PayloadRepo: "o/r", PayloadBranch: "b", PayloadPR: "7",
	}}

	_, err := probes.Latest(it, time.Time{})
	if err == nil {
		t.Fatal("all probes could-not-check must yield an error (BLIND), never a clean/no-life result")
	}
}

// --- ClassifyLiveness (the observer classification table, Task item 3) ---

func TestClassifyLiveness_Table(t *testing.T) {
	pol := DefaultLivenessPolicy()
	now := mustParse(t, "2026-09-02T12:00:00Z")

	cases := []struct {
		name  string
		clock ClaimClock
		tier  Tier
		obs   Observation
		want  Disposition
	}{
		{
			name:  "never observed, past schedule-to-start",
			clock: ClaimClock{DispatchedAt: mustParse(t, "2026-09-02T11:00:00Z")}, // 60m ago > 10m
			tier:  TierCheap,
			obs:   Observation{},
			want:  ReclaimNeverStarted,
		},
		{
			name:  "started long ago, silent past heartbeat gap",
			clock: ClaimClock{DispatchedAt: mustParse(t, "2026-09-02T10:00:00Z")},
			tier:  TierCheap,
			obs:   Observation{At: mustParse(t, "2026-09-02T11:00:00Z")}, // 60m stale > 20m gap
			want:  ReclaimHeartbeat,
		},
		{
			name:  "recently observed, within every timer",
			clock: ClaimClock{DispatchedAt: mustParse(t, "2026-09-02T11:55:00Z")},
			tier:  TierCheap,
			obs:   Observation{At: mustParse(t, "2026-09-02T11:58:00Z")}, // 2m ago
			want:  Alive,
		},
		{
			name:  "over its wall cap despite a recent observation",
			clock: ClaimClock{DispatchedAt: mustParse(t, "2026-09-02T10:00:00Z")},
			tier:  TierCheap, // 90m cap
			obs:   Observation{At: mustParse(t, "2026-09-02T10:05:00Z")}, // proves start at 10:05 — 115m ago
			want:  BlockedStartToClose,
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := ClassifyLiveness(pol, tc.clock, tc.tier, now, tc.obs)
			if got != tc.want {
				t.Fatalf("ClassifyLiveness() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestDisposition_String(t *testing.T) {
	cases := map[Disposition]string{
		Alive:               "ALIVE",
		ReclaimNeverStarted: "NEVER-STARTED",
		ReclaimHeartbeat:    "HEARTBEAT-EXPIRED",
		BlockedStartToClose: "OVER-WALL-CAP",
	}
	for d, want := range cases {
		if got := d.String(); got != want {
			t.Fatalf("Disposition(%d).String() = %q, want %q", d, got, want)
		}
	}
}

// TestJournalObserverDecision_WritesParseableRecord proves an observer-written journal
// line round-trips through the SAME parser (parseJournalRecord) the engine's own
// journalEvent lines go through — an observer decision must not be a second, incompatible
// schema.
func TestJournalObserverDecision_WritesParseableRecord(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	JournalObserverDecision("desksupervise", EventReclaim, "x/01", "cheap", "", "heartbeat-expired", "obs-run-1")

	entries, err := deskkit.LoadEntries()
	if err != nil {
		t.Fatalf("LoadEntries: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("expected at least one audit entry")
	}
	last := entries[len(entries)-1]
	rec, ok := parseJournalRecord(last)
	if !ok {
		t.Fatalf("journal line did not parse as a journalRecord: %+v", last)
	}
	if rec.Item != "x/01" || rec.kind != journalReclaim {
		t.Fatalf("unexpected record: %+v", rec)
	}
}
