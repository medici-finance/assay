package main

// witnessv2_attribution_test.go — statusgen/36 Verify rows 16 and 18: the
// identity reads keep their answers on a v2 cell, and the Evidence Command
// cell is bound the same way by coverage, the Command audit and the witness
// gate.

import (
	"strings"
	"testing"

	"github.com/medici-finance/assay/statusgen/evaluation"
)

// gv2RecordIDStub is a well-formed record id for the pure identity-read table
// (the reads never resolve it); low-entropy by construction.
var gv2RecordIDStub = strings.Repeat("ab", 32)

type gv2Identity struct {
	name     string
	identity string // the token before `@`
	tail     string // the parentheticals after the tree token
	// The absolute answers each read must give, on the v1 and the v2 cell.
	implementer bool
	independent bool // evidenceHasIndependentRow over the row
	belowFloor  bool
	human       string // humanRunnerName's name; "" when not a human stamp
	isHuman     bool
	key         string // runnerKey of the identity token
	clears      bool   // runnerClearsFloor
}

var gv2Identities = []gv2Identity{
	{name: "implementer", identity: "implementer-worker-app", tail: " (forge-identity)",
		implementer: true, independent: false, key: "implementer-worker-app", clears: true},
	{name: "verifier App", identity: "assay-verifier-app[bot]", tail: " (forge-identity)",
		independent: true, key: "assay-verifier-app", clears: true},
	{name: "App on behalf of a human", identity: "assay-verifier-app[bot]", tail: " (on-behalf-of human:ada) (forge-identity)",
		independent: true, key: "assay-verifier-app", clears: true},
	{name: "human runner", identity: "human:alex", tail: "",
		independent: true, human: "alex", isHuman: true, key: "human:alex", clears: true},
	{name: "below-floor runner", identity: "sonnet-verifier", tail: " (git-config)",
		independent: true, belowFloor: true, key: "sonnet-verifier", clears: false},
}

func gv2Cells(id gv2Identity) (v1, v2 string) {
	return id.identity + " @ 0a1b2c3" + id.tail,
		id.identity + " @ [w2] cr:" + gv2CR + " rec:" + gv2RecordIDStub + id.tail
}

func gv2WitnessRow(rowID, command, cell string) string {
	return "| " + rowID + " | `" + command + "` | pass exit=0 | " + gv2Output + " | " + gv2Date + " | " + cell + " |"
}

