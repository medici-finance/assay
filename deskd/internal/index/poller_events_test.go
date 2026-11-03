package index_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/medici-finance/assay/deskd/internal/adapter"
	"github.com/medici-finance/assay/deskd/internal/contract"
	"github.com/medici-finance/assay/deskd/internal/domain"
	"github.com/medici-finance/assay/deskd/internal/index"
)

// recordedEvent is one call [recordingPublisher] observed.
type recordedEvent struct {
	Type contract.EventType
	Data any
}

// recordingPublisher is a scripted [index.Publisher] for asserting exactly
// what PollOnce published, and how many times.
type recordingPublisher struct {
	mu     sync.Mutex
	events []recordedEvent
}

func (r *recordingPublisher) Publish(typ contract.EventType, data any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, recordedEvent{typ, data})
}

func (r *recordingPublisher) snapshot() []recordedEvent {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]recordedEvent(nil), r.events...)
}

func (r *recordingPublisher) countType(typ contract.EventType) int {
	n := 0
	for _, e := range r.snapshot() {
		if e.Type == typ {
			n++
		}
	}
	return n
}

// scriptedFetcher serves one escalation whose Closed/Comments the test flips
// between PollOnce calls, so the poller sees a real STATE CHANGE across
// cycles — the same shape a real answer-then-close would produce on poll.
type scriptedFetcher struct {
	mu     sync.Mutex
	closed bool
	iss    adapter.RawIssue
}

func (f *scriptedFetcher) set(closed bool, comments []adapter.RawComment) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closed = closed
	f.iss.Comments = comments
}

func (f *scriptedFetcher) FetchEscalations(_ context.Context, repo adapter.Repo) adapter.FetchResult {
	f.mu.Lock()
	iss := f.iss
	iss.Closed = f.closed
	f.mu.Unlock()

	p := domain.Probe{Source: "test double", Method: domain.MethodGraphQLAliased, Window: "fixture", Limit: 0}
	return adapter.FetchResult{
		Repo:       repo,
		Issues:     []adapter.RawIssue{iss},
		Health:     adapter.RepoHealth{Repo: repo.String(), CheckedAt: time.Now().UTC(), Assessment: domain.Clean(p)},
		Candidates: domain.NewCount(1, p),
		Derived:    domain.NewCount(1, p),
	}
}

// TestPollOnceEmitsTransitionOncePerChangeNeverPerPoll is Verify row 9's
// dedup claim, exercised end to end through the real Poller rather than the
// EventBus alone: an unchanged re-poll emits NOTHING, and a real state change
// (here: the escalation closing, the one transition this build's own
// attention derivation can always reach — see attentionFor's doc comment on
// why the richer working/waiting states are out of reach without the
// artifact/pod substrate) emits EXACTLY one transition event, not one per
// poll tick.
func TestPollOnceEmitsTransitionOncePerChangeNeverPerPoll(t *testing.T) {
	db, _ := openTemp(t)
	cfg := testConfig()
	fetcher := &scriptedFetcher{iss: adapter.RawIssue{
		Number: 41, Title: "URGENT: a thing overclaims", Body: "Situation: it overclaims.",
		Labels: []string{"needs-decision"}, Author: "a-review-app",
		CreatedAt: time.Now().Add(-48 * time.Hour), URL: "https://example.invalid/41",
	}}
	pub := &recordingPublisher{}
	p := &index.Poller{DB: db, Fetcher: fetcher, Config: cfg, Dir: index.NewDirectory(cfg), Events: pub}

	// Cycle 1: the escalation is freshly discovered — one transition (open
	// appearance) and one badge update (0 -> 1 open).
	if res := p.PollOnce(context.Background()); res.Outcome != "ok" {
		t.Fatalf("cycle 1 not clean: %s", res.Outcome)
	}
	if n := pub.countType(contract.EventTransition); n != 1 {
		t.Fatalf("cycle 1: %d transition events, want 1 (fresh appearance)", n)
	}
	if n := pub.countType(contract.EventBadge); n != 1 {
		t.Fatalf("cycle 1: %d badge events, want 1", n)
	}

	// Cycle 2: NOTHING changed. This is the dedup claim: re-polling an
	// unchanged escalation must publish nothing more.
	if res := p.PollOnce(context.Background()); res.Outcome != "ok" {
		t.Fatalf("cycle 2 not clean: %s", res.Outcome)
	}
	if n := pub.countType(contract.EventTransition); n != 1 {
		t.Fatalf("cycle 2 (unchanged): %d transition events total, want still 1 — an unchanged re-poll must publish nothing", n)
	}
	if n := pub.countType(contract.EventBadge); n != 1 {
		t.Fatalf("cycle 2 (unchanged): %d badge events total, want still 1", n)
	}

	// Cycle 3: the escalation closes with an answer marker in its now-polled
	// comments — a real state change, which must emit exactly one more
	// transition (never more than one for this one change) and one more
	// badge update (1 -> 0 open).
	fetcher.set(true, []adapter.RawComment{
		{Body: "<!-- assay:answer v1 login=operator option=opt-a options_rev=rev1 -->", Login: "operator", CreatedAt: time.Now().UTC()},
	})
	if res := p.PollOnce(context.Background()); res.Outcome != "ok" {
		t.Fatalf("cycle 3 not clean: %s", res.Outcome)
	}
	if n := pub.countType(contract.EventTransition); n != 2 {
		t.Fatalf("cycle 3 (closed): %d transition events total, want 2 (one more, for the one real change)", n)
	}
	if n := pub.countType(contract.EventBadge); n != 2 {
		t.Fatalf("cycle 3 (closed): %d badge events total, want 2", n)
	}

	// The closing transition's payload names the real escalation id and lands
	// on "closed" — proving the producer is wired to the SAME derivation the
	// stored row carries, not a parallel guess.
	var sawClose bool
	for _, e := range pub.snapshot() {
		if e.Type != contract.EventTransition {
			continue
		}
		td, ok := e.Data.(contract.TransitionData)
		if !ok {
			t.Fatalf("transition event data is %T, want contract.TransitionData", e.Data)
		}
		if td.To == "closed" {
			sawClose = true
			if td.From != "waiting" {
				t.Errorf("closing transition From = %q, want waiting", td.From)
			}
		}
	}
	if !sawClose {
		t.Fatal("no transition event carried To=closed")
	}

	// Cycle 4: unchanged again (still closed, same comments) — no further
	// events, proving the dedup rule holds after a close too, not just on the
	// open-inbox path.
	if res := p.PollOnce(context.Background()); res.Outcome != "ok" {
		t.Fatalf("cycle 4 not clean: %s", res.Outcome)
	}
	if n := pub.countType(contract.EventTransition); n != 2 {
		t.Fatalf("cycle 4 (still closed, unchanged): %d transition events total, want still 2", n)
	}
}

