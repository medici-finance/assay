package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVerifierEvidenceAdmissionRefusesBeforeWrites(t *testing.T) {
	for _, failure := range []error{deskkit.Refused("missing stamp"), deskkit.Unverifiable("unreadable stamp", errors.New("offline"))} {
		t.Run(failure.Error(), func(t *testing.T) {
			f, _ := setupFake(t)
			verifierEvidenceAdmissionFn = func(string, string, string) (deskkit.VerifierReceipt, error) {
				return deskkit.VerifierReceipt{}, failure
			}
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
	receipt := deskkit.VerifierReceipt{Issue: 77, Binding: deskkit.VerifierBinding{Repo: "example-org/tracker", Run: strings.Repeat("a", 64), Source: strings.Repeat("b", 40), Brief: "docs/streams/x/brief-01-source.md", PlanSHA256: planDigest(""), Model: "gpt-6.1-sol", Tier: "strong"}}
	target := "docs/streams/x/brief-01-source.md"
	verifierEvidenceAdmissionFn = func(root, repo, got string) (deskkit.VerifierReceipt, error) {
		if repo != receipt.Binding.Repo || got != target {
			t.Fatalf("admission asked for %s %s, want %s %s", repo, got, receipt.Binding.Repo, target)
		}
		return receipt, nil
	}
	root := rootWithFile(t, target, "## Evidence\n| 1 | fixture | 0 | ok |\n| 2 | fixture | 0 | ok |\n")
	if code := run([]string{"example-org/tracker", "main", "--evidence-file", target, "--root", root}); code != 0 {
		t.Fatal(code)
	}
	if len(f.changes) != 1 || !strings.Contains(f.changes[0].Body, receipt.EvidenceBinding()) {
		t.Fatalf("draft lost exact run binding: %+v", f.changes)
	}
}

// An attestation for one brief must not admit Evidence landing in another,
// whichever landing shape names the target; a bound direct commit carries the
// binding in its own message.
func TestVerifierEvidenceTargetBoundToAttestedBrief(t *testing.T) {
	receipt := deskkit.VerifierReceipt{Issue: 77, Binding: deskkit.VerifierBinding{Repo: "example-org/tracker", Run: strings.Repeat("a", 64), Source: strings.Repeat("b", 40), Brief: "docs/streams/x/brief-01-attested.md", PlanSHA256: planDigest("# Brief"), Model: "gpt-6-astra", Tier: "strong"}}
	// The stub answers as admission does for this receipt; the test proves each
	// landing shape hands admission the path it actually writes.
	stub := func(_, _, target string) (deskkit.VerifierReceipt, error) {
		return receipt, receipt.CheckEvidenceTarget(target)
	}
	unbound := "docs/streams/y/brief-02-never-attested.md"
	for name, args := range map[string][]string{
		"evidence-file": {"--evidence-file", unbound},
		"brief-path":    {"--evidence-file", "docs/streams/x/brief-01-attested.md", "--brief-path", unbound},
	} {
		t.Run(name, func(t *testing.T) {
			f, errBuf := setupFake(t)
			verifierEvidenceAdmissionFn = stub
			root := rootWithFile(t, unbound, "## Evidence\n| 1 | fixture | 0 | ok |\n")
			if err := os.MkdirAll(filepath.Join(root, "docs/streams/x"), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, "docs/streams/x/brief-01-attested.md"), []byte("## Evidence\n| 1 | fixture | 0 | ok |\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			f.setFile(unbound, "# Never attested\n\n## Evidence\n")
			if code := run(append([]string{"example-org/tracker", "main"}, append(args, "--root", root)...)); code == 0 {
				t.Fatal("Evidence for an unattested brief landed")
			}
			if !strings.Contains(errBuf.String(), "not bound to the attested brief") {
				t.Fatalf("refused for another reason: %s", errBuf)
			}
			if len(f.writes) != 0 || len(f.changes) != 0 {
				t.Fatalf("unbound Evidence mutated forge: %+v %+v", f.writes, f.changes)
			}
		})
	}
	t.Run("outcome-record", func(t *testing.T) {
		f, errBuf := setupFake(t)
		verifierEvidenceAdmissionFn = stub
		rec := `{"brief":"y/02","ts":"2026-10-06T00:00:00Z","verdict":"verified","digest":"0123456789abcdef"}`
		root := rootWithFile(t, "record.json", rec)
		if code := run([]string{"example-org/tracker", "main", "--outcome-record", "record.json", "--root", root}); code == 0 {
			t.Fatal("outcome record for an unattested brief landed")
		}
		if !strings.Contains(errBuf.String(), "not bound to the attested brief") {
			t.Fatalf("refused for another reason: %s", errBuf)
		}
		if len(f.writes) != 0 || len(f.changes) != 0 {
			t.Fatalf("unbound outcome record mutated forge: %+v %+v", f.writes, f.changes)
		}
	})
	t.Run("bound-direct-commit", func(t *testing.T) {
		f, _ := setupFake(t)
		verifierEvidenceAdmissionFn = stub
		target := receipt.Binding.Brief
		root := rootWithFile(t, target, "# Brief\n\n## Evidence\n| 1 | fixture | 0 | ok |\n")
		f.setFile(target, "# Brief\n\n## Evidence\n")
		if code := run([]string{"example-org/tracker", "main", "--evidence-file", target, "--root", root}); code != 0 {
			t.Fatalf("bound Evidence refused: %d", code)
		}
		if len(f.writes) != 1 || !strings.Contains(f.writes[0].Message, receipt.EvidenceBinding()) {
			t.Fatalf("direct commit lost the run binding: %+v", f.writes)
		}
	})
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

// planDigest is the attested plan digest of a brief whose text outside
// ## Evidence is plan.
func planDigest(plan string) string {
	h := sha256.Sum256([]byte(plan))
	return hex.EncodeToString(h[:])
}
