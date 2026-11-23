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
	// A one-line RISK-VALUE bullet whose prose holds a bare "|", outside any
	// code span (a pipe inside one never reaches the bullet read).
	shItemShape = shEvidence(shHandQuals, "",
		"- RISK-VALUE: DERIVED — the name check also rejects <>:\"|?* @ x.go:1 — the platform's set.\n\n")
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
	// The latest run's tool row failed: no passing witness, the hold stands
	// (the autoflip takes rule 3d's drop only behind the witness check).
	if got := shAutoflip(t, shAFEvidence("| 1 | `go vet ./...` | fail exit=1 | sha256:abc123def456", "")); !strings.Contains(got.Reason, "not marked deferred") {
		t.Errorf("no witness: want the held refusal, got %v (%s)", got.Outcome, got.Reason)
	}
	// A restatement note that reads as a failure is not corroboration for rule
	// 3c, so with rule 3d off the hold stands. With 3d on, the witnessed
	// latest strict PASS entry supersedes the BLOCKED entry and is read at
	// full strength: the note carries no hold, so the autoflip proceeds.
	failing := shAFEvidence(shAFPass, "; but the cache was missing")
	if got := shAutoflip(t, failing); got.Outcome != flipDone {
		t.Errorf("failing note, rule 3d: want the flip, got %v (%s)", got.Outcome, got.Reason)
	}
	defer func(on bool) { heldPassRule = on }(heldPassRule)
	heldPassRule = false
	if got := shAutoflip(t, failing); !strings.Contains(got.Reason, "not marked deferred") {
		t.Errorf("failing note, rule 3d off: want the held refusal, got %v (%s)", got.Outcome, got.Reason)
	}
}

// regression: #6767
func TestShItemBesideTableHeld(t *testing.T) {
	// A bullet beside a "|---|" bullet is not a lone one-line item: the scan
	// still cannot key it, so the latest entry is not aligned (rule 3c; rule
	// 3d, which supersedes the earlier entry whole, is off here).
	defer func(on bool) { heldPassRule = on }(heldPassRule)
	heldPassRule = false
	ev := shEvidence(shHandQuals, "", "- a | b\n- |---|\n\n")
	if held, _ := unroutedHeldLine(ev, true); !held {
		t.Fatal("a bullet next to a delimiter-shaped bullet was read as prose")
	}
}

// shVerdictFirst is an earlier entry whose FAIL verdict sits above its own
// rows (runner is its rows' Runner cell), then a latest PASS entry whose two
// rows carry q1 and q2. verifyflip reads the span from that verdict first and,
// when it refuses, again from the latest entry heading (latestPassRunFrom).
func shVerdictFirst(runner, q1, q2 string) string {
	early := func(row string) string {
		row = strings.ReplaceAll(row, "2026-08-13", "2026-08-12")
		return strings.Replace(row, vfRunner+" @ "+vfSHA+" |", runner+" |", 1)
	}
	return "### Verification — 2026-08-12\n\n**VERIFY: FAIL** — row 2 failed.\n\n" +
		witnessTableFor(early(vfRow1), early(vfRow2)) + "\n\n" +
		"### Verification — 2026-08-13\n\n" +
		witnessTableFor(shQual(vfRow1, q1), shQual(vfRow2, q2)) + "\n\n" + vfPass
}

// check-verified judges the run verifyflip's selector credits, never a span
// of its own: a verdict-first earlier entry does not hide a latest run whose
// rows name two principals, and does not refuse a latest run verifyflip
// credits.
// regression: #6767
func TestShCheckVerifiedSameRun(t *testing.T) {
	kim := " (on-behalf-of human:kim)"
	// Loose earlier rows, two principals in the latest run: both refuse.
	split := shVerdictFirst("ci-host", shPrincipal, kim)
	vfRefuses(t, shOpts(split), "")
	if code, diag := shCheckVerified(t, split, vfStamp); code == briefInfoExitOK || !strings.Contains(diag, "runner mismatch") {
		t.Errorf("check-verified exit = %d (%s); verifyflip credits no run here, on runner agreement", code, diag)
	}
	// Another principal only in the earlier entry: verifyflip credits the
	// latest run, and check-verified admits it.
	one := shVerdictFirst(vfRunner+" @ "+vfSHA+kim, shPrincipal, shPrincipal)
	root, readme := vfFixture(t, shOpts(one))
	vfExpect(t, root, readme, verifyflipExitOK, "")
	if code, diag := shCheckVerified(t, one, vfStamp); code != briefInfoExitOK {
		t.Errorf("check-verified exit = %d on the run verifyflip credits: %s", code, diag)
	}
}

