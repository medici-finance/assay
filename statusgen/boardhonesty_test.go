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
		name      string
		body      string
		readme    string
		merged    map[string]bool
		wantClass string // "" => must be clean
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
		{
			name:      "re-homed POSITIVE: README retired the row",
			readme:    "This stream was re-homed; the record merged as a plain commit.\n",
			wantClass: phantomReHomed,
		},
		{
			name:      "re-homed POSITIVE: do-not-re-implement wording",
			readme:    "Do not re-implement — the deliverable already landed.\n",
			wantClass: phantomReHomed,
		},
		{
			name:      "re-homed NEGATIVE: an ordinary README",
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
			class, reason, ok := classifyPhantom(id, tc.body, tc.readme, tc.merged)
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

	// Merged wins over every text detector.
	if class, _, ok := classifyPhantom(id, body, readme, map[string]bool{id: true}); !ok || class != phantomMergedUnflipped {
		t.Fatalf("merged must win: got ok=%v class=%q", ok, class)
	}
	// Without the merge, the source-moved banner (more specific) wins over the
	// deferred/re-homed matches also present.
	if class, _, ok := classifyPhantom(id, body, readme, nil); !ok || class != phantomStatusgenSource {
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

	t.Run("README-detected re-homed phantom fires without a brief body", func(t *testing.T) {
		root := t.TempDir()
		s := writeStream(t, root, "distribution", "This stream was re-homed; do not re-implement.\n",
			"07", "Plain body, no banner.\n")
		s.Briefs = []Brief{{Num: "07", Status: "todo"}}
		got := boardHonestyNotices([]*Stream{s}, nil, nil)
		if len(got) != 1 || !strings.Contains(got[0], phantomReHomed) {
			t.Fatalf("README re-homed banner must surface the row; got %v", got)
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
