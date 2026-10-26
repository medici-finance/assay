package contexthandoff

import (
	"reflect"
	"sort"
	"strings"
)

// Finding classes of a recheck.
const (
	FindingPreserved = "preserved" // an input changed, and no conclusion depends on it
	FindingReconcile = "reconcile" // a conclusion depends on a changed input: reconcile before acting
	FindingHold      = "hold"      // an input is unavailable or coverage is partial: could-not-check, scoped to it
	FindingIgnored   = "ignored"   // an input that cannot affect approved work (an unapproved rival)
	// FindingAssess (product.go) is a product-binding change for the deciding
	// role to assess; it neither reconciles nor holds.
)

// Finding is one recheck conclusion, scoped to the input it concerns.
type Finding struct {
	Scope  string `json:"scope"`
	Class  string `json:"class"`
	Reason string `json:"reason"`
}

func (f Finding) line() string { return f.Class + ": " + f.Scope + ": " + f.Reason }

// RecheckResult is the merged receipt over the new snapshot and what changed.
type RecheckResult struct {
	Receipt  *Handoff  `json:"receipt"`
	Findings []Finding `json:"findings"`
}

// Reconcile reports whether any finding requires reconciliation.
func (r *RecheckResult) Reconcile() bool { return r.count(FindingReconcile) > 0 }

// Hold reports whether any input was unavailable or either receipt's
// coverage is partial (a source it could not read, a query truncated at the
// result limit). A recheck that holds is never reported current.
func (r *RecheckResult) Hold() bool { return r.count(FindingHold) > 0 }

func (r *RecheckResult) count(class string) int {
	n := 0
	for _, f := range r.Findings {
		if f.Class == class {
			n++
		}
	}
	return n
}

// Recheck re-runs discovery over a new snapshot and compares it with a prior
// receipt. It does not trust previously read file hashes alone: a consumer
// added outside the prior read set, a changed guideline, a changed approved
// proposal or a changed declared owner, version or replacement each force
// reconciliation even when every previously read file is unchanged. An edit no
// conclusion depends on preserves the prior conclusions, and an unavailable
// input is a hold scoped to that input. Partial coverage on either side (a
// source the prior receipt could not read, a query truncated at the result
// limit) is a hold too, so a truncated receipt never reads as current. Over an
// unchanged snapshot with complete coverage the merged receipt reproduces the
// prior digest.
func Recheck(prior *Handoff, s *Snapshot) (*RecheckResult, error) {
	ix, err := newIndex(s)
	if err != nil {
		return nil, err
	}
	fresh, err := ix.discover()
	if err != nil {
		return nil, err
	}
	if prior.DeclaredBy != fresh.DeclaredBy {
		return nil, snapshotErr("prior receipt is declared by %s, the snapshot by %s", prior.DeclaredBy, fresh.DeclaredBy)
	}
	if prior.Product != nil && fresh.Product == nil {
		return nil, snapshotErr("the prior receipt carries product bindings but the snapshot supplies no product input: they cannot be rechecked, and dropping them would lose their dispositions")
	}
	c := &comparison{ix: ix, prior: prior, fresh: fresh, touched: map[string]bool{}}
	c.compare()
	c.compareProduct()
	receipt := c.merge()
	c.sortFindings()
	return &RecheckResult{Receipt: receipt, Findings: c.findings}, nil
}

type comparison struct {
	ix       *index
	prior    *Handoff
	fresh    *Handoff
	findings []Finding
	touched  map[string]bool // subjects needing reconciliation
	held     map[string]bool // prior subjects in unavailable sources
	explains map[string]bool // read-set locators some finding already accounts for
}

func (c *comparison) add(scope, class, reason string) {
	c.findings = append(c.findings, Finding{Scope: scope, Class: class, Reason: reason})
}

func queryResults(h *Handoff, rule string) []string {
	for _, q := range h.Discovery.Queries {
		if q.Rule == rule {
			return q.Results
		}
	}
	return nil
}

// splitAt splits "<locator>@<revision>" at its last '@'.
func splitAt(entry string) (string, string) {
	i := strings.LastIndexByte(entry, '@')
	if i < 0 {
		return entry, ""
	}
	return entry[:i], entry[i+1:]
}

func atMap(entries []string) map[string]string {
	m := map[string]string{}
	for _, e := range entries {
		k, v := splitAt(e)
		m[k] = v
	}
	return m
}

