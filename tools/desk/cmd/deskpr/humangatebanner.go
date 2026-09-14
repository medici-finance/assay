package main

// humangatebanner.go — brief-18 Task 3: the PR-body banner.
//
// THE DEFECT. sdlc/05's deliverable merged inside a ~20-PR batch pass with nothing in
// the PR body saying "gate:human, decision pending" — a human reviewing bodies at batch
// speed had nothing to catch on, and the merge landed three days before the decision
// issue was even filed (F-gate-merged-first). The decision-issue reply is the one signal
// standing between an unruled gate and an irreversible landing; the PR-body banner is the
// layer immediately behind it — the human merging the PR is handed the fact in the one
// place a batch pass actually reads.
//
// SCOPE. This binds `deskpr create` only. It is a BODY-CONTENT requirement layered on the
// SAME brief resolution requireTrailer already performs (the `Brief: <stream>/<NN>`
// trailer, globbed under --root) — not a second identity for the PR->brief link, and not
// a new network read: everything it checks is local (the brief file's own frontmatter,
// the body text the caller is about to submit). `deskpr update` pushes commits to an
// EXISTING PR without a new body (its own trailer check reads the PR's CURRENT body off
// the forge — see its own call to requireTrailer) and is deliberately NOT covered here:
// there is no new body text to check a banner against at that call.
//
// WHAT IT DOES NOT DO. It does not touch deskflip's ready-flip (brief-18 Verify row 8 is
// a NEGATIVE row against exactly that widening) and it does not fetch the decision
// issue's LIVE open/closed state over the network — deskpr's minted App token is scoped
// to the PR's OWN repo, which for a dehoused cross-repo item (brief lives in one repo,
// deliverable in another) is a DIFFERENT installation than the one that could read the
// issue. The banner instead states what is knowable OFFLINE, from the brief's own
// frontmatter: the decision-issue NUMBER when one is on record, or an explicit "none
// filed yet" when bf.DecisionIssue == 0 — never a fabricated open/closed guess.

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// humanGateBannerMarker is the fixed, grep-able substring every gate:human brief's PR
// body must carry — unmissable by construction, the same reasoning brief-18's Task 3
// states for why it must be FIXED rather than free prose a batch pass can skim past.
const humanGateBannerMarker = "GATE: HUMAN — DECISION"

// briefGateLineRe / briefDecisionIssueLineRe read the two frontmatter lines this check
// needs. Deliberately a SMALL local reader rather than importing deskkit's
// BriefRiskFromBody: that helper resolves the brief under RootForRepo(the PR's OWN
// repo), which is wrong for a dehoused cross-repo item (the brief's repo and the PR's
// repo differ); this instead resolves under the SAME explicit --root requireTrailer
// already uses, so the two checks agree by construction on which file they are reading.
var (
	briefGateLineRe          = regexp.MustCompile(`(?m)^gate:\s*(.*)$`)
	briefDecisionIssueLineRe = regexp.MustCompile(`(?m)^decision-issue:\s*(.*)$`)
)

// briefFrontmatterFenceLocal returns the text between a brief file's leading `---`
// fences, or "" when the content does not open with one. Mirrors
// deskkit.briefFrontmatterFence (unexported there) so this package does not need a new
// export from a security-relevant shared library for one small local read.
func briefFrontmatterFenceLocal(content string) string {
	lines := strings.Split(content, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return ""
	}
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			return strings.Join(lines[1:i], "\n")
		}
	}
	return ""
}

// resolvedBriefGateInfo is what checkHumanGateBanner needs from the brief a PR's
// Brief: trailer names.
type resolvedBriefGateInfo struct {
	gateHuman     bool
	decisionIssue int // 0 = none on record
}

