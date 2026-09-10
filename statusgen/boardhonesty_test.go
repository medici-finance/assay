package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestClassifyPhantom is the detector's PROOF IT CAN FAIL: every phantom class
// has a positive control that MUST fire and a negative control that MUST stay
// clean, so a regression that stops catching a class turns this red rather than
// going quietly green. classifyPhantom is pure, so each class is exercised in
// isolation with only the input its detector reads.
func TestClassifyPhantom(t *testing.T) {
	const id = "some-stream/03"

	cases := []struct {
		name        string
		body        string
		readme      string
		rowCell     string // the row's OWN brief-table cell — the re-homed class keys on THIS (#709)
		merged      map[string]bool
		filePresent bool   // the row's own brief file exists — gates ONLY the re-homed class (#581)
		wantClass   string // "" => must be clean
	}{
		// 1. already-merged-unflipped ------------------------------------------
		{
			name:      "merged-unflipped POSITIVE: a merge already named the brief",
			merged:    map[string]bool{id: true},
			wantClass: phantomMergedUnflipped,
		},
		{
			name:      "merged-unflipped NEGATIVE: no merge names it",
			merged:    map[string]bool{"other-stream/01": true},
			wantClass: "",
		},
		// 2. out-of-repo-deliverable -------------------------------------------
		{
			name:      "out-of-repo POSITIVE: body says the deliverable lands elsewhere",
			body:      "## Context\nThe deliverable lands in the console repo's cells config, not here.\n",
			wantClass: phantomOutOfRepo,
		},
		{
			name:      "out-of-repo NEGATIVE: an ordinary in-repo deliverable",
			body:      "## Deliverable\nAdd a check under statusgen/ in this repo.\n",
			wantClass: "",
		},
		// 3. dehoused ----------------------------------------------------------
		{
			name:      "dehoused POSITIVE (body): work de-housed to another repo",
			body:      "This brief was de-housed to the public repo by ruling.\n",
			wantClass: phantomDehoused,
		},
		{
			name:      "dehoused POSITIVE (readme): spelled 'dehoused'",
			readme:    "Status note: these rows were dehoused; owned elsewhere now.\n",
			wantClass: phantomDehoused,
		},
		{
			name:      "dehoused NEGATIVE: 'house' alone must not trip it",
			body:      "This is a house-layer methodology brief.\n",
			wantClass: "",
		},
		// 4. re-homed ----------------------------------------------------------
		// re-homed fires only on a POINTER row: the ROW's OWN cell carries a
		// re-home marker AND the brief file is GONE. filePresent=false is the
		// pointer-row case (#581); the marker lives in rowCell, never the stream
		// README (#709).
		{
			name:      "re-homed POSITIVE: the row's own cell carries the [homed→…] marker, no brief file",
			rowCell:   "Old distribution thing [homed→acme/widgets]",
			wantClass: phantomReHomed,
		},
		{
			name:      "re-homed POSITIVE: do-not-re-implement wording IN THE ROW CELL, no brief file",
			rowCell:   "~~Do the thing~~ do not re-implement — the deliverable already landed",
			wantClass: phantomReHomed,
		},
		{
			name:      "re-homed POSITIVE: a deliverable-repo: note carried on the row cell",
			rowCell:   "Ledger bridge change (deliverable-repo: acme/gadgets)",
			wantClass: phantomReHomed,
		},
		{
			// statusgen #709: a stream README mentioning re-homing (a scope note, or
			// a stream re-homed INTO this repo) must NOT flag a plain row whose OWN
			// cell carries no marker — that was the false-positive class.
			name:      "re-homed NEGATIVE (#709): README says re-homed but the ROW cell is ordinary",
			readme:    "This stream was re-homed here from the platform repo; do not re-implement the OLD rows.\n",
			rowCell:   "Add the vault balance check",
			wantClass: "",
		},
		{
			// The #581 fix still holds: a present brief file is live work, so even a
			// row whose cell HAS a marker is not flagged when its file is present.
			name:        "re-homed NEGATIVE (#581): the row's brief file is PRESENT (live re-homed-in row)",
			rowCell:     "Live brief [homed→acme/widgets]",
			body:        "## Context\nA live, dispatchable brief in this re-homed-in stream.\n",
			filePresent: true,
			wantClass:   "",
		},
		{
			name:      "re-homed NEGATIVE: an ordinary row cell",
			rowCell:   "Wave 1 brief, ready to dispatch",
			readme:    "Active stream. Wave 1 briefs are ready.\n",
			wantClass: "",
		},
		// 5. statusgen-source-elsewhere ----------------------------------------
		{
			name:      "statusgen-source POSITIVE: the source-moved banner",
			body:      "NOTE: Any statusgen SOURCE change for this brief must be made in medici-finance/assay, not here.\n",
			wantClass: phantomStatusgenSource,
		},
		{
			name:      "statusgen-source POSITIVE: the generic must-be-made-in form",
			body:      "The change must be made in the medici-finance/assay tree.\n",
			wantClass: phantomStatusgenSource,
		},
		{
			name:      "statusgen-source NEGATIVE: an ordinary statusgen brief here",
			body:      "Add a statusgen lint check in this repo's statusgen/ directory.\n",
			wantClass: "",
		},
		// 6. deferred-by-gate --------------------------------------------------
		{
			name:      "deferred-by-gate POSITIVE: STATUS DEFERRED",
			body:      "STATUS: DEFERRED / optional. Do not dispatch ahead of the gate brief.\n",
			wantClass: phantomDeferredByGate,
		},
		{
			name:      "deferred-by-gate POSITIVE: do-not-dispatch-until wording",
			body:      "Do not dispatch until the sequencing brief reaches done.\n",
			wantClass: phantomDeferredByGate,
		},
		{
			name:      "deferred-by-gate NEGATIVE: a live, un-gated brief",
			body:      "Ready to dispatch. No dependencies.\n",
			wantClass: "",
		},
		// fully clean ----------------------------------------------------------
		{
			name:      "clean: nothing matches any detector",
			body:      "## Context\nA perfectly ordinary, dispatchable brief.\n",
			readme:    "Active stream.\n",
			wantClass: "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			class, reason, ok := classifyPhantom(id, tc.body, tc.readme, tc.rowCell, tc.merged, tc.filePresent)
			if tc.wantClass == "" {
				if ok {
					t.Fatalf("want CLEAN, got class %q (reason %q)", class, reason)
				}
				return
			}
			if !ok {
				t.Fatalf("want class %q, got CLEAN", tc.wantClass)
			}
			if class != tc.wantClass {
				t.Fatalf("want class %q, got %q", tc.wantClass, class)
			}
			if reason == "" {
				t.Errorf("a phantom must carry a human reason; got empty")
			}
		})
	}
}

