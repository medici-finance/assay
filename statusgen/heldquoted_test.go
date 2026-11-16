package main

import "testing"

// Tests for the flip read's double-quoted output rule (heldscope.go, rule 7).
//
// A verifier records what a row's command printed — a test's failure message
// under a fail-first mutation, a diagnostic — in double quotes in prose or in
// a Result cell. The marker words inside such a span are the product's
// output, not the row's disposition. The flip read (flipHeldRefusal:
// verifyflip and the reconcile fold, both of which then require a passing
// execution witness for every Verify row) skips an occurrence inside one. The
// verify-gate card and close-verify, which run no witness check, read it as
// before, so every admitted case below is named in flipAdmits and still
// refuses there.

// quotedFailFirst is a fail-first note of the shape seen in the field: a row
// named in prose, the mutated test's own failure message quoted.
const quotedFailFirst = "Fail-first: with the guard disabled, row 2's command exits 1 with " +
	"`--- FAIL: TestBucketUnreadable (0.00s)` and \"brief file unreadable: bucket = none, want could-not-check\"."

// TestHeldQuotedFlipAdmits: an occurrence inside a double-quoted span is
// admitted by the flip read only; a genuine hold beside it still refuses.
// regression: #5482
func TestHeldQuotedFlipAdmits(t *testing.T) {
	admitted := map[string]bool{
		"quoted fail-first message":    true,
		"quoted output in result cell": true,
		"second quoted span":           true,
		"capitalised quoted marker":    true,
	}
	runHeldScanCasesFlip(t, []heldScanCase{
		// Admitted by the flip read.
		{"quoted fail-first message", heldEv("ok") + "\n" + quotedFailFirst + "\n", true},
		{"quoted output in result cell",
			heldEv(`ok — probe printed "tenant b: could-not-check, no grant"`), true},
		{"second quoted span",
			heldEv("ok") + "\nRow 2 printed \"ok\" and then \"HELD for audit: record 7\".\n", true},
		{"capitalised quoted marker",
			heldEv("ok") + "\nRow 2 base run prints \"COULD-NOT-CHECK: delivery not an ancestor\".\n", true},

		// Controls: refused by every read, the flip read included.
		{"genuinely held result cell", heldEv("could-not-check — no runner online"), true},
		{"marker outside the quotes",
			heldEv("ok") + "\nRow 2 printed \"ok\" but is HELD: no runner.\n", true},
		{"quote closes before marker",
			heldEv("ok") + "\nRow 2 printed \"ok\" then could-not-check.\n", true},
		{"bare marker in quotes", heldEv("ok") + "\nRow 2 is \"HELD\".\n", true},
		{"bare marker, punctuation", heldEv("ok") + "\nRow 2: \"**could-not-check:**\"\n", true},
		{"unpaired quote", heldEv("ok") + "\nRow 2 printed \"see log, could-not-check\n", true},
		{"odd quote count",
			heldEv("ok") + "\nRow 2 was 5\" short; \"could-not-check, no runner\n", true},
		{"escaped quote in the span",
			heldEv("ok") + "\nRow 2 printed \"a \\\" could-not-check b\".\n", true},
		{"span would cross a cell",
			heldEv(`ok "x | could-not-check | y"`), true},
		{"curly quotes not read",
			heldEv("ok") + "\nRow 2 printed “HELD for audit: record 7”.\n", true},
		{"link definition title",
			heldEv("ok") + "\n[x]: /u \"row 2 HELD for audit\"\n", true},
	}, admitted)
}

// TestHeldQuotedRealEvidence: the committed graph-execution/02 Evidence, whose
// only live occurrence is the last entry's base-tree diagnostic in double
// quotes (b02Quote), flips; close-verify and the card still refuse it. The
// same Evidence with a genuine hold appended refuses on every read.
// regression: #5482
func TestHeldQuotedRealEvidence(t *testing.T) {
	ev := b02Evidence(t)
	runHeldLayerCasesFlip(t, []heldScanCase{
		{"real Evidence as committed", ev, true},
		{"real Evidence, hold appended",
			ev + "\n### Non-implementer verifier re-run — 2026-10-26\n\n- Row 3: could-not-check, no Linux runner online.\n\n**VERIFY: BLOCKED**\n", true},
	}, map[string]bool{"real Evidence as committed": true})
}
