package service

import (
	"reflect"

	"github.com/medici-finance/assay/forgeread"
	"github.com/medici-finance/assay/forgeread/mirror"
)

func validateMirror(r Response, q Request) error {
	if r.Mirror != nil {
		s := r.Mirror
		if s.Scope.Repository != q.Repository || s.Scope.AdmissionID != q.Scope.AdmissionID || s.Scope.Kind != q.Mirror.Kind || s.Records == nil || len(s.Records) > MaxItems || s.Coverage.State != mirror.Complete {
			return ErrScope
		}
		// Reuse the mirror owner's structural record/scope/coverage validation.
		// This is no apply, freshness decision, or authority grant.
		if err := (mirror.ScanUpdate{Scope: s.Scope, Coverage: s.Coverage, Upsert: s.Records}).Validate(); err != nil {
			return invalid()
		}
		for _, row := range s.Records {
			if !text(row.Title, MaxTextBytes, true) || !text(row.Body, MaxContentBytes, true) || len(row.Labels) > MaxItems || len(row.RequestedReviewers) > MaxItems || len(row.Reviews.GitHub) > MaxItems || len(row.Reviews.GitLab) > MaxItems {
				return invalid()
			}
		}
		return nil
	}
	if r.Issues == nil || q.Mirror.Kind != mirror.Issues {
		return invalid()
	}
	// Legacy parser behavior is unchanged. The new successful transport profile
	// additionally requires explicit complete population rather than a partial prefix.
	e := r.Issues
	if e.Schema != forgeread.EnvelopeSchema || e.Kind != forgeread.IssuesKind || len(e.Repos) != 1 || e.Partial == nil || len(e.Partial) != 0 || e.Repos[0].Repo != q.Repository.Path || e.Repos[0].Issues == nil || len(e.Repos[0].Issues) > MaxItems {
		return invalid()
	}
	seen := map[int]bool{}
	for _, row := range e.Repos[0].Issues {
		if row.Number < 1 || seen[row.Number] || row.State != "open" || !text(row.Title, MaxTextBytes, true) {
			return invalid()
		}
		seen[row.Number] = true
	}
	return nil
}
func validateSource(r Response, q Request) error {
	if r.Source == nil || r.Source.Revision != q.Source.Revision || r.Source.Other != q.Source.Other || r.Source.Path != q.Source.Path || len(r.Source.Content) > MaxContentBytes || len(r.Source.Entries) > MaxItems {
		return invalid()
	}
	s := r.Source
	switch q.Operation {
	case FileAt:
		if s.Content == nil || s.Entries != nil || s.Base != "" {
			return invalid()
		}
	case Diff:
		if s.Content == nil || s.Entries == nil || s.Base != "" {
			return invalid()
		}
		seen := map[string]bool{}
		for _, e := range s.Entries {
			if !path(e.Path) || seen[e.Path] || (e.Status != "added" && e.Status != "modified" && e.Status != "deleted" && e.Status != "renamed") || (e.Before != "" && !oid.MatchString(e.Before)) || (e.After != "" && !oid.MatchString(e.After)) {
				return invalid()
			}
			seen[e.Path] = true
		}
	case MergeBase:
		if s.Content != nil || s.Entries != nil || !oid.MatchString(s.Base) {
			return invalid()
		}
	}
	return nil
}
func validateEffect(r Response, q Request) error {
	if r.Effect == nil {
		return invalid()
	}
	e := r.Effect
	id := q.Lookup
	if q.Effect != nil {
		id = &q.Effect.ID
	}
	if id == nil || e.ID != *id {
		return ErrConflict
	}
	if e.Projection != Current && e.Projection != Pending && e.Projection != NoProjection {
		return ErrSchema
	}
	if e.Outcome != Succeeded && e.Outcome != Failed && e.Outcome != Unknown {
		return ErrSchema
	}
	if e.Outcome != Succeeded {
		if e.Object != nil || e.Provenance != nil || e.Projection == Current {
			return invalid()
		}
		return nil
	}
	if e.Object == nil || !provenance(e.Provenance) || e.Object.Repository != q.Repository || !text(e.Object.ProviderID, 256, false) || e.Object.Number == nil || *e.Object.Number < 1 {
		return invalid()
	}
	if e.Object.Kind == "issue" {
		if e.Object.Parent != nil || q.Operation == CreateComment {
			return invalid()
		}
	} else if e.Object.Kind == "change-comment" {
		if e.Object.Parent == nil || *e.Object.Parent < 1 || q.Operation == CreateIssue || (q.Effect != nil && *e.Object.Parent != q.Effect.Number) {
			return invalid()
		}
	} else {
		return ErrSchema
	}
	if (e.Projection == Pending) != (r.Metadata.State == ProjectionPending) {
		return invalid()
	}
	return nil
}

