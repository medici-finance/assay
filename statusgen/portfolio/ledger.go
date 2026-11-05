package portfolio

import (
	"encoding/json"
	"sort"
	"time"
)

// Quantity is one measure's known subtotal and its coverage. A record whose value is unknown
// adds to UnknownRecords and nothing to Known: the subtotal is never presented as a total.
type Quantity struct {
	Known          int64 `json:"known"`
	KnownRecords   int   `json:"known-records"`
	UnknownRecords int   `json:"unknown-records"`
}

// Complete reports whether every record behind the quantity had a known value.
func (q Quantity) Complete() bool { return q.UnknownRecords == 0 }

// Tally holds agent and human measures side by side and never mixes them: an agent
// duration is not a human count, and a human count is not a duration.
type Tally struct {
	Agent map[string]Quantity `json:"agent"`
	Human map[string]Quantity `json:"human"`
}

func newTally() Tally {
	t := Tally{Agent: map[string]Quantity{}, Human: map[string]Quantity{}}
	for _, m := range append(append([]string(nil), agentMeasures...), ElapsedProxy) {
		t.Agent[m] = Quantity{}
	}
	for _, m := range humanMeasures {
		t.Human[m] = Quantity{}
	}
	return t
}

// Entry names one report the ledger did not count as known, and why.
type Entry struct {
	Kind   string `json:"kind"`
	Key    string `json:"key"`
	Reason string `json:"reason"`
}

// Ledger is the read-only measured allocation for one policy revision over its window.
// Total is the sum of every bucket and Unattributed; unattributed work stays visible.
type Ledger struct {
	Policy         string           `json:"policy"`
	PolicyRevision string           `json:"policy-revision"`
	Scope          string           `json:"scope"`
	Window         Interval         `json:"window"`
	Weights        string           `json:"weights"`
	WeightsRev     string           `json:"weights-revision"`
	Buckets        map[string]Tally `json:"buckets"`
	Unattributed   Tally            `json:"unattributed"`
	Total          Tally            `json:"total"`
	Duplicates     []Entry          `json:"duplicates"`
	Conflicts      []Entry          `json:"conflicts"`
	Excluded       []Entry          `json:"excluded"`
}

// Measure builds the ledger for a snapshot. It validates first and reads nothing else.
// semantic: S-portfolio-measurement
func Measure(s Snapshot) (Ledger, error) {
	if err := Validate(s); err != nil {
		return Ledger{}, err
	}
	p := s.Policy
	l := Ledger{
		Policy: p.ID, PolicyRevision: p.Revision, Scope: p.Scope, Window: p.Window,
		Weights: p.Weights.ID, WeightsRev: p.Weights.Revision,
		Buckets: map[string]Tally{}, Unattributed: newTally(), Total: newTally(),
		Duplicates: []Entry{}, Conflicts: []Entry{}, Excluded: []Entry{},
	}
	owner := map[Subject]string{}
	for _, b := range p.Buckets {
		l.Buckets[b.ID] = newTally()
		for _, m := range b.Members {
			owner[m] = b.ID
		}
	}
	weights := map[string]int64{}
	for _, m := range p.Weights.Models {
		weights[m.Model] = m.Milli
	}
	bucketOf := func(s *Subject) string {
		if s == nil {
			return ""
		}
		return owner[*s]
	}
	tally := func(bucket string) Tally {
		if bucket == "" {
			return l.Unattributed
		}
		return l.Buckets[bucket]
	}
	record := func(bucket string, agent bool, measure string, v *int64) {
		tallies := []Tally{tally(bucket), l.Total}
		pick := func(t Tally) map[string]Quantity {
			if agent {
				return t.Agent
			}
			return t.Human
		}
		// A value that would take any subtotal it joins outside the int64 range is unknown in
		// all of them, so Total stays the sum of the buckets and Unattributed.
		for _, t := range tallies {
			if _, ok := addInt64(pick(t)[measure].Known, deref(v)); !ok {
				v = nil
			}
		}
		for _, t := range tallies {
			m := pick(t)
			q := m[measure]
			if v == nil {
				q.UnknownRecords++
			} else {
				q.Known, _ = addInt64(q.Known, *v)
				q.KnownRecords++
			}
			m[measure] = q
		}
	}
	measureSegments(s, &l, weights, bucketOf, record)
	measureDecisions(s, &l, bucketOf, record)
	return l, nil
}

