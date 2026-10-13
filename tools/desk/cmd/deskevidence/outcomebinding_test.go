package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

var productionOutcomeEvidenceBinding = outcomeEvidenceBindingFn
var productionOutcomeObservationBinding = outcomeObservationBindingFn

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
	outcomeReceiptFn = func(raw []byte, _ string, _ deskkit.Forge, _ deskkit.ForgeRepo, _, _ string) ([]byte, error) {
		return raw, nil
	}
	t.Cleanup(func() { outcomeReceiptFn = oldDerive })
	base, err := os.ReadFile(filepath.Join(root, receiptBriefPath))
	if err != nil {
		t.Fatal(err)
	}
	f.setFile(receiptBriefPath, string(base))
	line := completeFail(t, fmt.Sprintf(`{"ts":"2026-10-12T00:00:00Z","brief":"example-stream/14","outcome":"blocked","sha":%q,"blocker_kind":"human-action","blocker_ref":"#7"}`, source))
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
			outcomeReceiptFn = func(raw []byte, _ string, _ deskkit.Forge, _ deskkit.ForgeRepo, _, _ string) ([]byte, error) {
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
			line := completeFail(t, fmt.Sprintf(`{"ts":"2026-10-12T00:00:00Z","brief":"example-stream/14","outcome":"blocked","sha":%q,"blocker_kind":"human-action","blocker_ref":"#7"}`, sha))
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
	outcomeReceiptFn = func(raw []byte, _ string, _ deskkit.Forge, _ deskkit.ForgeRepo, _, _ string) ([]byte, error) {
		return raw, nil
	}
	t.Cleanup(func() { outcomeReceiptFn = oldDerive })
	raw := []byte(completeFail(t, `{"ts":"2026-10-12T00:00:00Z","brief":"example-stream/14","outcome":"blocked","sha":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","blocker_kind":"human-action","blocker_ref":"#7"}`))
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

// A repair may land between the admitted observation and its Evidence append.
// The failure receipt must still describe the observed inputs so the repair wakes it.
func TestOutcomeObservationSurvivesRepairBeforeEvidence(t *testing.T) {
	f, errBuf := setupFake(t)
	outcomeObservationBindingFn = productionOutcomeObservationBinding
	outcomeEvidenceBindingFn = productionOutcomeEvidenceBinding
	outcomeReceiptFn = deriveOutcomeReceipt
	forgeForFn = func(owner, name string) (deskkit.Forge, deskkit.ForgeRepo, error) {
		return deskkit.OutboundChecked(issueForge{f}, "verifier"), deskkit.ForgeRepo{Owner: owner, Name: name}, nil
	}
	root, source := receiptGitRoot(t)
	base, err := os.ReadFile(filepath.Join(root, receiptBriefPath))
	if err != nil {
		t.Fatal(err)
	}
	receipt := deskkit.VerifierReceipt{Issue: 77, Binding: deskkit.VerifierBinding{
		Repo: "example-org/tracker", Brief: receiptBriefPath, Source: source, Run: strings.Repeat("a", 64), Model: "gpt-6-astra", Tier: "strong",
		PlanSHA256: planDigest(strings.TrimSpace(strings.SplitN(string(base), "\n## Evidence\n", 2)[0])),
	}}
	verifierEvidenceAdmissionFn = func(string, string, string, deskkit.Forge) (deskkit.VerifierReceipt, error) { return receipt, nil }
	observed, err := gitShowAt(root, source, "foo/bar.go")
	if err != nil {
		t.Fatal(err)
	}
	rvWriteAndAdd(t, root, "foo/bar.go", "package foo // repaired\n")
	rvGitIn(t, root, "commit", "-qm", "repair")
	landed := string(base) + "\n| 1 | command | 1 | failed at admitted source |\n" + receipt.EvidenceBinding() + "\n"
	rvWriteAndAdd(t, root, receiptBriefPath, landed)
	rvGitIn(t, root, "commit", "-qm", "Evidence")
	evidence := rvGitIn(t, root, "rev-parse", "HEAD")
	f.setFile(receiptBriefPath, landed)
	raw := withRunFields(t, fmt.Sprintf(`{"ts":"2026-10-12T00:00:00Z","brief":"example-stream/14","outcome":"verify-fail","rows_passed":0,"rows_total":1,"sha":%q,"blocker_kind":"implementation","blocker_ref":"#7"}`, evidence))
	file := writeRepoFile(t, "record.json", raw)
	if code := run([]string{"example-org/tracker", "main", "--outcome-record", file, "--root", root}); code != 0 || len(f.writes) != 1 {
		t.Fatalf("backed observation refused: code=%d writes=%d stderr=%s", code, len(f.writes), errBuf.String())
	}
	var wr deskkit.WakeReceipt
	if err := json.Unmarshal(f.writes[0].Content, &wr); err != nil {
		t.Fatal(err)
	}
	if wr.SHA != evidence {
		t.Fatal("record lost its Evidence commit binding")
	}
	if wr.Inputs["file:foo/bar.go"] != deskkit.Sha256Hex(observed) {
		t.Fatal("receipt substituted repaired inputs for the observed failure")
	}
	if wr.Inputs["file:"+receiptBriefPath] != deskkit.Sha256Hex([]byte(landed)) {
		t.Fatal("receipt did not retain the landed Evidence brief")
	}
	current := outcomeWakeInputs{}
	for k, v := range wr.Inputs {
		current[k] = v
	}
	repaired, err := gitShowAt(root, evidence, "foo/bar.go")
	if err != nil {
		t.Fatal(err)
	}
	current["file:foo/bar.go"] = deskkit.Sha256Hex(repaired)
	if state, _ := deskkit.Unchanged(wr, current); state != deskkit.InputsChanged {
		t.Fatal("repair was held as unchanged")
	}
	for _, admitted := range []bool{false, true} {
		f.writes, f.changes = nil, nil
		errBuf.Reset()
		observation := evidence
		stamp := "2026-10-12T00:01:00Z"
		if admitted {
			observation, stamp = source, "2026-10-12T00:02:00Z"
		}
		tree, err := newGitOutcomeTree(root, observation)
		if err != nil {
			t.Fatal(err)
		}
		plain := strings.ReplaceAll(raw, "2026-10-12T00:00:00Z", stamp)
		supplied, err := deskkit.BuildOutcomeRecord([]byte(plain), deskkit.OutcomeReceiptInput{
			BriefPath: receiptBriefPath, LandedBrief: []byte(landed), ToolVersion: deskkit.ReleaseTagOrDev(), Repo: "example-org/tracker",
		}, tree)
		if err != nil {
			t.Fatal(err)
		}
		file := writeRepoFile(t, "supplied.json", string(supplied))
		code := run([]string{"example-org/tracker", "main", "--outcome-record", file, "--root", root})
		if admitted {
			if code != 0 || len(f.writes) != 1 {
				t.Fatalf("admitted supplied receipt refused: code=%d writes=%d stderr=%s", code, len(f.writes), errBuf.String())
			}
		} else if code != deskkit.ExitRefused || len(f.writes) != 0 || len(f.changes) != 0 {
			t.Fatalf("supplied repaired-input receipt landed: code=%d writes=%d stderr=%s", code, len(f.writes), errBuf.String())
		}
	}
}

// Membership and path-kind changes need a fresh observation: absent or removed
// files cannot be represented as observed content hashes in the existing receipt.
func TestOutcomeInputShapeChangeRequiresFreshVerification(t *testing.T) {
	for _, mode := range []string{"planned-addition", "directory-addition", "file-removal", "directory-removal", "kind-change"} {
		t.Run(mode, func(t *testing.T) {
			f, errBuf := setupFake(t)
			outcomeEvidenceBindingFn = productionOutcomeEvidenceBinding
			outcomeObservationBindingFn = productionOutcomeObservationBinding
			outcomeReceiptFn = deriveOutcomeReceipt
			forgeForFn = func(owner, name string) (deskkit.Forge, deskkit.ForgeRepo, error) {
				return deskkit.OutboundChecked(issueForge{f}, "verifier"), deskkit.ForgeRepo{Owner: owner, Name: name}, nil
			}
			root, source := receiptGitRoot(t)
			base, err := os.ReadFile(filepath.Join(root, receiptBriefPath))
			if err != nil {
				t.Fatal(err)
			}
			receipt := deskkit.VerifierReceipt{Issue: 77, Binding: deskkit.VerifierBinding{Repo: "example-org/tracker", Brief: receiptBriefPath, Source: source, Run: strings.Repeat("a", 64), Model: "gpt-6-astra", Tier: "strong", PlanSHA256: planDigest(strings.TrimSpace(strings.SplitN(string(base), "\n## Evidence\n", 2)[0]))}}
			verifierEvidenceAdmissionFn = func(string, string, string, deskkit.Forge) (deskkit.VerifierReceipt, error) { return receipt, nil }
			switch mode {
			case "planned-addition":
				rvWriteAndAdd(t, root, "foo/planned.go", "package foo\n")
			case "directory-addition":
				rvWriteAndAdd(t, root, "foo/pkg/new.go", "package pkg\n")
			case "file-removal":
				rvGitIn(t, root, "rm", "foo/bar.go")
			case "directory-removal":
				rvGitIn(t, root, "rm", "foo/pkg/a.go")
			case "kind-change":
				rvGitIn(t, root, "rm", "foo/bar.go")
				rvWriteAndAdd(t, root, "foo/bar.go/new.go", "package foo\n")
			}
			rvGitIn(t, root, "commit", "-qm", "repair changes declared inputs")
			landed := string(base) + "\n| 1 | command | 1 | observed failure |\n" + receipt.EvidenceBinding() + "\n"
			rvWriteAndAdd(t, root, receiptBriefPath, landed)
			rvGitIn(t, root, "commit", "-qm", "Evidence")
			evidence := rvGitIn(t, root, "rev-parse", "HEAD")
			f.setFile(receiptBriefPath, landed)
			raw := withRunFields(t, fmt.Sprintf(`{"ts":"2026-10-12T00:00:00Z","brief":"example-stream/14","outcome":"verify-fail","rows_passed":0,"rows_total":1,"sha":%q,"blocker_kind":"implementation","blocker_ref":"#7"}`, evidence))
			file := writeRepoFile(t, "record.json", raw)
			code := run([]string{"example-org/tracker", "main", "--outcome-record", file, "--root", root})
			if code != deskkit.ExitRefused || len(f.writes) != 0 || len(f.changes) != 0 || !strings.Contains(errBuf.String(), "fresh independent verification") {
				t.Fatalf("shape change did not require new admission: code=%d writes=%d stderr=%s", code, len(f.writes), errBuf.String())
			}
			// Model a new independent admission at the repaired tree and its own
			// Evidence landing. The new observation can publish a backed record.
			fresh := receipt
			fresh.Binding.Source, fresh.Binding.Run = evidence, strings.Repeat("b", 64)
			verifierEvidenceAdmissionFn = func(string, string, string, deskkit.Forge) (deskkit.VerifierReceipt, error) { return fresh, nil }
			newLanded := landed + "\n" + fresh.EvidenceBinding() + "\n"
			rvWriteAndAdd(t, root, receiptBriefPath, newLanded)
			rvGitIn(t, root, "commit", "-qm", "fresh run Evidence")
			newSHA := rvGitIn(t, root, "rev-parse", "HEAD")
			f.setFile(receiptBriefPath, newLanded)
			newRaw := strings.ReplaceAll(strings.ReplaceAll(raw, evidence, newSHA), "2026-10-12T00:00:00Z", "2026-10-12T00:01:00Z")
			newFile := writeRepoFile(t, "fresh-record.json", newRaw)
			errBuf.Reset()
			if code := run([]string{"example-org/tracker", "main", "--outcome-record", newFile, "--root", root}); code != 0 || len(f.writes) != 1 {
				t.Fatalf("fresh observation refused: code=%d writes=%d stderr=%s", code, len(f.writes), errBuf.String())
			}
		})
	}
}

type outcomeWakeInputs map[string]string

func (m outcomeWakeInputs) Revision(key string) (string, bool)  { v, ok := m[key]; return v, ok }
func (m outcomeWakeInputs) ActionCompleted(string) (bool, bool) { return false, false }