func (c *comparison) compare() {
	c.held = map[string]bool{}
	c.explains = map[string]bool{}
	ix := c.ix

	// Unavailable inputs: a scoped hold, and the prior conclusions inside them
	// are kept unconfirmed rather than dropped or re-derived from nothing.
	for _, src := range ix.s.Sources {
		if src.Unavailable {
			c.add("source:"+src.ID, FindingHold, unavailableReason)
		}
	}
	for _, im := range c.prior.Impacts {
		if ix.unavailable(im.Subject) {
			c.held[im.Subject] = true
		}
	}
	c.coverageHolds()

	if !reflect.DeepEqual(c.prior.Target, c.fresh.Target) {
		c.add("target", FindingReconcile, "the selected target, its approval or its transition changed")
		for _, im := range c.fresh.Impacts {
			c.touched[im.Subject] = true
		}
	}

	c.compareEntries(QueryGuidelines, "guideline:", "guideline")
	c.compareEntries(QueryProposals, "proposal:", "approved proposal")
	for _, p := range ix.s.Proposals {
		if p.Handoff.Target.State != TargetApproved {
			c.add("proposal:"+escapePath(p.Locator), FindingIgnored, "an unapproved rival proposal cannot invalidate approved work")
		}
	}

	priorEdges, freshEdges := edgesBySubject(queryResults(c.prior, QueryDeclaredEdge)), edgesBySubject(queryResults(c.fresh, QueryDeclaredEdge))
	for _, subj := range unionKeys(priorEdges, freshEdges) {
		if !reflect.DeepEqual(priorEdges[subj], freshEdges[subj]) {
			c.touched[subj] = true
			c.add("subject:"+subj, FindingReconcile, "a declared semantic owner, producer version or planned replacement changed")
		}
	}

	priorRead := atMap(queryResults(c.prior, QueryReadSet))
	freshRead := atMap(queryResults(c.fresh, QueryReadSet))
	priorCut, freshCut := readSetTruncated(c.prior), readSetTruncated(c.fresh)
	priorImp, freshImp := impactsBySubject(c.prior), impactsBySubject(c.fresh)
	for _, subj := range unionKeys(priorImp, freshImp) {
		p, inPrior := priorImp[subj]
		f, inFresh := freshImp[subj]
		b := ix.briefs[subj]
		loc := ""
		if b != nil {
			loc = b.locator("")
			c.explains[loc] = true
		}
		switch {
		case c.held[subj]:
			// a prior conclusion inside an unavailable source: kept as a hold
			// above, never re-derived from what this run could not read
		case !inPrior:
			reason := "new consumer"
			if _, read := priorRead[loc]; loc == "" || (!read && !priorCut) {
				reason += " outside the prior read set"
			}
			c.touched[subj] = true
			c.add("subject:"+subj, FindingReconcile, reason)
		case !inFresh:
			c.touched[subj] = true
			c.add("subject:"+subj, FindingReconcile, "consumer no longer found")
		case p.Relationship != f.Relationship || p.Lifecycle != f.Lifecycle || !reflect.DeepEqual(p.Evidence, f.Evidence):
			c.touched[subj] = true
			c.add("subject:"+subj, FindingReconcile, "relationship, lifecycle or evidence changed")
		case b != nil:
			c.compareBrief(subj, b, priorRead, priorCut)
		}
	}

	priorProse := map[string]bool{}
	for _, r := range queryResults(c.prior, QueryProse) {
		priorProse[r] = true
	}
	for _, r := range queryResults(c.fresh, QueryProse) {
		if !priorProse[r] && freshImp[r].Subject == "" {
			c.explains[r] = true
			reason := "new prose candidate"
			if _, read := priorRead[r]; !read && !priorCut {
				reason += " outside the prior read set"
			}
			c.add("prose:"+r, FindingReconcile, reason)
		}
	}

	if !reflect.DeepEqual(c.prior.Discovery.DependencySlice, c.fresh.Discovery.DependencySlice) {
		c.add("dependency-slice", FindingReconcile, "the declaring brief's dependency slice changed")
	}

	for _, loc := range unionKeys(priorRead, freshRead) {
		pv, inP := priorRead[loc]
		fv, inF := freshRead[loc]
		if pv == fv || c.explains[loc] || c.inUnavailable(loc) || (!inP && priorCut) || (!inF && freshCut) {
			// unchanged, accounted for, unreadable, or cut from a truncated
			// read set (which the coverage hold already covers)
			continue
		}
		c.add("file:"+loc, FindingPreserved, "unrelated edit: no conclusion depends on it")
	}
}