// regression: #2026 — a v2 cell must not change who the reads say ran a row.
func TestWitnessV2AttributionUnchanged(t *testing.T) {
	for _, id := range gv2Identities {
		v1, v2 := gv2Cells(id)
		if p := evaluation.ParseRunnerCell(v2); p.Schema != evaluation.WitnessSchemaV2 || strings.TrimSpace(p.Identity) != id.identity {
			t.Fatalf("%s: the v2 cell parses as schema %d identity %q", id.name, p.Schema, p.Identity)
		}
		for _, c := range []struct{ form, cell string }{{"v1", v1}, {"v2", v2}} {
			row := gv2WitnessRow("1", "true", c.cell)
			section := witnessHeader + "\n" + row
			ers := parseEvidenceRows(section)["1"]
			if len(ers) != 1 {
				t.Fatalf("%s %s: parseEvidenceRows read %d rows", id.name, c.form, len(ers))
			}
			runner := ers[0].Runner // the full Runner cell, as every caller reads it
			if got := implementerAttributed(runner); got != id.implementer {
				t.Errorf("%s %s: implementerAttributed = %v, want %v", id.name, c.form, got, id.implementer)
			}
			if got := evidenceHasIndependentRow(section); got != id.independent {
				t.Errorf("%s %s: evidenceHasIndependentRow = %v, want %v", id.name, c.form, got, id.independent)
			}
			if got := belowFloorRunner(runner); got != id.belowFloor {
				t.Errorf("%s %s: belowFloorRunner = %v, want %v", id.name, c.form, got, id.belowFloor)
			}
			if got := runnerClearsFloor(runner); got != id.clears {
				t.Errorf("%s %s: runnerClearsFloor = %v, want %v", id.name, c.form, got, id.clears)
			}
			name, ok := humanRunnerName(runner)
			if ok != id.isHuman || name != id.human {
				t.Errorf("%s %s: humanRunnerName = (%q, %v), want (%q, %v)", id.name, c.form, name, ok, id.human, id.isHuman)
			}
			tok := strings.TrimSpace(evaluation.ParseRunnerCell(c.cell).Identity)
			if got := runnerKey(tok); got != id.key {
				t.Errorf("%s %s: runnerKey(identity) = %q, want %q", id.name, c.form, got, id.key)
			}
			// runnerKey over the full cell, as the Verified-cell agreement reads it.
			if !runnersAgree(id.identity, runner) {
				t.Errorf("%s %s: the Verified cell %q does not agree with the Evidence runner %q", id.name, c.form, id.identity, runner)
			}
			if runnersAgree("unrelated-app", runner) {
				t.Errorf("%s %s: an unrelated Verified cell agrees with %q", id.name, c.form, runner)
			}
			// A v2 cell is never read as its runner class.
			if strings.Contains(runnerKey(runner), evaluation.WitnessRunnerContainer) || runnerKey(tok) == "[w2]" {
				t.Errorf("%s %s: the runner key %q reads the runner class or the marker", id.name, c.form, runnerKey(runner))
			}
		}
	}

	// `--coverage --json` reports the same runner and independent values for
	// the v1 and v2 cell of each identity, and evidence_actor is the
	// Evidence-actor check's verdict on the commit that authored the rows.
	bin := buildStatusgenBinary(t)
	for _, author := range []struct {
		name, email, want string
	}{
		{fixtureVerifierName, fixtureVerifierEmail, evidenceActorVerifier},
		{fixtureWorkerName, fixtureWorkerEmail, evidenceActorOther},
	} {
		f := newGitV2Fixture(t)
		f.AuthorName, f.AuthorEmail = author.name, author.email
		f.Rows, f.Wits = nil, nil
		n := 0
		for _, id := range gv2Identities {
			for _, form := range []string{"v1", "v2"} {
				n++
				rid := itoaRow(n)
				f.Rows = append(f.Rows, gv2Row{ID: rid, Class: "check:ci", Command: "cat tools/x/pkg/a.go", Expect: "exit 0"})
				w := gv2Witness{Row: rid, Identity: id.identity, Tail: id.tail}
				if form == "v1" {
					w.V1Tree = "0a1b2c3"
				}
				f.Wits = append(f.Wits, w)
			}
		}
		f.build()
		policy := evidenceActorPolicyFromRoster()
		verdict, _ := policy.classify(author.name, author.email)
		wantActor := evidenceActorOther
		if verdict == actorVerifier {
			wantActor = evidenceActorVerifier
		}
		if wantActor != author.want {
			t.Fatalf("the Evidence-actor check classifies %s as %v, the fixture expects %s", author.name, verdict, author.want)
		}
		rows := f.cliCoverage(bin)
		n = 0
		for _, id := range gv2Identities {
			var got [2]*coverageWitnessJSON
			for i := range got {
				n++
				cl := cliClaimFor(t, rows, itoaRow(n))
				if cl.Witness == nil {
					t.Fatalf("%s row %d: no witness field", id.name, n)
				}
				got[i] = cl.Witness
			}
			if got[0].Schema != 1 || got[1].Schema != 2 {
				t.Errorf("%s: schemas %d/%d, want 1/2", id.name, got[0].Schema, got[1].Schema)
			}
			for i, w := range got {
				if w.Runner != id.identity {
					t.Errorf("%s form %d: runner %q, want %q", id.name, i+1, w.Runner, id.identity)
				}
				wantIndependent := id.independent && id.clears
				if w.Independent != wantIndependent {
					t.Errorf("%s form %d: independent %v, want %v", id.name, i+1, w.Independent, wantIndependent)
				}
				if w.EvidenceActor != wantActor {
					t.Errorf("%s form %d: evidence_actor %q, want %q", id.name, i+1, w.EvidenceActor, wantActor)
				}
			}
			if got[0].LandingCR == nil || *got[0].LandingCR != gv2CR {
				t.Errorf("%s: the v1 claim's landing_cr is %v, want %q", id.name, got[0].LandingCR, gv2CR)
			}
			if got[1].LandingCR != nil || got[1].CR != gv2CR || got[1].Record == "" {
				t.Errorf("%s: the v2 claim reports landing_cr %v, cr %q, record %q", id.name, got[1].LandingCR, got[1].CR, got[1].Record)
			}
		}
	}
}

func itoaRow(n int) string {
	return strings.TrimLeft(string([]byte{byte('0' + n/10), byte('0' + n%10)}), "0")
}

