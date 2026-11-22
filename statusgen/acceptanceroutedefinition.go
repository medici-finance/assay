package main

import (
	"crypto/sha256"
	"fmt"
	"path/filepath"
)

type acceptanceDefinition struct {
	version   int
	original  string
	tableHash string
	verify    string
}

// readAcceptanceDefinition uses the same protected-table provenance as the
// ordinary closure audit. Unprotected or unreadable coordinates cannot give an
// adjudication a substitute acceptance definition.
func readAcceptanceDefinition(root string, source acceptanceSource) (acceptanceDefinition, error) {
	blob, raw, err := readAcceptanceBlob(root, source.Revision, source.BriefPath)
	if err != nil {
		return acceptanceDefinition{}, err
	}
	if blob != source.BriefBlob {
		return acceptanceDefinition{}, fmt.Errorf("acceptance definition: brief blob differs from the bound source")
	}
	path := filepath.Join(root, filepath.FromSlash(source.BriefPath))
	bf, ok, err := parseBriefFileBytes(path, raw)
	if err != nil {
		return acceptanceDefinition{}, err
	}
	if !ok || bf.Gate != "human" {
		return acceptanceDefinition{}, fmt.Errorf("acceptance definition: expected a canonical human-gated brief")
	}
	verify, evidence := briefSectionsOf(string(raw))
	plan, refusal := protectedAcceptance(verify, evidence, path)
	if refusal != "" {
		return acceptanceDefinition{}, fmt.Errorf("acceptance definition: %s", refusal)
	}
	if !isObjectID(plan.Original) || len(briefVerifyRows(plan.Verify)) == 0 {
		return acceptanceDefinition{}, fmt.Errorf("acceptance definition: protected original table coordinates are missing")
	}
	return acceptanceDefinition{
		version: max(bf.Version, 1), original: plan.Original,
		tableHash: fmt.Sprintf("%x", sha256.Sum256([]byte(plan.Verify))), verify: plan.Verify,
	}, nil
}

func (m acceptanceManifest) checkDefinition(current acceptanceDefinition) error {
	if m.AcceptanceVersion != current.version || m.Original != current.original ||
		m.ProtectedVerifyHash != current.tableHash {
		return fmt.Errorf("acceptance adjudication: recorded acceptance definition no longer matches")
	}
	return nil
}