// coverageHolds raises a hold for every way either receipt's coverage falls
// short of complete: a query truncated at the result limit, prior or new, and
// a source the prior receipt could not read. A source unavailable to this run
// already holds, scoped to that source. Partial coverage is never reported as
// complete, so it is never reported current.
func (c *comparison) coverageHolds() {
	for _, side := range []struct {
		name string
		h    *Handoff
	}{{"prior", c.prior}, {"snapshot", c.fresh}} {
		n := 0
		for _, q := range side.h.Discovery.Queries {
			if q.Truncated {
				n++
				c.add("coverage:"+side.name+":"+q.Rule, FindingHold, "query truncated at the result limit: coverage is partial, so the receipt cannot be confirmed current")
			}
		}
		cov := side.h.Discovery.Coverage
		if side.name == "prior" {
			for _, u := range cov.Unavailable {
				n++
				c.add("coverage:prior:source:"+u.Source, FindingHold, "the prior receipt could not read this source, so its conclusions there were never checked: could-not-check")
			}
		} else {
			n += len(cov.Unavailable) // each already holds as source:<id>
		}
		if n == 0 && cov.State != "complete" {
			c.add("coverage:"+side.name, FindingHold, "coverage is "+cov.State+", not complete: could-not-check")
		}
	}
}

// compareBrief compares an impacted brief's revision in the prior receipt with
// its revision in this snapshot. The new revision comes from the brief itself,
// never from the new read set, so a truncated read set cannot hide an edit or
// invent one. When the prior read set was truncated before the brief, its
// prior revision is unknown: a hold scoped to the subject, never a match.
func (c *comparison) compareBrief(subj string, b *brief, priorRead map[string]string, priorCut bool) {
	prev, read := priorRead[b.locator("")]
	switch {
	case read && prev == BlobRevision(b.Raw):
	case !read && priorCut:
		c.touched[subj] = true
		c.add("subject:"+subj, FindingHold, "the prior receipt's read set was truncated before this brief, so its revision cannot be compared: could-not-check")
	default:
		c.touched[subj] = true
		c.add("subject:"+subj, FindingReconcile, "the brief changed since the prior receipt")
	}
}

func readSetTruncated(h *Handoff) bool {
	for _, q := range h.Discovery.Queries {
		if q.Rule == QueryReadSet {
			return q.Truncated
		}
	}
	return false
}

func (c *comparison) sortFindings() {
	sort.SliceStable(c.findings, func(i, j int) bool {
		if c.findings[i].Class != c.findings[j].Class {
			return classOrder[c.findings[i].Class] < classOrder[c.findings[j].Class]
		}
		return c.findings[i].Scope < c.findings[j].Scope
	})
}

var classOrder = map[string]int{FindingHold: 0, FindingReconcile: 1, FindingAssess: 2, FindingPreserved: 3, FindingIgnored: 4}

func (c *comparison) inUnavailable(loc string) bool {
	id, _, _ := strings.Cut(loc, ":")
	for _, src := range c.ix.s.Sources {
		if src.ID == id {
			return src.Unavailable
		}
	}
	return false
}

// compareEntries compares a "<locator>@<revision>" query between receipts:
// every added, changed or removed entry forces reconciliation.
func (c *comparison) compareEntries(rule, prefix, what string) {
	p, f := atMap(queryResults(c.prior, rule)), atMap(queryResults(c.fresh, rule))
	for _, k := range unionKeys(p, f) {
		c.explains[k] = true
		pv, inP := p[k]
		fv, inF := f[k]
		switch {
		case !inP:
			if rule == QueryGuidelines && c.inUnavailable(k) {
				continue
			}
			c.add(prefix+k, FindingReconcile, what+" added")
		case !inF:
			if c.inUnavailable(k) {
				continue
			}
			c.add(prefix+k, FindingReconcile, what+" removed")
		case pv != fv:
			c.add(prefix+k, FindingReconcile, what+" changed")
		default:
			continue
		}
		if rule == QueryGuidelines || rule == QueryProposals {
			for _, im := range c.fresh.Impacts {
				c.touched[im.Subject] = true
			}
		}
	}
}

func edgesBySubject(results []string) map[string][]string {
	m := map[string][]string{}
	for _, r := range results {
		subj, _, _ := strings.Cut(r, ";")
		m[subj] = append(m[subj], r)
	}
	return m
}

func impactsBySubject(h *Handoff) map[string]Impact {
	m := map[string]Impact{}
	for _, im := range h.Impacts {
		m[im.Subject] = im
	}
	return m
}

func unionKeys[A, B any](a map[string]A, b map[string]B) []string {
	k := map[string]bool{}
	for x := range a {
		k[x] = true
	}
	for x := range b {
		k[x] = true
	}
	return sortedKeys(k)
}