// TestNilBeforeSnapshotPublishes pins the could-not-check BEFORE-read case
// PollOnce's doc comment promises: when this cycle's before-read fails (nil
// maps, zero badge) but the same cycle's after-read succeeds, every after
// row publishes as a fresh appearance, the badge publishes against the
// zero-value baseline, and the health set against the empty one. The
// sibling AFTER-read failure only delays a transition to the next cycle;
// this one would lose it for good, because the next cycle's before-read
// already sees the new state — so silence here is a dropped event, not a
// deferred one.
//
// The after-state is real: a no-Events PollOnce stores the row through the
// same UpsertEscalations path production uses, and the producer half then
// re-reads it through the same p.DB.Inbox call. Only the BEFORE snapshot is
// substituted, with exactly the values a failed read leaves behind.
func TestNilBeforeSnapshotPublishes(t *testing.T) {
	db, _ := openTemp(t)
	cfg := testConfig()
	// A second, malformed repo slug makes this cycle's health carry one
	// could-not-check entry, so the health event has something to publish
	// against the empty before-set.
	cfg.Cells[0].Repos = append(cfg.Cells[0].Repos, "not-owner-slash-name")
	fetcher := &scriptedFetcher{iss: adapter.RawIssue{
		Number: 7, Title: "URGENT: decide the thing", Body: "Situation: a decision is due.",
		Labels: []string{"needs-decision"}, Author: "a-review-app",
		CreatedAt: time.Now().Add(-time.Hour), URL: "https://example.invalid/7",
	}}
	pub := &recordingPublisher{}
	p := &index.Poller{DB: db, Fetcher: fetcher, Config: cfg, Dir: index.NewDirectory(cfg)}

	// Seed with no Events wired: the row is stored, nothing is published.
	if res := p.PollOnce(context.Background()); res.Outcome != "1 of 2 repo slice(s) could-not-check" {
		t.Fatalf("seed cycle outcome %q, want exactly one reached and one could-not-check slice", res.Outcome)
	}
	p.Events = pub

	p.PublishForTest(nil, contract.BadgeData{}, nil, time.Now().UTC())

	if n := pub.countType(contract.EventTransition); n != 1 {
		t.Fatalf("nil before: %d transition events, want 1 (the stored row as a fresh appearance)", n)
	}
	if n := pub.countType(contract.EventBadge); n != 1 {
		t.Fatalf("nil before: %d badge events, want 1 (0 -> 1 open against the zero-value baseline)", n)
	}
	if n := pub.countType(contract.EventIndexHealth); n != 1 {
		t.Fatalf("nil before: %d index-health events, want 1 (one could-not-check repo against the empty baseline)", n)
	}
	for _, e := range pub.snapshot() {
		switch d := e.Data.(type) {
		case contract.TransitionData:
			if d.From != "" || d.To == "" {
				t.Errorf("transition From=%q To=%q, want From empty (fresh appearance) and a derived To", d.From, d.To)
			}
		case contract.BadgeData:
			if d.Open != 1 {
				t.Errorf("badge Open=%d, want 1", d.Open)
			}
		case contract.IndexHealthData:
			if len(d.CouldNotCheck) != 1 || d.CouldNotCheck[0] != "not-owner-slash-name" {
				t.Errorf("index-health CouldNotCheck=%v, want [not-owner-slash-name]", d.CouldNotCheck)
			}
		default:
			t.Errorf("unexpected event payload %T", e.Data)
		}
	}
}

// TestPollOnceNilEventsIsNoop: a Poller with no Events wired behaves exactly
// as before increment 3 — PollOnce never dereferences a nil Publisher.
func TestPollOnceNilEventsIsNoop(t *testing.T) {
	db, _ := openTemp(t)
	cfg := testConfig()
	fetcher := &scriptedFetcher{iss: adapter.RawIssue{
		Number: 99, Title: "question: parked", Body: "Situation: parked.",
		Labels: []string{"question"}, Author: "a-verify-app",
		CreatedAt: time.Now().Add(-time.Hour), URL: "https://example.invalid/99",
	}}
	p := &index.Poller{DB: db, Fetcher: fetcher, Config: cfg, Dir: index.NewDirectory(cfg)}
	if res := p.PollOnce(context.Background()); res.Outcome != "ok" {
		t.Fatalf("cycle not clean: %s", res.Outcome)
	}
}
