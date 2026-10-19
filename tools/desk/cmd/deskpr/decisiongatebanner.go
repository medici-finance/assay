package main

// decisiongatebanner.go — the PR-body banner of the decision-gate hold
// (spec/lifecycle-v1.md §4.5).
//
// A pull request that delivers a `gate: human` brief carries a fixed banner naming the
// brief's decision issue and whether it is ruled. A reviewer working through a batch of
// bodies quickly needs the fact in the one place they read. The banner INFORMS AND BLOCKS
// NOTHING: `deskpr create` inserts it when it is missing and never refuses on its account.
// The refusal itself lives at the status transition (statusgen's lint and
// --decision-gate), never here and never at the ready-flip.
//
// Everything it states is read OFFLINE from the brief the `Brief:` trailer resolves to,
// under the same --root requireTrailer already globbed: the `decision-issue:` number and
// whether a `ruling:` link is recorded. It does not fetch the issue, so it never claims
// the linked comment is a valid ruling — only that a link is on record. `deskpr update` and
// `deskpr edit` keep the body as it is; a banner written at create stays until the body
// is re-derived.

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// decisionGateBannerMarker is the fixed, grep-able head of the banner. A body that
// already carries it is left alone, so a re-run never stacks a second banner.
const decisionGateBannerMarker = "GATE: HUMAN — DECISION"

var (
	bannerGateRe   = regexp.MustCompile(`(?m)^gate:[ \t]*(.*)$`)
	bannerIssueRe  = regexp.MustCompile(`(?m)^decision-issue:[ \t]*(.*)$`)
	bannerRulingRe = regexp.MustCompile(`(?m)^ruling:[ \t]*(.*)$`)
)

// bannerBriefInfo is what the banner states about the delivered brief.
type bannerBriefInfo struct {
	gateHuman     bool
	decisionIssue int  // 0 = none on record
	rulingLinked  bool // a non-empty ruling: value is recorded
}

func bannerScalar(re *regexp.Regexp, fm string) string {
	m := re.FindStringSubmatch(fm)
	if m == nil {
		return ""
	}
	v := strings.TrimSpace(m[1])
	if i := strings.Index(v, " #"); i >= 0 {
		v = strings.TrimSpace(v[:i])
	}
	return strings.Trim(v, `"'`)
}

// readBannerBrief reads the frontmatter of the brief root/docs/streams/<stream>/brief-<nn>-*.md.
// ok is false when no such brief is found or it has no frontmatter.
func readBannerBrief(root, stream, nn string) (info bannerBriefInfo, ok bool) {
	matches, _ := filepath.Glob(filepath.Join(root, "docs", "streams", stream, "brief-"+nn+"-*.md"))
	if len(matches) == 0 {
		return info, false
	}
	raw, err := os.ReadFile(matches[0])
	if err != nil {
		return info, false
	}
	lines := strings.Split(strings.ReplaceAll(string(raw), "\r\n", "\n"), "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return info, false
	}
	end := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			end = i
			break
		}
	}
	if end < 0 {
		return info, false
	}
	fm := strings.Join(lines[1:end], "\n")
	info.gateHuman = strings.EqualFold(bannerScalar(bannerGateRe, fm), "human")
	if n, perr := strconv.Atoi(strings.TrimPrefix(bannerScalar(bannerIssueRe, fm), "#")); perr == nil && n > 0 {
		info.decisionIssue = n
	}
	info.rulingLinked = bannerScalar(bannerRulingRe, fm) != ""
	return info, true
}

// decisionGateBanner renders the fixed banner for brief <stream>/<nn>.
func decisionGateBanner(stream, nn string, info bannerBriefInfo) string {
	state := "NOT RULED"
	if info.rulingLinked && info.decisionIssue > 0 {
		state = "RULING LINKED"
	}
	issue := "none filed yet"
	if info.decisionIssue > 0 {
		issue = "#" + strconv.Itoa(info.decisionIssue)
	}
	ruling := "none recorded"
	if info.rulingLinked {
		ruling = "a `ruling:` link is recorded in the brief (its author is checked at the status transition, not here)"
	}
	return fmt.Sprintf("> **%s %s.** This PR delivers `%s/%s`, a `gate: human` brief.\n"+
		"> Decision issue: %s. Ruling: %s.\n"+
		"> Moving the brief to implemented, verified or done is refused until a human rules on its decision "+
		"issue (lifecycle-v1 §4.5). This banner informs; it blocks nothing.\n",
		decisionGateBannerMarker, state, stream, nn, issue, ruling)
}

// withDecisionGateBanner returns body with the banner prepended when its `Brief:` trailer
// resolves under root to a `gate: human` brief and the body does not already carry the
// banner. Every other body — an Issue: or Authors: trailer, the scan carrier, a brief that
// is not gate: human or cannot be read — comes back unchanged. It never returns an error:
// the banner blocks nothing. added reports whether it inserted one.
func withDecisionGateBanner(body []byte, root, dir string) (out []byte, added bool) {
	if strings.Contains(string(body), decisionGateBannerMarker) {
		return body, false
	}
	trs, err := deskkit.ParseTrailers(body)
	if err != nil || len(trs) == 0 || trs[0].Kind != deskkit.TrailerBrief {
		return body, false
	}
	stream, nn, ok := splitBriefTrailer(trs[0].Value)
	if !ok {
		return body, false
	}
	if !filepath.IsAbs(root) {
		root = filepath.Join(dir, root)
	}
	info, ok := readBannerBrief(root, stream, nn)
	if !ok || !info.gateHuman {
		return body, false
	}
	return append([]byte(decisionGateBanner(stream, nn, info)+"\n"), body...), true
}
