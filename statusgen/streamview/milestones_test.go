package streamview

import (
	"errors"
	"strings"
	"testing"
)

func milestoneView() *StreamView {
	v := validView()
	v.Contract = Version2
	src := SourceRef{Repo: "example-org/repo-a", Path: "docs/streams/shared/README.md", Revision: testRev}
	v.Milestones = &Milestones{
		Provenance: Provenance{Availability: Available, Sources: []SourceRef{src}, ObservedAt: testNow},
		Coverage:   CoverageKnown,
		Items: []MilestoneItem{
			{
				// Zero work remaining is not acceptance.
				Ref: "cell:repo:shared:m-01@1", Title: "Ready for acceptance", Scope: []string{"cell:repo:shared:01"},
				Work:         MilestoneWork{State: WorkKnown},
				Acceptance:   MilestoneAcceptance{State: AcceptanceOutstanding},
				Availability: MilestoneUnknown,
				Holds:        []MilestoneHold{{Ref: "cell:repo:shared:m-01@1", Kind: "acceptance", State: "held", Reason: "acceptance outstanding"}},
				Evidence:     []EvidenceRef{{Kind: EvidencePath, Repo: "example-org/repo-a", Path: "docs/report.md"}},
			},
			{
				Ref: "cell:repo:shared:m-02@1", Title: "Work remains", Aliases: []string{"legacy-m2"},
				Scope:         []string{"cell:repo:shared:01", "cell:repo:shared:02"},
				Work:          MilestoneWork{State: WorkKnown, Remaining: 1, Pending: []string{"cell:repo:shared:02"}},
				Acceptance:    MilestoneAcceptance{State: AcceptanceNotDue},
				Availability:  MilestoneBlocked,
				Prerequisites: []string{"cell:repo:shared:m-01@1"},
				Holds:         []MilestoneHold{{Ref: "cell:repo:shared:02", Kind: "work", State: "held", Reason: "work remaining"}},
			},
			{
				Ref: "cell:repo:shared:m-03@1", Title: "Unread", Scope: []string{},
				Work:         MilestoneWork{State: WorkUnknown, Reason: "plan coverage is unknown"},
				Acceptance:   MilestoneAcceptance{State: AcceptanceCouldNotCheck, Reason: "no acceptance observation supplied"},
				Availability: MilestoneUnknown,
			},
		},
		Membership: []MilestoneMembership{{Brief: "cell:repo:shared:01", Milestones: []string{"cell:repo:shared:m-01@1", "cell:repo:shared:m-02@1"}}},
	}
	return v
}

