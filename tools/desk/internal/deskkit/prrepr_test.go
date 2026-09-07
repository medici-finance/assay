package deskkit

import "testing"

// The stream slugs here are synthetic (example-*): a shipped test naming a real withheld stream
// would publish a map to it.

func TestRepresentedBriefsKeysOnTheTrailerNotTheBranch(t *testing.T) {
	prs := []PRRef{
		{Number: 373, State: "OPEN", Body: "the work\n\nBrief: example-a/00"},
		{Number: 372, State: "MERGED", Body: "Brief: example-b/08"},
		{Number: 401, State: "CLOSED", Body: "Brief: example-abandoned/01"}, // abandoned — no represent
		{Number: 500, State: "OPEN", Body: "no trailer here"},               // names no brief
		{Number: 501, State: "OPEN", Body: "Issue: #42"},                    // issue-only, no brief
	}
	m := RepresentedBriefs(prs)

	if got := m["example-a/00"]; got != 373 {
		t.Errorf("open PR by Brief trailer: got %d, want 373", got)
	}
	if got := m["example-b/08"]; got != 372 {
		t.Errorf("MERGED PR represents its brief: got %d, want 372", got)
	}
	if _, ok := m["example-abandoned/01"]; ok {
		t.Error("a CLOSED-unmerged PR must not represent its brief — the row is dispatchable again")
	}
	if len(m) != 2 {
		t.Errorf("only the OPEN + MERGED, trailer-bearing PRs count: %v", m)
	}
}

func TestBriefRepresentedPR(t *testing.T) {
	prs := []PRRef{{Number: 1903, State: "OPEN", Body: "Brief: example-two-part/08"}}
	if n, ok := BriefRepresentedPR("example-two-part/08", prs); !ok || n != 1903 {
		t.Errorf("BriefRepresentedPR = (%d,%v), want (1903,true)", n, ok)
	}
	// Case-insensitive on the trimmed id.
	if n, ok := BriefRepresentedPR("  Example-Two-Part/08 ", prs); !ok || n != 1903 {
		t.Errorf("BriefRepresentedPR (mixed case/space) = (%d,%v), want (1903,true)", n, ok)
	}
	if _, ok := BriefRepresentedPR("example-a/00", prs); ok {
		t.Error("an unrepresented brief must not match")
	}
	if _, ok := BriefRepresentedPR("", prs); ok {
		t.Error("an empty brief id must never match")
	}
}

// TestRepresentedBriefsCanonicalizesColonForm is the regression guard for the phantom
// double-dispatch a colon-form `Brief:` trailer could still cause. deskpr accepts BOTH
// `Brief: <stream>/<NN>` and `Brief: <stream>:<NN>`, but the phantom check and the
// fanoutloop exclusion look the brief up by its slash-form id (`<stream>/<NN>`). A PR
// authored with the colon form must therefore key on the slash-form id, or its real
// OPEN/MERGED PR is missed and a fresh worker is dispatched over it. This asserts every
// consumer (the map, the number lookup, the set) matches the slash-form id against a
// colon-form PR body.
func TestRepresentedBriefsCanonicalizesColonForm(t *testing.T) {
	prs := []PRRef{
		{Number: 610, State: "OPEN", Body: "the work\n\nBrief: example-colon:07"},
		{Number: 611, State: "MERGED", Body: "Brief: repo:example-prefixed:09"},
	}

	m := RepresentedBriefs(prs)
	if got := m["example-colon/07"]; got != 610 {
		t.Errorf("colon-form `Brief: example-colon:07` must key on the slash-form id example-colon/07: got %d, want 610 (map=%v)", got, m)
	}
	if got := m["example-prefixed/09"]; got != 611 {
		t.Errorf("repo-prefixed colon form must reduce to its slash-form id example-prefixed/09: got %d, want 611 (map=%v)", got, m)
	}

	if n, ok := BriefRepresentedPR("example-colon/07", prs); !ok || n != 610 {
		t.Errorf("BriefRepresentedPR(example-colon/07) = (%d,%v), want (610,true) — a colon-form PR must exclude its slash-form brief id", n, ok)
	}

	if set := RepresentedBriefSet(prs); !set["example-colon/07"] {
		t.Errorf("RepresentedBriefSet missing the slash-form id of a colon-form PR: %v", set)
	}
}

func TestParsePRList(t *testing.T) {
	prs, err := ParsePRList([]byte(`[{"number":7,"state":"OPEN","body":"Brief: s/01"}]`))
	if err != nil {
		t.Fatalf("ParsePRList: %v", err)
	}
	if len(prs) != 1 || prs[0].Number != 7 || prs[0].State != "OPEN" {
		t.Fatalf("unexpected parse: %+v", prs)
	}
	// Empty input is the empty list, not an error (the transport owns the could-not-read path).
	if got, err := ParsePRList([]byte("  \n")); err != nil || got != nil {
		t.Errorf("empty input: got (%v,%v), want (nil,nil)", got, err)
	}
	if _, err := ParsePRList([]byte("{not json")); err == nil {
		t.Error("malformed JSON must error, not silently empty")
	}
}

func TestRepresentedBriefSet(t *testing.T) {
	set := RepresentedBriefSet([]PRRef{
		{Number: 1, State: "OPEN", Body: "Brief: a/01"},
		{Number: 2, State: "MERGED", Body: "Brief: b/02"},
		{Number: 3, State: "CLOSED", Body: "Brief: c/03"},
	})
	if !set["a/01"] || !set["b/02"] {
		t.Errorf("open+merged briefs missing from the set: %v", set)
	}
	if set["c/03"] {
		t.Error("closed brief must not be in the represented set")
	}
}
