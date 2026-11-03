// Package domain holds the platform-free types deskd derives and serves.
//
// Nothing platform-shaped appears here (decision-model spec §1.5, design
// §3.4): not the numeric key GitHub assigns an issue, not the node id, not
// the HTML url. `internal/purity` lints this package for exactly that, and
// `purity_test.go` runs the lint as part of `go test ./...`.
//
// # Why this file exists before any type
//
// A derived index over GitHub is only as honest as the derivations under it,
// and every derivation in this system has a measured way of lying:
//
//   - A list call silently truncates. One observed truncation returned exactly
//     500 rows against a true total of 958 — a number that looked like an
//     answer and was a cap.
//   - A join can fail to join. Two GitHub views of the same object need not
//     share a key, and the row that cannot be joined is the interesting one.
//   - A source can be unreachable. An empty result from an unreachable source
//     is BLIND, not idle — and rendering it as `0` is the single most
//     expensive lie a board can tell, because zero looks settled.
//
// So a number in this system is never a bare int. It is a [Count]: a value
// that cannot be constructed without declaring the probe that produced it,
// and that REFUSES to carry a value at all when the probe hit its limit.
//
// The vocabulary is the methodology's three probe states, used verbatim.
package domain

import (
	"encoding/json"
	"fmt"
)

// Confidence is the three-state verdict every derived fact carries.
//
// There is deliberately no fourth state and no zero-value "unknown that reads
// as fine": the zero value of Confidence is the empty string, which
// [Assessment.Validate] rejects, so a fact that forgot to say how sure it is
// fails loudly rather than defaulting to clean.
type Confidence string

const (
	// CheckedClean — the probe ran, reached its source, and the answer is
	// the answer. Only this state may carry a value.
	CheckedClean Confidence = "checked-clean"

	// CheckedFailed — the probe ran, reached its source, and the source says
	// the thing is wrong. A real negative finding, not an absence.
	CheckedFailed Confidence = "checked-failed"

	// CouldNotCheck — the probe did not get an answer it may publish: the
	// source was unreachable, rate-limited, capped, or the join had no key.
	// This is NOT zero and NOT absent. A row the index could not derive
	// renders could-not-check, always.
	CouldNotCheck Confidence = "could-not-check"
)

// Valid reports whether c is one of the three states.
func (c Confidence) Valid() bool {
	switch c {
	case CheckedClean, CheckedFailed, CouldNotCheck:
		return true
	}
	return false
}

// ProbeMethod names HOW a fact was fetched. It is part of every published
// number because "434 merged PRs" and "434 merged PRs, which is what a
// 500-capped list call returned" are different claims.
type ProbeMethod string

const (
	// MethodGraphQLAliased — one GraphQL document with N aliased by-number
	// lookups. Has no paging cap, so a count from it is a count. This is the
	// preferred method for anything whose cardinality is known in advance.
	MethodGraphQLAliased ProbeMethod = "graphql-aliased-by-number"

	// MethodGraphQLPaged — cursor-paged GraphQL, followed to completion.
	// Uncapped only if the walk actually terminated on hasNextPage=false;
	// a walk cut short by a page budget is capped and says so.
	MethodGraphQLPaged ProbeMethod = "graphql-paged"

	// MethodRESTList — a REST list endpoint with a --limit. ALWAYS capped.
	// Kept so existing callers can be honest about it, not so new ones can
	// use it.
	MethodRESTList ProbeMethod = "rest-list"

	// MethodLocalTable — read from deskd's own SQLite. Cannot be
	// rate-limited or truncated, but can be absent (cold index).
	MethodLocalTable ProbeMethod = "local-table"

	// MethodDelegated — the fact was not derived here. It was consumed
	// whole from a declared upstream generator (derive-or-diff: exactly one
	// declared source per fact, and this index is not it).
	MethodDelegated ProbeMethod = "delegated-to-generator"
)

