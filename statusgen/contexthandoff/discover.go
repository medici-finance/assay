package contexthandoff

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// Discovery query rules. Each runs over the whole snapshot, independently of
// the others, so an identity one rule misses another can still find: a
// prose-only consumer with no typed edge and no README row is found by
// directory enumeration plus prose candidates.
const (
	QueryDeclaredEdge   = "declared-edge"         // semantic owners, producers, replacements the snapshot declares
	QueryPlanEdge       = "plan-edge"             // depends/unblocks frontmatter to or from the declaring brief
	QueryConsumersField = "consumers-field"       // consumers: entries naming a discovery term
	QueryDirectory      = "directory-enumeration" // every brief file, independent of README rows
	QueryProse          = "prose-candidate"       // term mentions in brief and plan prose
	QueryGuidelines     = "guideline-enumeration" // applicable engineering guidelines, by blob
	QueryProposals      = "approved-proposals"    // approved rival handoffs, by digest
	QueryReadSet        = "read-set"              // every file this run read, by blob
	QueryCoverageScope  = "coverage-scope"        // what kind of alignment the receipt covers
	queryVersion        = "v1"
	coverageScopeResult = "architecture"
	productScopeResult  = "product-binding"
)

// ArchitectureOnly is the prerequisite every receipt over a snapshot with no
// product input carries: an architecture receipt is never a product alignment
// result, and no product binding means unchecked, not aligned. A receipt with
// a product section carries ProductAssessed instead.
const ArchitectureOnly = "architecture-only coverage: product alignment unchecked (no product binding is unchecked, not aligned)"

const unavailableReason = "source unavailable to this run: could-not-check, scoped to this source"

// index is the resolver's view of one snapshot.
type index struct {
	s      *Snapshot
	decl   QID
	briefs map[string]*brief
	ids    []string // sorted brief identities
	files  []fileRef
	terms  []string
	dups   []string // "<id> <kept locator> <ignored locator>" for each brief id held by two files
}

type fileRef struct {
	src  *SnapshotSource
	path string
	raw  []byte
}

func (f fileRef) locator() string { return f.src.ID + ":" + escapePath(f.path) }

func newIndex(s *Snapshot) (*index, error) {
	decl, err := ResolveQualifiedID(s.DeclaredBy, nil)
	if err != nil {
		return nil, snapshotErr("declared_by: %v", err)
	}
	ix := &index{s: s, decl: decl, briefs: map[string]*brief{}}
	terms := map[string]bool{}
	for _, t := range s.Terms {
		terms[t] = true
	}
	for _, c := range s.Target.Contracts {
		terms[c.Name] = true
	}
	ix.terms = sortedKeys(terms)
	for i := range s.Sources {
		src := &s.Sources[i]
		if src.Unavailable {
			continue
		}
		lifecycles := map[string]map[string]string{}
		for _, p := range sortedKeys(src.Files) {
			ix.files = append(ix.files, fileRef{src: src, path: p, raw: src.Files[p]})
			if m := readmePathRe.FindStringSubmatch(p); m != nil && src.Role == RolePlan {
				lifecycles[m[1]] = readmeLifecycles(src.Files[p])
			}
		}
		if src.Role != RolePlan {
			continue
		}
		for _, p := range sortedKeys(src.Files) {
			m := briefPathRe.FindStringSubmatch(p)
			if m == nil {
				continue
			}
			b := parseBrief(src, p, src.Files[p], m[1], m[2])
			if kept, dup := ix.briefs[b.ID]; dup {
				// the first file in path order holds the identity; the
				// collision is recorded so it is visible on the receipt
				ix.dups = append(ix.dups, "duplicate: brief id "+b.ID+" is held by "+kept.locator("")+"; "+b.locator("")+" is ignored")
				continue
			}
			if lc, ok := lifecycles[m[1]][m[2]]; ok {
				b.Lifecycle = lc
			}
			ix.briefs[b.ID] = b
		}
	}
	ix.ids = sortedKeys(ix.briefs)
	return ix, nil
}

