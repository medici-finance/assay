package deskkit

import (
	"errors"
	"reflect"
	"testing"
)

// authorstrailer_test.go — #1339: `Authors:` is its own link kind, and no reader of the DELIVERY
// edge (`Brief:`) matches it. Stream slugs are synthetic (example-*).

func TestParseTrailersAuthors(t *testing.T) {
	trs, err := ParseTrailers([]byte("Authors briefs.\n\nAuthors: example-port/11, example-port/12\n"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(trs) != 1 || trs[0].Kind != TrailerAuthors || trs[0].Value != "example-port/11, example-port/12" || trs[0].Line != 3 {
		t.Fatalf("got %+v", trs)
	}
}

func TestParseTrailersAuthorsMultiplicity(t *testing.T) {
	_, err := ParseTrailers([]byte("Authors: a/01\nAuthors: a/02\n"))
	var dup *ErrTrailerDuplicate
	if !errors.As(err, &dup) || dup.Kind != TrailerAuthors {
		t.Fatalf("second Authors: want ErrTrailerDuplicate{authors}, got %v", err)
	}
	for _, body := range []string{"Authors: a/01\nBrief: a/01\n", "Issue: #7\nAuthors: a/01\n", "Brief: a/01\nIssue: #7\nAuthors: a/02\n"} {
		_, err := ParseTrailers([]byte(body))
		var mixed *ErrTrailerMixed
		if !errors.As(err, &mixed) || !mixed.Has(TrailerAuthors) {
			t.Fatalf("%q: want ErrTrailerMixed carrying Authors, got %v", body, err)
		}
	}
	// Brief: + Issue: with no Authors: keeps its original error type.
	_, err = ParseTrailers([]byte("Brief: a/01\nIssue: #7\n"))
	var both *ErrTrailerBoth
	if !errors.As(err, &both) {
		t.Fatalf("Brief:+Issue: want ErrTrailerBoth, got %v", err)
	}
}

func TestSplitAuthorsTrailer(t *testing.T) {
	ids, err := SplitAuthorsTrailer("Example-Port/11, example-port:12 cell:repo:example-port:13")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := []string{"example-port/11", "example-port/12", "example-port/13"}; !reflect.DeepEqual(ids, want) {
		t.Fatalf("got %v, want %v", ids, want)
	}
	for _, bad := range []string{"", " , ", "example-port", "example-port/xx", "a/01, a/01"} {
		if _, err := SplitAuthorsTrailer(bad); err == nil {
			t.Errorf("SplitAuthorsTrailer(%q) accepted, want an error", bad)
		}
	}
}

// TestAuthorsPRDoesNotRepresentBrief is the reader-side fail-first proof: a MERGED PR that carries
// `Authors:` for a brief must not represent (deliver) it. Before `Authors:` existed the only way to
// name the brief was `Brief:`, and that PR then represented it; with the new kind no reader keyed
// on TrailerBrief matches, so the brief stays dispatchable while a real `Brief:` delivery still
// represents its own brief.
func TestAuthorsPRDoesNotRepresentBrief(t *testing.T) {
	prs := []PRRef{
		{Number: 1439, State: "MERGED", Body: "Authors briefs 11-12.\n\nAuthors: example-port/11, example-port/12"},
		{Number: 1500, State: "MERGED", Body: "Implements 12.\n\nBrief: example-port/12"},
	}
	m := RepresentedBriefPRs(prs)
	if rp, ok := m["example-port/11"]; ok {
		t.Fatalf("an Authors: PR represents example-port/11 as %+v — the brief it only authored would never dispatch", rp)
	}
	if rp, ok := m["example-port/12"]; !ok || rp.Number != 1500 || !rp.Merged {
		t.Fatalf("the real Brief: delivery must still represent example-port/12; got %+v (present=%v)", rp, ok)
	}
	if bri := BriefRiskFromBody("example-org/example-repo", prs[0].Body); bri.RiskClassed || bri.OwningBrief != "" {
		t.Fatalf("an Authors: body declares no delivered brief, so it makes no brief risk claim; got %+v", bri)
	}
}

// TestRepresentingPRsByBriefKeepsEveryPR: the multi-valued reduction keeps every representing PR in
// list order (so a caller that sets an authoring PR aside still sees the delivery behind it), and
// RepresentedBriefPRs is exactly its first-of-each reduction.
func TestRepresentingPRsByBriefKeepsEveryPR(t *testing.T) {
	prs := []PRRef{
		{Number: 10, State: "MERGED", Body: "Brief: example-port/11"},
		{Number: 11, State: "CLOSED", Body: "Brief: example-port/11"},
		{Number: 12, State: "OPEN", Body: "Brief: example-port:11"},
	}
	all := RepresentingPRsByBrief(prs)
	want := []RepresentedPR{{Number: 10, Merged: true}, {Number: 12, Merged: false}}
	if !reflect.DeepEqual(all["example-port/11"], want) {
		t.Fatalf("got %+v, want %+v", all["example-port/11"], want)
	}
	if first := RepresentedBriefPRs(prs)["example-port/11"]; first != want[0] {
		t.Fatalf("RepresentedBriefPRs must be the first-of-each reduction; got %+v", first)
	}
}

// TestBriefImplicatedInMixedTrailers: a mixed set that includes Brief: still fails closed in the
// brief-risk term; a mix of Authors: and Issue: declares no delivered brief.
func TestBriefImplicatedInMixedTrailers(t *testing.T) {
	_, err := ParseTrailers([]byte("Authors: a/01\nBrief: a/01\n"))
	if !briefImplicatedInTrailerError(err) {
		t.Fatalf("Authors:+Brief: must implicate the Brief: trailer")
	}
	_, err = ParseTrailers([]byte("Authors: a/01\nIssue: #7\n"))
	if briefImplicatedInTrailerError(err) {
		t.Fatalf("Authors:+Issue: declares no delivered brief and must not implicate one")
	}
}