// TestClassifyPhantomPrecedence pins the contract that the STRONGEST, git-derived
// class wins when a row matches more than one detector: a row that is both
// already-merged and carries a text banner is reported as merged, never by the
// weaker text match.
func TestClassifyPhantomPrecedence(t *testing.T) {
	const id = "s/01"
	body := "Any statusgen SOURCE change must be made in medici-finance/assay. STATUS: DEFERRED.\n"
	readme := "This stream was re-homed.\n"
	// The re-home marker now lives in the ROW's own cell (#709); filePresent=false
	// plus this marker keeps the re-homed arm a live competing match (pointer row).
	rowCell := "Retired thing [homed→acme/widgets]"

	// filePresent=false keeps the re-homed arm a live competing match (pointer
	// row), so this proves precedence ORDER, not the #581 file-presence gate.
	// Merged wins over every text detector.
	if class, _, ok := classifyPhantom(id, body, readme, rowCell, map[string]bool{id: true}, false); !ok || class != phantomMergedUnflipped {
		t.Fatalf("merged must win: got ok=%v class=%q", ok, class)
	}
	// Without the merge, the source-moved banner (more specific) wins over the
	// deferred/re-homed matches also present.
	if class, _, ok := classifyPhantom(id, body, readme, rowCell, nil, false); !ok || class != phantomStatusgenSource {
		t.Fatalf("statusgen-source must win over deferred/re-homed: got ok=%v class=%q", ok, class)
	}
}

