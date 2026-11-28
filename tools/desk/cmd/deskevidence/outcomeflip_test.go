package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// regression: #7659
func TestOutcomeRecordWithVerifiedFlip(t *testing.T) {
	landedStatusgenOnPath(t)
	for _, tc := range []struct {
		name, refusal string
		dryRun        bool
	}{
		{name: "record and row in one write"},
		{name: "dry run", dryRun: true},
		{name: "wrong observation", refusal: "candidate outcome record source_sha"},
		{name: "different attestation", refusal: "latest PASS must name exactly the admitted"},
		{name: "newer blocked record", refusal: "candidate outcome record is not the latest"},
		{name: "human gate", refusal: "gate is human"},
		{name: "head moved", refusal: "head moved"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f, errOut := setupFake(t)
			for _, line := range strings.Split(fixtureRoster, "\n") {
				if key, value, ok := strings.Cut(line, "="); ok && strings.HasPrefix(key, "ASSAY_") {
					t.Setenv(key, value)
				}
			}
			root, source, head, files := landedFixture(t, false)
			const brief = "docs/streams/vf/brief-01-check.md"
			const readme = "docs/streams/vf/README.md"
			receipt := deskkit.VerifierReceipt{
				Issue: 41,
				Binding: deskkit.VerifierBinding{
					Brief: brief, Repo: fixtureRepo, Source: source, Run: strings.Repeat("a", 64),
					Model: "fixture-model", Tier: "strong",
				},
			}
			verifierEvidenceAdmissionFn = func(string, string, string, deskkit.Forge) (deskkit.VerifierReceipt, error) {
				return receipt, nil
			}
			landedClosureFn = landedClosureTree
			verifiedClosureCheckFn = verifiedClosureCheckAt
			outcomeEvidenceBindingFn = productionOutcomeEvidenceBinding
			files[readme] = strings.Replace(files[readme], "| verified | 2026-09-22 assay-verifier-app[bot] @ "+source[:12]+" |", "| implemented | — |", 1)
			binding := receipt.EvidenceBinding()
			if tc.name == "different attestation" {
				binding = strings.Replace(binding, "#41", "#42", 1)
			}
			files[brief] = strings.Replace(files[brief], "**VERIFY: PASS**", binding+"\n\n**VERIFY: PASS**", 1)
			if tc.name == "different attestation" {
				// The admitted marker exists in Evidence, outside the selected PASS.
				files[brief] = strings.Replace(files[brief], "\n\n## Review", "\n\n"+receipt.EvidenceBinding()+"\n\n## Review", 1)
			}
			if tc.name == "human gate" {
				files[brief] = strings.Replace(files[brief], "gate: model", "gate: human", 1)
			}
			plan, _, _ := strings.Cut(files[brief], "\n## Evidence\n")
			_, review, _ := strings.Cut(files[brief], "\n## Review\n")
			receipt.Binding.PlanSHA256 = planDigest(strings.TrimSpace(plan + "\n## Review\n" + review))
			flGit(t, root, "checkout", "-q", "-f", "--detach", head)
			for rel, content := range files {
				if err := os.WriteFile(filepath.Join(root, rel), []byte(content), 0o644); err != nil {
					t.Fatal(err)
				}
				f.setFile(rel, content)
			}
			flGit(t, root, "add", "--", readme, brief)
			if tc.name == "newer blocked record" {
				raw := []byte(fmt.Sprintf(`{"brief":"vf/01","ts":"2026-09-22T03:01:00Z","outcome":"blocked","sha":%q,"source_sha":%q}`, head, source))
				name, err := deskkit.RecordName(raw)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.MkdirAll(filepath.Dir(filepath.Join(root, name)), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(root, name), deskkit.CanonicalBytes(raw), 0o644); err != nil {
					t.Fatal(err)
				}
				flGit(t, root, "add", "--", name)
			}
			flGit(t, root, "commit", "-q", "-m", "land PASS Evidence without a flip")
			parent := flGit(t, root, "rev-parse", "HEAD")
			f.commitSHAs = map[string]string{"main": parent}
			f.commitAuthorLogin = "assay-verifier-app[bot]"
			if tc.name == "head moved" {
				f.writeErr = deskkit.Refused("head moved")
			}
			observed := source
			if tc.name == "wrong observation" {
				observed = strings.Repeat("b", 40)
			}
			raw := fmt.Sprintf(`{"brief":"vf/01","ts":"2026-09-22T03:00:00Z","outcome":"verified","sha":%q,"source_sha":%q}`, parent, observed)
			target, err := deskkit.RecordName([]byte(raw))
			if err != nil {
				t.Fatal(err)
			}
			args := []string{fixtureRepo, "main", "--root", root, "--outcome-record", writeRepoFile(t, "record.json", raw+"\n"), "--flip-verified"}
			if tc.dryRun {
				args = append(args, "--dry-run")
			}
			code := run(args)
			if tc.refusal != "" {
				if code != deskkit.ExitRefused || !strings.Contains(errOut.String(), tc.refusal) || len(f.writes) != 0 {
					t.Fatalf("exit=%d writes=%v stderr=%s; want refusal %q and no write", code, f.writes, errOut, tc.refusal)
				}
			} else {
				if code != deskkit.ExitOK {
					t.Fatalf("exit=%d stderr=%s stdout=%s", code, errOut, stdout)
				}
				if tc.dryRun {
					if len(f.writes) != 0 || !strings.Contains(stdout.(*bytes.Buffer).String(), "atomic verified flip and outcome record") {
						t.Fatalf("wrong dry run: writes=%v stdout=%s", f.writes, stdout)
					}
				} else {
					if len(f.writes) != 1 {
						t.Fatalf("want one conditional write, got %v", f.writes)
					}
					in := f.writes[0]
					if in.File != readme || in.ExpectedHead != parent || in.ExpectedSHA == "" ||
						!bytes.Contains(in.Content, []byte("| verified |")) || len(in.Companions) != 1 ||
						in.Companions[0].File != target || string(in.Companions[0].Content) != raw+"\n" {
						t.Fatalf("wrong conditional pair: %+v", in)
					}
					// Represent the forge's committed pair in the fixture's local
					// object store, then retry as if the write receipt was lost.
					writeBoard := []boardEdit{{Path: readme, Content: in.Content}, {Path: target, Content: in.Companions[0].Content}}
					if _, err := stageBoardEdits(root, writeBoard); err != nil {
						t.Fatal(err)
					}
					flGit(t, root, "add", "--", readme, target)
					flGit(t, root, "commit", "-q", "-m", "forge commits the pair")
					f.commitSHAs["main"] = flGit(t, root, "rev-parse", "HEAD")
					if code := run(args); code != deskkit.ExitOK || len(f.writes) != 1 {
						t.Fatalf("retry exit=%d writes=%v stderr=%s", code, f.writes, errOut)
					}
				}
			}
			if got := flGit(t, root, "status", "--porcelain"); got != "" {
				t.Fatalf("landing changed the verifier checkout: %s", got)
			}
		})
	}
}
