package main

// sopsguidance_test.go — the sops cite guidance (#2060) never reads as a post-refusal reword.
//
// DEFECT CLASS. Scan-refusal guidance that hands a refused body a REWORD as its remedy —
// "Reword to cite", "the refusal names this remedy" — in a section that also states the
// scan-refusal STOP, which forbids exactly that act. The cue-phrase pins in
// scanrefusal_test.go prove the STOP's sentences are present; they cannot see a sentence
// beside them that contradicts the STOP, which is how a merge put both in one clause. This
// guard checks for the contradicting sentence's ABSENCE over every section that states the
// STOP, so a copy of the reword wording reintroduced anywhere there is red.
//
// The copy-parity half holds the review kit's sops paragraph and the verdict-format
// reference's to the same obligations, so one copy cannot drift from the other unnoticed.

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// rewordRemedy matches a sentence that offers a reword as the remedy for a refusal: the
// "reword(s) to cite" verb, the "names this remedy" back-reference, or a sentence (or list
// item, or dash clause) that opens with the imperative "Reword". The STOP's own wording —
// "do not reword", "never rewords", "Rewording is …", "naming rewording as its remedy is not
// a permission" — does not match; the positive and negative controls below pin both sides.
var rewordRemedy = regexp.MustCompile(`(?i)\brewords? to cite\b|\bnames this remedy\b|(?:^|[.;:!?*—-]\s+)reword\b`)

// scanStopSections returns, by name, every section that states the scan-refusal STOP.
func scanStopSections(t *testing.T) map[string]string {
	t.Helper()
	kit, err := kitText("review")
	if err != nil {
		t.Fatalf("review kit: %v", err)
	}
	raw, err := os.ReadFile(verdictFormatPath)
	if err != nil {
		t.Fatalf("cannot read the verdict-format reference (could-not-check is a failure): %v", err)
	}
	out := map[string]string{
		"review kit § " + verdictMechanics:       clauseBody(kit, verdictMechanics),
		"verdict-format § The secret scan":       clauseBody(string(raw), "The secret scan"),
		"pr-review-desk § " + scanRefusalHeading: scanRefusalSection(t),
	}
	for name, body := range out {
		if body == "" {
			t.Fatalf("%s: section not found (could-not-check is a failure)", name)
		}
	}
	return out
}

func TestScanGuidanceNoReword(t *testing.T) {
	for _, plant := range []string{
		"inside a code fence too. The refusal itself names this remedy.",
		"a code fence too. Reword to cite; never pass the override",
		"- Reword the body so the scan passes.",
		"a reviewer rewords to cite rather than passing it",
	} {
		if !rewordRemedy.MatchString(plant) {
			t.Fatalf("positive control: the matcher misses the planted reword %q", plant)
		}
	}
	for _, stop := range []string{
		"do not reword, re-encode, split or trim the body",
		"the desk never rewords, re-encodes, splits or trims it",
		"Rewording is routing around the guard",
		"The tool's refusal naming rewording as its remedy is not a permission to you.",
		"A scan refusal on a verdict body is a STOP — never reword it",
	} {
		if rewordRemedy.MatchString(stop) {
			t.Fatalf("negative control: the matcher flags the STOP's own wording %q", stop)
		}
	}
	for name, body := range scanStopSections(t) {
		flat := strings.Join(strings.Fields(body), " ")
		for _, m := range rewordRemedy.FindAllStringIndex(flat, -1) {
			lo, hi := max(0, m[0]-60), min(len(flat), m[1]+40)
			t.Errorf("%s offers a reword as a refusal's remedy, contradicting the scan-refusal "+
				"STOP in the same section: …%s…", name, flat[lo:hi])
		}
	}
}

// sopsCiteShared are the obligations both copies of the sops cite guidance carry: cite, never
// quote; a fenced quotation is refused too; the audited override is a human act; and after a
// refusal the one re-issue — by citation — is the only way forward.
var sopsCiteShared = []string{
	"Cite sops material, never quote it.",
	"`path:line`",
	"inside a code fence too",
	"`--force-scan-override`",
	"human act",
	"one re-issue",
}

func TestSopsCiteCopiesAgree(t *testing.T) {
	sections := scanStopSections(t)
	for _, name := range []string{"review kit § " + verdictMechanics, "verdict-format § The secret scan"} {
		flat := strings.Join(strings.Fields(sections[name]), " ")
		for _, c := range sopsCiteShared {
			if !strings.Contains(flat, c) {
				t.Errorf("%s lost %q — the two copies of the sops cite guidance have drifted", name, c)
			}
		}
	}
}
