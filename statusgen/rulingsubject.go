package main

import (
	"fmt"
	"sort"
	"strings"

	"github.com/medici-finance/assay/statusgen/workref"
)

// Ruling-subject binding for the decision-gate hold and the ruling-link lane
// (milestone-support/09, design point 4). It runs after the existing ruling
// conditions — same repository, comment on the named issue, unedited, a
// personal account mapped as a human, the comment naming the brief, the issue
// carrying the brief's marker — and can only refuse. The comparison itself is
// the shared pure rule in statusgen/workref (BindRulingSubject); this adapter
// supplies statusgen's own observations: the judged brief's identity and
// explicit version, the registry at the judged base, and its own card read.

const (
	rulingWorkSubject          rulingReason = "work-subject"
	rulingWorkSubjectUnchecked rulingReason = "work-subject-could-not-check"
)

// gateSubject is the judged brief's side of the binding.
type gateSubject struct {
	Declared string // the brief's brief: value, else its board id
	PermID   string // frontmatter id: ("" = none)
	Version  uint64 // explicit frontmatter version: (0 = none; never defaulted)
	// registry reads the graph alias registry at the judged base. It is called
	// only when the comment carries a subject line.
	registry func() (*graphRepos, error)
}

func subjectOf(b *gateBrief, registry func() (*graphRepos, error)) gateSubject {
	declared := b.BriefID
	if declared == "" {
		declared = b.Board
	}
	return gateSubject{Declared: declared, PermID: b.PermID, Version: b.Version, registry: registry}
}

// gateSubjectRegistry builds the typed-reference registry the binding resolves
// against: the graph registry's roots, and the judged brief's own permanent id
// as its logical key. The declaring context is the registry's self root.
func gateSubjectRegistry(reg *graphRepos, declared, permID string) (*workref.Context, workref.Registry, error) {
	declaring, roots, why := milestoneOwner(reg)
	if why != "" || declaring == nil {
		return nil, workref.Registry{}, fmt.Errorf("%s", why)
	}
	if permID == "" {
		return declaring, roots, nil
	}
	r, err := workref.Resolve(declared, declaring, roots, workref.TypedBrief)
	if err != nil {
		return nil, workref.Registry{}, fmt.Errorf("the brief's identity %q does not resolve: %v", declared, err)
	}
	var rootList []workref.Root
	aliases := make([]string, 0, len(reg.Aliases))
	for a := range reg.Aliases {
		aliases = append(aliases, a)
	}
	sort.Strings(aliases)
	for _, a := range aliases {
		cell := reg.Aliases[a].Cell
		if cell == "" {
			cell = reg.Cell
		}
		rootList = append(rootList, workref.Root{Cell: cell, Repo: a})
	}
	full, err := workref.NewRegistry(rootList, []workref.Binding{{Identity: r.Identity, Key: permID}})
	if err != nil {
		return nil, workref.Registry{}, err
	}
	return declaring, full, nil
}

// bind is the rulingIssueBinding.subject hook.
func (s gateSubject) bind(comment, createdAt, issueBody string) (rulingReason, string) {
	ev := workref.RulingEvidence{Comment: comment, CreatedAt: createdAt}
	line, lerr := workref.ParseRulingSubject(comment)
	if lerr == nil && line == nil {
		// No subject line: only the cutover decides; no registry is needed.
		return bindingReason(workref.BindRulingSubject(ev, workref.Subject{}, nil, workref.Registry{}))
	}
	if lerr != nil {
		return bindingReason(workref.BindRulingSubject(ev, workref.Subject{}, nil, workref.Registry{}))
	}
	if s.registry == nil {
		return rulingWorkSubjectUnchecked, "no alias registry was supplied, so the ruling's subject could not be resolved"
	}
	reg, err := s.registry()
	if err != nil || reg == nil {
		return rulingWorkSubjectUnchecked, fmt.Sprintf("the alias registry at the judged base could not be read (%v), so the ruling's subject could not be resolved", err)
	}
	declaring, registry, err := gateSubjectRegistry(reg, s.Declared, s.PermID)
	if err != nil {
		return rulingWorkSubjectUnchecked, fmt.Sprintf("the ruling's subject could not be resolved: %v", err)
	}
	presented, err := workref.BriefSubject(s.Declared, s.Version, declaring, registry)
	if err != nil {
		return rulingWorkSubjectUnchecked, fmt.Sprintf("the judged brief's identity %q does not resolve: %v", s.Declared, err)
	}
	// Every marker spelling must resolve to the brief's logical key: a marker of
	// another cell, root or work key does not carry this ruling, even when its
	// stream and number match.
	if !markerNamesSubject(issueBody, presented.LogicalKey, declaring, registry) {
		return rulingWorkSubject, fmt.Sprintf("the decision issue carries no decision-gate marker that resolves to %s", presented.LogicalKey)
	}
	card, block, cerr := readGateCardBlock(issueBody)
	ev.Card = workref.CardObservation{Present: card != nil, Block: block, Err: cerr}
	if card != nil {
		ev.Card.Brief = card.Brief
		if card.Version > 0 {
			ev.Card.Version = uint64(card.Version)
		}
	}
	return bindingReason(workref.BindRulingSubject(ev, presented, declaring, registry))
}

func bindingReason(b workref.RulingBinding) (rulingReason, string) {
	switch b.State {
	case workref.Bound:
		return rulingOK, ""
	case workref.CouldNotCheck:
		return rulingWorkSubjectUnchecked, b.Reason + ": " + b.Detail
	default:
		return rulingWorkSubject, b.Reason + ": " + b.Detail
	}
}

// markerNamesSubject reports whether a decision-gate marker in body resolves to key.
func markerNamesSubject(body, key string, declaring *workref.Context, registry workref.Registry) bool {
	for _, m := range decisionGateMarkerRe.FindAllStringSubmatch(strings.ReplaceAll(body, "\r\n", "\n"), -1) {
		r, err := workref.Resolve(strings.TrimSpace(m[1]), declaring, registry, workref.TypedBrief)
		if err == nil && r.LogicalKey == key {
			return true
		}
	}
	return false
}
