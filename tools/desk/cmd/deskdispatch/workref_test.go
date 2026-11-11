package main

import (
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"testing"
)

// regression: #5704
func TestClaimKeySubjectSpelling(t *testing.T) {
	for _, value := range []string{"stream:b-08@2", "cell:repo:stream:b-08@2", "cell:repo:stream:08", "repo:stream/08"} {
		if got := claimKeyFor(value, "owner/repo"); got != value {
			t.Fatalf("subject spelling erased into physical key: %s", got)
		}
	}
	if got := claimKeyFor("repo--stream--08", "owner/repo"); got != "repo--stream--08" {
		t.Fatal("legacy transport key changed")
	}
}

// regression: #5704
func TestStrictCrossRepoMatch(t *testing.T) {
	good := deskkit.PRRef{Number: 1, State: "OPEN", Body: "Cross-repo-brief: repo:stream/08"}
	if _, ok := crossRepoRepresents(good, "repo:stream/08"); !ok {
		t.Fatal("strict cross-repo link lost")
	}
	// The legacy profile itself is unchanged: accepted raw stream spelling
	// remains accepted and matches itself, without case-folding a refused alias.
	legacy := good
	legacy.Body = "Cross-repo-brief: repo:STREAM/08"
	if _, ok := crossRepoRepresents(legacy, "repo:STREAM/08"); !ok {
		t.Fatal("accepted legacy spelling lost")
	}
	bad := good
	bad.Body = "Cross-repo-brief: REPO:stream/08"
	if _, ok := crossRepoRepresents(bad, "REPO:stream/08"); ok {
		t.Fatal("refused alias matched itself")
	}
	if _, ok := crossRepoRepresents(bad, "repo:stream/08"); ok {
		t.Fatal("case-fold admitted strict-refused alias")
	}
}