// The autoflip drops a superseded hold only behind the witness check
// verifyflip makes (closureRunWitnesses). Each case below reads clean on the
// rule 3c read alone, so only that check keeps the hold: a latest PASS that
// re-runs only some Verify rows, and one whose command is not the Verify
// row's.
// regression: #6767
func TestShAutoflipWitnessGate(t *testing.T) {
	early := func(row string) string {
		return shQual(strings.ReplaceAll(row, "2026-08-13", "2026-08-12"), shToolQuals)
	}
	partial := "### Verification — 2026-08-12\n\n" +
		witnessTableFor(early(strings.Replace(vfRow1, "pass exit=0", "could-not-run exit=- timed out", 1)), early(vfRow2)) + "\n\n" +
		"**VERIFY: BLOCKED** — row 1 COULD-NOT-CHECK (environment).\n\n" +
		"### Verification — 2026-08-13\n\n" + witnessTableFor(shQual(vfRow2, shToolQuals)) + "\n\n" + vfPass
	if held, _ := unroutedHeldLine(partial, true); held {
		t.Fatal("fixture: the rule 3c read alone holds the partial run, so it cannot show the witness check's work")
	}
	root, _ := vfFixture(t, shOpts(partial))
	brief := filepath.Join(root, "docs", "streams", "vf", "brief-01-flip.md")
	// Positive control: the same hold, then a PASS that re-runs both rows,
	// drops, so the brief path is read and the witness check can pass.
	full := strings.Replace(partial, witnessTableFor(shQual(vfRow2, shToolQuals)),
		witnessTableFor(shQual(vfRow1, shToolQuals), shQual(vfRow2, shToolQuals)), 1)
	if held, why := verifyPassHeldWitnessed(full, brief); held {
		t.Fatalf("control: a full witnessed PASS kept the hold (%s)", why)
	}
	if held, why := verifyPassHeldWitnessed(partial, brief); !held {
		t.Errorf("a PASS that re-ran only row 2 dropped row 1's hold (%s)", why)
	}
	vfRefuses(t, shOpts(partial), "")

	moved := shAFEvidence("| 1 | `go vet ./other/...` | pass exit=0 | sha256:abc123def456", "")
	if held, _ := unroutedHeldLine(moved, true); held {
		t.Fatal("fixture: the rule 3c read alone holds the moved-command run")
	}
	if got := shAutoflip(t, moved); !strings.Contains(got.Reason, "not marked deferred") {
		t.Errorf("a PASS whose command is not the Verify row's dropped the hold: %v (%s)", got.Outcome, got.Reason)
	}
}

// regression: #6767
func TestShCloseStampAgrees(t *testing.T) {
	// A hand row first: it omits the identity source the tool rows record
	// and carries a descriptive qualifier no other row has. The human close
	// (evidenceVerifierInfo) and verifyflip must read one login, one sha and
	// one qualifier list from it; the close keeps only the principal more.
	ev := "### Verification — 2026-08-13\n\n" + shHandHead +
		shHandRow("1", "pass exit=0", "2026-08-13", shHandQuals) + "\n" +
		shHandRow("2", "pass exit=1", "2026-08-13", shHandQuals) + "\n\n" +
		witnessTableFor(shQual(vfRow1, shToolQuals), shQual(vfRow2, shToolQuals)) + "\n\n" +
		"**VERIFY: PASS** — rows 1-2 pass."
	p, err := flipLatestPass(ev)
	if err != nil || p.StampErr != nil {
		t.Fatalf("verifyflip refuses the run: %v / %v", err, p.StampErr)
	}
	date, runner := evidenceVerifierInfo(ev)
	if date != "2026-08-13" || runner != vfRunner+" @ "+vfSHA+shPrincipal {
		t.Fatalf("close stamp = %q %q, want the agreed reading %q", date, runner, vfRunner+" @ "+vfSHA+shPrincipal)
	}
	if p.Stamp != date+" "+vfRunner+" @ "+vfSHA {
		t.Fatalf("verifyflip stamp = %q", p.Stamp)
	}
	closeRead, ok1 := readRunnerCell(runner)
	flipRead, ok2 := readRunnerCell(strings.TrimPrefix(p.Stamp, date+" "))
	if !ok1 || !ok2 || closeRead.login != flipRead.login || closeRead.sha != flipRead.sha {
		t.Fatalf("login/sha differ: close %+v, verifyflip %+v", closeRead, flipRead)
	}
	var kept []string
	for _, q := range closeRead.quals {
		if !strings.Contains(q, "human:") {
			kept = append(kept, q)
		}
	}
	if strings.Join(kept, "|") != strings.Join(flipRead.quals, "|") {
		t.Fatalf("qualifiers differ: close %q, verifyflip %q", closeRead.quals, flipRead.quals)
	}
	// Same tail on every row: the close still stamps that tail whole.
	same := strings.ReplaceAll(ev, shHandQuals+" |", shToolQuals+" |")
	if _, r := evidenceVerifierInfo(same); r != vfRunner+" @ "+vfSHA+shToolQuals {
		t.Fatalf("one-tail run: close runner = %q", r)
	}
}
