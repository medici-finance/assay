package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strings"
	"time"

	"github.com/medici-finance/assay/statusgen/evaluation"
)

const ContextSchema = "evaluation-context-v1"

// Frontier commits an exact normalized observation set, including absence. Its
// name is a public semantic label; the digest never contains source credentials.
type Frontier struct {
	Name   string `json:"name"`
	Digest string `json:"digest"`
}

// Dependency includes absent graph members. Presence changes are input changes;
// a digest alone cannot establish that the inventory includes every member.
type Dependency struct {
	Work    string `json:"work"`
	Present bool   `json:"present"`
	Digest  string `json:"digest"`
}

// EvaluationContext is a derivation receipt, never a capability. All snapshots,
// digests, completeness and expiry observations come from trusted composition.
// Query and options are digests of normalized inputs, never arbitrary payloads.
type EvaluationContext struct {
	Schema            string              `json:"schema"`
	Requires          []string            `json:"requires"`
	Work              WorkIdentity        `json:"work"`
	Candidate         Candidate           `json:"candidate"`
	Acceptance        AcceptanceBinding   `json:"acceptance"`
	Sources           []Frontier          `json:"sources"`
	Facts             []Frontier          `json:"facts"`
	Evaluator         evaluation.Identity `json:"evaluator"`
	QueryDigest       string              `json:"query-digest"`
	OptionsDigest     string              `json:"options-digest"`
	Dependencies      []Dependency        `json:"dependencies"`
	DependencyDigest  string              `json:"dependency-digest"`
	Coverage          Completeness        `json:"coverage"`
	PolicyDigest      string              `json:"policy-digest"`
	EnvironmentDigest string              `json:"environment-digest"`
	AccessEpoch       uint64              `json:"access-epoch"`
	Generation        uint64              `json:"generation"`
	ObservedCutoff    string              `json:"observed-cutoff"`
	Bounds            []ValidityBound     `json:"bounds"`
}

// Bounds must explicitly name lease and access, even when another bound is
// earlier. There is no implicit infinite grant. Additional expiry is optional.
type ValidityBound struct {
	Kind      string `json:"kind"`
	ExpiresAt string `json:"expires-at"`
}

func contextHash(v any) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	var tree any
	dec := json.NewDecoder(strings.NewReader(string(b)))
	dec.UseNumber()
	if err = dec.Decode(&tree); err != nil {
		return "", err
	}
	b, err = json.Marshal(tree)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}

// DependencySetDigest canonicalizes the entire inventory, including absence.
// It does not license selective coverage or resolve dependencies.
func DependencySetDigest(deps []Dependency) (string, error) {
	if deps == nil {
		return "", unknown("context-membership-missing")
	}
	sorted := append([]Dependency{}, deps...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Work < sorted[j].Work })
	for i, d := range sorted {
		if _, err := qualified(d.Work); err != nil {
			return "", err
		}
		if !digestRE.MatchString(d.Digest) || (i > 0 && sorted[i-1].Work == d.Work) {
			return "", refuse("context-membership-shape")
		}
	}
	return contextHash(sorted)
}

func (c EvaluationContext) Validate() error {
	if c.Schema != ContextSchema {
		return unknown("context-version")
	}
	if len(c.Requires) != 1 {
		return unknown("context-requires-missing")
	}
	for _, r := range c.Requires {
		if r != "context@1" {
			return unknown("context-capability")
		}
	}
	if err := c.Work.Validate(); err != nil {
		return err
	}
	if err := c.Acceptance.Validate(c.Candidate); err != nil {
		return err
	}
	for _, set := range [][]Frontier{c.Sources, c.Facts} {
		if len(set) == 0 {
			return unknown("context-frontier-missing")
		}
		seen := map[string]bool{}
		for _, f := range set {
			if !nodeRE.MatchString(f.Name) || !digestRE.MatchString(f.Digest) || seen[f.Name] {
				return refuse("context-frontier-shape")
			}
			seen[f.Name] = true
		}
	}
	if c.Evaluator.Version == "" || !objectRE.MatchString(c.Evaluator.PolicyDigest) || len(c.Evaluator.PolicyDigest) != 64 {
		return unknown("context-evaluator-missing")
	}
	for _, d := range []string{c.QueryDigest, c.OptionsDigest, c.PolicyDigest, c.EnvironmentDigest, c.DependencyDigest} {
		if !digestRE.MatchString(d) {
			return unknown("context-digest-missing")
		}
	}
	d, err := DependencySetDigest(c.Dependencies)
	if err != nil {
		return err
	}
	if d != c.DependencyDigest {
		return refuse("context-membership-digest")
	}
	if c.Coverage != Present && c.Coverage != Missing && c.Coverage != CouldNotCheck {
		return unknown("context-coverage-state")
	}
	if c.AccessEpoch == 0 || c.Generation == 0 {
		return unknown("context-currency-missing")
	}
	if !timestamp(c.ObservedCutoff) {
		return unknown("context-cutoff-missing")
	}
	cutoff, _ := time.Parse(time.RFC3339, c.ObservedCutoff)
	seen := map[string]bool{}
	for _, b := range c.Bounds {
		if b.Kind != "lease" && b.Kind != "access" && b.Kind != "evaluation" {
			return unknown("context-bound-kind")
		}
		if seen[b.Kind] || !timestamp(b.ExpiresAt) {
			return refuse("context-bound-shape")
		}
		expiry, _ := time.Parse(time.RFC3339, b.ExpiresAt)
		if !expiry.After(cutoff) {
			return refuse("context-bound-before-cutoff")
		}
		seen[b.Kind] = true
	}
	if !seen["lease"] || !seen["access"] {
		return unknown("context-bound-missing")
	}
	return nil
}

