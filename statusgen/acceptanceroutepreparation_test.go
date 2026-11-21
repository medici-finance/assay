package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAcceptancePreparationRequiresPinnedVerifierAndUneditedBody(t *testing.T) {
	creditFakeDeskread(t)
	t.Setenv("CREDIT_FAKE_CI", "1")
	scanWithRoster(t, rosterWith(map[string]string{
		scanEnvTrustedBotSlugs: "verifier=assay-verifier-app:300000005,worker=assay-worker-app:300000006",
	}))
	m := acceptanceManifest{Source: acceptanceSource{
		Repository: creditRepo, BriefPath: creditBriefRel, Revision: strings.Repeat("a", 40),
	}}
	want, err := acceptancePreparationBody(m)
	if err != nil {
		t.Fatal(err)
	}
	yes := true
	for _, tc := range []struct {
		name   string
		author creditAccount
		tail   string
		trust  creditTrust
		pass   bool
	}{
		{"exact", creditAccount{"assay-verifier-app[bot]", 300000005, "Bot"}, "", creditTrust{"", true, &yes}, true},
		{"worker", creditAccount{"assay-worker-app[bot]", 300000006, "Bot"}, "", creditTrust{"", true, &yes}, false},
		{"wrong id", creditAccount{"assay-verifier-app[bot]", 300000006, "Bot"}, "", creditTrust{"", true, &yes}, false},
		{"edited", creditAccount{"assay-verifier-app[bot]", 300000005, "Bot"}, "", creditTrust{"edited", true, &yes}, false},
		{"history unknown", creditAccount{"assay-verifier-app[bot]", 300000005, "Bot"}, "", creditTrust{"", true, nil}, false},
		{"changed manifest", creditAccount{"assay-verifier-app[bot]", 300000005, "Bot"}, " changed", creditTrust{"", true, &yes}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for kind, payload := range map[string]map[string]any{
				"issue": {"issue": creditIssue{Title: "Acceptance preparation: " + creditBriefRel, Body: want + tc.tail, Author: &tc.author}},
				"trust": {"trust": tc.trust},
			} {
				payload["repo"], payload["number"] = creditRepo, 41
				raw, err := json.Marshal(map[string]any{"schema": deskreadSchema, "kind": kind, "items": []any{payload}})
				if err != nil {
					t.Fatal(err)
				}
				putFile(t, filepath.Join(os.Getenv("CREDIT_FAKE_DIR"), kind+"-41.json"), string(raw))
			}
			if err := verifyAcceptancePreparation(m, 41, true); (err == nil) != tc.pass {
				t.Fatalf("pass=%v, got %v", tc.pass, err)
			}
		})
	}
}
