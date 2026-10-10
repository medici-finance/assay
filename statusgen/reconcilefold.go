package main

// reconcilefold.go — the witness wiring for DeriveLifecycle's fold inputs
// (derived-board/03 rework, #1787).
//
// lifecycle.go's fold declares Witnesses, Approvals, Rulings and IssueLabels,
// but a caller that builds LifecycleInput{Briefs: idents} alone can never
// derive above `implemented` — which is exactly what both production callers
// (reconcile, regen's drift comparator) did, so no live board cell ever
// derived verified/done/blocked and the drift comparator reported every done
// brief as drift. This file populates those maps from real reads, shared by
// both callers so the two agree by construction.
//
// The fold derives a state only by the rules the state's own writer obeys.
// It owns no rule of its own:
//
//   - Witnesses (`verified`): the latest strict **VERIFY: PASS** run, read by
//     verifyflip.go's flipLatestPass; the HELD and FAIL refusals closeVerify
//     applies; the closureWitnesses audit (`verifyrun --check`); flipProvenance
//     (the PASS lines committed by the roster's verifier, not the implementer);
//     the brief version read from the brief AS IT STOOD at the run's sha; and
//     the evidence-coverage verdict (coverage.go). A refusal from any of them
//     contributes no witness. A read that could not be made (no roster, a
//     shallow clone, git failing, the version unreadable at the run's sha)
//     marks the brief Undecided: its cell is `unknown`, never `verified` and
//     never a definite `implemented`.
//   - Approvals (`done`, gate:model): autoflip.go's decideModelFlip — the
//     delivering PR by trailer and history, the roster-bound reviewer App, its
//     approval at that PR's merged head. Asked only for a brief that would
//     otherwise derive `verified`. A refusal leaves it `verified`; a
//     could-not-check marks it Undecided.
//   - Rulings (`done`, gate:human): the README Reviewed cell's `human:<name>`
//     stamp — the cell closeVerify writes and hasHumanReviewer reads — parsed
//     by humanStampRe (the anchored reader the human-stamp guard uses) with
//     every on-behalf-of relay removed first. Relay text and a bare `human:`
//     never count. The derivation names the stamp it read.
//   - IssueLabels (`blocked`): one paged open-issues read per run, mapped onto
//     each brief's `issues:` list. PR entries are dropped. A failed or
//     truncated read (the page cap) marks every brief with an `issues:` list
//     LabelsUnread: its todo/in-progress cell is `unknown`.
//
// wireFoldInputs returns every could-not-check as a disclosure; reconcile puts
// them in its JSON (`unread`) and refuses `--apply` while any is present.
//
// All of it populates only in the ONLINE arm: --offline keeps every map nil
// and every cell unknown, so a could-not-check run never renders a drift.

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"sort"
)

// reconcileFoldData is one brief's fold-relevant record, read once from the
// tree and shared by the tree and forge gatherers below.
type reconcileFoldData struct {
	ident    BriefIdent
	path     string  // the brief file
	stream   *Stream // the stream it belongs to
	key      string  // the canonical "<stream>/<NN>" key (coverage, closeVerify)
	parsed   bool    // the brief file parsed
	verify   string  // the brief's ## Verify section
	evidence string  // the brief's ## Evidence section
	issues   []int   // the frontmatter issues: list
	reviewed string  // the README row's Reviewed cell
}

// loadReconcileFoldData enumerates every brief under boardRoot exactly as
// reconcileBriefIdents does — same streams, same files, same dedupe — and
// keeps the fold inputs' raw material alongside each ident. boardRoot is the
// findBoardRoot-resolved root.
func loadReconcileFoldData(boardRoot string) ([]reconcileFoldData, []*Stream, error) {
	streams, _, err := loadStreams(boardRoot)
	if err != nil {
		return nil, nil, err
	}
	var out []reconcileFoldData
	seen := map[string]bool{}
	for _, s := range streams {
		for _, path := range briefFilePaths(s) {
			bf, ok, perr := parseBriefFile(path)
			d := reconcileFoldData{path: path, stream: s}
			d.ident.Version = 1
			if perr == nil && ok {
				d.parsed = true
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
				d.key = s.Name + "/" + num
				if row := findRow(s, num); row != nil {
					d.reviewed = row.Reviewed
				}
			}
			out = append(out, d)
		}
	}
	return out, streams, nil
}

// foldTree is the tree-only half of the fold: the verified witnesses, the
// gate:human sign-offs, and the briefs whose verified decision could not be
// made (with the reason).
type foldTree struct {
	witnesses map[string]WitnessInfo
	rulings   map[string]string
	undecided map[string]string
}

