package improve

import (
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/askassay"
)

func testStamp() askassay.Stamp {
	return askassay.Stamp{At: time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC), Ref: "deadbeef"}
}

// landedSubstrate is a substrate reading that says the tool exists. It is used
// ONLY to exercise the render path; nothing in this package claims the tool
// actually exists (see TestImprovePaneUnlandedSubstrateCannotBeDerived and the
// brief's Verify table for the measured status).
func landedSubstrate(brief string) SubstrateReport {
	return SubstrateReport{
		Brief:  brief,
		Status: "done",
		Source: askassay.Source{
			Cmd: "statusgen --root <root> --json", Probe: "the status field of the brief entry",
			Window: "the working tree at <sha>", Limit: "none on rows — every brief file is walked",
		},
		Stamp: testStamp(),
	}
}

func unlandedSubstrate(brief, status string) SubstrateReport {
	r := landedSubstrate(brief)
	r.Status = status
	return r
}

func goodReport(id string) ReportRow {
	return ReportRow{ID: id, Class: ClassBad, Title: "a thing that failed",
		Program: "p", Epic: "e", Evidence: []string{"evidence-" + id}}
}

// TestImprovePaneCouldNotCheckNeverRendersAsAnEmptyList is the property this
// whole package exists to hold. Seven ways of failing to derive a strip, and
// none of them may render a row area at all — not "0 rows", not "no rows",
// not a blank list.
func TestImprovePaneCouldNotCheckNeverRendersAsAnEmptyList(t *testing.T) {
	def, ok := Lookup(StripReports)
	if !ok {
		t.Fatal("reports strip is not declared")
	}
	incomplete := def
	incomplete.Source.Limit = ""

	noEmptyDecl := def
	noEmptyDecl.RequiresBrief = ""

	cases := []struct {
		name string
		got  Strip
	}{
		{"substrate not landed", Derive(def, unlandedSubstrate(briefMCPSurface, "todo"), []Row{goodReport("R-1")}, testStamp())},
		{"substrate unmeasured", Derive(def, SubstrateReport{}, []Row{goodReport("R-1")}, testStamp())},
		{"substrate of the wrong brief", Derive(def, landedSubstrate("some-other-stream/01"), []Row{goodReport("R-1")}, testStamp())},
		{"source declares no limit", Derive(incomplete, landedSubstrate(briefMCPSurface), []Row{goodReport("R-1")}, testStamp())},
		{"no as-of stamp", Derive(def, landedSubstrate(briefMCPSurface), []Row{goodReport("R-1")}, askassay.Stamp{})},
		{"empty read with no rationale", Derive(noEmptyDecl, SubstrateReport{}, nil, testStamp())},
		{"a row that cannot render", Derive(def, landedSubstrate(briefMCPSurface), []Row{ReportRow{ID: "R-2", Class: ClassBad, Title: "t"}}, testStamp())},
		{"undeclared strip", Undeclared(StripID("measures"), testStamp())},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.got.State() != askassay.CouldNotCheck {
				t.Fatalf("state = %q, want %q", tc.got.State(), askassay.CouldNotCheck)
			}
			if rows, ok := tc.got.Rows(); ok {
				t.Fatalf("Rows() reported ok=true with %d rows on a could-not-check strip", len(rows))
			}
			out := tc.got.Render()
			if !strings.Contains(out, "rows="+RowsField) {
				t.Fatalf("render does not carry the could-not-check token in the row position:\n%s", out)
			}
			// The specific falsehoods this test forbids, spelled out so a
			// regression names the shape rather than just failing.
			for _, forbidden := range []string{"rows=0", "rows=" + MeasuredEmptyField} {
				if strings.Contains(out, forbidden) {
					t.Fatalf("a could-not-check strip rendered %q — an underivable strip and an empty one are the exact pair this package exists to keep apart:\n%s", forbidden, out)
				}
			}
			if strings.TrimSpace(tc.got.Reason()) == "" {
				t.Fatal("could-not-check with no reason is not a finding")
			}
		})
	}
}

