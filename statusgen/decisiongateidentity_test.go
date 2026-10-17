package main

import (
	"strings"
	"testing"
)

// Identity tests for the decision-gate hold (lifecycle-v1 §4.5): the hold
// judges a gate: human brief's transition against its OWN record, which needs
// a board identity to resolve to exactly one record on each side of the change,
// and a base brief to stand behind at most one brief after it. Each test below
// is one route by which an identity resolved to more than one record (or one
// record stood for two briefs) and the transition went unjudged; each ends in a
// refusal by layer one AND by the network lane, and each has a matched control
// in the older tests (the same change without the colliding record is refused).

// dhBriefN is a brief file for sdlc/<num> with the given gate line and extras.
func dhBriefN(num, gateLine, extra string) string {
	return strings.Replace(strings.Replace(dhFM(gateLine, extra), dhBoardID, dhStream+"/"+num, 1), "# Brief 18", "# Brief "+num, 1)
}

func dhBriefPathN(num string) string { return "docs/streams/" + dhStream + "/brief-" + num + ".md" }

// dhRefusedBoth asserts layer one refuses (at least one problem naming want)
// and the --decision-gate lane exits non-zero naming want as REFUSED. A want
// ending in "/" names any brief of the stream (the reported spelling of a
// colliding board id is whichever record is primary).
func dhRefusedBoth(t *testing.T, root, want string) {
	t.Helper()
	p := dhLayerOne(t, root)
	if len(p) == 0 {
		t.Errorf("layer one: 0 problems, want a refusal naming %s", want)
	}
	found := false
	for _, s := range p {
		if strings.Contains(s, want) {
			found = true
		}
	}
	if len(p) > 0 && !found {
		t.Errorf("layer one refusals do not name %s: %v", want, p)
	}
	code, out := dhGate(t, root)
	if code == 0 {
		t.Errorf("--decision-gate exit 0, want a refusal of %s:\n%s", want, out)
	}
	if !strings.HasSuffix(want, "/") {
		wantContains(t, "--decision-gate report", out, want+" REFUSED")
	} else if !strings.Contains(out, " REFUSED") || !strings.Contains(out, want) {
		t.Errorf("--decision-gate report refuses nothing in %s:\n%s", want, out)
	}
}

// A drop hidden by parking the dropped brief's permanent id: on another brief
// that already had its own base counterpart. Base: sdlc/18 (gate: human, id P,
// decision issue #41, NO ruling) and sdlc/19 (gate: human, own id). The change
// deletes sdlc/18 and gives sdlc/19 the id P, leaving sdlc/19 at todo. sdlc/19
// is not the same brief as sdlc/18 (both board ids existed at the base), so
// sdlc/18 is dropped, unruled — refused.
func TestDGParkedIDHidesDrop(t *testing.T) {
	dhSeams(t, nil)
	root := dhFixture(t, map[string]string{
		dhReadmePath:  dhReadme(dhRow("18", "todo"), dhRow("19", "todo")),
		dhBriefPath:   dhFM("gate: human", "id: "+dhPermID+"\ndecision-issue: 41\n"),
		dhBrief19Path: dhBrief19("id: other-example-id\n"),
	})
	dhWrite(t, root, map[string]string{
		dhReadmePath:  dhReadme(dhRow("19", "todo")),
		dhBriefPath:   "",
		dhBrief19Path: dhBrief19("id: " + dhPermID + "\n"),
	})
	dhRefusedBoth(t, root, dhBoardID)
	wantContains(t, "layer-one refusal", strings.Join(dhLayerOne(t, root), "\n"), "drops it")
}

