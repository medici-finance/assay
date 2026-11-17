package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
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

// representedPRFiles reads one PR's COMPLETE changed-file list for the briefs-AUTHORING exemption
// (#1339). NIL by default, like representedPRs: nil = no file transport, so no PR is set aside as
// authoring and every representing PR counts as before. main() wires liveRepresentedPRFiles.
var representedPRFiles func(repo string, number int) ([]deskkit.ChangedFile, error)

// representedPRBatchFiles optionally reads one lazy candidate frontier using the shared planner reader.
var representedPRBatchFiles func(repo string, numbers []int) map[int]deskkit.PendingChangeObservation

// representedSourceFor is the FanoutLoop.Represented closure cmdPlan wires when representedPRs is
// live: it reads the repo's open+merged PRs ONCE (representedPRs) and reduces them in memory to the
// brief→RepresentedPR map — never one list call per row. A transport error propagates as
// could-not-check, which SelectQueue turns into the fresh-lane HOLD; a nil transport reduces to the
// empty map (inert).
//
// THE AUTHORING EXEMPTION (#1339). A MERGED docs-only PR that only WROTE a brief used to carry that
// brief's `Brief:` trailer, so the brief it authored read as landed-unreconciled and was never
// offered — the same collision deskdispatch's phantom check had (its authoring.go). For each brief
// that is a CANDIDATE this run (candidates: the plan's Next-up and awaiting-rework rows), every PR
// naming it is checked in list order with deskkit.BriefAuthoringOnly, and the first PR that is not
// authoring-only represents the brief; a brief whose only representing PRs are authoring PRs is not
// represented at all. File reads are therefore bounded by the PRs naming a queued brief, never the
// repo's whole merged history. A file list that cannot be read (or proven complete) keeps that PR as
// the representing one — the pre-exemption answer, a row NOT dispatched — and says so on stderr:
// could-not-check is never rounded to "authoring". nil candidates (or no file transport) applies no
// exemption and reduces first-wins exactly as before.
//
// DECLARED PARTIAL DELIVERIES (#6751). Before either reduction, every MERGED PR the brief's own
// `delivery:` list declares `covers: partial` for this repo is set aside (dropDeclaredPartials), the
// reading statusgen applies: a partial delivery is work in progress, never the brief's delivery. A
// brief whose only representing PRs were declared partials is then not represented, and its row is
// offered as fresh dispatch exactly as statusgen offers it; an OPEN PR still routes to resume, and an
// undeclared or `full` merged PR still lands. partials (FanoutLoop.declaredPartials in cmdPlan) maps
// the lower-cased brief id to those PR numbers; nil applies no set-aside.
func representedSourceFor(repo string, candidates func() (map[string]bool, error), partials func(repo string) map[string]map[int]bool) func() (map[string]deskkit.RepresentedPR, error) {
	return func() (map[string]deskkit.RepresentedPR, error) {
		if representedPRs == nil {
			return nil, nil
		}
		prs, err := representedPRs(repo)
		if err != nil {
			return nil, err
		}
		all := deskkit.RepresentingPRsByBrief(prs)
		if partials != nil {
			dropDeclaredPartials(repo, all, partials(repo))
		}
		if representedPRFiles == nil || candidates == nil {
			out := make(map[string]deskkit.RepresentedPR, len(all))
			for id, list := range all {
				out[id] = list[0]
			}
			return out, nil
		}
		cands, cerr := candidates()
		if cerr != nil {
			return nil, cerr
		}
		out := make(map[string]deskkit.RepresentedPR, len(all))
		pending := make(map[string][]deskkit.RepresentedPR)
		for id, list := range all {
			if !cands[id] {
				out[id] = list[0]
				continue
			}
			if representedPRBatchFiles == nil {
				if rp, ok := firstDeliveringPR(repo, id, list); ok {
					out[id] = rp
				}
				continue
			}
			pending[id] = list
		}
		// Only the first unclassified PR for each brief is authoritative this round.
		// Never prefetch older history: an unreadable/delivering head ends that walk.
		for len(pending) > 0 {
			ids := make([]string, 0, len(pending))
			for id := range pending {
				ids = append(ids, id)
			}
			sort.Strings(ids)
			numbers := make([]int, 0, len(ids))
			for _, id := range ids {
				numbers = append(numbers, pending[id][0].Number)
			}
			observations := representedPRBatchFiles(repo, numbers)
			for _, id := range ids {
				list := pending[id]
				rp := list[0]
				observation, ok := observations[rp.Number]
				if !ok {
					observation.Err = fmt.Errorf("complete changed-file observation absent")
				}
				if !authoringPR(repo, id, rp, observation.Files, observation.Err) {
					out[id] = rp
					delete(pending, id)
				} else if len(list) == 1 {
					delete(pending, id)
				} else {
					pending[id] = list[1:]
				}
			}
		}
		return out, nil
	}
}