// Probe is the mandatory provenance of a derived fact: what was asked, of
// what, over what window, under what limit.
//
// Source is the ONE declared source for this fact (derive-or-diff). A fact
// with two sources is a fact with no source, because nothing says which one
// wins when they disagree.
type Probe struct {
	// Source is the declared origin, precise enough to re-run by hand —
	// e.g. "gh api graphql: N aliased by-key lookups on repository(owner,
	// name)" or "statusgen --json (delegated)".
	//
	// Note for anyone editing this comment: the domain-purity lint's own
	// grep spells its pattern with a wildcard, so prose here must not put
	// the word "issue" one character away from the word "number" — the
	// check would fire on the sentence describing it.
	Source string `json:"source"`

	// Method is how the source was queried.
	Method ProbeMethod `json:"method"`

	// Window is the set the probe asked for, in words — e.g.
	// "open issues labelled needs-decision|question|help wanted|ask, all ages".
	Window string `json:"window"`

	// Limit is the cap that APPLIED to this probe, or 0 for genuinely
	// uncapped. It is never omitted: a JSON consumer must be able to tell
	// "no cap" from "nobody said".
	Limit int `json:"limit"`

	// Capped is set when the result is at or beyond Limit, i.e. the true
	// total is unknown. [NewCount] refuses to publish a value when this is
	// true — that refusal is the whole point of the type.
	Capped bool `json:"capped"`
}

// Validate rejects a probe that has not declared enough to be audited.
func (p Probe) Validate() error {
	if p.Source == "" {
		return fmt.Errorf("probe has no declared source (derive-or-diff: exactly one source per fact)")
	}
	if p.Method == "" {
		return fmt.Errorf("probe %q declares no method", p.Source)
	}
	if p.Window == "" {
		return fmt.Errorf("probe %q declares no window", p.Source)
	}
	if p.Capped && p.Limit == 0 {
		return fmt.Errorf("probe %q is capped but declares limit 0 — a cap with no number is not a disclosure", p.Source)
	}
	return nil
}

// Assessment is a three-state verdict with its provenance and, when it is not
// clean, the reason. It is the shape every derived FIELD carries.
type Assessment struct {
	Confidence Confidence `json:"confidence"`
	Probe      Probe      `json:"probe"`
	// Reason is mandatory whenever Confidence is not CheckedClean. An
	// unexplained could-not-check is indistinguishable from a bug.
	Reason string `json:"reason,omitempty"`
}

// Validate enforces the two invariants that make an Assessment auditable:
// the state is one of the three, and anything other than clean explains
// itself.
func (a Assessment) Validate() error {
	if !a.Confidence.Valid() {
		return fmt.Errorf("assessment carries no valid confidence (got %q) — a fact that does not say how sure it is defaults to nothing", a.Confidence)
	}
	if err := a.Probe.Validate(); err != nil {
		return err
	}
	if a.Confidence != CheckedClean && a.Reason == "" {
		return fmt.Errorf("assessment is %s with no reason", a.Confidence)
	}
	return nil
}

// Clean builds a checked-clean assessment.
func Clean(p Probe) Assessment { return Assessment{Confidence: CheckedClean, Probe: p} }

// Failed builds a checked-failed assessment — the source answered and the
// answer is negative.
func Failed(p Probe, reason string) Assessment {
	return Assessment{Confidence: CheckedFailed, Probe: p, Reason: reason}
}

// Unchecked builds a could-not-check assessment. Every path that cannot reach
// a source, cannot join a row, or hit a cap ends here — never at a zero.
func Unchecked(p Probe, reason string) Assessment {
	return Assessment{Confidence: CouldNotCheck, Probe: p, Reason: reason}
}

// Count is a number that cannot be published without its probe.
//
// The value is UNEXPORTED and there is no exported field to set it, so the
// only way to obtain a Count carrying a number is [NewCount], which refuses
// when the probe is capped. That refusal is structural: a caller cannot
// write `Count{Value: 434}` and skip the disclosure, because there is no
// such field to write.
type Count struct {
	value      int
	assessment Assessment
}