// sourceOf returns the snapshot source a brief identity belongs to, or nil.
func (ix *index) sourceOf(id string) *SnapshotSource {
	q, err := ResolveQualifiedID(id, nil)
	if err != nil {
		return nil
	}
	for i := range ix.s.Sources {
		src := &ix.s.Sources[i]
		if src.Role == RolePlan && src.Cell == q.Cell && src.Repo == q.Repo {
			return src
		}
	}
	return nil
}

func (ix *index) unavailable(id string) bool {
	src := ix.sourceOf(id)
	return src != nil && src.Unavailable
}

// route names where a new obligation on a delivered brief goes: a linked
// follow-on (an unfinished brief that depends on it, or that it unblocks),
// else an unfinished declared owner of a discovery term. "" means none.
func (ix *index) route(b *brief) (string, string) {
	var follow []string
	for _, id := range ix.ids {
		o := ix.briefs[id]
		if o.ID != b.ID && o.unfinished() && contains(o.Depends, b.ID) {
			follow = append(follow, o.ID)
		}
	}
	for _, id := range b.Unblocks {
		if o := ix.briefs[id]; o != nil && o.unfinished() {
			follow = append(follow, o.ID)
		}
	}
	if len(follow) > 0 {
		sort.Strings(follow)
		return follow[0], "linked follow-on"
	}
	var owners []string
	for _, e := range ix.s.Edges {
		if e.Edge == "owns" && e.Subject != b.ID {
			if o := ix.briefs[e.Subject]; o != nil && o.unfinished() {
				owners = append(owners, o.ID)
			}
		}
	}
	if len(owners) > 0 {
		sort.Strings(owners)
		return owners[0], "unfinished owner"
	}
	return "", ""
}

// capped applies the result limit: a query over it is truncated, never
// silently short.
func capped(rule string, params map[string]string, results []string, max int) Query {
	sort.Strings(results)
	results = dedupe(results)
	q := Query{Rule: rule, Version: queryVersion, Parameters: params, Results: results}
	if max > 0 && len(results) > max {
		q.Results, q.Truncated = results[:max], true
	}
	if q.Results == nil {
		q.Results = []string{}
	}
	if q.Parameters == nil {
		q.Parameters = map[string]string{}
	}
	return q
}

func dedupe(s []string) []string {
	out := s[:0]
	for i, v := range s {
		if i == 0 || v != s[i-1] {
			out = append(out, v)
		}
	}
	return out
}

// candidate is one impact found by a query, before disposition.
type candidate struct {
	subject string
	rel     Relationship
	rank    int // lower wins: declared edge, consumers field, plan edge, prose
}

// Discover resolves a snapshot into a discovery receipt: every owner and
// consumer the four discovery routes find, the limits and coverage of the run
// (an unavailable source is a scoped could-not-check that names no content),
// the protected history of each impacted brief, the observed claims on them,
// and the readiness those records support. It is deterministic: equal
// snapshots give byte-identical receipts.
func Discover(s *Snapshot) (*Handoff, error) {
	ix, err := newIndex(s)
	if err != nil {
		return nil, err
	}
	return ix.discover()
}

