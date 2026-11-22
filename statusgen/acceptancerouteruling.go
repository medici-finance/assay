package main

import (
	"fmt"
	"strings"
)

const acceptanceAdjudicationID = "acceptance-adjudication"

func acceptanceAdjudicationMarker(briefPath string) string {
	return "<!-- " + acceptanceAdjudicationID + ": " + briefPath + " -->"
}

func acceptanceAdjudicationText(m acceptanceManifest, issue int) (string, error) {
	digest, err := m.digest()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("Approve %s v1\nrepo=%s\nissue=%d\nbrief=%s\nsource=%s\nmanifest=%s",
		acceptanceAdjudicationID, m.Source.Repository, issue, m.Source.BriefPath, m.Source.Revision, digest), nil
}

// corroborateAcceptanceAdjudication verifies the structured ruling attached to
// the independent preparation's decision issue. The caller still authenticates
// the preparation and rechecks source, coverage, floor and credit.
func corroborateAcceptanceAdjudication(c *ghClient, m acceptanceManifest, issue int, ruling string, humanLogins map[string]string) error {
	link, ok := parseRulingURL(ruling)
	if !ok || link.GitLab || issue <= 0 || link.Issue != issue || !strings.EqualFold(link.Repo, m.Source.Repository) {
		return fmt.Errorf("acceptance adjudication: ruling must link to the preparation's GitHub issue")
	}
	want, err := acceptanceAdjudicationText(m, issue)
	if err != nil {
		return err
	}
	marker := acceptanceAdjudicationMarker(m.Source.BriefPath)
	out := resolveRulingOn(c, m.Source.Repository, decisionRecordStamp{ID: acceptanceAdjudicationID, Ruling: ruling}, humanLogins, rulingIssueBinding{
		what: "acceptance adjudication for " + m.Source.BriefPath,
		rules: func(iss rulingIssue, _ rulingLink) bool {
			for _, line := range strings.Split(strings.ReplaceAll(iss.Body, "\r\n", "\n"), "\n") {
				if line == marker {
					return true
				}
			}
			return false
		},
		comment: func(body string) string {
			if strings.TrimSuffix(strings.ReplaceAll(body, "\r\n", "\n"), "\n") != want {
				return "does not approve this exact acceptance adjudication"
			}
			return ""
		},
	})
	if out.Reason != rulingOK {
		return fmt.Errorf("acceptance adjudication: %s: %s", out.Reason, out.Detail)
	}
	return nil
}
