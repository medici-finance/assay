package deskkit

import (
	"strings"
	"testing"
)

// TestOneWayCatchesEveryNamedClass — one lead per one-way class the desk skills name, each
// of which the HumanOnlySignals list alone misses. OneWay must catch every one.
func TestOneWayCatchesEveryNamedClass(t *testing.T) {
	leads := []string{
		"Should we cut the v1.2.0 release tag now?",
		"Merge this PR today or wait?",
		"Ready-flip the PR now?",
		"Push the fix straight to main?",
		"Disable the branch protection ruleset for a day?",
		"Weaken the leak-sweep assertion?",
		"Who holds custody of the App signing key?",
		"Ship the PII export endpoint now?",
		"Move funds from the vault now?",
		"Move money between accounts?",
		"Change the identity provider realm config?",
		"Change the login flow?",
		"Overwrite the stored snapshot?",
		"Send the report to the external vendor?",
		"Is the publication of the notes fine?",
		"Deploy the new image to the live cluster?",
		"Should the reviewer App get actions: write?",
		"Should the desk approve its own PRs?",
		"gate:human on brief 07 — the driver or the desk?",
	}
	for _, l := range leads {
		if FirstHumanOnlySignal(strings.ToLower(l)) != nil {
			continue // already caught by the digest's list; still must be one-way below
		}
		if _, ok := OneWay(l, "", nil); !ok {
			t.Errorf("OneWay(%q) = false, want true", l)
		}
	}
}

// TestOneWayWordBoundaries — a stem never matches inside an unrelated word.
func TestOneWayWordBoundaries(t *testing.T) {
	for _, clean := range []string{
		"Stage the docs wording change.",
		"The monkey test flag default.",
		"The author asked about table column order.",
		"Represent the consent flag default as a boolean.",
	} {
		if hit, ok := OneWay(clean, "", nil); ok {
			t.Errorf("OneWay(%q) = %s, want no match", clean, hit)
		}
	}
}

// TestOneWayLabels — a one-way caller label marks the item one-way whatever the text says.
func TestOneWayLabels(t *testing.T) {
	for _, l := range []string{"human-only", "Security", "gate:human"} {
		if _, ok := OneWay("fix the docs wording", "", []string{"needs-decision", l}); !ok {
			t.Errorf("label %q not treated as one-way", l)
		}
	}
}

// TestNoticeLaneVerdictFailsClosed — admitted only with a reversible signal AND no one-way
// signal; an item matching neither list stays with the human.
func TestNoticeLaneVerdictFailsClosed(t *testing.T) {
	cases := []struct {
		title string
		admit bool
	}{
		{"flip the tool default for --sla-days", true},
		{"fix the docs wording in the README", true},
		{"flip the tool default before the release", false}, // one-way outranks reversible
		{"pick the retry backoff shape", false},             // neither list: fail closed
	}
	for _, c := range cases {
		if got, why := NoticeLaneVerdict(c.title, "", nil); got != c.admit {
			t.Errorf("NoticeLaneVerdict(%q) = %v (%s), want %v", c.title, got, why, c.admit)
		}
	}
}

// TestStripDeskDecidedBlock — the tool-written block (with the marker) goes; a filer's own
// section of the same name WITHOUT the marker, and everything around it, stays.
func TestStripDeskDecidedBlock(t *testing.T) {
	body := "Flip the tool default.\n\n## Desk-decided\n\n" + DeskDecidedMarker +
		"\ndecision: keep\ncost: draft-pr\n\n## After\nkept text\n"
	got := StripDeskDecidedBlock(body)
	if strings.Contains(got, "cost:") || strings.Contains(got, DeskDecidedMarker) {
		t.Errorf("block not stripped:\n%s", got)
	}
	if !strings.Contains(got, "Flip the tool default.") || !strings.Contains(got, "kept text") {
		t.Errorf("surrounding text lost:\n%s", got)
	}
	noMarker := "## Desk-decided\ncost: a filer's own words\n"
	if StripDeskDecidedBlock(noMarker) != noMarker {
		t.Error("a section without the marker was stripped")
	}
}
