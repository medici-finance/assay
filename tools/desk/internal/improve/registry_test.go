package improve

import (
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/askassay"
)

// TestImprovePaneEveryStripDeclaresACompleteSource — no silent caps. A strip
// that cannot state its command, probe, window and limit cannot render a row
// set, so the registry is checked in full on every run.
func TestImprovePaneEveryStripDeclaresACompleteSource(t *testing.T) {
	defs := Strips()
	if len(defs) != 4 {
		t.Fatalf("the registry declares %d strips; §7.3 specifies four", len(defs))
	}
	seenID := map[StripID]bool{}
	seenSource := map[string]string{}
	for _, d := range defs {
		if err := d.Validate(); err != nil {
			t.Fatalf("%s: %v", d.ID, err)
		}
		if seenID[d.ID] {
			t.Fatalf("%s is declared twice", d.ID)
		}
		seenID[d.ID] = true

		// derive-or-diff: one row set, one source. Two strips deriving the
		// same rows from two commands is how two numbers on one screen
		// disagree.
		key := d.Source.Cmd + "\x00" + d.Source.Probe
		if prev, dup := seenSource[key]; dup {
			t.Fatalf("%s and %s derive from the same (command, probe) pair — one source per row set", d.ID, prev)
		}
		seenSource[key] = string(d.ID)

		if strings.TrimSpace(d.Source.Limit) == "" {
			t.Fatalf("%s declares no limit", d.ID)
		}
	}
	for _, want := range []StripID{StripReports, StripClusters, StripRetroQueue, StripDidItWork} {
		if !seenID[want] {
			t.Fatalf("the %s strip is not declared", want)
		}
	}
}

// TestImprovePaneEveryStripCarriesItsCaveats — the per-strip caveat floor,
// checked against a hand-written roster rather than a count, for the reason
// recorded on the side-effect roster test.
func TestImprovePaneEveryStripCarriesItsCaveats(t *testing.T) {
	want := map[StripID][]string{
		StripReports:    {CaveatEmptyStripIsBlind, CaveatTaxonomyUnhardened, CaveatEvidenceLinkNotEvidence},
		StripClusters:   {CaveatEmptyStripIsBlind, CaveatTaxonomyUnhardened},
		StripRetroQueue: {CaveatEmptyStripIsBlind, CaveatAdoptionIsHuman, CaveatRegistersAppendOnly},
		StripDidItWork:  {CaveatEmptyStripIsBlind, CaveatUnwiredMetricIsNotNoMovement, CaveatAdoptionIsHuman},
	}
	if len(requiredCaveats) != len(want) {
		t.Fatalf("the caveat floor covers %d strips, this roster declares %d", len(requiredCaveats), len(want))
	}
	for id, wantList := range want {
		gotList, ok := requiredCaveats[id]
		if !ok {
			t.Fatalf("%s has no caveat floor", id)
		}
		if len(gotList) != len(wantList) {
			t.Fatalf("%s: floor holds %d caveats, roster declares %d — a caveat dropped here is a qualification the operator stops seeing", id, len(gotList), len(wantList))
		}
		for i := range wantList {
			if gotList[i] != wantList[i] {
				t.Fatalf("%s caveat %d differs from the roster", id, i)
			}
		}
		// And it must actually reach the render.
		d, _ := Lookup(id)
		rendered := Undecidable(d, "for the test", testStamp()).Render()
		for _, c := range wantList {
			if !strings.Contains(rendered, c) {
				t.Fatalf("%s: a required caveat does not reach the rendered strip", id)
			}
		}
	}
}

// TestImprovePaneAStripMissingItsFloorIsRefused — the floor is enforced, not
// merely declared.
func TestImprovePaneAStripMissingItsFloorIsRefused(t *testing.T) {
	d := StripDef{
		ID: StripDidItWork, Title: "Did it work",
		Source:  askassay.Source{Cmd: "c", Probe: "p", Window: "w", Limit: "l"},
		Caveats: []string{CaveatEmptyStripIsBlind},
	}
	err := d.Validate()
	if err == nil || !strings.Contains(err.Error(), "requires a caveat it does not carry") {
		t.Fatalf("a strip missing its caveat floor must be refused, got %v", err)
	}
	s := Derive(d, landedSubstrate(briefMCPSurface), []Row{}, testStamp())
	if s.State() != askassay.CouldNotCheck {
		t.Fatalf("a strip that cannot validate must not render rows, got %q", s.State())
	}
}

