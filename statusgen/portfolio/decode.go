package portfolio

import (
	"encoding/json"
	"fmt"
	"regexp"
	"time"
	"unicode/utf8"

	"github.com/medici-finance/assay/deskcore/domain"
)

var (
	tokenRE = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:@/+-]{0,127}$`)
	repoRE  = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]*/[a-z0-9][a-z0-9._-]*$`)
	reqRE   = regexp.MustCompile(`^REQ-[a-z0-9][a-z0-9-]{8,18}[a-z0-9]$`)
	timeRE  = regexp.MustCompile(`^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}Z$`)
	textRE  = regexp.MustCompile(`[^ \t\r\n]`)
)

// Decode reads one portfolio-measurement-v1 document under its exact wire contract (no
// unknown, case-folded, duplicate or absent required key; nothing trailing) and validates it.
// Another version is refused by name; its payload is never evaluated.
func Decode(data []byte) (Snapshot, error) {
	if !utf8.Valid(data) {
		return Snapshot{}, fmt.Errorf("portfolio: invalid UTF-8")
	}
	var header struct {
		Schema string `json:"schema"`
	}
	if err := json.Unmarshal(data, &header); err != nil {
		return Snapshot{}, fmt.Errorf("portfolio: %w", err)
	}
	if header.Schema != Schema {
		return Snapshot{}, fmt.Errorf("portfolio: unsupported schema %q (supported: %s)", header.Schema, Schema)
	}
	var s Snapshot
	if err := domain.DecodeStrict(data, &s); err != nil {
		return Snapshot{}, fmt.Errorf("portfolio: %w", err)
	}
	if err := Validate(s); err != nil {
		return Snapshot{}, err
	}
	return s, nil
}

// Validate checks a snapshot's units, identities, bucket assignment and source coverage.
// It refuses rather than repairs: an ambiguous assignment or an undeclared source is an
// error, never a guess.
func Validate(s Snapshot) error {
	if s.Schema != Schema {
		return fmt.Errorf("portfolio: unsupported schema %q", s.Schema)
	}
	if err := validatePolicy(s.Policy); err != nil {
		return err
	}
	sources := set(s.Policy.Sources)
	for i, g := range s.Segments {
		at := fmt.Sprintf("segments[%d]", i)
		if !invocation(g.Invocation) || !isToken(g.ID) || !isToken(g.Revision) {
			return bad(at, "invocation, id or revision")
		}
		if !isToken(g.Profile.ID) || !isToken(g.Profile.Revision) || !isToken(g.Profile.Model) {
			return bad(at, "profile id, revision or pinned model")
		}
		if g.ActualModel != "" && !isToken(g.ActualModel) {
			return bad(at, "actual-model")
		}
		if !sources[g.Source] {
			return bad(at, "source (not covered by the policy's sources)")
		}
		if g.Subject != nil && !subject(*g.Subject) {
			return bad(at, "subject")
		}
		if g.Active != nil && !interval(*g.Active) {
			return bad(at, "active interval")
		}
		if (g.Elapsed != nil && *g.Elapsed < 0) || (g.Cost != nil && *g.Cost < 0) {
			return bad(at, "elapsed-seconds-proxy or cost-micros (negative)")
		}
	}
	for i, d := range s.Decisions {
		at := fmt.Sprintf("decisions[%d]", i)
		if !isToken(d.ID) || !interval(d.Window) {
			return bad(at, "id or window")
		}
		switch {
		case d.Unit == UnitDecision && d.Quantity == nil:
		case d.Unit == UnitMinute && d.Quantity != nil && *d.Quantity > 0:
		default:
			return bad(at, "unit/quantity (a decision counts one and carries no quantity; minutes carry a positive quantity)")
		}
		if !sources[d.Source] {
			return bad(at, "source (not covered by the policy's sources)")
		}
		if d.Primary != nil && !subject(*d.Primary) {
			return bad(at, "primary-subject")
		}
	}
	seen := map[Invocation]bool{}
	for i, r := range s.Reconciliations {
		at := fmt.Sprintf("reconciliations[%d]", i)
		if !invocation(r.Invocation) || seen[r.Invocation] || len(r.Accept) == 0 || !textRE.MatchString(r.Reason) || !sources[r.Source] {
			return bad(at, "invocation (once), accept, reason or source")
		}
		seen[r.Invocation] = true
		refs := map[string]bool{}
		for _, ref := range r.Accept {
			if !isToken(ref.ID) || !isToken(ref.Revision) || refs[ref.ID] {
				return bad(at, "accept (each segment id once, with a revision)")
			}
			refs[ref.ID] = true
		}
	}
	return nil
}

