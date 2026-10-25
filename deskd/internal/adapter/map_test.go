package adapter_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/deskd/internal/adapter"
	"github.com/medici-finance/assay/deskd/internal/domain"
)

// fakeDir is an automation directory with a deliberate hole in it.
type fakeDir struct{ m map[string]domain.Role }

func (f fakeDir) Lookup(cell domain.CellName, login string) (domain.AutomationRef, bool) {
	r, ok := f.m[login]
	if !ok {
		return domain.AutomationRef{}, false
	}
	return domain.AutomationRef{Cell: cell, Role: r}, true
}

func (f fakeDir) Known(cell domain.CellName) []string {
	var out []string
	for k := range f.m {
		out = append(out, k)
	}
	return out
}

func stubMint(_ adapter.Repo, n int) domain.EscalationID {
	return domain.EscalationID("01TESTULID" + string(rune('A'+n%26)))
}

var testDir = fakeDir{m: map[string]domain.Role{
	"a-review-app":  "review",
	"a-verify-app":  "verify",
	"a-metrics-app": "metrics",
}}

func repo() adapter.Repo { return adapter.Repo{Owner: "owner", Name: "name"} }

// TestUnjoinableRowBecomesUnresolved is the answer to "what does your index
// do with a row it cannot join?"
//
// It renders it. With automation null, a could-not-check on that field, a
// reason naming both the login it saw and the set it searched, and NO
// collapse key — so the service contract's folding can never merge it with
// anything. Three things it does NOT do: drop the row, guess an automation,
// or default the field to a zero value that reads like an answer.
func TestUnjoinableRowBecomesUnresolved(t *testing.T) {
	iss := adapter.RawIssue{
		Number:    41,
		Title:     "needs-decision: which of three shapes",
		Body:      "Situation: a fork with no typed options.",
		Labels:    []string{"needs-decision"},
		Author:    "some-unregistered-app[bot]",
		CreatedAt: time.Now().Add(-48 * time.Hour),
		URL:       "https://example.invalid/issues/41",
	}
	e := adapter.MapEscalation("assay", repo(), iss, testDir, stubMint, adapter.AnswerVerifier{})

	if e.Attributable() {
		t.Fatalf("an unregistered filing identity was attributed to %s", e.Automation)
	}
	a, ok := e.Fields["automation"]
	if !ok {
		t.Fatal("the automation field carries no assessment at all — silence is the failure mode this index exists to remove")
	}
	if a.Confidence != domain.CouldNotCheck {
		t.Fatalf("unjoinable automation rendered %s, want could-not-check", a.Confidence)
	}
	if !strings.Contains(a.Reason, "some-unregistered-app") {
		t.Errorf("the reason does not name the login it could not join: %q", a.Reason)
	}
	if !strings.Contains(a.Reason, "a-review-app") {
		t.Errorf("the reason does not name the set it searched, so nobody can tell whether the config or the filing is wrong: %q", a.Reason)
	}
	if key := e.Key().String(); key != "" {
		t.Errorf("an unattributable escalation was given the collapse key %q — spec §1.1.3 says such filings never merge, "+
			"and an empty key is how that is enforced rather than remembered", key)
	}

	// And it must SERIALISE as null, not as a zero-valued object.
	raw, err := json.Marshal(e)
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	if v, present := out["automation"]; !present || v != nil {
		t.Fatalf("automation serialised as %#v; an unjoinable field must be an explicit null, never a zero object", v)
	}
	unres, _ := out["unresolved_fields"].([]any)
	if len(unres) == 0 {
		t.Fatal("the row does not advertise which fields are unresolved")
	}
}

// TestJoinedRowIsCleanAndKeyed is the control for the above: when the join
// succeeds, the field is checked-clean and the row gets a collapse key.
func TestJoinedRowIsCleanAndKeyed(t *testing.T) {
	iss := adapter.RawIssue{
		Number: 42, Title: "question: parked", Body: "Situation: parked.",
		Labels: []string{"question"}, Author: "a-review-app[bot]",
		CreatedAt: time.Now().Add(-time.Hour),
	}
	e := adapter.MapEscalation("assay", repo(), iss, testDir, stubMint, adapter.AnswerVerifier{})
	if !e.Attributable() {
		t.Fatal("a registered filing identity was not attributed")
	}
	if e.Fields["automation"].Confidence != domain.CheckedClean {
		t.Fatalf("a successful join rendered %s", e.Fields["automation"].Confidence)
	}
	if got, want := e.Key().String(), "assay|assay/review|question"; got != want {
		t.Errorf("collapse key = %q, want %q", got, want)
	}
}

