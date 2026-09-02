// Package attribution implements M3 stage attribution (spec §6): it walks each
// traced defect's inducing change back through its provenance chain and names the
// STAGE the defect escaped at — spec / brief / implementation — or reports it
// untraceable when the chain is broken.
//
// The package is deliberately self-contained — no dependency on the qualgen
// command package (`package main`) — exactly as `qualgen/adapters` is. It CONSUMES
// brief-07's trace corpus through the on-disk `defects.jsonl` JSON contract (see
// Trace), never a Go import, so M3 stays a decoupled reader of an already-mined
// artifact and never re-mines history.
//
// Honest-claims discipline (spec §10): a stage is EVIDENCE-ASSEMBLED (the
// deterministic dossier, dossier.go), JUDGMENT-CLASSIFIED (the stage call,
// stage.go), and SPOT-AUDITED against that fixed dossier — never "measured." The
// three-state invariant (spec §3.2) governs every link and every call: a broken or
// missing chain link is emitted as `untraceable`/could-not-resolve, NEVER a silent
// stage guess.
//
// # Downstream seams (consumed by brief-12, kept stable)
//
// Two outputs of this package are a frozen interface for the M4 reflexivity joins:
//
//   - The per-stage defect LEDGER (ledger.go): one append-only file per defect under
//     the tracking root, plus a per-stage/per-stream/per-window Rollup. brief-12's
//     gate-yield and ritual joins read defect counts BY STAGE from it.
//   - The `review-escape` OVERLAY (ReviewEscape, stage.go): recorded for EVERY
//     defect, naming the review lanes that approved the inducing PR at head.
//     brief-12's gate-yield accounting attributes each escape to those lanes.
//
// Changing either shape later is a breaking change for brief-12.
package attribution

import (
	"fmt"
	"sort"
	"sync"
)

// LinkState is the three-state resolution outcome for one provenance chain link
// (spec §3.2). Every link a ProvenanceLinkage returns carries exactly one. The
// string values are the on-disk contract downstream readers key on.
type LinkState string

const (
	// LinkResolved — the link was followed and its target identified.
	LinkResolved LinkState = "resolved"
	// LinkAbsent — the link legitimately does not exist for this target (e.g. a
	// bare-git inducing commit with no issue reference). A measured absence,
	// distinct from could-not-resolve.
	LinkAbsent LinkState = "absent"
	// LinkCouldNotResolve — the link SHOULD exist but the run could not follow it
	// (the tracker was unreachable, the reference was malformed). Never rounded to
	// absent, never a silent skip.
	LinkCouldNotResolve LinkState = "could-not-resolve"
)

// LinkKind names the rung of the provenance chain a link occupies. The chain
// climbs from the concrete inducing change toward the governing requirement:
// pr -> issue -> brief -> stream -> spec/ruling. A target's provenance may reach
// only part way; unreached rungs are LinkAbsent or LinkCouldNotResolve, never
// omitted.
type LinkKind string

const (
	LinkKindPR     LinkKind = "pr"
	LinkKindIssue  LinkKind = "issue"
	LinkKindBrief  LinkKind = "brief"
	LinkKindStream LinkKind = "stream"
	LinkKindSpec   LinkKind = "spec"
	LinkKindRuling LinkKind = "ruling"
)

// ChainLink is one resolved (or unresolved) rung of an inducing change's
// provenance chain.
type ChainLink struct {
	Kind   LinkKind  `json:"kind"`
	Ref    string    `json:"ref,omitempty"`
	State  LinkState `json:"state"`
	Detail string    `json:"detail,omitempty"`
}

// Inducing identifies the inducing change a chain is resolved FOR, as it arrives
// from a brief-07 Trace: the commit SHA, its PR reference if the mine had PR
// metadata, and the commit/PR message text an adapter parses for references.
type Inducing struct {
	Commit  string `json:"commit"`
	PR      string `json:"pr,omitempty"`
	Message string `json:"message,omitempty"`
}

// Chain is the ordered provenance chain for one inducing change. Links are kept in
// climb order (pr first, requirement last). A Chain is never nil-linked: an adapter
// that cannot reach a rung records it as LinkAbsent / LinkCouldNotResolve.
type Chain struct {
	Inducing Inducing    `json:"inducing"`
	Links    []ChainLink `json:"links"`
}

