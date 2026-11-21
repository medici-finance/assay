package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
)

type acceptanceManifest struct {
	Source              acceptanceSource        `json:"source"`
	AcceptanceVersion   int                     `json:"acceptanceVersion"`
	Original            string                  `json:"original"`
	ProtectedVerifyHash string                  `json:"protectedVerifyHash"`
	Runs                []acceptanceRun         `json:"runs"`
	Dispositions        []acceptanceDisposition `json:"dispositions"`
}

type acceptanceDisposition struct {
	Line             int    `json:"evidenceLine"`
	Digest           string `json:"sha256"`
	Kind             string `json:"kind"`
	Explanation      string `json:"explanation"`
	EvidenceRevision string `json:"evidenceRevision,omitempty"`
	Owner            string `json:"owner,omitempty"`
}

// digest binds the complete ordered scan plus the proposed interpretation of
// each signal. It is a coordinate for human adjudication, not a grant.
func (m acceptanceManifest) digest() (string, error) {
	raw, err := json.Marshal(m)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", sha256.Sum256(raw)), nil
}

// checkDispositions only checks coverage and shape. The effect entrypoints
// must independently corroborate the preparation, ruling and cited evidence.
func (m acceptanceManifest) checkDispositions(current acceptanceSource, requireResolved bool) error {
	if !reflect.DeepEqual(m.Source, current) {
		return fmt.Errorf("acceptance adjudication: the recorded source and scan do not match")
	}
	if len(current.Refusals) == 0 || len(m.Dispositions) != len(current.Refusals) {
		return fmt.Errorf("acceptance adjudication: dispositions must cover every refusal in source order")
	}
	for i, refusal := range current.Refusals {
		d := m.Dispositions[i]
		if d.Line != refusal.Line || d.Digest != refusal.Digest {
			return fmt.Errorf("acceptance adjudication: disposition %d does not bind refusal %d", i+1, i+1)
		}
		if (refusal.Line == 0 && d.Kind != "unresolved") || (requireResolved && d.Kind == "unresolved") {
			return fmt.Errorf("acceptance adjudication: refusal %d is unresolved", i+1)
		}
		if strings.TrimSpace(d.Explanation) == "" {
			return fmt.Errorf("acceptance adjudication: disposition %d lacks an explanation", i+1)
		}
		switch d.Kind {
		case "unresolved":
		case "diagnostic-only":
			// The explanation must name the original obligation or explain
			// why this span is outside it. Independent review checks the claim.
		case "resolved":
			if !isObjectID(d.EvidenceRevision) || strings.TrimSpace(d.Owner) == "" {
				return fmt.Errorf("acceptance adjudication: resolved disposition %d needs immutable evidence and an owner", i+1)
			}
		default:
			return fmt.Errorf("acceptance adjudication: disposition %d uses an unknown kind", i+1)
		}
	}
	return nil
}

// checkResolvedEvidence binds a proposed resolution to a landed Git commit and
// an owner the repository roster identifies. The verifier and human still
// establish that the evidence resolves the exact scanned obligation.
func (m acceptanceManifest) checkResolvedEvidence(root string, cfg scanConfig) error {
	for i, d := range m.Dispositions {
		if d.Kind != "resolved" {
			continue
		}
		reader := ordinaryGitCore(root)
		if reader.err != nil || reader.native != "" {
			return fmt.Errorf("acceptance adjudication: could not open the canonical Git reader")
		}
		commit, err := reader.repo.CommitVerifyQuiet(d.EvidenceRevision)
		if err != nil || !commit {
			return fmt.Errorf("acceptance adjudication: resolved disposition %d has no landed evidence at the bound source", i+1)
		}
		// A cited resolution record must have landed. This does not certify
		// the applicability or outcome of any verification witness.
		landed, err := reader.repo.IsAncestor(d.EvidenceRevision, m.Source.Revision)
		if err != nil || !landed {
			return fmt.Errorf("acceptance adjudication: resolved disposition %d has no landed evidence at the bound source", i+1)
		}
		var known bool
		if human, ok := strings.CutPrefix(d.Owner, "human:"); ok {
			known = cfg.HumanLogins[human] != ""
		} else {
			known = cfg.RoleBots[d.Owner] != ""
		}
		if !known {
			return fmt.Errorf("acceptance adjudication: resolved disposition %d does not name a rostered accountable owner", i+1)
		}
	}
	return nil
}
