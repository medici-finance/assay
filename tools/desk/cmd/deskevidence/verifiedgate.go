package main

// verifiedgate.go — the sidecar `outcome:"verified"` acceptance gate.
//
// deskevidence lands the verify-outcomes sidecar (docs/streams/verify-outcomes.jsonl)
// as an append-only file write. verify-desk appends one row per verified/failed
// brief; the `"outcome":"verified"` rows are what the Change-Failure-Rate
// denominator counts. But a `verified` row is written from the desk's own view of
// the run, not from the tree's board state — so when Evidence is filled while the
// stream table STAYS at `implemented` (the Verified cell still `—`, or a required
// execution witness is absent), the sidecar still records `verified`. `verifyloop`
// then buckets the mismatch as a STUCK-FLIP (verified sidecar + filled Evidence +
// status still implemented, #1309), which review correctly refuses to merge.
//
// This gate refuses to LAND a `verified` sidecar row unless the landing tree
// actually presents a lint-valid `verified` closure for that brief — the Verified
// stamp present AND every Verify row carrying a passing execution witness. The
// acceptance decision itself lives in statusgen (`verifyclosure`), reusing the
// board's own Status/Verified read and the witness audit; this gate only parses
// the rows THIS commit adds, asks statusgen per verified brief, and refuses on a
// no. `verify-fail` (and every other outcome) is never gated — only `verified`.
//
// Three-state (common-clause C4): a brief statusgen could not evaluate is
// could-not-check, which refuses the landing (Unverifiable), never a silent pass.

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// closureVerdict is statusgen verifyclosure's answer for one brief in one tree.
type closureVerdict int

const (
	closureAccepted    closureVerdict = iota // the tree presents a lint-valid verified closure
	closureNotAccepted                       // the stuck-flip shape: no stamp, or an unwitnessed/failing row
	closureCouldNotErr                       // could-not-check (unreadable/unknown brief); carried as an error
)

// verifiedGateOutcome is the value of a sidecar row's `outcome` field this gate
// keys on. Only `verified` is gated.
const verifiedGateOutcome = "verified"

// verifiedOutcomeGateRow is the subset of a verify-outcomes sidecar row the gate reads.
type verifiedOutcomeGateRow struct {
	Brief   string `json:"brief"`
	Outcome string `json:"outcome"`
}

// verifiedBriefsAdded returns, in first-seen order, the distinct brief keys whose
// rows THIS commit ADDS to the sidecar carry outcome `verified`. It diffs against
// the branch's current content (addedLines) so a re-land that re-states rows the
// branch already carries gates nothing new; a brand-new sidecar (no remote base)
// is read whole. A malformed line, or a row with no brief or a non-verified
// outcome, is skipped — exactly the tolerance readOutcomeSidecar already applies.
func verifiedBriefsAdded(remoteContent, commitContent []byte, remoteExists bool) []string {
	scan := commitContent
	if remoteExists {
		scan = addedLines(remoteContent, commitContent)
	}
	var out []string
	seen := map[string]bool{}
	for _, line := range strings.Split(string(scan), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var row verifiedOutcomeGateRow
		if json.Unmarshal([]byte(line), &row) != nil {
			continue
		}
		brief := strings.TrimSpace(row.Brief)
		if brief == "" || row.Outcome != verifiedGateOutcome {
			continue
		}
		if seen[brief] {
			continue
		}
		seen[brief] = true
		out = append(out, brief)
	}
	return out
}

// verifiedClosureCheckFn is the seam cmdEvidence's sidecar gate calls; tests stub
// it so they need no real statusgen on PATH or real board tree. Production:
// verifiedClosureCheckAt.
var verifiedClosureCheckFn = verifiedClosureCheckAt

// verifiedClosureCheckAt shells `statusgen verifyclosure --root <root> --brief <key>`
// from a neutral working directory (os.TempDir(), against an absolute root — the
// same shape statusgenLintAt uses so statusgen scans exactly root, never the
// process cwd) and maps its three-state exit code:
//
//   - exit 0 → closureAccepted, nil.
//   - exit 1 → closureNotAccepted, nil (the reason statusgen printed on stdout is
//     returned so the refusal can quote it).
//   - exit 2, or a run that could not start → closureCouldNotErr with an
//     Unverifiable error (could-not-check; refuses the landing rather than trust
//     an un-evaluable answer).
func verifiedClosureCheckAt(root, briefKey string) (closureVerdict, string, error) {
	if _, err := exec.LookPath("statusgen"); err != nil {
		return closureCouldNotErr, "", deskkit.Unverifiable(
			"statusgen is not on PATH — cannot check whether "+briefKey+" presents a lint-valid verified closure before landing its verified sidecar row", err)
	}
	absRoot, aerr := filepath.Abs(root)
	if aerr != nil {
		return closureCouldNotErr, "", deskkit.Unverifiable("cannot resolve an absolute path for the landing root "+root, aerr)
	}
	cmd := exec.Command("statusgen", "verifyclosure", "--root", absRoot, "--brief", briefKey)
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
		return closureCouldNotErr, "", deskkit.Unverifiable("could not run statusgen verifyclosure for "+briefKey+" at "+absRoot, err)
	}
	switch exitErr.ExitCode() {
	case 1:
		if reason == "" {
			reason = strings.TrimSpace(stderr.String())
		}
		return closureNotAccepted, reason, nil
	default: // 2 and anything else: could-not-check
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = reason
		}
		return closureCouldNotErr, "", deskkit.Unverifiable(
			"statusgen verifyclosure could not evaluate whether "+briefKey+" presents a lint-valid verified closure at "+absRoot+
				" — a verified sidecar row cannot be trusted without it: "+detail, nil)
	}
}

// gateVerifiedSidecarLanding refuses a verify-outcomes landing that would add one
// or more `verified` rows for briefs whose closure the landing tree does not
// accept. It is a no-op for a non-sidecar target, and for a sidecar landing that
// adds no `verified` rows (a pure `verify-fail` append never reaches statusgen).
// The FIRST not-accepted brief refuses the whole landing; a could-not-check on any
// brief refuses it as Unverifiable.
func gateVerifiedSidecarLanding(targetRepoPath, lintRoot string, remoteContent, commitContent []byte, remoteExists bool) error {
	if !isVerifyOutcomesSidecar(targetRepoPath) {
		return nil
	}
	briefs := verifiedBriefsAdded(remoteContent, commitContent, remoteExists)
	for _, brief := range briefs {
		verdict, reason, err := verifiedClosureCheckFn(lintRoot, brief)
		if err != nil {
			return err
		}
		if verdict == closureNotAccepted {
			msg := "refused: landing " + targetRepoPath + " would append an \"outcome\":\"verified\" row for " + brief +
				", but the tree at " + lintRoot + " does not present a lint-valid verified closure for it"
			if reason != "" {
				msg += " — " + reason
			}
			msg += ". Do not record a verified sidecar row until the brief's board is flipped to verified with an execution witness present (#1309 stuck-flip); a verify-fail row is unaffected."
			return deskkit.Refused(msg)
		}
	}
	return nil
}
