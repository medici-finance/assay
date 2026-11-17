package main

import (
	"path/filepath"
	"strings"
	"testing"
)

// supersededheld_test.go: a superseded HELD or could-not-check line, then a
// later full PASS whose rows are recorded twice (the witness table and a hand
// Expected/Observed restatement with a model-named Runner cell). The two
// Evidence shapes are the ones the issue names: a restatement row carrying a
// note after its clean outcome, and a one-line RISK-VALUE bullet whose prose
// holds a "|". verifyflip, `brief --check-verified` and the model autoflip
// must all read such a run the same way (#6767).

const (
	shPrincipal = " (on-behalf-of human:pat)"
	shToolQuals = shPrincipal + " (forge-identity)"
	shHandQuals = " (model-x verifier)" + shPrincipal
	shHandHead  = "| # | Command | Expected | Observed | Date | Runner |\n" +
		"|---|---------|----------|----------|------|--------|\n"
)

// shHandRow is one hand restatement row for Verify row n.
func shHandRow(n, observed, date, quals string) string {
	return "| " + n + " | row " + n + " as in Verify | exit 0 | " + observed + " | " + date +
		" | " + vfRunner + " @ " + vfSHA + quals + " |"
}

// shQual re-qualifies a witness row's Runner cell.
func shQual(row, quals string) string {
	return strings.Replace(row, vfRunner+" @ "+vfSHA+" |", vfRunner+" @ "+vfSHA+quals+" |", 1)
}

// shEvidence is an earlier BLOCKED entry (row 1 could-not-check) and a later
// PASS entry recorded twice. handQuals qualifies the hand rows; extra goes
// between the hand table and the PASS verdict.
func shEvidence(handQuals, note, extra string) string {
	held := "### Verification — 2026-08-12\n\n" +
		witnessTableFor(shQual(strings.Replace(vfRow1, "pass exit=0", "could-not-run exit=- timed out", 1), shToolQuals),
			shQual(strings.ReplaceAll(vfRow2, "2026-08-13", "2026-08-12"), shToolQuals)) + "\n\n" +
		shHandHead +
		shHandRow("1", "COULD-NOT-CHECK (environment): timed out", "2026-08-12", handQuals) + "\n" +
		shHandRow("2", "pass exit=1", "2026-08-12", handQuals) + "\n\n" +
		"**VERIFY: BLOCKED** — row 1 COULD-NOT-CHECK (environment); row 2 passes.\n\n"
	latest := "### Verification — 2026-08-13\n\n" +
		witnessTableFor(shQual(vfRow1, shToolQuals), shQual(vfRow2, shToolQuals)) + "\n\n" +
		shHandHead +
		shHandRow("1", "pass exit=0 (sha256:6f1a0b3c9d22)"+note, "2026-08-13", handQuals) + "\n" +
		shHandRow("2", "pass exit=1", "2026-08-13", handQuals) + "\n\n" +
		extra + "**VERIFY: PASS** — rows 1-2 pass."
	return held + latest
}

// The two shapes, as the issue's two briefs record them.
var (
	// A restatement row with a note after its clean outcome.
	shNoteShape = shEvidence(shHandQuals, "; the condition of #12 is unchanged", "")
	// A one-line RISK-VALUE bullet whose prose holds a "|".
	shItemShape = shEvidence(shHandQuals, "",
		"- RISK-VALUE: DERIVED — forbidden name runes = `<>:\"|?*` @ x.go:1 — the platform's set.\n\n")
)

func shOpts(evidence string) vfOpts {
	o := vfDefaults()
	o.evidence = evidence
	return o
}

// shCheckVerified runs `brief --check-verified` on a fixture already closed
// as verified with stamp, and returns its exit code and stderr.
func shCheckVerified(t *testing.T, evidence, stamp string) (int, string) {
	t.Helper()
	o := shOpts(evidence)
	o.status, o.verified = "verified", stamp
	root, _ := vfFixture(t, o)
	code, _, diag := runBriefInfoCapture("--root", root, "--check-verified", "vf/01")
	return code, diag
}

// regression: #6767
func TestShFlipNoteShape(t *testing.T) {
	root, readme := vfFixture(t, shOpts(shNoteShape))
	vfExpect(t, root, readme, verifyflipExitOK, "")
}

// regression: #6767
func TestShFlipItemShape(t *testing.T) {
	root, readme := vfFixture(t, shOpts(shItemShape))
	vfExpect(t, root, readme, verifyflipExitOK, "")
}

// regression: #6767
func TestShCheckVerifiedAgrees(t *testing.T) {
	// One principal across both tables: verifyflip flips, and check-verified
	// admits the stamp it wrote.
	root, readme := vfFixture(t, shOpts(shNoteShape))
	code, out, errOut := runVF(t, root)
	if code != verifyflipExitOK {
		t.Fatalf("verifyflip exit = %d, out=%q err=%q", code, out, errOut)
	}
	_ = readme
	if code, diag := shCheckVerified(t, shNoteShape, vfStamp); code != briefInfoExitOK {
		t.Fatalf("check-verified exit = %d on the run verifyflip flips: %s", code, diag)
	}
	// Another principal on the hand rows: both refuse, on one reading.
	other := shEvidence(" (model-x verifier) (on-behalf-of human:kim)", "", "")
	vfRefuses(t, shOpts(other), "runner mismatch")
	if code, diag := shCheckVerified(t, other, vfStamp); code == briefInfoExitOK || !strings.Contains(diag, "runner mismatch") {
		t.Fatalf("check-verified exit = %d (%s); verifyflip refuses this run, so it must too", code, diag)
	}
}

