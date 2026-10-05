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
		{"fix the docs wording in the README", true},
		{"fix the docs wording before the release", false}, // one-way outranks reversible
		{"flip the tool default for --sla-days", false},    // shape-only signal: fail closed
		{"pick the retry backoff shape", false},            // neither list: fail closed
	}
	for _, c := range cases {
		// subject = title: these fixtures are single-clause, so the title itself is the
		// declared subject (round 4 binds admission to subject alone; see noticelane.go).
		if got, why := NoticeLaneVerdict(c.title, "", c.title, nil); got != c.admit {
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
		// subject is fixed (the reversible half); body carries the one-way phrasing under
		// test — the one-way check still reads title+body (unchanged by round 4).
		if admit, why := NoticeLaneVerdict("fix the docs wording of the --sla-days help text", body,
			"fix the docs wording of the --sla-days help text", nil); admit {
			t.Errorf("NoticeLaneVerdict(..., %q) admitted (%s), want refused", body, why)
		}
	}
}

// TestNoticeLaneVerdictStillAdmitsReversible — the widened set must not swallow R-3's own
// reversible examples; a false one-way costs a queue item, but a set that refuses every
// reversible item makes the lane dead. "lint level for the unrun check" is deliberately NOT
// on this list: it names a CI check, so round 4 (sec-1688-S1) refuses it — see
// TestNoticeLaneVerdictRefusesSubjectAboutCICheckOrJob. Nor is any lint-level/lint-severity/
// notice-or-error/port-or-drop example, named check or not: round 5 (sec-1688-S1) moved that
// whole needle set into NoticeLaneShapeOnlyNeedles, so it never admits on its own — see
// TestNoticeLaneVerdictShapeOnlyNotAdmitted.
func TestNoticeLaneVerdictStillAdmitsReversible(t *testing.T) {
	for _, title := range []string{
		"fix the docs wording in the README",
		"fix the typo in the digest header",
		"table column order in the digest",
	} {
		if admit, why := NoticeLaneVerdict(title, "", title, nil); !admit {
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

// TestNoticeLaneVerdictShapeOnlyNotAdmitted — a shape-only reversible needle never admits the
// notice lane on its own: "tool default", "default value", "flag default" and "rename the"
// name only the shape of a change, not what it governs (security review sec-1688-S1, round
// 3); "lint level", "lint severity", "notice or error" and "port-or-drop" are, by
// construction, always a classification question about SOME check or job, named or not
// (round 5 — see TestNoticeLaneVerdictRefusesNamedCheckByLintOrPortNeedle for the
// named-check half). Each title here matches no one-way term; the refusal names the
// shape-only signal.
func TestNoticeLaneVerdictShapeOnlyNotAdmitted(t *testing.T) {
	for _, title := range []string{
		"flip the tool default for --sla-days",
		"Default value of --window: 7 or 14?",
		"flag default for --window: 7 or 14 days?",
		"rename the digest's Age column",
		"Tool default: build untrusted fork heads in CI?",
		"Tool default: scale the worker pool to zero overnight?",
		"lint level for trailing whitespace: notice or error?",
		"port-or-drop the legacy helper scripts",
	} {
		admit, why := NoticeLaneVerdict(title, "", title, nil)
		if admit {
			t.Errorf("NoticeLaneVerdict(%q) admitted (%s), want refused", title, why)
			continue
		}
		if !strings.Contains(why, "shape-only") && !strings.HasPrefix(why, "one-way") {
			t.Errorf("NoticeLaneVerdict(%q) refused for %q, want the shape-only reason (or a one-way hit)", title, why)
		}
	}
	// Every shape-only needle is a real ReversibleSignals needle (so the digest still reads
	// it), and FirstNoticeLaneSignal never returns one.
	for _, n := range NoticeLaneShapeOnlyNeedles {
		if s := FirstReversibleSignal(n); s == nil || s.Needle != n {
			t.Errorf("shape-only needle %q is not a ReversibleSignals needle", n)
		}
		if s := FirstNoticeLaneSignal(n); s != nil {
			t.Errorf("FirstNoticeLaneSignal(%q) = %q, want nil", n, s.Needle)
		}
	}
}

// TestNoticeLaneVerdictRefusesSubjectAboutCICheckOrJob — round 4 (security review
// sec-1688-S1): a subject that genuinely, single-clause names a lint level, a lint severity,
// a notice-vs-error choice, or a port-or-drop question is STILL not reversible when what it
// classifies is a named CI check or job — that is a security/governance decision about the
// check, not an edit to it. Every subject below carries a real content-bearing R-3 needle (so
// subject-binding alone, without this rule, would admit it); ciCheckOrJobRe is what refuses
// it. Mirrors the arbiter packet's round-4 probes (issuecomment-5840449031) at the
// deskkit.NoticeLaneVerdict level; forkgate_round4_test.go pins the same shapes end to end
// through `deskfile new`.
func TestNoticeLaneVerdictRefusesSubjectAboutCICheckOrJob(t *testing.T) {
	for _, subject := range []string{
		"lint level for the control-sweep check: notice or error?",
		"pattern-sweep findings: notice or error?",
		"lint severity of the leak check: warn only?",
		"port-or-drop the pattern-sweep job?",
	} {
		if admit, why := NoticeLaneVerdict(subject, "", subject, nil); admit {
			t.Errorf("NoticeLaneVerdict(%q) admitted (%s), want refused (subject names a CI check or job)", subject, why)
		}
	}
}

// TestNoticeLaneVerdictRefusesNamedCheckByLintOrPortNeedle — round 5 (security review
// sec-1688-S1, re-review at 8eb647757): ciCheckOrJobRe recognises only the generic
// check/job/workflow/pipeline nouns and this codebase's own "<word>-sweep"/"<word> check"
// compounds — never a CI check's OWN NAME. A subject whose only reversible needle is lint
// level / lint severity / notice or error / port-or-drop, but which names the check by its
// bare name instead of a generic noun, used to admit for exactly that reason. None of the six
// subjects below matches ciCheckOrJobRe at all (asserted below), so a fix that only widened
// that noun list would still miss every one of them — this pins that the shape-only floor
// (NoticeLaneShapeOnlyNeedles) is what refuses them, unconditionally, not a longer denylist.
func TestNoticeLaneVerdictRefusesNamedCheckByLintOrPortNeedle(t *testing.T) {
	for _, subject := range []string{
		"lint level for pin-consistency: notice or error?",
		"lint severity for skillslint findings: warn only?",
		"port-or-drop forge-surface?",
		"lint level for the build-test step: notice or error?",
		"govulncheck findings: notice or error?",
		"lint level for the CodeQL scan: notice or error?",
	} {
		if namesCICheckOrJob(strings.ToLower(subject)) {
			t.Fatalf("fixture %q matches ciCheckOrJobRe — it must pin the shape-only floor alone, not the CI-noun backstop", subject)
		}
		if admit, why := NoticeLaneVerdict(subject, "", subject, nil); admit {
			t.Errorf("NoticeLaneVerdict(%q) admitted (%s), want refused", subject, why)
		}
	}
}

// TestNoticeLaneShapeOnlyNeedleVetoesPairedNeedle — round 6 (correctness
// re-review finding cor-1688-C7, residual; security review sec-1688-S1, round 6):
// FirstNoticeLaneSignal used to SKIP the four lint-level/port-or-drop shape-only needles
// while scanning for an admitting needle, rather than treating a match on one of them as a
// veto — so a subject pairing a shape-only phrase with an unrelated content-bearing needle
// still admitted through that second needle. Both subjects below are the exact cases the two
// re-reviews named; each carries a real content-bearing ReversibleSignals needle ("wording")
// or names a real check by its bare name, and each must still refuse.
//
// FAIL-FIRST (FirstNoticeLaneSignal reverted to `continue` instead of `return nil` on a
// shape-only match): "wording of the pin-consistency lint level: notice or error" admitted via
// the "wording" needle.
func TestNoticeLaneShapeOnlyNeedleVetoesPairedNeedle(t *testing.T) {
	for _, subject := range []string{
		"wording of the pin-consistency lint level: notice or error",
		"port-or-drop forge-surface",
	} {
		if admit, why := NoticeLaneVerdict(subject, "", subject, nil); admit {
			t.Errorf("NoticeLaneVerdict(%q) admitted (%s), want refused — a shape-only needle must veto, never be outvoted by a paired content needle", subject, why)
		}
	}
}

// TestNoticeLaneVerdictWordBoundaryOnWording — round 6 advisory (security review
// sec-1688-S1: "Substring match on 'wording'"): needle matches must respect word boundaries.
// "rewording" is not "wording" — the bare substring match used to admit it anyway.
//
// FAIL-FIRST (wordBoundaryContains reverted to a plain strings.Contains): this subject
// admitted via the "wording" needle matched inside "rewording".
func TestNoticeLaneVerdictWordBoundaryOnWording(t *testing.T) {
	subject := "rewording: make pin-consistency a notice instead of an error"
	if admit, why := NoticeLaneVerdict(subject, "", subject, nil); admit {
		t.Errorf("NoticeLaneVerdict(%q) admitted (%s) — \"wording\" must not match inside \"rewording\"", subject, why)
	}
}

// TestNoticeLaneVerdictCICheckBackstopCatchesContentNeedle — sec-1688-S5: ciCheckOrJobRe had
// no test that could fail when disarmed, because every existing test that reached it used a
// lint-level/port-or-drop needle, and those are now refused earlier as shape-only (round 5)
// regardless of ciCheckOrJobRe. This subject carries a DIFFERENT content-bearing needle
// ("typo") next to a generic check/job noun via the "<word>-sweep" compound
// ("pattern-sweep"), so only the ciCheckOrJobRe backstop — not the shape-only veto — refuses
// it.
//
// FAIL-FIRST (mutation ME — the `if namesCICheckOrJob(subject) { return nil }` early
// return in FirstNoticeLaneSignal disarmed; it read ciCheckOrJobRe directly before round 7.2): this subject admitted via the "typo" needle.
func TestNoticeLaneVerdictCICheckBackstopCatchesContentNeedle(t *testing.T) {
	subject := "fix the typo in the pattern-sweep job message"
	if !namesCICheckOrJob(subject) {
		t.Fatalf("fixture %q does not match ciCheckOrJobRe — it must pin the backstop, not the shape-only floor", subject)
	}
	for _, n := range NoticeLaneShapeOnlyNeedles {
		if wordBoundaryContains(subject, n) {
			t.Fatalf("fixture %q matches shape-only needle %q — it must pin the ciCheckOrJobRe backstop alone", subject, n)
		}
	}
	if admit, why := NoticeLaneVerdict(subject, "", subject, nil); admit {
		t.Errorf("NoticeLaneVerdict(%q) admitted (%s), want refused (ciCheckOrJobRe backstop)", subject, why)
	}
}

// TestNoticeLaneVerdictHyphenVariantsNormalized — round 6 advisory (security review
// sec-1688-S1: "fix the typo in the pattern‑sweep message" with U+2011 NON-BREAKING HYPHEN got
// past ciCheckOrJobRe). A check name typed with a Unicode hyphen look-alike must still match
// the same as its ASCII-hyphen spelling. Deliberately avoids any bare "job"/"check"/
// "workflow"/"pipeline" noun elsewhere in the sentence: ciCheckOrJobRe's second alternation
// would catch those regardless of hyphen normalisation, masking the very thing this test
// pins — that the "<word>-sweep" compound itself is recognised however its hyphen is typed.
//
// FAIL-FIRST (normalizeHyphens dropped from NoticeLaneVerdict's subj derivation): every
// variant below admitted (true) via the "typo" needle, where the ASCII-hyphen spelling
// refused (false, ciCheckOrJobRe backstop) — admitVariant != admitASCII.
func TestNoticeLaneVerdictHyphenVariantsNormalized(t *testing.T) {
	ascii := "fix the typo in the pattern-sweep output"
	admitASCII, whyASCII := NoticeLaneVerdict(ascii, "", ascii, nil)
	if admitASCII {
		t.Fatalf("fixture: NoticeLaneVerdict(%q) admitted (%s), want refused (ciCheckOrJobRe backstop) — fixture is not isolating the hyphen normalisation", ascii, whyASCII)
	}
	for _, variant := range []string{
		"fix the typo in the pattern‐sweep output",
		"fix the typo in the pattern‑sweep output",
		"fix the typo in the pattern–sweep output",
	} {
		admitVariant, why := NoticeLaneVerdict(variant, "", variant, nil)
		if admitVariant != admitASCII {
			t.Errorf("NoticeLaneVerdict(%q) admitted=%v (%s), want the same as the ASCII-hyphen spelling (admitted=%v)", variant, admitVariant, why, admitASCII)
		}
	}
}

// TestNoticeLaneVerdictRound7HyphenVariantsNormalized — round 7 widens hyphenVariantReplacer
// with further Unicode hyphen/dash look-alikes named in the withheld review detail:
// FULLWIDTH HYPHEN-MINUS, SMALL HYPHEN-MINUS, SOFT HYPHEN (which renders as no visible
// character at all), and HYPHEN BULLET. Same isolation discipline as
// TestNoticeLaneVerdictHyphenVariantsNormalized: no bare "job"/"check"/"workflow"/"pipeline"
// noun elsewhere in the sentence, so only the "<word>-sweep" compound's own hyphen is being
// exercised.
//
// FAIL-FIRST (the four new entries removed from hyphenVariantReplacer): every variant below
// admitted (true) via the "typo" needle, where the ASCII-hyphen spelling refused (false,
// ciCheckOrJobRe backstop) — admitVariant != admitASCII.
func TestNoticeLaneVerdictRound7HyphenVariantsNormalized(t *testing.T) {
	ascii := "fix the typo in the pattern-sweep output"
	admitASCII, whyASCII := NoticeLaneVerdict(ascii, "", ascii, nil)
	if admitASCII {
		t.Fatalf("fixture: NoticeLaneVerdict(%q) admitted (%s), want refused (ciCheckOrJobRe backstop) — fixture is not isolating the hyphen normalisation", ascii, whyASCII)
	}
	for _, variant := range []string{
		"fix the typo in the pattern－sweep output",                // U+FF0D FULLWIDTH HYPHEN-MINUS
		"fix the typo in the pattern﹣sweep output",                // U+FE63 SMALL HYPHEN-MINUS
		"fix the typo in the pattern" + "\u00ad" + "sweep output", // U+00AD SOFT HYPHEN
		"fix the typo in the pattern⁃sweep output",                // U+2043 HYPHEN BULLET
	} {
		admitVariant, why := NoticeLaneVerdict(variant, "", variant, nil)
		if admitVariant != admitASCII {
			t.Errorf("NoticeLaneVerdict(%q) admitted=%v (%s), want the same as the ASCII-hyphen spelling (admitted=%v)", variant, admitVariant, why, admitASCII)
		}
	}
}

// TestNoticeLaneRefusesNonAsciiSubject — round 7.1 (security review sec-1688-S1
// advisory): normalizeHyphens only ever widens to a FINITE list of Unicode hyphen/dash
// look-alikes, so enumerating more code points chases the same class one variant at a time.
// This subject carries a genuine, otherwise-admitting content needle ("docs wording") plus one
// trailing zero-width space (U+200B, invisible in any renderer) nowhere near the needle itself
// — proving the refusal below is the non-ASCII gate, not a needle match broken by the extra
// character.
func TestNoticeLaneRefusesNonAsciiSubject(t *testing.T) {
	ascii := "fix the docs wording of the --sla-days help text"
	admitASCII, whyASCII := NoticeLaneVerdict(ascii, "", ascii, nil)
	if !admitASCII {
		t.Fatalf("fixture: NoticeLaneVerdict(%q) refused (%s), want admitted — fixture is not isolating the non-ASCII gate", ascii, whyASCII)
	}
	withZeroWidth := ascii + "​"
	admit, why := NoticeLaneVerdict(withZeroWidth, "", withZeroWidth, nil)
	if admit {
		t.Errorf("NoticeLaneVerdict(%q) admitted (%s), want refused — a non-ASCII character anywhere in the subject must fail closed", withZeroWidth, why)
	}
	if !strings.Contains(why, "non-ASCII") {
		t.Errorf("why = %q, want it to name the non-ASCII character as the refusal reason", why)
	}
}

// TestNoticeLaneShapeOnlyNeedleCollapsesSeparators — round 7.1 (security review sec-1688-S1
// advisory, veto evasion): the shape-only veto's needles are written with a plain space
// ("lint level"), so a subject spelling the same phrase with an underscore, a hyphen, a dot or
// a doubled space used to slip past both the veto AND the shape-only exclusion, admitting
// through a paired content needle the veto is supposed to outrank. collapseSeparators closes
// the whole separator alphabet at once rather than enumerating each spelling as its own needle.
//
// Round 7.2 correction (cor-1688-C14's class): every subject here used to end in "notice or
// error", itself a shape-only needle spelled with plain spaces, so the veto fired on that phrase
// whatever "lint level" looked like and the test passed with collapseSeparators disarmed. The
// subjects now carry "lint level" as their ONLY shape-only phrase, next to the content needle
// "wording", and the fixture check below proves the veto is the only thing refusing them.
//
// FAIL-FIRST (mutation: collapseSeparators reduced to `return s`): the underscore, dot and
// doubled-space rows admit through "wording".
func TestNoticeLaneShapeOnlyNeedleCollapsesSeparators(t *testing.T) {
	for _, subject := range []string{
		"wording of the pin-consistency lint level",
		"wording of the pin-consistency lint_level",
		"wording of the pin-consistency lint.level",
		"wording of the pin-consistency lint  level",
	} {
		if namesCICheckOrJob(subject) {
			t.Fatalf("fixture %q names a CI check or job — it must pin the shape-only veto alone", subject)
		}
		for _, n := range NoticeLaneShapeOnlyNeedles {
			if n != "lint level" && subjectContainsNeedle(subject, n) {
				t.Fatalf("fixture %q matches shape-only needle %q — only %q may veto it", subject, n, "lint level")
			}
		}
		if admit, why := NoticeLaneVerdict(subject, "", subject, nil); admit {
			t.Errorf("NoticeLaneVerdict(%q) admitted (%s), want refused — the shape-only 'lint level' veto must match regardless of separator spelling", subject, why)
		}
	}
}

// TestNoticeLaneContentNeedleCollapsesSeparators — the content-admitting sibling of the veto
// test above: a genuinely reversible needle ("docs wording") must still admit when the subject
// spells it with a different separator than the needle list uses.
func TestNoticeLaneContentNeedleCollapsesSeparators(t *testing.T) {
	for _, subject := range []string{
		"fix the docs wording of the --sla-days help text",
		"fix the docs_wording of the --sla-days help text",
		"fix the docs.wording of the --sla-days help text",
	} {
		if admit, why := NoticeLaneVerdict(subject, "", subject, nil); !admit {
			t.Errorf("NoticeLaneVerdict(%q) refused (%s), want admitted — 'docs wording' must match regardless of separator spelling", subject, why)
		}
	}
}

// TestDeskDecidedMarkerClaimCoversReader — the caller-body refusal
// (HasDeskDecidedMarkerClaim) matches every spelling the reader (DeskDecidedMarkerRe) accepts,
// so a variant marker can never be filed by a caller and then read as a desk decision
// (review findings cor-1688-C6, sec-1688-S4).
func TestDeskDecidedMarkerClaimCoversReader(t *testing.T) {
	for _, m := range []string{
		DeskDecidedMarker,
		"<!--desk-r3-decision v1-->",
		"<!-- DESK-R3-DECISION V1 -->",
		"<!--  desk-r3-decision v1  -->",
		"<!--  desk-r3-decision v1\n-->",
		"<!--\tDesk-R3-Decision v1\t-->",
	} {
		if !DeskDecidedMarkerRe.MatchString(m) {
			t.Errorf("fixture: the reader does not accept %q", m)
		}
		if !HasDeskDecidedMarkerClaim(m) {
			t.Errorf("HasDeskDecidedMarkerClaim(%q) = false, but the reader accepts it", m)
		}
	}
	// Broader than the reader: other versions and unclosed attempts are refused too.
	for _, m := range []string{"<!-- desk-r3-decision v2 -->", "<!-- desk-r3-decision"} {
		if !HasDeskDecidedMarkerClaim(m) {
			t.Errorf("HasDeskDecidedMarkerClaim(%q) = false, want true (the refusal is a superset)", m)
		}
	}
	if HasDeskDecidedMarkerClaim("the desk-r3-decision marker, named in prose") {
		t.Error("prose naming the marker, with no comment opener, was treated as a marker")
	}
}

// TestStripDeskDecidedBlockReadsMarkerVariants — the classifier strip uses the reader's own
// matcher, so a block the digest would read as marked is also the block it strips.
func TestStripDeskDecidedBlockReadsMarkerVariants(t *testing.T) {
	body := "Kept.\n\n## Desk-decided\n\n<!--DESK-R3-DECISION V1-->\ndecision: keep\ncost: draft-pr\n"
	if got := StripDeskDecidedBlock(body); strings.Contains(got, "cost:") {
		t.Errorf("a variant-marker block was not stripped:\n%s", got)
	}
}

// TestNoticeLaneCICheckBackstopCollapsesSeparators — round 7.2 (security review sec-1688-S1
// advisory): the needle scans collapse ASCII separators, but the CI check/job backstop read the
// raw subject only, so a check name spelled with an underscore or a dot ("leak_sweep" — an
// underscore is a word character, so `\b` never fires inside it) slipped past it while the
// content needle beside it ("typo") still admitted. Each subject is first shown to be missed
// by the raw regexp, so the refusal below is the collapsed reading, nothing else.
//
// FAIL-FIRST (mutation: namesCICheckOrJob reduced to the raw ciCheckOrJobRe.MatchString): every
// row admits through "typo".
func TestNoticeLaneCICheckBackstopCollapsesSeparators(t *testing.T) {
	for _, subject := range []string{
		"fix the typo in the leak_sweep message",
		"fix the typo in the leak.sweep message",
		"fix the typo in the ci_checks message",
	} {
		if ciCheckOrJobRe.MatchString(subject) {
			t.Fatalf("fixture %q already matches the raw regexp — it must pin the collapsed reading", subject)
		}
		for _, n := range NoticeLaneShapeOnlyNeedles {
			if subjectContainsNeedle(subject, n) {
				t.Fatalf("fixture %q matches shape-only needle %q — it must pin the backstop alone", subject, n)
			}
		}
		if admit, why := NoticeLaneVerdict(subject, "", subject, nil); admit {
			t.Errorf("NoticeLaneVerdict(%q) admitted (%s), want refused — the CI check/job backstop must match regardless of separator spelling", subject, why)
		}
	}
}

// TestNoticeLaneShapeOnlyVetoCoversPlurals — round 7.2 (security review sec-1688-S1 advisory,
// veto evasion): "lint levels" never matched the needle "lint level" on a word boundary, so the
// plural slipped past the veto and the paired content needle "wording" admitted. Each fixture
// carries no other shape-only phrase and names no CI check or job, so the plural veto is the
// only thing refusing it.
//
// FAIL-FIRST (mutation: subjectContainsVetoNeedle reduced to subjectContainsNeedle): every row
// admits through "wording".
func TestNoticeLaneShapeOnlyVetoCoversPlurals(t *testing.T) {
	for _, subject := range []string{
		"wording of the pin-consistency lint levels",
		"wording of the flag defaults",
		"wording of the tool defaults",
	} {
		if namesCICheckOrJob(subject) {
			t.Fatalf("fixture %q names a CI check or job — it must pin the veto alone", subject)
		}
		for _, n := range NoticeLaneShapeOnlyNeedles {
			if subjectContainsNeedle(subject, n) {
				t.Fatalf("fixture %q matches shape-only needle %q exactly — it must pin the plural reading", subject, n)
			}
		}
		if admit, why := NoticeLaneVerdict(subject, "", subject, nil); admit {
			t.Errorf("NoticeLaneVerdict(%q) admitted (%s), want refused — a plural shape-only phrase must still veto", subject, why)
		}
	}
}

// TestVetoNeedleEsPlural pins the "+es" arm of subjectContainsVetoNeedle on its own: the
// fixture needle's plural is spelled with "es", so neither the exact match nor the "+s" arm
// reaches it. No current shape-only needle pluralises with "es", so the arm is pinned at the
// function rather than through NoticeLaneVerdict.
//
// FAIL-FIRST (mutation: drop the `needle+"es"` arm): the plural no longer vetoes.
func TestVetoNeedleEsPlural(t *testing.T) {
	const subject, needle = "wording of the lint boxes", "lint box"
	if subjectContainsNeedle(subject, needle) || subjectContainsNeedle(subject, needle+"s") {
		t.Fatalf("fixture %q matches %q exactly or with +s — it must pin the +es arm alone", subject, needle)
	}
	if !subjectContainsVetoNeedle(subject, needle) {
		t.Errorf("subjectContainsVetoNeedle(%q, %q) = false, want true — an +es plural must still veto", subject, needle)
	}
}

// TestVetoNeedleIesPlural — a shape-only needle ending in "y" ("lint severity") vetoes its
// "-ies" plural too, end to end through NoticeLaneVerdict, beside an admitting content needle.
//
// FAIL-FIRST (round-7.2 code, before the "-ies" arm): the plural slipped past the veto and the
// paired content needle admitted the subject.
func TestVetoNeedleIesPlural(t *testing.T) {
	const subject = "wording of the lint severities"
	if namesCICheckOrJob(subject) {
		t.Fatalf("fixture %q names a CI check or job — it must pin the veto alone", subject)
	}
	if FirstNoticeLaneSignal("wording of the docs") == nil {
		t.Fatal("control: the content needle in the fixture no longer admits on its own")
	}
	if admit, why := NoticeLaneVerdict(subject, "", subject, nil); admit {
		t.Errorf("NoticeLaneVerdict(%q) admitted (%s), want refused — an -ies plural of a shape-only needle must still veto", subject, why)
	}
}
