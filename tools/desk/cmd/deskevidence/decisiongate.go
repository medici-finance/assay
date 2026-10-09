package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// decisiongate.go — the decision-gate hold's network layer on an evidence landing
// (spec/lifecycle-v1.md §4.5).
//
// A gate: human brief moves to implemented, verified or done only once a human has
// ruled on its decision issue. The offline layer of that hold is part of
// `statusgen --lint`, so the PROBLEM-diff guard (lintdiff.go) already refuses a
// landing that adds a status move with no well-formed ruling: link. It cannot tell
// who wrote the linked comment. An evidence landing rides the Contents API straight
// to the default branch, with no pull request, so the pull-request check that reads
// the comment's author never sees it. This file runs that check here instead:
// `statusgen --decision-gate` before and after staging the write, refusing only the
// REFUSED lines the write adds.
//
// Same shape and the same rules as lintdiff.go: the stage is always reverted (the
// real write rides the Contents API); statusgen not on PATH, or one too old to know
// --decision-gate, is could-not-check, never a silent pass.

// decisionGateDiffFn is the seam cmdEvidence calls. setupFake stubs it to
// "introduces nothing"; decisiongate_test.go drives decisionGateDiffAt directly.
var decisionGateDiffFn = decisionGateDiffAt

// statusgenDecisionGateFn is ONE `statusgen --decision-gate` shell, returning the
// REFUSED lines it printed. Production: statusgenDecisionGateAt.
var statusgenDecisionGateFn = statusgenDecisionGateAt

// decisionGateDiffAt runs the decision gate on root as it stands, stages newContent
// at targetRepoPath, runs it again, reverts the stage, and returns the REFUSED lines
// present only after the stage — what THIS landing would introduce.
func decisionGateDiffAt(root, targetRepoPath string, newContent []byte) (introduced []string, err error) {
	before, err := statusgenDecisionGateFn(root)
	if err != nil {
		return nil, err
	}
	abs := filepath.Join(root, filepath.FromSlash(targetRepoPath))
	orig, rerr := os.ReadFile(abs)
	existed := rerr == nil
	if rerr != nil && !os.IsNotExist(rerr) {
		return nil, deskkit.Unverifiable("cannot read local "+abs+" to stage the decision-gate check", rerr)
	}
	if merr := os.MkdirAll(filepath.Dir(abs), 0o755); merr != nil {
		return nil, deskkit.Unverifiable("cannot create local directories under "+root+" to stage the decision-gate check", merr)
	}
	if werr := os.WriteFile(abs, newContent, 0o644); werr != nil {
		return nil, deskkit.Unverifiable("cannot stage the decision-gate check at "+abs, werr)
	}
	defer func() {
		if existed {
			_ = os.WriteFile(abs, orig, 0o644)
		} else {
			_ = os.Remove(abs)
		}
	}()
	after, aerr := statusgenDecisionGateFn(root)
	if aerr != nil {
		return nil, aerr
	}
	had := make(map[string]bool, len(before))
	for _, l := range before {
		had[l] = true
	}
	seen := map[string]bool{}
	for _, l := range after {
		if had[l] || seen[l] {
			continue
		}
		seen[l] = true
		introduced = append(introduced, l)
	}
	return introduced, nil
}

// decisionGateRefusedMark is the separator statusgen prints between a board id and
// the reason on a refused line ("<board> REFUSED — <reason>").
const decisionGateRefusedMark = " REFUSED — "

// decisionGateWholeChange is the board label statusgen uses when it fails closed on
// the change as a whole rather than on one brief.
const decisionGateWholeChange = "(board)" + decisionGateRefusedMark

// statusgenDecisionGateAt shells `statusgen --root <root> --decision-gate
// --decision-gate-base HEAD` from a neutral working directory: the working tree
// judged against its own HEAD, i.e. exactly the uncommitted write.
//
//   - exit 0                     → nothing refused.
//   - exit 1 with ≥1 REFUSED line → those lines.
//   - anything else (exit 1 with no REFUSED line, exit 2, an unknown flag from a
//     statusgen that predates the hold) → could-not-check.
func statusgenDecisionGateAt(root string) ([]string, error) {
	if _, err := exec.LookPath("statusgen"); err != nil {
		return nil, deskkit.Unverifiable("statusgen is not on PATH — cannot run the decision-gate check (lifecycle-v1 §4.5) on this landing", err)
	}
	cmd := exec.Command("statusgen", "--root", root, "--decision-gate", "--decision-gate-base", "HEAD")
	cmd.Dir = os.TempDir()
	out, cerr := cmd.CombinedOutput()
	return parseDecisionGateRun(root, string(out), cerr)
}

// parseDecisionGateRun reads one `statusgen --decision-gate` run (split out so the
// exit/line reading is testable without a statusgen binary).
func parseDecisionGateRun(root, out string, cerr error) ([]string, error) {
	var refused []string
	for _, ln := range strings.Split(out, "\n") {
		t := strings.TrimSpace(ln)
		if strings.Contains(t, decisionGateRefusedMark) {
			refused = append(refused, t)
		}
	}
	if cerr == nil {
		return nil, nil
	}
	ee, ok := cerr.(*exec.ExitError)
	if !ok {
		return nil, deskkit.Unverifiable("could not run statusgen --decision-gate at "+root, cerr)
	}
	if strings.Contains(out, "flag provided but not defined") {
		return nil, deskkit.Unverifiable("the statusgen on PATH predates the decision-gate hold (no --decision-gate flag) — upgrade statusgen to the release that ships this deskevidence; a landing is never let through unchecked", cerr)
	}
	if ee.ExitCode() == 1 && len(refused) > 0 {
		// A "(board)" refusal is statusgen failing closed on the whole change (the base
		// does not resolve, or the board cannot be read at one end): no brief was judged.
		// Diffing it before/after would cancel it out and let the landing through, so it
		// is could-not-check here, never a pass.
		for _, l := range refused {
			if strings.HasPrefix(l, decisionGateWholeChange) {
				return nil, deskkit.Unverifiable("statusgen --decision-gate could not judge "+root+": "+l, cerr)
			}
		}
		return refused, nil
	}
	msg := "statusgen --decision-gate could not evaluate " + root + " (exit nonzero with no REFUSED line)"
	if t := strings.TrimSpace(out); t != "" {
		if i := strings.LastIndex(t, "\n"); i >= 0 {
			t = t[i+1:]
		}
		msg += ": " + t
	}
	return nil, deskkit.Unverifiable(msg, cerr)
}
