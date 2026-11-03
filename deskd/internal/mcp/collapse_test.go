package mcp

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/deskd/internal/domain"
)

// TestCollapseFilingTime is Verify row 5 (filing-time half): a filing whose
// collapse key AND merge guard match an OPEN escalation comments "re-raised ×k"
// on the existing issue and opens NO new one (spec §1.1.3).
func TestCollapseFilingTime(t *testing.T) {
	// Derive the key+guard the escalate call will produce, by building the same
	// escalation the handler builds, and seed the fake filer with a matching
	// open escalation.
	opts := []domain.Option{{ID: "ship-partial", Label: "Ship", Consequence: "inbox live today"}}
	esc := domain.Escalation{
		Cell:       "assay",
		Automation: domain.AutomationRef{Cell: "assay", Role: "review"},
		Kind:       domain.KindNeedsDecision,
		Options:    opts,
		Rev:        domain.OptionsRev(opts),
	}
	openRef := FiledRef{Repo: "example-org/toolkit", Number: 42, URL: "https://github.com/example-org/toolkit/issues/42"}
	f := &fakeFiler{open: []OpenEscalation{{
		Ref:         openRef,
		CollapseKey: esc.Key().String(),
		MergeGuard:  esc.MergeGuard(),
		Repeat:      1,
	}}}
	s := testServer(t, f, DefaultFloodConfig)

	args := mustArgs(t, map[string]any{
		"cell":      "assay",
		"kind":      "needs-decision",
		"title":     "Ship the partial index?",
		"situation": "same recurring decision",
		"options":   []map[string]any{{"id": "ship-partial", "label": "Ship", "consequence": "inbox live today"}},
	})
	out, err := s.Dispatch(context.Background(), "desk__escalate", Call{Args: args, CallerLogin: reviewCaller})
	if err != nil {
		t.Fatalf("escalate: %v", err)
	}
	res := out.(EscalateResult)
	if res.Outcome != "merged" {
		t.Fatalf("outcome = %q, want merged (filing-time collapse)", res.Outcome)
	}
	if res.Repeat != 2 {
		t.Fatalf("repeat = %d, want 2 (re-raised ×2)", res.Repeat)
	}
	if len(f.filed) != 0 {
		t.Fatalf("a filing-time collapse must open NO new issue; opened %d", len(f.filed))
	}
	if len(f.comments) != 1 || f.comments[0].Ref.Number != 42 || !strings.Contains(f.comments[0].Note, "re-raised ×2") {
		t.Fatalf("want one 're-raised ×2' comment on #42; got %+v", f.comments)
	}
}

// TestCollapseChangedOptionsFilesFresh guards the other side of §1.1.3: a
// filing with a DIFFERENT option set (different guard) is a different decision
// and never merges — it opens a fresh issue even against a matching key.
func TestCollapseChangedOptionsFilesFresh(t *testing.T) {
	optsA := []domain.Option{{ID: "ship-partial", Label: "Ship", Consequence: "inbox live today"}}
	esc := domain.Escalation{
		Cell:       "assay",
		Automation: domain.AutomationRef{Cell: "assay", Role: "review"},
		Kind:       domain.KindNeedsDecision,
		Options:    optsA,
		Rev:        domain.OptionsRev(optsA),
	}
	f := &fakeFiler{open: []OpenEscalation{{
		Ref:         FiledRef{Repo: "example-org/toolkit", Number: 42},
		CollapseKey: esc.Key().String(),
		MergeGuard:  esc.MergeGuard(), // guard for optsA
		Repeat:      1,
	}}}
	s := testServer(t, f, DefaultFloodConfig)

	// Same key (cell/automation/kind) but a CHANGED option set.
	args := mustArgs(t, map[string]any{
		"cell":      "assay",
		"kind":      "needs-decision",
		"title":     "Ship the partial index?",
		"situation": "same recurring decision",
		"options":   []map[string]any{{"id": "wait-for-fix", "label": "Hold", "consequence": "no rework risk"}},
	})
	out, err := s.Dispatch(context.Background(), "desk__escalate", Call{Args: args, CallerLogin: reviewCaller})
	if err != nil {
		t.Fatalf("escalate: %v", err)
	}
	res := out.(EscalateResult)
	if res.Outcome != "filed" {
		t.Fatalf("changed options must file fresh, got outcome %q", res.Outcome)
	}
	if len(f.filed) != 1 || len(f.comments) != 0 {
		t.Fatalf("want one fresh issue and no re-raise; filed=%d comments=%d", len(f.filed), len(f.comments))
	}
}

// TestCollapseFlood is Verify row 5 (flood half): past the per-identity
// threshold a loop's escalations collapse into ONE "loop X is escalating
// abnormally" item (§6.6) rather than filling the queue.
func TestCollapseFlood(t *testing.T) {
	f := &fakeFiler{} // no open escalations, so nothing merges — flood is the only collapse
	s := testServer(t, f, FloodConfig{Window: time.Hour, FloodThreshold: 2})

	call := func() EscalateResult {
		args := mustArgs(t, map[string]any{
			"cell":      "assay",
			"kind":      "question",
			"title":     "help",
			"situation": "the loop keeps hitting the same wall",
		})
		out, err := s.Dispatch(context.Background(), "desk__escalate", Call{Args: args, CallerLogin: reviewCaller})
		if err != nil {
			t.Fatalf("escalate: %v", err)
		}
		return out.(EscalateResult)
	}

	// Below the threshold (2): each files its own issue.
	if r := call(); r.Outcome != "filed" {
		t.Fatalf("call 1 outcome = %q, want filed", r.Outcome)
	}
	if r := call(); r.Outcome != "filed" {
		t.Fatalf("call 2 outcome = %q, want filed", r.Outcome)
	}
	// The third crosses the threshold: it collapses into the abnormal item.
	if r := call(); r.Outcome != "flood-collapsed" {
		t.Fatalf("call 3 outcome = %q, want flood-collapsed", r.Outcome)
	}
	// The fourth folds onto the SAME abnormal item — no second flood issue.
	if r := call(); r.Outcome != "flood-collapsed" {
		t.Fatalf("call 4 outcome = %q, want flood-collapsed", r.Outcome)
	}

	// Exactly three issues were opened: two normal filings plus ONE flood item.
	if len(f.filed) != 3 {
		t.Fatalf("want 3 issues opened (2 normal + 1 flood), got %d", len(f.filed))
	}
	floodItems := 0
	for _, e := range f.filed {
		if strings.Contains(e.Title, "escalating abnormally") {
			floodItems++
		}
	}
	if floodItems != 1 {
		t.Fatalf("a flood must collapse to ONE abnormal item, got %d", floodItems)
	}
	// The fourth filing folded onto the flood item as a comment.
	if len(f.comments) != 1 {
		t.Fatalf("want one flood re-raise comment, got %d", len(f.comments))
	}
}
