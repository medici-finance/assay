package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

var vgSource = strings.Repeat("a", 40)

// ooRecord is a complete vg/01 outcome record; withheld "" omits the reason key.
func ooRecord(t *testing.T, outcome, sha, withheld string) string {
	t.Helper()
	extra := ""
	if withheld != "" {
		extra = fmt.Sprintf(`,"evidence_withheld":%q`, withheld)
	}
	return completeFail(t, fmt.Sprintf(`{"ts":"2026-11-01T00:00:00Z","brief":"vg/01","outcome":%q,"sha":%q,"blocker_kind":"human-action","blocker_ref":"#7"%s}`,
		outcome, sha, extra))
}

// ooBoard is a verifier home whose board reads vg/01 at status, admitted for
// vgBrief at vgSource. The Evidence binding is wired to fail loudly: an
// outcome-only record must never reach it.
func ooBoard(t *testing.T, status string) (root string, evidenceBound *int) {
	t.Helper()
	root = rootWithFile(t, vgBrief, vgBriefText(vgPriorRun))
	readme := "---\nstream: vg\nstatus: active\npriority: P1\ntrack: platform\n---\n\n# VG\n\n## Briefs\n\n" +
		"| # | Brief | Wave | Effort | Status | Verified | Reviewed |\n" +
		"|---|-------|------|--------|--------|----------|----------|\n" +
		"| 01 | [t](./brief-01-gate.md) | 0 | S | " + status + " | 2026-08-13 human:alex | 2026-08-13 human:alex |\n"
	if err := os.WriteFile(filepath.Join(root, "docs", "streams", "vg", "README.md"), []byte(readme), 0o644); err != nil {
		t.Fatal(err)
	}
	verifierEvidenceAdmissionFn = func(string, string, string, deskkit.Forge) (deskkit.VerifierReceipt, error) {
		return deskkit.VerifierReceipt{Binding: deskkit.VerifierBinding{Brief: vgBrief, Source: vgSource}}, nil
	}
	n := 0
	outcomeEvidenceBindingFn = func(string, deskkit.Forge, deskkit.ForgeRepo, string, deskkit.VerifierReceipt, string) error {
		n++
		return deskkit.Refused("refused: outcome record does not name the Evidence as landed on its target branch")
	}
	return root, &n
}

func ooRun(t *testing.T, root, record string, outcomeOnly bool) int {
	t.Helper()
	file := filepath.Join(t.TempDir(), "record.json")
	if err := os.WriteFile(file, []byte(record), 0o644); err != nil {
		t.Fatal(err)
	}
	args := []string{fixtureRepo, "main", "--root", root, "--outcome-record", file}
	if outcomeOnly {
		args = append(args, "--outcome-only")
	}
	return run(args)
}

// regression: #5759
func TestOutcomeOnlyVerifiedLands(t *testing.T) {
	f, errBuf := setupFake(t)
	realVerdictCheck(t)
	root, bound := ooBoard(t, "verified")
	for _, outcome := range []string{"blocked", "needs_context"} {
		code := ooRun(t, root, ooRecord(t, outcome, vgSource, evidenceWithheldReason), true)
		if code != deskkit.ExitOK {
			t.Fatalf("%s outcome-only on a verified brief: exit = %d, want it to land; stderr %q", outcome, code, errBuf.String())
		}
	}
	if len(f.writes) != 2 || *bound != 0 {
		t.Fatalf("writes = %d, evidence-binding calls = %d; want 2 and 0", len(f.writes), *bound)
	}
	if !strings.Contains(string(f.writes[0].Content), `"evidence_withheld":"`+evidenceWithheldReason+`"`) {
		t.Fatalf("landed record does not state its reason: %s", f.writes[0].Content)
	}
}

