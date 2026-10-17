package main

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// The label-removal hold.
//
// An excluded system-state label (scanExcludedLabelSet) parks an issue: no placeholder is
// created while it is on, and a retired placeholder is not reactivated. The scanners used to
// read only the issue's CURRENT labels, so anyone with label-write on the repo could un-park
// an undecided issue by removing the label. This file is the layer behind that: before a
// create or a reactivate plan for an OPEN issue that carries no excluded label now, the whole
// label history is read and the issue is held unless the removal was made by a declared human
// or the ratifying identity has since commented.
//
// The rule is stateless. It never classifies an actor by login shape, account type or the
// trusted-bot roster: "declared human" is the id-pinned authorized-author set and "ratifying
// comment" is the id-pinned blessing authority, both of which the roster already holds. The
// text of a comment is never read.

const (
	// labelHoldClause is the clause a hold records; labelHoldClauseCNC is the clause for a
	// hold whose reason is that the history (or the comments it needs) could not be read.
	labelHoldClause    = "label-removal-hold"
	labelHoldClauseCNC = "label-removal-hold (could-not-check)"
)

// labelHoldVerdict is the decision for one issue. The zero value is clear. CouldNotCheck
// implies a hold: the history could not be read in full, which is never a pass.
type labelHoldVerdict struct {
	Held          bool
	CouldNotCheck bool
	Label         string // the excluded label the hold is about ("" for an unnamed event or a read failure)
	Actor         string // the actor of the event that holds, AS THE FORGE RENDERED THE LOGIN
	Detail        string // one sentence: what holds, and what releases it
}

// IsHold reports whether the issue must not be created or reactivated.
func (v labelHoldVerdict) IsHold() bool { return v.Held || v.CouldNotCheck }

// Clause is the clause name a hold is recorded under.
func (v labelHoldVerdict) Clause() string {
	if v.CouldNotCheck {
		return labelHoldClauseCNC
	}
	return labelHoldClause
}

// labelHoldChecker evaluates the hold for one open issue. The default is
// defaultScanLabelHoldChecker; tests inject a fixture. A nil checker, and a checker that
// returns an error, both hold as could-not-check.
type labelHoldChecker func(repo string, issue int) (labelHoldVerdict, error)

func holdCNC(format string, args ...any) labelHoldVerdict {
	return labelHoldVerdict{Held: true, CouldNotCheck: true, Detail: fmt.Sprintf(format, args...)}
}

// labelHoldGate runs the checker for one issue and folds every shape into (held, clause,
// detail). It is the ONE place a nil checker, a checker error and a could-not-check verdict
// become a hold, so the two scanners cannot drift on it.
func labelHoldGate(check labelHoldChecker, repo string, issue int) (held bool, clause, detail string) {
	if check == nil {
		return true, labelHoldClauseCNC, "no label-history check is configured, so whether the excluded label was removed by a declared human could not be established"
	}
	v, err := check(repo, issue)
	if err != nil {
		return true, labelHoldClauseCNC, fmt.Sprintf("the label history could not be checked — %v", err)
	}
	if !v.IsHold() {
		return false, "", ""
	}
	return true, v.Clause(), v.Detail
}

// labelHoldMemo caches labelHoldGate per (repo, issue) for the length of one plan, so an
// issue reached by more than one path costs one read and yields one message.
type labelHoldMemo struct {
	check labelHoldChecker
	seen  map[string]labelHoldResult
}

type labelHoldResult struct {
	held   bool
	clause string
	detail string
}

func newLabelHoldMemo(check labelHoldChecker) *labelHoldMemo {
	return &labelHoldMemo{check: check, seen: map[string]labelHoldResult{}}
}

func (m *labelHoldMemo) gate(repo string, issue int) (held bool, clause, detail string) {
	key := fmt.Sprintf("%s#%d", repo, issue)
	if r, ok := m.seen[key]; ok {
		return r.held, r.clause, r.detail
	}
	h, c, d := labelHoldGate(m.check, repo, issue)
	m.seen[key] = labelHoldResult{h, c, d}
	return h, c, d
}

// declaredHuman reports whether the actor is a declared human: the login is in the
// authorized-author set AND the forge-reported numeric id equals the pinned one. An id of 0
// (no actor, or one the forge did not report) is never a declared human.
func declaredHuman(login string, id int64) bool {
	if id == 0 {
		return false
	}
	want, ok := authorizedAuthorSet()[strings.ToLower(strings.TrimSpace(login))]
	return ok && want == id
}

