package main

import (
	"strings"
	"testing"
)

// Tests for heldScanScope rule 3d, the maintainer's ruling
// https://github.com/medici-finance/assay/issues/4993#issuecomment-7115589994
// (ratified at https://github.com/medici-finance/assay/issues/4993#issuecomment-7115641667):
// the hold scan does not read an Evidence entry that a later strict
// **VERIFY: PASS** entry in the same brief supersedes, unless a verdict that
// is not a pass follows that marker. The superseding entry and everything
// after it are read at full strength.

const (
	sdHead   = "| # | Command | Exit | Result | Date | Runner |\n|---|---------|------|--------|------|--------|\n"
	sdRunner = vfRunner + " @ " + vfSHA
	// sdRow1 is a clean re-run of row 1, dated, with a Runner cell.
	sdRow1 = "| 1 | `go test ./...` | 0 | pass exit=0 | 2026-07-11 | " + sdRunner + " |\n"
	// sdLatest is a superseding entry: a dated clean run closing on the
	// strict marker.
	sdLatest = "### Re-run — 2026-07-11\n\n" + sdHead + sdRow1 + "\n**VERIFY: PASS** — re-run green.\n"
)

// The earlier entries a later strict PASS supersedes. Each refuses on every
// read when it stands alone or is followed by sdLatest with rule 3d off: the
// row-key and whole-entry rules (3a, 3b) cannot drop any of them.
var sdEarlier = map[string]string{
	"prose hold": "### Run — 2026-07-10\n\nRow 2 HELD: no runner was online.\n\n" +
		"**VERIFY: BLOCKED** — row 2 held.\n\n",
	"unkeyable table": "### Run — 2026-07-10\n\n| step | note |\n|---|---|\n| smoke | could-not-check |\n\n" +
		"**VERIFY: BLOCKED** — the smoke step could not run.\n\n",
	"skipped row": "### Run — 2026-07-10\n\n" + sdHead + sdRow1 +
		"| 2 | `go test ./integration/...` | -1 | could-not-check exit=-1 | 2026-07-10 | " + sdRunner + " |\n\n" +
		"**VERIFY: BLOCKED** — row 2 could not run.\n\n",
	"partial verdict": "### Run — 2026-07-10\n\nRow 2 could-not-check: the cluster was down.\n\n" +
		"**VERIFY: PARTIAL** — row 2 open.\n\n",
}

// sdReads is what every consumer of the hold scan answers on evidence: true
// is a hold. The autoflip is left out: it takes 3d's drop only behind the
// witness check (TestHeldSupersedeAutoflipBehindWitness).
func sdReads(evidence string) map[string]bool {
	held, _ := verifyPassHeldContradiction(evidence)
	return map[string]bool{
		"verify-gate card":         held,
		"close-verify implemented": closeVerifyHeldRefusal("sd/01", "implemented", evidence) != nil,
		"close-verify verified":    closeVerifyHeldRefusal("sd/01", "verified", evidence) != nil,
		"acceptance-route signals": len(scanHeldRefusals(evidence, false)) > 0,
		"awaiting sign-off act":    signOffAct(awaitBrief{Status: "implemented", Gate: "human"}, evidence) != nextActCloseCard,
		"verifyflip read":          flipHeldRefusal("sd/01", "implemented", evidence) != nil,
	}
}

func sdExpect(t *testing.T, name, evidence string, wantHeld bool) {
	t.Helper()
	for read, held := range sdReads(evidence) {
		if read == "verify-gate card" && !hasVerifyPass(evidence) {
			continue // no strict marker: the card is not raised at all
		}
		if held != wantHeld {
			t.Errorf("%s: %s held=%v, want %v", name, read, held, wantHeld)
		}
	}
}

// sdWithout3d runs f with rule 3d off.
func sdWithout3d(f func()) {
	defer func(on bool) { heldPassRule = on }(heldPassRule)
	heldPassRule = false
	f()
}

// TestHeldSupersedeClears: each earlier hold, followed by a later strict PASS
// entry, is read by no consumer; with rule 3d off each still refuses, so the
// clearing is 3d's alone. Text before the first heading is the first entry
// and is superseded like any other.
func TestHeldSupersedeClears(t *testing.T) {
	cases := map[string]string{
		"before first heading": "Row 2 HELD: no runner online.\n\n**VERIFY: BLOCKED** — row 2 held.\n\n" + sdLatest,
		"two earlier entries":  sdEarlier["prose hold"] + sdEarlier["unkeyable table"] + sdLatest,
		"neutral entry after":  sdEarlier["prose hold"] + sdLatest + "\n### Notes\n\nThe re-run used the cached module set.\n",
		// The LAST strict PASS supersedes: an earlier PASS entry followed by a
		// hold is itself superseded, hold included.
		"earlier pass then hold": sdEarlier["unkeyable table"] + sdLatest + "\n" + sdEarlier["prose hold"] + sdLatest,
	}
	for name, early := range sdEarlier {
		cases[name] = early + sdLatest
	}
	for name, ev := range cases {
		sdExpect(t, name, ev, false)
		if got := heldScanScope(ev, false).pass; got < 0 {
			t.Errorf("%s: rule 3d named no superseding entry", name)
		}
		sdWithout3d(func() {
			if held, _ := unroutedHeldLine(ev, false); !held {
				t.Errorf("%s: with rule 3d off the earlier hold is not read, so this case does not test 3d", name)
			}
		})
	}
}

