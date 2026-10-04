package streamview

import (
	"errors"
	"strings"
	"testing"
	"time"
)

var testNow = time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)

const testRev = "0123456789abcdef0123456789abcdef01234567"

// validView is a minimal contract-valid view; each test corrupts one field.
func validView() *StreamView {
	src := SourceRef{Repo: "example-org/repo-a", Path: "docs/streams/shared/README.md", Revision: testRev}
	prov := func(a Availability, reason string, srcs ...SourceRef) Provenance {
		return Provenance{Availability: a, Sources: srcs, ObservedAt: testNow, Reason: reason}
	}
	return &StreamView{
		Contract:    Version,
		Identity:    Identity{Key: Key{Repo: "example-org/repo-a", Slug: "shared"}, DisplayName: "Shared"},
		GeneratedAt: testNow,
		Mission: Mission{
			Provenance: prov(Available, "", src),
			Origin:     OriginAuthored,
			Outcome:    "A reader can tell what this is for.",
			Success: []SuccessCriterion{{
				Criterion: "The report exists.",
				Evidence:  []EvidenceRef{{Kind: EvidencePath, Repo: "example-org/repo-a", Path: "docs/report.md"}},
			}},
		},
		CurrentState: CurrentState{Provenance: prov(Available, "", src), Status: "active"},
		Changes:      Changes{Provenance: prov(CouldNotCheck, "no history input"), Window: TrailingWindow(testNow)},
		NeedsYou:     NeedsYou{Provenance: prov(CouldNotCheck, "no decision source")},
		Evidence:     Evidence{Provenance: prov(NotAssessed, "no producer")},
		Outcomes:     Outcomes{Provenance: prov(NotAssessed, "no producer")},
	}
}

func TestNegotiate(t *testing.T) {
	got, err := Negotiate([]string{"stream-view/v1", "stream-view/v2"}, []string{"stream-view/v2", "stream-view/v1"})
	if err != nil || got != "stream-view/v2" {
		t.Fatalf("consumer preference order must win: got %q, %v", got, err)
	}
	got, err = Negotiate([]string{"stream-view/v1"}, []string{"stream-view/v3", "stream-view/v1"})
	if err != nil || got != "stream-view/v1" {
		t.Fatalf("first common version: got %q, %v", got, err)
	}
	if got, err := Negotiate([]string{"stream-view/v1"}, []string{"stream-view/v2"}); err == nil {
		t.Fatalf("no common version must be an error, got %q", got)
	}
}

func TestEncodeDecodeRoundTrip(t *testing.T) {
	v := validView()
	enc, err := Encode(v)
	if err != nil {
		t.Fatal(err)
	}
	dec, err := Decode(enc)
	if err != nil {
		t.Fatal(err)
	}
	again, err := Encode(dec)
	if err != nil || string(again) != string(enc) {
		t.Fatalf("round trip not byte-stable: %v", err)
	}
}

func TestDecodeVersionHandling(t *testing.T) {
	enc, err := Encode(validView())
	if err != nil {
		t.Fatal(err)
	}
	s := string(enc)
	var uv *UnsupportedVersionError

	future := strings.Replace(s, `"contract": "stream-view/v1"`, `"contract": "stream-view/v2"`, 1)
	if _, err := Decode([]byte(future)); !errors.As(err, &uv) || uv.Got != "stream-view/v2" {
		t.Errorf("unsupported version: got %v", err)
	}
	missing := strings.Replace(s, `"contract": "stream-view/v1",`, "", 1)
	if _, err := Decode([]byte(missing)); !errors.As(err, &uv) || uv.Got != "" {
		t.Errorf("missing version: got %v", err)
	}
	if _, err := Decode(enc, "stream-view/v2"); !errors.As(err, &uv) {
		t.Errorf("consumer that does not accept v1 must refuse it: got %v", err)
	}
	if _, err := Decode(enc, "stream-view/v1", "stream-view/v2"); err != nil {
		t.Errorf("accepted version refused: %v", err)
	}
	unknown := strings.Replace(s, `"contract": "stream-view/v1",`, `"contract": "stream-view/v1", "readiness": 0.9,`, 1)
	if _, err := Decode([]byte(unknown)); err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Errorf("unknown field within v1 must be refused: got %v", err)
	}
	if _, err := Decode(append(append([]byte{}, enc...), []byte(" {}")...)); err == nil {
		t.Errorf("trailing data must be refused")
	}
}