func validatePolicy(p Policy) error {
	if !isToken(p.ID) || !isToken(p.Revision) || !repoRE.MatchString(p.Scope) || !interval(p.Window) {
		return bad("policy", "id, revision, scope or window")
	}
	if len(p.Sources) == 0 || len(set(p.Sources)) != len(p.Sources) {
		return bad("policy", "sources (at least one, each once)")
	}
	for _, src := range p.Sources {
		if !isToken(src) {
			return bad("policy", "source")
		}
	}
	if !isToken(p.Weights.ID) || !isToken(p.Weights.Revision) {
		return bad("policy.weights", "id or revision")
	}
	models := map[string]bool{}
	for _, m := range p.Weights.Models {
		if !isToken(m.Model) || models[m.Model] || m.Milli < 0 {
			return bad("policy.weights", "model (each once) or milli")
		}
		models[m.Model] = true
	}
	if p.Tolerance < 0 || p.Tolerance > 10000 {
		return bad("policy", "tolerance-basis-points")
	}
	if len(p.Buckets) == 0 {
		return bad("policy", "buckets")
	}
	buckets, owner := map[string]bool{}, map[Subject]string{}
	var agentSum, humanSum int64
	for _, b := range p.Buckets {
		if !isToken(b.ID) || buckets[b.ID] || len(b.Members) == 0 {
			return bad("policy.buckets", "id (each once) or members")
		}
		buckets[b.ID] = true
		for _, m := range b.Members {
			if !subject(m) {
				return bad("policy.buckets."+b.ID, "member")
			}
			if prior, ok := owner[m]; ok {
				return fmt.Errorf("portfolio: %s %s is assigned to buckets %s and %s; assignment must be unambiguous", m.Repository, m.Requirement, prior, b.ID)
			}
			owner[m] = b.ID
		}
		if !member(b.AgentTarget.Measure, agentMeasures) || !member(b.HumanTarget.Measure, humanMeasures) {
			return bad("policy.buckets."+b.ID, "target measure (agent targets name an agent measure, human targets a human one)")
		}
		if !share(b.AgentTarget.BasisPoints) || !share(b.HumanTarget.BasisPoints) {
			return bad("policy.buckets."+b.ID, "target basis-points")
		}
		var okAgent, okHuman bool
		agentSum, okAgent = addInt64(agentSum, b.AgentTarget.BasisPoints)
		humanSum, okHuman = addInt64(humanSum, b.HumanTarget.BasisPoints)
		if !okAgent || !okHuman || agentSum > 10000 || humanSum > 10000 {
			return bad("policy.buckets", "targets (one party's shares exceed 10000 basis points)")
		}
	}
	limits := map[[2]string]bool{}
	for _, l := range p.Limits {
		key := [2]string{l.Bucket, l.Measure}
		if !buckets[l.Bucket] || !(member(l.Measure, agentMeasures) || member(l.Measure, humanMeasures)) || l.Max < 0 || !isToken(l.Authorization) || limits[key] {
			return bad("policy.limits", "bucket, measure, max, authorization or repeat")
		}
		limits[key] = true
	}
	return nil
}

func bad(at, what string) error { return fmt.Errorf("portfolio: %s: invalid or missing %s", at, what) }

func isToken(s string) bool { return tokenRE.MatchString(s) }

func share(bp int64) bool { return bp >= 0 && bp <= 10000 }

func invocation(i Invocation) bool { return isToken(i.Caller) && isToken(i.ID) }

func subject(s Subject) bool {
	return repoRE.MatchString(s.Repository) && reqRE.MatchString(s.Requirement)
}

func interval(i Interval) bool {
	start, end, ok := bounds(i)
	return ok && end.After(start)
}

func bounds(i Interval) (time.Time, time.Time, bool) {
	if !timeRE.MatchString(i.Start) || !timeRE.MatchString(i.End) {
		return time.Time{}, time.Time{}, false
	}
	start, err1 := time.Parse(time.RFC3339, i.Start)
	end, err2 := time.Parse(time.RFC3339, i.End)
	return start, end, err1 == nil && err2 == nil
}

func member(s string, of []string) bool {
	for _, x := range of {
		if s == x {
			return true
		}
	}
	return false
}

func set(xs []string) map[string]bool {
	out := map[string]bool{}
	for _, x := range xs {
		out[x] = true
	}
	return out
}
