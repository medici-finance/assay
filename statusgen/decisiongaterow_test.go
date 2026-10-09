package main

import (
	"strings"
	"testing"
)

// Row tests for the decision-gate hold (lifecycle-v1 §4.5): the README table is
// the board, so a gate: human brief stays on the board across a change only
// while a README row still stands behind it. A brief file left behind — at the
// stream root, under done/ or in the archive tree — is not a brief on the
// board: removing the row is a drop, refused while the decision issue has no
// ruling.

const dhDonePath = "docs/streams/" + dhStream + "/done/brief-18.md"

// dhRowGoneRefused asserts the change is refused by both layers as a drop that
// names the missing row.
func dhRowGoneRefused(t *testing.T, root string) {
	t.Helper()
	dhRefusedBoth(t, root, dhBoardID)
	wantContains(t, "layer-one refusal", strings.Join(dhLayerOne(t, root), "\n"), "drops it", "README row", "is gone")
}

// The row is removed and the brief file moved under done/, unruled. Refused.
func TestDGRowGoneFileToDoneRefused(t *testing.T) {
	dhSeams(t, nil)
	root := dhFixture(t, unruledBase())
	dhWrite(t, root, map[string]string{
		dhReadmePath: dhReadme(),
		dhBriefPath:  "",
		dhDonePath:   dhFM("gate: human", "decision-issue: 41\n"),
	})
	dhRowGoneRefused(t, root)
}

// The same with a sibling brief still on the board, so the stream keeps rows.
func TestDGRowGoneFileToDoneBesideSiblingRefused(t *testing.T) {
	dhSeams(t, nil)
	files := unruledBase()
	files[dhReadmePath] = dhReadme(dhRow("18", "todo"), dhRow("19", "todo"))
	files[dhBrief19Path] = dhBrief19("")
	root := dhFixture(t, files)
	dhWrite(t, root, map[string]string{
		dhReadmePath: dhReadme(dhRow("19", "todo")),
		dhBriefPath:  "",
		dhDonePath:   dhFM("gate: human", "decision-issue: 41\n"),
	})
	dhRowGoneRefused(t, root)
}

// The row is removed and the brief file stays at the stream root. Refused.
func TestDGRowGoneFileStaysRefused(t *testing.T) {
	dhSeams(t, nil)
	root := dhFixture(t, unruledBase())
	dhWrite(t, root, map[string]string{dhReadmePath: dhReadme()})
	dhRowGoneRefused(t, root)
}

// The row's number cell is rewritten so it keys to another board id, at
// implemented, and the brief file moved under done/. The brief keeps its board
// id but loses its row: a drop, refused — the look-alike row carries no gate.
func TestDGRowGoneDecoratedCellRefused(t *testing.T) {
	for _, cell := range []string{"**18**", "18.", "#18"} {
		t.Run(cell, func(t *testing.T) {
			dhSeams(t, nil)
			root := dhFixture(t, unruledBase())
			dhWrite(t, root, map[string]string{
				dhReadmePath: dhReadme(dhRow(cell, "implemented")),
				dhBriefPath:  "",
				dhDonePath:   dhFM("gate: human", "decision-issue: 41\n"),
			})
			dhRowGoneRefused(t, root)
		})
	}
}

// The stream is moved to the archive tree and the brief's row removed there,
// the brief file kept. Refused.
func TestDGRowGoneInArchiveRefused(t *testing.T) {
	dhSeams(t, nil)
	files := unruledBase()
	files["docs/streams/control/README.md"] = dhReadme(dhRow("1", "todo"))
	root := dhFixture(t, files)
	dhWrite(t, root, map[string]string{
		dhReadmePath: "", dhBriefPath: "",
		"docs/archive/" + dhStream + "/README.md":   dhReadme(),
		"docs/archive/" + dhStream + "/brief-18.md": dhFM("gate: human", "decision-issue: 41\n"),
	})
	dhRowGoneRefused(t, root)
}

// Controls. A recorded ruling lifts the drop like any other drop; a brief whose
// row stays (moved under done/ with the row kept) is not a drop.
func TestDGRowGoneRuledPasses(t *testing.T) {
	dhSeams(t, dhForge("Option 1 for "+dhBoardID+"."))
	root := dhFixture(t, map[string]string{
		dhReadmePath: dhReadme(dhRow("18", "todo")),
		dhBriefPath:  dhFM("gate: human", dhRuledLines()),
	})
	dhWrite(t, root, map[string]string{
		dhReadmePath: dhReadme(),
		dhBriefPath:  "",
		dhDonePath:   dhFM("gate: human", dhRuledLines()),
	})
	if p := dhLayerOne(t, root); len(p) != 0 {
		t.Fatalf("a ruled drop was refused by layer one: %v", p)
	}
	if code, out := dhGate(t, root); code != 0 {
		t.Fatalf("a ruled drop turned the network lane red (exit %d):\n%s", code, out)
	}
}

func TestDGRowKeptFileToDoneQuiet(t *testing.T) {
	dhSeams(t, nil)
	root := dhFixture(t, map[string]string{
		dhReadmePath: dhReadme(dhRow("18", "done")),
		dhBriefPath:  dhFM("gate: human", ""),
	})
	dhWrite(t, root, map[string]string{
		dhBriefPath: "",
		dhDonePath:  dhFM("gate: human", ""),
	})
	if p := dhLayerOne(t, root); len(p) != 0 {
		t.Fatalf("moving a done brief under done/ with its row kept was refused: %v", p)
	}
	if code, out := dhGate(t, root); code != 0 {
		t.Fatalf("moving a done brief under done/ with its row kept turned the network lane red (exit %d):\n%s", code, out)
	}
}
