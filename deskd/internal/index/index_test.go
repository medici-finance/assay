package index_test

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/deskd/config/schema"
	"github.com/medici-finance/assay/deskd/internal/adapter"
	"github.com/medici-finance/assay/deskd/internal/domain"
	"github.com/medici-finance/assay/deskd/internal/index"
)

// ---- test doubles ---------------------------------------------------------

// liveSource returns two escalations for any repo.
type liveSource struct{ calls int }

func (s *liveSource) FetchEscalations(_ context.Context, repo adapter.Repo) adapter.FetchResult {
	s.calls++
	p := domain.Probe{Source: "test double", Method: domain.MethodGraphQLAliased, Window: "fixture", Limit: 0}
	issues := []adapter.RawIssue{
		{Number: 41, Title: "URGENT: a thing overclaims", Body: "Situation: it overclaims.",
			Labels: []string{"needs-decision"}, Author: "a-review-app", CreatedAt: time.Now().Add(-48 * time.Hour),
			URL: "https://example.invalid/41"},
		{Number: 42, Title: "question: parked on a missing binary",
			Body:   "Situation: parked.\n\n### Options\n<!-- assay:options v2 -->\n#### `retry` — Retry once\nConsequence: one more attempt.\n",
			Labels: []string{"question"}, Author: "a-verify-app", CreatedAt: time.Now().Add(-3 * time.Hour),
			URL: "https://example.invalid/42"},
	}
	return adapter.FetchResult{
		Repo: repo, Issues: issues,
		Health:     adapter.RepoHealth{Repo: repo.String(), CheckedAt: time.Now().UTC(), Assessment: domain.Clean(p)},
		Candidates: domain.NewCount(len(issues), p),
		Derived:    domain.NewCount(len(issues), p),
	}
}

// deadSource is the POSITIVE CONTROL: a source that cannot be reached.
//
// It returns exactly what a real unreachable source returns — no issues —
// and the honest verdict beside it. If the index rendered the empty slice
// and dropped the verdict, the board would read "nothing to do" while blind.
type deadSource struct{ reason string }

func (s *deadSource) FetchEscalations(_ context.Context, repo adapter.Repo) adapter.FetchResult {
	p := domain.Probe{
		Source: "GET {api}/repos/…/issues (conditional)", Method: domain.MethodRESTList,
		Window: "open escalation-labelled issues", Limit: 4000,
	}
	return adapter.FetchResult{
		Repo:       repo,
		Health:     adapter.RepoHealth{Repo: repo.String(), CheckedAt: time.Now().UTC(), Assessment: domain.Unchecked(p, s.reason)},
		Candidates: domain.NoCount(p, s.reason),
		Derived:    domain.NoCount(p, s.reason),
	}
}

func testConfig() *schema.File {
	return &schema.File{Cells: []schema.Cell{{
		Name:        "assay",
		Repos:       []string{"owner/alpha"},
		StreamsRoot: "docs/streams",
		Loops: []schema.Loop{
			{Role: "review", Schedule: "*/10 * * * *", Models: []string{"m"}, App: "a-review-app"},
			{Role: "verify", Schedule: "*/15 * * * *", Models: []string{"m"}, App: "a-verify-app"},
		},
		SilenceThreshold: "48h",
	}}}
}