func reviewRepository(r ReviewRepository) mirror.RepositoryID {
	return mirror.RepositoryID{Provider: mirror.Provider(*r.Provider), Host: *r.Host, Path: *r.Repository}
}
func reviewScope(requested, admitted []ReviewRepository, s Scope) bool {
	if len(requested) != len(s.Repositories) || len(admitted) != len(s.Repositories) {
		return false
	}
	for _, list := range [][]ReviewRepository{requested, admitted} {
		seen := map[mirror.RepositoryID]bool{}
		for _, item := range list {
			r := reviewRepository(item)
			if !member(s, r) || seen[r] {
				return false
			}
			seen[r] = true
		}
	}
	return true
}
func validateReview(r Response, q Request) error {
	if r.Review == nil || presentWire(reflect.ValueOf(*r.Review)) != nil {
		return invalid()
	}
	o := r.Review
	if *o.Schema != "review-observation-v1" || !reviewScope(o.Inputs.Requested, o.Inputs.Admitted, q.Scope) || len(o.Result.Rows) != len(o.Inputs.Changes) || *o.Result.Depths.Total != len(o.Result.Rows) {
		return ErrScope
	}
	if *o.Inputs.Evaluator != "reviewflow-v1" || *o.Inputs.Policy == "" || o.Inputs.Collection.Start.IsZero() || o.Inputs.Collection.End.IsZero() || o.Inputs.Collection.Start.After(*o.Inputs.Collection.End) {
		return invalid()
	}
	populations := map[mirror.RepositoryID]int{}
	for _, p := range o.Inputs.Population {
		repo := reviewRepository(p.Repository)
		if _, duplicate := populations[repo]; duplicate || !member(q.Scope, repo) || *p.Count < 0 || !reviewProvenance(p.Input, o.Inputs.Collection) {
			return invalid()
		}
		populations[repo] = *p.Count
	}
	if len(populations) != len(q.Scope.Repositories) {
		return invalid()
	}
	counts := map[mirror.RepositoryID]int{}
	// These checks bind transport identities only. PlanObservation remains the
	// sole owner of recomputing action legality, ranks, lanes, holds and depths.
	seen := map[mirror.RecordID]bool{}
	for i, c := range o.Inputs.Changes {
		repo := reviewRepository(c.Identity.Repository)
		id := mirror.RecordID{Repository: repo, Kind: "change", Number: int64(*c.Identity.Number)}
		if !member(q.Scope, repo) || id.Number < 1 || seen[id] || !oid.MatchString(*c.Identity.Head) || (*c.Identity.BaseUsed && !oid.MatchString(*c.Identity.Base)) || (!*c.Identity.BaseUsed && *c.Identity.Base != "") {
			return invalid()
		}
		seen[id] = true
		counts[repo]++
		if *c.AppliesTo.Head != *c.Identity.Head || *c.AppliesTo.Base != *c.Identity.Base || *c.AppliesTo.Body != *c.Inputs.Body.Revision || *c.AppliesTo.Reviews != *c.Inputs.Reviews.Revision || *c.AppliesTo.Checks != *c.Inputs.Checks.Revision {
			return invalid()
		}
		for _, lane := range c.Lanes {
			if !reflect.DeepEqual(lane.AppliesTo, c.AppliesTo) {
				return invalid()
			}
		}
		for _, p := range []ReviewProvenance{c.Inputs.Subject, c.Inputs.Body, c.Inputs.Reviews, c.Inputs.Checks, c.Inputs.Enrichment} {
			if !reviewProvenance(p, o.Inputs.Collection) {
				return invalid()
			}
		}
		row := o.Result.Rows[i]
		// Rows may be independently ranked, so find the matching exact identity.
		found := false
		for _, candidate := range o.Result.Rows {
			if reflect.DeepEqual(candidate.Identity, c.Identity) {
				row, found = candidate, true
				break
			}
		}
		if !found || !reflect.DeepEqual(row.AppliesTo, c.AppliesTo) {
			return invalid()
		}
	}
	for repo, count := range populations {
		if counts[repo] != count {
			return invalid()
		}
	}
	return nil
}

// This checks consistency of supplied wire coverage. Current caller freshness,
// output recomputation and consequential use still require the domain owner.
func reviewProvenance(p ReviewProvenance, c ReviewCollection) bool {
	return *p.Source != "" && *p.Version != "" && *p.Revision != "" && *p.Complete && *p.Freshness == "fresh" && !p.ObservedAt.IsZero() && !p.ObservedAt.After(*c.End)
}
func validateProjection(r Response, q Request) error {
	if r.Projection == nil || presentWire(reflect.ValueOf(*r.Projection)) != nil {
		return invalid()
	}
	p := r.Projection
	if *p.Schema != "issue-projection-v1" || !reviewScope(p.Requested, p.Admitted, q.Scope) {
		return ErrScope
	}
	switch *p.Projection {
	case "full", "work-ready", "intake", "inbox":
	default:
		return ErrSchema
	}
	seen := map[mirror.RecordID]bool{}
	for _, row := range p.Rows {
		id := mirror.RecordID{Repository: reviewRepository(row.Repository), Kind: "issue", Number: int64(*row.Number)}
		if !member(q.Scope, id.Repository) || id.Number < 1 || seen[id] {
			return invalid()
		}
		seen[id] = true
		// Preserve the existing classifier's rows without computing a new one.
		switch *row.Action {
		case "ESCALATE", "ADDRESSED", "CREATE-PLACEHOLDER", "RETIRE", "AWAIT", "NONE":
		default:
			return ErrSchema
		}
	}
	for _, row := range p.Quarantined {
		id := mirror.RecordID{Repository: reviewRepository(row.Repository), Kind: "issue", Number: int64(*row.Number)}
		if !member(q.Scope, id.Repository) || id.Number < 1 || seen[id] {
			return invalid()
		}
		seen[id] = true
	}
	return nil
}
