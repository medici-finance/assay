package main

import (
	"errors"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"github.com/medici-finance/assay/tools/desk/internal/loopengine"
	"strings"
	"testing"
	"time"
)

type pendingFixture struct {
	refs map[string]string
	err  error
}

func (p pendingFixture) IssueState(deskkit.BlockerRef) (deskkit.BlockerState, string) {
	return deskkit.BlockerCouldNotCheck, "offline"
}
func (p pendingFixture) PendingEvidence(root, repo string, it loopengine.Item, now time.Time, inputs deskkit.WakeInputs) (string, error) {
	return p.refs[it.ID], p.err
}

// regression: #2939
func TestPendingQueue(t *testing.T) {
	root := planFixtureRoot(t, map[string]string{"01": planBrief("model", "no", "no", "no", "no"), "02": planBrief("human", "no", "no", "no", "no"), "03": planBrief("model", "no", "no", "no", "no")})
	v := &VerifyLoop{Root: root, Issues: pendingFixture{refs: map[string]string{"example-stream/01": "#41", "example-stream/02": "#42"}}}
	items, err := v.SelectQueue()
	if err != nil {
		t.Fatal(err)
	}
	for _, it := range items {
		tier, _ := v.TierPolicy(it)
		d, why := classifyItem(it, tier)
		want := "awaiting-merge"
		if strings.HasSuffix(it.ID, "/03") {
			want = "dispatch"
		}
		if d.String() != want {
			t.Errorf("%s = %s (%s), want %s", it.ID, d, why, want)
		}
	}
	// Planted second caller: the same queue join must cover a configured second root.
	v.Roots = []deskkit.RootConfig{{Repo: "example-org/second", Path: root}}
	v.Issues = pendingFixture{refs: map[string]string{"example-org/second:example-stream/01": "#51", "example-org/second:example-stream/02": "#52"}}
	items, err = v.SelectQueue()
	if err != nil {
		t.Fatal(err)
	}
	for _, it := range items {
		tier, _ := v.TierPolicy(it)
		d, _ := classifyItem(it, tier)
		if !strings.HasSuffix(it.ID, "/03") && d.String() != "awaiting-merge" {
			t.Errorf("second root %s = %s, want awaiting-merge", it.ID, d)
		}
	}
	v.Roots = nil
	v.Issues = pendingFixture{err: errors.New("Evidence unreadable")}
	items, err = v.SelectQueue()
	if err != nil {
		t.Fatal(err)
	}
	for _, it := range items {
		tier, _ := v.TierPolicy(it)
		d, _ := classifyItem(it, tier)
		if d != dispCouldNotCheck {
			t.Errorf("unreadable %s = %s", it.ID, d)
		}
	}
}