func (ix *index) discover() (*Handoff, error) {
	s := ix.s
	declID := ix.decl.String()
	max := s.Limits.orDefault().MaxResults
	cands := map[string]candidate{}
	offer := func(c candidate) {
		if c.subject == declID {
			return
		}
		if old, ok := cands[c.subject]; !ok || c.rank < old.rank {
			cands[c.subject] = c
		}
	}
	termParam := map[string]string{"terms": strings.Join(ix.terms, ",")}

	var res []string
	for _, e := range s.Edges {
		res = append(res, edgeResult(e))
		offer(candidate{subject: e.Subject, rank: 0, rel: Relationship{Basis: "declared", Edge: e.Edge, Locator: e.Locator}})
	}
	queries := []Query{capped(QueryDeclaredEdge, map[string]string{"source": "snapshot"}, res, max)}

	res = nil
	decl := ix.briefs[declID]
	for _, id := range ix.ids {
		b := ix.briefs[id]
		if contains(b.Depends, declID) {
			res = append(res, b.ID+";depends;"+declID)
			offer(candidate{subject: b.ID, rank: 2, rel: Relationship{Basis: "observed", Edge: "consumes", Locator: b.locator("depends")}})
		}
		if contains(b.Unblocks, declID) {
			res = append(res, b.ID+";unblocks;"+declID)
			offer(candidate{subject: b.ID, rank: 2, rel: Relationship{Basis: "observed", Edge: "provides", Locator: b.locator("unblocks")}})
		}
	}
	if decl != nil {
		for _, d := range decl.Depends {
			res = append(res, declID+";depends;"+d)
			offer(candidate{subject: d, rank: 2, rel: Relationship{Basis: "observed", Edge: "provides", Locator: decl.locator("depends")}})
		}
		for _, u := range decl.Unblocks {
			res = append(res, declID+";unblocks;"+u)
			offer(candidate{subject: u, rank: 2, rel: Relationship{Basis: "observed", Edge: "consumes", Locator: decl.locator("unblocks")}})
		}
	}
	queries = append(queries, capped(QueryPlanEdge, map[string]string{"edges": "depends,unblocks"}, res, max))

	res = nil
	for _, id := range ix.ids {
		b := ix.briefs[id]
		for _, c := range b.Consumers {
			if ix.mentions(c) {
				res = append(res, b.ID)
				offer(candidate{subject: b.ID, rank: 1, rel: Relationship{Basis: "declared", Edge: "changes", Locator: b.locator("consumers")}})
				break
			}
		}
	}
	queries = append(queries, capped(QueryConsumersField, termParam, res, max))
	queries = append(queries, capped(QueryDirectory, map[string]string{"pattern": "docs/streams/<stream>/brief-<NN>-*.md"}, append([]string(nil), ix.ids...), max))

	res = nil
	var proseBriefs []string
	for _, f := range ix.files {
		if f.src.Role != RolePlan || readmePathRe.MatchString(f.path) || !ix.mentions(string(f.raw)) {
			continue
		}
		if m := briefPathRe.FindStringSubmatch(f.path); m != nil {
			id := QID{Cell: f.src.Cell, Repo: f.src.Repo, Stream: m[1], NN: m[2]}.String()
			if b := ix.briefs[id]; b != nil && b.Path == f.path {
				res = append(res, id)
				proseBriefs = append(proseBriefs, id)
				continue
			}
		}
		res = append(res, f.locator())
	}
	prose := capped(QueryProse, termParam, res, max)
	queries = append(queries, prose)
	kept := map[string]bool{}
	for _, r := range prose.Results {
		kept[r] = true
	}
	for _, id := range proseBriefs {
		if kept[id] {
			offer(candidate{subject: id, rank: 3, rel: Relationship{Basis: "inferred", Edge: "consumes", Locator: ix.briefs[id].locator("")}})
		}
	}

	res = nil
	var readSet []string
	for _, f := range ix.files {
		entry := f.locator() + "@" + BlobRevision(f.raw)
		readSet = append(readSet, entry)
		if f.src.Role == RoleGuideline {
			res = append(res, entry)
		}
	}
	queries = append(queries, capped(QueryGuidelines, map[string]string{"role": RoleGuideline}, res, max))

	res = nil
	for _, p := range s.Proposals {
		if p.Handoff.Target.State == TargetApproved {
			res = append(res, escapePath(p.Locator)+"@"+p.Handoff.Digest())
		}
	}
	queries = append(queries, capped(QueryProposals, map[string]string{"state": TargetApproved}, res, max))
	queries = append(queries, capped(QueryReadSet, map[string]string{"files": "regular .md files under each available source"}, readSet, max))
	if s.Product == nil {
		queries = append(queries, capped(QueryCoverageScope, map[string]string{"scope": "architecture", "product-alignment": "unchecked"}, []string{coverageScopeResult}, max))
	} else {
		queries = append(queries, capped(QueryCoverageScope, map[string]string{"scope": "architecture,product-binding", "product-alignment": "assessed"}, []string{coverageScopeResult, productScopeResult}, max))
	}

	h := &Handoff{
		Schema:     HandoffSchema,
		DeclaredBy: declID,
		Target:     s.Target,
		Amendments: []Amendment{},
		History:    []History{},
		ActiveWork: []ActiveWork{},
	}
	if h.Target.Contracts == nil {
		h.Target.Contracts = []ContractChange{}
	}
	h.Discovery = ix.coverage(queries)

	var unresolved []string
	for _, id := range sortedKeys(cands) {
		im := ix.impact(cands[id])
		if im.Lifecycle == "withdrawn" {
			h.Schema = HandoffSchemaV2
		}
		if im.Disposition == "unresolved" {
			unresolved = append(unresolved, im.Subject)
		}
		h.Impacts = append(h.Impacts, im)
		h.History = append(h.History, ix.history(id)...)
	}
	if h.Impacts == nil {
		h.Impacts = []Impact{}
	}
	h.ActiveWork = ix.activeWork(h.Impacts)
	h.Discovery.DependencySlice = ix.dependencySlice()
	if h.Product = ix.product(h); h.Product != nil {
		h.Schema = HandoffSchemaV3
	}
	h.Readiness = ix.readiness(h, unresolved)
	return roundTrip(h)
}

