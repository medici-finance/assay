package deskkit

import (
	"slices"
	"strings"
	"testing"
)

// #6695: one launched model reached the authored stamp under two spellings, the family alias
// and the full id. Each run removed the other's label as stale, so the stamp flipped on every
// run. SettleRequest keeps the standing full id when only the alias is requested.

func stampTL(by string, labels ...string) StampTimeline {
	tl := StampTimeline{Present: labels}
	for _, l := range labels {
		tl.Events = append(tl.Events, LabelEvent{Name: l, AppliedBy: by})
	}
	return tl
}

// regression: #6695
func TestSettleAliasKeepsFullID(t *testing.T) {
	ok := func(l string) bool { return l == "authority" }
	alias, _ := StampAuthored.Labels("opus", "strong")
	full, _ := StampAuthored.Labels("claude-opus-5-5", "strong")
	for _, tc := range []struct {
		name  string
		tl    StampTimeline
		want  []string
		label string
		keep  bool // the settled request removes nothing
	}{
		{"alias over standing full id keeps it", stampTL("authority", full...), alias, "authored-model:claude-opus-5-5", true},
		{"full id over standing alias replaces it", stampTL("authority", alias...), full, "authored-model:claude-opus-5-5", false},
		{"foreign full id is not kept", stampTL("someone", full...), alias, "authored-model:opus", false},
		{"alias that is not a word of the id", stampTL("authority", "authored-model:claude-opusx-5", "authored-tier:strong"), alias, "authored-model:opus", false},
		{"versioned request is never rewritten", stampTL("authority", full...), []string{"authored-model:claude-opus-5-6", "authored-tier:strong"}, "authored-model:claude-opus-5-6", false},
		{"half stamp is not kept", stampTL("authority", "authored-model:claude-opus-5-5"), alias, "authored-model:opus", false},
		// Only the family position settles: the vendor word and a suffix word name no family.
		{"vendor word is not the family", stampTL("authority", full...), []string{"authored-model:claude", "authored-tier:strong"}, "authored-model:claude", false},
		{"vendor word beside the version is not the family", stampTL("authority", "authored-model:gpt-6-1-sol", "authored-tier:strong"), []string{"authored-model:gpt", "authored-tier:strong"}, "authored-model:gpt", false},
		{"suffix word is not the family", stampTL("authority", "authored-model:gpt-6-1-sol", "authored-tier:strong"), []string{"authored-model:sol", "authored-tier:strong"}, "authored-model:sol", false},
		// A digit-bearing word of the standing slug is a version, never a family alias.
		{"digit word never settles", stampTL("authority", full...), []string{"authored-model:5", "authored-tier:strong"}, "authored-model:5", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := StampAuthored.SettleRequest(tc.tl, tc.want, ok)
			if !slices.Contains(got, tc.label) || len(got) != 2 {
				t.Fatalf("settled %v, want it to carry %s", got, tc.label)
			}
			if got[1] != tc.want[1] {
				t.Fatalf("tier came from the standing stamp: %v", got)
			}
			if rm := StampAuthored.ReStampRemovals(tc.tl, got, ok); tc.keep && len(rm) != 0 {
				t.Fatalf("the settled request still removes %v", rm)
			}
		})
	}
}

// regression: #6695
func TestSettleTierStillMoves(t *testing.T) {
	ok := func(string) bool { return true }
	tl := stampTL("authority", "dispatched-model:claude-opus-5-5", "dispatched-tier:strong")
	got := StampDispatch.SettleRequest(tl, []string{"dispatched-model:opus", "dispatched-tier:any"}, ok)
	if strings.Join(got, ",") != "dispatched-model:claude-opus-5-5,dispatched-tier:any" {
		t.Fatalf("settled %v", got)
	}
	if rm := StampDispatch.ReStampRemovals(tl, got, ok); strings.Join(rm, ",") != "dispatched-tier:strong" {
		t.Fatalf("removals %v, want only the old tier", rm)
	}
}

// changeStampForge is a one-change forge for ApplyVerifiedStamp: it serves the label set and
// its history, applies label writes as the given actor, and counts them.
type changeStampForge struct {
	Forge
	labels []string
	events []LabelEvent
	actor  string
	writes int
}

func (f *changeStampForge) GetPullRequest(_ ForgeRepo, n int) (*PullRequest, error) {
	return &PullRequest{Number: n, State: "open", Labels: append([]string(nil), f.labels...)}, nil
}
func (f *changeStampForge) ListLabelEvents(ForgeRepo, int) ([]LabelEvent, error) {
	return append([]LabelEvent(nil), f.events...), nil
}
func (f *changeStampForge) ApplyLabels(_ ForgeRepo, _ int, in LabelChange) (*LabelOutcome, error) {
	f.writes++
	for _, r := range in.Remove {
		for i, l := range f.labels {
			if l == r {
				f.labels = append(f.labels[:i], f.labels[i+1:]...)
				f.events = append(f.events, LabelEvent{Name: r, AppliedBy: f.actor, Removed: true})
				break
			}
		}
	}
	for _, a := range in.Add {
		f.labels = append(f.labels, a.Name)
		f.events = append(f.events, LabelEvent{Name: a.Name, AppliedBy: f.actor})
	}
	return &LabelOutcome{}, nil
}

// regression: #6695
// The shared writer is a plain "write exactly what was asked" choke point: settlement lives in
// the unattested stamp-only path, never here, so an attestation record or a bound run that asks
// for the alias gets the alias, replacing a standing full id from an earlier cycle.
func TestApplyStampWritesAsAsked(t *testing.T) {
	ok := func(l string) bool { return l == "authority" }
	full, _ := StampAuthored.Labels("claude-opus-5-5", "strong")
	tl := stampTL("authority", full...)
	f := &changeStampForge{labels: tl.Present, events: tl.Events, actor: "authority"}
	alias, _ := StampAuthored.Labels("opus", "strong")
	if err := ApplyVerifiedStamp(f, ForgeRepo{Owner: "example-org", Name: "one"}, 7, TargetChange, StampAuthored, alias, ok); err != nil {
		t.Fatal(err)
	}
	got, st := StampAuthored.AttestedStampOf(StampTimeline{Present: f.labels, Events: f.events}, ok)
	if st != ModelStamped || got.Model != "opus" {
		t.Fatalf("writes=%d labels=%v, want the requested alias standing", f.writes, f.labels)
	}
}
