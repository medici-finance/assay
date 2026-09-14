package deskkit

import (
	"strings"
	"testing"
	"time"
)

// excludedSignalDenylist is the set of identity-adjacent categories the provenance probe's
// brief and design record exclude BY DESIGN: profile text, avatar, follower/following counts,
// named employer, geography, and account name shape.
// TestProvenanceExcludedSignals scans every registry entry's Name, NotableBand and
// Explanation against this list, case-insensitively, so a signal that describes the PERSON
// rather than the SUBMISSION fails the build the moment it is added, whether or not its
// author read this file's header comment first.
var excludedSignalDenylist = []string{
	"profile",
	"avatar",
	"follower",
	"following",
	"employer",
	"geograph", // matches "geography" and "geographic"
	"location",
	"username",
	"account name",
	"real name",
	"display name",
}

func TestProvenanceExcludedSignals(t *testing.T) {
	if len(signalRegistry) == 0 {
		t.Fatal("signalRegistry is empty — nothing to check, which is itself suspicious for this test")
	}
	for _, spec := range signalRegistry {
		haystack := strings.ToLower(spec.Name + " " + spec.NotableBand + " " + spec.Explanation)
		for _, bad := range excludedSignalDenylist {
			if strings.Contains(haystack, bad) {
				t.Errorf("signal %q reads as identity-adjacent: its text contains the excluded category %q — signals must describe the SUBMISSION, never the PERSON", spec.Name, bad)
			}
		}
	}
}

func TestProvenanceRegistryHasNoAggregateField(t *testing.T) {
	// SignalResult and ProvenanceInput are asserted, by their own field lists in
	// provenance.go, to carry no score/rating/aggregate field. This test pins the
	// documentation claim against the actual rendered output: Card must never emit either
	// word, over every state a card can reach.
	for _, tc := range []struct {
		name  string
		input ProvenanceInput
	}{
		{"clean", ordinaryInput()},
		{"flagged", bulkSweepInput()},
		{"could-not-check", unreadableInput()},
	} {
		card := Card(Gather(tc.input))
		lower := strings.ToLower(card)
		for _, bad := range []string{"score", "verdict", "rating"} {
			if strings.Contains(lower, bad) {
				t.Errorf("%s card contains %q, which is a verdict word this probe must never render:\n%s", tc.name, bad, card)
			}
		}
	}
}

func TestProvenanceCardAlwaysCarriesAbstention(t *testing.T) {
	// Pre-mortem: "the flagged card reads as an accusation because the abstention only
	// renders on the clean path." Assert it on all three states, not just the happy one.
	for _, tc := range []struct {
		name  string
		input ProvenanceInput
	}{
		{"clean", ordinaryInput()},
		{"flagged", bulkSweepInput()},
		{"could-not-check", unreadableInput()},
	} {
		card := Card(Gather(tc.input))
		if !strings.Contains(card, "not a judgement of the change") {
			t.Errorf("%s card is missing the abstention (\"not a judgement of the change\"):\n%s", tc.name, card)
		}
	}
}

func TestProvenanceOverallCouldNotCheckDominates(t *testing.T) {
	// A card with one notable signal AND one unreadable signal must render
	// could-not-check, never flagged: a partially blind card is never presented with the
	// confidence a plain "flagged" carries.
	results := []SignalResult{
		{Name: "a", State: StateFlagged},
		{Name: "b", State: StateCouldNotCheck},
		{Name: "c", State: StateClean},
	}
	if got := Overall(results); got != StateCouldNotCheck {
		t.Fatalf("Overall() = %s, want could-not-check when a could-not-check signal is present alongside a flagged one", got)
	}
}

func TestProvenanceOverallFlaggedBeatsClean(t *testing.T) {
	results := []SignalResult{
		{Name: "a", State: StateClean},
		{Name: "b", State: StateFlagged},
	}
	if got := Overall(results); got != StateFlagged {
		t.Fatalf("Overall() = %s, want flagged", got)
	}
}

func TestProvenanceOverallAllCleanIsClean(t *testing.T) {
	results := []SignalResult{
		{Name: "a", State: StateClean},
		{Name: "b", State: StateClean},
	}
	if got := Overall(results); got != StateClean {
		t.Fatalf("Overall() = %s, want clean", got)
	}
}

func TestProvenanceExitCodes(t *testing.T) {
	cases := []struct {
		state ProvenanceState
		want  int
	}{
		{StateClean, 0},
		{StateFlagged, 1},
		{StateCouldNotCheck, 6},
	}
	for _, c := range cases {
		if got := c.state.ExitCode(); got != c.want {
			t.Errorf("%s.ExitCode() = %d, want %d", c.state, got, c.want)
		}
	}
}

func TestProvenanceGatherOrdinaryAuthorIsClean(t *testing.T) {
	results := Gather(ordinaryInput())
	if overall := Overall(results); overall != StateClean {
		t.Fatalf("ordinary author: Overall() = %s, want clean; results=%+v", overall, results)
	}
	for _, r := range results {
		if r.State != StateClean {
			t.Errorf("ordinary author: signal %q = %s, want clean", r.Name, r.State)
		}
	}
}

func TestProvenanceGatherBulkSweepIsFlagged(t *testing.T) {
	results := Gather(bulkSweepInput())
	if overall := Overall(results); overall != StateFlagged {
		t.Fatalf("bulk-sweep author: Overall() = %s, want flagged; results=%+v", overall, results)
	}
	notable := 0
	for _, r := range results {
		if r.State == StateFlagged {
			notable++
		}
		if r.State == StateCouldNotCheck {
			t.Errorf("bulk-sweep author: signal %q is could-not-check, want every signal readable in this fixture", r.Name)
		}
	}
	if notable == 0 {
		t.Fatal("bulk-sweep author: no signal came back flagged")
	}
}

