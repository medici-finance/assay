package attestation

import (
	"strings"
	"testing"
)

func TestCapabilityProfiles(t *testing.T) {
	h := strings.Repeat("a", 64)
	s := strings.Repeat("b", 40)
	b := Binding{Version: 1, Run: h, Repo: "example-org/tracker", Source: s, Brief: "docs/streams/demo/brief-01.md", BriefSHA256: h, PlanSHA256: h, HomeSHA256: h, Model: "fixture", Tier: "strong", Checkout: "autocrlf=false eol=lf"}
	for _, p := range []Profile{ContainerV1, GeneralV1V2} {
		if why := BindingProblem(b, b.Repo, p, false); why != "" {
			t.Fatal(why)
		}
	}
	b.Version = 2
	b.SourceRepo = "example-org/deliverable"
	b.TrackingSource = s
	b.TrackingHomeSHA256 = h
	b.TrackingCheckout = b.Checkout
	if BindingProblem(b, b.Repo, ContainerV1, true) == "" {
		t.Fatal("container accepted v2")
	}
	if why := BindingProblem(b, b.Repo, GeneralV1V2, true); why != "" {
		t.Fatal(why)
	}
	if BindingProblem(b, b.Repo, GeneralV1V2, false) == "" {
		t.Fatal("unknown source accepted")
	}
}

// TestBriefPathOutsideRepository pins that a brief path which cleans to the
// repository's parent (or to the root itself) is refused under every profile,
// while an ordinary repository path still binds.
func TestBriefPathOutsideRepository(t *testing.T) {
	h := strings.Repeat("a", 64)
	s := strings.Repeat("b", 40)
	b := Binding{Version: 1, Run: h, Repo: "example-org/tracker", Source: s, BriefSHA256: h, PlanSHA256: h, HomeSHA256: h, Model: "fixture", Tier: "strong", Checkout: "autocrlf=false eol=lf"}
	for _, p := range []Profile{ContainerV1, GeneralV1V2} {
		for _, brief := range []string{"..", "../", "docs/../..", "a/../../", `..\`, ".", "./", "/docs/brief.md", "../docs/brief.md"} {
			b.Brief = brief
			if BindingProblem(b, b.Repo, p, false) == "" {
				t.Errorf("profile %v accepted brief path %q", p, brief)
			}
		}
		b.Brief = "docs/streams/demo/../demo/brief-01.md"
		if why := BindingProblem(b, b.Repo, p, false); why != "" {
			t.Errorf("profile %v refused an in-repository brief path: %s", p, why)
		}
	}
}

func TestStandingStamp(t *testing.T) {
	labels := []string{"dispatched-model:fixture", "dispatched-tier:strong"}
	ev := []LabelEvent{{Label: labels[0], Authority: true}, {Label: labels[1], Authority: true}}
	if m, r, w := Stamp(labels, ev); m != "fixture" || r != "strong" || w != "" {
		t.Fatal(m, r, w)
	}
	ev = append(ev, LabelEvent{Label: labels[0], Removed: true}, LabelEvent{Label: labels[0]})
	if _, _, w := Stamp(labels, ev); w == "" {
		t.Fatal("foreign standing application admitted")
	}
	if HistoryProblem(false, true, false) == "" || HistoryProblem(true, true, true) == "" {
		t.Fatal("unproven immutable history admitted")
	}
}

// Sibling sources bind only on a cross-repository run, in one canonical
// rendering, each distinct by name from both repositories already bound; an
// empty set renders exactly as a binding without the field.
// regression: #4876
func TestSiblingSourcesBinding(t *testing.T) {
	h := strings.Repeat("a", 64)
	s := strings.Repeat("b", 40)
	c := strings.Repeat("c", 40)
	base := Binding{Version: 2, Run: h, Repo: "example-org/tracker", SourceRepo: "example-org/one", Source: s, TrackingSource: s, TrackingHomeSHA256: h, TrackingCheckout: "autocrlf=false eol=lf", Brief: "docs/streams/demo/brief-01.md", BriefSHA256: h, PlanSHA256: h, HomeSHA256: h, Model: "fixture", Tier: "strong", Checkout: "autocrlf=false eol=lf"}
	if strings.Contains(Body(base), "SiblingSources") {
		t.Fatal("an empty sibling set changed the rendering")
	}
	two := FormatSiblingSources(map[string]string{"example-org/two": c, "example-org/three": s})
	if two != "example-org/three@"+s+",example-org/two@"+c {
		t.Fatalf("non-canonical rendering %q", two)
	}
	for _, tc := range []struct {
		siblings string
		ok       bool
	}{
		{two, true},
		{"example-org/two@" + c, true},
		{"example-org/two@" + c + ",example-org/three@" + s, false}, // unsorted
		{"example-org/two@" + c + ",fork-org/two@" + s, false},      // one name twice
		{"example-org/two=" + c, false},
		{"example-org/two@" + c[:39], false},
		{"example-org/two@" + strings.ToUpper(c), false},
		{"two@" + c, false},
		{"example-org/two/x@" + c, false},
		{"example-org/two@" + c + ",", false},
		{"example-org/one@" + c, false},     // the source repository again
		{"other-org/ONE@" + c, false},       // its name in another owner
		{"example-org/tracker@" + c, false}, // the tracking repository
		{"./two@" + c, false},               // dot-only owner
		{"example-org/..@" + c, false},      // dot-only name
		{"example-org/.@" + c, false},
		{"example-org/.two@" + c, true}, // a dot inside a segment stays legal
	} {
		b := base
		b.SiblingSources = tc.siblings
		if why := BindingProblem(b, b.Repo, GeneralV1V2, true); (why == "") != tc.ok {
			t.Errorf("%q: %q, want ok=%v", tc.siblings, why, tc.ok)
		}
	}
	same := Binding{Version: 1, Run: h, Repo: "example-org/tracker", Source: s, Brief: base.Brief, BriefSHA256: h, PlanSHA256: h, HomeSHA256: h, Model: "fixture", Tier: "strong", Checkout: base.Checkout, SiblingSources: "example-org/two@" + c}
	if BindingProblem(same, same.Repo, GeneralV1V2, false) == "" {
		t.Fatal("same-repository binding carried sibling sources")
	}
	bound := base
	bound.SiblingSources = two
	parsed, err := ParseBody(Body(bound))
	if err != nil || parsed != bound {
		t.Fatalf("round trip lost sibling sources: %v %+v", err, parsed)
	}
}
