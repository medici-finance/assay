package deskkit

import "fmt"

// StampTimelineFor keeps issue and change numbers distinct on both forges.
func StampTimelineFor(f Forge, repo ForgeRepo, number int, kind TargetKind) (StampTimeline, error) {
	var tl StampTimeline
	var err error
	if kind == TargetIssue {
		var issue *Issue
		issue, err = f.GetIssueTyped(repo, number, TargetIssue)
		if err == nil && issue == nil {
			err = fmt.Errorf("missing issue")
		}
		if err == nil {
			tl.Present = issue.Labels
			tl.Events, err = f.ListIssueLabelEvents(repo, number)
		}
	} else if kind == TargetChange {
		var pr *PullRequest
		pr, err = f.GetPullRequest(repo, number)
		if err == nil && pr == nil {
			err = fmt.Errorf("missing change")
		}
		if err == nil {
			tl.Present = pr.Labels
			tl.Events, err = f.ListLabelEvents(repo, number)
		}
	} else {
		err = fmt.Errorf("unknown stamp target kind")
	}
	if err != nil {
		return tl, Unverifiable("cannot read model stamp provenance", err)
	}
	return tl, nil
}

// ApplyVerifiedModelStamp is the shared issue/change applier. Success means the
// requested pair was read back under accepted authority, including on a no-op.
func ApplyVerifiedModelStamp(f Forge, repo ForgeRepo, number int, kind TargetKind, labels []string, authority func(string) bool) error {
	return ApplyVerifiedStamp(f, repo, number, kind, StampDispatch, labels, authority)
}

// ApplyVerifiedStamp is ApplyVerifiedModelStamp for one stamp kind. It reads, removes and
// verifies only labels of that kind, so applying one pair never touches the other (#2760).
func ApplyVerifiedStamp(f Forge, repo ForgeRepo, number int, kind TargetKind, sk StampKind, labels []string, authority func(string) bool) error {
	expected, state := sk.StampOf(labels)
	if state != ModelStamped {
		return Refused("invalid requested model stamp")
	}
	tl, err := StampTimelineFor(f, repo, number, kind)
	if err != nil {
		return err
	}
	got, state := sk.AttestedStampOf(tl, authority)
	if state == ModelStamped && got == expected {
		return nil
	}
	remove := sk.ReStampRemovals(tl, labels, authority)
	if len(remove) > 0 {
		if _, err = f.ApplyLabels(repo, number, LabelChange{Target: kind, Remove: remove}); err != nil {
			return Unverifiable("cannot remove conflicting model stamp", err)
		}
	}
	var add []LabelSpec
	for _, label := range labels {
		add = append(add, LabelSpec{Name: label, Color: "ededed", Description: sk.LabelDescription()})
	}
	if _, err = f.ApplyLabels(repo, number, LabelChange{Target: kind, Add: add}); err != nil {
		return Unverifiable("cannot apply model stamp", err)
	}
	tl, err = StampTimelineFor(f, repo, number, kind)
	if err != nil {
		return err
	}
	got, state = sk.AttestedStampOf(tl, authority)
	if state != ModelStamped || got != expected {
		return Unverifiable("model stamp readback does not prove the requested dispatcher attestation", nil)
	}
	return nil
}
