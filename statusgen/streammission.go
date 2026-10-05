package main

// streammission.go — the statusgen side of the stream-view contract
// (statusgen/streamview, docs/stream-view-contract.md): a stream's
// repository-qualified identity, its mission section by precedence, and the
// lint diagnostics for an invalid authored mission block.
//
// Pure over the parsed Stream and its README; no forge, no clock (the caller
// injects the observation time), no store.

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/medici-finance/assay/statusgen/streamview"
)

// streamIdentity returns the stream's canonical identity: owning repo +
// stream slug. The repo comes from the stream's own `repo:` frontmatter when
// declared, else from defaultRepo — the repo the producer was configured to
// read this root as. A declared repo that disagrees with defaultRepo, or no
// repo at all, is an error: an identity is never guessed.
func streamIdentity(s *Stream, defaultRepo string) (streamview.Identity, error) {
	repo := s.Repo
	switch {
	case repo != "" && defaultRepo != "" && repo != defaultRepo:
		return streamview.Identity{}, fmt.Errorf("stream %q declares repo %q but the producer reads this root as %q", s.Name, repo, defaultRepo)
	case repo == "":
		repo = defaultRepo
	}
	if repo == "" {
		return streamview.Identity{}, fmt.Errorf("stream %q: no owning repo — declare `repo:` or configure the producer's repo for this root", s.Name)
	}
	key := streamview.Key{Repo: repo, Slug: s.Name}
	if err := key.Validate(); err != nil {
		return streamview.Identity{}, fmt.Errorf("stream %q: %w", s.Name, err)
	}
	return streamview.Identity{Key: key, DisplayName: streamDisplayName(s)}, nil
}

// streamDisplayName is the README H1 heading up to its em-dash tagline (the
// same separator streamOutcome reads the legacy outcome from), else the slug.
// Presentation only: it never identifies.
func streamDisplayName(s *Stream) string {
	raw, err := os.ReadFile(filepath.Join(s.Dir, "README.md"))
	if err != nil {
		return s.Name
	}
	for _, ln := range strings.Split(strings.ReplaceAll(string(raw), "\r\n", "\n"), "\n") {
		t := strings.TrimSpace(ln)
		if !strings.HasPrefix(t, "# ") {
			continue
		}
		head := strings.TrimSpace(strings.TrimPrefix(t, "# "))
		for _, sep := range []string{" — ", " - ", "—", " – "} {
			if i := strings.Index(head, sep); i >= 0 {
				head = strings.TrimSpace(head[:i])
				break
			}
		}
		if head = stripInlineMarkdown(head); head != "" {
			return head
		}
		break
	}
	return s.Name
}

// streamMissionSection builds the mission section by the contract's
// precedence: a valid authored `mission:` block, else the legacy README prose
// outcome, else absent. A present-but-invalid block yields could-not-check
// with its diagnostics — it is NOT replaced by legacy prose, and success
// criteria are never derived from prose. src is the README the stream was
// read from, at its exact revision; observed is the injected read time.
func streamMissionSection(s *Stream, key streamview.Key, src streamview.SourceRef, observed time.Time) streamview.Mission {
	prov := streamview.Provenance{
		Availability: streamview.Available,
		Sources:      []streamview.SourceRef{src},
		ObservedAt:   observed.UTC(),
	}
	if len(s.MissionDiagnostics) > 0 {
		prov.Availability = streamview.CouldNotCheck
		prov.Reason = "invalid authored mission metadata"
		prov.Diagnostics = append([]string(nil), s.MissionDiagnostics...)
		return streamview.Mission{Provenance: prov}
	}
	if s.Mission != nil {
		m := streamview.Mission{
			Provenance:  prov,
			Origin:      streamview.OriginAuthored,
			Outcome:     s.Mission.Outcome,
			Commitments: append([]string(nil), s.Mission.Commitments...),
			Exclusions:  append([]string(nil), s.Mission.Exclusions...),
		}
		for _, c := range s.Mission.Success {
			sc := streamview.SuccessCriterion{Criterion: c.Criterion}
			for _, e := range c.Evidence {
				if e.Kind == streamview.EvidencePath {
					e.Repo = key.Repo
				}
				sc.Evidence = append(sc.Evidence, e)
			}
			m.Success = append(m.Success, sc)
		}
		return m
	}
	if outcome := streamOutcome(s.Dir); outcome != "" {
		return streamview.Mission{Provenance: prov, Origin: streamview.OriginLegacyProse, Outcome: outcome}
	}
	return streamview.Mission{Provenance: prov, Origin: streamview.OriginAbsent}
}

