package main

import (
	"strings"
	"testing"
)

func TestAcceptanceManifestBindsOrderedRefusals(t *testing.T) {
	source := acceptanceSource{
		Repository: "example-org/tracker",
		Revision:   strings.Repeat("a", 40),
		BriefBlob:  strings.Repeat("b", 40),
		ReadmeBlob: strings.Repeat("c", 40),
		Refusals: []acceptanceRefusal{
			{Line: 1, Digest: strings.Repeat("a", 64)},
			{Line: 3, Digest: strings.Repeat("b", 64)},
		},
	}
	valid := acceptanceManifest{Source: source, Dispositions: []acceptanceDisposition{
		{Line: 1, Digest: source.Refusals[0].Digest, Kind: "diagnostic-only", Explanation: "Example output"},
		{Line: 3, Digest: source.Refusals[1].Digest, Kind: "resolved", Explanation: "Confirmed outcome", EvidenceRevision: source.Revision, Owner: "verify-desk"},
	}}
	if err := valid.checkDispositions(source, true); err != nil {
		t.Fatal(err)
	}
	digest, err := valid.digest()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		change func(*acceptanceManifest, *acceptanceSource)
	}{
		{"omitted", func(m *acceptanceManifest, _ *acceptanceSource) { m.Dispositions = m.Dispositions[:1] }},
		{"reordered", func(m *acceptanceManifest, _ *acceptanceSource) {
			m.Dispositions[0], m.Dispositions[1] = m.Dispositions[1], m.Dispositions[0]
		}},
		{"unknown", func(m *acceptanceManifest, _ *acceptanceSource) { m.Dispositions[0].Kind = "unknown" }},
		{"unresolved", func(m *acceptanceManifest, _ *acceptanceSource) { m.Dispositions[0].Kind = "unresolved" }},
		{"no immutable evidence", func(m *acceptanceManifest, _ *acceptanceSource) { m.Dispositions[1].EvidenceRevision = "" }},
		{"stale README", func(_ *acceptanceManifest, s *acceptanceSource) { s.ReadmeBlob = s.BriefBlob }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, current := valid, source
			m.Dispositions = append([]acceptanceDisposition(nil), valid.Dispositions...)
			tc.change(&m, &current)
			if err := m.checkDispositions(current, true); err == nil {
				t.Fatal("expected refusal")
			}
		})
	}
	m := valid
	m.Dispositions = append([]acceptanceDisposition(nil), valid.Dispositions...)
	m.Dispositions[0].Kind = "unresolved"
	if err := m.checkDispositions(source, false); err != nil {
		t.Fatalf("preparation may record an unresolved disposition: %v", err)
	}
	if err := m.checkDispositions(source, true); err == nil {
		t.Fatal("unresolved disposition must block an effect")
	}
	valid.Dispositions[0].Explanation += " with context"
	changed, err := valid.digest()
	if err != nil || changed == digest {
		t.Fatalf("edited explanation must change the digest, got %q (%v)", changed, err)
	}
}

func TestAcceptanceResolvedEvidenceRequiresLandedCommitAndOwner(t *testing.T) {
	fx := newCreditRepo(t)
	head := vfRev(t, fx.root, "HEAD")
	m := acceptanceManifest{
		Source: acceptanceSource{Revision: head},
		Dispositions: []acceptanceDisposition{
			{Kind: "resolved", EvidenceRevision: head, Owner: "verifier"},
		},
	}
	cfg := scanConfig{RoleBots: map[string]string{"verifier": "example-verifier"}}
	if err := m.checkResolvedEvidence(fx.root, cfg); err != nil {
		t.Fatal(err)
	}
	m.Dispositions[0].Owner = "unassigned"
	if err := m.checkResolvedEvidence(fx.root, cfg); err == nil || !strings.Contains(err.Error(), "owner") {
		t.Fatalf("expected an unknown-owner refusal, got %v", err)
	}
	m.Dispositions[0].Owner = "verifier"
	m.Dispositions[0].EvidenceRevision = strings.Repeat("9", 40)
	if err := m.checkResolvedEvidence(fx.root, cfg); err == nil || !strings.Contains(err.Error(), "no landed evidence") {
		t.Fatalf("expected missing evidence refusal, got %v", err)
	}
	fx.commit(t, vfMailVerifier, creditPreface+"later evidence\n")
	m.Dispositions[0].EvidenceRevision = vfRev(t, fx.root, "HEAD")
	if err := m.checkResolvedEvidence(fx.root, cfg); err == nil || !strings.Contains(err.Error(), "no landed evidence") {
		t.Fatalf("expected evidence from after the binding to refuse, got %v", err)
	}
}
