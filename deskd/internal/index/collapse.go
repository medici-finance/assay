package index

import (
	"sort"
	"time"

	"github.com/medici-finance/assay/deskd/internal/domain"
)

// foldCollapse is the derivation-time collapse fold (decision-model spec
// §1.1.3): the adapter-side safety net beneath the preferred filing-time
// collapse in desk__escalate. A retrying loop is the normal case; without
// dedup the queue fills with N copies of one decision.
//
// Collapse key = (Cell, Automation, Kind). Merge guard = OptionsRev, or the
// content hash of the normalised situation for option-less kinds. Two filings
// fold iff their key AND guard both match:
//
//   - Repeat becomes the sum across the merged group (each filing counts ≥ 1).
//   - Urgency = max, so a re-raise at higher urgency is never swallowed.
//   - FiledAt = min: a retrying loop MUST NOT reset its own clock (spec §3.1);
//     the earliest filing is the survivor, so its ULID and body represent the
//     group.
//   - Updated = the latest re-raise time in the group — the honest "last
//     merge" moment. It is derived from the group's own timestamps, not the
//     request clock, so a re-poll of an unchanged group does not churn it
//     (the spec says "Updated = now"; at derivation time the most recent
//     re-raise IS that moment, and using it keeps the value stable).
//
// A filing whose OPTIONS CHANGED is a DIFFERENT decision (different guard) and
// does NOT merge — it stays a separate row. Unattributable filings (empty
// collapse key) NEVER merge, because the adapter cannot prove same-origin;
// each keeps its own row. Closed escalations are already out of the inbox
// window, so a re-ask after a close is a fresh open row, never a merge.
func foldCollapse(rows []Row, now time.Time) []Row {
	// Sort by (FiledAt, ID) first so the survivor of every group is the
	// earliest filing — that is what makes FiledAt = min structural rather
	// than a comparison inside the loop.
	ordered := make([]Row, len(rows))
	copy(ordered, rows)
	sort.SliceStable(ordered, func(i, j int) bool {
		if !ordered[i].FiledAt.Equal(ordered[j].FiledAt) {
			return ordered[i].FiledAt.Before(ordered[j].FiledAt)
		}
		return ordered[i].ID < ordered[j].ID
	})

	type gkey struct{ key, guard string }
	survivors := map[gkey]int{} // group key -> index into out
	var out []Row

	for _, r := range ordered {
		ck := r.Key().String()
		if ck == "" || r.Closed {
			// Unattributable never folds: give it a group nobody else can
			// join by keying on its own id. A CLOSED row folds into nothing
			// either, for the same reason spelled out above this function —
			// once includeClosed=true is in play (the service contract increment
			// 3), a closed row and a fresh re-ask of the same key/guard could
			// otherwise collide here and the closed row's own decided outcome
			// would vanish into a live group's Repeat count.
			out = append(out, r)
			continue
		}
		gk := gkey{key: ck, guard: r.MergeGuard()}
		if idx, ok := survivors[gk]; ok {
			s := &out[idx]
			s.Repeat += repeatOf(r)
			if r.Urgency > s.Urgency {
				s.Urgency = r.Urgency
			}
			if t := latestActivity(r); s.Updated == nil || t.After(*s.Updated) {
				tt := t
				s.Updated = &tt
			}
			continue
		}
		if r.Repeat < 1 {
			r.Repeat = 1
		}
		survivors[gk] = len(out)
		out = append(out, r)
	}
	return out
}

// repeatOf is a filing's own repeat contribution, never below one — a stored
// row already folded at filing time carries its count, and a fresh row counts
// as one.
func repeatOf(r Row) int {
	if r.Repeat < 1 {
		return 1
	}
	return r.Repeat
}

// latestActivity is a row's most recent timestamp: its Updated if set, else
// its FiledAt. Used to derive the merged group's Updated.
func latestActivity(r Row) time.Time {
	if r.Updated != nil && r.Updated.After(r.FiledAt) {
		return *r.Updated
	}
	return r.FiledAt
}

// attentionFor derives the row's automation state for the live inbox (spec
// §1.2). Every inbox row is an OPEN escalation, so an attributable row's
// automation has ≥ 1 open escalation and derives Waiting — the "⟡ waiting on
// you" gold gate. An UNATTRIBUTABLE row has no automation to attribute a state
// to, so the state is underivable and renders as JSON null, exactly as its
// automation does (a could-not-check, never a fabricated Silent/Working).
//
// This build ingests no artifact or dispatch stream beyond escalation issues,
// so the richer working/idle/failed-to-start/silent states are not reachable
// on the live inbox surface; they are exercised by the unit derivation and
// arrive live with the Portfolio strip and pod substrate. DeriveAttention is
// the single derivation both paths call.
func attentionFor(r Row, openByAutomation map[string]int, now time.Time) *domain.AttentionRender {
	if !r.Attributable() {
		return nil
	}
	if r.Closed {
		// A closed row is never "waiting on you" — it has been decided. This
		// only matters once a caller reads with includeClosed=true
		// (the service contract increment 3, task 6): before Row carried a Closed
		// field, a closed row here was structurally indistinguishable from an
		// open one, so this branch could not exist and could not be reached.
		return nil
	}
	in := AttentionInputs{OpenEscalations: openByAutomation[r.Automation.String()]}
	st := DeriveAttention(in, AttentionConfig{}, now)
	render := domain.Render(st)
	return &render
}
