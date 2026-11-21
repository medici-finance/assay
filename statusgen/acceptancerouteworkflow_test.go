package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestClosedAdjudicatedCardRequiresCanonicalIssuerAndPinnedHuman(t *testing.T) {
	ref := adjudicatedCardRef{Preparation: rlIssue, Ruling: rlURL(rlRepo, rlIssue, rlComment), Manifest: strings.Repeat("a", 64)}
	marker, err := ref.marker()
	if err != nil {
		t.Fatal(err)
	}
	closer := scanIdentity{Login: rlHuman, ID: 1234}
	valid := creditIssue{State: "closed", Labels: []string{"verify-gate"}, Body: verifyMarker("vf/01") + "\n" + marker,
		Author:   &creditAccount{Login: "github-actions[bot]", ID: 41898282, Type: "Bot"},
		ClosedBy: &creditAccount{Login: rlHuman, ID: 1234, Type: "User"},
	}
	got, err := checkClosedAdjudicatedCard(&valid, "vf/01", closer)
	if err != nil || got != ref {
		t.Fatalf("expected exact card, got %+v (%v)", got, err)
	}
	for _, tc := range []struct {
		name   string
		change func(*creditIssue)
	}{
		{"open card", func(i *creditIssue) { i.State = "open" }},
		{"different issuer", func(i *creditIssue) { i.Author = &creditAccount{Login: "github-actions[bot]", ID: 1, Type: "Bot"} }},
		{"bot close", func(i *creditIssue) { i.ClosedBy = i.Author }},
		{"wrong human id", func(i *creditIssue) { i.ClosedBy = &creditAccount{Login: rlHuman, ID: 2345, Type: "User"} }},
		{"missing label", func(i *creditIssue) { i.Labels = nil }},
		{"ambiguous marker", func(i *creditIssue) { i.Body += "\n" + verifyMarker("vf/02") }},
		{"missing route", func(i *creditIssue) { i.Body = verifyMarker("vf/01") }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			i := valid
			tc.change(&i)
			if _, err := checkClosedAdjudicatedCard(&i, "vf/01", closer); err == nil {
				t.Fatal("expected refusal")
			}
		})
	}
}

func TestAcceptanceWorkflowAuthorityStopsOutsideCI(t *testing.T) {
	fx := newCreditRepo(t)
	t.Setenv("GITHUB_ACTIONS", "false")
	var out, stderr bytes.Buffer
	got := runAcceptanceRoute([]string{"close", "--root", fx.root, "--brief", "demo/01", "--card", "7", "--acceptance-forge"}, &out, &stderr)
	if got != 1 || out.Len() != 0 || !strings.Contains(stderr.String(), "explicit CI workflow token") {
		t.Fatalf("exit=%d out=%q err=%q", got, &out, &stderr)
	}
}