// The same route the other way round: the surviving brief is the landed, ruled
// one, and the unruled brief whose id it takes is the one removed.
func TestDGParkedIDHidesDropOfOther(t *testing.T) {
	dhSeams(t, nil)
	root := dhFixture(t, map[string]string{
		dhReadmePath:  dhReadme(dhRow("18", "done"), dhRow("19", "todo")),
		dhBriefPath:   dhFM("gate: human", "id: "+dhPermID+"\n"+dhRuledLines()),
		dhBrief19Path: dhBrief19("id: other-example-id\n"),
	})
	dhWrite(t, root, map[string]string{
		dhReadmePath:  dhReadme(dhRow("18", "done")),
		dhBrief19Path: "",
		dhBriefPath:   dhFM("gate: human", "id: other-example-id\n"+dhRuledLines()),
	})
	dhRefusedBoth(t, root, "sdlc/19")
}

// A second README row and brief file whose number normalises to the same board
// id (018 beside 18), listed first, hides the move of the existing unruled
// gate: human sdlc/18 to implemented.
func TestDGNumberDecoyHidesMove(t *testing.T) {
	dhSeams(t, nil)
	root := dhFixture(t, unruledBase())
	dhWrite(t, root, map[string]string{
		dhReadmePath:        dhReadme(dhRow("018", "todo"), dhRow("18", "implemented")),
		dhBriefPathN("018"): dhBriefN("018", "gate: human", "decision-issue: 43\n"),
	})
	dhRefusedBoth(t, root, dhStream+"/")
	wantContains(t, "layer-one refusal", strings.Join(dhLayerOne(t, root), "\n"), "more than one record")
}

// A new gate: human row with no decision issue rides a landed row's number
// under another spelling (018 beside a done 18).
func TestDGDecoyRidesLandedRow(t *testing.T) {
	// The forge would confirm the landed row's own ruling: it must not lift the
	// refusal of a board id that now resolves to two records.
	dhSeams(t, dhForge("Option 1 for "+dhBoardID+"."))
	root := dhFixture(t, map[string]string{
		dhReadmePath: dhReadme(dhRow("18", "implemented")),
		dhBriefPath:  dhFM("gate: human", dhRuledLines()),
	})
	dhWrite(t, root, map[string]string{
		dhReadmePath:        dhReadme(dhRow("18", "implemented"), dhRow("018", "implemented")),
		dhBriefPathN("018"): dhBriefN("018", "gate: human", ""),
	})
	dhRefusedBoth(t, root, dhStream+"/")
	wantContains(t, "layer-one refusal", strings.Join(dhLayerOne(t, root), "\n"), "more than one record")
	// Refused as unrulable, not merely as unruled: no record's ruling, the
	// landed row's included, can stand for the colliding board id.
	_, out := dhGate(t, root)
	wantContains(t, "--decision-gate report", out, "each stream its own name across docs/streams and docs/archive")
}

// A new gate: human brief hidden behind a non-human decoy file whose number
// spelling sorts first: both brief-018.md (gate: model) and brief-18.md
// (gate: human) are new, and row 18 goes straight to implemented. The second
// file's gate: human must still be read.
func TestDGModelDecoyHidesHuman(t *testing.T) {
	dhSeams(t, nil)
	root := dhFixture(t, map[string]string{
		dhReadmePath:       dhReadme(dhRow("19", "todo")),
		dhBriefPathN("19"): dhBriefN("19", "gate: model", ""),
	})
	dhWrite(t, root, map[string]string{
		dhReadmePath:        dhReadme(dhRow("18", "implemented"), dhRow("19", "todo")),
		dhBriefPathN("018"): dhBriefN("018", "gate: model", ""),
		dhBriefPath:         dhFM("gate: human", ""),
	})
	dhRefusedBoth(t, root, dhStream+"/")
}

