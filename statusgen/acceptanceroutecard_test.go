package main

import (
	"strings"
	"testing"
)

func TestAdjudicatedCardRefRoundTrip(t *testing.T) {
	ref := adjudicatedCardRef{
		Preparation: rlIssue, Ruling: rlURL(rlRepo, rlIssue, rlComment), Manifest: strings.Repeat("a", 64),
	}
	marker, err := ref.marker()
	if err != nil {
		t.Fatal(err)
	}
	body := "<!-- verify-gate: demo/01 -->\n" + marker + "\n\nClose to accept."
	got, routed, err := parseAdjudicatedCardRef(body)
	if err != nil || !routed || got != ref {
		t.Fatalf("expected exact typed coordinates, got %+v %v (%v)", got, routed, err)
	}
	for name, invalid := range map[string]string{
		"duplicate":    body + "\n" + marker,
		"malformed":    strings.Replace(body, ref.Manifest, "truncated", 1),
		"noncanonical": strings.Replace(body, adjudicatedCardPrefix, adjudicatedCardPrefix+" ", 1),
	} {
		t.Run(name, func(t *testing.T) {
			if _, present, err := parseAdjudicatedCardRef(invalid); !present || err == nil {
				t.Fatalf("present but invalid must refuse, got present=%v err=%v", present, err)
			}
		})
	}
	if _, present, err := parseAdjudicatedCardRef("<!-- verify-gate: demo/01 -->"); present || err != nil {
		t.Fatalf("ordinary marker must remain ordinary, got present=%v err=%v", present, err)
	}
}
