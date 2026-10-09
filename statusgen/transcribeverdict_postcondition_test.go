package main

// transcribeverdict_postcondition_test.go — pins the emits-and-asserts seam
// (transcribeverdict_postcondition.go). Each case drives the ARMED, non-dry-run
// lane end to end with a substituted apply step that reports success while the
// artifact it owed does not land where the board reads it. Before the seam every
// one of these exited 0; now each must exit non-zero with POSTCONDITION FAILED.
// The honest apply path is TestTranscribeVerdictValidSignatureConsumed, which now
// runs through the same postcondition and must still exit 0.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const postEvLine = "| 1 | check:ci | true | 0 | PASS | 2026-08-17 | verifier |"

// armedVerdictRun builds an armed world with one gate:model brief whose signed
// verdict yields one Evidence append and one flip, installs apply as the apply
// step, runs the lane for real, and returns the captured run plus the root.
func armedVerdictRun(t *testing.T, apply func(verdictDelta) (int, error)) (capturedRun, string) {
	t.Helper()
	scanWithRoster(t, verdictRoster())
	key := verdictTestKey(t)
	root := verdictBaseRepo(t, r6Armed)
	rel := writeVerdictBrief(t, root, "01", "model", false)
	body := signVerdictBody(t, key, okPayload(okEntry(rel, postEvLine)))
	list := fixtureLister(map[string][]ghIssue{verdictTestRepo: {{Number: 701}}}, "")
	resolve := fixtureVerdictResolver(map[string]verdictIssue{
		verdictTestRepo + "#701": {Author: verifierIdent, Body: body},
	})
	old := applyVerdictDeltaFn
	applyVerdictDeltaFn = apply
	t.Cleanup(func() { applyVerdictDeltaFn = old })
	out := captureRun(t, func() int {
		return runTranscribeVerdict(root, false, "", list, resolve, passCheckCI, noHealthHold, blessR6Resolver)
	})
	return out, root
}

func wantPostconditionFail(t *testing.T, out capturedRun, why string) {
	t.Helper()
	if out.code == 0 {
		t.Fatalf("%s: the lane exited 0 although the artifact never landed:\n%s%s", why, out.log, out.err)
	}
	if !strings.Contains(out.err, "POSTCONDITION FAILED") {
		t.Fatalf("%s: want a POSTCONDITION FAILED line, got:\n%s%s", why, out.log, out.err)
	}
	if strings.Contains(out.log, "transcribe-verdict: applied") {
		t.Fatalf("%s: success was reported before the postcondition ran:\n%s", why, out.log)
	}
}

// TestNoopApplyFails: an apply that claims every owed file was touched but wrote
// nothing (the silent no-op) must fail.
func TestNoopApplyFails(t *testing.T) {
	out, _ := armedVerdictRun(t, func(d verdictDelta) (int, error) {
		return len(d.Appends) + len(d.Flips), nil
	})
	wantPostconditionFail(t, out, "silent no-op apply")
}

// TestNoopApplyZeroTouched: an apply that touched nothing for a non-empty delta
// must fail on the non-empty check alone.
func TestNoopApplyZeroTouched(t *testing.T) {
	out, _ := armedVerdictRun(t, func(verdictDelta) (int, error) { return 0, nil })
	wantPostconditionFail(t, out, "zero-touched apply")
	if !strings.Contains(out.err, "owed 2") {
		t.Fatalf("the failure should name the owed count:\n%s", out.err)
	}
}

// TestRefuseFlipUnseenByBoard: the real apply runs, then a racing rewrite puts
// the README row back to implemented. The board loader is the consumer, and it
// no longer reads verified — so the run must fail rather than report a flip.
func TestRefuseFlipUnseenByBoard(t *testing.T) {
	out, _ := armedVerdictRun(t, func(d verdictDelta) (int, error) {
		n, err := applyVerdictDelta(d)
		if err != nil {
			return n, err
		}
		for _, f := range d.Flips {
			b, _ := os.ReadFile(f.ReadmePath)
			reverted := strings.Replace(string(b), "| verified |", "| implemented |", 1)
			if werr := os.WriteFile(f.ReadmePath, []byte(reverted), 0o644); werr != nil {
				return n, werr
			}
		}
		return n, nil
	})
	wantPostconditionFail(t, out, "flip reverted after apply")
	if !strings.Contains(out.err, "not verified") {
		t.Fatalf("the failure should name the status the board read:\n%s", out.err)
	}
}

// TestRefuseEvidenceOutsideSection: the Evidence line lands in the file but
// outside its `## Evidence` section (here: at EOF, under `## Review`), which no
// reader of the Evidence section would see. The run must fail.
func TestRefuseEvidenceOutsideSection(t *testing.T) {
	out, root := armedVerdictRun(t, func(d verdictDelta) (int, error) {
		for _, a := range d.Appends {
			b, _ := os.ReadFile(a.Path)
			if werr := os.WriteFile(a.Path, append(b, []byte(a.Line+"\n")...), 0o644); werr != nil {
				return 0, werr
			}
		}
		n := 0
		for _, f := range d.Flips {
			b, _ := os.ReadFile(f.ReadmePath)
			u, ferr := flipRowToVerified(string(b), f.Num, f.Stamp)
			if ferr != nil {
				return n, ferr
			}
			if werr := os.WriteFile(f.ReadmePath, []byte(u), 0o644); werr != nil {
				return n, werr
			}
			n++
		}
		return n + len(d.Appends), nil
	})
	wantPostconditionFail(t, out, "evidence appended outside its section")
	if !strings.Contains(out.err, "Evidence") {
		t.Fatalf("the failure should name the Evidence section:\n%s", out.err)
	}
	// The planted line really is in the file — the check is section-scoped, not a
	// whole-file grep that this plant would satisfy.
	b, _ := os.ReadFile(filepath.Join(root, "docs/streams/verdict-lane/brief-01-fixture.md"))
	if !strings.Contains(string(b), postEvLine) {
		t.Fatalf("fixture error: the planted line is not in the brief at all:\n%s", b)
	}
}

// TestEvidenceSectionHasLine pins the section scoping the postcondition reads.
func TestEvidenceSectionHasLine(t *testing.T) {
	raw := "# B\n\n## Verify\n| x |\n\n## Evidence\n<!-- c -->\n| e1 |\n\n## Review\n| e2 |\n"
	cases := map[string]bool{"| e1 |": true, "| e2 |": false, "| x |": false, "| e": false, "": false}
	for line, want := range cases {
		if got := evidenceSectionHasLine(raw, line); got != want {
			t.Errorf("evidenceSectionHasLine(%q) = %v, want %v", line, got, want)
		}
	}
	if evidenceSectionHasLine("# no evidence heading\n| e1 |\n", "| e1 |") {
		t.Error("a brief with no `## Evidence` section must never satisfy the check")
	}
}
