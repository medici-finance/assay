package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/verifycmd"
)

// facts.go — gather the RowFacts the classifier decides on, from a real checkout: which
// path the row pins and whether it still exists, the single rename hop git records for a
// missing one, and (only when the pinned path is intact) whether the row's command still
// runs and returns a different result. Gathering is kept OUT of classify.go so the decision
// is a pure function of facts; the git and command seams below are package vars so a test
// can supply a fixture without a checkout.

// gitOutput runs `git -C dir <args...>` and returns stdout (trimmed) and whether it
// succeeded. It is a package var so tests can stub git. It never contacts a remote — every
// call the gatherer makes reads local history only (offline envelope, common clause C3).
var gitOutput = func(dir string, args ...string) (string, bool) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	return strings.TrimSpace(string(out)), err == nil
}

// runCommand runs the row's command in dir under `sh -c` and reports whether it EXECUTED at
// all (ran), its exit code and its combined output (the Expect judge reads it). ran is false for the shell's could-not-execute codes (127
// command-not-found, 126 not-executable): those are stale-path signals, not behaviour
// changes. It is a package var so tests can probe behaviour without executing anything.
//
// The row command runs even in dry-run (the default) — the behaviour probe IS the
// classification signal, so classification has a side effect: it executes trusted-authored
// Verify-row shell. That is documented in docs/rebaseline.md ("dry-run executes the row
// command"). As defence in depth for the offline envelope (common clause C3) the probe
// forces KUBECONFIG=/dev/null in the child's environment, so a row command that would reach a
// cluster cannot, whatever the ambient KUBECONFIG (F-dryrun-executes-command, PR #1511).
var runCommand = func(dir, command string) (ran bool, rc int, out string) {
	cmd := exec.Command("sh", "-c", command)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "KUBECONFIG=/dev/null")
	b, err := cmd.CombinedOutput()
	if err == nil {
		return true, 0, string(b)
	}
	if ee, ok := err.(*exec.ExitError); ok {
		code := ee.ExitCode()
		if code == 126 || code == 127 {
			return false, code, string(b)
		}
		return true, code, string(b)
	}
	// The process could not be started at all — not a behaviour signal.
	return false, -1, string(b)
}

// probeRow runs command and judges it against the row's raw Expect cell (expect.go).
func probeRow(dir, command, expectCell string) (ran bool, j judgement) {
	ran, rc, out := runCommand(dir, command)
	return ran, parseExpect(expectCell).judge(ran, rc, out)
}

// pathToken matches a path-like token in a command cell: a run of path characters that
// either contains a slash or ends in a common source/artifact extension. It is intentionally
// conservative — a false path candidate that exists is simply reported PathExists, and one
// that neither exists nor has a rename hop lands in the fail-closed refused:gone.
var pathToken = regexp.MustCompile(`[A-Za-z0-9_./-]*(?:/[A-Za-z0-9_./-]+|\.(?:go|md|ya?ml|json|sh|txt|toml))\b`)

// gatherRowFacts assembles the RowFacts for one row against repoRoot. riskBearing/riskReason
// come from the owning brief's own frontmatter (brief.go), read once by the caller. The
// command behaviour probe runs ONLY when the pinned path is intact — a command whose target
// moved must be classified by the rename hop, not by the failure its missing path produces.
func gatherRowFacts(repoRoot string, row verifyRow, riskBearing bool, riskReason string) RowFacts {
	f := RowFacts{
		Row:         row.Num,
		Command:     row.Command,
		Expect:      row.Expect,
		RiskBearing: riskBearing,
		RiskReason:  riskReason,
	}

	f.PathRef, f.PathExists, f.RenameHop = resolvePinnedPath(repoRoot, row.Command)

	// mayRun is the ONE gate both execution paths below test, the rename-proof re-run
	// and the behaviour probe: a row whose raw cell statusgen refuses (a shell comment,
	// a no-op, a prose mention, a blank lift) is never executed, nor is a lifted command
	// that is itself blank, a comment or a no-op (a row built without a cell). Each exits
	// 0 having checked nothing, and either probe would read that zero as proof: a rename
	// proven safe, or a row that still passes (#2684, #3147).
	mayRun := row.Refused == "" && verifycmd.CommandRefused(row.Command) == ""

	// A rename hop is only proof once the row, rewritten to the hop, passes its own Expect
	// against the renamed file (#1953). This runs the rewritten command, even in dry-run.
	if mayRun && f.PathRef != "" && !f.PathExists && f.RenameHop != "" {
		_, f.RenameProbe = probeRow(repoRoot, renamed(row.Command, f), renamed(row.ExpectCell, f))
	}

	// NOTE: this gatherer populates the rename + behaviour-probe facts only. It does NOT set
	// CountShaped/OnlyAdditions (safe:count) or RetiredIdiom (safe:idiom): those classes are
	// classifier-ready scaffolding not yet wired to a real fact source, so the shipped verb
	// produces only safe:rename among the safe verdicts. Wiring them is a tracked follow-up;
	// docs/rebaseline.md marks safe:count/safe:idiom not-yet-implemented for that reason.

	// Behaviour probe only when there is no stale-path question to answer first.
	if mayRun && (f.PathRef == "" || f.PathExists) {
		ran, j := probeRow(repoRoot, row.Command, row.ExpectCell)
		f.CommandProbed = true
		f.CommandRan = ran
		f.RCDiffers = j == judgeFail // judged against the row's Expect, not rc != 0 (#1953)
	}
	return f
}