// regression: milestone-support/11 — the v2 section must keep scope, work,
// acceptance, availability and holds distinct, and missing must never read as
// a value. Dropping a mandatory unknown check makes this test fail.
func TestMilestoneStreamView(t *testing.T) {
	v := milestoneView()
	enc, err := Encode(v)
	if err != nil {
		t.Fatalf("valid v2 view refused: %v", err)
	}
	dec, err := Decode(enc)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	again, err := Encode(dec)
	if err != nil || string(again) != string(enc) {
		t.Fatalf("round trip not byte-stable: %v", err)
	}
	it := dec.Milestones.Items[0]
	if it.Work.Remaining != 0 || it.Acceptance.State != AcceptanceOutstanding || it.Availability == MilestoneSupported {
		t.Errorf("zero work with acceptance outstanding must stay distinct from supported: %+v", it)
	}

	mutate := func(name string, f func(*StreamView), want string) {
		t.Helper()
		x := milestoneView()
		f(x)
		err := x.Validate()
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: want error containing %q, got %v", name, want, err)
		}
	}
	mutate("coverage unknown without unknowns", func(x *StreamView) { x.Milestones.Coverage = CoverageUnknown }, "no unknown is listed")
	mutate("coverage known with unknowns", func(x *StreamView) { x.Milestones.Unknowns = []string{"a gap"} }, "unknowns are listed")
	mutate("partial claims known", func(x *StreamView) {
		x.Milestones.Provenance.Availability = Partial
		x.Milestones.Provenance.Reason = "r"
	}, "partial section cannot claim known")
	mutate("unknown work carries a count", func(x *StreamView) { x.Milestones.Items[2].Work.Remaining = 2 }, "missing must never read as a value")
	mutate("unknown work without reason", func(x *StreamView) { x.Milestones.Items[2].Work.Reason = "" }, "unknown work gives no reason")
	mutate("could-not-check acceptance without reason", func(x *StreamView) { x.Milestones.Items[2].Acceptance.Reason = "" }, "gives no reason")
	mutate("outstanding with work remaining", func(x *StreamView) { x.Milestones.Items[1].Acceptance.State = AcceptanceOutstanding }, "requires known work with none remaining")
	mutate("not-due with no work remaining", func(x *StreamView) { x.Milestones.Items[0].Acceptance.State = AcceptanceNotDue }, "not-due requires")
	mutate("supported without acceptance", func(x *StreamView) {
		x.Milestones.Items[0].Availability = MilestoneSupported
		x.Milestones.Items[0].Holds = nil
	}, "supported requires accepted")
	mutate("blocked without a hold", func(x *StreamView) { x.Milestones.Items[1].Holds = nil }, "names no hold")
	mutate("alias collides with another ref", func(x *StreamView) { x.Milestones.Items[1].Aliases = []string{"cell:repo:shared:m-01@1"} }, "one milestone, one row")
	mutate("membership names a non-item", func(x *StreamView) { x.Milestones.Membership[0].Milestones = []string{"nope@1"} }, "not an item")
	mutate("bad evidence", func(x *StreamView) { x.Milestones.Items[0].Evidence[0].Path = "../escape" }, "evidence[0]")
	mutate("missing section in v2", func(x *StreamView) { x.Milestones = nil }, "mandatory in v2")
	mutate("section in v1", func(x *StreamView) { x.Contract = Version }, "v1 view cannot carry")
	mutate("not-assessed carries items", func(x *StreamView) {
		x.Milestones.Provenance = Provenance{Availability: NotAssessed, ObservedAt: testNow, Reason: "r"}
	}, "carries content")

	// A stream that did not opt in states not-assessed; it is valid and empty.
	none := milestoneView()
	none.Milestones = &Milestones{Provenance: Provenance{Availability: NotAssessed, ObservedAt: testNow, Reason: "stream did not opt in to milestone-v1"}}
	if err := none.Validate(); err != nil {
		t.Errorf("not-assessed section refused: %v", err)
	}

	// Old view compatibility: v1 decodes unchanged, and a consumer that needs
	// milestones refuses it by version rather than reading a missing section as empty.
	old, err := Encode(validView())
	if err != nil {
		t.Fatal(err)
	}
	if got, err := Decode(old); err != nil || got.Milestones != nil || got.Contract != Version {
		t.Fatalf("v1 view must decode unchanged: %v", err)
	}
	var uv *UnsupportedVersionError
	if _, err := Decode(old, Version2); !errors.As(err, &uv) {
		t.Errorf("a milestone consumer must refuse v1: %v", err)
	}
	if _, err := Decode([]byte(strings.Replace(string(enc), `"contract": "stream-view/v2"`, `"contract": "stream-view/v3"`, 1))); !errors.As(err, &uv) {
		t.Errorf("v3 must be unsupported: %v", err)
	}
	extra := strings.Replace(string(enc), `"coverage": "known"`, `"coverage": "known", "done": true`, 1)
	if _, err := Decode([]byte(extra)); err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Errorf("unknown field in the milestone section must be refused: %v", err)
	}
	if got, err := Negotiate([]string{Version, Version2}, SupportedVersions); err != nil || got != Version2 {
		t.Errorf("negotiation prefers v2: %q %v", got, err)
	}
}

type fakeResolver map[string]string

func (f fakeResolver) Blob(repo, rev, path string) (string, error) { return f[repo+"|"+path], nil }
func (f fakeResolver) Head(repo string) (string, error)            { return testRev, nil }

func TestMilestoneBindings(t *testing.T) {
	v := milestoneView()
	r := fakeResolver{
		"example-org/repo-a|docs/streams/shared/README.md": "b1",
		"example-org/repo-a|docs/report.md":                "b2",
	}
	if rep := CheckBindings(v, r); len(rep.Problems) != 0 {
		t.Fatalf("resolvable milestone bindings reported problems: %v", rep.Problems)
	}
	delete(r, "example-org/repo-a|docs/report.md")
	rep := CheckBindings(v, r)
	if !strings.Contains(strings.Join(rep.Problems, "\n"), "milestones.items[0].evidence[0]") {
		t.Fatalf("a missing milestone evidence path must be a problem: %v", rep.Problems)
	}
}