func TestValidateRules(t *testing.T) {
	cases := []struct {
		name string
		mut  func(v *StreamView)
		want string
	}{
		{"since-last-visit window", func(v *StreamView) { v.Changes.Window.Label = "since your last visit" }, "window label"},
		{"window kind", func(v *StreamView) { v.Changes.Window.Kind = "since-last-visit" }, "window kind"},
		{"window span", func(v *StreamView) { v.Changes.Window.Start = v.Changes.Window.End.Add(-48 * time.Hour) }, "window spans"},
		{"could-not-check with content", func(v *StreamView) {
			v.NeedsYou.Decisions = []Decision{{ID: "d1"}}
		}, "missing must never read as a value"},
		{"could-not-check without reason", func(v *StreamView) { v.Changes.Provenance.Reason = "" }, "gives no reason"},
		{"available without source", func(v *StreamView) { v.CurrentState.Provenance.Sources = nil }, "names no source revision"},
		{"partial without reason", func(v *StreamView) { v.CurrentState.Provenance.Availability = Partial }, "partial section gives no reason"},
		{"abbreviated revision", func(v *StreamView) { v.Mission.Provenance.Sources[0].Revision = "0123456" }, "revision"},
		{"bad availability", func(v *StreamView) { v.Outcomes.Provenance.Availability = "unknown" }, "availability"},
		{"legacy-prose with success", func(v *StreamView) { v.Mission.Origin = OriginLegacyProse }, "exist only when authored"},
		{"absent with content", func(v *StreamView) { v.Mission.Origin = OriginAbsent }, "absent mission carries content"},
		{"authored without outcome", func(v *StreamView) { v.Mission.Outcome = " " }, "has no outcome"},
		{"invalid key repo", func(v *StreamView) { v.Identity.Key.Repo = "repo-a" }, "repo"},
		{"wrong contract", func(v *StreamView) { v.Contract = "stream-view/v0" }, "contract"},
		{"evidence url not https", func(v *StreamView) {
			v.Mission.Success[0].Evidence[0] = EvidenceRef{Kind: EvidenceURL, URL: "http://example.org/x"}
		}, "https"},
		{"evidence path escapes", func(v *StreamView) { v.Mission.Success[0].Evidence[0].Path = "../x.md" }, ".."},
	}
	if err := validView().Validate(); err != nil {
		t.Fatalf("baseline view invalid: %v", err)
	}
	for _, c := range cases {
		v := validView()
		c.mut(v)
		err := v.Validate()
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: want error containing %q, got %v", c.name, c.want, err)
		}
		if _, encErr := Encode(v); encErr == nil {
			t.Errorf("%s: Encode accepted an invalid view", c.name)
		}
	}
}

func TestIdentityHelpers(t *testing.T) {
	a := Identity{Key: Key{Repo: "example-org/repo-a", Slug: "shared"}, DisplayName: "Shared"}
	b := Identity{Key: Key{Repo: "example-org/repo-b", Slug: "shared"}, DisplayName: "Shared"}
	r := Identity{Key: Key{Repo: "example-org/repo-a", Slug: "shared-v2"}, DisplayName: "Shared"}
	if SameStream(a, b) || SameStream(a, r) || !SameStream(a, a) {
		t.Fatal("SameStream must compare the repository-qualified key only")
	}
	if ContinuesHistory(a.Key, r.Key) {
		t.Fatal("a rename must not continue history")
	}
	if a.Key.String() != "example-org/repo-a:shared" {
		t.Fatalf("key string %q", a.Key.String())
	}
}
