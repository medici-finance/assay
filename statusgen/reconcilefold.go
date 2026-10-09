package main

// reconcilefold.go — the witness wiring for DeriveLifecycle's four fold inputs
// (derived-board/03 rework, #1787).
//
// lifecycle.go's fold declares Witnesses, Approvals, Rulings and IssueLabels,
// but a caller that builds LifecycleInput{Briefs: idents} alone can never
// derive above `implemented` — which is exactly what both production callers
// (reconcile, regen's drift comparator) did, so no live board cell ever
// derived verified/done/blocked and the drift comparator reported every done
// brief as drift. This file populates the four maps from real reads, shared by
// both callers so the two agree by construction:
//
//   - Witnesses: the brief's own Evidence table, audited by checkWitnesses —
//     the exact `verifyrun --check` code path spec §2 names as the `verified`
//     witness. A clean pass (every row witnessed, matched, passed) yields
//     Passed; any failed row yields Passed=false (the red witness: the cell
//     falls back to its PR base, never a promotion). A brief with no failed
//     row but unwitnessed rows (could-not-run) contributes NO entry — the
//     three-state invariant: could-not-check never rounds to pass or fail.
//     Released is set true explicitly: this fold stands by the row-level
//     witness alone (the evidence-coverage demotion is coverage.go's read,
//     not the board's), per the false-by-default contract on deriveOne.
//   - Rulings: a gate:human brief's `done` ruling is the README Reviewed
//     cell's `human:<login>` stamp — the same stamp closeVerify writes when a
//     human closes the verify-gate issue (verifyissues.go).
//   - Approvals: ReviewsAtHead on the latest merged PR of every brief whose
//     witness passed and whose gate is model — the only briefs an approval can
//     promote, so the per-brief reviews read stays bounded to the promotable
//     set. A failed read contributes NO entry (the brief derives no higher
//     than verified) and is disclosed to the caller as a could-not-check
//     notice — fail closed, never a silent done.
//   - IssueLabels: one paged open-issues read per run (never per-brief),
//     mapped onto each brief's `issues:` list. PR entries in the issues API
//     (same namespace) are filtered out. A failed read leaves the blocked
//     overlay off for the whole run and is disclosed the same way.
//
// All four populate only in the ONLINE arm: --offline keeps every map nil and
// every cell unknown, so a could-not-check run never renders a drift.

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
)

// reconcileFoldData is one brief's fold-relevant record, read once from the
// tree and shared by the tree and forge gatherers below.
type reconcileFoldData struct {
	ident    BriefIdent
	verify   string // the brief's ## Verify section
	evidence string // the brief's ## Evidence section
	issues   []int  // the frontmatter issues: list
	reviewed string // the README row's Reviewed cell
}

// loadReconcileFoldData enumerates every brief under boardRoot exactly as
// reconcileBriefIdents does — same streams, same files, same dedupe — and
// keeps the four fold inputs' raw material alongside each ident. boardRoot is
// the findBoardRoot-resolved root.
func loadReconcileFoldData(boardRoot string) ([]reconcileFoldData, error) {
	streams, _, err := loadStreams(boardRoot)
	if err != nil {
		return nil, err
	}
	var out []reconcileFoldData
	seen := map[string]bool{}
	for _, s := range streams {
		for _, path := range briefFilePaths(s) {
			bf, ok, perr := parseBriefFile(path)
			var d reconcileFoldData
			d.ident.Version = 1
			if perr == nil && ok {
				d.ident.ID = bf.Brief
				d.ident.Gate = bf.Gate
				if bf.Version > 0 {
					d.ident.Version = bf.Version
				}
				d.verify = bf.Verify
				d.evidence = bf.Evidence
				d.issues = bf.Issues
			} else {
				derived, _, okName := expectedBriefID(path)
				if !okName {
					continue // not a brief file shape we can key on
				}
				d.ident.ID = derived
			}
			if d.ident.ID == "" || seen[d.ident.ID] {
				continue
			}
			seen[d.ident.ID] = true
			if _, num, okName := expectedBriefID(path); okName {
				if row := findRow(s, num); row != nil {
					d.reviewed = row.Reviewed
				}
			}
			out = append(out, d)
		}
	}
	return out, nil
}

// treeFoldInputs derives the two inputs that need no forge: the verify
// witnesses (from each brief's own Evidence audit) and the gate:human rulings
// (from the README Reviewed-cell human: stamp).
func treeFoldInputs(data []reconcileFoldData) (witnesses map[string]WitnessInfo, rulings map[string]bool) {
	witnesses = map[string]WitnessInfo{}
	rulings = map[string]bool{}
	for _, d := range data {
		if w, ok := foldWitness(d); ok {
			witnesses[d.ident.ID] = w
		}
		if d.ident.Gate == "human" && strings.Contains(d.reviewed, "human:") {
			rulings[d.ident.ID] = true
		}
	}
	return witnesses, rulings
}

// foldWitness audits one brief's Evidence against its Verify table via
// checkWitnesses — the verifyrun --check code path, so the board's `verified`
// is the same verdict a verifier would compute. ok is false when the audit
// cannot assert either direction (no Verify rows, or rows only could-not-run):
// an instrument that could not run contributes no witness rather than a guess.
func foldWitness(d reconcileFoldData) (WitnessInfo, bool) {
	if len(briefVerifyRows(d.verify)) == 0 {
		return WitnessInfo{}, false
	}
	// closureWitnesses, never checkWitnesses directly (the htmlcomment.go class
	// guard): an unterminated `<!--` opener refuses the audit, and a refused
	// audit contributes no witness — the could-not-check arm.
	findings, refusal := closureWitnesses(d.verify, d.evidence)
	if refusal != "" {
		return WitnessInfo{}, false
	}
	switch {
	case checkExitCode(findings) == verifyrunExitPass:
		return WitnessInfo{Passed: true, Version: d.ident.Version, Released: true}, true
	case len(failedWitnessRows(findings)) > 0:
		return WitnessInfo{Passed: false, Version: d.ident.Version}, true
	}
	return WitnessInfo{}, false
}

