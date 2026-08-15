package main

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

const goodActions = `{
  "asOf": "2026-08-13T12:00:00Z",
  "stale": false,
  "scope": {"repos": ["example-org/example-repo"], "count": 1, "source": "compiled-in fixed set (deskkit)"},
  "prPopulation": {"complete": true, "cap": 100},
  "rows": [
    {"repo": "example-org/example-repo", "number": 42, "title": "t", "action": "NEEDS-REVIEW", "score": 9, "note": "n"},
    {"repo": "example-org/example-repo", "number": 43, "title": "u", "action": "READY", "score": 2, "note": "n"}
  ],
  "tombstones": [],
  "external": []
}`

const goodPRs = `{"prs": [
  {"repo": "example-org/example-repo", "number": 42, "headSHA": "aaaaaaaaaaaabbbbbbbbbbbb"}
]}`

// TestReadBoard_JoinsHeadsFromPRs — the two-verb join. `actions` has the ACTION, `prs` has
// the head; a row present in both gets a real key, a row missing from `prs` gets
// HeadUnresolved rather than an empty string that would compare equal to nothing.
func TestReadBoard_JoinsHeadsFromPRs(t *testing.T) {
	b, err := ReadBoard([]byte(goodActions), []byte(goodPRs))
	if err != nil {
		t.Fatalf("ReadBoard: %v", err)
	}
	if len(b.Rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(b.Rows))
	}
	var joined, unresolved int
	for _, r := range b.Rows {
		if r.Head == HeadUnresolved {
			unresolved++
		} else {
			joined++
		}
	}
	if joined != 1 || unresolved != 1 {
		t.Fatalf("joined=%d unresolved=%d, want 1/1", joined, unresolved)
	}
	if got := b.UnresolvedHeads(); len(got) != 1 || got[0] != "example-org/example-repo#43" {
		t.Fatalf("UnresolvedHeads = %v, want the one unjoined row named", got)
	}
}

// TestReadBoard_EmptyIsBlindNotIdle is the fleet-observed trap: ~16+ concurrent
// gh-hitting agents on the shared token trip GitHub's secondary limit, deskboard exits 6,
// and its stdout is empty. A reader that decoded that into "0 rows, 0 actionable" would
// report all-clear from an outage.
func TestReadBoard_EmptyIsBlindNotIdle(t *testing.T) {
	for _, in := range []string{"", "   \n", "\t"} {
		_, err := ReadBoard([]byte(in), nil)
		if err == nil {
			t.Fatalf("empty actions payload %q produced a Board — an empty sweep is BLIND, not idle", in)
		}
		if got := deskkit.ExitCodeOf(err); got != deskkit.ExitUnverifiable {
			t.Fatalf("exit code = %d, want %d", got, deskkit.ExitUnverifiable)
		}
		// The DIAGNOSTIC is load-bearing, not just the exit code. A JSON decode error
		// would also fail closed here, and it would send the operator to look for a
		// corrupt payload — but "the sweep produced nothing" and "the sweep produced
		// garbage" have different remedies, and the first one is what a secondary-rate-
		// limit 403 on the shared token looks like. Collapsing them costs the reader the
		// actual next step.
		if !strings.Contains(err.Error(), "produced no output") {
			t.Fatalf("an EMPTY sweep must be diagnosed as producing no output, not as a decode failure; got: %v", err)
		}
	}
}

// TestReadBoard_NoRowsIsStillNotIdle — the subtler half. A WELL-FORMED payload with an
// empty rows array is a legitimate quiet board, so it must read fine; the idle decision is
// then the gate's, not the parser's. This is the positive control that stops the empty
// check above from degenerating into "refuse everything quiet".
func TestReadBoard_NoRowsIsStillNotIdle(t *testing.T) {
	quiet := strings.Replace(goodActions, `"rows": [
    {"repo": "example-org/example-repo", "number": 42, "title": "t", "action": "NEEDS-REVIEW", "score": 9, "note": "n"},
    {"repo": "example-org/example-repo", "number": 43, "title": "u", "action": "READY", "score": 2, "note": "n"}
  ]`, `"rows": []`, 1)
	b, err := ReadBoard([]byte(quiet), nil)
	if err != nil {
		t.Fatalf("a well-formed quiet board must still parse: %v", err)
	}
	if v := Idle(b, at(t, fixedNow)); v.State != IdleYes {
		t.Fatalf("state = %s (%v), want IDLE — a genuinely quiet, complete, fresh board", v.State, v.BlindReasons)
	}
}

// TestReadBoard_RejectsMissingAsOf — without the sweep's own timestamp, freshness cannot
// be checked, so the board cannot support an idle claim at all.
func TestReadBoard_RejectsMissingAsOf(t *testing.T) {
	_, err := ReadBoard([]byte(`{"rows": []}`), nil)
	if err == nil {
		t.Fatal("a payload with no asOf produced a Board — its freshness is uncheckable")
	}
	if !strings.Contains(err.Error(), "asOf") {
		t.Fatalf("error does not name the missing field: %v", err)
	}
}

// TestReadBoard_UnknownActionFailsTheWholeRead — one unrecognised ACTION poisons the read
// rather than being skipped. A skipped row is a board surface that vanished.
func TestReadBoard_UnknownActionFailsTheWholeRead(t *testing.T) {
	bad := strings.Replace(goodActions, `"action": "READY"`, `"action": "TOTALLY-NEW-STATE"`, 1)
	_, err := ReadBoard([]byte(bad), nil)
	if err == nil {
		t.Fatal("an unknown ACTION was silently accepted")
	}
	if got := deskkit.ExitCodeOf(err); got != deskkit.ExitUnverifiable {
		t.Fatalf("exit code = %d, want %d", got, deskkit.ExitUnverifiable)
	}
}

// TestReadBoard_MalformedPRsPayloadIsBlind — a broken head-SHA half must not degrade
// silently into "no heads found", which would suppress every verb as could-not-check while
// looking like an ordinary quiet pass.
func TestReadBoard_MalformedPRsPayloadIsBlind(t *testing.T) {
	_, err := ReadBoard([]byte(goodActions), []byte(`{"prs": [ not json`))
	if err == nil {
		t.Fatal("a malformed prs payload was accepted")
	}
	if got := deskkit.ExitCodeOf(err); got != deskkit.ExitUnverifiable {
		t.Fatalf("exit code = %d, want %d", got, deskkit.ExitUnverifiable)
	}
}

// TestReactorDoesNotImplementTheDrainContract is the brief's Verify row 4 as a compiled
// test rather than only a grep: no file in this package may reference loopengine at all.
// The reactor and the drain engine share vocabulary in prose and nothing in code.
func TestReactorDoesNotImplementTheDrainContract(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	fset := token.NewFileSet()
	checked := 0
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") {
			continue
		}
		f, err := parser.ParseFile(fset, filepath.Clean(e.Name()), nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("parse %s: %v", e.Name(), err)
		}
		checked++
		for _, imp := range f.Imports {
			if strings.Contains(imp.Path.Value, "internal/loopengine") {
				t.Errorf("%s imports the drain engine (%s) — the board reactor must not take the drain contract, "+
					"and the brief's title is the constraint: formalize, do NOT drain-ify", e.Name(), imp.Path.Value)
			}
		}
	}
	if checked < 5 {
		t.Fatalf("only %d Go files inspected — the scan broke, which would make this test vacuous", checked)
	}
}