// treeFoldInputs derives the inputs that need no forge. Coverage is computed
// once for the whole run, as autoFlipModel does.
func treeFoldInputs(root string, streams []*Stream, data []reconcileFoldData) foldTree {
	t := foldTree{witnesses: map[string]WitnessInfo{}, rulings: map[string]string{}, undecided: map[string]string{}}
	coverage := evaluateCoverage(root, streams, coverageOptions{})
	for _, d := range data {
		w, ok, why := foldVerified(d, coverage)
		switch {
		case why != "":
			t.undecided[d.ident.ID] = why
		case ok:
			t.witnesses[d.ident.ID] = w
		}
		if d.ident.Gate == "human" {
			if stamp := humanSignoff(d.reviewed); stamp != "" {
				t.rulings[d.ident.ID] = stamp
			}
		}
	}
	return t
}

// foldVerified reads one brief's `verified` witness by the rules verifyflip
// and closeVerify apply before writing `verified`. ok reports a passing
// witness; why, when non-empty, is a could-not-check (the brief is Undecided).
// Gate, risk and row class are not checked: they decide who may WRITE the
// cell, not what the record shows.
func foldVerified(d reconcileFoldData, coverage map[string]Coverage) (w WitnessInfo, ok bool, why string) {
	if !d.parsed || len(briefVerifyRows(d.verify)) == 0 {
		return WitnessInfo{}, false, ""
	}
	p, err := flipLatestPass(d.evidence)
	if err != nil {
		var r *flipRefusal
		if errors.As(err, &r) {
			return WitnessInfo{}, false, ""
		}
		return WitnessInfo{}, false, "could-not-check the Evidence read: " + err.Error()
	}
	if closeVerifyHeldRefusal(d.ident.ID, "implemented", d.evidence) != nil ||
		closeVerifyFailRefusal(d.ident.ID, "implemented", d.evidence) != nil {
		return WitnessInfo{}, false, ""
	}
	// closureWitnesses, never checkWitnesses directly (the htmlcomment.go class
	// guard): an unterminated `<!--` opener refuses the audit.
	findings, refusal := closureWitnesses(d.verify, p.Run)
	if refusal != "" || len(findings) == 0 || checkExitCode(findings) != verifyrunExitPass {
		return WitnessInfo{}, false, ""
	}
	if err := flipProvenance(d.path, d.evidence, p.Marks, p.RunStart); err != nil {
		var r *flipRefusal
		if errors.As(err, &r) {
			return WitnessInfo{}, false, ""
		}
		return WitnessInfo{}, false, "could-not-check provenance: " + err.Error()
	}
	version, verr := briefVersionAt(d.path, p.SHA)
	if verr != nil {
		return WitnessInfo{}, false, fmt.Sprintf("could-not-check the brief version at the run's sha %s: %v", p.SHA, verr)
	}
	return WitnessInfo{
		Passed:   true,
		Version:  version,
		Released: coverage[d.key].Released,
		Run:      p.Runner + " @ " + p.SHA,
	}, true, ""
}

// briefVersionAt reads the brief's `version:` as the file stood at sha — the
// version the recorded run was made against, never the current one copied.
func briefVersionAt(path, sha string) (int, error) {
	out, err := coverageGit(filepath.Dir(path), "show", "--end-of-options", sha+":./"+filepath.Base(path)).Output()
	if err != nil {
		return 0, fmt.Errorf("git show: %w", err)
	}
	bf, ok, perr := parseBriefFileBytes(path, out)
	if perr != nil {
		return 0, perr
	}
	if !ok {
		return 0, errors.New("not a brief file at that sha")
	}
	if bf.Version > 0 {
		return bf.Version, nil
	}
	return 1, nil
}

// humanSignoff returns the README Reviewed cell's human stamp ("human:<name>")
// or "". On-behalf-of relays are removed first: a relay records whom an App
// acted for and is never that human's sign-off.
func humanSignoff(reviewed string) string {
	m := humanStampRe.FindStringSubmatch(withoutOnBehalfOfRelays(reviewed))
	if m == nil {
		return ""
	}
	return "human:" + m[1]
}

// reconcileFlipSource is the forge read decideModelFlip uses for the fold;
// tests replace it.
var reconcileFlipSource = liveModelFlipSource