// regression: #6767
func TestShRunnerCellsAgree(t *testing.T) {
	tool := vfRunner + " @ " + vfSHA + shToolQuals
	hand := vfRunner + " @ " + vfSHA + shHandQuals
	if _, _, tail, err := runnerCellsAgree([]string{tool, hand}); err != nil || tail != shPrincipal {
		t.Fatalf("tool + hand forms: tail=%q err=%v, want %q and no error", tail, err, shPrincipal)
	}
	for _, bad := range []string{
		vfRunner + " @ " + vfSHA + " (model-x verifier) (on-behalf-of human:kim)",    // another principal
		vfRunner + " @ " + vfSHA + " (model-x verifier)" + shPrincipal + " (ci-env)", // another source
		vfRunner + " @ " + vfSHA + shPrincipal + " (model-x verifier) by human:pat",  // not a clean list
		"other-app[bot] @ " + vfSHA + shHandQuals,                                    // another login
		vfRunner + " @ " + strings.ToUpper(vfSHA) + shHandQuals,                      // another sha text
		vfRunner + " @ " + vfSHA + " (model-x verifier) (tree: dirty)" + shPrincipal, // an extra identity qualifier
	} {
		if _, _, _, err := runnerCellsAgree([]string{tool, bad}); err == nil {
			t.Errorf("runnerCellsAgree admitted %q beside %q", bad, tool)
		}
	}
}

// shAutoflip decides af/01's model flip on evidence (af/01's Verify is the one
// row `go vet ./...`).
func shAutoflip(t *testing.T, evidence string) modelFlipResult {
	t.Helper()
	root, streams := loadAFStreams(t)
	s := streams[0]
	path := filepath.Join(s.Dir, "brief-01-model-approved-at-head.md")
	return decideModelFlip(root, s, path, "af/01", evidence, briefFilesOf(&BriefFile{}), afSource(), ghReviewer(afReviewer))
}

// shAFEvidence is the af/01 form of the superseded-hold shape: a BLOCKED
// entry, then a PASS whose row 1 is recorded by the tool and restated by hand
// with note. latest is the latest run's tool witness row.
func shAFEvidence(latest, note string) string {
	held := " | 2026-07-07 | " + vfRunner + " @ " + vfSHA
	cell := " | 2026-07-08 | " + vfRunner + " @ " + vfSHA
	return "### Verification — 2026-07-07\n\n" +
		"| # | Command | Result | Output | Date | Runner |\n|---|---------|--------|--------|------|--------|\n" +
		"| 1 | `go vet ./...` | could-not-run exit=- timed out | sha256:abc123def456" + held + shToolQuals + " |\n\n" +
		"**VERIFY: BLOCKED** — row 1 COULD-NOT-CHECK (environment).\n\n" +
		"### Verification — 2026-07-08\n\n" +
		"| # | Command | Result | Output | Date | Runner |\n|---|---------|--------|--------|------|--------|\n" +
		latest + cell + shToolQuals + " |\n\n" + shHandHead +
		"| 1 | row 1 as in Verify | exit 0 | pass exit=0 (sha256:abc123def456)" + note + cell + shHandQuals + " |\n\n" +
		"**VERIFY: PASS** — row 1 passes."
}

const shAFPass = "| 1 | `go vet ./...` | pass exit=0 | sha256:abc123def456"

// regression: #6767
func TestShAutoflipSuperseded(t *testing.T) {
	got := shAutoflip(t, shAFEvidence(shAFPass, "; the condition of #12 is unchanged"))
	if got.Outcome != flipDone {
		t.Fatalf("a hold superseded by a witnessed full PASS stopped the autoflip: %v (%s)", got.Outcome, got.Reason)
	}
}

// regression: #6767
func TestShAutoflipStillHeld(t *testing.T) {
	for name, ev := range map[string]string{
		// The latest run's tool row failed: no passing witness, the hold stands.
		"no witness": shAFEvidence("| 1 | `go vet ./...` | fail exit=1 | sha256:abc123def456", ""),
		// A restatement note that reads as a failure is not corroboration.
		"failing note": shAFEvidence(shAFPass, "; but the cache was missing"),
	} {
		got := shAutoflip(t, ev)
		if !strings.Contains(got.Reason, "not marked deferred") {
			t.Errorf("%s: want the held refusal, got %v (%s)", name, got.Outcome, got.Reason)
		}
	}
}

// regression: #6767
func TestShItemBesideTableHeld(t *testing.T) {
	// A bullet beside a "|---|" bullet is not a lone one-line item: the scan
	// still cannot key it, so the latest entry is not aligned.
	ev := shEvidence(shHandQuals, "", "- a | b\n- |---|\n\n")
	if held, _ := unroutedHeldLine(ev, true); !held {
		t.Fatal("a bullet next to a delimiter-shaped bullet was read as prose")
	}
}
