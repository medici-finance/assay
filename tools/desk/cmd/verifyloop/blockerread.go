package main

// blockerread.go — verify-reset/03: the planner's blocker read. A held non-pass stays held
// only while its blocker issue is open; this is the read that says so, through the forge the
// repo resolves to, as the verifier App. It never reports "closed" without a read that said
// closed: a missing token, a refused mint, an HTTP error and an unknown state are all
// could-not-check, and the hold is surfaced rather than released.

import (
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// issueSourceFn builds the plan's issue-state source. A package var so a test can install a
// forge that fails the way a missing credential does.
var issueSourceFn = func() deskkit.IssueStateSource {
	return &forgeIssueSource{forgeFor: func(repo deskkit.ForgeRepo) (deskkit.Forge, error) {
		return deskkit.ForgeFor(repo, verifyloopRole)
	}}
}

// forgeIssueSource reads issue state through a per-repo forge, resolving each repo's forge (and
// so its credential) once per plan pass.
type forgeIssueSource struct {
	forgeFor func(deskkit.ForgeRepo) (deskkit.Forge, error)
	forges   map[string]deskkit.Forge
	errs     map[string]error
}

func (s *forgeIssueSource) IssueState(ref deskkit.BlockerRef) (deskkit.BlockerState, string) {
	repo := ref.Repo()
	key := repo.Slug()
	if s.forges == nil {
		s.forges, s.errs = map[string]deskkit.Forge{}, map[string]error{}
	}
	fg, ok := s.forges[key]
	if !ok {
		if err, failed := s.errs[key]; failed {
			return deskkit.BlockerCouldNotCheck, "no forge read for " + key + ": " + oneLine(err)
		}
		f, err := s.forgeFor(repo)
		if err != nil || f == nil {
			if err == nil {
				err = deskkit.Unverifiable("no forge resolved", nil)
			}
			s.errs[key] = err
			return deskkit.BlockerCouldNotCheck, "no forge read for " + key + ": " + oneLine(err)
		}
		s.forges[key], fg = f, f
	}
	var (
		iss *deskkit.Issue
		err error
	)
	if ref.Kind == "change" {
		iss, err = fg.GetIssueTyped(repo, ref.Number, deskkit.TargetChange)
	} else {
		iss, err = fg.GetIssue(repo, ref.Number)
	}
	if err != nil || iss == nil {
		if err == nil {
			err = deskkit.Unverifiable("empty issue read", nil)
		}
		return deskkit.BlockerCouldNotCheck, "reading " + ref.Raw + ": " + oneLine(err)
	}
	switch strings.ToLower(strings.TrimSpace(iss.State)) {
	case "open", "opened":
		return deskkit.BlockerOpen, ""
	case "closed", "merged":
		return deskkit.BlockerClosed, ""
	}
	return deskkit.BlockerCouldNotCheck, "reading " + ref.Raw + ": unknown state " + iss.State
}

func oneLine(err error) string {
	s := strings.TrimSpace(err.Error())
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	return s
}