// mentions reports whether text names any discovery term.
func (ix *index) mentions(text string) bool {
	for _, t := range ix.terms {
		if hasTerm(text, t) {
			return true
		}
	}
	return false
}

func edgeResult(e DeclaredEdge) string {
	v := e.Version
	if v == "" {
		v = "-"
	}
	return e.Subject + ";" + e.Edge + ";" + e.Term + ";" + e.Locator + "@" + v
}

// coverage records the source universe and what this run could and could not
// read. An unavailable source gets no revision and a reason that names nothing
// inside it.
func (ix *index) coverage(queries []Query) Discovery {
	d := Discovery{Queries: queries, Coverage: Coverage{State: "complete", Enumerated: []string{}, Unavailable: []Unavailable{}}}
	for _, src := range ix.s.Sources {
		rec := Source{ID: src.ID, Membership: "included", Reason: src.Role + " source in the authorized universe"}
		if src.Required {
			rec.Reason = "required " + rec.Reason
		}
		if src.Unavailable {
			d.Coverage.Unavailable = append(d.Coverage.Unavailable, Unavailable{Source: src.ID, Reason: unavailableReason})
			d.Coverage.State = "partial"
		} else {
			rev := sourceRevision(src.Files)
			rec.Revision = &rev
			d.Coverage.Enumerated = append(d.Coverage.Enumerated, src.ID)
		}
		d.Sources = append(d.Sources, rec)
	}
	if d.Sources == nil {
		d.Sources = []Source{}
	}
	for _, q := range queries {
		if q.Truncated {
			d.Coverage.State = "partial"
		}
	}
	return d
}

// sourceRevision is the content revision of one source: sha256 over its
// sorted (path, blob) pairs.
func sourceRevision(files map[string][]byte) string {
	var b strings.Builder
	for _, p := range sortedKeys(files) {
		b.WriteString(escapePath(p) + "\x00" + BlobRevision(files[p]) + "\n")
	}
	return sha256Hex([]byte(b.String()))
}

// impact gives one candidate its lifecycle, evidence and disposition. Only a
// delivered brief with a route is settled (retained, its obligation routed);
// every other impact is unresolved until an author decides it.
func (ix *index) impact(c candidate) Impact {
	im := Impact{Subject: c.subject, Lifecycle: "unknown", Relationship: c.rel, Disposition: "unresolved"}
	b := ix.briefs[c.subject]
	switch {
	case b == nil && ix.unavailable(c.subject):
		im.Reason = "found by " + c.rel.Basis + " " + c.rel.Edge + " edge; its source is unavailable: could-not-check"
		return im
	case b == nil:
		im.Reason = "found by " + c.rel.Basis + " " + c.rel.Edge + " edge; no brief with this identity is enumerated in the snapshot"
		return im
	}
	im.Lifecycle = b.Lifecycle
	if b.Evidence {
		loc := b.locator("evidence")
		im.Evidence = &loc
	}
	im.Reason = "found by " + c.rel.Basis + " " + c.rel.Edge + " edge; needs an author disposition"
	if b.delivered() {
		if to, how := ix.route(b); to != "" {
			im.Disposition = "retain"
			im.Reason = "delivered with populated Evidence: left unchanged; a new obligation routes to " + to + " (" + how + ")"
		} else {
			im.Reason = "delivered with populated Evidence and no unfinished owner or linked follow-on: a new follow-on brief is needed"
		}
	}
	return im
}