type recordFunc func(bucket string, agent bool, measure string, v *int64)

func measureSegments(s Snapshot, l *Ledger, weights map[string]int64, bucketOf func(*Subject) string, record recordFunc) {
	reports := map[Invocation]map[string][]Segment{}
	seen := map[string]bool{}
	for _, g := range s.Segments {
		enc := encode(g)
		if seen[enc] {
			l.Duplicates = append(l.Duplicates, Entry{"segment", segKey(g), "duplicate report counted once"})
			continue // duplicate report
		}
		seen[enc] = true
		if reports[g.Invocation] == nil {
			reports[g.Invocation] = map[string][]Segment{}
		}
		reports[g.Invocation][g.ID] = append(reports[g.Invocation][g.ID], g)
	}
	reconciled := map[Invocation]Reconciliation{}
	for _, r := range s.Reconciliations {
		reconciled[r.Invocation] = r
	}
	invs := make([]Invocation, 0, len(reports))
	for inv := range reports {
		invs = append(invs, inv)
	}
	sort.Slice(invs, func(i, j int) bool {
		if invs[i].Caller != invs[j].Caller {
			return invs[i].Caller < invs[j].Caller
		}
		return invs[i].ID < invs[j].ID
	})
	for _, inv := range invs {
		groups := reports[inv]
		key := inv.Caller + "/" + inv.ID
		selected, rejected, reason := selectSegments(groups, reconciled, inv)
		if reason == "" {
			reason = overlap(selected)
		}
		if reason != "" && wholly(groups, l.Window) {
			// Every report lies outside the window: whichever one is right, none counts here.
			l.Excluded = append(l.Excluded, Entry{"invocation", key, "every conflicting report lies outside the policy window"})
			continue
		}
		if reason != "" {
			// A conflict is not resolved by counting one side or both: the invocation stays
			// visible as unknown in every agent measure until it is reconciled.
			l.Conflicts = append(l.Conflicts, Entry{"invocation", key, reason})
			bucket := agreedBucket(groups, bucketOf)
			for _, m := range append(append([]string(nil), agentMeasures...), ElapsedProxy) {
				record(bucket, true, m, nil)
			}
			continue
		}
		l.Excluded = append(l.Excluded, rejected...)
		for _, g := range selected {
			bucket := bucketOf(g.Subject)
			active, at := activeSeconds(g, l.Window)
			switch at {
			case outside:
				l.Excluded = append(l.Excluded, Entry{"segment", segKey(g), "active interval outside the policy window"})
				continue
			case straddles:
				// The window cannot apportion the segment, so none of its agent measures is
				// attributed to the window.
				for _, m := range append(append([]string(nil), agentMeasures...), ElapsedProxy) {
					record(bucket, true, m, nil)
				}
				continue
			}
			var secs, weighted *int64
			if at == inside {
				secs = &active
				if milli, ok := weights[g.ActualModel]; ok && g.ActualModel != "" {
					if w, ok := mulInt64(active, milli); ok {
						weighted = &w
					}
				}
			}
			record(bucket, true, ActiveSeconds, secs)
			record(bucket, true, WeightedActive, weighted)
			record(bucket, true, CostMicros, g.Cost)
			record(bucket, true, ElapsedProxy, g.Elapsed)
		}
	}
}

// wholly reports whether every report of an invocation carries an active interval lying
// entirely outside the window. A report without an interval could lie anywhere.
func wholly(groups map[string][]Segment, window Interval) bool {
	for _, gs := range groups {
		for _, g := range gs {
			if _, at := activeSeconds(g, window); at != outside {
				return false
			}
		}
	}
	return true
}

