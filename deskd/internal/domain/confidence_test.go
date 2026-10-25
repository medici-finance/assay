package domain_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/deskd/internal/domain"
)

func probe(limit int, capped bool) domain.Probe {
	return domain.Probe{
		Source: "gh pr list --state merged --limit 500",
		Method: domain.MethodRESTList,
		Window: "merged pull requests, all time",
		Limit:  limit,
		Capped: capped,
	}
}

// TestCappedProbeRefusesToPublishACount is the measured failure this type
// exists to prevent, run as a positive control.
//
// A list call once returned exactly 500 rows against a true total of 958.
// Nothing about the 500 looked wrong: it was an integer, from a real API, in
// a field called "count". The only thing that could have caught it was the
// probe's own limit sitting beside the number.
//
// So: a Count built from a capped probe carries NO value, whatever integer
// was handed in.
func TestCappedProbeRefusesToPublishACount(t *testing.T) {
	c := domain.NewCount(500, probe(500, true))

	if v, ok := c.Value(); ok {
		t.Fatalf("POSITIVE CONTROL SURVIVED: a capped probe published %d as a total. "+
			"This is the exact shape of a 500-that-was-really-958.", v)
	}
	if c.Assessment().Confidence != domain.CouldNotCheck {
		t.Fatalf("a capped count rendered %s", c.Assessment().Confidence)
	}
	if !strings.Contains(c.Assessment().Reason, "500") || !strings.Contains(c.Assessment().Reason, "limit") {
		t.Errorf("the refusal does not state the cap it hit: %q", c.Assessment().Reason)
	}

	// And it must serialise as an explicit null, not as 0 and not omitted.
	raw, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	v, present := out["value"]
	if !present {
		t.Fatal("the value key was omitted; a consumer cannot tell an absent key from a missing measurement")
	}
	if v != nil {
		t.Fatalf("a capped count serialised as %#v, want null — a 0 here is a confident lie", v)
	}
	if out["probe"] == nil {
		t.Error("the count serialised without its probe")
	}
}

// TestUncappedCountPublishes is the control: the type is not simply refusing
// everything.
func TestUncappedCountPublishes(t *testing.T) {
	p := domain.Probe{
		Source: "gh api graphql: aliased by-number lookups",
		Method: domain.MethodGraphQLAliased,
		Window: "the 434 named pull requests",
		Limit:  0,
	}
	c := domain.NewCount(434, p)
	v, ok := c.Value()
	if !ok || v != 434 {
		t.Fatalf("an uncapped count refused to publish: %d ok=%v (%s)", v, ok, c.Assessment().Reason)
	}
	raw, _ := json.Marshal(c)
	var out map[string]any
	_ = json.Unmarshal(raw, &out)
	if out["value"] != float64(434) {
		t.Fatalf("value serialised as %#v", out["value"])
	}
}

// TestCountCannotBeBuiltWithoutAProbe — an unauditable probe is refused, so
// nobody can slip a number through by leaving the provenance blank.
func TestCountCannotBeBuiltWithoutAProbe(t *testing.T) {
	for name, p := range map[string]domain.Probe{
		"no source":            {Method: domain.MethodGraphQLAliased, Window: "w"},
		"no method":            {Source: "s", Window: "w"},
		"no window":            {Source: "s", Method: domain.MethodGraphQLAliased},
		"capped with no limit": {Source: "s", Method: domain.MethodRESTList, Window: "w", Capped: true},
	} {
		t.Run(name, func(t *testing.T) {
			if _, ok := domain.NewCount(7, p).Value(); ok {
				t.Fatalf("a count published under an unauditable probe (%s)", name)
			}
		})
	}
}

// TestAssessmentRejectsAnUnexplainedFailure — a could-not-check with no
// reason is indistinguishable from a bug, so it does not validate.
func TestAssessmentRejectsAnUnexplainedFailure(t *testing.T) {
	good := domain.Probe{Source: "s", Method: domain.MethodLocalTable, Window: "w"}
	if err := (domain.Assessment{Confidence: domain.CouldNotCheck, Probe: good}).Validate(); err == nil {
		t.Fatal("an unexplained could-not-check validated")
	}
	if err := (domain.Assessment{Probe: good}).Validate(); err == nil {
		t.Fatal("an assessment with no confidence at all validated — the zero value must not read as clean")
	}
	if err := domain.Clean(good).Validate(); err != nil {
		t.Fatalf("a well-formed clean assessment was rejected: %v", err)
	}
}

var refTime = time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)

// TestSortInboxIsUrgencyThenAge pins the ordering rule.
func TestSortInboxIsUrgencyThenAge(t *testing.T) {
	mk := func(id string, u domain.Urgency, hoursAgo int) domain.Escalation {
		return domain.Escalation{ID: domain.EscalationID(id), Urgency: u,
			FiledAt: refTime.Add(-time.Duration(hoursAgo) * time.Hour)}
	}
	items := []domain.Escalation{
		mk("c", domain.UrgencyRoutine, 100),
		mk("a", domain.UrgencyUrgent, 2),
		mk("b", domain.UrgencyUrgent, 50),
		mk("d", domain.UrgencyGold, 1),
	}
	domain.SortInbox(items)
	got := ""
	for _, i := range items {
		got += string(i.ID)
	}
	if got != "dbac" {
		t.Fatalf("order = %q, want %q (gold, then the older urgent, then the newer urgent, then routine)", got, "dbac")
	}
}
