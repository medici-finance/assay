package main

// decisiongateanchor.go — the THIRD human-stamp corroboration anchor.
//
// The stamp gate (corroborate.go) accepts a gate:human `human:<name>` token when
// the named human left an APPROVED review or an explicit approval COMMENT on the
// brief's own PR. But the house's sanctioned ratification channel for a gate:human
// DECISION is not a PR approval at all: the human ratifies and CLOSES the linked
// `needs-decision` issue, and that closed issue IS the record. A decision brief
// ratified correctly on that channel therefore could not green its own corroborate
// lint — the mismatch the house tracker's ruling resolves (Option 1: teach the lint
// to accept the decision-issue channel).
//
// This file adds that third accepted anchor ALONGSIDE the two PR anchors, never
// replacing them. It fires ONLY when all three of the ruling's conditions hold:
//
//	(a) the linked issue is CLOSED by the blessed human (ASSAY_BLESS_LOGIN) — the
//	    single highest-authority login in the trust roster (rosterconfig.go), and
//	    the only close that ratifies;
//	(b) the issue carries the per-record marker "<!-- decision-gate: <id> -->"
//	    naming THIS EXACT record — so a decision issue filed for one record cannot
//	    ratify another;
//	(c) the record file LINKS the issue.
//
// The anchor fires for TWO kinds of stamp-carrying file, resolved to a per-record
// id by decisionGateAnchorID (never replacing one form with the other):
//
//   - a brief-<NN>.md brief file, whose id is the canonical "<stream>/<NN>"
//     (expectedBriefID) — the original case. The house's sanctioned channel for a
//     gate:human DECISION is closing the linked needs-decision issue, so a decision
//     brief ratified that way corroborates here rather than through a PR approval.
//
//   - a DR-<slug>.md design-decision record under docs/streams/decisions/, whose id
//     is its "DR-<slug>" (decisionRecordID). A DR's `decided-by:` names the approving
//     human, but the human act was CLOSING the linked needs-decision issue, not
//     signing the DR's own PR — so a concrete `decided-by: "human:<name>"` on a DR
//     would otherwise come back MISSING-CORROBORATION. This anchor lets it corroborate
//     against the same decision-issue channel, so a DR can carry a real approver name
//     instead of the literal "human:<name>" placeholder.
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

// decisionGateMarker renders the per-record decision-gate marker a needs-decision
// issue must carry to corroborate a gate:human stamp through this third anchor.
// recordID is the canonical per-record id: a brief's "<stream>/<NN>" (expectedBriefID)
// or a decision record's "DR-<slug>" (decisionRecordID). The marker names the EXACT
// record so a decision issue filed for one record cannot ratify another — condition
// (b) of the ruling.
//
// It is DELIBERATELY DISTINCT from decisionMarker ("<!-- needs-decision: … -->",
// decisionissues.go), which is the emitter's idempotency key. This marker is the
// corroboration anchor a human (or the decision-issue template) places to say "this
// closed issue ratifies THIS brief"; keeping the two strings separate means the
// idempotency key alone never satisfies the corroboration gate.
func decisionGateMarker(recordID string) string {
	return "<!-- decision-gate: " + recordID + " -->"
}

// decisionRecordID returns the "DR-<slug>" id of a design-decision record from its
// file path, when the basename is a DR-<slug>.md file whose slug matches the shared
// DR-id grammar (decisionIDRe, designgate.go — the same shape the design gate
// validates). It is the DR analogue of expectedBriefID: the per-record identifier
// that keys the decision-gate marker. ok is false for anything that is not a
// DR-<slug>.md file.
//
// It matches on the basename shape only, exactly as expectedBriefID does for briefs
// (which constrains no directory): the anchor's security lives in conditions
// (a)/(b)/(c), not in the path — a stamp on a DR-shaped file still corroborates only
// against a blessed-human-closed issue carrying THIS record's marker.
func decisionRecordID(path string) (id string, ok bool) {
	base := filepath.Base(path)
	if !strings.HasSuffix(base, ".md") {
		return "", false
	}
	id = strings.TrimSuffix(base, ".md")
	if !decisionIDRe.MatchString(id) {
		return "", false
	}
	return id, true
}

