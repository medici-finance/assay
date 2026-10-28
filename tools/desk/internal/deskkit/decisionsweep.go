package deskkit

import (
	"strings"
	"sync"
)

// DecisionSweep is a single caller-owned observation pass over ONE admitted repo.
// Discard it after the pass: it has no disk cache, TTL, ambient identity or authority.
// Marker discovery is shared; selected issues still go through BriefDecisionWait,
// including the original label/history/trust reducer. Concurrent rows referencing
// the same issue share that observation, including its explicit unknown result.
type DecisionSweep struct {
	forge        DecisionForge
	repo         ForgeRepo
	ratifier     func(string, int64) bool
	discover     sync.Once
	markers      map[string][]int
	discoveryErr error
	mu           sync.Mutex
	reads        map[int]*decisionSweepRead
	issues       map[int]*decisionSweepIssue
}

type decisionSweepIssue struct {
	done chan struct{}
	item *Issue
	err  error
}

type decisionSweepRead struct {
	done chan struct{}
	wait DecisionWait
}

func NewDecisionSweep(f DecisionForge, repo ForgeRepo, isRatifier func(string, int64) bool) *DecisionSweep {
	return &DecisionSweep{forge: f, repo: repo, ratifier: isRatifier, reads: map[int]*decisionSweepRead{}, issues: map[int]*decisionSweepIssue{}}
}

func (s *DecisionSweep) Wait(b BriefAdmission) DecisionWait {
	n := b.DecisionIssue
	if n == 0 {
		id := strings.TrimSpace(b.ID)
		if id == "" {
			return DecisionWait{}
		}
		if err := validateDecisionSearch(SearchIssuesInput{DecisionGateMarkers: true, DecisionGateBrief: id}); err != nil {
			return DecisionWait{Unknown: decisionSearchUnknown(s.repo, id, err)}
		}
		s.discover.Do(func() {
			hits, err := s.forge.SearchIssues(s.repo, SearchIssuesInput{DecisionGateMarkers: true})
			s.discoveryErr = err
			s.markers = map[string][]int{}
			if err != nil {
				return // no partial page is used as absence
			}
			for _, hit := range hits {
				for _, m := range decisionMarkerRe.FindAllStringSubmatch(hit.Body, -1) {
					// Discovery must preserve the spelling filter used by the
					// dispatcher's per-brief search. Normalizing here would admit
					// another project's colon id with the same stream and number.
					key := m[1]
					s.markers[key] = append(s.markers[key], hit.Number)
				}
			}
		})
		if s.discoveryErr != nil {
			return DecisionWait{Unknown: decisionSearchUnknown(s.repo, id, s.discoveryErr)}
		}
		spellings := decisionBriefSpellings(id)
		// Copy before sorting: concurrent briefs may share index entries.
		var candidates []int
		seen := map[int]bool{}
		for _, spelling := range spellings {
			for _, n := range s.markers[spelling] {
				if !seen[n] {
					seen[n] = true
					candidates = append(candidates, n)
				}
			}
		}
		var why string
		n, why = decisionIssueFromCandidates(s, s.repo, id, candidates)
		if why != "" {
			return DecisionWait{Unknown: why}
		}
		if n == 0 {
			return DecisionWait{}
		}
	}
	s.mu.Lock()
	if read := s.reads[n]; read != nil {
		s.mu.Unlock()
		<-read.done
		return read.wait
	}
	read := &decisionSweepRead{done: make(chan struct{})}
	s.reads[n] = read
	s.mu.Unlock()
	// Only issue identity is shared. Do not duplicate the ruling evaluator here.
	read.wait = BriefDecisionWait(s, s.repo, BriefAdmission{DecisionIssue: n}, s.ratifier)
	close(read.done)
	return read.wait
}

func (s *DecisionSweep) GetIssue(repo ForgeRepo, n int) (*Issue, error) {
	if repo != s.repo {
		return nil, Refused("decision sweep cannot read a different repository")
	}
	s.mu.Lock()
	if read := s.issues[n]; read != nil {
		s.mu.Unlock()
		<-read.done
		return read.item, read.err
	}
	read := &decisionSweepIssue{done: make(chan struct{})}
	s.issues[n] = read
	s.mu.Unlock()
	read.item, read.err = s.forge.GetIssue(repo, n)
	close(read.done)
	return read.item, read.err
}

func (s *DecisionSweep) SearchIssues(repo ForgeRepo, in SearchIssuesInput) ([]IssueSearchResult, error) {
	if repo != s.repo {
		return nil, Refused("decision sweep cannot search a different repository")
	}
	return s.forge.SearchIssues(repo, in)
}

func (s *DecisionSweep) ListIssueLabelEvents(repo ForgeRepo, n int) ([]LabelEvent, error) {
	if repo != s.repo {
		return nil, Refused("decision sweep cannot read a different repository")
	}
	return s.forge.ListIssueLabelEvents(repo, n)
}

func (s *DecisionSweep) IssueTrustEvents(repo ForgeRepo, n int) (*TrustPayload, error) {
	if repo != s.repo {
		return nil, Refused("decision sweep cannot read a different repository")
	}
	return s.forge.IssueTrustEvents(repo, n)
}