// TestImprovePaneStripDefValidationRefusesEveryIncompleteShape — one row per
// field a definition can be missing.
func TestImprovePaneStripDefValidationRefusesEveryIncompleteShape(t *testing.T) {
	base, _ := Lookup(StripReports)
	cases := []struct {
		name   string
		mutate func(StripDef) StripDef
		want   string
	}{
		{"no command", func(d StripDef) StripDef { d.Source.Cmd = ""; return d }, "declares no command"},
		{"no probe", func(d StripDef) StripDef { d.Source.Probe = ""; return d }, "declares no probe"},
		{"no window", func(d StripDef) StripDef { d.Source.Window = ""; return d }, "declares no window"},
		{"no limit", func(d StripDef) StripDef { d.Source.Limit = ""; return d }, "declares no limit"},
		{"no title", func(d StripDef) StripDef { d.Title = ""; return d }, "no operator-facing title"},
		{"undeclared strip id", func(d StripDef) StripDef { d.ID = StripID("measures"); return d }, "not a declared strip"},
		{"required brief with no reason", func(d StripDef) StripDef { d.RequiresWhy = ""; return d }, "does not say what it delivers"},
		{"empty-means-empty with no rationale", func(d StripDef) StripDef { d.EmptyMeansEmpty = true; return d }, "gives no rationale"},
		{"rationale with no declaration", func(d StripDef) StripDef { d.EmptyRationale = "because"; return d }, "does not declare EmptyMeansEmpty"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.mutate(base).Validate()
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error %v does not name %q", err, tc.want)
			}
		})
	}
	if err := base.Validate(); err != nil {
		t.Fatalf("the unmutated definition must validate, got %v", err)
	}
}

// TestImprovePaneEveryStripSourceWouldPassTheReadOnlyGate — for the strips
// whose source is a shell command. The three that read the not-yet-existing
// tool are checked differently: their source must NAME the brief that
// delivers it, which is what stops them being quietly wired to something else.
func TestImprovePaneEveryStripSourceWouldPassTheReadOnlyGate(t *testing.T) {
	for _, d := range Strips() {
		bin, _, _ := strings.Cut(d.Source.Cmd, " ")
		if strings.HasPrefix(bin, "desk__") {
			if d.RequiresBrief == "" {
				t.Fatalf("%s reads the tool %q but names no brief that delivers it — a strip may not read a tool from nowhere", d.ID, bin)
			}
			continue
		}
		if err := GuardStripSource(strings.Fields(d.Source.Cmd)); err != nil {
			t.Fatalf("%s declares a source the read-only gate refuses: %v", d.ID, err)
		}
	}
}

// TestImprovePaneSubstrateReportCannotBeAsserted — "the tool landed" is a
// statement with a provenance, not a boolean.
func TestImprovePaneSubstrateReportCannotBeAsserted(t *testing.T) {
	cases := []struct {
		name string
		r    SubstrateReport
		want string
	}{
		{"empty", SubstrateReport{}, "names no brief"},
		{"no status", SubstrateReport{Brief: "b"}, "carries no status"},
		{"no source", SubstrateReport{Brief: "b", Status: "done"}, "declares no"},
		{"no stamp", SubstrateReport{Brief: "b", Status: "done",
			Source: askassay.Source{Cmd: "c", Probe: "p", Window: "w", Limit: "l"}}, "no as-of stamp"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.r.Validate()
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error %v does not name %q", err, tc.want)
			}
		})
	}
	if err := landedSubstrate("x/01").Validate(); err != nil {
		t.Fatalf("a complete substrate report must validate, got %v", err)
	}
}

// TestImprovePaneImplementedIsNotLanded — the board carries `implemented` on
// work only its own author has checked. A pane rendering against an unverified
// tool inherits that, so `implemented` is deliberately not a landed status.
func TestImprovePaneImplementedIsNotLanded(t *testing.T) {
	for _, s := range []string{"todo", "in-progress", "implemented", "blocked", ""} {
		if unlandedSubstrate("b", s).Landed() {
			t.Fatalf("status %q was treated as landed", s)
		}
	}
	for _, s := range []string{"verified", "done", " Done "} {
		if !unlandedSubstrate("b", s).Landed() {
			t.Fatalf("status %q was not treated as landed", s)
		}
	}
	if len(landedStatuses) != 2 {
		t.Fatalf("%d landed statuses declared; the set is verified and done, and widening it is a decision", len(landedStatuses))
	}
}
