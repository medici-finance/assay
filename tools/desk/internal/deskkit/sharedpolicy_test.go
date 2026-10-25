package deskkit

import "testing"

func TestSharedPolicyConsumers(t *testing.T) {
	v := loadCouplingVectors(t)
	withRoster(t, v.Roster)
	if TrustedAuthorID("example-reviewer-app[bot]", 9999) || trustedContentAuthor("example-reviewer-app[bot]", 9999) {
		t.Fatal("wrong supplied bot ID admitted")
	}
	classed, _, readable := RiskFromBriefContent("---\ngate: human\nrisk: {regulatory: \"yes\"}\n---\n")
	if !classed || !readable {
		t.Fatal("quoted risk was missed")
	}
	if b := ParseBriefAdmission("---\ngate: \"human\"\nexec-tier: strong\n---\n"); !b.Frontmatter || !b.GatesHuman() {
		t.Fatal(b)
	}
	ts, e := ParseTrailers([]byte("~~~\nBrief: demo/99\n~~~\nBrief: demo/01\n"))
	if e != nil || len(ts) != 1 || ts[0].Value != "demo/01" {
		t.Fatal(ts, e)
	}
}