// foldIssueLabels maps one paged open-issues read onto each brief's declared
// issues: list. The issues API returns PRs in the same namespace; entries
// carrying a pull_request key are not issues and are dropped. A brief whose
// issue is closed (or never existed) contributes no labels — a closed blocker
// no longer blocks.
func foldIssueLabels(data []reconcileFoldData, byNumber map[int][]string) map[string][]string {
	out := map[string][]string{}
	for _, d := range data {
		if len(d.issues) == 0 {
			continue
		}
		var labels []string
		for _, n := range d.issues {
			labels = append(labels, byNumber[n]...)
		}
		if len(labels) > 0 {
			out[d.ident.ID] = labels
		}
	}
	return out
}

// ghIssueListEntry is the issues-list entry shape the fold reads. Labels reuse
// scanissues.go's ghLabel; PullRequest is present (non-null) on PR entries,
// which share the issues namespace but are not issues.
type ghIssueListEntry struct {
	Number      int             `json:"number"`
	Labels      []ghLabel       `json:"labels"`
	PullRequest json.RawMessage `json:"pull_request"`
}

// listOpenIssueLabels reads the repo's open issues, paged like fetchAllPulls,
// and returns issue number → label names. lookedAt=false with a reason on any
// fetch or decode failure — the caller then leaves the blocked overlay off and
// discloses the could-not-check.
func (c *ghClient) listOpenIssueLabels(repo string) (byNumber map[int][]string, lookedAt bool, reason string) {
	const perPage = 100
	const maxPages = 20
	byNumber = map[int][]string{}
	for page := 1; page <= maxPages; page++ {
		url := fmt.Sprintf("%s/repos/%s/issues?state=open&per_page=%d&page=%d", c.base, repo, perPage, page)
		body, status, err := c.get(url)
		if err != nil {
			return nil, false, err.Error()
		}
		if status != 200 {
			return nil, false, fmt.Sprintf("HTTP %d listing open issues (page %d)", status, page)
		}
		var batch []ghIssueListEntry
		if err := json.Unmarshal(body, &batch); err != nil {
			return nil, false, fmt.Sprintf("decoding open issues (page %d): %v", page, err)
		}
		for _, is := range batch {
			if len(is.PullRequest) > 0 && string(is.PullRequest) != "null" {
				continue // a PR, not an issue
			}
			if names := labelNames(is.Labels); len(names) > 0 {
				byNumber[is.Number] = names
			}
		}
		if len(batch) < perPage {
			return byNumber, true, ""
		}
	}
	return byNumber, true, ""
}

// foldApprovals reads the App review state at the merged head for every brief
// an approval could still promote: witness passed, gate model, a merged PR on
// record. misses lists the brief ids whose reviews read failed (fail closed:
// they derive no higher than verified this run, and the caller discloses the
// could-not-check).
func (c *ghClient) foldApprovals(repo string, data []reconcileFoldData, prs []PRRecord, witnesses map[string]WitnessInfo) (map[string]ApprovalInfo, []string) {
	prsByBrief := map[string][]PRRecord{}
	for _, pr := range prs {
		key := canonicalBriefKey(pr.BriefRef)
		prsByBrief[key] = append(prsByBrief[key], pr)
	}
	out := map[string]ApprovalInfo{}
	var misses []string
	for _, d := range data {
		w, ok := witnesses[d.ident.ID]
		if !ok || !w.Passed || d.ident.Gate != "model" {
			continue
		}
		latestMerged, _ := classifyPRs(prsByBrief[canonicalBriefKey(d.ident.ID)])
		if latestMerged == nil {
			continue
		}
		approved, atHead, lookedAt, _ := c.ReviewsAtHead(repo, latestMerged.Number, latestMerged.HeadSHA)
		if !lookedAt {
			misses = append(misses, d.ident.ID)
			continue
		}
		out[d.ident.ID] = ApprovalInfo{Approved: approved, AtHead: atHead}
	}
	sort.Strings(misses)
	return out, misses
}

// wireFoldInputs populates in's four witness-source maps for one online run:
// the tree inputs always; the forge inputs through client, with every failed
// read disclosed on stderr as a could-not-check NOTICE and its overlay left
// off (fail closed). It is the single wiring point reconcile and regen share,
// so the verb and the drift comparator derive from the same witness set.
func wireFoldInputs(in *LifecycleInput, boardRoot string, client *ghClient, repo string, stderr *os.File) {
	data, err := loadReconcileFoldData(boardRoot)
	if err != nil {
		fmt.Fprintf(stderr, "reconcile: could-not-check the fold reads: %v — witness, ruling, approval and label overlays are off this run\n", err)
		return
	}
	in.Witnesses, in.Rulings = treeFoldInputs(data)

	labels, labelsOK, labelsReason := client.listOpenIssueLabels(repo)
	if !labelsOK {
		fmt.Fprintf(stderr, "reconcile: could-not-check the open-issues read: %s — the blocked overlay is not applied this run\n", labelsReason)
	} else {
		in.IssueLabels = foldIssueLabels(data, labels)
	}

	approvals, misses := client.foldApprovals(repo, data, in.PRs, in.Witnesses)
	in.Approvals = approvals
	if len(misses) > 0 {
		fmt.Fprintf(stderr, "reconcile: could-not-check the reviews read for %d brief(s) (%s) — they derive no higher than verified this run\n",
			len(misses), strings.Join(misses, ", "))
	}
}