// The colliding record on the BASE side: a base board that already carries two
// rows for one board id (an implemented 018 listed before a todo 18) is cleaned
// up by the change, which also moves sdlc/18 to implemented. The move is judged
// against the lowest base status for that id, so it is a move, unruled — refused.
func TestDGBaseDecoyRemovedWithMove(t *testing.T) {
	dhSeams(t, nil)
	root := dhFixture(t, map[string]string{
		dhReadmePath:        dhReadme(dhRow("018", "implemented"), dhRow("18", "todo")),
		dhBriefPathN("018"): dhBriefN("018", "gate: human", "decision-issue: 43\n"),
		dhBriefPath:         dhFM("gate: human", "decision-issue: 41\n"),
	})
	dhWrite(t, root, map[string]string{
		dhReadmePath:        dhReadme(dhRow("18", "implemented")),
		dhBriefPathN("018"): "",
	})
	dhRefusedBoth(t, root, dhBoardID)
}

// A new active stream rides an archived stream's name: the archived sdlc/18 is
// a landed gate: human brief; the change adds docs/streams/sdlc with a new
// gate: human brief 18 at implemented and no decision issue.
func TestDGArchivedNameReused(t *testing.T) {
	dhSeams(t, nil)
	root := dhFixture(t, map[string]string{
		"docs/archive/" + dhStream + "/README.md":        dhReadme(dhRow("18", "implemented")),
		"docs/archive/" + dhStream + "/done/brief-18.md": dhFM("gate: human", dhRuledLines()),
	})
	dhWrite(t, root, map[string]string{
		dhReadmePath: dhReadme(dhRow("18", "implemented")),
		dhBriefPath:  dhFM("gate: human", "id: new-example-id\n"),
	})
	dhRefusedBoth(t, root, dhBoardID)
	wantContains(t, "layer-one refusal", strings.Join(dhLayerOne(t, root), "\n"), "docs/streams and docs/archive")
}

// A new active brief file rides an archived README row under the same stream
// name: the archive holds a landed row for sdlc/18 with no brief file, and the
// change adds a gate: human brief-18.md under docs/streams with no README row.
// One row and one file, but from two trees: the new brief must not read as
// already landed.
func TestDGArchivedRowLendsStatus(t *testing.T) {
	dhSeams(t, nil)
	root := dhFixture(t, map[string]string{
		"docs/archive/" + dhStream + "/README.md": dhReadme(dhRow("18", "done")),
	})
	dhWrite(t, root, map[string]string{
		dhBriefPath: dhFM("gate: human", "id: new-example-id\n"),
	})
	dhRefusedBoth(t, root, dhBoardID)
}

// One base brief cannot stand behind two briefs after the change: sdlc/18
// (gate: human, id P, RULED on its own issue) is deleted and two new rows,
// sdlc/19 and sdlc/20, both take id P and move to implemented with no decision
// issue of their own. Neither is THE renumber of sdlc/18, so neither inherits
// its ruling — both refused, even though the forge confirms sdlc/18's ruling.
func TestDGOneRulingTwoRenumbers(t *testing.T) {
	dhSeams(t, dhForge("Option 1 for "+dhBoardID+"."))
	root := dhFixture(t, map[string]string{
		dhReadmePath: dhReadme(dhRow("18", "todo")),
		dhBriefPath:  dhFM("gate: human", "id: "+dhPermID+"\n"+dhRuledLines()),
	})
	dhWrite(t, root, map[string]string{
		dhReadmePath:       dhReadme(dhRow("19", "implemented"), dhRow("20", "implemented")),
		dhBriefPath:        "",
		dhBriefPathN("19"): dhBriefN("19", "gate: human", "id: "+dhPermID+"\n"),
		dhBriefPathN("20"): dhBriefN("20", "gate: human", "id: "+dhPermID+"\n"),
	})
	dhRefusedBoth(t, root, "sdlc/19")
	_, out := dhGate(t, root)
	wantContains(t, "--decision-gate report", out, "sdlc/20 REFUSED")
}

