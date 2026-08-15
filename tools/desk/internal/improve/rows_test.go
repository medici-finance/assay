package improve

import (
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/askassay"
)

func figure(v int64) askassay.Answer {
	q, ok := askassay.Lookup("flow-throughput")
	if !ok {
		panic("the answer layer no longer declares flow-throughput")
	}
	return askassay.Computed(q, v, testStamp())
}

func missingFigure(reason string) askassay.Answer {
	q, _ := askassay.Lookup("flow-throughput")
	return askassay.Unavailable(q, reason, testStamp())
}

// TestImprovePaneReportWithoutEvidenceIsRefused — §7.3: every report links its
// evidence. A report with none is an opinion with a class attached.
func TestImprovePaneReportWithoutEvidenceIsRefused(t *testing.T) {
	cases := []struct {
		name string
		row  ReportRow
		want string
	}{
		{"no evidence at all", ReportRow{ID: "R-1", Class: ClassBad, Title: "t"}, "no evidence link"},
		{"blank evidence", ReportRow{ID: "R-1", Class: ClassBad, Title: "t", Evidence: []string{"", "  "}}, "no evidence link"},
		{"no ID", ReportRow{Class: ClassBad, Title: "t", Evidence: []string{"e"}}, "no ID"},
		{"no title", ReportRow{ID: "R-1", Class: ClassBad, Evidence: []string{"e"}}, "no title"},
		{"undeclared class", ReportRow{ID: "R-1", Class: Class("meh"), Title: "t", Evidence: []string{"e"}}, "not one of the declared classes"},
		{"empty class", ReportRow{ID: "R-1", Title: "t", Evidence: []string{"e"}}, "not one of the declared classes"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.row.Validate()
			if err == nil {
				t.Fatal("row validated when it should have been refused")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error %q does not name %q", err.Error(), tc.want)
			}
		})
	}
	// The control: each of the three declared classes, with evidence, renders.
	for _, c := range Classes() {
		r := ReportRow{ID: "R-ok", Class: c, Title: "t", Evidence: []string{"e"}}
		if err := r.Validate(); err != nil {
			t.Fatalf("class %q with evidence should validate, got %v", c, err)
		}
	}
}

// TestImprovePaneReportRendersItsScopeHonestly — an unscoped report says
// unscoped rather than being filed under a default program.
func TestImprovePaneReportRendersItsScopeHonestly(t *testing.T) {
	r := ReportRow{ID: "R-1", Class: ClassUgly, Title: "t", Evidence: []string{"e"}}
	out := r.Render()
	if !strings.Contains(out, "program=unscoped") || !strings.Contains(out, "epic=unscoped") {
		t.Fatalf("an unscoped report must render as unscoped:\n%s", out)
	}
}

// TestImprovePaneClusterMembersThatDoNotResolveAreReportedNotDropped — the
// drill-down map holds what you can click through to; the unresolved list
// holds what you cannot; neither is silently shortened.
func TestImprovePaneClusterMembersThatDoNotResolveAreReportedNotDropped(t *testing.T) {
	reports := []ReportRow{goodReport("R-1"), goodReport("R-2")}
	clusters := []ClusterRow{
		{ID: "C-1", Title: "three of a kind", MemberIDs: []string{"R-1", "R-2", "R-404"}, Window: "14d"},
		{ID: "C-2", Title: "an empty slot", MemberIDs: []string{"R-1", "  "}, Window: "14d"},
	}
	members, unresolved := ResolveClusterMembers(clusters, reports)

	if got := len(members["C-1"]); got != 2 {
		t.Fatalf("C-1 resolved %d members, want 2", got)
	}
	if len(unresolved) != 2 {
		t.Fatalf("got %d unresolved, want 2 (a missing report and an empty slot): %v", len(unresolved), unresolved)
	}
	var sawMissing, sawEmpty bool
	for _, u := range unresolved {
		if u.From == "C-1" && u.Ref == "R-404" {
			sawMissing = true
		}
		if u.From == "C-2" && u.Ref == "(empty)" {
			sawEmpty = true
		}
	}
	if !sawMissing || !sawEmpty {
		t.Fatalf("both failure shapes must be reported, got %v", unresolved)
	}
	// The count that matters: the cluster still says three members.
	if !strings.Contains(clusters[0].Render(), "members=3") {
		t.Fatalf("an unresolved member stays in the count:\n%s", clusters[0].Render())
	}
	// The control: a fully resolving cluster reports nothing unresolved.
	_, clean := ResolveClusterMembers([]ClusterRow{{ID: "C-3", Title: "t", MemberIDs: []string{"R-1"}, Window: "14d"}}, reports)
	if len(clean) != 0 {
		t.Fatalf("a fully resolved cluster must report nothing unresolved, got %v", clean)
	}
}

