package main

import (
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// trace_test.go — the dedupe-search outage says WHAT failed, at the forge seam.
//
// deskfile fails CLOSED when its dedupe search cannot be answered: minting a possibly-duplicate
// issue is the expensive direction, so an unanswered search is exit 6 rather than a guess at
// absence. Since the write-verbs-C migration the search goes through Forge.SearchIssues, and the
// backend carries the API status on a *ForgeAPIError-wrapped could-not-check (the 401/403/429
// tiers a reader has to tell apart are pinned in deskkit's TestForgeGitlabTierErrors and the
// github error-mapping goldens, and the backends StripControl remote-authored text at ingest).
// What this file pins is the deskfile-SIDE property: dedupeSearch PROPAGATES that diagnosis
// unswallowed and the caller's refusal stays exit-6 fail-closed.

var traceRepo = deskkit.ForgeRepo{Owner: "medici-finance", Name: "assay"}

// TestDedupeSearchPropagatesTheForgeDiagnosis is the per-status pin: whatever the backend named
// as the failure class reaches the operator through dedupeSearch and the exit-6 refusal over it.
func TestDedupeSearchPropagatesTheForgeDiagnosis(t *testing.T) {
	cases := []struct {
		name, forgeMsg, want string
	}{
		{"forbidden", "could-not-check: GET /search/issues — permission or tier gate (HTTP 403)", "HTTP 403"},
		{"rate limited", "could-not-check: GET /search/issues — rate limited (HTTP 429) by the instance", "HTTP 429"},
		{"bad credentials", "could-not-check: GET /search/issues — credential rejected (HTTP 401)", "HTTP 401"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := &dfForge{fr: traceRepo, searchErr: deskkit.Unverifiable(c.forgeMsg, nil)}
			_, err := dedupeSearch(f, traceRepo, "a title with several scorable words")
			if err == nil {
				t.Fatal("a failed forge search must return an error")
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Errorf("the search failure does not name %s — an operator cannot tell this from the "+
					"other two failure classes:\n%s", c.want, err.Error())
			}
			// The exit-6 refusal the caller builds over it must carry the status too.
			refusal := deskkit.Unverifiable(
				"dedupe search failed — refuse rather than mint a possible duplicate "+
					"(override with --force-new --reason)", err)
			if !strings.Contains(refusal.Error(), c.want) {
				t.Errorf("the exit-6 refusal swallowed the API status:\n%s", refusal.Error())
			}
			if deskkit.ExitCodeOf(refusal) != deskkit.ExitUnverifiable {
				t.Errorf("the fail-closed verdict changed: got %d", deskkit.ExitCodeOf(refusal))
			}
		})
	}
}

// TestDedupeSearchControlBytesStrippedByBackend — remote-authored text (issue titles on public
// repos) is stripped at the forge backend's ingest, so a diagnosis that survives to dedupeSearch
// carries no terminal-active bytes. The fake stands in for that already-stripped error; the
// property this pins is that dedupeSearch does not RE-INTRODUCE control bytes.
func TestDedupeSearchControlBytesStrippedByBackend(t *testing.T) {
	f := &dfForge{fr: traceRepo, searchErr: deskkit.Unverifiable(
		deskkit.StripControl("could-not-check: could not resolve \x1b[31mtitle\x07 (HTTP 422)"), nil)}
	_, err := dedupeSearch(f, traceRepo, "a title with several scorable words")
	if err == nil {
		t.Fatal("expected the stubbed failure")
	}
	if strings.ContainsAny(err.Error(), "\x1b\x07") {
		t.Errorf("terminal-active bytes reached the message: %q", err.Error())
	}
	if !strings.Contains(err.Error(), "HTTP 422") {
		t.Errorf("stripping ate the diagnosis: %q", err.Error())
	}
}
