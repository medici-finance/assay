package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

type outcomeFlipPlan struct {
	Readme  string
	Updated []byte
}

var verifyOutcomeFlipPlanFn = statusgenOutcomeFlipPlan

func statusgenOutcomeFlipPlan(root, brief, source, runner, binding string, record []byte, options childOptions) (outcomeFlipPlan, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	cmd := statusgenAudit(ctx, auditFlip, root, options, "--brief", brief, "--sha", source, "--runner", runner,
		"--dry-run", "--plan-outcome", binding)
	cmd.Stdin = bytes.NewReader(record)
	var errout bytes.Buffer
	cmd.Stderr = &errout
	b, err := cmd.Output()
	if err != nil {
		if exit, ok := err.(*exec.ExitError); ok && exit.ExitCode() == 1 {
			return outcomeFlipPlan{}, deskkit.Refused("cannot plan verified transition: " + strings.TrimSpace(errout.String()))
		}
		return outcomeFlipPlan{}, deskkit.Unverifiable("cannot plan verified transition: "+strings.TrimSpace(errout.String()), err)
	}
	var plan outcomeFlipPlan
	if err := json.Unmarshal(b, &plan); err != nil {
		return outcomeFlipPlan{}, deskkit.Unverifiable("invalid statusgen outcome flip plan", err)
	}
	return plan, nil
}

// planOutcomeFlip judges the exact pinned target tree with both edits staged in
// its disposable checkout. The forge must commit that same pair on that same
// parent as the verifier App, or refuse the entire write.
func planOutcomeFlip(root, head, branch, target, repoSlug string, record []byte, fg deskkit.Forge, fr deskkit.ForgeRepo, receipt deskkit.VerifierReceipt, outcome deskkit.WakeReceipt, options childOptions) (deskkit.WriteFileInput, error) {
	if head == "" {
		return deskkit.WriteFileInput{}, deskkit.Unverifiable("a verified flip requires an exact landed parent", nil)
	}
	runner, ok := deskkit.RoleAppLogin("verifier")
	if !ok {
		return deskkit.WriteFileInput{}, deskkit.Unverifiable("a verified flip requires a bound verifier identity", nil)
	}
	plan, err := verifyOutcomeFlipPlanFn(root, outcome.Brief, receipt.Binding.Source, runner, receipt.EvidenceBinding(), record, options)
	if err != nil {
		return deskkit.WriteFileInput{}, err
	}
	readme := path.Join(path.Dir(receipt.Binding.Brief), "README.md")
	if plan.Readme != readme || len(plan.Updated) == 0 {
		return deskkit.WriteFileInput{}, deskkit.Unverifiable("statusgen plan did not name the admitted brief's board", nil)
	}
	if err := refuseLinkedHomePath(root, readme); err != nil {
		return deskkit.WriteFileInput{}, err
	}
	prior, err := fg.ReadFile(fr, deskkit.ReadFileInput{File: readme, Ref: head})
	if err != nil {
		return deskkit.WriteFileInput{}, err
	}
	if err := plainFileRead(prior, fr, readme, head, "the verified flip's pinned board", false); err != nil {
		return deskkit.WriteFileInput{}, err
	}
	original, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(readme)))
	if err != nil {
		return deskkit.WriteFileInput{}, deskkit.Unverifiable("cannot read the pinned board for the verified flip", err)
	}
	if !prior.Exists || prior.SHA == "" || !bytes.Equal(prior.Content, original) || bytes.Equal(original, plan.Updated) {
		return deskkit.WriteFileInput{}, deskkit.Refused("verified flip requires a change to the exact pinned board")
	}
	if err := evidenceOutboundCheck(repoSlug, readme, addedLines(original, plan.Updated), plan.Updated); err != nil {
		return deskkit.WriteFileInput{}, err
	}
	if err := checkPairedClosure(root, target, readme, plan.Updated, record, outcome.Brief, options); err != nil {
		return deskkit.WriteFileInput{}, err
	}
	return deskkit.WriteFileInput{
		File: readme, Branch: branch, Content: plan.Updated, ExpectedSHA: prior.SHA, ExpectedHead: head,
		Companions: []deskkit.CompanionFile{{File: target, Content: record}},
	}, nil
}

func checkPairedClosure(root, target, readme string, updated, record []byte, brief string, options childOptions) error {
	before, err := statusgenLintFn(root, options)
	if err != nil {
		return err
	}
	restore, err := stageBoardEdits(root, []boardEdit{{Path: readme, Content: updated}, {Path: target, Content: record}})
	if err != nil {
		return err
	}
	defer restore()
	if _, err := closureCheckFn(root, brief, options); err != nil {
		return err
	}
	if err := gateVerifiedSidecarLanding(target, root, nil, record, false, options); err != nil {
		return err
	}
	after, err := statusgenLintFn(root, options)
	if err != nil {
		return err
	}
	if err := requireCleanClosure(after, map[string]bool{brief: true}); err != nil {
		return err
	}
	introduced, excused := attributeIntroduced(before, after, flipScope(readme, brief))
	printExcused(stderr, readme, excused)
	if len(introduced) > 0 {
		return deskkit.Refused("verified flip would introduce statusgen PROBLEM(s):\n" + strings.Join(introduced, "\n"))
	}
	return nil
}