// writeStream materializes a stream dir with a README and one brief file so the
// driver's file reads have something real to load, and returns a *Stream whose
// Dir/Name match the layout expectedBriefID derives an id from.
func writeStream(t *testing.T, root, name, readme string, briefNum, briefBody string) *Stream {
	t.Helper()
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte(readme), 0o644); err != nil {
		t.Fatal(err)
	}
	if briefNum != "" {
		fn := filepath.Join(dir, "brief-"+briefNum+"-x.md")
		if err := os.WriteFile(fn, []byte(briefBody), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return &Stream{Name: name, Dir: dir}
}

// TestBoardHonestyNotices exercises the DRIVER: real file reads, the todo-only
// scope, the three-state git arm, and the surfaced NOTICE shape.
func TestBoardHonestyNotices(t *testing.T) {
	t.Run("surfaces a body-detected phantom, one NOTICE, names class + id + fix", func(t *testing.T) {
		root := t.TempDir()
		s := writeStream(t, root, "gtm", "Active stream.\n",
			"08", "Any statusgen SOURCE change must be made in medici-finance/assay, not here.\n")
		s.Briefs = []Brief{{Num: "08", Status: "todo"}}

		got := boardHonestyNotices([]*Stream{s}, nil, nil)
		if len(got) != 1 {
			t.Fatalf("want exactly 1 NOTICE, got %d: %v", len(got), got)
		}
		for _, want := range []string{"NON-DISPATCHABLE", phantomStatusgenSource, "gtm/08", "Fix:", "board-honesty"} {
			if !strings.Contains(got[0], want) {
				t.Errorf("NOTICE must contain %q; got:\n%s", want, got[0])
			}
		}
	})

	t.Run("only todo rows are judged", func(t *testing.T) {
		root := t.TempDir()
		s := writeStream(t, root, "gtm", "Active stream.\n",
			"08", "The deliverable lands in another repo.\n")
		// Same phantom text, but the row is implemented — past the line this draws.
		s.Briefs = []Brief{{Num: "08", Status: "implemented"}}
		if got := boardHonestyNotices([]*Stream{s}, nil, nil); len(got) != 0 {
			t.Fatalf("a non-todo row must never surface; got %v", got)
		}
	})

	t.Run("a clean todo row is silent", func(t *testing.T) {
		root := t.TempDir()
		s := writeStream(t, root, "gtm", "Active stream.\n",
			"08", "## Context\nAn ordinary dispatchable brief in this repo.\n")
		s.Briefs = []Brief{{Num: "08", Status: "todo"}}
		if got := boardHonestyNotices([]*Stream{s}, nil, nil); len(got) != 0 {
			t.Fatalf("a clean todo row must be silent; got %v", got)
		}
	})

	t.Run("re-homed pointer row: the ROW cell carries the marker AND the brief file is gone -> one NOTICE", func(t *testing.T) {
		root := t.TempDir()
		// No brief file on disk (briefNum ""): only a pointer row remains after the
		// record was re-homed OUT of this root. The marker is in the row's OWN cell
		// (#709), not just the stream README.
		s := writeStream(t, root, "distribution", "Active stream.\n", "", "")
		s.Briefs = []Brief{{Num: "07", Status: "todo", RawCell: "Old distribution row [homed→acme/widgets]"}}
		got := boardHonestyNotices([]*Stream{s}, nil, nil)
		if len(got) != 1 || !strings.Contains(got[0], phantomReHomed) {
			t.Fatalf("a re-homed pointer row with no brief file must surface; got %v", got)
		}
	})

	// The #581 fail-first case: a stream re-homed INTO this repo has "re-homed"
	// in its README (describing its own arrival) but its rows are LIVE work with
	// real brief files. The board must NOT tell the dispatcher to skip them.
	t.Run("#581: a live row of a stream re-homed INTO this repo is silent despite the README history", func(t *testing.T) {
		root := t.TempDir()
		s := writeStream(t, root, "harness-portability",
			"This stream was re-homed here from the platform repo; do not re-implement the OLD rows.\n",
			"13", "## Context\nA live, dispatchable brief in this re-homed-in stream.\n")
		s.Briefs = []Brief{{Num: "13", Status: "todo"}}
		if got := boardHonestyNotices([]*Stream{s}, nil, nil); len(got) != 0 {
			t.Fatalf("a live re-homed-in row (brief file present) must be silent; got %v", got)
		}
	})

	// #709 fail-first: the stream-level README mentions "re-homed" (a policy/
	// boundary note describing the STREAM), but a plain todo row with no brief
	// file and an ORDINARY cell is NOT re-homed. Keying re-homed on the whole
	// README flagged every such row NON-DISPATCHABLE (board-honesty false
	// positive). The classifier must key on the ROW's own re-home marker, never a
	// stream-level inference.
	t.Run("#709: a plain todo row (no file, ordinary cell) is silent even when the stream README says re-homed", func(t *testing.T) {
		root := t.TempDir()
		s := writeStream(t, root, "human-gates",
			"Scope: these are human-gated briefs. An earlier row was re-homed; do not re-implement it.\n",
			"", "")
		// An ordinary in-repo todo row: no brief file, a plain cell with no
		// re-home marker of its own.
		s.Briefs = []Brief{{Num: "05", Status: "todo", Title: "Add the vault balance check", RawCell: "Add the vault balance check"}}
		if got := boardHonestyNotices([]*Stream{s}, nil, nil); len(got) != 0 {
			t.Fatalf("a plain todo row must not be flagged re-homed on stream-level README text; got %v", got)
		}
	})

	// #709 true-positive preserved: a genuine pointer row carries the re-home
	// marker in its OWN README cell (the rendered [homed→…] marker or explicit
	// retirement text), and its brief file is gone. That still surfaces.
	t.Run("#709: a pointer row carrying [homed→…] in its own cell (no file) still surfaces re-homed", func(t *testing.T) {
		root := t.TempDir()
		s := writeStream(t, root, "distribution",
			"Active stream. Wave 1 briefs are ready.\n", "", "")
		s.Briefs = []Brief{{Num: "07", Status: "todo", Title: "Old thing", RawCell: "Old thing [homed→acme/widgets]"}}
		got := boardHonestyNotices([]*Stream{s}, nil, nil)
		if len(got) != 1 || !strings.Contains(got[0], phantomReHomed) {
			t.Fatalf("a pointer row with [homed→…] in its own cell must surface re-homed; got %v", got)
		}
	})

	t.Run("class 1 uses the merged set, and merge wins over text", func(t *testing.T) {
		root := t.TempDir()
		s := writeStream(t, root, "oss-replacement", "Active stream.\n",
			"08", "The deliverable lands in another repo.\n") // also an out-of-repo text match
		s.Briefs = []Brief{{Num: "08", Status: "todo"}}
		merged := []mergedPR{{
			Number:  1086,
			Subject: "Merge pull request #1086 from x/brief/oss-replacement-08-thing",
			Briefs:  []string{"oss-replacement/08"},
		}}
		got := boardHonestyNotices([]*Stream{s}, merged, nil)
		if len(got) != 1 || !strings.Contains(got[0], phantomMergedUnflipped) {
			t.Fatalf("a locally-merged brief must surface as merged-unflipped (wins over out-of-repo); got %v", got)
		}
	})

	t.Run("three-state: a git read error makes class 1 could-not-check, text classes still run", func(t *testing.T) {
		root := t.TempDir()
		s := writeStream(t, root, "gtm", "Active stream.\n",
			"08", "STATUS: DEFERRED. Do not dispatch ahead of the gate.\n")
		s.Briefs = []Brief{{Num: "08", Status: "todo"}}

		got := boardHonestyNotices([]*Stream{s}, nil, errNoGitForMerge)
		var cnc, phantom bool
		for _, n := range got {
			if strings.Contains(n, "could-not-check") && strings.Contains(n, phantomMergedUnflipped) {
				cnc = true
			}
			if strings.Contains(n, phantomDeferredByGate) {
				phantom = true
			}
		}
		if !cnc {
			t.Errorf("a git read error must report could-not-check for class 1; got %v", got)
		}
		if !phantom {
			t.Errorf("the text detectors must still run despite the git error; got %v", got)
		}
	})

	t.Run("could-not-check on an unreadable README names the stream", func(t *testing.T) {
		root := t.TempDir()
		// A stream dir with NO README.md — the read fails.
		dir := filepath.Join(root, "gtm")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		s := &Stream{Name: "gtm", Dir: dir, Briefs: []Brief{{Num: "08", Status: "todo"}}}
		got := boardHonestyNotices([]*Stream{s}, nil, nil)
		var sawCNC bool
		for _, n := range got {
			if strings.Contains(n, "could-not-check") && strings.Contains(n, "gtm") {
				sawCNC = true
			}
		}
		if !sawCNC {
			t.Fatalf("an unreadable README must be reported as could-not-check naming the stream; got %v", got)
		}
	})

	t.Run("no todo rows: nothing read, nothing surfaced", func(t *testing.T) {
		root := t.TempDir()
		s := writeStream(t, root, "gtm", "Active stream.\n", "08", "anything")
		s.Briefs = []Brief{{Num: "08", Status: "done"}}
		if got := boardHonestyNotices([]*Stream{s}, nil, nil); len(got) != 0 {
			t.Fatalf("a stream with no todo rows must produce nothing (not even a README read); got %v", got)
		}
	})
}