// Fingerprint identifies immutable derivation inputs, excluding observation
// cutoff, expiry bounds, access epoch and reevaluation generation. Those remain
// mandatory separately checked currency; incomplete coverage cannot produce a
// reusable fingerprint. Set order has no meaning; duplicate members are refused.
func (c EvaluationContext) Fingerprint() (string, error) {
	if err := c.Validate(); err != nil {
		return "", err
	}
	if c.Coverage != Present {
		return "", unknown("context-coverage-incomplete")
	}
	c.Requires = append([]string{}, c.Requires...)
	sort.Strings(c.Requires)
	c.Acceptance.RequiredFacts = append([]string{}, c.Acceptance.RequiredFacts...)
	sort.Strings(c.Acceptance.RequiredFacts)
	c.Sources = append([]Frontier{}, c.Sources...)
	sort.Slice(c.Sources, func(i, j int) bool { return c.Sources[i].Name < c.Sources[j].Name })
	c.Facts = append([]Frontier{}, c.Facts...)
	sort.Slice(c.Facts, func(i, j int) bool { return c.Facts[i].Name < c.Facts[j].Name })
	c.Dependencies = append([]Dependency{}, c.Dependencies...)
	sort.Slice(c.Dependencies, func(i, j int) bool { return c.Dependencies[i].Work < c.Dependencies[j].Work })
	// Explicit immutable projection: currency cannot accidentally become part of
	// the derivation identity merely by adding another receipt field.
	return contextHash(map[string]any{
		"schema": c.Schema, "requires": c.Requires, "work": c.Work, "candidate": c.Candidate,
		"acceptance": c.Acceptance, "sources": c.Sources, "facts": c.Facts,
		"evaluator": c.Evaluator, "query-digest": c.QueryDigest, "options-digest": c.OptionsDigest,
		"dependencies": c.Dependencies, "dependency-digest": c.DependencyDigest,
		"coverage": c.Coverage, "policy-digest": c.PolicyDigest, "environment-digest": c.EnvironmentDigest,
	})
}

// Applicability reports computational reuse only. A reusable result still needs
// current caller authorization and an independent lease/access check at admission.
type Applicability struct {
	State  string `json:"state"`
	Reason string `json:"reason"`
}

// CheckApplicability reads no clock and authenticates nothing. current must be
// independently observed, never copied from the cached receipt as authority.
func CheckApplicability(cached, current EvaluationContext, observedNow string) Applicability {
	if !timestamp(observedNow) {
		return Applicability{"held", "context-time-unavailable"}
	}
	old, err := cached.Fingerprint()
	if err != nil {
		return Applicability{"held", "context-cached-incomplete"}
	}
	fresh, err := current.Fingerprint()
	if err != nil {
		return Applicability{"held", "context-current-incomplete"}
	}
	if old != fresh {
		return Applicability{"reevaluate", "context-inputs-changed"}
	}
	if cached.AccessEpoch != current.AccessEpoch || cached.Generation != current.Generation {
		return Applicability{"reevaluate", "context-currency-changed"}
	}
	now, _ := time.Parse(time.RFC3339, observedNow)
	// The validity window is the intersection of the cached receipt and the
	// independently observed current receipt. A renewed grant cannot erase an old
	// expiry, and a shortened/revoked current grant cannot inherit the old bound.
	for _, receipt := range []EvaluationContext{cached, current} {
		cutoff, _ := time.Parse(time.RFC3339, receipt.ObservedCutoff)
		if now.Before(cutoff) {
			return Applicability{"held", "context-time-before-cutoff"}
		}
		for _, b := range receipt.Bounds {
			expiry, _ := time.Parse(time.RFC3339, b.ExpiresAt)
			if !now.Before(expiry) {
				return Applicability{"reevaluate", "context-expired"}
			}
		}
	}

	return Applicability{"reusable", "context-unchanged"}
}