// TestProvenanceDerivesAgentRaisedFromAbsence pins ruling D4: this build
// lands the field and nothing that produces the other value. Every v1
// escalation derives agent-raised, from the ABSENCE of the wire label.
func TestProvenanceDerivesAgentRaisedFromAbsence(t *testing.T) {
	base := adapter.RawIssue{Number: 1, Labels: []string{"needs-decision"}, Author: "a-review-app"}
	if got := adapter.MapEscalation("assay", repo(), base, testDir, stubMint, adapter.AnswerVerifier{}).Provenance; got != domain.AgentRaised {
		t.Fatalf("provenance = %q, want agent-raised (absence of the wire label)", got)
	}
	// The field can carry the other value — that is the point of landing it
	// now — even though nothing in this build produces it.
	labelled := base
	labelled.Labels = append(labelled.Labels, "policy-raised")
	if got := adapter.MapEscalation("assay", repo(), labelled, testDir, stubMint, adapter.AnswerVerifier{}).Provenance; got != domain.PolicyRaised {
		t.Fatalf("provenance = %q with the wire label present, want policy-raised", got)
	}
}

// TestKindPrecedenceOnMultiLabelledIssues reflects the real corpus: 13 label
// instances across 10 unique issues, so three of them carry two escalation
// labels. The most human-actionable label must win, or a needs-decision gets
// demoted to a question and drops down the queue.
func TestKindPrecedenceOnMultiLabelledIssues(t *testing.T) {
	iss := adapter.RawIssue{Number: 2, Labels: []string{"question", "needs-decision"}, Author: "a-review-app"}
	if got := adapter.MapEscalation("assay", repo(), iss, testDir, stubMint, adapter.AnswerVerifier{}).Kind; got != domain.KindNeedsDecision {
		t.Fatalf("kind = %q on a needs-decision+question issue, want needs-decision", got)
	}
	// `help wanted` carries a space on the wire and a hyphen in the domain.
	spaced := adapter.RawIssue{Number: 3, Labels: []string{"help wanted"}, Author: "a-review-app"}
	if got := adapter.MapEscalation("assay", repo(), spaced, testDir, stubMint, adapter.AnswerVerifier{}).Kind; got != domain.KindHelpWanted {
		t.Fatalf("kind = %q for the space-spelled wire label, want help-wanted", got)
	}
}

// TestUrgencyFromTheCorpusShapes checks the two title markers that actually
// appear in the ten real escalations, and the gold gate that must never be
// answerable in-list (ruling D1).
func TestUrgencyFromTheCorpusShapes(t *testing.T) {
	cases := []struct {
		title  string
		labels []string
		want   domain.Urgency
	}{
		{"URGENT: reports overclaim", []string{"needs-decision"}, domain.UrgencyUrgent},
		{`BLOCKER: "no published SDK" is false`, []string{"question"}, domain.UrgencyUrgent},
		{"routine question", []string{"question"}, domain.UrgencyRoutine},
		{"sign-off required", []string{"needs-decision", "gate:human"}, domain.UrgencyGold},
		{"a bug that parks a loop", []string{"question", "bug"}, domain.UrgencyElevated},
	}
	for _, tc := range cases {
		iss := adapter.RawIssue{Number: 9, Title: tc.title, Labels: tc.labels, Author: "a-review-app"}
		if got := adapter.MapEscalation("assay", repo(), iss, testDir, stubMint, adapter.AnswerVerifier{}).Urgency; got != tc.want {
			t.Errorf("%q %v: urgency = %s, want %s", tc.title, tc.labels, got, tc.want)
		}
	}
}

// TestMalformedEscalationIsRenderedRaw — design §10: an escalation that does
// not parse still appears in the inbox, raw. Never silently dropped, because
// an unparseable question is still a stalled loop.
func TestMalformedEscalationIsRenderedRaw(t *testing.T) {
	body := "Situation: broken.\n\n### Options\n<!-- assay:options v2 -->\n#### `x` - hyphen not em-dash\nConsequence: nope.\n"
	iss := adapter.RawIssue{Number: 5, Title: "malformed", Body: body, Labels: []string{"needs-decision"}, Author: "a-review-app"}
	e := adapter.MapEscalation("assay", repo(), iss, testDir, stubMint, adapter.AnswerVerifier{})

	if e.RawBody == "" {
		t.Fatal("a malformed v2 body was not retained — the item would render with nothing to look at")
	}
	if e.Options != nil {
		t.Fatal("a malformed v2 block published options")
	}
	if e.Fields["options"].Confidence != domain.CouldNotCheck {
		t.Fatalf("options rendered %s on a malformed body; folding this into the prose-v1 case would silently "+
			"downgrade a broken escalation into a legacy one", e.Fields["options"].Confidence)
	}
	if e.Title == "" {
		t.Fatal("the row lost its title — it must still be findable")
	}
}

// TestEveryDerivedFieldCarriesAnAssessment is the standing invariant: a
// derived field with no verdict is a field nobody can audit.
func TestEveryDerivedFieldCarriesAnAssessment(t *testing.T) {
	iss := adapter.RawIssue{Number: 7, Title: "x", Body: "y", Labels: []string{"question"}, Author: "a-review-app"}
	e := adapter.MapEscalation("assay", repo(), iss, testDir, stubMint, adapter.AnswerVerifier{})
	for _, f := range []string{"kind", "provenance", "automation", "urgency", "options"} {
		a, ok := e.Fields[f]
		if !ok {
			t.Errorf("derived field %q carries no assessment", f)
			continue
		}
		if err := a.Validate(); err != nil {
			t.Errorf("field %q: %v", f, err)
		}
	}
}