// streamStateSection copies the stream status and every brief's number and
// status verbatim from the source — no normalisation — with the per-status
// counts. Frontier and holds need the eligibility evaluator, which this seed
// does not run, so the section is honestly partial rather than claiming an
// empty frontier.
func streamStateSection(s *Stream, key streamview.Key, src streamview.SourceRef, observed time.Time) streamview.CurrentState {
	cs := streamview.CurrentState{
		Provenance: streamview.Provenance{
			Availability: streamview.Partial,
			Sources:      []streamview.SourceRef{src},
			ObservedAt:   observed.UTC(),
			Reason:       "frontier and holds not projected: no eligibility input",
		},
		Status: s.Status,
		Counts: map[string]int{},
	}
	for _, b := range s.Briefs {
		cs.Briefs = append(cs.Briefs, streamview.BriefRef{ID: key.Slug + "/" + b.Num, Num: b.Num, Title: b.Title, Status: b.Status})
		cs.Counts[b.Status]++
	}
	return cs
}

// streamViewSeed assembles a contract-valid view from one parsed stream: the
// identity, the mission section and the verbatim current state, with every
// section that has no producer yet stated as could-not-check / not-assessed —
// never as empty. readmeRel is the README's repository-relative path and
// revision the exact commit it was read at; observed is injected so a fixed
// input yields fixed output.
func streamViewSeed(s *Stream, defaultRepo, readmeRel, revision string, observed time.Time, ctx streamview.AccessContext) (*streamview.StreamView, error) {
	id, err := streamIdentity(s, defaultRepo)
	if err != nil {
		return nil, err
	}
	observed = observed.UTC()
	src := streamview.SourceRef{Repo: id.Key.Repo, Path: filepath.ToSlash(readmeRel), Revision: revision}
	none := func(reason string, a streamview.Availability) streamview.Provenance {
		return streamview.Provenance{Availability: a, ObservedAt: observed, Reason: reason}
	}
	v := &streamview.StreamView{
		Contract:     streamview.Version,
		Identity:     id,
		Context:      ctx,
		GeneratedAt:  observed,
		Mission:      streamMissionSection(s, id.Key, src, observed),
		CurrentState: streamStateSection(s, id.Key, src, observed),
		Changes: streamview.Changes{
			Provenance: none("no history input supplied", streamview.CouldNotCheck),
			Window:     streamview.TrailingWindow(observed),
		},
		NeedsYou: streamview.NeedsYou{Provenance: none("no decision source supplied", streamview.CouldNotCheck)},
		Evidence: streamview.Evidence{Provenance: none("no evidence producer in this revision", streamview.NotAssessed)},
		Outcomes: streamview.Outcomes{Provenance: none("no outcome producer in this revision", streamview.NotAssessed)},
	}
	if err := v.Validate(); err != nil {
		return nil, err
	}
	return v, nil
}

// missionProblems turns every invalid authored mission block into a hard lint
// PROBLEM naming the stream and the defect. Inert for a stream with no
// `mission:` block, so a legacy corpus lints exactly as before.
func missionProblems(streams []*Stream) []string {
	var problems []string
	for _, s := range streams {
		for _, d := range s.MissionDiagnostics {
			problems = append(problems, fmt.Sprintf("%s: invalid mission metadata — %s", s.Name, d))
		}
	}
	sort.Strings(problems)
	return problems
}