func TestProvenanceGatherUnreadableIsCouldNotCheck(t *testing.T) {
	results := Gather(unreadableInput())
	if overall := Overall(results); overall != StateCouldNotCheck {
		t.Fatalf("unreadable author: Overall() = %s, want could-not-check; results=%+v", overall, results)
	}
	sawCouldNotCheck := false
	for _, r := range results {
		if r.State == StateCouldNotCheck {
			sawCouldNotCheck = true
			if r.Reason == "" {
				t.Errorf("signal %q is could-not-check with no Reason", r.Name)
			}
		}
	}
	if !sawCouldNotCheck {
		t.Fatal("expected at least one could-not-check signal")
	}
}

func TestProvenanceGatherCIPathsIsFlagged(t *testing.T) {
	results := Gather(ciPathsInput())
	if overall := Overall(results); overall != StateFlagged {
		t.Fatalf("ci-paths author: Overall() = %s, want flagged; results=%+v", overall, results)
	}
	card := Card(results)
	if !strings.Contains(card, "continuous-integration") {
		t.Errorf("ci-paths card does not mention continuous-integration:\n%s", card)
	}
}

func TestProvenanceCardIsDeterministic(t *testing.T) {
	in := bulkSweepInput()
	first := Card(Gather(in))
	second := Card(Gather(in))
	if first != second {
		t.Fatalf("Card() is not deterministic over identical input:\nfirst:\n%s\nsecond:\n%s", first, second)
	}
}

func TestProvenanceCleanCardNeverMentionsFlagged(t *testing.T) {
	card := Card(Gather(ordinaryInput()))
	if strings.Contains(card, "flagged") {
		t.Errorf("clean card mentions \"flagged\":\n%s", card)
	}
}

// --- shared fixtures for Go-level tests (the CLI's own fixtures live under
// cmd/deskprovenance/testdata/ as JSON and are exercised by the Verify table directly) ---

func ptrTime(t time.Time) *time.Time { return &t }
func ptrInt(i int) *int              { return &i }
func ptrBool(b bool) *bool           { return &b }
func ptrStrings(s []string) *[]string {
	return &s
}

func ordinaryInput() ProvenanceInput {
	created := time.Date(2019, 3, 1, 0, 0, 0, 0, time.UTC)
	firstActivity := created.Add(400 * 24 * time.Hour)
	forkCreated := time.Date(2026, 9, 1, 9, 0, 0, 0, time.UTC)
	prOpened := forkCreated.Add(3 * 24 * time.Hour)
	return ProvenanceInput{
		AccountCreatedAt:    ptrTime(created),
		FirstActivityAt:     ptrTime(firstActivity),
		ForkCreatedAt:       ptrTime(forkCreated),
		PROpenedAt:          ptrTime(prOpened),
		CrossRepoPRCount24h: ptrInt(1),
		PriorMergedCount:    ptrInt(9),
		PriorClosedCount:    ptrInt(1),
		ThisBody:            strPtr("Fixes a null pointer in the widget loader when config is absent."),
		RecentBodies:        ptrStrings([]string{"Adds a missing test for the widget loader."}),
		CommitsSigned:       ptrBool(true),
		ChangedPaths:        ptrStrings([]string{"internal/widget/loader.go", "internal/widget/loader_test.go"}),
	}
}

func bulkSweepInput() ProvenanceInput {
	created := time.Date(2026, 9, 12, 8, 0, 0, 0, time.UTC)
	firstActivity := created.Add(2 * time.Hour)
	forkCreated := time.Date(2026, 9, 12, 8, 10, 0, 0, time.UTC)
	prOpened := forkCreated.Add(90 * time.Second)
	return ProvenanceInput{
		AccountCreatedAt:    ptrTime(created),
		FirstActivityAt:     ptrTime(firstActivity),
		ForkCreatedAt:       ptrTime(forkCreated),
		PROpenedAt:          ptrTime(prOpened),
		CrossRepoPRCount24h: ptrInt(37),
		PriorMergedCount:    ptrInt(0),
		PriorClosedCount:    ptrInt(11),
		ThisBody:            strPtr("bump dependency to 1.2.3"),
		RecentBodies:        ptrStrings([]string{"bump dependency to 1.2.3", "bump dependency to 1.2.2"}),
		CommitsSigned:       ptrBool(false),
		ChangedPaths:        ptrStrings([]string{"README.md"}),
	}
}

func unreadableInput() ProvenanceInput {
	created := time.Date(2019, 3, 1, 0, 0, 0, 0, time.UTC)
	firstActivity := created.Add(400 * 24 * time.Hour)
	return ProvenanceInput{
		AccountCreatedAt: ptrTime(created),
		FirstActivityAt:  ptrTime(firstActivity),
		// fork/PR timestamps unavailable
		// cross-repo count unavailable
		PriorMergedCount: ptrInt(4),
		PriorClosedCount: ptrInt(1),
		// body/recent bodies unavailable
		// commit signature unavailable
		ChangedPaths: ptrStrings([]string{"docs/readme.md"}),
	}
}

func ciPathsInput() ProvenanceInput {
	in := ordinaryInput()
	in.ChangedPaths = ptrStrings([]string{".github/workflows/ci.yml", "go.sum"})
	return in
}

func strPtr(s string) *string { return &s }
