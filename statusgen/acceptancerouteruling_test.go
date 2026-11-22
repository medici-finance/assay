package main

import (
	"strings"
	"testing"
)

func TestAcceptanceAdjudicationRequiresExactHumanRuling(t *testing.T) {
	m := acceptanceManifest{Source: acceptanceSource{
		Repository: rlRepo,
		BriefPath:  "docs/streams/demo/brief-01-demo.md",
		Revision:   strings.Repeat("a", 40),
	}}
	payload, err := acceptanceAdjudicationText(m, rlIssue)
	if err != nil {
		t.Fatal(err)
	}
	link := rlURL(rlRepo, rlIssue, rlComment)
	marker := acceptanceAdjudicationMarker(m.Source.BriefPath)
	for _, tc := range []struct {
		name, actor, typ, body, updated, issueBody, ruling string
		pass                                               bool
	}{
		{"exact", rlHuman, "User", payload, rlPosted, marker, link, true},
		{"generic approval", rlHuman, "User", "Approve acceptance-adjudication", rlPosted, marker, link, false},
		{"edited", rlHuman, "User", payload, "2026-09-01T10:00:01Z", marker, link, false},
		{"bot", rlHuman, "Bot", payload, rlPosted, marker, link, false},
		{"unmapped human", "example-unmapped", "User", payload, rlPosted, marker, link, false},
		{"other repository", rlHuman, "User", payload, rlPosted, marker, rlURL("example-org/other", rlIssue, rlComment), false},
		{"other issue", rlHuman, "User", payload, rlPosted, marker, rlURL(rlRepo, rlIssue+1, rlComment), false},
		{"wrong marker", rlHuman, "User", payload, rlPosted, acceptanceAdjudicationMarker("docs/streams/demo/brief-02-demo.md"), link, false},
		{"different manifest", rlHuman, "User", payload + "0", rlPosted, marker, link, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			forge := defaultRLForge()
			forge.commentBody = rlCommentJSONWith(tc.actor, tc.typ, rlRepo, rlIssue, tc.body, rlPosted, tc.updated)
			forge.issueBody = rlIssueJSON(tc.issueBody)
			err := corroborateAcceptanceAdjudication(forge.client(t), m, rlIssue, tc.ruling, rlLogins)
			if (err == nil) != tc.pass {
				t.Fatalf("pass=%v, got %v", tc.pass, err)
			}
		})
	}
}