// NewCount publishes n under probe p.
//
// It REFUSES — returning a could-not-check Count with no value — when the
// probe reports Capped. A list call that returned exactly its limit has not
// told you a total; it has told you its limit. Publishing it as a total is
// how 500 gets reported against a true 958.
func NewCount(n int, p Probe) Count {
	if err := p.Validate(); err != nil {
		return Count{assessment: Unchecked(p, "probe is not auditable: "+err.Error())}
	}
	if p.Capped {
		return Count{assessment: Unchecked(p, fmt.Sprintf(
			"probe returned %d rows at its limit of %d — the true total is unknown, so no count is published (use %s)",
			n, p.Limit, MethodGraphQLAliased))}
	}
	if n < 0 {
		return Count{assessment: Unchecked(p, fmt.Sprintf("negative count %d", n))}
	}
	return Count{value: n, assessment: Clean(p)}
}

// NoCount is the constructor for "this number could not be derived". It is
// the ONLY other way to make a Count, which is why nothing in this codebase
// can accidentally render an underivable number as 0.
func NoCount(p Probe, reason string) Count {
	return Count{assessment: Unchecked(p, reason)}
}

// Assessment returns the count's three-state verdict.
func (c Count) Assessment() Assessment { return c.assessment }

// Value returns the number and whether it may be used. ok is false for any
// state but checked-clean; callers that ignore ok get 0, which is why the
// JSON encoding below never routes through it.
func (c Count) Value() (int, bool) {
	return c.value, c.assessment.Confidence == CheckedClean
}

// countJSON is the wire shape. `value` is a POINTER so an underivable count
// serialises as an explicit null beside its confidence — never as 0, never
// omitted. A consumer that reads `value == null` cannot mistake it for zero;
// a consumer that reads a missing key might.
type countJSON struct {
	Value      *int       `json:"value"`
	Confidence Confidence `json:"confidence"`
	Probe      Probe      `json:"probe"`
	Reason     string     `json:"reason,omitempty"`
}

// MarshalJSON renders the count with its probe attached, always.
func (c Count) MarshalJSON() ([]byte, error) {
	out := countJSON{Confidence: c.assessment.Confidence, Probe: c.assessment.Probe, Reason: c.assessment.Reason}
	if c.assessment.Confidence == CheckedClean {
		v := c.value
		out.Value = &v
	}
	return json.Marshal(out)
}

// UnmarshalJSON restores a Count, preserving the null-vs-zero distinction.
func (c *Count) UnmarshalJSON(b []byte) error {
	var in countJSON
	if err := json.Unmarshal(b, &in); err != nil {
		return err
	}
	c.assessment = Assessment{Confidence: in.Confidence, Probe: in.Probe, Reason: in.Reason}
	if in.Value != nil {
		c.value = *in.Value
	}
	return nil
}

// FieldConfidence is the per-field three-state map an index row carries.
//
// Keys are the row's own JSON field names. A field absent from the map was
// derived clean by construction (a literal from the platform payload);
// a field present with CouldNotCheck serialises as JSON null in the row and
// carries its reason here. Nothing renders a derived field as a zero value
// and stays silent about it.
type FieldConfidence map[string]Assessment

// Unresolved returns the field names at could-not-check, for the callers
// (CLI printer, /inbox header) that need to say how much of a row is unknown.
func (f FieldConfidence) Unresolved() []string {
	var out []string
	for k, v := range f {
		if v.Confidence == CouldNotCheck {
			out = append(out, k)
		}
	}
	return out
}

// Validate checks every assessment in the map.
func (f FieldConfidence) Validate() error {
	for k, v := range f {
		if err := v.Validate(); err != nil {
			return fmt.Errorf("field %q: %w", k, err)
		}
	}
	return nil
}