// readBriefGateInfo globs for the brief file the same way requireTrailer does (root/dir,
// stream, nn) and reads its gate:/decision-issue: frontmatter lines. ok is false when the
// brief cannot be resolved or read at all, in which case the caller treats brief-18 as
// simply not applicable here (requireTrailer has ALREADY hard-refused an unresolvable
// Brief: trailer by the time this runs, so ok==false here means "not a Brief: trailer at
// all" — an Issue-only PR, which this check does not apply to).
func readBriefGateInfo(root, dir, stream, nn string) (info resolvedBriefGateInfo, ok bool) {
	if !filepath.IsAbs(root) {
		root = filepath.Join(dir, root)
	}
	matches, _ := filepath.Glob(filepath.Join(root, "docs", "streams", stream, "brief-"+nn+"-*.md"))
	if len(matches) == 0 {
		return info, false
	}
	raw, err := os.ReadFile(matches[0])
	if err != nil {
		return info, false
	}
	fence := briefFrontmatterFenceLocal(string(raw))
	if fence == "" {
		return info, false
	}
	if m := briefGateLineRe.FindStringSubmatch(fence); m != nil {
		info.gateHuman = strings.EqualFold(strings.Trim(strings.TrimSpace(m[1]), `"'`), "human")
	}
	if m := briefDecisionIssueLineRe.FindStringSubmatch(fence); m != nil {
		if n, perr := strconv.Atoi(strings.TrimSpace(m[1])); perr == nil && n > 0 {
			info.decisionIssue = n
		}
	}
	return info, true
}

// checkHumanGateBanner requires the fixed banner in body when the resolved brief gates
// on a human. It is a no-op when the brief cannot be resolved (readBriefGateInfo's
// ok==false) or resolves but is not gate:human.
//
// Two things are required, not just the marker's presence: the banner must NAME the
// decision issue it is warning about (its own number when one is on record — Task 3 says
// "naming its decision issue", not merely gesturing at the existence of one) or say
// plainly that none is filed yet (the true, honest state for a brief like this one, at
// authoring time, before any decision issue exists).
func checkHumanGateBanner(body []byte, root, dir, stream, nn string) error {
	info, ok := readBriefGateInfo(root, dir, stream, nn)
	if !ok || !info.gateHuman {
		return nil
	}
	b := string(body)
	if !strings.Contains(b, humanGateBannerMarker) {
		return deskkit.Refused(fmt.Sprintf(
			"refused: this PR delivers a gate:human brief (%s/%s) but its body carries no %q banner — "+
				"add a fixed, unmissable line naming its decision issue and state (brief-18: sdlc/05's "+
				"deliverable merged inside a ~20-PR batch pass with nothing in the body saying so)",
			stream, nn, humanGateBannerMarker))
	}
	if info.decisionIssue > 0 {
		ref := "#" + strconv.Itoa(info.decisionIssue)
		if !strings.Contains(b, ref) {
			return deskkit.Refused(fmt.Sprintf(
				"refused: the %q banner is present but does not name the brief's own decision issue "+
					"(%s) — the banner must say WHICH issue, not just that one exists", humanGateBannerMarker, ref))
		}
		return nil
	}
	lower := strings.ToLower(b)
	if !strings.Contains(lower, "no decision issue") && !strings.Contains(lower, "none filed") {
		return deskkit.Refused(fmt.Sprintf(
			"refused: the %q banner is present but the brief carries no decision-issue on record and "+
				"the body does not say so — state plainly that none is filed yet (e.g. \"no decision "+
				"issue filed yet\")", humanGateBannerMarker))
	}
	return nil
}

// checkHumanGateBannerFromBody re-derives the Brief: trailer's (stream, nn) from body and
// applies checkHumanGateBanner. A no-op (nil) for a body with no trailer at all, a
// malformed trailer set, or an Issue: trailer — requireTrailer has already refused a
// malformed or absent trailer by the time this runs; this simply has nothing further to
// check for an Issue-only PR.
func checkHumanGateBannerFromBody(body []byte, root, dir string) error {
	trs, err := deskkit.ParseTrailers(body)
	if err != nil || len(trs) == 0 || trs[0].Kind != deskkit.TrailerBrief {
		return nil
	}
	stream, nn, ok := splitBriefTrailer(trs[0].Value)
	if !ok {
		return nil
	}
	return checkHumanGateBanner(body, root, dir, stream, nn)
}