// The mirror: two base briefs sharing a permanent id (a ruled, done sdlc/18 and
// an unruled sdlc/19 at todo) are both removed and ONE new row takes the id at
// done. It is the renumber of neither, so it cannot borrow sdlc/18's ruling or
// its done status, and the unruled sdlc/19 is dropped — refused.
func TestDGTwoBasesOneRenumber(t *testing.T) {
	dhSeams(t, dhForge("Option 1 for "+dhBoardID+"."))
	root := dhFixture(t, map[string]string{
		dhReadmePath:  dhReadme(dhRow("18", "done"), dhRow("19", "todo")),
		dhBriefPath:   dhFM("gate: human", "id: "+dhPermID+"\n"+dhRuledLines()),
		dhBrief19Path: dhBrief19("id: " + dhPermID + "\n"),
	})
	dhWrite(t, root, map[string]string{
		dhReadmePath:       dhReadme(dhRow("20", "done")),
		dhBriefPath:        "",
		dhBrief19Path:      "",
		dhBriefPathN("20"): dhBriefN("20", "gate: human", "id: "+dhPermID+"\n"),
	})
	dhRefusedBoth(t, root, "sdlc/20")
	_, out := dhGate(t, root)
	wantContains(t, "--decision-gate report", out, "sdlc/19 REFUSED")
}

// Pin-bump control: a base board that already carries a colliding record for a
// gate: human board id, left exactly as it is by a change elsewhere, turns
// nothing red.
func TestDGUnchangedCollisionIsQuiet(t *testing.T) {
	dhSeams(t, nil)
	root := dhFixture(t, map[string]string{
		dhReadmePath:                   dhReadme(dhRow("018", "todo"), dhRow("18", "implemented")),
		dhBriefPathN("018"):            dhBriefN("018", "gate: human", "decision-issue: 43\n"),
		dhBriefPath:                    dhFM("gate: human", "decision-issue: 41\n"),
		"docs/streams/other/README.md": dhReadme(dhRow("1", "todo")),
	})
	dhWrite(t, root, map[string]string{"docs/streams/other/README.md": dhReadme(dhRow("1", "in-progress"))})
	if p := dhLayerOne(t, root); len(p) != 0 {
		t.Fatalf("an untouched collision turned red: %v", p)
	}
	if code, out := dhGate(t, root); code != 0 {
		t.Fatalf("an untouched collision turned the network lane red (exit %d):\n%s", code, out)
	}
}

// Class guard (the snapshot side): every README row and every brief file the
// snapshot reads is accounted for in exactly one board id's Records — nothing
// read is skipped as a duplicate, so no record can sit unseen behind another —
// and every board id with more than one record is marked ambiguous.
func TestDGEveryRecordAccounted(t *testing.T) {
	dhSeams(t, nil)
	root := dhFixture(t, map[string]string{
		dhReadmePath:        dhReadme(dhRow("018", "todo"), dhRow("18", "implemented"), dhRow("19", "todo")),
		dhBriefPathN("018"): dhBriefN("018", "gate: human", "decision-issue: 43\n"),
		dhBriefPath:         dhFM("gate: human", "decision-issue: 41\n"),
		"docs/streams/" + dhStream + "/done/brief-19.md":  dhBrief19(""),
		"docs/archive/" + dhStream + "/README.md":         dhReadme(dhRow("20", "done")),
		"docs/archive/" + dhStream + "/done/brief-019.md": dhBriefN("019", "gate: human", ""),
	})
	snap := gateSnapshotOnDisk(root)
	// 3 + 1 rows, 2 + 1 + 1 brief files.
	const wantRecords = 8
	seen := map[string]string{}
	total := 0
	for k, gb := range snap {
		for _, r := range gb.Records {
			if other, dup := seen[r]; dup {
				t.Errorf("record %q accounted under both %s and %s", r, other, k)
			}
			seen[r] = k
			total++
		}
		if len(gb.Records) > 1 && gb.Ambiguous == "" {
			t.Errorf("%s has %d records but is not marked ambiguous: %v", k, len(gb.Records), gb.Records)
		}
	}
	if total != wantRecords {
		t.Errorf("snapshot accounts for %d records, want every one of the %d read: %v", total, wantRecords, seen)
	}
	for _, k := range []string{dhBoardID, "sdlc/19"} {
		if snap[k] == nil || snap[k].Ambiguous == "" {
			t.Errorf("%s collides (two spellings, or both trees) but is not marked ambiguous", k)
		}
	}
	if snap["sdlc/20"] == nil || snap["sdlc/20"].Ambiguous != "" {
		t.Errorf("sdlc/20 has one record and must not be ambiguous: %+v", snap["sdlc/20"])
	}
	if b := snap[dhBoardID]; b != nil && (b.Status != "todo" || b.DecisionIssue != 0) {
		t.Errorf("ambiguous %s must judge from the lowest row status with no ruling, got status %q decision issue %d", dhBoardID, b.Status, b.DecisionIssue)
	}
}

