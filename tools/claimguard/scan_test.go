package main

import "testing"

// TestScan_UnresolvedCompetitorClaim_Flagged is the fail-first case this
// tool exists for: a named-competitor claim with no citation at all must be
// flagged. Modeled on the triggering evidence (an invented "SLAW" claim with
// no source, and an invented "VerdictCI" name asserted flatly).
func TestScan_UnresolvedCompetitorClaim_Flagged(t *testing.T) {
	text := "The competitor's product VerdictCI claims industry-leading SLAW support " +
		"with no comparable option on the market."

	findings := Scan(text, newDefaultAllow(), 0)

	want := map[string]bool{"VerdictCI": false, "SLAW": false}
	for _, f := range findings {
		if _, ok := want[f.Token]; ok {
			want[f.Token] = true
		}
	}
	for tok, found := range want {
		if !found {
			t.Errorf("expected %q to be flagged as an unresolved claim; findings=%+v", tok, findings)
		}
	}
}

// TestScan_ResolvedCompetitorClaim_Passes is the corresponding pass case:
// the same shape of claim, this time carrying a resolving citation, must NOT
// be flagged — a markdown link around the name, and a bare URL nearby.
func TestScan_ResolvedCompetitorClaim_Passes(t *testing.T) {
	text := "The competitor's product [VerdictCI](https://get-verdict.example/product) claims " +
		"industry-leading SLAW support, see https://vendor.example/slaw-spec for the spec."

	findings := Scan(text, newDefaultAllow(), 0)

	for _, f := range findings {
		if f.Token == "VerdictCI" || f.Token == "SLAW" {
			t.Errorf("expected %q not to be flagged once a resolving URL is present; findings=%+v", f.Token, findings)
		}
	}
}

// TestScan_FarAwayURL_StillFlagged proves the window is bounded: a URL many
// tokens away (a different paragraph) does not silently resolve a claim.
func TestScan_FarAwayURL_StillFlagged(t *testing.T) {
	filler := ""
	for i := 0; i < 40; i++ {
		filler += "word "
	}
	text := "Their VerdictCI offering is unmatched. " + filler + "See https://unrelated.example/ for details."

	findings := Scan(text, newDefaultAllow(), 5) // small window forces the far URL out of range

	found := false
	for _, f := range findings {
		if f.Token == "VerdictCI" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected VerdictCI to be flagged when the only URL is far outside the window; findings=%+v", findings)
	}
}

// TestScan_DefaultAllowlist_NoFalsePositive checks the common-acronym
// allowlist actually suppresses ordinary technical prose.
func TestScan_DefaultAllowlist_NoFalsePositive(t *testing.T) {
	text := "Send a request to the API over HTTPS and parse the JSON response using the SDK; " +
		"see GitHub for the source and OAuth for auth."

	findings := Scan(text, newDefaultAllow(), 0)
	if len(findings) != 0 {
		t.Errorf("expected no findings from common allowlisted acronyms/products, got %+v", findings)
	}
}

// TestScan_CustomAllow_Suppresses checks a caller-supplied allow entry
// (a house/product term) suppresses a candidate that would otherwise flag.
func TestScan_CustomAllow_Suppresses(t *testing.T) {
	text := "Built on the OurCoreEngine platform with no external dependency."
	allow := newDefaultAllow()
	allow["OurCoreEngine"] = true

	findings := Scan(text, allow, 0)
	for _, f := range findings {
		if f.Token == "OurCoreEngine" {
			t.Errorf("expected caller-supplied allowlist entry to suppress the finding; findings=%+v", findings)
		}
	}
}

// TestScan_LineNumbers checks a finding's reported line matches where the
// candidate actually appears, across a multi-line document.
func TestScan_LineNumbers(t *testing.T) {
	text := "Line one is plain prose.\n" +
		"Line two names CompetitorWidget with nothing to back it up.\n" +
		"Line three is unrelated.\n"

	findings := Scan(text, newDefaultAllow(), 0)
	if len(findings) != 1 {
		t.Fatalf("expected exactly 1 finding, got %d: %+v", len(findings), findings)
	}
	if findings[0].Line != 2 {
		t.Errorf("expected line 2, got %d", findings[0].Line)
	}
	if findings[0].Token != "CompetitorWidget" {
		t.Errorf("expected token CompetitorWidget, got %q", findings[0].Token)
	}
}
