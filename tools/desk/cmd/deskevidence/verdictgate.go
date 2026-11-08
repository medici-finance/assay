package main

// verdictgate.go — refuse a non-decisive Evidence append on a verified or done
// brief (#5759).
//
// A re-run of a brief that is already `verified` or `done` can come back BLOCKED
// or could-not-check for a reason outside the brief. Appended to Evidence, that
// run becomes the brief's latest recorded verdict, and the protected-closure
// lint then refuses the whole tree — after the PR is open, with the verdict
// already recorded and not to be struck. This gate asks the same question
// before any write: statusgen's own `evidenceverdict` subcommand reads the
// board status and the latest recorded verdict with the selector the lint uses
// (flipLatestPass's), so the two cannot drift. A FAIL (a reproducing
// regression) and a PASS still land; an implemented brief's BLOCKED run is
// untouched. The refused run is recorded in the outcome record only:
// `--outcome-record <file> --outcome-only` (outcomeonly.go).
//
// The rule is a whitelist: every verdict other than PASS and FAIL (BLOCKED,
// NEEDS-CONTEXT, PARTIAL, HELD, any other word) is refused on a verified or
// done brief, the same set the lint refuses.
//
// Where the status is read. The landing root is the admitted verifier home,
// pinned at the dispatch commit and so behind the target branch for a brief
// target (landedClosureTree returns it unchanged). The status this gate and the
// outcome-only check act on is the branch's: every board file the branch tip
// changed since the home's HEAD (decisionGateBranchBoardFn, the decision gate's
// own read) is staged over the home for the one statusgen call and put back
// after (evidenceVerdictAtTip). A tip that cannot be read, or a status that
// cannot be resolved, is could-not-check and refuses the landing.
//
// Fields read: a brief's board status and the latest VERIFY verdict of its
// Evidence, through statusgen. No Runner cell, witness record or revision is
// read here.

import (
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// evidenceVerdictCheckFn is the seam the verdict gate and the outcome-only
// record call; tests stub it so they need no real statusgen on PATH.
// Production: evidenceVerdictCheckAt.
var evidenceVerdictCheckFn = evidenceVerdictCheckAt

// isBriefFile reports whether a repo-relative path is a stream brief file
// (docs/streams/<stream>/brief-*.md).
func isBriefFile(p string) bool {
	p = path.Clean(filepath.ToSlash(p))
	return strings.HasPrefix(p, "docs/streams/") && strings.HasPrefix(path.Base(p), "brief-") && strings.HasSuffix(p, ".md")
}

// evidenceVerdictCheckAt shells `statusgen evidenceverdict` against root for the
// brief at briefPath. With verdict set it asks about that verdict word;
// otherwise it asks about the latest verdict in after (the brief as it would
// land), against before when the file already exists. Exit 0 → closureAccepted
// (landable), 1 → closureNotAccepted (the stdout reason is returned), anything
// else or a run that could not start → could-not-check.
func evidenceVerdictCheckAt(root, briefPath string, before, after []byte, hasBefore bool, verdict string) (closureVerdict, string, error) {
	if _, err := exec.LookPath("statusgen"); err != nil {
		return closureCouldNotErr, "", deskkit.Unverifiable(
			"statusgen is not on PATH — cannot check whether this run's verdict may be appended to "+briefPath+"'s Evidence", err)
	}
	absRoot, aerr := filepath.Abs(root)
	if aerr != nil {
		return closureCouldNotErr, "", deskkit.Unverifiable("cannot resolve an absolute path for the landing root "+root, aerr)
	}
	args := []string{"evidenceverdict", "--root", absRoot, "--brief-path", briefPath}
	if verdict != "" {
		args = append(args, "--verdict", verdict)
	} else {
		dir, err := os.MkdirTemp("", "deskevidence-verdict-")
		if err != nil {
			return closureCouldNotErr, "", deskkit.Unverifiable("cannot stage the brief text for the verdict check", err)
		}
		defer os.RemoveAll(dir)
		afterPath := filepath.Join(dir, "after.md")
		if err := os.WriteFile(afterPath, after, 0o600); err != nil {
			return closureCouldNotErr, "", deskkit.Unverifiable("cannot stage the brief text for the verdict check", err)
		}
		args = append(args, "--after", afterPath)
		if hasBefore {
			beforePath := filepath.Join(dir, "before.md")
			if err := os.WriteFile(beforePath, before, 0o600); err != nil {
				return closureCouldNotErr, "", deskkit.Unverifiable("cannot stage the brief text for the verdict check", err)
			}
			args = append(args, "--before", beforePath)
		}
	}
	cmd := exec.Command("statusgen", acceptanceArgs(args...)...)
	cmd.Dir = os.TempDir()
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	reason := strings.TrimSpace(stdout.String())
	if err == nil {
		return closureAccepted, reason, nil
	}
	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		return closureCouldNotErr, "", deskkit.Unverifiable("could not run statusgen evidenceverdict for "+briefPath+" at "+absRoot, err)
	}
	if exitErr.ExitCode() == 1 {
		if reason == "" {
			reason = strings.TrimSpace(stderr.String())
		}
		return closureNotAccepted, reason, nil
	}
	detail := strings.TrimSpace(stderr.String())
	if detail == "" {
		detail = reason
	}
	return closureCouldNotErr, "", deskkit.Unverifiable(
		"statusgen evidenceverdict could not evaluate whether this run's verdict may be appended to "+briefPath+" at "+absRoot+": "+detail, nil)
}

// evidenceVerdictAtTip is the ONLY caller of evidenceVerdictCheckFn: it stages
// tip (the target branch's board files) over root, asks statusgen, and puts
// every touched file back. TestVerdictCheckOnlyAtTip holds that to one call
// site, so no reader can ask the question against the home's stale board.
func evidenceVerdictAtTip(root string, tip []boardEdit, briefPath string, before, after []byte, hasBefore bool, verdict string) (closureVerdict, string, error) {
	restore, err := stageBoardEdits(root, tip)
	if err != nil {
		return closureCouldNotErr, "", err
	}
	defer restore()
	return evidenceVerdictCheckFn(root, briefPath, before, after, hasBefore, verdict)
}

// gateNonDecisiveVerdict refuses an Evidence landing on a brief file whose board
// status (at the target branch tip, tip staged over root) is verified or done
// when the landing's latest recorded verdict is any verdict other than PASS or
// FAIL. It is a no-op for any other target.
func gateNonDecisiveVerdict(targetRepoPath, root string, tip []boardEdit, before, after []byte, exists bool) error {
	if !isBriefFile(targetRepoPath) {
		return nil
	}
	verdict, reason, err := evidenceVerdictAtTip(root, tip, targetRepoPath, before, after, exists, "")
	if err != nil {
		return err
	}
	if verdict != closureNotAccepted {
		return nil
	}
	msg := "refused: landing " + targetRepoPath + " would append a verdict other than PASS or FAIL to a brief that is already verified or done"
	if reason != "" {
		msg += " — " + reason
	}
	msg += ". Only a reproducing FAIL or a fresh PASS earns an Evidence append; record this run in the outcome record only " +
		"(deskevidence --outcome-record <file> --outcome-only), with no Evidence PR."
	return deskkit.Refused(msg)
}
