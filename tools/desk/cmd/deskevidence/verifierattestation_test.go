package main

import (
	"errors"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"strings"
	"testing"
)

func TestVerifierEvidenceAdmissionRefusesBeforeWrites(t *testing.T) {
	for _, failure := range []error{deskkit.Refused("missing stamp"), deskkit.Unverifiable("unreadable stamp", errors.New("offline"))} {
		t.Run(failure.Error(), func(t *testing.T) {
			f, _ := setupFake(t)
			verifierEvidenceAdmissionFn = func(string, string) (deskkit.VerifierReceipt, error) { return deskkit.VerifierReceipt{}, failure }
			root := rootWithFile(t, "docs/streams/x/brief.md", "## Evidence\nfixture\n")
			if code := run([]string{"example-org/tracker", "main", "--evidence-file", "docs/streams/x/brief.md", "--root", root}); code == 0 {
				t.Fatal("unadmitted Evidence succeeded")
			}
			if f.putCalls != 0 || len(f.changes) != 0 {
				t.Fatal("unadmitted Evidence mutated forge")
			}
		})
	}
}
func TestVerifierEvidenceDraftBindsExactRun(t *testing.T) {
	f, _ := setupFake(t)
	f.defaultBranch = "main"
	receipt := deskkit.VerifierReceipt{Issue: 77, Binding: deskkit.VerifierBinding{Repo: "example-org/tracker", Run: strings.Repeat("a", 64), Source: strings.Repeat("b", 40), Brief: "docs/streams/x/source.md", Model: "gpt-6.1-sol", Tier: "strong"}}
	verifierEvidenceAdmissionFn = func(root, repo string) (deskkit.VerifierReceipt, error) {
		if repo != receipt.Binding.Repo {
			t.Fatal("wrong repo")
		}
		return receipt, nil
	}
	target := "docs/streams/x/evidence.md"
	root := rootWithFile(t, target, "## Evidence\n| 1 | fixture | 0 | ok |\n| 2 | fixture | 0 | ok |\n")
	if code := run([]string{"example-org/tracker", "main", "--evidence-file", target, "--root", root}); code != 0 {
		t.Fatal(code)
	}
	if len(f.changes) != 1 || !strings.Contains(f.changes[0].Body, receipt.EvidenceBinding()) {
		t.Fatalf("draft lost exact run binding: %+v", f.changes)
	}
}

func TestVerifierOutcomeDraftBindsExactRun(t *testing.T) {
	f, _ := setupFake(t)
	f.defaultBranch = "main"
	receipt := deskkit.VerifierReceipt{Issue: 77, Binding: deskkit.VerifierBinding{Repo: "example-org/tracker", Run: strings.Repeat("a", 64), Source: strings.Repeat("b", 40), Brief: "docs/streams/x/source.md", Model: "gpt-6-astra", Tier: "strong"}}
	ac := &auditCtx{attestation: receipt.EvidenceBinding()}
	if err := landOutcomeRecordAsChange(f, deskkit.ForgeRepo{Owner: "example-org", Name: "tracker"}, "example-org/tracker", "main", "docs/streams/verify-outcomes/x/fixture.json", []byte("{}"), "x/source", ac); err != nil {
		t.Fatal(err)
	}
	if len(f.changes) != 1 || !strings.Contains(f.changes[0].Body, receipt.EvidenceBinding()) {
		t.Fatalf("outcome draft lost binding: %+v", f.changes)
	}
}