// dropDeclaredPartials removes, in place, the merged PRs each brief declares a partial delivery from
// that brief's representing list, deleting a brief left with none. Each set-aside is disclosed on
// stderr, so a reader of the plan sees why a merged PR no longer lands its row.
func dropDeclaredPartials(repo string, all map[string][]deskkit.RepresentedPR, partial map[string]map[int]bool) {
	for id, nums := range partial {
		list, ok := all[id]
		if !ok {
			continue
		}
		kept := deskkit.DropDeclaredPartials(list, nums)
		for _, rp := range list {
			if rp.Merged && nums[rp.Number] {
				fmt.Fprintf(os.Stderr, "fanoutloop: NOTICE — %s#%d names %s in its `Brief:` trailer, but the brief's "+
					"delivery: list declares it a partial delivery, so it is work in progress, not the brief's "+
					"delivery (#6751)\n", repo, rp.Number, id)
			}
		}
		if len(kept) == 0 {
			delete(all, id)
		} else {
			all[id] = kept
		}
	}
}

// declaredPartials is the partials source cmdPlan hands representedSourceFor: for each of this run's
// Next-up and awaiting-rework rows, the merged PRs its brief file (read from --root, the same tree the
// board reader resolves frontmatter from) declares `covers: partial` for repo, resolved through
// --root's alias registry. A brief that cannot be read, or whose partial claims cannot be resolved,
// contributes nothing (the pre-#6751 answer: its merged PR still lands the row) and says why on
// stderr — could-not-check is never rounded to "partial".
func (f *FanoutLoop) declaredPartials(repo string) map[string]map[int]bool {
	rows, err := f.boardSource()
	if err != nil {
		return nil
	}
	rework, err := f.reworkSource()
	if err != nil {
		return nil
	}
	out := map[string]map[int]bool{}
	var aliases *deskkit.DeliveryAliases
	for _, r := range append(rows, rework...) {
		if strings.TrimSpace(r.BriefPath) == "" {
			continue
		}
		id := strings.ToLower(r.Stream + "/" + r.Num)
		if _, done := out[id]; done {
			continue
		}
		raw, rerr := os.ReadFile(filepath.Join(f.Root, filepath.FromSlash(r.BriefPath)))
		if rerr != nil {
			continue // no brief to read: nothing declared, the pre-#6751 answer
		}
		if aliases == nil {
			a := deskkit.LoadDeliveryAliases(f.Root)
			aliases = &a
		}
		nums, why := deskkit.DeclaredPartialDeliveries(string(raw), repo, *aliases)
		if why != "" {
			fmt.Fprintf(os.Stderr, "fanoutloop: NOTICE — %s's delivery: list could not be read with certainty (%s); "+
				"no merged PR is set aside as a partial delivery for it (could-not-check, not a pass)\n", id, why)
			continue
		}
		if len(nums) > 0 {
			out[id] = nums
		}
	}
	return out
}

// firstDeliveringPR returns the first PR in list that is not an authoring-only PR for briefID, and
// false when every PR in list only authored it. An unreadable file list ends the walk on that PR,
// which then represents the brief (the conservative answer, reported on stderr).
func firstDeliveringPR(repo, briefID string, list []deskkit.RepresentedPR) (deskkit.RepresentedPR, bool) {
	for _, rp := range list {
		files, err := representedPRFiles(repo, rp.Number)
		if authoringPR(repo, briefID, rp, files, err) {
			continue
		}
		return rp, true
	}
	return deskkit.RepresentedPR{}, false
}

// authoringPR is the shared classification for serial and batched observations.
func authoringPR(repo, briefID string, rp deskkit.RepresentedPR, files []deskkit.ChangedFile, err error) bool {
	if err != nil {
		fmt.Fprintf(os.Stderr, "fanoutloop: NOTICE — could not read %s#%d's changed files to tell whether it "+
			"DELIVERED %s or only AUTHORED it (%v); counting it as the brief's representing PR, the answer "+
			"before the authoring exemption (could-not-check, not a pass)\n", repo, rp.Number, briefID, err)
		return false
	}
	if deskkit.BriefAuthoringOnly(briefID, files) {
		fmt.Fprintf(os.Stderr, "fanoutloop: NOTICE — %s#%d names %s in its `Brief:` trailer but only "+
			"AUTHORED it (it adds the brief's file and touches only stream board READMEs, brief files and "+
			"changelog fragments), so it does not represent the brief\n", repo, rp.Number, briefID)
		return true
	}
	return false
}

// candidateBriefIDs is the candidate set cmdPlan hands representedSourceFor: the lower-cased brief
// ids of this run's Next-up and awaiting-rework rows — the only rows the represented map is consulted
// for, so the only briefs whose representing PRs are worth a file read.
func (f *FanoutLoop) candidateBriefIDs() (map[string]bool, error) {
	out := map[string]bool{}
	rows, err := f.boardSource()
	if err != nil {
		return nil, err
	}
	rework, err := f.reworkSource()
	if err != nil {
		return nil, err
	}
	for _, r := range append(rows, rework...) {
		out[strings.ToLower(r.Stream+"/"+r.Num)] = true
	}
	return out, nil
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