// Class guard (the judge side): one base record stands behind at most one
// brief after the change, and one brief after it is the same brief as at most
// one base record — over every route's base and head.
func TestDGSameBriefOneToOne(t *testing.T) {
	type side = map[string]string
	cases := []struct{ base, head side }{
		{ // one base id, two renumbers
			side{dhReadmePath: dhReadme(dhRow("18", "todo")), dhBriefPath: dhFM("gate: human", "id: "+dhPermID+"\n")},
			side{dhReadmePath: dhReadme(dhRow("19", "todo"), dhRow("20", "todo")), dhBriefPath: "",
				dhBriefPathN("19"): dhBriefN("19", "gate: human", "id: "+dhPermID+"\n"),
				dhBriefPathN("20"): dhBriefN("20", "gate: human", "id: "+dhPermID+"\n")},
		},
		{ // two base ids, one renumber
			side{dhReadmePath: dhReadme(dhRow("18", "todo"), dhRow("19", "todo")),
				dhBriefPath: dhFM("gate: human", "id: "+dhPermID+"\n"), dhBrief19Path: dhBrief19("id: " + dhPermID + "\n")},
			side{dhReadmePath: dhReadme(dhRow("20", "todo")), dhBriefPath: "", dhBrief19Path: "",
				dhBriefPathN("20"): dhBriefN("20", "gate: human", "id: "+dhPermID+"\n")},
		},
		{ // a true renumber pairs
			side{dhReadmePath: dhReadme(dhRow("18", "todo")), dhBriefPath: dhFM("gate: human", "id: "+dhPermID+"\n")},
			side{dhReadmePath: dhReadme(dhRow("19", "todo")), dhBriefPath: "",
				dhBriefPathN("19"): dhBriefN("19", "gate: human", "id: "+dhPermID+"\n")},
		},
	}
	for i, c := range cases {
		dhSeams(t, nil)
		root := dhFixture(t, c.base)
		dhWrite(t, root, c.head)
		base, head := mustGateAtRev(t, root), gateSnapshotOnDisk(root)
		byPerm := map[string][]*gateBrief{}
		for _, b := range base {
			for _, id := range gatePermIDs(b) {
				byPerm[id] = append(byPerm[id], b)
			}
		}
		perBase := map[*gateBrief]int{}
		perHead := map[*gateBrief]int{}
		for p := range gateSameBriefPairs(base, head, byPerm) {
			perBase[p.b]++
			perHead[p.h]++
		}
		for b, n := range perBase {
			if n > 1 {
				t.Errorf("case %d: base %s stands behind %d briefs after the change", i, b.Key, n)
			}
		}
		for h, n := range perHead {
			if n > 1 {
				t.Errorf("case %d: %s is the same brief as %d base records", i, h.Key, n)
			}
		}
		if i == 2 && len(perHead) != 1 {
			t.Errorf("case %d: a one-to-one renumber must pair, got %d pairs", i, len(perHead))
		}
	}
}