// selectSegments returns the one report per segment that counts and the reports a
// reconciliation rejected, or a conflict reason. Without a reconciliation every segment must
// have exactly one distinct report; with one, exactly the accepted (id, revision) reports
// count and the rest are rejected. The caller lists the rejected reports only when the
// invocation is not a conflict, so no report is listed twice.
func selectSegments(groups map[string][]Segment, reconciled map[Invocation]Reconciliation, inv Invocation) ([]Segment, []Entry, string) {
	ids := make([]string, 0, len(groups))
	for id := range groups {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	r, ok := reconciled[inv]
	if !ok {
		var out []Segment
		for _, id := range ids {
			if len(groups[id]) != 1 {
				return nil, nil, "conflicting reports of segment " + id + " need explicit reconciliation"
			}
			out = append(out, groups[id][0])
		}
		return out, nil, ""
	}
	accept := map[string]string{}
	for _, ref := range r.Accept {
		accept[ref.ID] = ref.Revision
	}
	var out []Segment
	var rejected []Entry
	found := map[SegmentRef]bool{}
	for _, id := range ids {
		var match []Segment
		for _, g := range groups[id] {
			if rev, ok := accept[id]; ok && g.Revision == rev {
				match = append(match, g)
				continue
			}
			rejected = append(rejected, Entry{"segment", segKey(g), "not accepted by reconciliation"})
		}
		if len(match) > 1 {
			return nil, nil, "reconciliation accepts more than one distinct report of segment " + id
		}
		for _, g := range match {
			found[SegmentRef{g.ID, g.Revision}] = true
		}
		out = append(out, match...)
	}
	for _, ref := range r.Accept {
		if !found[ref] {
			return nil, nil, "reconciliation accepts an unreported segment " + ref.ID + "@" + ref.Revision
		}
	}
	return out, rejected, ""
}

// overlap refuses two counted segments of one invocation whose active intervals overlap. It
// sorts the intervals by start once and sweeps them, so its work grows as n log n.
func overlap(segments []Segment) string {
	type span struct {
		start, end time.Time
		id         string
	}
	spans := make([]span, 0, len(segments))
	for _, g := range segments {
		if g.Active == nil {
			continue
		}
		s, e, _ := bounds(*g.Active)
		spans = append(spans, span{s, e, g.ID})
	}
	sort.SliceStable(spans, func(i, j int) bool { return spans[i].start.Before(spans[j].start) })
	var reach span // the swept interval that ends last
	for k, sp := range spans {
		if k > 0 && sp.start.Before(reach.end) {
			return "segments " + reach.id + " and " + sp.id + " overlap; overlapping intervals need explicit reconciliation"
		}
		if k == 0 || sp.end.After(reach.end) {
			reach = sp
		}
	}
	return ""
}

// Where a segment's active interval lies against the policy window.
const (
	inside     = iota // wholly inside: its active seconds are known
	unmeasured        // no interval was measured: not placed against the window
	straddles         // crosses a window boundary: the window cannot apportion it
	outside           // disjoint from the window: excluded
)

// activeSeconds places a segment's active interval against the window and, when it lies
// inside, returns its exact length in seconds.
func activeSeconds(g Segment, window Interval) (int64, int) {
	if g.Active == nil {
		return 0, unmeasured
	}
	ws, we, _ := bounds(window)
	s, e, _ := bounds(*g.Active)
	switch {
	case !s.Before(we) || !e.After(ws):
		return 0, outside
	case s.Before(ws) || e.After(we):
		return 0, straddles
	}
	secs, _ := subInt64(e.Unix(), s.Unix())
	return secs, inside
}

func measureDecisions(s Snapshot, l *Ledger, bucketOf func(*Subject) string, record recordFunc) {
	reports := map[string][]Decision{}
	var order []string
	seen := map[string]bool{}
	for _, d := range s.Decisions {
		enc := encode(d)
		if seen[enc] {
			l.Duplicates = append(l.Duplicates, Entry{"decision", d.ID, "duplicate report counted once"})
			continue
		}
		seen[enc] = true
		if reports[d.ID] == nil {
			order = append(order, d.ID)
		}
		reports[d.ID] = append(reports[d.ID], d)
	}
	sort.Strings(order)
	for _, id := range order {
		ds := reports[id]
		if len(ds) > 1 && decisionsOutside(ds, l.Window) {
			l.Excluded = append(l.Excluded, Entry{"decision", id, "every conflicting report's declared window lies outside the policy window"})
			continue
		}
		if len(ds) > 1 {
			l.Conflicts = append(l.Conflicts, Entry{"decision", id, "conflicting reports of one decision identity"})
			bucket := bucketOf(ds[0].Primary)
			units := map[string]bool{}
			for _, d := range ds[1:] {
				if bucketOf(d.Primary) != bucket {
					bucket = ""
				}
			}
			for _, d := range ds {
				if !units[d.Unit] {
					units[d.Unit] = true
					record(bucket, false, humanMeasure(d.Unit), nil)
				}
			}
			continue
		}
		d := ds[0]
		ws, we, _ := bounds(l.Window)
		start, end, _ := bounds(d.Window)
		var v *int64
		switch {
		case !start.Before(we) || !end.After(ws):
			l.Excluded = append(l.Excluded, Entry{"decision", d.ID, "declared window outside the policy window"})
			continue
		case start.Before(ws) || end.After(we):
			// A declared window straddling the policy window cannot be attributed to it.
		case d.Unit == UnitDecision:
			one := int64(1)
			v = &one
		default:
			v = d.Quantity
		}
		record(bucketOf(d.Primary), false, humanMeasure(d.Unit), v)
	}
}

// decisionsOutside reports whether every report's declared window lies entirely outside the
// policy window.
func decisionsOutside(ds []Decision, window Interval) bool {
	ws, we, _ := bounds(window)
	for _, d := range ds {
		start, end, _ := bounds(d.Window)
		if start.Before(we) && end.After(ws) {
			return false
		}
	}
	return true
}

func deref(v *int64) int64 {
	if v == nil {
		return 0
	}
	return *v
}

func humanMeasure(unit string) string {
	if unit == UnitMinute {
		return DecisionMinutes
	}
	return Decisions
}

func agreedBucket(groups map[string][]Segment, bucketOf func(*Subject) string) string {
	bucket, first := "", true
	for _, gs := range groups {
		for _, g := range gs {
			b := bucketOf(g.Subject)
			if first {
				bucket, first = b, false
			} else if b != bucket {
				return ""
			}
		}
	}
	return bucket
}

func segKey(g Segment) string {
	return g.Invocation.Caller + "/" + g.Invocation.ID + "/" + g.ID + "@" + g.Revision
}

func encode(v any) string {
	data, _ := json.Marshal(v)
	return string(data)
}

// Share is one bucket's observed share of a measure's scope total, as an exact fraction.
// It is defined only when every record behind the total is known and the total is not zero.
type Share struct {
	Measure     string `json:"measure"`
	Numerator   int64  `json:"numerator"`
	Denominator int64  `json:"denominator"`
	Defined     bool   `json:"defined"`
	Reason      string `json:"reason,omitempty"`
}

// ShareOf returns bucket's share of measure (an agent measure when agent is true).
func (l Ledger) ShareOf(bucket string, agent bool, measure string) Share {
	pick := func(t Tally) Quantity {
		if agent {
			return t.Agent[measure]
		}
		return t.Human[measure]
	}
	total := pick(l.Total)
	sh := Share{Measure: measure, Numerator: pick(l.Buckets[bucket]).Known, Denominator: total.Known}
	switch {
	case !total.Complete():
		sh.Reason = "unknown coverage: the total has unknown records"
	case total.Known == 0:
		sh.Reason = "zero total: shares are undefined"
	default:
		sh.Defined = true
	}
	if !sh.Defined {
		sh.Numerator, sh.Denominator = 0, 0
	}
	return sh
}
