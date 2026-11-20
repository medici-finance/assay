package main

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/statusgen/evaluation"
	"github.com/medici-finance/assay/statusgen/streamview"
)

func milestoneStreams(t *testing.T) (opted, legacy *Stream) {
	t.Helper()
	advertiseMilestones(t)
	root := milestoneTree(t)
	writeMilestoneFile(t, root, "docs/streams/delivery/milestones/m-01-runner.md", milestoneFixture(t, "compatible.md"))
	writeMilestoneFile(t, root, "docs/streams/delivery/milestones/m-03-build-less-brittle-v2.md", milestoneFixture(t, "canonical.md"))
	streams, _, err := loadStreams(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range streams {
		switch s.Name {
		case "delivery":
			opted = s
		case "legacy":
			legacy = s
		}
	}
	if opted == nil || legacy == nil || opted.Milestones == nil || len(opted.Milestones.Records) == 0 {
		t.Fatalf("fixture streams not loaded: %+v", opted)
	}
	return opted, legacy
}

// regression: milestone-support/11 — the producer projects the canonical
// reader and evaluator; it never derives done or accepted from a zero count.
func TestMilestoneStreamProjection(t *testing.T) {
	opted, legacy := milestoneStreams(t)
	observed := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	rev := strings.Repeat("a", 40)
	src := streamview.SourceRef{Repo: "example/assay", Path: "docs/streams/delivery/README.md", Revision: rev}

	// Without a readiness evaluation, work and acceptance are unknown, never zero.
	sec := streamMilestoneSection(opted.Milestones, nil, "example/assay", src, observed)
	for _, it := range sec.Items {
		if it.Work.State != streamview.WorkUnknown || it.Acceptance.State != streamview.AcceptanceCouldNotCheck || it.Availability != streamview.MilestoneUnknown {
			t.Errorf("%s: missing readiness must stay unknown: %+v", it.Ref, it)
		}
	}

	// m-03: no work remaining but no acceptance; m-01: work remains.
	rr := &evaluation.ReadinessResult{Milestones: map[string]evaluation.MilestoneReadiness{
		"assay:assay:delivery:m-03@1": {
			Ref: "assay:assay:delivery:m-03@1", WorkComplete: true, AcceptanceOutstanding: true,
			Subject:      evaluation.ReadinessSubject{Token: "t", Environment: "e", Interface: "i", Profile: "p"},
			Availability: evaluation.AvailabilityBlocked,
			Holds:        []evaluation.ReadinessHold{{Ref: "assay:assay:delivery:m-03@1", Kind: evaluation.HoldAcceptance, State: evaluation.StateUnsatisfied, Reason: "acceptance outstanding"}},
		},
		"assay:assay:delivery:m-01@1": {
			Ref: "assay:assay:delivery:m-01@1", WorkRemaining: 1, Remaining: []string{"assay:assay:delivery:b-11"},
			Availability: evaluation.AvailabilityBlocked,
			Holds:        []evaluation.ReadinessHold{{Ref: "assay:assay:delivery:b-11", Kind: evaluation.HoldWork, State: evaluation.StateUnsatisfied, Reason: "work remaining"}},
		},
	}}
	sec = streamMilestoneSection(opted.Milestones, rr, "example/assay", src, observed)
	by := map[string]streamview.MilestoneItem{}
	for _, it := range sec.Items {
		by[it.Ref] = it
	}
	m3, m1 := by["assay:assay:delivery:m-03@1"], by["assay:assay:delivery:m-01@1"]
	if m3.Work.State != streamview.WorkKnown || m3.Work.Remaining != 0 || m3.Acceptance.State != streamview.AcceptanceOutstanding || m3.Availability != streamview.MilestoneBlocked {
		t.Errorf("zero work must read as acceptance outstanding, not done: %+v", m3)
	}
	if m1.Work.Remaining != 1 || m1.Acceptance.State != streamview.AcceptanceCouldNotCheck || len(m1.Holds) != 1 {
		t.Errorf("remaining work with no subject: %+v", m1)
	}
	if len(m3.Scope) != 2 || len(m3.Prerequisites) != 1 || m3.Prerequisites[0] != "assay:assay:delivery:m-01@1" {
		t.Errorf("scope or prerequisites lost: %+v", m3)
	}
	var members []string
	for _, mb := range sec.Membership {
		members = append(members, mb.Brief+"="+strings.Join(mb.Milestones, "+"))
	}
	if !strings.Contains(strings.Join(members, " "), "assay:assay:delivery:b-11=assay:assay:delivery:m-03@1") {
		t.Errorf("reverse membership missing: %v", members)
	}

	// The versioned builder: opted-in streams get v2 when the consumer accepts
	// it; the non-opted-in view is the unchanged v1 seed.
	v, err := streamViewVersioned(opted, rr, "example/assay", "docs/streams/delivery/README.md", rev, observed, streamview.AccessContext{}, streamview.SupportedVersions)
	if err != nil || v.Contract != streamview.Version2 || v.Milestones == nil {
		t.Fatalf("opted-in v2: %v %+v", err, v)
	}
	if _, err := streamview.Encode(v); err != nil {
		t.Fatalf("v2 view does not encode: %v", err)
	}
	old, err := streamViewVersioned(opted, rr, "example/assay", "docs/streams/delivery/README.md", rev, observed, streamview.AccessContext{}, []string{streamview.Version})
	if err != nil || old.Contract != streamview.Version || old.Milestones != nil {
		t.Fatalf("a v1-only consumer must get the v1 view: %v", err)
	}
	seed, err := streamViewSeed(legacy, "example/assay", "docs/streams/legacy/README.md", rev, observed, streamview.AccessContext{})
	if err != nil {
		t.Fatal(err)
	}
	got, err := streamViewVersioned(legacy, nil, "example/assay", "docs/streams/legacy/README.md", rev, observed, streamview.AccessContext{}, streamview.SupportedVersions)
	if err != nil {
		t.Fatal(err)
	}
	a, _ := json.Marshal(seed)
	b, _ := json.Marshal(got)
	if string(a) != string(b) || got.Milestones != nil {
		t.Errorf("a non-opted-in stream's view must be unchanged:\nseed %s\ngot  %s", a, b)
	}
	if _, err := streamViewVersioned(legacy, nil, "example/assay", "docs/streams/legacy/README.md", rev, observed, streamview.AccessContext{}, []string{streamview.Version2}); err == nil {
		t.Error("a milestone-only consumer must get no view of a non-opted-in stream, not an invented empty section")
	}

	// A plan that could not be read is could-not-check with its diagnostics, not an empty list.
	bad := &MilestonePlan{Stream: "delivery", OptedIn: true, Coverage: MilestoneCoverageUnknown,
		Unknowns: []MilestoneUnknown{{Path: "milestones/m-02.md", Reason: "not a regular file"}}}
	sec = streamMilestoneSection(bad, nil, "example/assay", src, observed)
	if sec.Provenance.Availability != streamview.CouldNotCheck || len(sec.Items) != 0 || len(sec.Provenance.Diagnostics) != 1 {
		t.Errorf("unreadable plan: %+v", sec)
	}
	// Records read but one file unreadable: partial, coverage unknown, the gap listed.
	partial := *opted.Milestones
	partial.Coverage = MilestoneCoverageUnknown
	partial.Unknowns = []MilestoneUnknown{{Path: "milestones/m-09.md", Reason: "not a regular file"}}
	sec = streamMilestoneSection(&partial, nil, "example/assay", src, observed)
	if sec.Provenance.Availability != streamview.Partial || sec.Coverage != streamview.CoverageUnknown || len(sec.Unknowns) != 1 || len(sec.Items) == 0 {
		t.Errorf("a gap must make coverage unknown and list it: %+v", sec)
	}
	notIn := *opted.Milestones
	notIn.OptedIn = false
	if sec = streamMilestoneSection(&notIn, nil, "example/assay", src, observed); sec.Provenance.Availability != streamview.NotAssessed || len(sec.Items) != 0 {
		t.Errorf("a plan that did not opt in must not project records: %+v", sec)
	}
	if sec = streamMilestoneSection(nil, nil, "example/assay", src, observed); sec.Provenance.Availability != streamview.NotAssessed || len(sec.Items) != 0 {
		t.Errorf("non-opted-in stream must not get invented milestones: %+v", sec)
	}
}
