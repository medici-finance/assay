package deskkit

import "testing"

func TestSharedPolicyConsumers(t *testing.T) {
	v := loadCouplingVectors(t)
	withRoster(t, v.Roster)
	if TrustedAuthorID("example-reviewer-app[bot]", 9999) || trustedContentAuthor("example-reviewer-app[bot]", 9999) {
		t.Error("wrong supplied bot ID admitted")
	}
	// gate: model, so the quoted risk answer alone must class the brief.
	classed, why, readable := RiskFromBriefContent("---\ngate: model\nrisk: {regulatory: \"yes\"}\n---\n")
	if !classed || !readable {
		t.Errorf("quoted risk was missed: classed=%v readable=%v %s", classed, readable, why)
	}
	if b := ParseBriefAdmission("---\ngate: \"human\"\nexec-tier: strong\n---\n"); !b.Frontmatter || !b.GatesHuman() {
		t.Error("quoted human gate was missed", b)
	}
	ts, e := ParseTrailers([]byte("~~~\nBrief: demo/99\n~~~\nBrief: demo/01\n"))
	if e != nil || len(ts) != 1 || ts[0].Value != "demo/01" {
		t.Error("tilde-fenced example counted as a work link", ts, e)
	}
}
