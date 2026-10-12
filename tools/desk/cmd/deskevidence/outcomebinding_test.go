package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

var productionOutcomeEvidenceBinding = outcomeEvidenceBindingFn

func TestOutcomeCannotLandWithoutCurrentEvidence(t *testing.T) {
	f, errBuf := setupFake(t)
	outcomeEvidenceBindingFn = productionOutcomeEvidenceBinding
	root, source := receiptGitRoot(t)
	receipt := deskkit.VerifierReceipt{Issue: 77, Binding: deskkit.VerifierBinding{
		Repo: "example-org/tracker", Brief: receiptBriefPath, Source: source,
		Run: strings.Repeat("a", 64), Model: "gpt-6-astra", Tier: "strong",
	}}
	baseForPlan, err := os.ReadFile(filepath.Join(root, receiptBriefPath))
	if err != nil {
		t.Fatal(err)
	}
	receipt.Binding.PlanSHA256 = planDigest(strings.TrimSpace(strings.SplitN(string(baseForPlan), "\n## Evidence\n", 2)[0]))
	verifierEvidenceAdmissionFn = func(string, string, string, deskkit.Forge) (deskkit.VerifierReceipt, error) {
		return receipt, nil
	}
	// Keep receipt derivation out of this test: both hand-supplied and derived
	// records must pass the independent Evidence/run gate before any write.
	oldDerive := outcomeReceiptFn
	outcomeReceiptFn = func(raw []byte, _ string, _ deskkit.Forge, _ deskkit.ForgeRepo, _ string) ([]byte, error) {
		return raw, nil
	}
	t.Cleanup(func() { outcomeReceiptFn = oldDerive })
	base, err := os.ReadFile(filepath.Join(root, receiptBriefPath))
	if err != nil {
		t.Fatal(err)
	}
	f.setFile(receiptBriefPath, string(base))
	line := fmt.Sprintf(`{"ts":"2026-10-12T00:00:00Z","brief":"example-stream/14","outcome":"blocked","sha":%q,"blocker_kind":"human-action","blocker_ref":"#7"}`, source)
	file := writeRepoFile(t, "record.json", line)
	code := run([]string{"example-org/tracker", "evidence-branch", "--outcome-record", file, "--root", root})
	if code != deskkit.ExitRefused {
		t.Fatalf("orphan outcome exit=%d, want refusal; stderr=%s, writes=%d", code, errBuf.String(), len(f.writes))
	}
	if len(f.writes) != 0 || len(f.changes) != 0 {
		t.Fatal("an outcome with no current-run Evidence changed the forge")
	}
}

func TestOutcomeBindingRequiresCurrentLandedEvidence(t *testing.T) {
	for _, mode := range []string{"current", "previous-run", "previous-source", "code-example", "mixed-fence", "outside-evidence", "branch-mismatch", "old-sha", "unreadable-sha"} {
		t.Run(mode, func(t *testing.T) {
			f, errBuf := setupFake(t)
			outcomeEvidenceBindingFn = productionOutcomeEvidenceBinding
			root, source := receiptGitRoot(t)
			base, err := os.ReadFile(filepath.Join(root, receiptBriefPath))
			if err != nil {
				t.Fatal(err)
			}
			receipt := deskkit.VerifierReceipt{Issue: 77, Binding: deskkit.VerifierBinding{
				Repo: "example-org/tracker", Brief: receiptBriefPath, Source: source,
				Run: strings.Repeat("a", 64), Model: "gpt-6-astra", Tier: "strong",
				PlanSHA256: planDigest(strings.TrimSpace(strings.SplitN(string(base), "\n## Evidence\n", 2)[0])),
			}}
			verifierEvidenceAdmissionFn = func(string, string, string, deskkit.Forge) (deskkit.VerifierReceipt, error) { return receipt, nil }
			oldDerive := outcomeReceiptFn
			outcomeReceiptFn = func(raw []byte, _ string, _ deskkit.Forge, _ deskkit.ForgeRepo, _ string) ([]byte, error) {
				return raw, nil
			}
			t.Cleanup(func() { outcomeReceiptFn = oldDerive })
			marker := receipt.EvidenceBinding()
			switch mode {
			case "previous-run":
				old := receipt
				old.Binding.Run = strings.Repeat("d", 64)
				marker = old.EvidenceBinding()
			case "previous-source":
				old := receipt
				old.Binding.Source = strings.Repeat("d", 40)
				marker = old.EvidenceBinding()
			case "code-example":
				marker = "```text\n" + marker + "\n```"
			case "mixed-fence":
				marker = "~~~text\n```\n" + marker + "\n~~~"
			case "outside-evidence":
				marker = "## Notes\n" + marker
			}
			landed := string(base) + "\n| 1 | command | 0 | observed |\n" + marker + "\n"
			rvWriteAndAdd(t, root, receiptBriefPath, landed)
			rvGitIn(t, root, "commit", "-qm", "Evidence")
			sha := rvGitIn(t, root, "rev-parse", "HEAD")
			if mode == "branch-mismatch" {
				f.setFile(receiptBriefPath, string(base))
			} else {
				f.setFile(receiptBriefPath, landed)
			}
			if mode == "old-sha" {
				sha = source
			} else if mode == "unreadable-sha" {
				sha = strings.Repeat("0", 40)
			}
			line := fmt.Sprintf(`{"ts":"2026-10-12T00:00:00Z","brief":"example-stream/14","outcome":"blocked","sha":%q,"blocker_kind":"human-action","blocker_ref":"#7"}`, sha)
			file := writeRepoFile(t, "record.json", line)
			code := run([]string{"example-org/tracker", "evidence-branch", "--outcome-record", file, "--root", root})
			if mode == "current" {
				if code != 0 || len(f.writes) != 1 {
					t.Fatalf("backed record refused: exit=%d stderr=%s writes=%d", code, errBuf.String(), len(f.writes))
				}
			} else if code == 0 || len(f.writes) != 0 || len(f.changes) != 0 {
				t.Fatalf("unsupported record landed: exit=%d stderr=%s writes=%d", code, errBuf.String(), len(f.writes))
			}
		})
	}
}

func TestOutcomeBindingKeepsExistingIdenticalRecordIdempotent(t *testing.T) {
	f, errBuf := setupFake(t)
	outcomeEvidenceBindingFn = func(string, deskkit.Forge, deskkit.ForgeRepo, string, deskkit.VerifierReceipt, string) error {
		t.Fatal("an existing identical record was treated as a new receipt")
		return nil
	}
	oldDerive := outcomeReceiptFn
	outcomeReceiptFn = func(raw []byte, _ string, _ deskkit.Forge, _ deskkit.ForgeRepo, _ string) ([]byte, error) {
		return raw, nil
	}
	t.Cleanup(func() { outcomeReceiptFn = oldDerive })
	raw := []byte(`{"ts":"2026-10-12T00:00:00Z","brief":"example-stream/14","outcome":"blocked","sha":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","blocker_kind":"human-action","blocker_ref":"#7"}`)
	name, err := deskkit.RecordName(raw)
	if err != nil {
		t.Fatal(err)
	}
	f.setFile(name, string(deskkit.CanonicalBytes(raw)))
	file := writeRepoFile(t, "record.json", string(raw))
	if code := run([]string{"example-org/tracker", "evidence-branch", "--outcome-record", file}); code != 0 {
		t.Fatalf("existing identical record refused: exit=%d stderr=%s", code, errBuf.String())
	}
	if len(f.writes) != 0 || len(f.changes) != 0 {
		t.Fatal("idempotent record changed the forge")
	}
}