// TestImprovePaneMeasuredEmptyStillRenders is the control in the other
// direction. Without it the rule above would be satisfiable by refusing
// everything, which is its own kind of useless pane.
func TestImprovePaneMeasuredEmptyStillRenders(t *testing.T) {
	def := StripDef{
		ID: StripRetroQueue, Title: "Retro queue",
		Source:          askassay.Source{Cmd: "c", Probe: "p", Window: "w", Limit: "l"},
		Caveats:         []string{CaveatEmptyStripIsBlind, CaveatAdoptionIsHuman, CaveatRegistersAppendOnly},
		EmptyMeansEmpty: true,
		EmptyRationale:  "the register is read whole from a local file with no paging and no network, so an empty read is an empty register",
	}
	s := Derive(def, SubstrateReport{}, nil, testStamp())
	if s.State() != askassay.Checked {
		t.Fatalf("state = %q, want %q (reason: %s)", s.State(), askassay.Checked, s.Reason())
	}
	rows, ok := s.Rows()
	if !ok || len(rows) != 0 {
		t.Fatalf("Rows() = %v, ok=%v; want an empty-but-present row set", rows, ok)
	}
	out := s.Render()
	if !strings.Contains(out, "rows="+MeasuredEmptyField) {
		t.Fatalf("a measured-empty strip must render the measured-empty token:\n%s", out)
	}
	if strings.Contains(out, "rows="+RowsField) {
		t.Fatalf("a measured-empty strip must NOT render as could-not-check:\n%s", out)
	}
	if !strings.Contains(s.Reason(), "measured empty") {
		t.Fatalf("a measured-empty strip must say so in its reason, got %q", s.Reason())
	}
}

// TestImprovePaneDerivedStripRendersItsRows is the plain happy path: a strip
// that CAN be derived renders its rows, its count and its source.
func TestImprovePaneDerivedStripRendersItsRows(t *testing.T) {
	def, _ := Lookup(StripReports)
	s := Derive(def, landedSubstrate(briefMCPSurface),
		[]Row{goodReport("R-1"), goodReport("R-2")}, testStamp())
	if s.State() != askassay.Checked {
		t.Fatalf("state = %q (reason: %s)", s.State(), s.Reason())
	}
	out := s.Render()
	for _, want := range []string{"rows=2", "report R-1", "report R-2", "source=", "probe=", "window=", "limit=", "as-of="} {
		if !strings.Contains(out, want) {
			t.Fatalf("render is missing %q:\n%s", want, out)
		}
	}
}

// TestImprovePaneSaturatedRowSetIsRefused — a row set at its cap is not a row
// set, for the same reason a count at its cap is not a count.
func TestImprovePaneSaturatedRowSetIsRefused(t *testing.T) {
	def, _ := Lookup(StripReports)
	def.SaturatesAt = 3
	sub := landedSubstrate(briefMCPSurface)

	under := Derive(def, sub, []Row{goodReport("R-1"), goodReport("R-2")}, testStamp())
	if under.State() != askassay.Checked {
		t.Fatalf("2 rows under a cap of 3 should render, got %q (%s)", under.State(), under.Reason())
	}
	for _, n := range []int{3, 4} {
		rows := make([]Row, 0, n)
		for i := 0; i < n; i++ {
			rows = append(rows, goodReport("R-"+string(rune('a'+i))))
		}
		got := Derive(def, sub, rows, testStamp())
		if got.State() != askassay.CouldNotCheck {
			t.Fatalf("%d rows at/over a cap of 3: state = %q, want could-not-check", n, got.State())
		}
		if !strings.Contains(got.Reason(), "cap of 3") {
			t.Fatalf("the refusal must name the cap, got %q", got.Reason())
		}
	}
}

// TestImprovePaneNilRowTakesTheStripDown — a hole in a row set would render as
// a shorter list, which is a silent loss.
func TestImprovePaneNilRowTakesTheStripDown(t *testing.T) {
	def, _ := Lookup(StripReports)
	s := Derive(def, landedSubstrate(briefMCPSurface), []Row{goodReport("R-1"), nil}, testStamp())
	if s.State() != askassay.CouldNotCheck {
		t.Fatalf("state = %q, want could-not-check", s.State())
	}
	if !strings.Contains(s.Reason(), "nil") {
		t.Fatalf("reason should name the nil row, got %q", s.Reason())
	}
}

