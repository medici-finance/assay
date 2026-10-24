package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	rf "github.com/medici-finance/assay/tools/desk/internal/reviewflow"
)

// resolvedReview is captured at the existing classifier boundary. The bridge must
// not re-fetch heads or independently reconstruct the classification decision.
type resolvedReview struct {
	change      prBase
	reviews     []review
	facts       rf.ClassificationFacts
	correctness string
	security    string
	complete    bool
	score       int
}

func resolveObservation(p prBase, rs reviewState, reviews []review, ciRequired bool, zero string, in classifyInput, score int, complete bool) resolvedReview {
	f := classificationFacts(p, rs, ciRequired, zero)
	if f.Change.Labels == nil {
		f.Change.Labels = []string{}
	}
	f.OwnFilesChanged, f.RiskClassed, f.RiskReason = in.ownFilesChanged, in.riskClassed, in.riskReason
	f.NonCommitResolution, f.NonCommitResolutionNote = in.nonCommitResolution, in.nonCommitResolutionNote
	// Use the existing marker parser/reducer on the correctness lane independently;
	// the legacy aggregate may include security grants and cannot establish this lane.
	correctness := make([]review, 0, len(reviews))
	for _, r := range reviews {
		if classifySecurityBody(r.Body) == secNone {
			correctness = append(correctness, r)
		}
	}
	cr := reduceReviews(correctness, p.HeadRefOid)
	cr = applyBodyEditReverification(cr, correctness, p.HeadRefOid, p.Body, p.LastEditedAt)
	state := "missing"
	switch {
	case !complete:
		state = "unknown"
	case cr.blocking:
		state = "blocked"
	case cr.approved:
		state = "approved"
	case cr.ever:
		state = "stale"
	}
	security := "missing"
	for _, r := range reviews {
		if isReviewerBot(r.User.Login) && classifySecurityBody(r.Body) != secNone {
			if sameHead(r.CommitID, p.HeadRefOid) {
				security = "blocked"
				break
			}
			security = "stale"
		}
	}
	if f.Reviews.SecurityPass {
		security = "approved"
	}
	return resolvedReview{change: p, reviews: reviews, facts: f, correctness: state, security: security, complete: complete, score: score}
}

func observationRevision(v any) string {
	b, _ := json.Marshal(v) // only concrete JSON-compatible structs/maps enter here
	return fmt.Sprintf("sha256:%x", sha256.Sum256(b))
}