// foldApprovals asks decideModelFlip about every gate:model brief that would
// otherwise derive `verified`: a merged PR read, a passing released witness at
// the brief's version, not Undecided. flipDone yields an approval naming the
// reviewer, PR and head; flipRefused yields none (the brief stays verified);
// flipUnchecked marks the brief Undecided.
func foldApprovals(root string, in *LifecycleInput, data []reconcileFoldData) map[string]ApprovalInfo {
	out := map[string]ApprovalInfo{}
	if !in.LookedAt {
		return out
	}
	prsByBrief := map[string][]PRRecord{}
	for _, pr := range in.PRs {
		key := canonicalBriefKey(pr.BriefRef)
		prsByBrief[key] = append(prsByBrief[key], pr)
	}
	rev := modelReviewer()
	src := reconcileFlipSource(rev.Forge)
	for _, d := range data {
		id := d.ident.ID
		if d.ident.Gate != "model" {
			continue
		}
		if _, undecided := in.Undecided[id]; undecided {
			continue
		}
		w, ok := in.Witnesses[id]
		if !ok || !w.Passed || !w.Released || w.Version != d.ident.Version {
			continue
		}
		if merged, _ := classifyPRs(prsByBrief[canonicalBriefKey(id)]); merged == nil {
			continue
		}
		res := decideModelFlip(root, d.stream, d.path, id, d.evidence, src, rev)
		switch res.Outcome {
		case flipDone:
			out[id] = ApprovalInfo{Approved: true, AtHead: true,
				Witness: fmt.Sprintf("App approval by %s on PR #%d @ %s", rev.Display, res.PR, shortSHA(res.SHA))}
		case flipUnchecked:
			in.Undecided[id] = "could-not-check the App approval: " + res.Reason
		}
	}
	return out
}

// foldIssueLabels maps one paged open-issues read onto each brief's declared
// issues: list. A brief whose issue is closed (or never existed) contributes
// no labels — a closed blocker no longer blocks.
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

// openIssuePages caps the open-issues read. Reaching it is a truncated read:
// lookedAt=false, never a partial label set read as complete.
const openIssuePages = 20

// listOpenIssueLabels reads the repo's open issues, paged like fetchAllPulls,
// and returns issue number → label names. lookedAt=false with a reason on any
// fetch or decode failure, and when the page cap is reached with a full last
// page (the rest was not looked at).
func (c *ghClient) listOpenIssueLabels(repo string) (byNumber map[int][]string, lookedAt bool, reason string) {
	return c.listOpenIssueLabelsN(repo, openIssuePages)
}

func (c *ghClient) listOpenIssueLabelsN(repo string, maxPages int) (byNumber map[int][]string, lookedAt bool, reason string) {
	const perPage = 100
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
	return nil, false, fmt.Sprintf("open-issues read hit the %d-page cap — later pages were not looked at", maxPages)
}

// wireFoldInputs populates in's fold maps for one online run and returns every
// could-not-check as a disclosure (each also printed to stderr). A brief whose
// read failed is Undecided or LabelsUnread — `unknown` with the reason — never
// the lower state the missing read would leave. It is the single wiring point
// reconcile and regen share, so the verb and the drift comparator derive from
// the same witness set.
func wireFoldInputs(in *LifecycleInput, boardRoot string, client *ghClient, repo string, stderr io.Writer) []string {
	var disclosures []string
	disclose := func(format string, a ...any) {
		msg := fmt.Sprintf(format, a...)
		disclosures = append(disclosures, msg)
		fmt.Fprintf(stderr, "reconcile: %s\n", msg)
	}
	in.Undecided = map[string]string{}
	in.LabelsUnread = map[string]string{}

	data, streams, err := loadReconcileFoldData(boardRoot)
	if err != nil {
		why := fmt.Sprintf("could-not-check the fold reads: %v", err)
		for _, b := range in.Briefs {
			in.Undecided[b.ID] = why
			in.LabelsUnread[b.ID] = why
		}
		disclose("%s — every brief's verified/done/blocked state is unknown this run", why)
		return disclosures
	}
	tree := treeFoldInputs(boardRoot, streams, data)
	in.Witnesses, in.Rulings = tree.witnesses, tree.rulings
	for id, why := range tree.undecided {
		in.Undecided[id] = why
	}

	labels, labelsOK, labelsReason := client.listOpenIssueLabels(repo)
	if labelsOK {
		in.IssueLabels = foldIssueLabels(data, labels)
	} else {
		why := "could-not-check the open-issues read: " + labelsReason
		n := 0
		for _, d := range data {
			if len(d.issues) > 0 {
				in.LabelsUnread[d.ident.ID] = why
				n++
			}
		}
		if n > 0 {
			disclose("%s — %d brief(s) with linked issues are unknown, not unblocked", why, n)
		} else {
			// No brief links an issue, so the failed read decides no cell: it is
			// noted, but it is not an unread input and does not hold --apply.
			fmt.Fprintf(stderr, "reconcile: %s — no brief links an issue, so no cell depends on it\n", why)
		}
	}

	in.Approvals = foldApprovals(boardRoot, in, data)

	if len(in.Undecided) > 0 {
		ids := make([]string, 0, len(in.Undecided))
		for id := range in.Undecided {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		for _, id := range ids {
			disclose("%s: %s", id, in.Undecided[id])
		}
		disclose("%d brief(s) could not be decided verified/done this run and derive unknown", len(ids))
	}
	return disclosures
}
