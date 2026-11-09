package main

// Host mapping for DR-milestone-support-06. The explicit source port supplies
// frozen repository/verification observations. Forge authentication reuses the
// existing admitted ruling seam; no client, credential, scan or store is added.
import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/medici-finance/assay/statusgen/domain"
	"github.com/medici-finance/assay/statusgen/workref"
)

// milestoneReceiptSources must be composed by an authorized host, never from
// manifest claims. ReadAt reads exactly repository/commit/path through a confined
// snapshot view. Protected proves both protected-default-branch identity and
// source reachability; Missing is an observed unmerged source, unknown otherwise.
// Contract maps the existing loader's record at that source, including the
// authority selected by its Acceptance authority section. Brief selects existing
// immutable verification records, evaluator facts and trusted provenance custody.
// Decisions lists the WHOLE selected decision thread, with explicit completeness.
// No production selector is installed or capability advertised by this adapter.
type milestoneReceiptSources interface {
	ReadAt(domain.EvidencePointer) ([]byte, error)
	Protected(domain.EvidencePointer) domain.Completeness
	Contract(domain.EvidencePointer) (MilestoneRecord, domain.EvidencePointer, workref.Context, workref.Registry, bool, error)
	Brief(domain.MilestoneContributor) (domain.MilestoneBriefObservation, error)
	Prerequisite(domain.MilestonePrerequisite) (domain.MilestonePrerequisiteObservation, error)
	DecisionLineage(domain.Actor) ([]string, error)
	Decisions(repo string, issue int) ([]rulingLink, bool, error)
}

type milestoneAcceptanceRequest struct {
	Manifest      domain.EvidencePointer
	DecisionIssue int
	Current       domain.MilestoneInputs // current subject/currency/capability only
}

