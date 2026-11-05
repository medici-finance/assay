package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/medici-finance/assay/deskcore/issueflow"
)

func assessIssueHandoff(ph placeholderRow, item string, labels []string, now time.Time) issueflow.IntakeAssessment {
	facts := issueflow.IntakeFacts{Item: item, Opened: ph.HandoffOpened, Candidate: issueflow.CandidateForLabels(issueflow.Candidate{Excluded: hasExcludedLabel(labels), DecisionOwed: hasDecisionLabel(labels)}, labels)}
	for _, label := range labels {
		if strings.EqualFold(strings.TrimSpace(label), "help wanted") {
			facts.Candidate.HelpWanted = true
		}
	}
	h, err := readIssueHandoff(ph)
	facts.Complete = err == nil
	if h.Verification != nil {
		facts.Verification = issueflow.HandoffFacts{Source: h.Verification.Source, Complete: false, Now: now}
	}
	return issueflow.AssessIntakeHandoff(h, facts)
}

func readIssueHandoff(ph placeholderRow) (issueflow.IntakeHandoff, error) {
	if ph.Handoff == "" || filepath.Base(ph.Handoff) != ph.Handoff || !strings.HasSuffix(ph.Handoff, ".json") {
		return issueflow.IntakeHandoff{}, fmt.Errorf("missing or invalid handoff filename")
	}
	dir, err := os.OpenRoot(filepath.Dir(ph.Path))
	if err != nil {
		return issueflow.IntakeHandoff{}, err
	}
	defer dir.Close()
	raw, err := dir.ReadFile(ph.Handoff)
	if err != nil {
		return issueflow.IntakeHandoff{}, err
	}
	return issueflow.DecodeIntakeHandoff(raw)
}
