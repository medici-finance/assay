package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// IssueLabelHistory answers the recording fake's label-history read (the label-removal hold, op 64). An
// issue named "locked" or numbered 404 answers could-not-check; number 9 is an incomplete
// history; number 8 is an empty one.
func (f *recForge) IssueLabelHistory(repo deskkit.ForgeRepo, number int) (*deskkit.LabelHistory, error) {
	f.rec(recCall{Op: "IssueLabelHistory", Repo: repo.Slug(), Number: number})
	if err := failFor(repo, number); err != nil {
		return nil, err
	}
	switch number {
	case 8:
		return &deskkit.LabelHistory{Complete: true}, nil
	case 9:
		return &deskkit.LabelHistory{Complete: false, Events: []deskkit.LabelEvent{
			{Name: "needs-human", AppliedBy: "someone", AppliedByID: 42, CreatedAt: "2026-09-01T00:00:00Z"},
		}}, nil
	}
	return &deskkit.LabelHistory{Complete: true, Events: []deskkit.LabelEvent{
		{Name: "needs-decision", AppliedBy: "authority", AppliedByID: 100001, CreatedAt: "2026-09-01T00:00:00Z"},
		{Name: "needs-decision", AppliedBy: "example-app", AppliedByID: 300000001, CreatedAt: "2026-09-02T00:00:00Z", Removed: true},
		// An event whose label the forge no longer names, and one with no actor: both kept as
		// the seam served them, neither dropped nor filled in.
		{Name: "", AppliedBy: "authority", AppliedByID: 100001, CreatedAt: "2026-09-03T00:00:00Z", Removed: true},
		{Name: "verify-gate", CreatedAt: "2026-09-04T00:00:00Z"},
	}}, nil
}

type labelEventsEnv struct {
	Kind  string `json:"kind"`
	Items []struct {
		Repo        string           `json:"repo"`
		Number      int              `json:"number"`
		LabelEvents *LabelEventsJSON `json:"labelEvents"`
	} `json:"items"`
	Partial []PartialItem `json:"partial"`
}

// regression: #2414 — the label-events kind carries every event, the unnamed ones too,
// with actor login and id, and its completeness flag; it is transport, so it passes no verdict.
func TestDeskreadLabelEvents(t *testing.T) {
	f := stubRec(t)
	out, code, errs := rawOut("label-events", "--issue", "example-org/alpha#7", "--issue", "example-org/alpha#8", "--issue", "example-org/alpha#9")
	if code != deskkit.ExitOK {
		t.Fatalf("exit=%d; stderr=%s", code, errs)
	}
	var env labelEventsEnv
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatalf("stdout is not the envelope: %v\n%s", err, out)
	}
	if env.Kind != "label-events" || len(env.Items) != 3 || len(env.Partial) != 0 {
		t.Fatalf("envelope = %s", out)
	}
	byNum := map[int]*LabelEventsJSON{}
	for _, it := range env.Items {
		byNum[it.Number] = it.LabelEvents
	}
	full := byNum[7]
	if full == nil || !full.Complete || len(full.Events) != 4 {
		t.Fatalf("issue 7 = %+v, want a complete four-event history", full)
	}
	want := []LabelEventJSON{
		{Label: "needs-decision", CreatedAt: "2026-09-01T00:00:00Z", ActorLogin: "authority", ActorID: 100001},
		{Label: "needs-decision", Removed: true, CreatedAt: "2026-09-02T00:00:00Z", ActorLogin: "example-app", ActorID: 300000001},
		{Label: "", Removed: true, CreatedAt: "2026-09-03T00:00:00Z", ActorLogin: "authority", ActorID: 100001},
		{Label: "verify-gate", CreatedAt: "2026-09-04T00:00:00Z"},
	}
	for i, w := range want {
		if full.Events[i] != w {
			t.Errorf("event %d = %+v, want %+v", i, full.Events[i], w)
		}
	}
	if e := byNum[8]; e == nil || !e.Complete || e.Events == nil || len(e.Events) != 0 {
		t.Errorf("issue 8 = %+v, want a complete EMPTY history rendered as [] (a real answer)", e)
	}
	if !strings.Contains(out, `"events": []`) {
		t.Errorf("an empty history must render events as [], got %s", out)
	}
	if e := byNum[9]; e == nil || e.Complete {
		t.Errorf("issue 9 = %+v, want complete=false carried through (an answer, not a partial)", e)
	}
	// The unnamed event keeps an empty label; a missing actor keeps an empty login and no id.
	if !strings.Contains(out, `"label": ""`) {
		t.Errorf("the unnamed event must keep an empty label key, got %s", out)
	}
	for _, c := range f.recorded() {
		if c.Op != "IssueLabelHistory" {
			t.Errorf("label-events reached for %s; it must touch only IssueLabelHistory", c.Op)
		}
	}
	// No verdict vocabulary anywhere in the output.
	for _, w := range []string{"held", "clear", "human", "release"} {
		if strings.Contains(strings.ToLower(out), `"`+w) {
			t.Errorf("output carries a verdict-shaped key %q: %s", w, out)
		}
	}
}

// regression: #2414 — an issue whose history could not be read lands in partial with its
// reason and is absent from items; the others are still served; every issue unreadable is exit 6.
func TestDeskreadLabelEventsPartial(t *testing.T) {
	stubRec(t)
	out, code, errs := rawOut("label-events", "--issue", "example-org/alpha#7", "--issue", "example-org/alpha#404", "--issue", "example-org/alpha#405")
	if code != deskkit.ExitOK {
		t.Fatalf("exit=%d; stderr=%s", code, errs)
	}
	var env labelEventsEnv
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatal(err)
	}
	if len(env.Items) != 1 || env.Items[0].Number != 7 {
		t.Fatalf("items = %s", out)
	}
	if len(env.Partial) != 2 {
		t.Fatalf("partial = %+v, want issues 404 and 405", env.Partial)
	}
	for _, p := range env.Partial {
		if p.Reason == "" || (p.Number != 404 && p.Number != 405) {
			t.Errorf("partial entry %+v", p)
		}
	}
	if strings.Contains(out, `"labelEvents": null`) {
		t.Errorf("a partial issue must carry no data field: %s", out)
	}

	_, code, _ = rawOut("label-events", "--issue", "example-org/alpha#404")
	if code != deskkit.ExitUnverifiable {
		t.Errorf("every issue unreadable: exit=%d, want %d", code, deskkit.ExitUnverifiable)
	}

	// The kind is per-issue: a repo or change address is refused, and it is not CI-served.
	if _, code, _ := rawOut("label-events", "--repo", "example-org/alpha"); code != deskkit.ExitRefused {
		t.Errorf("--repo exit=%d, want refused", code)
	}
	if _, code, _ := rawOut("label-events", "--change", "example-org/alpha#7"); code != deskkit.ExitRefused {
		t.Errorf("--change exit=%d, want refused", code)
	}
}
