package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// Moving an outstanding decision off the queue needs more than authorship:
// the human must actually have chosen an offered option on this issue.
func verifyQueueReply(it item, ruling ghComment, thread []deskkit.Comment, idx int, rec deskkit.HumanDecisionRecord) error {
	c := thread[idx]
	if c.Minimized || c.Body != ruling.Body || !deskkit.IsBlessAuthorityIDStrict(c.Author.Login, c.Author.ID) {
		return deskkit.Refused("refused: the cited human reply is hidden or changed")
	}
	if rec.Picked == deskkit.DecisionPickUnparsed || rec.Picked == deskkit.DecisionPickAmbiguous {
		return deskkit.Refused("refused: the cited reply does not resolve to an offered choice")
	}
	if rec.EditState == deskkit.DecisionEditUnverifiable {
		return deskkit.Unverifiable("could-not-check the edit history of the decision and ruling", nil)
	}
	if rec.EditState != deskkit.DecisionEditUnedited {
		return deskkit.Refused("refused: the question or ruling changed after the decision")
	}
	for _, later := range thread[idx+1:] {
		if deskkit.IsBlessAuthorityIDStrict(later.Author.Login, later.Author.ID) {
			return deskkit.Refused("refused: the human has replied again since the cited decision")
		}
	}
	if deskkit.DecisionAnchor(thread[idx+1:]) >= 0 {
		return deskkit.Refused("refused: a newer decision question follows the cited ruling")
	}
	created, cerr := time.Parse(time.RFC3339, c.CreatedAt)
	updated, uerr := time.Parse(time.RFC3339, c.UpdatedAt)
	if cerr != nil || uerr != nil || !created.Equal(updated) {
		return deskkit.Refused("refused: the cited reply must be unedited")
	}
	answer := strings.TrimSpace(c.Body)
	if answer != strings.TrimSpace(deskkit.VisibleDecisionText(c.Body)) {
		return deskkit.Refused("refused: the decision must be the human’s own visible reply")
	}
	if strings.Contains(it.Body, "<!-- decision-gate:") {
		if rec.Brief == "" || !strings.HasPrefix(answer, rec.Brief+":") {
			return deskkit.Refused("refused: the ruling must name this decision's board ID")
		}
		answer = strings.TrimSpace(strings.TrimPrefix(answer, rec.Brief+":"))
	} else if rec.PickedVia == deskkit.DecisionViaRatified {
		switch strings.ToLower(answer) {
		case "ratified", "i ratify", "approved":
			return nil // ParseRulingPick matched the trusted relay to an offered option.
		}
	}
	answer = strings.TrimPrefix(answer, "Option ")
	for _, opt := range rec.Options {
		if answer == opt.ID && rec.Picked == opt.ID {
			return nil
		}
	}
	return deskkit.Refused("refused: the cited reply does not select one offered option unambiguously")
}

// An interrupted run may already have removed needs-decision. Keep the same
// proof requirement on retry when the remaining decided label was applied by
// an App. Pre-existing manual dispositions retain their original close path.
func decisionQueueHistory(r triageReq, it item) (*deskkit.LabelHistory, error) {
	if !hasLabel(it, labelNeedsDecision) && !hasLabel(it, dispositionHumanDecided) {
		return nil, nil
	}
	fg, fr, err := forgeForFn(r.repo)
	if err != nil {
		return nil, err
	}
	history, err := fg.IssueLabelHistory(fr, r.number)
	if err != nil || history == nil || !history.Complete {
		return nil, deskkit.Unverifiable("could-not-check the decision issue's label history", err)
	}
	if hasLabel(it, labelNeedsDecision) {
		return history, nil
	}
	for i := len(history.Events) - 1; i >= 0; i-- {
		event := history.Events[i]
		if strings.EqualFold(event.Name, dispositionHumanDecided) {
			if !event.Removed && !deskkit.IsBlessAuthorityIDStrict(event.AppliedBy, event.AppliedByID) {
				return history, nil
			}
			break
		}
	}
	return nil, nil
}

// A reopened issue or a new request for a decision starts a new round, even
// if no one commented. Compare the ruling with both provider histories.
func verifyDecisionRound(fg deskkit.Forge, fr deskkit.ForgeRepo, n int, rulingTime string, labels *deskkit.LabelHistory) error {
	ruled, err := time.Parse(time.RFC3339, rulingTime)
	if err != nil {
		return deskkit.Unverifiable("could-not-check decision timestamp", err)
	}
	check := func(stamp string) error {
		at, err := time.Parse(time.RFC3339, stamp)
		if err != nil {
			return deskkit.Unverifiable("could-not-check when the decision was requeued", err)
		}
		if !at.Before(ruled) {
			return deskkit.Refused("refused: the decision must follow the latest request or reopening")
		}
		return nil
	}
	for _, e := range labels.Events {
		if !e.Removed && (e.Name == "" || strings.EqualFold(e.Name, labelNeedsDecision)) {
			if err := check(e.CreatedAt); err != nil {
				return err
			}
		}
	}
	state, err := fg.IssueStateEvents(fr, n)
	if err != nil || state == nil || !state.Complete {
		return deskkit.Unverifiable("could-not-check the decision issue's reopen history", err)
	}
	for _, e := range state.Events {
		if e.Kind == deskkit.IssueEventReopened {
			if err := check(e.CreatedAt); err != nil {
				return err
			}
		}
	}
	return nil
}

// Add and observe the decided label before removing the queue label. Some
// backends remove first in a combined change. A failed add must leave the
// original queue visible; an interrupted transition can resume with both labels.
func moveDecidedLabels(r triageReq, it item, a *auditCtx) error {
	fg, fr, err := forgeForFn(r.repo)
	if err != nil {
		return err
	}
	for _, step := range []struct {
		name string
		want bool
	}{
		{dispositionHumanDecided, true},
		{labelNeedsDecision, false},
	} {
		if hasLabel(it, step.name) == step.want {
			continue
		}
		if err := allowWrite(r.repo, r.number); err != nil {
			return err
		}
		change := deskkit.LabelChange{Target: deskkit.TargetIssue}
		if step.want {
			change.Add = []deskkit.LabelSpec{{Name: step.name, Color: "0e8a16", Description: "The driver recorded a decision on this issue"}}
		} else {
			change.Remove = []string{step.name}
		}
		if _, err := fg.ApplyLabels(fr, r.number, change); err != nil {
			return deskkit.Unverifiable("could-not-check decision label write for "+step.name, err)
		}
		a.log(deskkit.ResultOK, "wrote decision label change for "+step.name)
		it, err = fetchItem(r.repo, r.number, deskkit.TargetIssue)
		if err != nil {
			return err
		}
		if !hasLabel(it, dispositionHumanDecided) || hasLabel(it, step.name) != step.want {
			return deskkit.Unverifiable(fmt.Sprintf("could-not-check: decision label %s did not reach its required state", step.name), nil)
		}
	}
	return nil
}