// merge builds the rechecked receipt: discovery, coverage and target from the
// new snapshot; for every subject no finding touches, the prior disposition,
// amendment, history and acknowledged active work carried over unchanged.
func (c *comparison) merge() *Handoff {
	h := *c.fresh
	priorImp := impactsBySubject(c.prior)
	preserved := func(subj string) bool {
		_, ok := priorImp[subj]
		return ok && !c.touched[subj] && !c.held[subj]
	}

	h.Impacts = nil
	seen := map[string]bool{}
	for _, im := range c.fresh.Impacts {
		seen[im.Subject] = true
		if preserved(im.Subject) {
			p := priorImp[im.Subject]
			im.Disposition, im.Reason = p.Disposition, p.Reason
		}
		h.Impacts = append(h.Impacts, im)
	}
	for _, p := range c.prior.Impacts {
		if c.held[p.Subject] && !seen[p.Subject] {
			p.Disposition = "unresolved"
			p.Reason = "hold: its source is unavailable; the prior conclusion cannot be rechecked"
			h.Impacts = append(h.Impacts, p)
		}
	}
	sort.SliceStable(h.Impacts, func(i, j int) bool { return h.Impacts[i].Subject < h.Impacts[j].Subject })

	h.Amendments = []Amendment{}
	for _, a := range c.prior.Amendments {
		if preserved(a.Subject) {
			h.Amendments = append(h.Amendments, a)
		}
	}

	h.History = []History{}
	for _, x := range c.prior.History {
		if preserved(x.Subject) || c.held[x.Subject] {
			h.History = append(h.History, x)
		}
	}
	for _, x := range c.fresh.History {
		if !preserved(x.Subject) {
			h.History = append(h.History, x)
		}
	}
	sort.SliceStable(h.History, func(i, j int) bool { return h.History[i].Subject < h.History[j].Subject })

	h.ActiveWork = []ActiveWork{}
	priorWork := map[string]ActiveWork{}
	for _, w := range c.prior.ActiveWork {
		priorWork[w.Subject+"\x00"+w.Claim+"\x00"+w.BriefRevision] = w
	}
	for _, w := range c.fresh.ActiveWork {
		if p, ok := priorWork[w.Subject+"\x00"+w.Claim+"\x00"+w.BriefRevision]; ok && preserved(w.Subject) {
			w = p
		} else if !ok {
			c.add("active-work:"+w.Subject, FindingReconcile, "a claim not bound in the prior receipt")
		}
		h.ActiveWork = append(h.ActiveWork, w)
	}

	var unresolved []string
	for _, im := range h.Impacts {
		if im.Disposition == "unresolved" {
			unresolved = append(unresolved, im.Subject)
		}
	}
	if unresolved == nil {
		unresolved = []string{}
	}
	h.Readiness.Unresolved = unresolved
	h.Product = c.mergeProduct()

	have := map[string]bool{}
	var pre []string
	push := func(l string) {
		if !have[l] {
			have[l] = true
			pre = append(pre, l)
		}
	}
	// The scope line and the product lines come from the merged product
	// section (with its carried dispositions); the prior receipt's are
	// superseded, never accumulated.
	for _, l := range withProductScope(c.fresh.Readiness.Prerequisites, h.Product) {
		push(l)
	}
	for _, l := range c.prior.Readiness.Prerequisites {
		if l == ArchitectureOnly || l == ProductAssessed || strings.HasPrefix(l, productLinePrefix) {
			continue
		}
		if !strings.HasPrefix(l, "reconcile: ") && !strings.HasPrefix(l, "hold: ") && !strings.HasPrefix(l, "partial: ") && !strings.HasPrefix(l, "duplicate: ") {
			push(l)
		}
	}
	for _, f := range c.findings {
		if f.Class == FindingReconcile || f.Class == FindingHold {
			push(f.line())
		}
	}
	h.Readiness.Prerequisites = pre

	act := nextAct(&h, len(unresolved) > 0)
	if len(c.touched) > 0 || hasClass(c.findings, FindingReconcile) {
		act = "explore"
	} else if act != "explore" && c.prior.Readiness.NextAct != "explore" && validAct(&h, c.prior.Readiness.NextAct) {
		act = c.prior.Readiness.NextAct
	}
	h.Readiness.NextAct = act

	out, err := roundTrip(&h)
	if err != nil {
		// A merge of two contract-valid receipts that the contract refuses is
		// a resolver defect; fail closed with the fresh receipt and say so.
		c.add("receipt", FindingReconcile, "the merged receipt did not validate; the fresh receipt is returned: "+err.Error())
		return c.fresh
	}
	return out
}

func hasClass(fs []Finding, class string) bool {
	for _, f := range fs {
		if f.Class == class {
			return true
		}
	}
	return false
}

func validAct(h *Handoff, act string) bool {
	switch act {
	case "approve":
		return h.Target.State != TargetApproved
	case "publish", "dispatch":
		return h.Target.State == TargetApproved && len(h.Readiness.Unresolved) == 0
	}
	return false
}
