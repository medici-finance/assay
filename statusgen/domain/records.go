// Package domain owns verification-domain-v1: pure, candidate-bound records and
// an adapter to the existing evaluation library. No result authorizes an effect,
// proves independence, records a human act, or advances a lifecycle.
package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/medici-finance/assay/statusgen/contexthandoff"
)

const Schema = "verification-domain-v1"

// Refusal carries a stable code and three-state outcome, never input values.
type Refusal struct {
	Code  string
	State string
}

func (r *Refusal) Error() string { return r.State + ": " + r.Code }
func refuse(code string) error   { return &Refusal{code, "checked-failed"} }
func unknown(code string) error  { return &Refusal{code, "could-not-check"} }

// WorkIdentity qualifies the existing work, instance, node and attempt IDs.
// Paths are deliberately absent. Graph-execution/09 owns the last three forms.
type WorkIdentity struct {
	Brief    string `json:"brief"`
	Instance string `json:"instance"`
	Node     string `json:"node"`
	Attempt  string `json:"attempt"`
}

func NewWork(ref, declaring, instance, node string, ordinal uint64) (WorkIdentity, error) {
	var base *contexthandoff.QID
	if declaring != "" {
		q, err := contexthandoff.ResolveQualifiedID(declaring, nil)
		if err != nil {
			return WorkIdentity{}, refuse("work-identity")
		}
		base = &q
	}
	q, err := contexthandoff.ResolveQualifiedID(ref, base)
	if err != nil {
		return WorkIdentity{}, refuse("work-identity")
	}
	w := WorkIdentity{q.String(), instance, instance + "." + node, instance + "." + node + ".a" + strconv.FormatUint(ordinal, 10)}
	return w, w.Validate()
}