// resolvePinnedPath finds the deliverable path the command pins and reports whether it
// exists and, if not, the single rename hop git records for it. A command that pins several
// paths is resolved to the first MISSING one (the stale candidate the row is failing on);
// when every pinned path exists the first is reported, PathExists true.
func resolvePinnedPath(repoRoot, command string) (pathRef string, exists bool, renameHop string) {
	tokens := dedupe(pathToken.FindAllString(command, -1))
	var firstExisting string
	for _, tok := range tokens {
		tok = strings.Trim(tok, "./")
		if tok == "" || tok == "." {
			continue
		}
		if fileExists(repoRoot, tok) {
			if firstExisting == "" {
				firstExisting = tok
			}
			continue
		}
		// A missing path-like token. Only treat it as the pinned deliverable when git has
		// history for it (it was once a tracked file) — otherwise it is a glob, an option
		// value or a path that never was, not a stale deliverable. A tracked-then-gone path
		// with a single rename hop is safe:rename; one with no single hop (a plain delete, or
		// a rename chain) falls to refused:gone with renameHop "".
		if pathHadHistory(repoRoot, tok) {
			hop, _ := singleRenameHop(repoRoot, tok)
			return tok, false, hop
		}
	}
	if firstExisting != "" {
		return firstExisting, true, ""
	}
	return "", false, ""
}

// singleRenameHop reports whether git records a SINGLE rename hop from a now-missing path to
// a path that exists today. had is true only when the path has rename history at all (so the
// caller can tell "gone with a chain / no existing endpoint" — refused:gone — from "never a
// tracked file" — not a deliverable). A rename CHAIN (old→mid→new) is deliberately not a
// single hop: it is reported had=true, hop="" so it fails closed to refused:gone.
//
// The proof reads HEAD's history only, never `--all`: a rename recorded only on an unmerged
// or unrelated ref is not evidence about the tree being verified. Under --open, HEAD is the
// fetched base (openPreflight), so the proof is the base branch's own history.
func singleRenameHop(repoRoot, missing string) (hop string, had bool) {
	// -M enables rename detection; --diff-filter=R lists rename pairs as `Rnnn\told\tnew`.
	out, ok := gitOutput(repoRoot, "-C", repoRoot, "log", "HEAD", "-M", "--diff-filter=R",
		"--name-status", "--format=")
	if !ok || out == "" {
		return "", false
	}
	var hops []string
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Split(strings.TrimSpace(line), "\t")
		if len(fields) != 3 || !strings.HasPrefix(fields[0], "R") {
			continue
		}
		old, neu := fields[1], fields[2]
		if old == missing {
			hops = append(hops, neu)
		}
	}
	if len(hops) == 0 {
		return "", false
	}
	// Exactly one recorded hop, and its target exists now → a single, verifiable hop.
	uniq := dedupe(hops)
	if len(uniq) == 1 && fileExists(repoRoot, uniq[0]) {
		return uniq[0], true
	}
	// A chain, or an endpoint that no longer exists: history exists but no single safe hop.
	return "", true
}

// pathHadHistory reports whether git has any commit history touching rel — i.e. rel was
// once a tracked file. It is how a now-missing pinned deliverable (refused:gone / safe:rename)
// is told apart from a path-like token that was never tracked (a glob, an option value).
// Like singleRenameHop it reads HEAD's history only, never `--all`.
func pathHadHistory(repoRoot, rel string) bool {
	out, ok := gitOutput(repoRoot, "-C", repoRoot, "log", "HEAD", "--oneline", "-1", "--", rel)
	return ok && strings.TrimSpace(out) != ""
}

func fileExists(repoRoot, rel string) bool {
	_, err := os.Stat(filepath.Join(repoRoot, rel))
	return err == nil
}

func dedupe(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}
