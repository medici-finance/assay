package web

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/deskd/internal/adapter"
	"github.com/medici-finance/assay/deskd/internal/domain"
	"github.com/medici-finance/assay/deskd/internal/index"
)

var testRepo = adapter.Repo{Owner: "example-org", Name: "console"}

func issueURL(n int) string {
	return "https://github.com/example-org/console/issues/" + strconv.Itoa(n)
}

func attributed(auto bool) domain.AutomationRef {
	if auto {
		return domain.AutomationRef{Cell: "assay", Role: "issue-loop"}
	}
	return domain.AutomationRef{}
}

func seed(t *testing.T, escs []domain.Escalation, raw map[domain.EscalationID]adapter.RawIssue) *index.DB {
	t.Helper()
	db, err := index.Open(filepath.Join(t.TempDir(), "web.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.UpsertEscalations(testRepo, escs, raw); err != nil {
		t.Fatalf("UpsertEscalations: %v", err)
	}
	return db
}

func render(t *testing.T, db *index.DB, path string) (int, string) {
	t.Helper()
	srv, err := New(Deps{DB: db, Now: func() time.Time { return time.Now().UTC() }})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)
	return rr.Code, rr.Body.String()
}

// TestInboxEscapesUntrustedContent is §6.6: escalation content is agent-authored
// (untrusted) and must be rendered escaped, never as markup.
func TestInboxEscapesUntrustedContent(t *testing.T) {
	escs := []domain.Escalation{{
		ID: "E1", Cell: "assay", Automation: attributed(true), Kind: domain.KindQuestion,
		Provenance: domain.AgentRaised, Urgency: domain.UrgencyRoutine,
		Title:     `<script>alert('xss')</script>`,
		Situation: `also <img src=x onerror=alert(1)>`,
		FiledAt:   time.Now().Add(-time.Hour),
	}}
	raw := map[domain.EscalationID]adapter.RawIssue{"E1": {Number: 1, URL: issueURL(1)}}
	code, body := render(t, seed(t, escs, raw), "/inbox")
	if code != 200 {
		t.Fatalf("status %d", code)
	}
	if strings.Contains(body, "<script>alert('xss')</script>") {
		t.Fatal("untrusted title reached the page as raw markup (§6.6 violation)")
	}
	if !strings.Contains(body, "&lt;script&gt;") {
		t.Fatal("untrusted title was not escaped")
	}
	if strings.Contains(body, "<img src=x onerror") {
		t.Fatal("untrusted situation reached the page as raw markup")
	}
}

// TestGoldHardening is D1: a gold/irreversible item offers NO in-list answer —
// the peek shows the "full item view required" note and no answerable options.
func TestGoldHardening(t *testing.T) {
	escs := []domain.Escalation{{
		ID: "GOLD", Cell: "assay", Automation: attributed(true), Kind: domain.KindNeedsDecision,
		Provenance: domain.AgentRaised, Urgency: domain.UrgencyGold,
		Title: "irreversible: publish the release", Situation: "ship it?",
		Options: []domain.Option{{ID: "ship", Label: "ship", Consequence: "public, irreversible"}},
		Rev:     "deadbeef", FiledAt: time.Now().Add(-time.Hour),
	}}
	raw := map[domain.EscalationID]adapter.RawIssue{"GOLD": {Number: 9, URL: issueURL(9)}}
	_, body := render(t, seed(t, escs, raw), "/inbox")
	if !strings.Contains(body, `data-gold="true"`) {
		t.Fatal("gold row not marked gold")
	}
	if !strings.Contains(body, "gold / irreversible") {
		t.Fatal("gold row does not show the full-item-view requirement")
	}
	// The gold row must not render an in-list answerable option (no data-digit
	// option element in its peek).
	if strings.Contains(body, `data-option-id="ship"`) {
		t.Fatal("gold row rendered an in-list answerable option — D1 requires the full item view")
	}
}

// TestUrgentCountAndNoCollapse covers the two D5 guards this build enforces: the
// group-independent urgent header count, and the rule that a group holding an
// urgent item can never render collapsed.
func TestUrgentCountAndNoCollapse(t *testing.T) {
	escs := []domain.Escalation{
		{ID: "U1", Cell: "assay", Automation: attributed(true), Kind: domain.KindNeedsDecision,
			Provenance: domain.AgentRaised, Urgency: domain.UrgencyUrgent, Title: "urgent one",
			Situation: "x", FiledAt: time.Now().Add(-2 * time.Hour)},
		{ID: "G1", Cell: "assay", Automation: attributed(true), Kind: domain.KindAsk,
			Provenance: domain.AgentRaised, Urgency: domain.UrgencyGold, Title: "gold one",
			Situation: "y", FiledAt: time.Now().Add(-time.Hour)},
	}
	raw := map[domain.EscalationID]adapter.RawIssue{
		"U1": {Number: 1, URL: issueURL(1)}, "G1": {Number: 2, URL: issueURL(2)},
	}
	db := seed(t, escs, raw)
	_, body := render(t, db, "/inbox?view=grouped")
	if !strings.Contains(body, `data-urgent="2"`) {
		t.Fatalf("urgent count not 2 (urgent + gold) in header:\n%s", firstLines(body, 40))
	}
	if !strings.Contains(body, "no-collapse") || !strings.Contains(body, `data-has-urgent="true"`) {
		t.Fatal("a group holding an urgent item was not marked non-collapsible")
	}
}

