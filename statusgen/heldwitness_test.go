package main

import (
	"encoding/json"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// Tests for the held scan reading statusgen verifyrun's own execution witness
// (#4294). A witness table's Output cell is a digest and its Result cell may
// carry a sandbox token; a write-mode run puts its attestation (and, for a
// container run, its tree, image and bracket lines) directly under the table.
// Each is a shape the tool writes, matched whole, so a later witness that
// re-ran every row clean supersedes an earlier hold the way a hand-written
// results table already does (#1894, #3841) — and nothing else about the
// scope changes.

const (
	witHex64 = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	witHex40 = "0123456789abcdef0123456789abcdef01234567"
	// witAttest is a version-1 attestation line as the admission receipt
	// renders it.
	witAttest = "Verification-Attestation: example-org/example-repo#4017 run=" + witHex64 +
		" source=" + witHex40 + " brief=docs/streams/fixture/brief-02-fixture.md model=fixture-model tier=strong"
	// witContainer is the block a container run writes under its table
	// (containerWitnessLines) plus the bracket close's line.
	witContainer = "Tree-Digest: sha256:" + witHex64 + "\n" +
		"Container-Image: sha256:" + witHex64 + "\n" + witAttest + "\n" +
		"Admission-Bracket: run=" + witHex64 + " source=" + witHex40 + " tree=sha256:" + witHex64 +
		" witness=sha256:" + witHex64 + " opened=2026-10-22T17:09:57Z closed=2026-10-22T17:12:01Z before=receipt after=check-verifier"
)

// witPass re-runs a row clean; witRow3 returns state for row 3 and a clean
// pass for every other row.
func witPass(int) (string, int, string) { return statePass, 0, "" }

func witRow3(state string, exit int, note string) func(int) (string, int, string) {
	return func(r int) (string, int, string) {
		if r == 3 {
			return state, exit, note
		}
		return statePass, 0, ""
	}
}

// witTable renders a verifyrun witness table for rows 1..n through the
// production renderer (witness.row), each row's state from state, the sandbox
// token on every row, then trailer (if any) directly under the last row.
func witTable(n int, sandbox string, state func(int) (string, int, string), trailer string) string {
	s := witnessHeader + "\n"
	for r := 1; r <= n; r++ {
		st, exit, note := state(r)
		w := witness{ID: strconv.Itoa(r), Command: "tool check " + strconv.Itoa(r), State: st, Exit: exit,
			OutHash: "0123456789ab", Date: "2026-10-22", Runner: "fixture-verifier[bot]", Tree: "0123456789ab",
			HostOnBehalfOf: true, Sandbox: sandbox, Note: note}
		s += w.row() + "\n"
	}
	if trailer != "" {
		s += trailer + "\n"
	}
	return s
}

// witEntry is a level-3 Evidence entry: heading, witness, prose, verdict.
func witEntry(heading, table, prose, verdict string) string {
	s := "### " + heading + "\n\n" + table + "\n"
	if prose != "" {
		s += prose + "\n\n"
	}
	return s + verdict + "\n\n"
}

// witLatest is the latest entry: a write-mode container witness that re-ran
// all four rows clean, closing on a strict PASS.
func witLatest(state func(int) (string, int, string)) string {
	return witEntry("2026-10-22 verify pass (witnessed)", witTable(4, sandboxContainerNetns, state, witContainer),
		"", "**VERIFY: PASS** — 4/4 rows at the attested source")
}

const (
	witCNR    = "check:ci hermetic execution requires a network-off sandbox, unavailable on this host"
	witBullet = "- Row 3: BLOCKED — could-not-check, no Linux runner online."
	witHeldBy = "| # | Command | Expect | Observed | Date | Runner |\n|---|---------|--------|----------|------|--------|\n" +
		"| 3 | `tool check 3` | exit 0 | could-not-check — no runner online | 2026-09-23 | fixture-verifier |\n"
)

// The three superseded shapes the issue names, each in an earlier entry whose
// own witness ran under unshare and carries its attestation line straight
// under the table.
var (
	// (a) an old results-table row: row 3 held in its Observed cell.
	witOldRow = witEntry("2026-09-23 verifier run", witTable(4, sandboxUnshare, witRow3(stateCouldNotRun, -1, witCNR), witAttest)+
		"\n"+witHeldBy, "", "VERIFY: BLOCKED")
	// (b) an old BLOCKED bullet in the entry's prose.
	witOldBullet = witEntry("2026-09-27 verifier re-run", witTable(4, sandboxUnshare, witRow3(stateCouldNotRun, -1, witCNR), witAttest),
		witBullet, "**VERIFY: BLOCKED** — 3/4 pass, 1 could-not-check")
	// (c) an old entry heading carrying VERIFY: BLOCKED.
	witOldHeading = witEntry("Non-implementer verifier run — VERIFY: BLOCKED — 3/4 pass, 1 could-not-check, 0 fail — 2026-09-23",
		witTable(4, sandboxUnshare, witRow3(stateCouldNotRun, -1, witCNR), witAttest), "", "")
)

// TestHeldScopeWitnessCover: a later write-mode verifyrun witness that re-ran
// every row clean supersedes an earlier held row (rule 3a) and, as a whole, an
// earlier BLOCKED bullet or BLOCKED heading (rule 3b). Each case refused before
// #4294: the Output digest kept every row from covering, and the attestation
// line under an earlier witness read as a row the scan could not key.
// regression: #4294
func TestHeldScopeWitnessCover(t *testing.T) {
	handLatest := witEntry("2026-10-22 verify pass", "| # | Command | Exit | Result |\n|---|---|---|---|\n"+
		"| 1 | `tool check 1` | 0 | ok |\n| 2 | `tool check 2` | 0 | ok |\n| 3 | `tool check 3` | 0 | ok |\n| 4 | `tool check 4` | 0 | ok |\n",
		"", "**VERIFY: PASS**")
	unshareLatest := witEntry("2026-10-22 verify pass", witTable(4, "", witPass, witAttest), "", "**VERIFY: PASS**")
	runHeldLayerCases(t, []heldScanCase{
		{"(a) old held table row", witOldRow + witLatest(witPass), false},
		{"(b) old BLOCKED bullet", witOldBullet + witLatest(witPass), false},
		{"(c) old BLOCKED heading", witOldHeading + witLatest(witPass), false},
		{"all three, oldest first", witOldHeading + witOldBullet + witOldRow + witLatest(witPass), false},
		// The digest alone: the earlier entry has no trailer line.
		{"digest only, no earlier trailer",
			witEntry("Run 1", witTable(4, "", witRow3(stateFail, 2, ""), ""), witBullet, "VERIFY: BLOCKED") + witLatest(witPass), false},
		// The trailer alone: the latest entry is a hand table.
		{"trailer only, hand-table latest", witOldBullet + handLatest, false},
		// A witness with no sandbox and a bare attestation line.
		{"host witness, attestation only", witOldHeading + unshareLatest, false},
		// Planted second instance: the container block under an earlier witness.
		{"container block under earlier witness",
			witEntry("Run 1", witTable(4, sandboxContainerNetns, witRow3(stateFail, 1, ""), witContainer), witBullet, "VERIFY: BLOCKED") +
				witLatest(witPass), false},
	})
}

// TestHeldScopeWitnessLive: a hold that is still live keeps refusing — the
// latest entry is not a PASS, its witness did not re-run the held row clean,
// or a line under an earlier witness is not one the tool writes.
func TestHeldScopeWitnessLive(t *testing.T) {
	failed := witRow3(stateFail, 1, "")
	cnr := witRow3(stateCouldNotRun, -1, witCNR)
	blank := func(trailer string) string {
		return witEntry("Run 1", witTable(4, sandboxUnshare, cnr, trailer), witBullet, "VERIFY: BLOCKED")
	}
	runHeldLayerCases(t, []heldScanCase{
		// The latest entry itself is the hold.
		{"latest entry BLOCKED", witLatest(witPass) + witOldHeading, true},
		{"latest PASS with a held bullet", witEntry("Run 2", witTable(4, "", witPass, witAttest), witBullet, "**VERIFY: PASS**"), true},
		// A later row with the same key that did not pass.
		{"(a) re-run row 3 failed", witOldRow + witLatest(failed), true},
		{"(a) re-run row 3 could-not-run", witOldRow + witLatest(cnr), true},
		{"(b) re-run row 3 failed", witOldBullet + witLatest(failed), true},
		{"(c) re-run row 3 could-not-run", witOldHeading + witLatest(cnr), true},
		{"(b) latest skipped row 3",
			witOldBullet + witEntry("Run 2", strings.Replace(witTable(4, "", witPass, witAttest), witnessRow3(), "", 1), "", "**VERIFY: PASS**"), true},
		// A digest is no outcome: a row whose Result is empty does not cover.
		{"digest with an empty Result", witOldRow + witEntry("Run 2", witnessHeader+"\n"+
			"| 1 | `tool check 1` |  | sha256:0123456789ab | 2026-10-22 | r @ 0123456789ab |\n"+
			"| 2 | `tool check 2` |  | sha256:0123456789ab | 2026-10-22 | r @ 0123456789ab |\n"+
			"| 3 | `tool check 3` |  | sha256:0123456789ab | 2026-10-22 | r @ 0123456789ab |\n", "", "**VERIFY: PASS**"), true},
		{"digest outside an Output column", witOldRow + witEntry("Run 2", "| # | Command | Result | Notes |\n|---|---|---|---|\n"+
			"| 3 | `tool check 3` | pass exit=0 | sha256:0123456789ab |\n", "", "**VERIFY: PASS**"), true},
		{"digest of the wrong length", witOldRow + witEntry("Run 2", strings.ReplaceAll(witTable(4, "", witPass, witAttest),
			"sha256:0123456789ab", "sha256:0123456789abcdef"), "", "**VERIFY: PASS**"), true},
		{"sandbox token outside the closed set", witOldRow + witEntry("Run 2", strings.ReplaceAll(witTable(4, "", witPass, witAttest),
			"pass exit=0 ", "pass exit=0 sandbox=none "), "", "**VERIFY: PASS**"), true},
		// Under an earlier witness, only the tool's own lines are passed over.
		{"prose straight after the trailer", strings.Replace(blank(witAttest), witAttest+"\n", witAttest+"\nrow 3 still waits\n", 1) +
			witLatest(witPass), true},
		{"prose straight after the table", blank("row 3 still waits") + witLatest(witPass), true},
		{"trailer with a note appended", blank(witAttest+" (row 3 HELD)") + witLatest(witPass), true},
		{"trailer under a hand table", witEntry("Run 1", "| # | Command | Exit | Result |\n|---|---|---|---|\n"+
			"| 1 | `tool check 1` | 0 | ok |\n"+witAttest+"\n", witBullet, "VERIFY: BLOCKED") + witLatest(witPass), true},
		{"fence straight after the trailer", blank(witAttest+"\n```\nlog\n```") + witLatest(witPass), true},
	})
}

// witnessRow3 is row 3 of a host witness of clean rows, as witTable renders it.
func witnessRow3() string {
	w := witness{ID: "3", Command: "tool check 3", State: statePass, Exit: 0, OutHash: "0123456789ab",
		Date: "2026-10-22", Runner: "fixture-verifier[bot]", Tree: "0123456789ab", HostOnBehalfOf: true}
	return w.row() + "\n"
}

// TestHeldScopeWitnessAnchors pins each anchor the witness reading rests on,
// one fixture per anchor: every trailer line is matched to its end, the
// witness header is matched exactly, a digest is neutral only in an Output
// column, and the sandbox token is one whole closed-set word. Each case is a
// hold that must keep refusing; the matching heldscope-mutations.json entry
// loosens the one anchor and is caught here.
// regression: #4294
func TestHeldScopeWitnessAnchors(t *testing.T) {
	cnr := witRow3(stateCouldNotRun, -1, witCNR)
	held := func(table string) string { return witEntry("Run 1", table, witBullet, "VERIFY: BLOCKED") }
	under := func(trailer string) string { return held(witTable(4, sandboxUnshare, cnr, trailer)) }
	block := strings.Split(witContainer, "\n") // Tree-Digest, Container-Image, attestation, bracket
	note := " (row 3 HELD)"
	header := func(from, to string) string {
		return held(strings.Replace(witTable(4, sandboxUnshare, cnr, witAttest), from, to, 1))
	}
	sandbox := func(tok string) string {
		return witOldRow + witEntry("Run 2", witTable(4, tok, witPass, witAttest), "", "**VERIFY: PASS**")
	}
	runHeldLayerCases(t, []heldScanCase{
		// Each trailer line is anchored at its end, not only the attestation.
		{"Tree-Digest with a note appended", under(block[0]+note) + witLatest(witPass), true},
		{"Container-Image with a note", under(block[1]+note) + witLatest(witPass), true},
		{"Admission-Bracket with a note", under(block[3]+note) + witLatest(witPass), true},
		// The witness header is matched as the tool writes it, byte for byte.
		{"header with a trailing space", header("| Runner |\n", "| Runner | \n") + witLatest(witPass), true},
		{"header with a leading space", header("| # |", " | # |") + witLatest(witPass), true},
		// A digest is neutral only under an Output header, not any result column.
		{"digest in an Observed column", witOldRow + witEntry("Run 2", "| # | Command | Result | Observed |\n|---|---|---|---|\n"+
			"| 3 | `tool check 3` | pass exit=0 | sha256:0123456789ab |\n", "", "**VERIFY: PASS**"), true},
		// The sandbox token is a whole word: a closed-set prefix is not enough.
		{"sandbox=unshare-ok", sandbox(sandboxUnshare + "-ok"), true},
		{"sandbox=container-netns-ok", sandbox(sandboxContainerNetns + "-ok"), true},
		// A trailer line in the latest entry is read for holds like any line.
		{"hold word in a latest trailer", witOldRow + witEntry("Run 2", witTable(4, sandboxContainerNetns, witPass,
			strings.Replace(witContainer, "model=fixture-model", "model=HELD", 1)), "", "**VERIFY: PASS**"), true},
	})
}

// witTableFF renders a fail-first verifyrun witness table for rows 1..n
// through the production table renderer (witnessTable), so the header is
// witnessHeaderFailFirst and every row carries the Base cell failFirstCell
// writes: a mix of the four base states, as a real fail-first run records
// them. trailer (if any) follows directly under the last row.
func witTableFF(n int, sandbox string, state func(int) (string, int, string), trailer string) string {
	bases := [][3]string{{"abc1234", baseRed, "1"}, {"-", baseNotSelected, ""}, {"-", baseUnproven, ""}, {"abc1234", baseGreen, "0"}}
	var ws []witness
	for r := 1; r <= n; r++ {
		st, exit, note := state(r)
		b := bases[(r-1)%len(bases)]
		rc, _ := strconv.Atoi(b[2])
		w := witness{ID: strconv.Itoa(r), Command: "tool check " + strconv.Itoa(r), State: st, Exit: exit,
			OutHash: "0123456789ab", Date: "2026-10-22", Runner: "fixture-verifier[bot]", Tree: "0123456789ab",
			HostOnBehalfOf: true, Sandbox: sandbox, Note: note}
		w.Base = failFirstCell(strings.TrimPrefix(b[0], "-"), b[1], rc, w.Tree)
		ws = append(ws, w)
	}
	s := witnessTable(ws) + "\n"
	if trailer != "" {
		s += trailer + "\n"
	}
	return s
}

// witAttestV2 is a version-2 (cross-repository) attestation line, rendered
// by the production receipt reader (verifierAdmissionReceipt) so its fields
// sit in the order the tool writes them: source-repo= before source=,
// tracking-source= after it.
func witAttestV2(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs("fixture-tracking")
	if err != nil {
		t.Fatal(err)
	}
	home, err := filepath.Abs("fixture-source")
	if err != nil {
		t.Fatal(err)
	}
	type binding struct {
		Version                                                           int
		Run, Repo, Source, SourceRepo, TrackingSource, Brief, Model, Tier string
	}
	out, err := json.Marshal(struct {
		Issue      int
		BriefRoot  string
		SourceHome string
		Binding    binding
	}{4017, root, home, binding{2, witHex64, "example-org/example-tracking", witHex40, "example-org/example-source",
		witHex40, "docs/streams/fixture/brief-02-fixture.md", "fixture-model", "strong"}})
	if err != nil {
		t.Fatal(err)
	}
	line, _, _, err := verifierAdmissionReceipt(out, "docs/streams/fixture/brief-02-fixture.md")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(line, " source-repo=") || !strings.Contains(line, " tracking-source=") {
		t.Fatalf("not a version-2 attestation: %q", line)
	}
	return line
}

// TestHeldScopeWitnessForms: each production form a clean verifyrun witness
// takes covers, so none of them falsely holds: the fail-first witness
// (witnessHeaderFailFirst with its Base cell) as an earlier or the latest
// entry, a version-2 attestation under an earlier or the latest witness, and
// the default unshare sandbox on a clean latest row (a container run writes
// container-netns on every row, so unshare never sits in a container block).
// Each form is pinned by a heldscope-mutations.json entry that drops it and
// is caught here. A trailer in the latest entry does not affect alignment,
// so the v2 latest cases are coverage of that position, not a mutant's kill.
// regression: #4294
func TestHeldScopeWitnessForms(t *testing.T) {
	cnr := witRow3(stateCouldNotRun, -1, witCNR)
	v2 := witAttestV2(t)
	runHeldLayerCases(t, []heldScanCase{
		// The fail-first header passes its trailer over as witnessHeader does.
		{"fail-first witness, earlier",
			witEntry("Run 1", witTableFF(4, sandboxUnshare, cnr, witAttest), witBullet, "VERIFY: BLOCKED") + witLatest(witPass), false},
		{"fail-first container block, earlier",
			witEntry("Run 1", witTableFF(4, sandboxContainerNetns, cnr, witContainer), witBullet, "VERIFY: BLOCKED") + witLatest(witPass), false},
		// A fail-first witness's Base cell records the merge-base run, not
		// the head outcome, so a clean latest fail-first row covers.
		{"fail-first witness, latest", witOldRow + witEntry("Run 2", witTableFF(4, sandboxContainerNetns, witPass, witContainer),
			"", "**VERIFY: PASS**"), false},
		{"fail-first latest, (c) heading", witOldHeading + witEntry("Run 2", witTableFF(4, sandboxUnshare, witPass, witAttest),
			"", "**VERIFY: PASS**"), false},
		// A cross-repository run's version-2 attestation is a trailer line.
		{"v2 attestation, earlier",
			witEntry("Run 1", witTable(4, sandboxUnshare, cnr, v2), witBullet, "VERIFY: BLOCKED") + witLatest(witPass), false},
		{"v2 attestation in a container block",
			witEntry("Run 1", witTable(4, sandboxContainerNetns, cnr, strings.Replace(witContainer, witAttest, v2, 1)), witBullet,
				"VERIFY: BLOCKED") + witLatest(witPass), false},
		{"v2 attestation, latest", witOldRow + witEntry("Run 2", witTable(4, sandboxUnshare, witPass, v2), "", "**VERIFY: PASS**"), false},
		{"v2 container block, latest", witOldBullet + witEntry("Run 2", witTable(4, sandboxContainerNetns, witPass,
			strings.Replace(witContainer, witAttest, v2, 1)), "", "**VERIFY: PASS**"), false},
		// unshare is the default sandbox: a clean row carrying it covers.
		{"unshare sandbox, latest", witOldRow + witEntry("Run 2", witTable(4, sandboxUnshare, witPass, witAttest), "", "**VERIFY: PASS**"), false},
		{"unshare latest, (b) bullet", witOldBullet + witEntry("Run 2", witTable(4, sandboxUnshare, witPass, witAttest), "", "**VERIFY: PASS**"), false},
	})
}

// TestHeldScopeWitnessExact pins the cells read as neutral to the exact
// text the tool writes: a digest or a Base cell with anything else in it
// (before, after or around it), or a Base cell outside a Base column, is a
// note and keeps the row from covering, and a Base cell with an empty Result
// beside it is no outcome. Each case is a hold that must keep refusing.
// regression: #4294
func TestHeldScopeWitnessExact(t *testing.T) {
	ff := witTableFF(4, sandboxUnshare, witPass, witAttest)
	base3 := failFirstCell("", baseUnproven, -1, "0123456789ab")
	latest := func(table string) string { return witOldRow + witEntry("Run 2", table, "", "**VERIFY: PASS**") }
	runHeldLayerCases(t, []heldScanCase{
		{"digest in emphasis", latest(strings.ReplaceAll(witTable(4, "", witPass, witAttest),
			"sha256:0123456789ab", "**sha256:0123456789ab**")), true},
		{"digest in upper case", latest(strings.ReplaceAll(witTable(4, "", witPass, witAttest),
			"sha256:0123456789ab", "SHA256:0123456789AB")), true},
		{"Base cell with a note", latest(strings.Replace(ff, base3+" |", base3+" (see log) |", 1)), true},
		// Anchored at the start too: text before base= is a note.
		{"Base cell with leading text", latest(strings.Replace(ff, "| "+base3+" |", "| see log; "+base3+" |", 1)), true},
		// As the tool writes it, not after heldNorm: emphasis is a note.
		{"Base cell in emphasis", latest(strings.Replace(ff, "| "+base3+" |", "| **"+base3+"** |", 1)), true},
		{"Base cell in a Notes column", latest("| # | Command | Result | Notes |\n|---|---|---|---|\n" +
			"| 3 | `tool check 3` | pass exit=0 | " + base3 + " |\n"), true},
		// A Base cell is no outcome: a row whose Result is empty does not cover.
		{"Base cell, empty Result", latest("| # | Command | Result | Base |\n|---|---|---|---|\n" +
			"| 3 | `tool check 3` |  | " + base3 + " |\n"), true},
	})
}
