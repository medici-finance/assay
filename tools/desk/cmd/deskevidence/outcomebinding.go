package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"maps"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

var outcomeEvidenceBindingFn = validateOutcomeEvidenceBinding
var outcomeObservationBindingFn = validateOutcomeObservationBinding

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

// Supplied receipts must obey the same observed-input contract as derived ones.
// Keep the Evidence commit binding and landed brief hash, while preventing a
// later repair from being substituted for the admitted observation's inputs.
func validateOutcomeObservationBinding(root string, fg deskkit.Forge, fr deskkit.ForgeRepo, branch string, receipt deskkit.VerifierReceipt, outcome deskkit.WakeReceipt) error {
	if !outcome.IsFailedOrBlocked() || outcome.WakePredicate != deskkit.WakeRelevantInputChanged {
		return nil
	}
	if err := validateOutcomeInputShape(root, receipt, outcome.SHA); err != nil {
		return err
	}
	observed := outcome
	observed.Schema = "" // rederive even a caller-supplied complete receipt
	raw, err := json.Marshal(observed)
	if err != nil {
		return err
	}
	expected, err := deriveOutcomeReceipt(raw, root, fg, fr, branch, receipt.Binding.Source)
	if err != nil {
		return err
	}
	var binding deskkit.WakeReceipt
	if err := json.Unmarshal(expected, &binding); err != nil {
		return err
	}
	// Retain the supplied tool-version contract; the new binding concerns
	// observed deliverables and the Evidence brief, not a tool upgrade.
	binding.Inputs["tool"] = outcome.ToolVersion
	if !maps.Equal(outcome.Inputs, binding.Inputs) {
		return deskkit.Refused("refused: outcome receipt inputs do not describe the admitted observation and landed Evidence")
	}
	return nil
}

// The existing receipt hashes present files, not absence or directory membership.
// Changed membership/kinds need a new admission instead of a stale failure hold.
func validateOutcomeInputShape(root string, receipt deskkit.VerifierReceipt, evidenceSHA string) error {
	source, err := newGitOutcomeTree(root, receipt.Binding.Source)
	if err != nil {
		return err
	}
	evidence, err := newGitOutcomeTree(root, evidenceSHA)
	if err != nil {
		return err
	}
	sourceBrief, err := source.ReadFile(receipt.Binding.Brief)
	if err != nil {
		return deskkit.Unverifiable("cannot read observed input declarations", err)
	}
	evidenceBrief, err := evidence.ReadFile(receipt.Binding.Brief)
	if err != nil {
		return deskkit.Unverifiable("cannot read Evidence input declarations", err)
	}
	paths := func(content []byte) []string {
		unique := map[string]bool{}
		for _, decl := range deskkit.BriefDeclaredFiles(string(content)) {
			if p := strings.TrimSuffix(decl, "/"); p != "" {
				unique[p] = true
			}
		}
		var result []string
		for p := range unique {
			result = append(result, p)
		}
		sort.Strings(result)
		return result
	}
	declared := paths(sourceBrief)
	changed := func() error {
		return deskkit.Refused("refused: declared deliverables changed shape between admitted Source and Evidence; fresh independent verification on the current target branch is required before recording an outcome")
	}
	if !slices.Equal(declared, paths(evidenceBrief)) {
		return changed()
	}
	for _, p := range declared {
		before, err := source.PathKind(p)
		if err != nil {
			return deskkit.Unverifiable("cannot read observed input kind", err)
		}
		after, err := evidence.PathKind(p)
		if err != nil {
			return deskkit.Unverifiable("cannot read Evidence input kind", err)
		}
		if before != after {
			return changed()
		}
		if before == "tree" {
			oldFiles, err := source.ListFiles(p)
			if err != nil {
				return deskkit.Unverifiable("cannot read observed directory inputs", err)
			}
			newFiles, err := evidence.ListFiles(p)
			if err != nil {
				return deskkit.Unverifiable("cannot read Evidence directory inputs", err)
			}
			sort.Strings(oldFiles)
			sort.Strings(newFiles)
			if !slices.Equal(oldFiles, newFiles) {
				return changed()
			}
		}
	}
	return nil
}
