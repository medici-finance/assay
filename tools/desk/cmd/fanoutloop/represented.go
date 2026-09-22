package main

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// represented.go — the already-represented reconciliation `fanoutloop plan` applies to fresh rows
// (#1339).
//
// THE DEFECT IT CLOSES. `plan` read the board from `refs/remotes/origin/main` and offered every
// `todo`/`in-progress` row, but a row's board cell is UNRELIABLE as an eligibility signal: a brief
// whose deliverable already merged reads `todo` until a separate `statusgen reconcile` flips it, and
// no scheduled job writes that cell (#1175). So `plan` kept offering already-landed rows as fresh
// dispatch, burning a worker per tick on work that returns nothing (the field evidence in #1339). The
// witness that does NOT drift like a board cell is the PR body's `Brief:` trailer; this file moves it
// FORWARD into `plan`, where SelectQueue routes a fresh row by its representing PR's state (merged →
// landed-unreconciled, never dispatched; open → resume; none → fresh).
//
// TRANSPORT — DEFERRED, exactly like deskdispatch's listRepresentedPRs and fanoutloop's Orphans.
// representedPRs is a package var, NIL by default. Nil = the transport is NOT wired: the shipped
// offline build performs no forge read and offers rows exactly as before. The closed forge surface
// (internal/forgeban; the CI-gating TestNoForgeCLIShellout) ships NO forge-CLI call, and the typed
// Forge seam has no open+merged-changes list op yet — so wiring the LIVE read means adding that typed
// op (never a raw `gh`) and is the cutover work the autonomous drive is gated behind. Until then this
// file supplies the whole CONSUMER: the classification, the repo resolution, and the one-read-per-run
// reduction — ready to activate the instant a typed transport is assigned to representedPRs. Tests
// assign representedPRs (or inject FanoutLoop.Represented directly) to drive it against a recorded PR
// list with no live forge.

// representedPRs is the PR-list transport `plan` reconciles through — ONE list read per plan run when
// wired. NIL in the shipped build (no forge-CLI is shipped and the typed op is deferred to the
// cutover); when nil, cmdPlan leaves FanoutLoop.Represented unset and every row is offered as before.
var representedPRs func(repo string) ([]deskkit.PRRef, error)

// representedSourceFor is the FanoutLoop.Represented closure cmdPlan wires when representedPRs is
// live: it reads the repo's open+merged PRs ONCE (representedPRs) and reduces them in memory to the
// brief→RepresentedPR map — never one call per row. A transport error propagates as could-not-check,
// which SelectQueue turns into the fresh-lane HOLD; a nil transport reduces to the empty map (inert).
func representedSourceFor(repo string) func() (map[string]deskkit.RepresentedPR, error) {
	return func() (map[string]deskkit.RepresentedPR, error) {
		if representedPRs == nil {
			return nil, nil
		}
		prs, err := representedPRs(repo)
		if err != nil {
			return nil, err
		}
		return deskkit.RepresentedBriefPRs(prs), nil
	}
}

// resolveRepoForPlan resolves the owner/name `plan` reconciles against: an explicit --repo wins; else
// the repo the configured roots map --root to; else the checkout's own `origin` remote. When none
// resolves it returns an error — a could-not-resolve the caller wires as a fresh-lane HOLD, never a
// silent skip that would let phantoms through.
func resolveRepoForPlan(root, repoFlag string) (string, error) {
	if r := strings.TrimSpace(repoFlag); r != "" {
		return r, nil
	}
	if repo := repoFromConfiguredRoots(root); repo != "" {
		return repo, nil
	}
	if repo := deskkit.RepoSlugForDir(root); repo != "" {
		return repo, nil
	}
	return "", fmt.Errorf("could not resolve the repo for --root %q (not a configured DESK_ROOTS root, and no origin remote resolved) — pass --repo <owner>/<name>", root)
}

// repoFromConfiguredRoots matches an absolute --root path against the DESK_ROOTS map and returns the
// repo configured for it, or "" when the root is not one of the configured checkouts (or the roots
// are unreadable). Path comparison is on cleaned absolute paths so a trailing slash or a relative
// spelling does not defeat the match.
func repoFromConfiguredRoots(root string) string {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return ""
	}
	roots, err := deskkit.ConfiguredRoots()
	if err != nil {
		return ""
	}
	for _, r := range roots {
		if absR, aerr := filepath.Abs(r.Path); aerr == nil && filepath.Clean(absR) == filepath.Clean(absRoot) {
			return r.Repo
		}
	}
	return ""
}
