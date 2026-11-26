package deskkit

import (
	"errors"
	"fmt"
	"strings"
)

// OutboundBodyMaxBytes is the size cap for outward comment and review bodies.
const OutboundBodyMaxBytes = 16 * 1024

// CheckStandaloneChangeComment applies common publication checks with the
// application's policy data. Its diagnostics omit the offending text.
func CheckStandaloneChangeComment(body, repo string, number int, configured, live string, humans, privateRepos, withheld []string) error {
	configuredRank := visibilityExposure(ParseVisibility(configured))
	liveRank := visibilityExposure(ParseVisibility(live))
	if liveRank < 0 || configuredRank < liveRank {
		return errors.New("repository visibility exceeds the configured publication scope or could not be verified")
	}
	const surface = "change comment"
	if err := ScanSurfaceSecrets(surface, []byte(body)); err != nil {
		var finding *ScanFinding
		if errors.As(err, &finding) {
			return fmt.Errorf("comment body refused by secret scan (%s at line %d)", finding.Rule, finding.Line)
		}
		return errors.New("comment body refused by secret scan")
	}
	if _, found := impersonatedRulingBy(markerSurface(body), humans); found {
		return errors.New("comment body refused by ruling attribution scan")
	}
	for _, finding := range scanPersonalData(body, defaultEmailAllowed) {
		if finding.rule != RulePIIPhoneAmbiguous {
			return fmt.Errorf("comment body refused by %s at line %d", finding.rule, finding.line)
		}
	}
	if configuredRank > 0 {
		private := make(map[string]bool, len(privateRepos))
		for _, name := range privateRepos {
			if !strings.EqualFold(name, repo) {
				private[strings.ToLower(name)] = true
			}
		}
		findings, _ := scanSelfContainment(surface, body, SelfContainOpts{Repo: repo, NumberHint: number}, private, nil, withheld)
		if len(findings) != 0 {
			return fmt.Errorf("comment body refused by publication scan (%s at line %d)", findings[0].category, findings[0].line)
		}
	}
	return nil
}
