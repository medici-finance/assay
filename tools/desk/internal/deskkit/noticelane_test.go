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

// TestOneWayReadsHumanOnlySignals — leads that ONLY the digest's HumanOnlySignals list
// catches (no OneWayPatterns entry matches them) are still one-way. Pins the HumanOnlySignals
// half of OneWay, which the class table above cannot: every lead there also matches a
// pattern (review finding cor-1688-C5).
//
// FAIL-FIRST: with OneWay's HumanOnlySignals branch disarmed (`false && s != nil`):
//
//	--- FAIL: TestOneWayReadsHumanOnlySignals
//	    OneWay("This change is irreversible.") = false, want true
func TestOneWayReadsHumanOnlySignals(t *testing.T) {
	for _, l := range []string{
		"This change is irreversible.",
		"It comes out of this quarter's budget.",
		"This filing also touches a security control on the ledger boundary.",
	} {
		hay := strings.ToLower(l)
		if FirstHumanOnlySignal(hay) == nil {
			t.Fatalf("fixture: %q matches no HumanOnlySignals needle", l)
		}
		for _, p := range OneWayPatterns {
			if m := p.Re.FindString(hay); m != "" {
				t.Fatalf("fixture: %q also matches OneWayPattern %s (`%s`) — it cannot isolate the list", l, p.Name, m)
			}
		}
		if _, ok := OneWay(l, "", nil); !ok {
			t.Errorf("OneWay(%q) = false, want true", l)
		}
	}
}

// TestNoticeLaneVerdictRefusesControlPhrasings — the security review's round-2 probes
// (head 834c4f8d5): a reversible R-3 subject paired with one-way wording the first pattern
// set missed. Every one must stay with the human.
//
// FAIL-FIRST: at head 834c4f8d5 every case was admitted, e.g.:
//
//	--- FAIL: TestNoticeLaneVerdictRefusesControlPhrasings
//	    NoticeLaneVerdict(..., "Turn off the 2FA requirement") admitted (reversible: tool default (R-3 example) (`tool default`)), want refused
//	    NoticeLaneVerdict(..., "Charge the card on file") admitted (reversible: tool default (R-3 example) (`tool default`)), want refused
func TestNoticeLaneVerdictRefusesControlPhrasings(t *testing.T) {
	for _, body := range []string{
		// the review's end-to-end titles
		"Flip the tool default of the commit guard: allow commits straight to main without a PR?",
		"Default value of --required-reviewers: 1 or 0?",
		"Tool default for the ready step: let the desk take PRs out of draft itself?",
		"Flag default for --no-verify on desk commits: on or off?",
		"Tool default: stop requiring 2FA for bot accounts?",
		"Tool default: auto-close needs-decision issues older than 30 days?",
		"Rename the repository and move it under the other org?",
		"Tool default for the trust gate: act on comments from any commenter?",
		// the review's NoticeLaneVerdict-level probes
		"Turn off the 2FA requirement",
		"Drop the MFA requirement",
		"Archive the repository",
		"Change repo visibility so anyone can see it",
		"Allow unsigned commits",
		"Let the reviewer App dismiss stale reviews",
		"Give the bot the org owner role",
		"Turn off the scan in CI",
		"Charge the card on file",
		// the control-verb class beside other control nouns
		"Switch off the pre-commit check",
		"Skip the guard on bot commits",
		"No longer require a review on docs-only changes",
		"Opt out of the secret scan for fixtures",
		"Allow the desk to land changes without the gate",
		"Commit directly to main for doc fixes",
		"Move the PR from draft to ready automatically",
		"Add the bot to the roster",
		"Make the repo private",
		"Transfer the repo to a new owner",
		"Two-factor for bot accounts: keep it or drop it?",
		"Close the human-only issues after a month",
	} {
		if admit, why := NoticeLaneVerdict("the tool default for --sla-days", body, nil); admit {
			t.Errorf("NoticeLaneVerdict(..., %q) admitted (%s), want refused", body, why)
		}
	}
}

// TestNoticeLaneVerdictStillAdmitsReversible — the widened set must not swallow R-3's own
// reversible examples; a false one-way costs a queue item, but a set that refuses every
// reversible item makes the lane dead.
func TestNoticeLaneVerdictStillAdmitsReversible(t *testing.T) {
	for _, title := range []string{
		"flip the tool default for --sla-days",
		"fix the docs wording in the README",
		"lint level for the unrun check: notice or error?",
		"rename the digest's Age column",
		"flag default for --window: 7 or 14 days?",
	} {
		if admit, why := NoticeLaneVerdict(title, "", nil); !admit {
			t.Errorf("NoticeLaneVerdict(%q) refused (%s), want admitted", title, why)
		}
	}
}

// TestOneWayExemptingRuling — OneWayExempting reads every one-way list EXCEPT the named
// HumanOnlySignals needles: the ruled-check line's "no prior ruling found" is not one-way,
// but a one-way subject on that same line still is.
func TestOneWayExemptingRuling(t *testing.T) {
	if hit, ok := OneWayExempting(`ruled-check: searched the tracker for "sla default" → no prior ruling found`, "ruling"); ok {
		t.Errorf("exempt `ruling` still matched: %s", hit)
	}
	if _, ok := OneWayExempting("ruled-check: merge and tag the v1.2.0 release -> nothing", "ruling"); !ok {
		t.Error("a one-way subject on the ruled-check line was not read")
	}
	if _, ok := OneWayExempting("ruled-check: this is irreversible -> nothing", "ruling"); !ok {
		t.Error("a non-exempt HumanOnlySignals needle on the ruled-check line was not read")
	}
}
