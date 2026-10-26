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