func openTemp(t *testing.T) (*index.DB, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "deskd-index.db")
	db, err := index.Open(path)
	if err != nil {
		t.Fatalf("could-not-check: index did not open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db, path
}

func newPoller(db *index.DB, f index.Fetcher) *index.Poller {
	cfg := testConfig()
	return &index.Poller{DB: db, Fetcher: f, Config: cfg, Dir: index.NewDirectory(cfg)}
}

// ---- the positive control -------------------------------------------------

// TestUnreachableSourceRendersCouldNotCheckNotAnEmptyIndex is the red run
// this brief's honesty rests on.
//
// The failure it exists to catch is not hypothetical and not rare: ~16+
// concurrent GitHub operations on one shared token trip the secondary rate
// limit and return 403. To a caller that only checks `len(items)`, that is
// indistinguishable from a quiet queue. An empty result from an unreachable
// source is BLIND, not idle.
//
// So: point the poller at a source that cannot be reached, and assert that
// EVERY surface says could-not-check —
//
//	the repo's health verdict          could-not-check, with the reason
//	the cycle outcome                  not "ok"
//	the repo counts                    no value at all, not 0
//
// and that nothing anywhere reports a confident zero.
func TestUnreachableSourceRendersCouldNotCheckNotAnEmptyIndex(t *testing.T) {
	db, _ := openTemp(t)
	const reason = "HTTP 403: SECONDARY rate limit — concurrent readers on one token trip this"
	res := newPoller(db, &deadSource{reason: reason}).PollOnce(context.Background())

	if res.Outcome == "ok" {
		t.Fatal("POSITIVE CONTROL SURVIVED: a cycle in which no repo could be reached reported outcome=ok")
	}
	if _, ok := res.Reached.Value(); !ok {
		t.Error("the reached-repo count refused to publish; it is derivable and should be 0 clean")
	}
	if n, _ := res.Reached.Value(); n != 0 {
		t.Errorf("reached = %d against an unreachable source", n)
	}

	health, err := db.Health()
	if err != nil {
		t.Fatalf("could-not-check: %v", err)
	}
	if len(health) == 0 {
		t.Fatal("POSITIVE CONTROL SURVIVED: an unreachable repo left NO health row — the index would render a blank, confident board")
	}
	for _, h := range health {
		if h.Assessment.Confidence != domain.CouldNotCheck {
			t.Errorf("repo %s rendered %s against an unreachable source, want could-not-check", h.Repo, h.Assessment.Confidence)
		}
		if !strings.Contains(h.Assessment.Reason, "403") {
			t.Errorf("repo %s: the reason does not carry the failure a human would act on: %q", h.Repo, h.Assessment.Reason)
		}
	}

	// The inbox is empty — correctly, there is nothing indexed — but the
	// index age must NOT read as fresh-and-fine, and the health block must
	// carry the blindness.
	rows, err := db.Inbox("assay", false)
	if err != nil {
		t.Fatalf("could-not-check: %v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("rows appeared from an unreachable source: %d", len(rows))
	}
	_, ageAssessment := db.IndexAge(time.Now())
	if ageAssessment.Confidence == domain.CheckedClean {
		t.Error("the index reported a checked-clean age after a cycle in which every repo was blind")
	}
}

// TestUnreachableRepoDoesNotErasePreviousRows — the second half of the same
// rule. Once a repo HAS been indexed, a later blind cycle must leave those
// rows alone. Clearing them would convert "we cannot see" into "there is
// nothing", which is the identical lie one step later.
func TestUnreachableRepoDoesNotErasePreviousRows(t *testing.T) {
	db, _ := openTemp(t)

	if res := newPoller(db, &liveSource{}).PollOnce(context.Background()); res.Outcome != "ok" {
		t.Fatalf("warm-up cycle was not clean: %s", res.Outcome)
	}
	before, _ := db.Inbox("assay", false)
	if len(before) != 2 {
		t.Fatalf("warm-up indexed %d rows, want 2", len(before))
	}

	res := newPoller(db, &deadSource{reason: "HTTP 403: SECONDARY rate limit"}).PollOnce(context.Background())
	if res.Outcome == "ok" {
		t.Fatal("a blind cycle reported ok")
	}
	after, _ := db.Inbox("assay", false)
	if len(after) != len(before) {
		t.Fatalf("a blind cycle erased indexed rows: %d → %d. An unreachable repo degrades its own slice, "+
			"it does not empty the inbox (design §10, partial repo failure).", len(before), len(after))
	}
	health, _ := db.Health()
	for _, h := range health {
		if h.Assessment.Confidence != domain.CouldNotCheck {
			t.Errorf("stale rows are being served with a %s verdict — the operator has no way to know they are stale", h.Assessment.Confidence)
		}
	}
}

// ---- the pure-cache proof -------------------------

// TestDerivedIndexIsAPureCache is Verify row 7, automated.
//
// Drop the derived cache, re-poll, and assert: every GitHub-derived row is
// back, and the local read/ack store is untouched. That is what makes the
// index a cache rather than a database, and it is the property ruling D7
// depends on — the read/ack timestamps are the FIRST deskd state not
// rebuildable from GitHub, so they have to survive a rebuild of everything
// that is.
func TestDerivedIndexIsAPureCache(t *testing.T) {
	db, _ := openTemp(t)
	src := &liveSource{}
	p := newPoller(db, src)
	p.PollOnce(context.Background())

	before, err := db.Inbox("assay", false)
	if err != nil {
		t.Fatalf("could-not-check: %v", err)
	}
	if len(before) == 0 {
		t.Fatal("nothing indexed; the rebuild proof would be vacuous")
	}

	// Seed the LOCAL store, standing in for the human attention
	// the service contract's setters will write. The column exists in this
	// build; only the setter is deferred.
	if err := seedLocalAttention(db, string(before[0].ID)); err != nil {
		t.Fatalf("could-not-check: %v", err)
	}

	if err := db.DropDerived(); err != nil {
		t.Fatalf("DropDerived: %v", err)
	}
	if mid, _ := db.Inbox("assay", false); len(mid) != 0 {
		t.Fatalf("the derived cache was not actually dropped: %d rows remain", len(mid))
	}
	if n := countLocalAttention(t, db); n != 1 {
		t.Fatalf("DropDerived removed %d local attention row(s) — it must not be able to touch local state (ruling D7)", 1-n)
	}

	p.PollOnce(context.Background())
	after, err := db.Inbox("assay", false)
	if err != nil {
		t.Fatalf("could-not-check: %v", err)
	}
	if len(after) != len(before) {
		t.Fatalf("rebuild produced %d rows, want %d — the index is not a pure cache", len(after), len(before))
	}
	if n := countLocalAttention(t, db); n != 1 {
		t.Fatalf("the local attention row did not survive the rebuild (found %d)", n)
	}
	// Ids are stable across a warm rebuild because the platform-ref map is
	// local. A COLD rebuild may re-mint, which the spec accepts.
	for i := range after {
		if after[i].ID != before[i].ID {
			t.Errorf("row %d changed id across a rebuild: %s → %s", i, before[i].ID, after[i].ID)
		}
	}
}

// TestSchemaCarriesTheDeferredColumns is the read surface Verify row 6 as a Go test:
// read_at, acked_at, updated_at and collapse_key are present as COLUMNS in
// this build, with no setter and no folding (both the service contract).
func TestSchemaCarriesTheDeferredColumns(t *testing.T) {
	db, path := openTemp(t)
	_ = path
	for _, want := range []struct{ table, column string }{
		{"local_attention", "read_at"},
		{"local_attention", "acked_at"},
		{"derived_escalations", "updated_at"},
		{"derived_escalations", "collapse_key"},
		{"derived_escalations", "repeat"},
	} {
		if !hasColumn(t, db, want.table, want.column) {
			t.Errorf("%s.%s is missing — fields are near-free at greenfield and cost a migration later, "+
				"which is why the read surface lands them and the service contract lands their behaviour", want.table, want.column)
		}
	}
}

// TestColdIndexAgeIsNotZero — a cold index has an UNKNOWN age, not an age of
// zero. Zero reads as "just refreshed", which is the most misleading thing a
// board that has never polled can say.
func TestColdIndexAgeIsNotZero(t *testing.T) {
	db, _ := openTemp(t)
	age, a := db.IndexAge(time.Now())
	if a.Confidence != domain.CouldNotCheck {
		t.Fatalf("a cold index reported %s age (%s); it has never polled", a.Confidence, age)
	}
	if a.Reason == "" {
		t.Error("the could-not-check carries no reason")
	}
}

// TestRestartIsWarm — the index survives a close/reopen without re-polling
// (design §5 step 3).
func TestRestartIsWarm(t *testing.T) {
	path := filepath.Join(t.TempDir(), "warm.db")
	db, err := index.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	newPoller(db, &liveSource{}).PollOnce(context.Background())
	before, _ := db.Inbox("assay", false)
	db.Close()

	reopened, err := index.Open(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer reopened.Close()
	after, err := reopened.Inbox("assay", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(after) != len(before) {
		t.Fatalf("restart was cold: %d rows before, %d after", len(before), len(after))
	}
	if _, a := reopened.IndexAge(time.Now()); a.Confidence == domain.CouldNotCheck {
		t.Error("a warm restart lost the poll marker, so it cannot say how stale it is")
	}
}

// TestInboxIsUrgencyThenAge pins the sort (design §5.3).
func TestInboxIsUrgencyThenAge(t *testing.T) {
	db, _ := openTemp(t)
	newPoller(db, &liveSource{}).PollOnce(context.Background())
	rows, err := db.Inbox("assay", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) < 2 {
		t.Fatalf("need at least 2 rows, got %d", len(rows))
	}
	if rows[0].Urgency < rows[1].Urgency {
		t.Errorf("sort is not urgency-first: %s before %s", rows[0].Urgency, rows[1].Urgency)
	}
	if rows[0].Urgency != domain.UrgencyUrgent {
		t.Errorf("the URGENT-titled row is not first (got %s) — this is the exact row that sat invisible", rows[0].Urgency)
	}
}

// TestTypedOptionsSurviveTheRoundTrip — a v2 body parsed by the adapter,
// stored, and read back must still carry its typed list and rev.
func TestTypedOptionsSurviveTheRoundTrip(t *testing.T) {
	db, _ := openTemp(t)
	newPoller(db, &liveSource{}).PollOnce(context.Background())
	rows, _ := db.Inbox("assay", false)

	var typed, prose int
	for _, r := range rows {
		if r.Options == nil {
			prose++
			continue
		}
		typed++
		if r.Rev == "" {
			t.Errorf("row %s has options but no rev", r.ID)
		}
		if r.Rev != domain.OptionsRev(r.Options) {
			t.Errorf("row %s: the stored rev does not match a re-derivation from the stored options", r.ID)
		}
		for _, o := range r.Options {
			if o.ID == "" || o.Label == "" || o.Consequence == "" {
				t.Errorf("row %s: option %+v lost a field in storage", r.ID, o)
			}
		}
	}
	if typed == 0 || prose == 0 {
		t.Fatalf("the fixture set did not exercise both encodings (typed=%d prose=%d)", typed, prose)
	}
}

// ---- small SQL helpers ----------------------------------------------------

func seedLocalAttention(db *index.DB, escID string) error {
	return db.ExecForTest(
		`INSERT INTO local_attention (escalation_id, actor, read_at, acked_at) VALUES (?,?,?,?)`,
		escID, "operator", time.Now().UTC().Format(time.RFC3339Nano), nil)
}

func countLocalAttention(t *testing.T, db *index.DB) int {
	t.Helper()
	n, err := db.QueryIntForTest(`SELECT COUNT(*) FROM local_attention`)
	if err != nil {
		t.Fatalf("could-not-check: %v", err)
	}
	return n
}

func hasColumn(t *testing.T, db *index.DB, table, column string) bool {
	t.Helper()
	n, err := db.QueryIntForTest(fmt.Sprintf(
		`SELECT COUNT(*) FROM pragma_table_info(%q) WHERE name = %q`, table, column))
	if err != nil {
		if errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
		t.Fatalf("could-not-check: %v", err)
	}
	return n > 0
}