// TestHeldSupersedePlainPassAfter: a plain "VERIFY: PASS" after the marker
// is not a verdict that keeps the earlier hold read. verifyflip refuses that
// Evidence on its own (its latest verdict is not the strict marker), so the
// flip read keeps 3d off there and is left out.
func TestHeldSupersedePlainPassAfter(t *testing.T) {
	ev := sdEarlier["prose hold"] + sdLatest + "\n### Notes\n\nVERIFY: PASS stands; nothing further to record.\n"
	for read, held := range sdReads(ev) {
		if read != "verifyflip read" && held {
			t.Errorf("%s held an Evidence whose later line is a plain PASS", read)
		}
	}
	if _, err := flipLatestPass(ev); err == nil {
		t.Error("flipLatestPass accepted a plain latest PASS")
	}
}

// TestHeldSupersedeLatestFullStrength: a hold in or after the superseding
// entry is read as always.
func TestHeldSupersedeLatestFullStrength(t *testing.T) {
	early := sdEarlier["prose hold"]
	for name, ev := range map[string]string{
		"hold after the last PASS": early + sdLatest + "\n### Follow-up — 2026-07-12\n\nRow 2 HELD again: the runner went away.\n",
		"hold in the PASS entry": early + "### Re-run — 2026-07-11\n\n" + sdHead + sdRow1 +
			"\nRow 2 could-not-check: still no runner.\n\n**VERIFY: PASS** — re-run green.\n",
		"hold row in the PASS entry": early + "### Re-run — 2026-07-11\n\n" + sdHead + sdRow1 +
			"| 2 | `go test ./integration/...` | -1 | could-not-check exit=-1 | 2026-07-11 | " + sdRunner + " |\n" +
			"\n**VERIFY: PASS** — re-run green.\n",
		"hold only in the PASS entry": "### Re-run — 2026-07-11\n\n" + sdHead + sdRow1 +
			"\nRow 2 HELD on its check definition.\n\n**VERIFY: PASS** — re-run green.\n",
	} {
		sdExpect(t, name, ev, true)
	}
}

// TestHeldSupersedeNeedsStrictPass: an earlier hold with no later strict
// PASS entry refuses, and a PASS that is not the strict marker supersedes
// nothing.
func TestHeldSupersedeNeedsStrictPass(t *testing.T) {
	early := sdEarlier["prose hold"]
	latest := func(marker string) string {
		return "### Re-run — 2026-07-11\n\n" + sdHead + sdRow1 + "\n" + marker + "\n"
	}
	for name, ev := range map[string]string{
		"no later entry":       early,
		"no later marker":      early + latest("Re-run green."),
		"plain PASS":           early + latest("VERIFY: PASS — re-run green."),
		"lower-case bold":      early + latest("**verify: pass** — re-run green."),
		"quoted marker":        early + latest("> **VERIFY: PASS** — re-run green."),
		"list item marker":     early + latest("- **VERIFY: PASS** — re-run green."),
		"indented marker":      early + latest("  **VERIFY: PASS** — re-run green."),
		"fenced marker":        early + latest("```\n**VERIFY: PASS** — re-run green.\n```"),
		"struck marker":        early + latest("~~**VERIFY: PASS**~~ — re-run green."),
		"marker in code":       early + latest("`**VERIFY: PASS**` — re-run green."),
		"two verdicts on line": early + latest("**VERIFY: PASS** — rows 1-2; VERIFY: BLOCKED on row 3."),
		"same entry as hold": "### Run — 2026-07-10\n\nRow 2 HELD: no runner was online.\n\n" +
			sdHead + sdRow1 + "\n**VERIFY: PASS** — re-run green.\n",
	} {
		sdExpect(t, name, ev, true)
	}
}

