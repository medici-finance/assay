package reviewflow

import (
	"fmt"
	"sort"
)

// Key identifies a proposed operation within the supplied board observations.
// It is not a completed-write key: review dispatch has no verdict or body yet,
// and a prior ready attempt cannot override a board that still reports a draft.
type Key struct {
	Repo string
	PR   int
	Head string
	Verb string
}

func (k Key) String() string { return fmt.Sprintf("%s#%d@%s:%s", k.Repo, k.PR, short(k.Head), k.Verb) }

// PlannedVerb is one coalesced outward action.
type PlannedVerb struct {
	Key Key
	Row Row
	// Observations is how many board deltas collapsed into this one verb. It is the
	// coalescing evidence: N sweeps seeing the same (pr, head) must produce ONE action.
	Observations int
	// Suppressed is set when the verb will NOT be emitted, with Why naming which gate
	// stopped it: an unresolved head.
	Suppressed bool
	Why        string
}

// Coalesce collapses a stream of board observations into at most ONE outward verb per
// (repo, pr, head, verb).
//
// `sweeps` is the sequence of boards observed since the last plan — in the standing window
// that is every cadence tick and every event wake between two dispatch decisions. Feeding
// the same PR N times at one head must produce one verb; a PR whose head ADVANCES between
// sweeps produces a DIFFERENT key and therefore re-arms, which is the archetype-B property
// this whole driver exists to preserve. The board decides WHAT the advance means
// (MERGE-CURR for a benign keep-current merge, RE-REVIEW when the PR's own files changed);
// the reactor honours that classification and never second-guesses it.
//
// A row with HeadUnresolved is SUPPRESSED: the operation cannot be bound to a
// revision. Audit history is not an input. The board determines what needs doing;
// deskdispatch must acquire a live claim before launching a reviewer, and deskpost
// rechecks current forge state and exact write identity under its outward lock.
func Coalesce(sweeps []*Board) []PlannedVerb {
	byKey := map[Key]*PlannedVerb{}
	var order []Key

	for _, b := range sweeps {
		if b == nil {
			continue
		}
		for _, r := range b.Rows {
			if r.Rule.Verb == "" {
				continue // WAIT / SURFACE / NO-OP rows carry no outward verb
			}
			k := Key{Repo: r.Repo, PR: r.Number, Head: r.Head, Verb: r.Rule.Verb}
			if pv, ok := byKey[k]; ok {
				pv.Observations++
				pv.Row = r // the newest observation wins for display
				continue
			}
			byKey[k] = &PlannedVerb{Key: k, Row: r, Observations: 1}
			order = append(order, k)
		}
	}

	out := make([]PlannedVerb, 0, len(order))
	for _, k := range order {
		pv := byKey[k]
		switch {
		case k.Head == HeadUnresolved:
			pv.Suppressed = true
			pv.Why = "head SHA unresolved — `deskboard actions` carries no head and the `prs` join found no row, " +
				"so the (repo,pr,head,verb) idempotency key cannot be formed; COULD-NOT-CHECK, the verb is not emitted"
		}
		out = append(out, *pv)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Key.String() < out[j].Key.String() })
	return out
}

// Dispatched returns the proposed verbs with resolved heads. These are candidates,
// not proof of a free dispatch claim or permission to write.
func Dispatched(vs []PlannedVerb) []PlannedVerb {
	var out []PlannedVerb
	for _, v := range vs {
		if !v.Suppressed {
			out = append(out, v)
		}
	}
	return out
}
