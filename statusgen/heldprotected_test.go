package main

import (
	"os"
	"strings"
	"testing"
)

// The #5541 shape: a brief whose Evidence carries a protected-table marker
// (Verification-Table:, or an Acceptance-Transition:) records an earlier run
// that ends HELD / could-not-check, then a later entry that re-runs every row
// clean and closes on the strict PASS. The protected witness audit reads only
// that latest PASS run, so rule 3c (read the latest PASS run only) applies to
// it the way it applies to an ordinary record. These tests use only
// identifiers the code had before the fix, so they run red on it unchanged.

// hpOldHold is the earlier run's hold as #5541's brief records it: a prose
// line naming a row and a could-not-check exit, then a BLOCKED verdict.
const hpOldHold = "Row 2 (bracket 2). The witness records `fail exit=2`. Exit 2 is the gate's COULD-NOT-CHECK exit, so row 2 is held.\n\n" +
	"**VERIFY: BLOCKED** — row 2 could not run.\n\n"

// hpFixture lays the protected two-run brief on disk (see
// TestProtectedRerunConsumers for the delivery shape, without its
// supplemental row) and returns its root, README, brief path, Verify table,
// Evidence and delivery. latest is the latest entry's body below its marker;
// nil uses every row passing.
func hpFixture(t *testing.T, latest func(rows, extra string) string) (root, readme, path, verify, evidence, delivery string) {
	t.Helper()
	o := vfDefaults()
	o.evidence = ""
	root, readme = vfFixture(t, o)
	path = vfBrief(root)
	base := vfRev(t, root, "HEAD")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	verify = vfVerify
	bare := strings.Replace(string(raw), "# Brief 01\n", "# Brief 01 (delivered)\n", 1)
	if err := os.WriteFile(path, []byte(bare), 0o644); err != nil {
		t.Fatal(err)
	}
	vfCommit(t, root, vfMailWorker)
	delivery = vfRev(t, root, "HEAD")
	marker := "Verification-Table: delivery=" + delivery + " base=" + base + " original=" + base + "\n\n"
	rows := strings.ReplaceAll(witnessTableFor(vfRow1, vfRow2), vfSHA, delivery)
	extra := "\n"
	// The older run's row 2 splits on an unescaped "|", so no row key reads
	// from it and the row cover (rule 3a) cannot discharge it.
	older := strings.ReplaceAll(strings.Replace(rows, "| `false` | pass exit=1 |", "| `false | cat` | could-not-check exit=-1 |", 1), "2026-08-13", "2026-08-10") + extra
	body := rows + extra + "\n" + vfPass + "\n"
	if latest != nil {
		body = latest(rows, extra)
	}
	evidence = "### Non-implementer run — 2026-08-10\n\n" + marker + older + "\n" + hpOldHold +
		"### Non-implementer re-run — 2026-08-13\n\n" + marker + body
	if err := os.WriteFile(path, []byte(strings.Replace(bare, "## Evidence\n", "## Evidence\n"+evidence, 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	vfCommit(t, root, vfMailVerifier)
	resetReadMemo()
	vfOutcomeCommitted(t, root, "verified", delivery, "2026-08-14T12:00:00Z")
	return root, readme, path, verify, evidence, delivery
}

// TestVerifyflipProtectedOldHold: the earlier run's hold does not stop the
// flip of a protected record whose latest run passes every row, and the
// flipped record still passes brief --check-verified.
// regression: #5541
func TestVerifyflipProtectedOldHold(t *testing.T) {
	root, readme, path, verify, evidence, delivery := hpFixture(t, nil)
	if !heldScanScope(evidence, true).latest {
		t.Errorf("rule 3c is off for a protected record whose latest run passes")
	}
	if held, why := unroutedHeldLine(evidence, true); held {
		t.Errorf("the flip read finds a superseded hold: %s", why)
	}
	if held, why := verifyPassHeldWitnessed(evidence, path); held {
		t.Errorf("the autoflip read finds a superseded hold: %s", why)
	}
	// The plain read (close-verify, latestRun false) reads it only with rule
	// 3d off; with 3d on the later strict PASS entry supersedes it there too.
	if held, why := unroutedHeldLine(evidence, false); held {
		t.Errorf("the plain held read finds a superseded hold: %s", why)
	}
	func() {
		defer func(on bool) { heldPassRule = on }(heldPassRule)
		heldPassRule = false
		if held, _ := unroutedHeldLine(evidence, false); !held {
			t.Errorf("with rule 3d off the plain held read no longer sees the earlier hold")
		}
	}()
	if code, out, diag := runVF(t, root, "--sha", delivery); code != verifyflipExitOK {
		t.Fatalf("flip refused: code=%d out=%s diag=%s", code, out, diag)
	}
	after, err := os.ReadFile(readme)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(after), "| verified | 2026-08-13 "+vfRunner+" @ "+delivery) {
		t.Errorf("flip did not write the expected stamp: %s", after)
	}
	resetReadMemo()
	info, err := resolveBriefKey(root, "vf/01")
	if err != nil {
		t.Fatal(err)
	}
	if err := checkBriefInfoVerified(root, info); err != nil {
		t.Errorf("brief --check-verified disagrees with the flip: %v", err)
	}
	if findings, refusal := closureWitnesses(verify, evidence, path); refusal != "" || checkExitCode(findings) != verifyrunExitPass {
		t.Errorf("closure audit disagrees with the flip: %s %+v", refusal, findings)
	}
}

// TestVerifyflipProtectedLatestHold: what still refuses on a protected record
// with an earlier hold: a hold inside the latest run, a row the latest run
// did not re-run, a run at another sha than the one asked for, and a run by
// another runner than the one asked for.
// regression: #5541
func TestVerifyflipProtectedLatestHold(t *testing.T) {
	for name, c := range map[string]struct {
		latest func(rows, extra string) string
		args   func(delivery string) []string
		want   string
	}{
		"hold in the latest run": {
			latest: func(rows, extra string) string { return rows + extra + "\n" + vlHeldProse + vfPass + "\n" },
			want:   "still says HELD/could-not-check",
		},
		"latest run misses row 1": {
			latest: func(rows, extra string) string { return dropRow(rows, "| 1 |") + extra + "\n" + vfPass + "\n" },
			want:   "row 1: could-not-run",
		},
		"other sha asked": {
			args: func(string) []string { return []string{"--sha", strings.Repeat("c", 40)} },
			want: "sha mismatch",
		},
		"other runner asked": {
			args: func(d string) []string { return []string{"--sha", d, "--runner", "other-verifier[bot]"} },
			want: "refusing",
		},
	} {
		t.Run(name, func(t *testing.T) {
			root, readme, _, _, _, delivery := hpFixture(t, c.latest)
			args := []string{"--sha", delivery}
			if c.args != nil {
				args = c.args(delivery)
			}
			before, err := os.ReadFile(readme)
			if err != nil {
				t.Fatal(err)
			}
			code, out, diag := runVF(t, root, args...)
			if code != verifyflipExitRefused || !strings.Contains(diag, c.want) {
				t.Errorf("code=%d, want a refusal naming %q; out=%s diag=%s", code, c.want, out, diag)
			}
			after, err := os.ReadFile(readme)
			if err != nil {
				t.Fatal(err)
			}
			if string(after) != string(before) {
				t.Error("refused flip changed the README")
			}
		})
	}
}

// TestHeldLatestTransitionMarker: the second protected marker,
// Acceptance-Transition:, is read the same way. The protected audit narrows
// to the latest PASS run for it too, so it no longer keeps rule 3c off on its
// own; a row a flip reader credits above the latest heading still does.
// regression: #5541
func TestHeldLatestTransitionMarker(t *testing.T) {
	sha := strings.Repeat("a", 40)
	for name, marker := range map[string]string{
		"verification table":    "Verification-Table: delivery=" + sha + " base=" + sha + " original=" + sha + "\n\n",
		"acceptance transition": "Acceptance-Transition: amendment=" + sha + " approval=https://example.invalid/r\n\n",
	} {
		t.Run(name, func(t *testing.T) {
			ev := vlOldHeld() + vlLatest + marker + witnessTableFor(vfRow1, vlRow2) + "\n\n" + vfPass
			if !hasProtectedTableMarker(ev) {
				t.Fatal("fixture carries no protected marker")
			}
			if !heldScanScope(ev, true).latest {
				t.Errorf("rule 3c is off for a protected record whose latest run passes")
			}
			if held, why := unroutedHeldLine(ev, true); held {
				t.Errorf("the flip read finds a superseded hold: %s", why)
			}
			split := vlPart1 + marker + vlUTCTable(vfRow1, vlRow2) + "\n\n" + vlHeldProse + vlPart2 +
				witnessTableFor(vfRow1) + "\n\n" + vfPass
			if heldScanScope(split, true).latest {
				t.Errorf("rule 3c is on with a credited witness table above the latest heading")
			}
		})
	}
}

// dropRow returns table without its line that starts with prefix.
func dropRow(table, prefix string) string {
	var keep []string
	for _, l := range strings.Split(table, "\n") {
		if !strings.HasPrefix(l, prefix) {
			keep = append(keep, l)
		}
	}
	return strings.Join(keep, "\n")
}
