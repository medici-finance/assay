package main

// decisiongateanchor.go — the THIRD human-stamp corroboration anchor.
//
// The stamp gate (corroborate.go) accepts a gate:human `human:<name>` token when
// the named human left an APPROVED review or an explicit approval COMMENT on the
// brief's own PR. But the house's sanctioned ratification channel for a gate:human
// DECISION is not a PR approval at all: the human ratifies and CLOSES the linked
// `needs-decision` issue, and that closed issue IS the record. A decision brief
// ratified correctly on that channel therefore could not green its own corroborate
// lint — the mismatch the tracker ruling #2237 resolves (Option 1: teach the lint
// to accept the decision-issue channel).
//
// This file adds that third accepted anchor ALONGSIDE the two PR anchors, never
// replacing them. It fires ONLY when all three of the ruling's conditions hold:
//
//	(a) the linked issue is CLOSED by the blessed human (ASSAY_BLESS_LOGIN) — the
//	    single highest-authority login in the trust roster (rosterconfig.go), and
//	    the only close that ratifies;
//	(b) the issue carries the per-brief marker "<!-- decision-gate: <stream>/<NN> -->"
//	    naming THIS EXACT brief — so a decision issue filed for one brief cannot
//	    ratify another;
//	(c) the brief LINKS the issue.
//
// The split mirrors citationcorroborate.go: decisionGateCorroboration is the pure,
// offline-testable core (it consumes pre-fetched issue state); gatherDecisionGateLinks
// / fetchDecisionGateIssue are the untested live-fetch plumbing, kept out of the
// offline --lint envelope for the same reason the citation lane is (a cited issue is
// a LIVE, possibly cross-repo read that belongs on the network-capable --corroborate
// verb only).

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// decisionGateMarker renders the per-brief decision-gate marker a needs-decision
// issue must carry to corroborate a gate:human brief through this third anchor.
// briefID is the canonical "<stream>/<NN>" form (expectedBriefID). The marker names
// the EXACT brief so a decision issue filed for one brief cannot ratify another —
// condition (b) of the ruling.
//
// It is DELIBERATELY DISTINCT from decisionMarker ("<!-- needs-decision: … -->",
// decisionissues.go), which is the emitter's idempotency key. This marker is the
// corroboration anchor a human (or the decision-issue template) places to say "this
// closed issue ratifies THIS brief"; keeping the two strings separate means the
// idempotency key alone never satisfies the corroboration gate.
func decisionGateMarker(briefID string) string {
	return "<!-- decision-gate: " + briefID + " -->"
}

// decisionGateIssue is the pre-fetched state of ONE needs-decision issue a brief
// links, consumed by the third corroboration anchor. The plumbing fetches it so
// decisionGateCorroboration stays pure and offline-testable.
type decisionGateIssue struct {
	Ref      string // "owner/repo#N", for the evidence line
	ClosedBy string // login of the actor who CLOSED the issue ("" if open/not-closed)
	Body     string // issue body, scanned for the per-brief decision-gate marker
}

// decisionGateLinks maps a brief file path (as it appears in stamp.File) to the
// needs-decision issues that brief LINKS. Only issues the brief actually links are
// present, so a brief appearing here at all encodes condition (c); membership is
// established by the plumbing (gatherDecisionGateLinks) reading the brief file on
// disk. It is empty/nil whenever the third anchor is not in play — every existing
// caller and test — so the two PR anchors decide exactly as before.
type decisionGateLinks map[string][]decisionGateIssue

// decisionGateCorroboration is the pure core of the third corroboration anchor
// (tracker ruling #2237). It reports CORROBORATED for a human:<name> stamp found in
// a brief file when a needs-decision issue that brief LINKS satisfies ALL THREE
// conditions (a)/(b)/(c) above.
//
// It fails CLOSED in every ambiguous direction: a stamp on a non-brief file, an
// empty links map, an unset bless login, an open (or otherwise non-blessed-closed)
// issue, or a marker naming a different brief all yield ok == false, leaving the two
// PR anchors to decide. None of the three conditions is sufficient alone.
func decisionGateCorroboration(s stamp, gates decisionGateLinks) (evidence string, ok bool) {
	briefID, _, okID := expectedBriefID(s.File)
	if !okID {
		return "", false // stamp is not in a brief-<NN>.md file — this anchor is N/A
	}
	linked := gates[s.File] // (c) only issues THIS brief links are present here
	if len(linked) == 0 {
		return "", false
	}
	blessLogin := scanEffectiveConfig().Bless.Login
	if blessLogin == "" {
		return "", false // no blessed closer configured — the strict-but-inert direction
	}
	want := decisionGateMarker(briefID)
	for _, iss := range linked {
		if !strings.EqualFold(iss.ClosedBy, blessLogin) { // (a) closed by the blessed human
			continue
		}
		if !strings.Contains(iss.Body, want) { // (b) marker names THIS exact brief
			continue
		}
		return fmt.Sprintf("needs-decision issue %s closed by the blessed human %s and carrying the "+
			"per-brief marker %q (tracker ruling #2237)", iss.Ref, blessLogin, want), true
	}
	return "", false
}

