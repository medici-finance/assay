package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/medici-finance/assay/deskcore/attestation"
)

func acceptancePreparationBody(m acceptanceManifest) (string, error) {
	raw, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return "", err
	}
	return acceptanceAdjudicationMarker(m.Source.BriefPath) + "\n\n" +
		"Independent verifier preparation for human adjudication.\n\n" +
		"```json\n" + string(raw) + "\n```", nil
}

// verifyAcceptancePreparation verifies a forge-attributed, unedited record.
// A worker-shaped manifest can be inspected but cannot satisfy this check.
// Its caller must still rederive the live source and recheck verification.
func verifyAcceptancePreparation(m acceptanceManifest, issue int, ciWorkflowToken bool) error {
	cfg := scanEffectiveConfig()
	policy := evidenceActorPolicyFromRoster()
	worker := cfg.RoleBots["worker"]
	if policy.Unavailable != "" || !policy.idPinned() || policy.VerifierForge != forgeGitHub ||
		worker == "" || cfg.Bots[worker] == 0 ||
		strings.EqualFold(worker, policy.Verifier.Login) || cfg.Bots[worker] == policy.Verifier.ID {
		return fmt.Errorf("acceptance preparation: could-not-check distinct verifier authority")
	}
	if issue <= 0 || !repoFrontmatterRe.MatchString(m.Source.Repository) || m.Source.BriefPath == "" {
		return fmt.Errorf("acceptance preparation: expected a canonical repository, brief and issue number")
	}
	want, err := acceptancePreparationBody(m)
	if err != nil {
		return err
	}
	title := "Acceptance preparation: " + m.Source.BriefPath
	target := fmt.Sprintf("%s#%d", m.Source.Repository, issue)
	cc := creditContext{ciWorkflowToken: ciWorkflowToken}
	read := func() error {
		record, err := cc.readIssue(target)
		if err != nil {
			return fmt.Errorf("acceptance preparation: could-not-check issue: %w", err)
		}
		a := record.Author
		if a == nil || !strings.EqualFold(a.Type, "Bot") ||
			!strings.EqualFold(a.Login, policy.Verifier.Login+"[bot]") || a.ID != policy.Verifier.ID {
			return fmt.Errorf("acceptance preparation: issue is not authored by the pinned verifier App")
		}
		if record.Title != title || strings.TrimSuffix(record.Body, "\n") != want {
			return fmt.Errorf("acceptance preparation: issue does not carry the exact manifest")
		}
		return nil
	}

	if err := read(); err != nil {
		return err
	}
	history, err := cc.readTrust(target)
	if err != nil {
		return fmt.Errorf("acceptance preparation: could-not-check issue history: %w", err)
	}
	if why := attestation.HistoryProblem(history.BodyHistoryKnown != nil && *history.BodyHistoryKnown, history.Complete, history.BodyEditedAt != ""); why != "" {
		return fmt.Errorf("acceptance preparation: %s", why)
	}
	return read()
}

// loadAcceptancePreparation reads by the checkout's repository, never an
// address taken from the issue's body. Admission re-reads and authenticates it.
func loadAcceptancePreparation(repo string, issue int, ciWorkflowToken bool) (acceptanceManifest, error) {
	if !repoFrontmatterRe.MatchString(repo) || issue <= 0 {
		return acceptanceManifest{}, fmt.Errorf("acceptance preparation: invalid issue coordinate")
	}
	cc := creditContext{ciWorkflowToken: ciWorkflowToken}
	record, err := cc.readIssue(fmt.Sprintf("%s#%d", repo, issue))
	if err != nil {
		return acceptanceManifest{}, err
	}
	_, section, ok := strings.Cut(record.Body, "\n\n```json\n")
	raw, tail, ended := strings.Cut(section, "\n```")
	if !ok || !ended || strings.TrimSpace(tail) != "" {
		return acceptanceManifest{}, fmt.Errorf("acceptance preparation: issue does not contain a single manifest")
	}
	var m acceptanceManifest
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return acceptanceManifest{}, fmt.Errorf("acceptance preparation: decode manifest: %w", err)
	}
	if !strings.EqualFold(m.Source.Repository, repo) {
		return acceptanceManifest{}, fmt.Errorf("acceptance preparation: manifest names another repository")
	}
	want, err := acceptancePreparationBody(m)
	if err != nil {
		return acceptanceManifest{}, err
	}
	if strings.TrimSuffix(record.Body, "\n") != want {
		return acceptanceManifest{}, fmt.Errorf("acceptance preparation: issue is not the canonical manifest rendering")
	}
	return m, nil
}
