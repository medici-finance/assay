package main

import (
	"strings"
	"testing"
)

// vlUTCTable is a witness table whose Date header carries a qualifier. The
// witness check credits its rows, but flipDateRunnerRows does not read it as a
// Date/Runner table.
func vlUTCTable(rows ...string) string {
	return strings.Replace(witnessTableFor(rows...), "| Date |", "| Date (UTC) |", 1)
}

const (
	vlPart1     = "### Non-implementer run part 1 — 2026-08-13\n\n"
	vlPart2     = "### Non-implementer run part 2 — 2026-08-13\n\n"
	vlHeldProse = "Row 2 is HELD: its pass above is from a stub, the real endpoint could not be reached.\n\n"
	vlBlocked   = "### Non-implementer run — VERIFY: BLOCKED — 2026-08-13\n\n"
)

// TestVerifyflipCommentHiddenHeadingRefuses: a comment span whose markers sit
// in inline code or in fenced code renders as no comment, so the latest entry
// heading between them is live on the page. The run reader strips that span
// and loses the heading, so rule 3c must stay off and the hold above the
// heading, inside the run verifyflip credits, still refuses.
// regression: #4294
func TestVerifyflipCommentHiddenHeadingRefuses(t *testing.T) {
	cases := map[string]string{
		"split run, inline code": vlPart1 + witnessTableFor(vfRow1, vlRow2) + "\n\n" + vlHeldProse +
			"The stub is described at `<!--` in the runbook.\n\n" + vlPart2 + "Closed by `-->`.\n\n" +
			witnessTableFor(vfRow1) + "\n\n" + vfPass,
		"split run, fenced code": vlPart1 + witnessTableFor(vfRow1, vlRow2) + "\n\n" + vlHeldProse +
			"```\n<!--\n```\n\n" + vlPart2 + "```\n-->\n```\n\n" + witnessTableFor(vfRow1) + "\n\n" + vfPass,
		"verdict-first BLOCKED, inline code": vlBlocked + witnessTableFor(vfRow1, vlRow2) + "\n\n" +
			vlBlockedBullet + " See `<!--`\n\n" + vlLatest + "Closed by `-->`\n\n" +
			witnessTableFor(vfRow1) + "\n\n" + vfPass,
	}
	for name, ev := range cases {
		t.Run(name, func(t *testing.T) {
			o := vlOpts()
			o.evidence = ev
			if s := heldScanScope(ev, true); s.latest {
				t.Errorf("rule 3c is on for a heading the run reader cannot see")
			}
			vfRefuses(t, o, "could-not-check")
		})
	}
}

// TestVerifyflipLooseHeaderWitnessRefuses: a table above the latest heading
// whose Date header carries a qualifier is credited by the witness check, so
// its rows are witnesses the flip counts from before that heading. Rule 3c
// must stay off, and the hold beside them still refuses.
// regression: #4294
func TestVerifyflipLooseHeaderWitnessRefuses(t *testing.T) {
	older := strings.ReplaceAll(strings.ReplaceAll(vlUTCTable(vfRow1, vlRow2), vfSHA, vlOldSHA), "2026-08-13", "2026-08-10")
	cases := map[string]string{
		"split run": vlPart1 + vlUTCTable(vfRow1, vlRow2) + "\n\n" + vlHeldProse + vlPart2 +
			witnessTableFor(vfRow1) + "\n\n" + vfPass,
		"verdict-first BLOCKED": vlBlocked + vlUTCTable(vfRow1, vlRow2) + "\n\n" + vlBlockedBullet + "\n\n" +
			vlLatest + witnessTableFor(vfRow1) + "\n\n" + vfPass,
		"split run, older sha": vlPart1 + older + "\n\n" + vlHeldProse + vlPart2 +
			witnessTableFor(vfRow1) + "\n\n" + vfPass,
	}
	for name, ev := range cases {
		t.Run(name, func(t *testing.T) {
			o := vlOpts()
			o.evidence = ev
			if s := heldScanScope(ev, true); s.latest {
				t.Errorf("rule 3c is on with a credited witness table above the latest heading")
			}
			vfRefuses(t, o, "could-not-check")
		})
	}
}

// TestHeldLatestCreditedAboveProtected: a protected-table marker makes the witness
// check read the full history, so it keeps 3c off whatever lies above the
// heading, and so does any row a flip reader reads there; plain prose above
// it does not.
// regression: #4294
func TestHeldLatestCreditedAboveProtected(t *testing.T) {
	sha := strings.Repeat("a", 40)
	marker := "Verification-Table: delivery=" + sha + " base=" + sha + " original=" + sha + "\n"
	if !heldCreditedAbove([]string{"prose only"}, marker+vlLatestRun()) {
		t.Errorf("a protected-table marker does not keep 3c off")
	}
	if heldCreditedAbove([]string{"prose only"}, vlLatestRun()) {
		t.Errorf("prose above the heading keeps 3c off")
	}
	if !heldCreditedAbove(strings.Split(vlUTCTable(vfRow1), "\n"), vlLatestRun()) {
		t.Errorf("a credited Date (UTC) row above the heading does not keep 3c off")
	}
	// A Date/Runner row whose Result is no witness is still a row the run
	// reader dates and sha-checks, so it keeps 3c off too.
	unread := "| 2 | `true` | see the notes below | sha256:aa11bb22cc33 | 2026-08-13 | " + vfRunner + " @ " + vfSHA + " |"
	if !heldCreditedAbove(strings.Split(witnessTableFor(unread), "\n"), vlLatestRun()) {
		t.Errorf("a Date/Runner row above the heading does not keep 3c off")
	}
}

// TestVerifyflipRestrippedCommentRefuses: the run reader strips comments once,
// and the witness check strips the run again. A raw `<!<!--x-->--` leaves a new
// `<!--` after the first strip, which the second pairs with a `-->` below the
// latest heading, so a witness row starts above that heading. Rule 3c must stay
// off whenever the witness check's own strip would change the run, and the hold
// above the heading still refuses.
// regression: #4294
func TestVerifyflipRestrippedCommentRefuses(t *testing.T) {
	head := vlPart1 + vlHeldProse +
		"| # | Command | Result | Output | Date (UTC) | Runner |\n|---|---------|--------|--------|------------|--------|\n" +
		"| 2 | `true` | pass exit=0 <!<!--x-->--\n\n" + vlPart2
	tail := "--> | sha256:aa11bb22cc33 | 2026-08-13 | " + vfRunner + " @ " + vfSHA + " |\n"
	cases := map[string]string{
		"split run":             head + tail + "\n" + witnessTableFor(vfRow1) + "\n\n" + vfPass,
		"split run, fenced end": head + "```\n" + tail + "```\n\n" + witnessTableFor(vfRow1) + "\n\n" + vfPass,
	}
	for name, ev := range cases {
		t.Run(name, func(t *testing.T) {
			o := vlOpts()
			o.evidence = ev
			if s := heldScanScope(ev, true); s.latest {
				t.Errorf("rule 3c is on for a run the witness check strips again")
			}
			vfRefuses(t, o, "could-not-check")
		})
	}
}