func labelEventTime(s string) (time.Time, bool) {
	t, err := time.Parse(time.RFC3339, strings.TrimSpace(s))
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

func renderActor(login string, id int64) string {
	if strings.TrimSpace(login) == "" {
		return "an unidentified actor"
	}
	return login
}

// decideLabelHold is the pure decision. excluded is the excluded label set (lowercase),
// events the issue's whole label history oldest first, complete whether the forge served all
// of it, and comments a lazy reader for the issue's comments — called at most once, and only
// when a comparison needs it.
//
// The caller has established the issue is OPEN and carries no excluded label now.
func decideLabelHold(excluded map[string]bool, events []labelEvent, complete bool, comments func() ([]issueComment, error)) labelHoldVerdict {
	// Step 1: a history that is not the whole history decides nothing, and no comment
	// releases it.
	if !complete {
		return holdCNC("the label history is incomplete (the forge read stopped at its page bound), so a removal of an excluded label could be hidden in the part that was not read")
	}

	// Comments are read lazily and once; a failed read is remembered.
	var (
		loaded   bool
		ratifier []time.Time // creation times of ratifying comments (those that parse)
		loadErr  error
	)
	load := func() error {
		if loaded {
			return loadErr
		}
		loaded = true
		if comments == nil {
			loadErr = fmt.Errorf("no comment reader")
			return loadErr
		}
		cs, err := comments()
		if err != nil {
			loadErr = err
			return err
		}
		for _, c := range cs {
			if !isBlessAuthorityID(c.User.Login, c.User.ID) {
				continue
			}
			if t, ok := labelEventTime(c.CreatedAt); ok {
				ratifier = append(ratifier, t)
			} // a ratifying comment with an unparsable time never releases
		}
		return nil
	}

	// released: is there a ratifying comment STRICTLY after the event at refAt? Equal times do
	// not release. An unparsable reference time cannot be compared, which is a plain hold.
	// The second result is a could-not-check verdict when the comments are unreadable.
	released := func(refAt string) (bool, *labelHoldVerdict) {
		ref, ok := labelEventTime(refAt)
		if !ok {
			return false, nil
		}
		if err := load(); err != nil {
			v := holdCNC("the issue's comments could not be read, so whether the ratifying identity has commented since could not be established — %v", err)
			return false, &v
		}
		for _, t := range ratifier {
			if t.After(ref) {
				return true, nil
			}
		}
		return false, nil
	}

	// Step 2: an event whose label the forge no longer names might have been any excluded
	// label. It clears only on a ratifying comment strictly after the LATEST such event.
	lastUnnamed := -1
	for i, e := range events {
		if strings.TrimSpace(e.Label) == "" {
			lastUnnamed = i
		}
	}
	if lastUnnamed >= 0 {
		e := events[lastUnnamed]
		ok, cnc := released(e.At)
		for _, u := range events {
			if strings.TrimSpace(u.Label) != "" {
				continue
			}
			if _, parses := labelEventTime(u.At); !parses {
				ok, cnc = false, nil
			}
		}
		if cnc != nil {
			return *cnc
		}
		if !ok {
			return labelHoldVerdict{
				Held:  true,
				Label: "",
				Actor: renderActor(e.Login, e.ID),
				Detail: fmt.Sprintf("the label history holds an event by %s at %s on a label the forge no longer names, which may have been an excluded label; it is released when the ratifying identity comments on the issue after that time",
					renderActor(e.Login, e.ID), e.At),
			}
		}
	}

	// Step 3: each excluded label with a named event, in a fixed order, judged by its LAST event.
	names := make([]string, 0, len(excluded))
	for l := range excluded {
		names = append(names, l)
	}
	sort.Strings(names)
	for _, label := range names {
		last := -1
		lastApply := -1
		for i, e := range events {
			if strings.ToLower(strings.TrimSpace(e.Label)) != label {
				continue
			}
			last = i
			if !e.Removed {
				lastApply = i
			}
		}
		if last < 0 {
			continue
		}
		e := events[last]
		who := renderActor(e.Login, e.ID)
		if !e.Removed {
			// The last thing that happened to the label was an application, yet it is absent
			// now. The history does not explain the absence; a comment does not release it.
			return labelHoldVerdict{
				Held:  true,
				Label: label,
				Actor: who,
				Detail: fmt.Sprintf("excluded label %q was last applied by %s at %s and is absent now with no removal in the history; it is released only when a declared human applies and removes it",
					label, who, e.At),
			}
		}
		if declaredHuman(e.Login, e.ID) {
			continue
		}
		refAt, refWhat := e.At, "the removal"
		if lastApply >= 0 {
			refAt, refWhat = events[lastApply].At, "the latest application"
		}
		// A comparison is only as good as the times behind it: an event of this label whose
		// time cannot be read leaves the hold standing, whichever event the comparison keys on.
		ok, cnc := released(refAt)
		if _, parses := labelEventTime(e.At); !parses {
			ok, cnc = false, nil
		}
		if cnc != nil {
			v := *cnc
			v.Label = label
			return v
		}
		if !ok {
			return labelHoldVerdict{
				Held:  true,
				Label: label,
				Actor: who,
				Detail: fmt.Sprintf("excluded label %q was removed by %s at %s, who is not a declared human; it is released when the ratifying identity comments on the issue after %s (%s)",
					label, who, e.At, refAt, refWhat),
			}
		}
	}

	// Step 5: nothing holds.
	return labelHoldVerdict{}
}

// deskreadLabelHoldChecker is the default checker: the label history first, through
// deskread's `label-events` kind, and the comments only when the decision needs them. Every
// read failure — an error, a partial envelope, a kind this deskread rejects, a history with
// no completeness field, an unreadable comment thread — is a could-not-check hold, never a
// pass.
func deskreadLabelHoldChecker(repo string, issue int) (labelHoldVerdict, error) {
	rd := newDeskreadReader()
	h, err := rd.IssueLabelHistory(repo, issue)
	if err != nil {
		return holdCNC("the label history could not be read — %v", err), nil
	}
	return decideLabelHold(scanExcludedLabelSet(), h.Events, h.Complete, func() ([]issueComment, error) {
		return rd.IssueComments(repo, issue)
	}), nil
}

// defaultScanLabelHoldChecker is the label-removal check the production scan paths run. It
// is a var with a stable name so main.go wires it and tests can drive the entry points on
// exactly what main wires.
var defaultScanLabelHoldChecker labelHoldChecker = deskreadLabelHoldChecker
