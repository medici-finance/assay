package main

// blockerread.go — verify-reset/03: the planner's blocker read. A held non-pass stays held
// only while its blocker issue is open; this is the read that says so, through the forge the
// repo resolves to, as the verifier App. It never reports "closed" without a read that said
// closed: a missing token, a refused mint, an HTTP error and an unknown state are all
// could-not-check, and the hold is surfaced rather than released.
//
// The repository a read goes to comes from record content (blocker_ref, or the record's repo
// key for a bare #N), so it is bounded here, before any credential is minted: a repository
// outside the configured repository set (deskkit.IsAllowedRepo — the check the board reader
// applies) is could-not-check and the forge is never resolved for it. resolveForge is the ONE
// place this package resolves a forge; TestForgeForSingleCaller pins that.

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"github.com/medici-finance/assay/tools/desk/internal/loopengine"
)

// issueSourceFn builds the plan's issue-state source. A package var so a test can install a
// forge that fails the way a missing credential does.
var issueSourceFn = func() deskkit.IssueStateSource {
	s := &forgeIssueSource{notice: os.Stderr}
	s.forgeFor = func(repo deskkit.ForgeRepo) (deskkit.Forge, error) {
		return resolveForge(repo, s.observer)
	}
	return s
}

// resolveForge is this package's only forge resolution: the verifier-role forge for repo.
func resolveForge(repo deskkit.ForgeRepo, observer deskkit.RequestObserver) (deskkit.Forge, error) {
	return deskkit.ForgeFor(repo, verifyloopRole, observer)
}

// forgeIssueSource reads issue state through a per-repo forge, resolving each repo's forge (and
// so its credential) once per plan pass, and only for a repository in the configured set.
type forgeIssueSource struct {
	performance    verifyPerformance
	observer       deskkit.RequestObserver
	forgeFor       func(deskkit.ForgeRepo) (deskkit.Forge, error)
	notice         io.Writer // when set, names each repository once, before its first forge contact
	forges         map[string]deskkit.Forge
	errs           map[string]error
	pendingTargets map[string]map[string][]loopengine.Item
	pending        map[string]*pendingChanges
	blockers       map[blockerObservationKey]blockerObservation
}

// A source can be used by a loop for multiple selections. Mutable scheduling
// observations are refreshed at selection entry, never retained into a new pass.
func (s *forgeIssueSource) beginPlanningPass() {
	s.blockers, s.pending = nil, nil
	s.pendingTargets = nil
	s.forges, s.errs = nil, nil
}

type blockerObservationKey struct {
	repo   string
	kind   deskkit.TargetKind
	number int
}
type blockerObservation struct {
	state deskkit.BlockerState
	why   string
}

func (s *forgeIssueSource) IssueState(ref deskkit.BlockerRef) (deskkit.BlockerState, string) {
	defer s.performance.phase("blocker")()
	repo := ref.Repo()
	fg, err := s.forge(repo)
	if err != nil {
		return deskkit.BlockerCouldNotCheck, oneLine(err)
	}
	kind := deskkit.TargetIssue
	if ref.Kind == "change" {
		kind = deskkit.TargetChange
	}
	key := blockerObservationKey{repo: repo.Slug(), kind: kind, number: ref.Number}
	if s.blockers == nil {
		s.blockers = map[blockerObservationKey]blockerObservation{}
	}
	observed, ok := s.blockers[key]
	if !ok {
		var iss *deskkit.Issue
		if kind == deskkit.TargetChange {
			iss, err = fg.GetIssueTyped(repo, ref.Number, kind)
		} else {
			iss, err = fg.GetIssue(repo, ref.Number)
		}
		observed.state = deskkit.BlockerCouldNotCheck
		switch {
		case err != nil:
			observed.why = oneLine(err)
		case iss == nil:
			observed.why = "empty issue read"
		default:
			switch strings.ToLower(strings.TrimSpace(iss.State)) {
			case "open", "opened":
				observed.state = deskkit.BlockerOpen
			case "closed", "merged":
				observed.state = deskkit.BlockerClosed
			default:
				observed.why = "unknown state " + iss.State
			}
		}
		s.blockers[key] = observed
	}
	if observed.state == deskkit.BlockerCouldNotCheck {
		return observed.state, "reading " + ref.Raw + ": " + observed.why
	}
	return observed.state, ""
}

// forge is the existing allowlisted coordinate and credential boundary, shared
// by blocker and pending-Evidence reads. Errors remain cached for this pass.
func (s *forgeIssueSource) forge(repo deskkit.ForgeRepo) (deskkit.Forge, error) {
	key := repo.Slug()
	if !deskkit.IsAllowedRepo(key) {
		return nil, fmt.Errorf("repository %s is outside the configured repository set — not read", key)
	}
	if s.forges == nil {
		s.forges, s.errs = map[string]deskkit.Forge{}, map[string]error{}
	}
	fg, ok := s.forges[key]
	if !ok {
		if err, failed := s.errs[key]; failed {
			return nil, fmt.Errorf("no forge read for %s: %w", key, err)
		}
		if s.notice != nil {
			fmt.Fprintf(s.notice, "verify-desk plan: reading verification scheduling state from %s as the verifier App\n", key)
		}
		f, err := s.forgeFor(repo)
		if err != nil || f == nil {
			if err == nil {
				err = deskkit.Unverifiable("no forge resolved", nil)
			}
			s.errs[key] = err
			return nil, fmt.Errorf("no forge read for %s: %w", key, err)
		}
		s.forges[key], fg = f, f
	}
	return fg, nil
}

func oneLine(err error) string {
	s := strings.TrimSpace(err.Error())
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	return s
}
