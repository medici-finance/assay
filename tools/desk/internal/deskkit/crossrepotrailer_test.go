package deskkit

// crossrepotrailer_test.go — the `Cross-repo-brief:` link kind (medici-finance/assay#2748): a PR
// whose brief is homed in ANOTHER repo names it as `<alias>:<stream>/<NN>`, never as a `Brief:`
// that would resolve against this repo's own board. Synthetic example-* fixtures only.

import (
	"errors"
	"testing"
)

// regression: #2748
// TestCrossRepoTrailerParses — the new line is its own kind with the explicit grammar, and a
// `Brief:` line keeps its kind and meaning, alias-prefixed or not.
func TestCrossRepoTrailerParses(t *testing.T) {
	trs, err := ParseTrailers([]byte("body\n\nCross-repo-brief: example-trk:example-stream/05\n"))
	if err != nil || len(trs) != 1 || trs[0].Kind != TrailerCrossRepoBrief ||
		trs[0].Value != "example-trk:example-stream/05" || trs[0].Line != 3 {
		t.Fatalf("ParseTrailers = %+v, %v", trs, err)
	}
	alias, stream, nn, ok := SplitCrossRepoBriefTrailer(trs[0].Value)
	if !ok || alias != "example-trk" || stream != "example-stream" || nn != "05" {
		t.Fatalf("Split = (%q,%q,%q,%v)", alias, stream, nn, ok)
	}
	for _, bad := range []string{
		"example-stream/05",                // no alias: that is a Brief: value
		"example-trk:example-stream:05",    // colon form: not the item-key grammar
		"Example:example-stream/05",        // alias grammar is lowercase
		"example-trk:example-stream/0x",    // NN numeric
		"example-trk:example-stream/05/x",  // one slash
		"a:b:example-stream/05",            // exactly one alias
		"example-trk:/05",                  // empty stream
		"example-trk:example-stream/05 #c", // no trailing prose
	} {
		if _, _, _, ok := SplitCrossRepoBriefTrailer(bad); ok {
			t.Errorf("SplitCrossRepoBriefTrailer(%q) accepted a malformed value", bad)
		}
	}
	trs, err = ParseTrailers([]byte("Brief: example-trk:example-stream:05\n"))
	if err != nil || len(trs) != 1 || trs[0].Kind != TrailerBrief {
		t.Fatalf("a Brief: line must stay TrailerBrief: %+v, %v", trs, err)
	}
}

// regression: #2748
// TestCrossRepoTrailerAlone — one link per body: a second Cross-repo-brief: is a duplicate, and one
// beside a Brief:, Issue: or Authors: line is a mixed set that implicates a brief.
func TestCrossRepoTrailerAlone(t *testing.T) {
	const xr = "Cross-repo-brief: example-trk:example-stream/05\n"
	var dup *ErrTrailerDuplicate
	if _, err := ParseTrailers([]byte(xr + xr)); !errors.As(err, &dup) || dup.Kind != TrailerCrossRepoBrief {
		t.Errorf("two Cross-repo-brief: lines = %v, want a duplicate", err)
	}
	for _, other := range []string{"Brief: example-stream/05\n", "Issue: #7\n", "Authors: example-stream/05\n"} {
		_, err := ParseTrailers([]byte(xr + other))
		var mixed *ErrTrailerMixed
		if !errors.As(err, &mixed) || !mixed.Has(TrailerCrossRepoBrief) {
			t.Errorf("Cross-repo-brief: + %q = %v, want a mixed-trailer error", other, err)
			continue
		}
		if !briefImplicatedInTrailerError(err) {
			t.Errorf("Cross-repo-brief: + %q must implicate a brief (fail closed)", other)
		}
	}
}

// regression: #2748
// TestCrossRepoRiskFailsClosed — the brief named by Cross-repo-brief: is not in this repo's tree,
// so its gate and risk flags cannot be read here: the PR is risk-classed UNVERIFIABLE, never the
// trailer-absent "no claim" answer that would switch the brief-declared risk term off.
func TestCrossRepoRiskFailsClosed(t *testing.T) {
	br := BriefRiskFromBody("medici-finance/assay", "x\nCross-repo-brief: example-trk:example-stream/05\n")
	if !br.RiskClassed || br.Resolved || br.OwningBrief != "example-trk:example-stream/05" {
		t.Fatalf("BriefRiskFromBody(Cross-repo-brief) = %+v, want risk-classed unverifiable", br)
	}
	br = BriefRiskFromBody("medici-finance/assay", "Cross-repo-brief: example-trk:example-stream/05\nIssue: #7\n")
	if !br.RiskClassed {
		t.Fatalf("a malformed set carrying Cross-repo-brief: = %+v, want fail closed", br)
	}
}

// regression: #2748
// TestCrossRepoClaimKey — the PR-side claim key mirrors the key deskdispatch takes the cross-repo
// claim under: the TRACKING alias, then the brief id with "/" as "--".
func TestCrossRepoClaimKey(t *testing.T) {
	k, ok := ClaimKeyForPR("medici-finance/assay", "x\nCross-repo-brief: example-trk:example-stream/05\n")
	if !ok || k != "example-trk--example-stream--05" {
		t.Fatalf("ClaimKeyForPR = (%q, %v), want example-trk--example-stream--05", k, ok)
	}
	if _, ok := ClaimKeyForPR("medici-finance/assay", "Cross-repo-brief: example-stream/05\n"); ok {
		t.Error("a malformed Cross-repo-brief: value must derive no key")
	}
}