// regression: #5759
func TestOutcomeOnlyImplRefused(t *testing.T) {
	f, errBuf := setupFake(t)
	realVerdictCheck(t)
	root, _ := ooBoard(t, "implemented")
	code := ooRun(t, root, ooRecord(t, "blocked", vgSource, evidenceWithheldReason), true)
	if code != deskkit.ExitRefused || len(f.writes) != 0 {
		t.Fatalf("outcome-only on an implemented brief: exit = %d writes = %d, want refused; stderr %q", code, len(f.writes), errBuf.String())
	}
	if !strings.Contains(errBuf.String(), "land the Evidence first") {
		t.Fatalf("refusal does not say why: %q", errBuf.String())
	}
}

// regression: #5759
func TestOutcomeOnlyShapeRefused(t *testing.T) {
	for name, c := range map[string]struct {
		record func(t *testing.T) string
		want   string
	}{
		"no-reason":    {func(t *testing.T) string { return ooRecord(t, "blocked", vgSource, "") }, evidenceWithheldKey},
		"wrong-reason": {func(t *testing.T) string { return ooRecord(t, "blocked", vgSource, "other") }, evidenceWithheldKey},
		"verify-fail":  {func(t *testing.T) string { return ooRecord(t, "verify-fail", vgSource, evidenceWithheldReason) }, "decisive"},
		"evidence-sha": {func(t *testing.T) string {
			return ooRecord(t, "blocked", strings.Repeat("b", 40), evidenceWithheldReason)
		}, "admitted observation commit"},
	} {
		t.Run(name, func(t *testing.T) {
			f, errBuf := setupFake(t)
			root, bound := ooBoard(t, "verified")
			asked := 0
			evidenceVerdictCheckFn = func(string, string, []byte, []byte, bool, string) (closureVerdict, string, error) {
				asked++
				return closureNotAccepted, "", nil
			}
			code := ooRun(t, root, c.record(t), true)
			if code != deskkit.ExitRefused || len(f.writes) != 0 || *bound != 0 {
				t.Fatalf("exit = %d writes = %d binding = %d, want refused before any write; stderr %q", code, len(f.writes), *bound, errBuf.String())
			}
			if !strings.Contains(errBuf.String(), c.want) {
				t.Fatalf("refusal %q does not name %q", errBuf.String(), c.want)
			}
		})
	}
}

// regression: #5759
func TestWithheldNeedsOutcomeOnly(t *testing.T) {
	f, errBuf := setupFake(t)
	root, bound := ooBoard(t, "verified")
	code := ooRun(t, root, ooRecord(t, "blocked", vgSource, evidenceWithheldReason), false)
	if code != deskkit.ExitRefused || len(f.writes) != 0 || *bound != 0 {
		t.Fatalf("withheld record without --outcome-only: exit = %d writes = %d binding = %d, want refused; stderr %q", code, len(f.writes), *bound, errBuf.String())
	}
	if !strings.Contains(errBuf.String(), "--outcome-only") {
		t.Fatalf("refusal does not name the mode: %q", errBuf.String())
	}
}

// regression: #5759
func TestOrdinaryRecordStillBound(t *testing.T) {
	f, _ := setupFake(t)
	root, bound := ooBoard(t, "verified")
	code := ooRun(t, root, ooRecord(t, "blocked", vgSource, ""), false)
	if code != deskkit.ExitRefused || len(f.writes) != 0 || *bound != 1 {
		t.Fatalf("ordinary record: exit = %d writes = %d binding = %d, want the Evidence binding to decide (refused)", code, len(f.writes), *bound)
	}
}

func TestOutcomeOnlyNeedsRecord(t *testing.T) {
	_, errBuf := setupFake(t)
	root := rootWithFile(t, vgBrief, vgBriefText(vgPriorRun))
	code := run([]string{fixtureRepo, "main", "--root", root, "--evidence-file", vgBrief, "--outcome-only"})
	if code != deskkit.ExitRefused || !strings.Contains(errBuf.String(), "only with --outcome-record") {
		t.Fatalf("--outcome-only without --outcome-record: exit = %d stderr %q", code, errBuf.String())
	}
}