func cmdReviewQueue() (*Report, error) {
	start := time.Now().UTC()
	boardMeter.begin(rateFloor)
	defer boardMeter.end()
	scores, err := fetchAndMapGateScores()
	if err != nil {
		return nil, deskkit.Unverifiable("review-queue: gate-scores unavailable", err)
	}
	known := make([]string, 0, len(scores))
	for k := range scores {
		known = append(known, k)
	}
	repos := deskkit.AllowedRepos()
	// Revision binds collection configuration, including access scope, trust, CI and
	// reviewer identity. Only its digest is stored, never configuration/credentials.
	repoPolicies := map[string]any{}
	for _, repo := range repos {
		repoPolicies[repo] = struct{ CIRequired, VisibilityRiskClassed bool }{deskkit.CIRequired(repo), deskkit.VisibilityRiskClassed(repo)}
	}
	policy := "deskboard-review-v1:" + observationRevision(struct {
		Config       any
		Scores       map[string]int
		RepoPolicies map[string]any
	}{deskkit.EffectiveConfig(), scores, repoPolicies})
	partials, err := sweepRepos(repos, sweepConcurrency, func(repo string) (actionsPartial, error) {
		return sweepActionsRepo(repo, scores, known, nil, start, true)
	})
	if cost := boardMeter.snapshot(); cost.FloorBreach != nil {
		return nil, cost.floorError(time.Now())
	}
	if err != nil {
		return nil, err
	}
	in := rf.ObservationInputs{Evaluator: rf.ObservationEvaluator, Policy: policy, ReviewerDisplay: reviewerBotDisplay(), Requested: []rf.Repository{}, Admitted: []rf.Repository{}, Population: []rf.PopulationCoverage{}, Changes: []rf.ObservedChange{}}
	for i, repo := range repos {
		owner, name, _ := strings.Cut(repo, "/")
		resolution, err := deskkit.ForgeKindFor(deskkit.ForgeRepo{Owner: owner, Name: name})
		if err != nil {
			return nil, err
		}
		// The current GitLab reader lacks complete pinned review facts. Do not label
		// a guessed SaaS host or its degraded reductions as a qualified observation.
		if resolution.Kind != deskkit.ForgeGitHub {
			return nil, deskkit.Unverifiable("review-queue: complete host-qualified observation is not supported for this forge", nil)
		}
		r := rf.Repository{Provider: string(resolution.Kind), Host: "github.com", Repository: strings.ToLower(repo)}
		part := partials[i]
		if part.truncated || len(part.external) != 0 {
			return nil, deskkit.Unverifiable("review-queue: truncated or quarantined population cannot support a complete observation", nil)
		}
		in.Requested = append(in.Requested, r)
		in.Admitted = append(in.Admitted, r)
		provenance := func(v any) rf.InputProvenance {
			return rf.InputProvenance{Source: r.String(), Version: "deskboard-review-v1", Revision: observationRevision(v), ObservedAt: start, Freshness: "fresh", Complete: true}
		}
		in.Population = append(in.Population, rf.PopulationCoverage{Repository: r, Count: len(part.resolved), Input: provenance(part.currentOpen)})
		for _, v := range part.resolved {
			if !v.complete {
				return nil, deskkit.Unverifiable("review-queue: degraded classification requires refresh; no complete artifact emitted", nil)
			}
			inputs := rf.ReviewInputs{Subject: provenance(v.change), Body: provenance(v.change.Body), Reviews: provenance(struct {
				Raw      []review
				Resolved rf.ReviewState
			}{v.reviews, v.facts.Reviews}), Checks: provenance(v.facts.Change.Checks), Enrichment: provenance(v.facts)}
			id := rf.ChangeIdentity{Repository: r, Number: v.change.Number, Head: v.change.HeadRefOid}
			if v.facts.Reviews.Ever && !v.facts.Reviews.AtHead && shasComparable(v.facts.Reviews.LastSHA, id.Head) {
				id.Base, id.BaseUsed = v.facts.Reviews.LastSHA, true
			}
			applies := rf.Applicability{Head: id.Head, Base: id.Base, Body: inputs.Body.Revision, Reviews: inputs.Reviews.Revision, Checks: inputs.Checks.Revision}
			security := v.security
			in.Changes = append(in.Changes, rf.ObservedChange{Identity: id, Inputs: inputs, AppliesTo: applies, Facts: v.facts, Score: v.score, Lanes: []rf.LaneConclusion{{Name: "correctness", State: v.correctness, AppliesTo: applies}, {Name: "security", State: security, AppliesTo: applies}}})
		}
	}
	in.Collection = rf.CollectionInterval{Start: start, End: time.Now().UTC()}
	observation, err := rf.BuildObservation(in, rf.ObservationPolicy{Now: in.Collection.End, Oldest: start, Policy: policy})
	if err != nil {
		return nil, deskkit.Unverifiable("review-queue: cannot qualify collected inputs", err)
	}
	summary := fmt.Sprintf("review-queue: %d PR(s), %d repo(s); generation %s; cutoff %s; policy %s", observation.Result.Depths.Total, len(in.Admitted), in.Collection.Start.Format(time.RFC3339Nano), in.Collection.End.Format(time.RFC3339Nano), policy)
	return &Report{value: observation, detail: summary, render: func(w io.Writer) { fmt.Fprintln(w, summary) }}, nil
}

// Sensitive artifacts replace the destination atomically without following its
// final symlink. CreateTemp supplies 0600 even under a permissive operator umask.
func writeReviewObservation(path string, value any) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".review-observation-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	encodeErr := json.NewEncoder(f).Encode(value)
	closeErr := f.Close()
	if encodeErr != nil {
		return encodeErr
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(f.Name(), path)
}
