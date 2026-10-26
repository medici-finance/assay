package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/medici-finance/assay/deskcore/issueflow"
)

// Companion files live beside the source record. OpenRoot prevents a symlink
// from turning a source-controlled pointer into an arbitrary local file read.
func readIntakeHandoff(sourcePath, name string) (issueflow.IntakeHandoff, error) {
	if name == "" || filepath.Base(name) != name || !strings.HasSuffix(name, ".json") {
		return issueflow.IntakeHandoff{}, fmt.Errorf("missing or invalid handoff filename")
	}
	dir, err := os.OpenRoot(filepath.Dir(sourcePath))
	if err != nil {
		return issueflow.IntakeHandoff{}, err
	}
	defer dir.Close()
	raw, err := dir.ReadFile(name)
	if err != nil {
		return issueflow.IntakeHandoff{}, err
	}
	return issueflow.DecodeIntakeHandoff(raw)
}

func assessRecordedIntake(h issueflow.IntakeHandoff, item, opened string, labels []string, complete bool) issueflow.IntakeAssessment {
	facts := issueflow.IntakeFacts{Item: item, Opened: opened, Complete: complete, Candidate: issueflow.CandidateForLabels(issueflow.Candidate{Excluded: hasExcludedLabel(labels)}, labels)}
	for _, label := range labels {
		switch strings.ToLower(strings.TrimSpace(label)) {
		case "needs-decision", "question":
			facts.Candidate.DecisionOwed = true
		case "help wanted":
			facts.Candidate.HelpWanted = true
		}
	}
	if h.Verification != nil {
		facts.Verification = issueflow.HandoffFacts{Source: h.Verification.Source, Complete: false, Now: time.Now().UTC()}
	}
	return issueflow.AssessIntakeHandoff(h, facts)
}

// Separate explicit export: --intake-exits retains its exact v1 output shape.
// Unstamped/legacy entries are emitted as classification-needed, never omitted
// because their legacy disposition happened to place them under completed/.
func writeIntakeHandoffs(root string, w, errw io.Writer) int {
	entries, err := loadIntake(root)
	if err != nil {
		fmt.Fprintln(errw, "intake handoffs: could-not-check:", err)
		return 6
	}
	enc := json.NewEncoder(w)
	result := 0
	for _, e := range entries {
		h, err := readIntakeHandoff(e.Path, e.Handoff)
		complete := err == nil && intakeStamped(e) && e.TriagedBy == h.DecidingRole && (h.Disposition != issueflow.NeedsScoping || e.TriagerTier == "strong")
		a := assessRecordedIntake(h, e.ID, e.HandoffOpened, nil, complete)
		if e.Handoff != "" && err != nil {
			result = 6
		}
		if err := enc.Encode(struct {
			Item   string                   `json:"item"`
			State  string                   `json:"state"`
			Reason string                   `json:"reason"`
			Record *issueflow.IntakeHandoff `json:"record"`
		}{e.ID, a.State, a.Reason, func() *issueflow.IntakeHandoff {
			if err != nil {
				return nil
			}
			return &h
		}()}); err != nil {
			fmt.Fprintln(errw, err)
			return 1
		}
	}
	return result
}