// history records the protected regions of an impacted brief, so a later
// check can prove an amendment left them in place.
func (ix *index) history(id string) []History {
	b := ix.briefs[id]
	if b == nil {
		return nil
	}
	var out []History
	seen := map[string]int{}
	for _, r := range protectedRegions(b.Raw) {
		anchor := r.anchor
		seen[anchor]++
		if n := seen[anchor]; n > 1 {
			anchor = fmt.Sprintf("%s-%d", anchor, n)
		}
		out = append(out, History{Kind: r.kind, Subject: id, Locator: b.locator(anchor), Digest: digest([]byte(r.text))})
	}
	return out
}

// activeWork binds every observed claim on an impacted brief. Discovery never
// acknowledges on the owner's behalf: each binding starts as reconcile.
func (ix *index) activeWork(impacts []Impact) []ActiveWork {
	subjects := map[string]bool{}
	for _, im := range impacts {
		subjects[im.Subject] = true
	}
	out := []ActiveWork{}
	for _, c := range ix.s.Claims {
		if subjects[c.Subject] {
			out = append(out, ActiveWork{Subject: c.Subject, Claim: c.Claim, BriefRevision: c.BriefRevision,
				ObservedAt: c.ObservedAt, Owner: c.Owner, Checkpoint: c.Checkpoint, Treatment: "reconcile"})
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Subject != out[j].Subject {
			return out[i].Subject < out[j].Subject
		}
		return out[i].Claim < out[j].Claim
	})
	return out
}

// dependencySlice is the declaring brief's transitive depends closure.
func (ix *index) dependencySlice() []string {
	seen := map[string]bool{}
	var walk func(id string)
	walk = func(id string) {
		b := ix.briefs[id]
		if b == nil {
			return
		}
		for _, d := range b.Depends {
			if !seen[d] {
				seen[d] = true
				walk(d)
			}
		}
	}
	walk(ix.decl.String())
	delete(seen, ix.decl.String())
	out := sortedKeys(seen)
	if out == nil {
		out = []string{}
	}
	return out
}

func (ix *index) readiness(h *Handoff, unresolved []string) Readiness {
	if unresolved == nil {
		unresolved = []string{}
	}
	rd := Readiness{Unresolved: unresolved, DecidingRole: ix.s.DecidingRole, Prerequisites: ix.prerequisites(h)}
	rd.Facts = Facts{SourceAvailable: len(h.Discovery.Coverage.Unavailable) == 0, ProposalApproved: h.Target.State == TargetApproved}
	rd.NextAct = nextAct(h, len(unresolved) > 0)
	return rd
}

// prerequisites are the discovery-derived lines every receipt over this
// snapshot carries.
func (ix *index) prerequisites(h *Handoff) []string {
	out := withProductScope(nil, h.Product)
	for _, src := range ix.s.Sources {
		if src.Unavailable {
			req := ""
			if src.Required {
				req = "required "
			}
			out = append(out, "hold: "+req+"source "+src.ID+" unavailable: could-not-check, scoped to this source")
		}
	}
	for _, q := range h.Discovery.Queries {
		if q.Truncated {
			out = append(out, "partial: query "+q.Rule+" truncated at the result limit")
		}
	}
	return append(out, ix.dups...)
}

func nextAct(h *Handoff, unresolved bool) string {
	switch {
	case unresolved || h.Discovery.Coverage.State != "complete":
		return "explore"
	case h.Target.State != TargetApproved:
		return "approve"
	}
	return "publish"
}

// roundTrip serializes a built receipt and decodes it with the statusgen/19
// reader, so every receipt the resolver emits is one the contract accepts.
func roundTrip(h *Handoff) (*Handoff, error) {
	raw, err := json.Marshal(h)
	if err != nil {
		return nil, err
	}
	out, err := DecodeHandoff(raw)
	if err != nil {
		return nil, fmt.Errorf("resolver produced a receipt the contract refuses: %w", err)
	}
	return out, nil
}
