package main

import (
	"bytes"
	"encoding/hex"
	"regexp"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

var outcomeEvidenceBindingFn = validateOutcomeEvidenceBinding

// An outcome may hold future verification work. Admission alone does not prove
// that this run's Evidence landed: its earlier write may have been refused.
// Read both the named commit and target branch before publishing a new receipt.
func validateOutcomeEvidenceBinding(root string, fg deskkit.Forge, fr deskkit.ForgeRepo, branch string, receipt deskkit.VerifierReceipt, sha string) error {
	b := receipt.Binding
	if b.Repo != fr.Slug() || b.Brief == "" || len(b.Source) != 40 || len(b.Run) != 64 || receipt.Issue < 1 {
		return deskkit.Refused("refused: outcome record has no complete admitted Evidence run")
	}
	for _, value := range []string{b.Source, b.Run, sha} {
		if _, err := hex.DecodeString(value); err != nil || value == "" {
			return deskkit.Refused("refused: outcome record has an invalid Evidence revision or run")
		}
	}
	if len(sha) != 40 {
		return deskkit.Refused("refused: outcome record must name the full Evidence commit SHA")
	}
	if err := receipt.CheckEvidenceTarget(b.Brief); err != nil {
		return err
	}
	atCommit, err := gitShowAt(root, sha, b.Brief)
	if err != nil {
		return deskkit.Unverifiable("cannot read the outcome record's Evidence commit", err)
	}
	landed, err := fg.ReadFile(fr, deskkit.ReadFileInput{File: b.Brief, Ref: branch})
	if err != nil {
		return deskkit.Unverifiable("cannot read landed Evidence for the outcome record", err)
	}
	if !landed.Exists || !bytes.Equal(atCommit, landed.Content) {
		return deskkit.Refused("refused: outcome record does not name the Evidence as landed on its target branch")
	}
	if err := receipt.CheckEvidenceContent(b.Brief, atCommit, nil); err != nil {
		return err
	}
	// The binding must be a real line inside Evidence, not a mention in Context,
	// a code example, or a previous run's stamp. Historical Evidence is allowed
	// alongside it, but cannot substitute for this exact admitted run.
	inEvidence := false
	fence := ""
	fenceLine := regexp.MustCompile("^ {0,3}(`{3,}|~{3,})(.*)$")
	for _, line := range strings.Split(string(atCommit), "\n") {
		if match := fenceLine.FindStringSubmatch(line); match != nil {
			if fence == "" {
				fence = match[1]
			} else if match[1][0] == fence[0] && len(match[1]) >= len(fence) && strings.TrimSpace(match[2]) == "" {
				fence = ""
			}
			continue
		}
		if fence != "" {
			continue
		}
		if strings.HasPrefix(line, "## ") {
			inEvidence = strings.TrimSpace(strings.TrimPrefix(line, "## ")) == "Evidence"
		}
		if inEvidence && strings.TrimRight(line, " \t\r") == receipt.EvidenceBinding() {
			return nil
		}
	}
	return deskkit.Refused("refused: outcome record has no landed Evidence for the current admitted run; land the Evidence successfully before writing its outcome")
}