// TestImprovePaneClusterValidation — a cluster is an assertion; the fields
// that make it one are required.
func TestImprovePaneClusterValidation(t *testing.T) {
	cases := []struct {
		name string
		row  ClusterRow
		want string
	}{
		{"no members", ClusterRow{ID: "C-1", Title: "t", Window: "14d"}, "groups no members"},
		{"blank members", ClusterRow{ID: "C-1", Title: "t", MemberIDs: []string{" "}, Window: "14d"}, "groups no members"},
		{"no window", ClusterRow{ID: "C-1", Title: "t", MemberIDs: []string{"R-1"}}, "states no window"},
		{"no title", ClusterRow{ID: "C-1", MemberIDs: []string{"R-1"}, Window: "14d"}, "no title"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.row.Validate()
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error %v does not name %q", err, tc.want)
			}
		})
	}
}

// TestImprovePaneProposalWithoutItsTwoMeasurementsIsRefused — the deck's hard
// rule and its missing twin, both enforced: a proposal states the measurement
// that motivated it AND the one that will judge it.
func TestImprovePaneProposalWithoutItsTwoMeasurementsIsRefused(t *testing.T) {
	base := ProposalRow{
		ID: "P-1", Title: "t", IntakeRef: "I-01",
		MotivatingEvidence: []string{"e"}, TargetMetric: "lead time",
		Adopt: HumanCommitAdopt("adopt"),
	}
	if err := base.Validate(); err != nil {
		t.Fatalf("the complete proposal should validate, got %v", err)
	}

	noMotive := base
	noMotive.MotivatingEvidence = nil
	if err := noMotive.Validate(); err == nil || !strings.Contains(err.Error(), "motivated it") {
		t.Fatalf("a proposal with no motivating evidence must be refused, got %v", err)
	}

	noTarget := base
	noTarget.TargetMetric = ""
	if err := noTarget.Validate(); err == nil || !strings.Contains(err.Error(), "JUDGES it") {
		t.Fatalf("a proposal with no target metric must be refused, got %v", err)
	}

	noRegister := base
	noRegister.IntakeRef = ""
	if err := noRegister.Validate(); err == nil || !strings.Contains(err.Error(), "register") {
		t.Fatalf("a proposal the pane cannot point at in the register must be refused, got %v", err)
	}
}

