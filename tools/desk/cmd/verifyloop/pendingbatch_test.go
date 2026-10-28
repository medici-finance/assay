package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"github.com/medici-finance/assay/tools/desk/internal/loopengine"
)

func TestPendingBatchPreparationKeepsValidationAndFreshHeads(t *testing.T) {
	v, base, _, brief := pendingSetup(t)
	f := &countedPendingForge{pendingForge: base}
	s := v.Issues.(*forgeIssueSource)
	s.forgeFor = func(deskkit.ForgeRepo) (deskkit.Forge, error) { return f, nil }
	// Existing fixture forge has no native batch capability: selection still
	// exercises the original lazy fallback, including a positive hold.
	items, err := v.SelectQueue()
	if err != nil {
		t.Fatal(err)
	}
	item := items[0]
	if item.BriefPath != brief || item.Payload["awaiting_merge"] != "#51" {
		t.Fatal("fixture lost its positive hold")
	}
	item.Payload = map[string]string{} // use fresh evaluation after preparation
	repo := deskkit.ForgeRepo{Owner: "example-org", Name: "tracker"}
	calls := 0
	prepare := func(fail bool) {
		t.Helper()
		s.beginPlanningPass()
		s.preparePendingRepository(f, repo, "example-verifier[bot]", map[string][]loopengine.Item{brief: {item}}, func(r deskkit.ForgeRepo, ns []int) map[int]deskkit.VerifierAttestationResult {
			calls++
			if r != repo || len(ns) != 1 || ns[0] != 41 {
				t.Fatalf("wrong candidate set: %v %v", r, ns)
			}
			receipt, err := deskkit.ReadVerifierAttestation(f, r, ns[0])
			if fail {
				err = errors.New("batch unavailable")
			}
			return map[int]deskkit.VerifierAttestationResult{41: {Receipt: receipt, Err: err}}
		})
	}
	prepare(false)
	before := base.prReads[51]
	issues := f.issueReads
	for range 2 {
		ref, err := s.PendingEvidence(v.Root, v.RootRepo, item, v.wakeNow(), nil)
		if err != nil || ref != "#51" {
			t.Fatalf("prepared hold %q %v", ref, err)
		}
	}
	if f.issueReads != issues || base.prReads[51] != before+2 {
		t.Fatal("preparation reread an attestation or skipped fresh head reconciliation")
	}
	base.move = true
	if ref, err := s.PendingEvidence(v.Root, v.RootRepo, item, v.wakeNow(), nil); err == nil || ref != "" {
		t.Fatal("moved head retained prepared hold")
	}
	base.move = false
	prepare(true)
	for range 2 {
		if _, err := s.PendingEvidence(v.Root, v.RootRepo, item, v.wakeNow(), nil); err == nil {
			t.Fatal("failed batch became absence")
		}
	}
	// A fresh selection must discard both the prepared failure and its receipt.
	prepare(false)
	if ref, err := s.PendingEvidence(v.Root, v.RootRepo, item, v.wakeNow(), nil); err != nil || ref != "#51" {
		t.Fatalf("fresh preparation did not recover: %q %v", ref, err)
	}
	if calls != 3 {
		t.Fatal("wrong per-pass preparation count")
	}
}

func TestPendingBatchPreparationScopesCandidates(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*pendingForge)
	}{
		{"foreign-receipt-repo", func(f *pendingForge) {
			for key, blob := range f.blobs {
				f.blobs[key] = bytes.ReplaceAll(blob, []byte("example-org/tracker#41"), []byte("example-org/other#41"))
			}
		}},
		{"change-kind-receipt", func(f *pendingForge) {
			for key, blob := range f.blobs {
				f.blobs[key] = bytes.ReplaceAll(blob, []byte("example-org/tracker#41"), []byte("https://github.com/example-org/tracker/pull/41"))
			}
		}},
		{"irrelevant", func(f *pendingForge) { f.files[0].Filename = "elsewhere.md" }},
		{"removed", func(f *pendingForge) { f.files[0].Status = "removed" }},
		{"foreign-author", func(f *pendingForge) { f.change.Author.Login = "stranger" }},
		{"recycled-author-id", func(f *pendingForge) { f.pr.Author.ID = 999 }},
		{"incomplete-files", func(f *pendingForge) { f.pr.ChangedFiles = 2 }},
		{"moved-head", func(f *pendingForge) { f.pr.HeadSHA = strings.Repeat("e", 40) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v, f, _, brief := pendingSetup(t)
			tc.change(f)
			s := v.Issues.(*forgeIssueSource)
			called := false
			s.preparePendingRepository(f, deskkit.ForgeRepo{Owner: "example-org", Name: "tracker"}, "example-verifier[bot]", map[string][]loopengine.Item{brief: {{BriefPath: brief}}}, func(deskkit.ForgeRepo, []int) map[int]deskkit.VerifierAttestationResult { called = true; return nil })
			if called {
				t.Fatal("irrelevant or unproven change introduced attestation reads")
			}
		})
	}
}

func TestPendingBatchMissingResultRemainsUnavailable(t *testing.T) {
	v, f, _, brief := pendingSetup(t)
	s := v.Issues.(*forgeIssueSource)
	s.preparePendingRepository(f, deskkit.ForgeRepo{Owner: "example-org", Name: "tracker"}, "example-verifier[bot]", map[string][]loopengine.Item{brief: {{BriefPath: brief}}}, func(deskkit.ForgeRepo, []int) map[int]deskkit.VerifierAttestationResult { return nil })
	receipt := s.pending[v.RootRepo].receipts[pendingAttestationKey{v.RootRepo, 41}]
	if receipt.err == nil {
		t.Fatal("missing batch result became an admitted/empty observation")
	}
}
