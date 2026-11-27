package deskkit

import (
	"fmt"
	"strconv"
	"strings"
)

// DecisionHoldState is the outcome of re-reading a CARRIED decision hold: a hold a session
// carried forward from an earlier tick that names a decision issue as its source.
type DecisionHoldState int

const (
	// DecisionHoldLive means the hold stands this tick: its decision issue is still open, or it
	// closed without the ruling label.
	DecisionHoldLive DecisionHoldState = iota
	// DecisionHoldLifted means the hold's own lift condition was observed met this tick: the
	// decision issue is closed AND carries DecidedLabel.
	DecisionHoldLifted
	// DecisionHoldUnknown means the source could not be read this tick. The hold is kept;
	// could-not-check is never a lift.
	DecisionHoldUnknown
)

func (s DecisionHoldState) String() string {
	switch s {
	case DecisionHoldLifted:
		return "lifted"
	case DecisionHoldUnknown:
		return "could-not-check"
	default:
		return "live"
	}
}

// IssueGetter is the one forge read a decision-hold re-read needs.
type IssueGetter interface {
	GetIssue(repo ForgeRepo, number int) (*Issue, error)
}

// DecisionHold is one re-read of a carried decision hold. Why states what was observed, so the
// caller can print the hold with its source on the same line.
type DecisionHold struct {
	Repo  ForgeRepo
	Issue int
	State DecisionHoldState
	Why   string
}

// Holds reports whether the held item must stay out of dispatch this tick. Only a lift that
// was observed this tick releases it.
func (h DecisionHold) Holds() bool { return h.State != DecisionHoldLifted }

// RereadDecisionHold re-reads a carried decision hold's source issue. It is meant to run on
// EVERY tick for every carried hold: a hold carried from an earlier tick is a claim about the
// past, and only this tick's read of its source may keep or lift it.
//
// The lift condition is the ruling, evidenced by the decision issue being CLOSED and carrying
// DecidedLabel. An open issue keeps the hold, whatever its labels. A closed issue without
// DecidedLabel keeps it too: a closed container is not a lift by itself (it may have been
// closed as a duplicate or by mistake). A read error, a missing issue, or a number that names
// a change rather than an issue is could-not-check, which keeps the hold.
func RereadDecisionHold(f IssueGetter, fr ForgeRepo, number int) DecisionHold {
	h := DecisionHold{Repo: fr, Issue: number, State: DecisionHoldUnknown}
	if f == nil {
		h.Why = fmt.Sprintf("decision issue %s#%d could not be read: no forge", fr.Slug(), number)
		return h
	}
	if number <= 0 {
		h.Why = fmt.Sprintf("the hold names no decision issue in %s", fr.Slug())
		return h
	}
	item, err := f.GetIssue(fr, number)
	if err != nil || item == nil {
		h.Why = fmt.Sprintf("decision issue %s#%d could not be read", fr.Slug(), number)
		if err != nil {
			h.Why += ": " + err.Error()
		}
		return h
	}
	if item.IsPullRequest {
		h.Why = fmt.Sprintf("%s#%d is a change, not a decision issue", fr.Slug(), number)
		return h
	}
	decided := hasLabel(item.Labels, DecidedLabel)
	switch strings.ToLower(strings.TrimSpace(item.State)) {
	case "open":
		h.State = DecisionHoldLive
		h.Why = fmt.Sprintf("decision issue %s#%d is open", fr.Slug(), number)
	case "closed":
		if decided {
			h.State = DecisionHoldLifted
			h.Why = fmt.Sprintf("decision issue %s#%d is closed with %s", fr.Slug(), number, DecidedLabel)
		} else {
			h.State = DecisionHoldLive
			h.Why = fmt.Sprintf("decision issue %s#%d is closed without %s; a closed issue is not a lift by itself", fr.Slug(), number, DecidedLabel)
		}
	default:
		h.Why = fmt.Sprintf("decision issue %s#%d has unrecognised state %q", fr.Slug(), number, item.State)
	}
	return h
}

// ParseIssueRef parses an `owner/name#N` issue reference. It refuses a bare `#N` (the repo is
// part of the identity) and anything that is not a valid slug plus a positive number.
func ParseIssueRef(s string) (ForgeRepo, int, error) {
	slug, num, ok := strings.Cut(strings.TrimSpace(s), "#")
	if !ok || !ValidRepoSlug(slug) {
		return ForgeRepo{}, 0, fmt.Errorf("issue reference %q is not owner/name#N", s)
	}
	n, err := strconv.Atoi(num)
	if err != nil || n <= 0 || strings.TrimSpace(num) != num || strings.HasPrefix(num, "+") {
		return ForgeRepo{}, 0, fmt.Errorf("issue reference %q is not owner/name#N", s)
	}
	owner, name, _ := strings.Cut(slug, "/")
	return ForgeRepo{Owner: owner, Name: name}, n, nil
}
