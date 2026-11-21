package main

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"
)

const adjudicatedCardPrefix = "<!-- verify-gate-adjudication: "

var adjudicatedCardDigest = regexp.MustCompile(`^[a-f0-9]{64}$`)

type adjudicatedCardRef struct {
	Preparation int    `json:"preparation"`
	Ruling      string `json:"ruling"`
	Manifest    string `json:"manifest"`
}

func (r adjudicatedCardRef) marker() (string, error) {
	link, ok := parseRulingURL(r.Ruling)
	if !ok || link.GitLab || link.Issue != r.Preparation || !adjudicatedCardDigest.MatchString(r.Manifest) {
		return "", fmt.Errorf("acceptance card: invalid preparation, ruling or manifest coordinate")
	}
	raw, err := json.Marshal(r)
	if err != nil {
		return "", err
	}
	return adjudicatedCardPrefix + string(raw) + " -->", nil
}

// parseAdjudicatedCardRef distinguishes an absent marker from a malformed one.
// A present but malformed route must never fall through to an ordinary close.
func parseAdjudicatedCardRef(body string) (adjudicatedCardRef, bool, error) {
	var ref adjudicatedCardRef
	lines := strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n")
	var found bool
	for _, line := range lines {
		if !strings.Contains(line, "<!-- verify-gate-adjudication:") {
			continue
		}
		raw, ok := strings.CutPrefix(line, adjudicatedCardPrefix)
		raw, end := strings.CutSuffix(raw, " -->")
		if found || !ok || !end || json.Unmarshal([]byte(raw), &ref) != nil {
			return adjudicatedCardRef{}, true, fmt.Errorf("acceptance card: malformed or duplicate adjudication marker")
		}
		want, err := ref.marker()
		if err != nil || want != line {
			return adjudicatedCardRef{}, true, fmt.Errorf("acceptance card: adjudication marker is not canonical")
		}
		found = true
	}
	return ref, found, nil
}

// renderAdjudicatedCard is called after verifyAcceptanceReadiness. The normal
// gate marker deduplicates this against an ordinary card for the same brief.
func renderAdjudicatedCard(root string, bf *BriefFile, m acceptanceManifest, preparation int, ruling string) (verifyIssue, error) {
	digest, err := m.digest()
	if err != nil {
		return verifyIssue{}, err
	}
	ref := adjudicatedCardRef{Preparation: preparation, Ruling: ruling, Manifest: digest}
	routeMarker, err := ref.marker()
	if err != nil {
		return verifyIssue{}, err
	}
	card := makeGateCard(root, bf, cardAcceptance)
	if card.Unavailable != "" {
		return verifyIssue{}, fmt.Errorf("acceptance card: %s", card.Unavailable)
	}
	marker := verifyMarker(bf.Brief)
	scan, err := json.MarshalIndent(m.Source.Refusals, "", "  ")
	if err != nil {
		return verifyIssue{}, err
	}
	dispositions, err := json.MarshalIndent(m.Dispositions, "", "  ")
	if err != nil {
		return verifyIssue{}, err
	}
	var body strings.Builder
	fmt.Fprintf(&body, "%s\n%s\n\n", marker, routeMarker)
	fmt.Fprintf(&body, "## Final acceptance for %s\n\n", card.Brief)
	fmt.Fprintf(&body, "The ordinary hold scan still reports the signals listed below. The [exact adjudication](%s) supports this separate route.\n\n", ruling)
	fmt.Fprintf(&body, "Source `%s` · brief blob `%s` · README blob `%s` · manifest `%s`.\n\n", m.Source.Revision, m.Source.BriefBlob, m.Source.ReadmeBlob, digest)
	fmt.Fprintf(&body, "[Original Evidence and advisories](https://github.com/%s/blob/%s/%s#evidence)\n\n", m.Source.Repository, m.Source.Revision, m.Source.BriefPath)
	fmt.Fprintf(&body, "### Full hold scan\n\n```json\n%s\n```\n\n", scan)
	fmt.Fprintf(&body, "### Adjudicated dispositions\n\n```json\n%s\n```\n\n", dispositions)
	body.WriteString("Close this issue to accept the brief. The close workflow rechecks the source, verification credit and adjudication.\n")
	if body.Len() > 60_000 {
		return verifyIssue{}, fmt.Errorf("acceptance card: the full scan and dispositions exceed the issue body limit")
	}
	return verifyIssue{
		Card: card, Brief: bf.Brief, Title: issueTitle("verify-gate: ", bf.Brief, bf.Title),
		Labels: []string{"verify-gate"}, Marker: marker, Body: body.String(),
	}, nil
}

// makeAdjudicatedVerifyIssue is the issuer's readiness boundary. It renders
// only, leaving deduplication and publication to the canonical workflow
// adapter. It also preflights the same decision gate used on close.
func makeAdjudicatedVerifyIssue(root, briefRef string, preparation int, ruling string,
	forge *ghClient, humanLogins map[string]string, ciWorkflowToken bool) (verifyIssue, error) {
	bf, err := locateAcceptanceBrief(root, briefRef)
	if err != nil {
		return verifyIssue{}, err
	}
	repo, err := attestationTrackingRepo(root)
	if err != nil {
		return verifyIssue{}, err
	}
	m, err := loadAcceptancePreparation(repo, preparation, ciWorkflowToken)
	if err != nil {
		return verifyIssue{}, err
	}
	if _, _, _, err := planAdjudicatedClose(root, briefRef, m, preparation, ruling, time.Now(), forge, humanLogins, ciWorkflowToken); err != nil {
		return verifyIssue{}, err
	}
	return renderAdjudicatedCard(root, bf, m, preparation, ruling)
}