// TestImprovePaneUnwiredMetricIsUndeterminedNotNoMovement is the single most
// load-bearing branch in this package. A target metric that cannot be read
// renders UNDETERMINED. The alternative — treating a missing figure as no
// movement — converts an unwired instrument into a verdict about somebody's
// process change.
func TestImprovePaneUnwiredMetricIsUndeterminedNotNoMovement(t *testing.T) {
	row := func(before, after askassay.Answer) RetroActionRow {
		return RetroActionRow{ID: "A-1", Title: "t", AdoptedBy: "the human gate",
			Cadence: "2026-W33", TargetMetric: "lead time", Direction: DirectionDown,
			Before: before, After: after}
	}
	cases := []struct {
		name string
		row  RetroActionRow
		want string
	}{
		{"neither figure", row(missingFigure("the log does not exist"), missingFigure("the log does not exist")), "neither figure exists"},
		{"no before", row(missingFigure("no baseline was captured"), figure(4)), "BEFORE figure does not exist"},
		{"no after", row(figure(4), missingFigure("the probe came back blind")), "AFTER figure does not exist"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.row.Verdict(); got != VerdictUndetermined {
				t.Fatalf("verdict = %q, want %q — a metric that could not be read is NOT a change that did nothing", got, VerdictUndetermined)
			}
			reason := tc.row.VerdictReason()
			if !strings.Contains(reason, tc.want) {
				t.Fatalf("verdict reason %q does not name %q", reason, tc.want)
			}
			if !strings.Contains(reason, "not no-movement") {
				t.Fatalf("the reason must say plainly that this is not no-movement, got %q", reason)
			}
			out := tc.row.Render()
			if strings.Contains(out, "verdict="+string(VerdictNoMovement)) {
				t.Fatalf("an undetermined row rendered as no-movement:\n%s", out)
			}
			if !strings.Contains(out, askassay.FigureField) {
				t.Fatalf("the missing figure must render as could-not-check inside the row:\n%s", out)
			}
		})
	}
}

// TestImprovePaneMeasuredMovementIsStillJudged is the control in the other
// direction: when both figures are real, all three real verdicts are reachable
// — including the no-movement verdict the strip exists to surface.
func TestImprovePaneMeasuredMovementIsStillJudged(t *testing.T) {
	row := func(dir Direction, before, after int64) RetroActionRow {
		return RetroActionRow{ID: "A-1", Title: "t", AdoptedBy: "the human gate",
			Cadence: "2026-W33", TargetMetric: "lead time", Direction: dir,
			Before: figure(before), After: figure(after)}
	}
	cases := []struct {
		name string
		row  RetroActionRow
		want Verdict
	}{
		{"down, and it fell", row(DirectionDown, 10, 4), VerdictMoved},
		{"down, and it rose", row(DirectionDown, 4, 10), VerdictRegressed},
		{"up, and it rose", row(DirectionUp, 4, 10), VerdictMoved},
		{"up, and it fell", row(DirectionUp, 10, 4), VerdictRegressed},
		{"flat", row(DirectionDown, 7, 7), VerdictNoMovement},
		{"flat at zero — a real measured zero on both sides", row(DirectionUp, 0, 0), VerdictNoMovement},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.row.Verdict(); got != tc.want {
				t.Fatalf("verdict = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestImprovePaneRetroActionValidation — an adopted change with no named human
// is a gate that was not passed, and a change with no direction makes every
// movement look like success.
func TestImprovePaneRetroActionValidation(t *testing.T) {
	base := RetroActionRow{ID: "A-1", Title: "t", AdoptedBy: "the human gate",
		Cadence: "2026-W33", TargetMetric: "lead time", Direction: DirectionDown,
		Before: figure(1), After: figure(1)}
	if err := base.Validate(); err != nil {
		t.Fatalf("the complete row should validate, got %v", err)
	}
	cases := []struct {
		name   string
		mutate func(RetroActionRow) RetroActionRow
		want   string
	}{
		{"no adopter", func(r RetroActionRow) RetroActionRow { r.AdoptedBy = ""; return r }, "names no adopter"},
		{"no cadence", func(r RetroActionRow) RetroActionRow { r.Cadence = ""; return r }, "names no cadence"},
		{"no target metric", func(r RetroActionRow) RetroActionRow { r.TargetMetric = ""; return r }, "names no target metric"},
		{"no direction", func(r RetroActionRow) RetroActionRow { r.Direction = ""; return r }, "not a declared direction"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.mutate(base).Validate()
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error %v does not name %q", err, tc.want)
			}
		})
	}
}