// readMilestoneAcceptance is the actual receipt consumer seam for /07. It returns
// observations only. Legacy scans, graph gates and advertisements are unchanged.
func readMilestoneAcceptance(req milestoneAcceptanceRequest, sources milestoneReceiptSources, forge *ghClient, humans map[string]string) domain.MilestoneAssessment {
	held := func(reason string) domain.MilestoneAssessment {
		return domain.MilestoneAssessment{State: "unknown", AcceptanceOutstanding: true, Unknowns: []string{reason}}
	}
	if sources == nil || req.Manifest.Validate() != nil || req.DecisionIssue < 1 {
		return held("receipt-selection")
	}
	raw, err := sources.ReadAt(req.Manifest)
	if err != nil || domain.ContentDigest(raw) != req.Manifest.Digest {
		return held("receipt-unavailable")
	}
	var manifest domain.MilestoneManifest
	if err = milestoneDecode(raw, &manifest); err != nil {
		return held("receipt-payload")
	}
	canonical, err := manifest.Canonical()
	if err != nil || !bytes.Equal(raw, canonical) {
		return held("receipt-canonical")
	}
	record, authority, owner, registry, known, err := sources.Contract(manifest.Contract)
	if err != nil {
		return held("contract-unavailable")
	}
	// Source proofs are supplied by the host. The core independently requires
	// them; direct-core tests bypass this host mapping to exercise that boundary.
	cp, ap := sources.Protected(manifest.Contract), sources.Protected(authority)
	in := req.Current
	in.Owner, in.Registry = owner, registry
	in.ContractProtected, in.AuthorityProtected = cp, ap
	in.Definition = domain.MilestoneDefinition{Milestone: record.Ref(), Source: manifest.Contract, Authority: authority, Contributors: record.Contributes, CoverageKnown: known}
	for _, p := range record.RequiresMilestones {
		in.Definition.Prerequisites = append(in.Definition.Prerequisites, p.Identity.String()+fmt.Sprintf("@%d", p.Revision))
	}
	in.Definition.CriterionContributors = map[string][]string{}
	for _, c := range record.Criteria {
		in.Definition.CriterionContributors[c.ID] = append([]string(nil), c.Contributors...)
		in.Definition.Criteria = append(in.Definition.Criteria, c.ID)
	}
	policyRaw, err := sources.ReadAt(authority)
	if err != nil || domain.ContentDigest(policyRaw) != authority.Digest {
		return held("authority-unavailable")
	}
	if err = milestoneDecode(policyRaw, &in.Policy); err != nil {
		return held("authority-payload")
	}
	in.Policy.Source = authority
	// All trust-bearing collections are replaced, never inherited from a caller's
	// current-subject envelope or echoed from the manifest.
	in.Briefs = nil
	in.Prerequisites = nil
	in.Decisions = nil
	in.Evidence = nil
	for _, c := range manifest.Contributors {
		if o, e := sources.Brief(c); e == nil {
			in.Briefs = append(in.Briefs, o)
		}
	}
	for _, p := range manifest.Prerequisites {
		if o, e := sources.Prerequisite(p); e == nil {
			in.Prerequisites = append(in.Prerequisites, o)
		}
	}
	for _, p := range domain.MilestoneEvidencePointers(manifest) {
		state := domain.CouldNotCheck
		if b, e := sources.ReadAt(p); e == nil && domain.ContentDigest(b) == p.Digest {
			state = domain.Present
		}
		in.Evidence = append(in.Evidence, domain.MilestoneEvidenceObservation{Pointer: p, State: state})
	}
	links, complete, err := sources.Decisions(manifest.Contract.Repository, req.DecisionIssue)
	in.DecisionsComplete = complete && err == nil
	for _, link := range links {
		// A source cannot redirect authentication outside the selected repository or
		// issue. resolveRulingOn independently checks actual comment parentage.
		if link.Repo != manifest.Contract.Repository || link.Issue != req.DecisionIssue {
			in.DecisionsComplete = false
			continue
		}
		url := fmt.Sprintf("https://github.com/%s/issues/%d#issuecomment-%d", link.Repo, link.Issue, link.CommentID)
		rec := decisionRecordStamp{ID: manifest.Milestone, Ruling: url}
		outcome := resolveRulingOn(forge, manifest.Contract.Repository, rec, humans, rulingIssueBinding{
			what: "the exact milestone version",
			rules: func(issue rulingIssue, _ rulingLink) bool {
				return strings.Contains(issue.Body, "<!-- milestone-decision: "+manifest.Milestone+" -->")
			},
		})
		d := domain.MilestoneDecision{Pointer: domain.DecisionPointer{Repository: link.Repo, Issue: link.Issue, Comment: link.CommentID}}
		if outcome.Reason != rulingOK {
			d.Unavailable = outcome.Reason == rulingUnresolvable || outcome.Reason == rulingDeleted
			in.Decisions = append(in.Decisions, d)
			continue
		}
		d.Authenticated = true
		d.Actor = domain.Actor{ID: "human:" + outcome.Name, Kind: "human"}
		d.Lineage, _ = sources.DecisionLineage(d.Actor)
		d.Pointer.Digest = domain.ContentDigest([]byte(outcome.Body))
		// Ordinary discussion has no receipt semantics. A recognized but unsupported
		// payload is explicit unknown; an authenticated reject/hold never accepts.
		if !strings.Contains(outcome.Body, "milestone-decision-") {
			continue
		}
		if e := milestoneDecode([]byte(outcome.Body), &d.Payload); e != nil {
			d.Unavailable = true
		}
		in.Decisions = append(in.Decisions, d)
	}
	return domain.AssessMilestone(manifest, in)
}

// Reject duplicate/case-folded keys by requiring the exact deterministic JSON
// encoding for payloads; manifests additionally require sorted canonical keys.
func milestoneDecode(raw []byte, target any) error {
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(target); err != nil {
		return err
	}
	if err := d.Decode(new(any)); err != io.EOF {
		return fmt.Errorf("expected one payload")
	}
	// Remarshal through untyped JSON to compare normalized keys, but retain the
	// typed values so case-insensitive and duplicate names cannot override a field.
	var untyped any
	if err := json.Unmarshal(raw, &untyped); err != nil {
		return err
	}
	a, _ := json.Marshal(untyped)
	b, _ := json.Marshal(target)
	var typed any
	_ = json.Unmarshal(b, &typed)
	b, _ = json.Marshal(typed)
	if !bytes.Equal(a, b) {
		return fmt.Errorf("noncanonical payload fields")
	}
	// A second token walk rejects duplicate names (including equal-value copies).
	return milestoneUnique(json.NewDecoder(bytes.NewReader(raw)))
}
func milestoneUnique(d *json.Decoder) error {
	t, e := d.Token()
	if e != nil {
		return e
	}
	delim, ok := t.(json.Delim)
	if !ok {
		return nil
	}
	seen := map[string]bool{}
	for d.More() {
		if delim == '{' {
			t, e = d.Token()
			if e != nil {
				return e
			}
			name, ok := t.(string)
			if !ok || seen[name] {
				return fmt.Errorf("duplicate payload field")
			}
			seen[name] = true
		}
		if e = milestoneUnique(d); e != nil {
			return e
		}
	}
	_, e = d.Token()
	return e
}