// TestHeldSupersedeLaterVerdictKeepsHold: a verdict that is not a pass,
// written after the superseding marker in any form, keeps every entry read,
// so the EARLIER hold refuses (the later verdict line itself carries no hold
// word). The control is the same Evidence with a neutral line there.
func TestHeldSupersedeLaterVerdictKeepsHold(t *testing.T) {
	early := sdEarlier["prose hold"]
	after := func(line string) string {
		return early + sdLatest + "\n### Follow-up — 2026-07-12\n\n" + line + "\n"
	}
	control := after("The re-run used the cached module set.")
	sdExpect(t, "control", control, false)
	for name, line := range map[string]string{
		"bold BLOCKED":          "**VERIFY: BLOCKED** — rerun pending.",
		"plain BLOCKED":         "VERIFY: BLOCKED — rerun pending.",
		"plain BLOCKED in line": "Second look — VERIFY: BLOCKED on row 3.",
		"lower-case blocked":    "verify: blocked — rerun pending.",
		"bold label":            "**VERIFY:** BLOCKED — rerun pending.",
		"emphasis label":        "_VERIFY_: blocked — rerun pending.",
		"space before colon":    "VERIFY : BLOCKED — rerun pending.",
		"bold FAIL":             "**VERIFY: FAIL** — row 1 regressed.",
		"plain FAIL":            "VERIFY: FAIL — row 1 regressed.",
		"FAILED":                "VERIFY: FAILED on row 1.",
		"bold PARTIAL":          "**VERIFY: PARTIAL** — row 3 open.",
		"plain PARTIAL":         "VERIFY: PARTIAL — row 3 open.",
		"lower-case partial":    "verify: partial (row 3 open).",
		"quoted BLOCKED":        "> VERIFY: BLOCKED — rerun pending.",
		"fenced FAIL":           "```\nVERIFY: FAIL\n```",
	} {
		ev := after(line)
		reads := sdReads(ev)
		for read, held := range reads {
			if !held {
				t.Errorf("%s: %s admitted a hold a later non-pass verdict keeps read", name, read)
			}
		}
		if held, why := verifyPassHeldContradiction(ev); held && !strings.Contains(why, "no runner was online") {
			t.Errorf("%s: refused on %q, want the earlier hold", name, why)
		}
		if got := heldScanScope(ev, false).pass; got >= 0 {
			t.Errorf("%s: rule 3d named a superseding entry at line %d", name, got+1)
		}
	}
	// The verdict on the marker's own line, after it, is the same.
	sdExpect(t, "same line", early+"### Re-run — 2026-07-11\n\n"+sdHead+sdRow1+
		"\n**VERIFY: PASS** — re-run green, verify: partial for row 3.\n", true)
}

// TestHeldLaterVerdictRe pins the later-verdict pattern on its own: every
// non-pass verdict form matches, and a pass or a bare word does not.
func TestHeldLaterVerdictRe(t *testing.T) {
	for _, s := range []string{
		"**VERIFY: BLOCKED**", "VERIFY: BLOCKED —", "verify: blocked", "**VERIFY:** BLOCKED",
		"_VERIFY_: blocked", "VERIFY : FAIL", "VERIFY: FAILED", "VERIFY: PARTIAL", "verify: partially",
		"VERIFY: HELD", "VERIFY: could-not-check", "VERIFY: INCOMPLETE", "VERIFY: inconclusive",
		"x — VERIFY: BLOCKED on row 3",
	} {
		if !heldLaterVerdictRe.MatchString(s) {
			t.Errorf("heldLaterVerdictRe missed %q", s)
		}
	}
	for _, s := range []string{
		"**VERIFY: PASS**", "VERIFY: PASS", "verified: blocked", "unverify: fail", "blocked", "FAIL",
		"the verifyrun tool: failing rows none",
	} {
		if heldLaterVerdictRe.MatchString(s) {
			t.Errorf("heldLaterVerdictRe matched %q", s)
		}
	}
}

// TestHeldSupersedeBoundaryDoubt: a superseding heading the page may not
// draw (an element left open before it) supersedes nothing.
func TestHeldSupersedeBoundaryDoubt(t *testing.T) {
	ev := sdEarlier["prose hold"] + "<details>\n\n" + sdLatest
	if got := heldScanScope(ev, false).pass; got >= 0 {
		t.Fatalf("rule 3d named a superseding entry at line %d behind an open element", got+1)
	}
	sdExpect(t, "open element", ev, true)
}

// TestHeldSupersedeAutoflipBehindWitness: the model autoflip keeps a hold
// rule 3d drops until verifyflip's witness check passes on the run it
// credits; with no brief to check against it keeps the hold.
func TestHeldSupersedeAutoflipBehindWitness(t *testing.T) {
	ev := sdEarlier["skipped row"] + sdLatest
	if held, _ := verifyPassHeldContradiction(ev); held {
		t.Fatal("fixture: the plain read holds, so this case does not test the autoflip")
	}
	held, why := verifyPassHeldWitnessed(ev, "")
	if !held || !strings.Contains(why, "could-not-check") {
		t.Errorf("verifyPassHeldWitnessed = %v %q, want the superseded could-not-check row held", held, why)
	}
	if held, why := verifyPassHeldWitnessed(sdLatest, ""); held {
		t.Errorf("verifyPassHeldWitnessed held a clean run: %q", why)
	}
}