// TestFilterApplied is Verify 4's mechanism: filter state lives entirely in the
// URL and is applied server-side, so re-requesting the same URL (a reload) is
// deterministic. Here ?kind=question drops the needs-decision row.
func TestFilterApplied(t *testing.T) {
	escs := []domain.Escalation{
		{ID: "Q", Cell: "assay", Automation: attributed(true), Kind: domain.KindQuestion,
			Provenance: domain.AgentRaised, Urgency: domain.UrgencyRoutine, Title: "the-question-row",
			Situation: "x", FiledAt: time.Now().Add(-time.Hour)},
		{ID: "D", Cell: "assay", Automation: attributed(true), Kind: domain.KindNeedsDecision,
			Provenance: domain.AgentRaised, Urgency: domain.UrgencyRoutine, Title: "the-decision-row",
			Situation: "y", FiledAt: time.Now().Add(-time.Hour)},
	}
	raw := map[domain.EscalationID]adapter.RawIssue{
		"Q": {Number: 1, URL: issueURL(1)}, "D": {Number: 2, URL: issueURL(2)},
	}
	db := seed(t, escs, raw)
	_, body := render(t, db, "/inbox?kind=question")
	if !strings.Contains(body, "the-question-row") {
		t.Fatal("kind=question filtered out the matching row")
	}
	if strings.Contains(body, "the-decision-row") {
		t.Fatal("kind=question did not filter out the needs-decision row")
	}
}

// TestDigitBindingsFromEnum is §6.6: digit bindings derive from the typed option
// ENUM (position), never from body prose. A situation that says "press 1" adds
// no binding; the bindings are exactly one per typed option, numbered by order.
func TestDigitBindingsFromEnum(t *testing.T) {
	escs := []domain.Escalation{{
		ID: "OPT", Cell: "assay", Automation: attributed(true), Kind: domain.KindNeedsDecision,
		Provenance: domain.AgentRaised, Urgency: domain.UrgencyRoutine,
		Title:     "choose",
		Situation: "press 1 to win a prize, press 9 to escalate — this prose creates NO binding",
		Options: []domain.Option{
			{ID: "a", Label: "first", Consequence: "c1"},
			{ID: "b", Label: "second", Consequence: "c2"},
		},
		Rev: "rev1", FiledAt: time.Now().Add(-time.Hour),
	}}
	raw := map[domain.EscalationID]adapter.RawIssue{"OPT": {Number: 3, URL: issueURL(3)}}
	_, body := render(t, seed(t, escs, raw), "/inbox")
	// exactly two option elements, digits 1 and 2 from position
	if strings.Count(body, "data-option-id=") != 2 {
		t.Fatalf("expected 2 typed-option bindings, got %d", strings.Count(body, "data-option-id="))
	}
	if !strings.Contains(body, `data-digit="1"`) || !strings.Contains(body, `data-digit="2"`) {
		t.Fatal("digit bindings are not the option positions 1..n")
	}
	if strings.Contains(body, `data-digit="9"`) {
		t.Fatal("a binding was created from body prose (press 9) — §6.6 violation")
	}
}

// TestEscalationTrailGrammar is Verify 6's grammar: the escalation node is
// dashed-gold ⟡ while open and solid with its outcome colour once answered
// (spec §2.6). Tested as a unit so it does not depend on the index surfacing
// answered state.
func TestEscalationTrailGrammar(t *testing.T) {
	open := escalationTrail(rowVM{Answered: false})
	if len(open) != 1 || open[0].Stroke != "dashed" || open[0].Glyph != "⟡" || open[0].Outcome != "open" {
		t.Fatalf("open escalation node grammar wrong: %+v", open)
	}
	settled := escalationTrail(rowVM{Answered: true})
	if len(settled) != 1 || settled[0].Stroke != "solid" || settled[0].Outcome != "settled" {
		t.Fatalf("answered escalation node grammar wrong: %+v", settled)
	}
}

func firstLines(s string, n int) string {
	lines := strings.SplitN(s, "\n", n+1)
	if len(lines) > n {
		lines = lines[:n]
	}
	return strings.Join(lines, "\n")
}