// ---- live-fetch plumbing (untested, like fetchPRData / fetchCitedArtifact) --------

// decisionGateRef is one issue reference a brief links.
type decisionGateRef struct {
	Repo   string // "owner/repo"
	Number int
}

func (r decisionGateRef) key() string { return fmt.Sprintf("%s#%d", r.Repo, r.Number) }

// briefLinkedIssueRefs reads the brief file at root/briefFile and returns every
// GitHub issue/PR reference it links — an explicit owner/repo#N, a GitHub URL, or a
// bare #N (resolved against prRepo). It reuses the citation lane's reference grammar
// (citedRefRe / citedURLRe / citedBareRe) so the two lanes agree on what a link is.
//
// It reads the brief FILE on disk rather than the PR diff so a link that lives in the
// brief but outside the PR's added lines still counts as a link. Best-effort: an
// unreadable brief file yields no refs, and the third anchor simply does not fire.
func briefLinkedIssueRefs(root, prRepo, briefFile string) []decisionGateRef {
	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(briefFile)))
	if err != nil {
		return nil
	}
	seen := map[string]bool{}
	var out []decisionGateRef
	add := func(repo string, n int) {
		if repo == "" {
			repo = prRepo
		}
		r := decisionGateRef{Repo: repo, Number: n}
		if n <= 0 || repo == "" || seen[r.key()] {
			return
		}
		seen[r.key()] = true
		out = append(out, r)
	}
	for _, line := range strings.Split(string(raw), "\n") {
		for _, m := range citedRefRe.FindAllStringSubmatch(line, -1) {
			n, _ := strconv.Atoi(m[2])
			add(m[1], n)
		}
		for _, m := range citedURLRe.FindAllStringSubmatch(line, -1) {
			n, _ := strconv.Atoi(m[3])
			add(m[1]+"/"+m[2], n)
		}
		for _, m := range citedBareRe.FindAllStringSubmatch(line, -1) {
			n, _ := strconv.Atoi(m[1])
			add("", n)
		}
	}
	return out
}

// fetchDecisionGateIssue reads one issue's state, closer, and body via the REST
// "Get an issue" endpoint. Untested plumbing (like fetchPRData): a fetch failure or
// an unparseable payload yields nil, so the third anchor does not fire for it —
// never a fabricated corroboration. GitHub populates closed_by only for a CLOSED
// issue; ClosedBy is left empty for anything else, which fails condition (a).
func fetchDecisionGateIssue(repo string, number int) *decisionGateIssue {
	out, err := exec.Command("gh", "api",
		fmt.Sprintf("repos/%s/issues/%d", repo, number)).Output()
	if err != nil {
		return nil
	}
	var payload struct {
		State    string   `json:"state"`
		Body     string   `json:"body"`
		ClosedBy ghAuthor `json:"closed_by"`
	}
	if json.Unmarshal(out, &payload) != nil {
		return nil
	}
	iss := &decisionGateIssue{Ref: fmt.Sprintf("%s#%d", repo, number), Body: payload.Body}
	if strings.EqualFold(payload.State, "closed") {
		iss.ClosedBy = payload.ClosedBy.Login
	}
	return iss
}

// gatherDecisionGateLinks builds the third anchor's pre-fetched data: for every
// brief-file stamp, the needs-decision issues that brief links (briefLinkedIssueRefs)
// with each issue's closer/body fetched once (deduplicated across briefs). A stamp
// whose file is not a brief, or whose brief links nothing, contributes no entry, so
// the anchor does not fire for it. root is the checkout root the brief files are read
// from (".", as the rest of runCorroborate uses).
func gatherDecisionGateLinks(root, repo string, stamps []stamp) decisionGateLinks {
	links := decisionGateLinks{}
	processed := map[string]bool{}
	fetched := map[string]*decisionGateIssue{}
	for _, s := range stamps {
		if processed[s.File] {
			continue
		}
		processed[s.File] = true
		if _, _, ok := expectedBriefID(s.File); !ok {
			continue // not a brief-<NN>.md file
		}
		refs := briefLinkedIssueRefs(root, repo, s.File)
		if len(refs) == 0 {
			continue
		}
		var issues []decisionGateIssue
		for _, ref := range refs {
			iss, done := fetched[ref.key()]
			if !done {
				iss = fetchDecisionGateIssue(ref.Repo, ref.Number)
				fetched[ref.key()] = iss
			}
			if iss != nil {
				issues = append(issues, *iss)
			}
		}
		if len(issues) > 0 {
			links[s.File] = issues
		}
	}
	return links
}