// TestImprovePaneUnlandedSubstrateCannotBeDerived is the anti-mock, and it is
// the brief's positive control: a source that cannot be reached is PROVEN to
// render could-not-check rather than an empty pane.
//
// The four board statuses below are the ones a brief actually carries.
// `implemented` is included deliberately as a NEGATIVE: work that only its
// own author has checked is not a landed tool.
func TestImprovePaneUnlandedSubstrateCannotBeDerived(t *testing.T) {
	def, _ := Lookup(StripReports)
	rows := []Row{goodReport("R-1")}

	for _, status := range []string{"todo", "in-progress", "implemented"} {
		got := Derive(def, unlandedSubstrate(briefMCPSurface, status), rows, testStamp())
		if got.State() != askassay.CouldNotCheck {
			t.Fatalf("status %q: state = %q, want could-not-check", status, got.State())
		}
		if !strings.Contains(got.Reason(), briefMCPSurface) {
			t.Fatalf("status %q: the refusal must name the brief that has not landed, got %q", status, got.Reason())
		}
		if !strings.Contains(got.Reason(), "not empty") && !strings.Contains(got.Reason(), "nothing was asked") {
			t.Fatalf("status %q: the refusal must distinguish itself from emptiness, got %q", status, got.Reason())
		}
	}
	for _, status := range []string{"verified", "done", "DONE"} {
		got := Derive(def, unlandedSubstrate(briefMCPSurface, status), rows, testStamp())
		if got.State() != askassay.Checked {
			t.Fatalf("status %q should be treated as landed, got %q (%s)", status, got.State(), got.Reason())
		}
	}
}

// TestImprovePaneUnresolvedReferencesRenderOnTheStrip — a join failure carried
// alongside the rows, never subtracted from them.
func TestImprovePaneUnresolvedReferencesRenderOnTheStrip(t *testing.T) {
	def, _ := Lookup(StripClusters)
	c := ClusterRow{ID: "C-1", Title: "3 of the same, 2 weeks", MemberIDs: []string{"R-1", "R-9"}, Window: "14d"}
	s := Derive(def, landedSubstrate(briefMCPSurface), []Row{c}, testStamp()).
		WithUnresolved([]Unresolved{{From: "C-1", Ref: "R-9", Why: "not in the rendered report set"}})
	out := s.Render()
	if !strings.Contains(out, "UNRESOLVED") || !strings.Contains(out, "R-9") {
		t.Fatalf("an unresolved reference must render on the strip:\n%s", out)
	}
	if !strings.Contains(out, "members=2") {
		t.Fatalf("the cluster's member count must still be 2 — an unresolved member is not a removed one:\n%s", out)
	}
}

// TestImprovePaneEndToEndTheWholePaneRendersCouldNotCheckToday is the
// whole-surface positive control. With the substrate measured at its real
// status, EVERY strip renders could-not-check with a named reason, and NOT one
// of them renders an empty list. This is the pane as it would honestly appear
// today.
func TestImprovePaneEndToEndTheWholePaneRendersCouldNotCheckToday(t *testing.T) {
	// The substrate reading a real pane would supply: the brief that delivers
	// the read surface, at the status the board actually carries.
	sub := unlandedSubstrate(briefMCPSurface, "todo")

	defs := Strips()
	if len(defs) != 4 {
		t.Fatalf("the pane declares %d strips, want the four of §7.3", len(defs))
	}
	for _, d := range defs {
		s := Derive(d, sub, nil, testStamp())
		if s.State() != askassay.CouldNotCheck {
			t.Fatalf("%s: state = %q, want could-not-check", d.ID, s.State())
		}
		out := s.Render()
		if strings.Contains(out, "rows=0") || strings.Contains(out, "rows="+MeasuredEmptyField) {
			t.Fatalf("%s rendered as an empty strip rather than an underivable one:\n%s", d.ID, out)
		}
		if !strings.Contains(out, "rows="+RowsField) {
			t.Fatalf("%s did not render the could-not-check token:\n%s", d.ID, out)
		}
		if !strings.Contains(s.Reason(), briefMCPSurface) {
			t.Fatalf("%s: reason must name the missing substrate, got %q", d.ID, s.Reason())
		}
		if len(s.Caveats()) == 0 {
			t.Fatalf("%s: a could-not-check strip still carries its caveats", d.ID)
		}
	}
}
