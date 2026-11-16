package main

import (
	"errors"
	"testing"
)

// A PR that only revises a brief (its own file, its stream board, changelog fragments) is not
// the brief's delivery for the reconcile. Anything else it writes keeps it a witness.
// regression: #6091
func TestBriefRevisionOnly(t *testing.T) {
	const own = "docs/streams/example-port/brief-11-portable-pollers.md"
	m := func(p string) prFile { return prFile{Filename: p, Status: "modified"} }
	for _, c := range []struct {
		why   string
		id    string
		files []prFile
		want  bool
	}{
		{"own brief only modified", "example-port/11", []prFile{m(own)}, true},
		{"colon-form id", "example-repo:example-port:11", []prFile{m(own)}, true},
		{"own brief, own board, fragment", "example-port/11", []prFile{m(own), m("docs/streams/example-port/README.md"), {Filename: "changelog/x.md", Status: "added"}}, true},
		{"own brief renamed within itself", "example-port/11", []prFile{{Filename: own, Status: "renamed", PreviousFilename: "docs/streams/example-port/brief-11-old.md"}}, true},
		{"board only, brief untouched", "example-port/11", []prFile{m("docs/streams/example-port/README.md")}, false},
		{"another brief too", "example-port/11", []prFile{m(own), {Filename: "docs/streams/example-port/brief-12-next.md", Status: "added"}}, false},
		{"another stream board", "example-port/11", []prFile{m(own), m("docs/streams/example-other/README.md")}, false},
		{"a stream deliverable doc", "example-port/11", []prFile{m(own), {Filename: "docs/streams/example-port/audit.md", Status: "added"}}, false},
		{"code", "example-port/11", []prFile{m(own), m("tools/x.go")}, false},
		{"rename of code into the brief", "example-port/11", []prFile{{Filename: own, Status: "renamed", PreviousFilename: "tools/x.go"}}, false},
		{"only a different brief", "example-port/12", []prFile{m(own)}, false},
		{"STATUS.md", "example-port/11", []prFile{m(own), m("STATUS.md")}, false},
		{"empty list", "example-port/11", nil, false},
		{"unparseable id", "not-a-brief", []prFile{m(own)}, false},
	} {
		if got := briefRevisionOnly(c.id, c.files); got != c.want {
			t.Errorf("%s: briefRevisionOnly(%q) = %v, want %v", c.why, c.id, got, c.want)
		}
	}
}

// Both reconcile readers that make a PR a witness — the Brief: trailer fold and the --backfill
// match — set a brief-revision PR (#22) aside, with no shape read, and keep a code PR (#21).
// Backfill tries the higher-numbered candidate first, so #22 must be walked past to reach #21.
// regression: #6091
func TestRevisionNotWitness(t *testing.T) {
	cl := newAuthoringClassifier(nil, "o/r", nil)
	cl.cache[21] = filesResult{files: []prFile{{Filename: "docs/streams/fx/brief-01-example.md", Status: "modified"}, {Filename: "internal/fx/fx.go", Status: "modified"}}}
	cl.cache[22] = filesResult{files: []prFile{{Filename: "docs/streams/fx/brief-01-example.md", Status: "modified"}}}
	// A shape read would answer could-not-check: the revision rule must decide first.
	cl.shapes[22] = shapeResult{err: errors.New("no shape read is expected for PR #22")}

	prs := []PRRecord{
		{BriefRef: "fx/01", Number: 21, State: prMerged, MergeSHA: "aaaaaaa"},
		{BriefRef: "fx/01", Number: 22, State: prMerged, MergeSHA: "bbbbbbb"},
	}
	kept, undecided, unread := dropAuthoringPRs(cl, prs, []BriefIdent{{ID: "fx/01"}}, nil)
	if len(kept) != 1 || kept[0].Number != 21 || len(undecided) != 0 || len(unread) != 0 {
		t.Fatalf("trailer fold: kept=%+v undecided=%v unread=%v, want only PR #21 kept", kept, undecided, unread)
	}

	code := ghPull{Number: 21, MergedAt: "2026-10-01T00:00:00Z"}
	code.Head.Ref = "feat/fx-01"
	revision := ghPull{Number: 22, MergedAt: "2026-10-02T00:00:00Z"}
	revision.Head.Ref = "docs/fx-01-revise"
	match, ok, why := matchBackfillPRChecked([]ghPull{code, revision}, "fx", "01", cl.nonDelivery)
	if !ok || match.Number != 21 || why != "" {
		t.Fatalf("backfill: match=#%d ok=%v unread=%q, want PR #21", match.Number, ok, why)
	}
}