// regression: #2026 — the Evidence Command cell binds a v2 witness in
// coverage, in the Command audit and in the witness gate alike.
func TestWitnessV2CommandCellAgrees(t *testing.T) {
	bin := buildStatusgenBinary(t)
	const oldCmd, newCmd, adapted = "cat tools/x/pkg/a.go", "cat tools/x/pkg/b.go", "cat ./tools/x/pkg/a.go"

	// Consistent: the record digests its own Evidence Command cell; the Verify
	// row's Command changed after the run.
	consistent := newGitV2Fixture(t)
	consistent.Rows = []gv2Row{{ID: "1", Class: "check:ci", Command: oldCmd, Expect: "exit 0"}}
	consistent.Wits = []gv2Witness{{Row: "1", Identity: "verifier-app", Tail: " (forge-identity)"}}
	consistent.build()
	changed := consistent.briefWithRows([]gv2Row{{ID: "1", Class: "check:ci", Command: newCmd, Expect: "exit 0"}})
	gv2Write(t, consistent.Root, gv2BriefRel(), changed)
	gv2Git(t, consistent.Root, "add", "-A")
	gv2Git(t, consistent.Root, "commit", "-q", "-m", "fixture: the Verify command changes")

	// Inconsistent: the record digests the current Verify Command, but its own
	// Evidence Command cell names an adapted command.
	inconsistent := newGitV2Fixture(t)
	inconsistent.Rows = []gv2Row{{ID: "1", Class: "check:ci", Command: oldCmd, Expect: "exit 0"}}
	inconsistent.Wits = []gv2Witness{{Row: "1", Identity: "verifier-app", Tail: " (forge-identity)", EvidenceCommand: adapted, RecordCommand: oldCmd}}
	inconsistent.build()

	// The positive control: the same shape with nothing changed releases.
	control := newGitV2Fixture(t)
	control.Rows = []gv2Row{{ID: "1", Class: "check:ci", Command: oldCmd, Expect: "exit 0"}}
	control.Wits = []gv2Witness{{Row: "1", Identity: "verifier-app", Tail: " (forge-identity)"}}
	control.build()

	for _, tc := range []struct {
		name    string
		f       *gitV2Fixture
		gap     string
		refuses bool
	}{
		{"consistent record, Command changed after the run", consistent, evaluation.GapCommandChanged, true},
		{"inconsistent record", inconsistent, evaluation.GapCommandCellMismatch, true},
		{"unaltered control", control, "", false},
	} {
		// 1. --coverage --json (the built binary).
		cl := cliClaimFor(t, tc.f.cliCoverage(bin), "1")
		if tc.refuses {
			if cl.Claim.Released || cl.Claim.Result != covError || !strings.HasPrefix(cl.Claim.Reason, tc.gap) {
				t.Errorf("%s: --coverage --json gives %s released=%v: %s; want error starting %s", tc.name, cl.Claim.Result, cl.Claim.Released, cl.Claim.Reason, tc.gap)
			}
			if cl.Witness == nil || cl.Witness.Gap != tc.gap {
				t.Errorf("%s: the witness field's gap is %+v, want %s", tc.name, cl.Witness, tc.gap)
			}
		} else if !cl.Claim.Released {
			t.Errorf("%s: --coverage --json refuses: %s", tc.name, cl.Claim.Reason)
		}

		// 2. checkWitnesses, the Command audit `--check` runs.
		bf := mustParseBriefFile(t, tc.f.BriefPath)
		var finding checkFinding
		for _, fd := range checkWitnesses(bf.Verify, bf.Evidence) {
			if fd.ID == "1" {
				finding = fd
			}
		}
		if tc.refuses && finding.State == statePass {
			t.Errorf("%s: checkWitnesses passes the row", tc.name)
		}
		if !tc.refuses && finding.State != statePass {
			t.Errorf("%s: checkWitnesses refuses the control: %+v", tc.name, finding)
		}

		// 3. The witness gate's latestFailFirst.
		var command string
		for _, r := range briefVerifyRows(bf.Verify) {
			if r.ID == "1" {
				command = r.Command
			}
		}
		_, _, stale := latestFailFirst(parseEvidenceRows(bf.Evidence)["1"], command)
		if stale != tc.refuses {
			t.Errorf("%s: latestFailFirst stale = %v, want %v", tc.name, stale, tc.refuses)
		}
	}
}