// Broken reports whether the chain is broken for attribution purposes: it fails to
// resolve to at least an issue OR brief rung. A broken chain drives the classifier
// to `untraceable` (spec §3.2) — it is never a silent stage guess. An adapter that
// resolves the PR but finds NO issue/brief reference (LinkAbsent) is broken: the
// chain does not reach a requirement to attribute against. A LinkCouldNotResolve on
// any rung is likewise broken (the run could not read the chain).
func (c Chain) Broken() bool {
	reachedRequirement := false
	for _, l := range c.Links {
		if l.State == LinkCouldNotResolve {
			return true
		}
		if l.State == LinkResolved && (l.Kind == LinkKindIssue || l.Kind == LinkKindBrief) {
			reachedRequirement = true
		}
	}
	return !reachedRequirement
}

// Link returns the first link of the given kind, or ok=false if none is present.
func (c Chain) Link(kind LinkKind) (ChainLink, bool) {
	for _, l := range c.Links {
		if l.Kind == kind {
			return l, true
		}
	}
	return ChainLink{}, false
}

// ProvenanceLinkage is the pluggable adapter that resolves an inducing change's
// chain links (PR -> issue/brief -> stream -> spec/ruling) as far as the target's
// provenance permits (task item 1). It is the same shape of seam as quality/06's
// LinkageAdapter: the chain lives in a tracker and a repo's own conventions, not in
// a fixed algorithm, so resolving it is an adapter's job and never hardcoded.
//
// The generic commit->issue reference adapter (commitissue_adapter.go) ships as the
// reference implementation and is registered as the default; richer chains
// (brief/stream/spec rungs for a house that records them) register ADDITIONAL
// adapters via config — configuration, never new mining code.
type ProvenanceLinkage interface {
	// Resolve returns the ordered chain for one inducing change. It returns an
	// error only for an internal failure; an unreachable rung is reported IN the
	// chain as LinkCouldNotResolve, and a legitimately missing rung as LinkAbsent —
	// the three-state invariant lives in the link states, not in the error.
	Resolve(inducing Inducing) (Chain, error)
}

// registry holds the named provenance adapters. The default adapter is the one the
// dossier assembler uses when a caller supplies none; additional adapters are
// registered by name so a house wires a richer chain as configuration.
var registry = struct {
	sync.RWMutex
	byName      map[string]ProvenanceLinkage
	defaultName string
}{byName: map[string]ProvenanceLinkage{}}

// Register makes a provenance adapter available under name. Registering the empty
// name, a nil adapter, or a duplicate name is an error — a silent overwrite would
// let one house's config shadow another's default without a trace.
func Register(name string, adapter ProvenanceLinkage) error {
	if name == "" {
		return fmt.Errorf("attribution.Register: empty adapter name")
	}
	if adapter == nil {
		return fmt.Errorf("attribution.Register: nil adapter for %q", name)
	}
	registry.Lock()
	defer registry.Unlock()
	if _, dup := registry.byName[name]; dup {
		return fmt.Errorf("attribution.Register: adapter %q already registered", name)
	}
	registry.byName[name] = adapter
	return nil
}

// setDefault records name as the default adapter. name must already be registered.
func setDefault(name string) error {
	registry.Lock()
	defer registry.Unlock()
	if _, ok := registry.byName[name]; !ok {
		return fmt.Errorf("attribution.setDefault: adapter %q not registered", name)
	}
	registry.defaultName = name
	return nil
}

// Adapter returns the registered adapter of the given name.
func Adapter(name string) (ProvenanceLinkage, bool) {
	registry.RLock()
	defer registry.RUnlock()
	a, ok := registry.byName[name]
	return a, ok
}

// DefaultAdapter returns the default provenance adapter (the generic commit->issue
// reference adapter unless a config registered and selected another). ok is false
// when no default is set — a caller reads that as could-not-resolve, never as a
// pass.
func DefaultAdapter() (ProvenanceLinkage, bool) {
	registry.RLock()
	defer registry.RUnlock()
	if registry.defaultName == "" {
		return nil, false
	}
	a, ok := registry.byName[registry.defaultName]
	return a, ok
}

// RegisteredAdapters returns the registered adapter names, sorted — a deterministic
// listing for a config surface to present.
func RegisteredAdapters() []string {
	registry.RLock()
	defer registry.RUnlock()
	out := make([]string, 0, len(registry.byName))
	for n := range registry.byName {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}