var (
	instanceRE = regexp.MustCompile(`^inst-[a-z0-9][a-z0-9-]{2,62}$`)
	nodeRE     = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)
	ordinalRE  = regexp.MustCompile(`^[1-9][0-9]*$`)
	objectRE   = regexp.MustCompile(`^([0-9a-f]{40}|[0-9a-f]{64})$`)
	digestRE   = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
	repoRE     = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*/[A-Za-z0-9][A-Za-z0-9._-]*$`)
)

func (w WorkIdentity) Validate() error {
	if _, err := contexthandoff.ResolveQualifiedID(w.Brief, nil); err != nil {
		return refuse("work-identity")
	}
	if !instanceRE.MatchString(w.Instance) || !strings.HasPrefix(w.Node, w.Instance+".") || !nodeRE.MatchString(strings.TrimPrefix(w.Node, w.Instance+".")) {
		return refuse("instance-identity")
	}
	if !strings.HasPrefix(w.Attempt, w.Node+".a") || !ordinalRE.MatchString(strings.TrimPrefix(w.Attempt, w.Node+".a")) {
		return refuse("attempt-identity")
	}
	return nil
}

// LocatedWork is an adapter observation for detecting duplicate declarations.
// A renamed file uses the same Brief value; a path never becomes the key.
type LocatedWork struct {
	Path  string
	Brief string
}

func ValidateLocations(items []LocatedWork) error {
	seen := map[string]bool{}
	for _, item := range items {
		q, err := contexthandoff.ResolveQualifiedID(item.Brief, nil)
		if err != nil || item.Path == "" {
			return refuse("work-identity")
		}
		key := q.String()
		if seen[key] {
			return refuse("duplicate-work")
		}
		seen[key] = true
	}
	return nil
}

// Candidate names a frozen input. Branch names and paths are not subjects.
// Digests identify content, not authenticity, cleanliness or authorization.
type Candidate struct {
	Repository     string `json:"repository"`
	BaseCommit     string `json:"base-commit"`
	BaseTree       string `json:"base-tree"`
	Commit         string `json:"commit"`
	Tree           string `json:"tree"`
	ManifestDigest string `json:"manifest-digest"`
}

func NewCandidate(c Candidate) (Candidate, error) { return c, c.Validate() }
func (c Candidate) Validate() error {
	if !repoRE.MatchString(c.Repository) || !digestRE.MatchString(c.ManifestDigest) {
		return refuse("candidate-shape")
	}
	for _, oid := range []string{c.BaseCommit, c.BaseTree, c.Commit, c.Tree} {
		if !objectRE.MatchString(oid) {
			return refuse("candidate-object")
		}
	}
	if len(c.BaseCommit) != len(c.Commit) || len(c.BaseTree) != len(c.Commit) || len(c.Tree) != len(c.Commit) {
		return refuse("candidate-object-format")
	}
	return nil
}

// AcceptanceBinding is the accepted definition, not a verdict. Its digest is
// supplied from the authoritative definition; this package cannot authenticate it.
// RequiredFacts is fixed with the binding. coverage and eligibility are mandatory.
type AcceptanceBinding struct {
	Version       uint64    `json:"version"`
	Digest        string    `json:"digest"`
	Candidate     Candidate `json:"candidate"`
	RequiredFacts []string  `json:"required-facts"`
}

func BindAcceptance(c Candidate, version uint64, digest string, required []string) (AcceptanceBinding, error) {
	b := AcceptanceBinding{version, digest, c, append([]string(nil), required...)}
	return b, b.Validate(c)
}
func (b AcceptanceBinding) Validate(c Candidate) error {
	if err := b.Candidate.Validate(); err != nil {
		return err
	}
	if err := c.Validate(); err != nil {
		return err
	}
	if b.Candidate != c {
		return refuse("candidate-mismatch")
	}
	if b.Version == 0 || !digestRE.MatchString(b.Digest) {
		return refuse("acceptance-shape")
	}
	seen := map[string]bool{}
	for _, id := range b.RequiredFacts {
		if !nodeRE.MatchString(id) || seen[id] {
			return refuse("required-fact-set")
		}
		seen[id] = true
	}
	if !seen["coverage"] || !seen["eligibility"] {
		return refuse("required-fact-set")
	}
	return nil
}

type Completeness string

const (
	Present       Completeness = "present"
	Missing       Completeness = "missing"
	CouldNotCheck Completeness = "could-not-check"
)

type Fact struct {
	ID    string       `json:"id"`
	State Completeness `json:"state"`
}
type FactSet struct {
	ObservedAt string `json:"observed-at"`
	Entries    []Fact `json:"entries"`
}

func (f FactSet) Validate() error {
	if !timestamp(f.ObservedAt) || f.Entries == nil {
		return refuse("fact-set-shape")
	}
	seen := map[string]bool{}
	for _, fact := range f.Entries {
		if !nodeRE.MatchString(fact.ID) || seen[fact.ID] {
			return refuse("fact-identity")
		}
		seen[fact.ID] = true
		if fact.State != Present && fact.State != Missing && fact.State != CouldNotCheck {
			return unknown("fact-state")
		}
	}
	return nil
}

// Generation is reevaluation currency, independent of assignment generation.
// Historical records may be retained, but Assess and CheckCurrent require equality.
type Generation struct {
	Requested uint64 `json:"requested"`
	Recorded  uint64 `json:"recorded"`
}

func (g Generation) Validate() error {
	if g.Requested == 0 || g.Recorded == 0 {
		return refuse("generation-shape")
	}
	return nil
}
func (g Generation) CheckCurrent() error {
	if err := g.Validate(); err != nil {
		return err
	}
	if g.Recorded != g.Requested {
		return refuse("stale-generation")
	}
	return nil
}

type Components struct {
	Bind    string `json:"bind"`
	Admit   string `json:"admit"`
	Execute string `json:"execute"`
	Assess  string `json:"assess"`
	Decide  string `json:"decide"`
	Publish string `json:"publish"`
}

// Hold describes a resumable wait; it grants no routing or retry authority.
type Hold struct {
	Kind       string `json:"kind"`
	Owner      string `json:"owner"`
	Required   string `json:"required"`
	ObservedAt string `json:"observed-at"`
}
type Progress struct {
	Stage       string     `json:"stage"`
	Components  Components `json:"components"`
	Execution   string     `json:"execution"`
	Acceptance  string     `json:"acceptance"`
	Publication string     `json:"publication"`
	Retirement  string     `json:"retirement"`
	ObservedAt  string     `json:"observed-at"`
	Holds       []Hold     `json:"holds"`
}

var states = map[string][]string{
	"stage":       {"received", "bound", "admitted", "executing", "assessing", "decided", "publishing", "reconciling", "complete", "held", "cancelled", "superseded"},
	"component":   {"pending", "running", "complete", "held", "cancelled", "could-not-check"},
	"execution":   {"not-started", "running", "succeeded", "failed", "cancelled", "could-not-check"},
	"acceptance":  {"unaccepted", "held", "repair-required", "accepted"},
	"publication": {"not-requested", "pending", "confirmed", "unknown"},
	"retirement":  {"not-requested", "pending", "held", "retired"},
	"hold":        {"fact", "eligibility", "coverage", "authority", "human", "environment", "publication", "retirement"},
}

func known(kind, value string) bool {
	for _, s := range states[kind] {
		if value == s {
			return true
		}
	}
	return false
}
func timestamp(s string) bool {
	t, e := time.Parse(time.RFC3339, s)
	return e == nil && t.UTC().Format(time.RFC3339) == s
}
func (p Progress) Validate() error {
	for k, v := range map[string]string{"stage": p.Stage, "execution": p.Execution, "acceptance": p.Acceptance, "publication": p.Publication, "retirement": p.Retirement} {
		if !known(k, v) {
			return unknown("unsupported-" + k)
		}
	}
	for _, v := range []string{p.Components.Bind, p.Components.Admit, p.Components.Execute, p.Components.Assess, p.Components.Decide, p.Components.Publish} {
		if !known("component", v) {
			return unknown("unsupported-component")
		}
	}
	if !timestamp(p.ObservedAt) || p.Holds == nil {
		return refuse("progress-shape")
	}
	if p.Stage == "complete" && (p.Acceptance != "accepted" || p.Publication != "confirmed") {
		return refuse("completion-premature")
	}
	for _, h := range p.Holds {
		if !known("hold", h.Kind) {
			return unknown("unsupported-hold")
		}
		if strings.TrimSpace(h.Owner) == "" || strings.TrimSpace(h.Required) == "" || !timestamp(h.ObservedAt) {
			return refuse("hold-shape")
		}
	}
	if (p.Stage == "held" || p.Acceptance == "held" || p.Retirement == "held") && len(p.Holds) == 0 {
		return refuse("hold-missing")
	}
	return nil
}

type Record struct {
	Schema     string             `json:"schema"`
	Requires   []string           `json:"requires"`
	Work       WorkIdentity       `json:"work"`
	Candidate  Candidate          `json:"candidate"`
	Acceptance *AcceptanceBinding `json:"acceptance,omitempty"`
	Facts      FactSet            `json:"facts"`
	Generation Generation         `json:"generation"`
	Progress   Progress           `json:"progress"`
}

func NewRecord(r Record) (Record, error) { return r, r.Validate() }
func (r Record) Validate() error {
	if r.Schema != Schema {
		return unknown("unsupported-schema")
	}
	if r.Requires == nil {
		return refuse("requires-missing")
	}
	for _, v := range r.Requires {
		if v != "domain@1" {
			return unknown("unsupported-capability")
		}
	}
	for _, err := range []error{r.Work.Validate(), r.Candidate.Validate(), r.Facts.Validate(), r.Generation.Validate(), r.Progress.Validate()} {
		if err != nil {
			return err
		}
	}
	if r.Acceptance != nil {
		if err := r.Acceptance.Validate(r.Candidate); err != nil {
			return err
		}
	}
	if r.Progress.Acceptance == "accepted" && r.Acceptance == nil {
		return refuse("acceptance-missing")
	}
	return nil
}

// Canonical validates before writing deterministic JSON. Object keys are sorted;
// array order is preserved and integer values have decimal notation.
func (r Record) Canonical() ([]byte, error) {
	if err := r.Validate(); err != nil {
		return nil, err
	}
	b, err := json.Marshal(r)
	if err != nil {
		return nil, err
	}
	var tree any
	dec := json.NewDecoder(strings.NewReader(string(b)))
	dec.UseNumber()
	if err := dec.Decode(&tree); err != nil {
		return nil, err
	}
	return json.Marshal(tree)
}
func (r Record) Digest() (string, error) {
	b, e := r.Canonical()
	if e != nil {
		return "", e
	}
	sum := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}

func qualified(ref string) (contexthandoff.QID, error) {
	q, err := contexthandoff.ResolveQualifiedID(ref, nil)
	if err != nil {
		return q, refuse("work-identity")
	}
	return q, nil
}