// decisionGateAnchorID resolves a stamp-carrying file path to the per-record id used
// in its decision-gate marker, for the TWO file kinds this anchor covers: a
// brief-<NN>.md brief file yields its canonical "<stream>/<NN>" id (expectedBriefID),
// and a DR-<slug>.md decision record yields its "DR-<slug>" id (decisionRecordID).
// ok is false for any other file — the anchor is then N/A and the two PR anchors
// decide. The two id namespaces never collide: a brief id always carries a "/", a DR
// id never does.
func decisionGateAnchorID(path string) (id string, ok bool) {
	if id, _, ok := expectedBriefID(path); ok {
		return id, true
	}
	if id, ok := decisionRecordID(path); ok {
		return id, true
	}
	return "", false
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
// (the house tracker's ruling (Option 1: a linked decision issue closed by the
// blessed login corroborates)). It reports CORROBORATED for a human:<name> stamp
// found in a brief file OR a DR-<slug>.md decision record when a needs-decision issue
// that record LINKS satisfies ALL THREE conditions (a)/(b)/(c) above.
//
// It fails CLOSED in every ambiguous direction: a stamp on a file that is neither a
// brief nor a decision record, an empty links map, an unset bless login, an open (or
// otherwise non-blessed-closed) issue, or a marker naming a different record all
// yield ok == false, leaving the two PR anchors to decide. None of the three
// conditions is sufficient alone.
func decisionGateCorroboration(s stamp, gates decisionGateLinks) (evidence string, ok bool) {
	recordID, okID := decisionGateAnchorID(s.File)
	if !okID {
		return "", false // stamp is not in a brief-<NN>.md or DR-<slug>.md file — this anchor is N/A
	}
	linked := gates[s.File] // (c) only issues THIS record links are present here
	if len(linked) == 0 {
		return "", false
	}
	blessLogin := scanEffectiveConfig().Bless.Login
	if blessLogin == "" {
		return "", false // no blessed closer configured — the strict-but-inert direction
	}
	want := decisionGateMarker(recordID)
	for _, iss := range linked {
		if !strings.EqualFold(iss.ClosedBy, blessLogin) { // (a) closed by the blessed human
			continue
		}
		if !strings.Contains(iss.Body, want) { // (b) marker names THIS exact brief
			continue
		}
		return fmt.Sprintf("needs-decision issue %s closed by the blessed human %s and carrying the "+
			"per-brief marker %q (the house tracker's ruling)", iss.Ref, blessLogin, want), true
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

// recordLinkedIssueRefs reads the record file at root/recordFile (a brief-<NN>.md
// brief or a DR-<slug>.md decision record) and returns every GitHub issue/PR
// reference it links — an explicit owner/repo#N, a GitHub URL, or a bare #N (resolved
// against prRepo). It reuses the citation lane's reference grammar (citedRefRe /
// citedURLRe / citedBareRe) so the two lanes agree on what a link is.
//
// It reads the FILE on disk rather than the PR diff so a link that lives in the
// record but outside the PR's added lines still counts as a link. Best-effort: an
// unreadable file yields no refs, and the third anchor simply does not fire.
func recordLinkedIssueRefs(root, prRepo, recordFile string) []decisionGateRef {
	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(recordFile)))
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
// stamp on a brief-<NN>.md brief or a DR-<slug>.md decision record, the needs-decision
// issues that record links (recordLinkedIssueRefs) with each issue's closer/body
// fetched once (deduplicated across records). A stamp whose file is neither, or whose
// record links nothing, contributes no entry, so the anchor does not fire for it.
// root is the checkout root the record files are read from (".", as the rest of
// runCorroborate uses).
func gatherDecisionGateLinks(root, repo string, stamps []stamp) decisionGateLinks {
	links := decisionGateLinks{}
	processed := map[string]bool{}
	fetched := map[string]*decisionGateIssue{}
	for _, s := range stamps {
		if processed[s.File] {
			continue
		}
		processed[s.File] = true
		if _, ok := decisionGateAnchorID(s.File); !ok {
			continue // not a brief-<NN>.md or DR-<slug>.md file
		}
		refs := recordLinkedIssueRefs(root, repo, s.File)
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
