package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/statusgen/streamview"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

const (
	msRev     = "0123456789abcdef0123456789abcdef01234567"
	msAllowed = "example-org/tracker"
	msHidden  = "hidden-org/private-repo"
)

var msNow = time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)

func msView(repo, slug string, section *streamview.Milestones) *streamview.StreamView {
	src := streamview.SourceRef{Repo: repo, Path: "docs/streams/" + slug + "/README.md", Revision: msRev}
	prov := func(a streamview.Availability, reason string, srcs ...streamview.SourceRef) streamview.Provenance {
		return streamview.Provenance{Availability: a, Sources: srcs, ObservedAt: msNow, Reason: reason}
	}
	v := &streamview.StreamView{
		Contract:    streamview.Version,
		Identity:    streamview.Identity{Key: streamview.Key{Repo: repo, Slug: slug}, DisplayName: slug},
		GeneratedAt: msNow,
		Mission: streamview.Mission{
			Provenance: prov(streamview.Available, "", src), Origin: streamview.OriginAuthored, Outcome: "o",
			Success: []streamview.SuccessCriterion{{Criterion: "c", Evidence: []streamview.EvidenceRef{{Kind: streamview.EvidencePath, Repo: repo, Path: "docs/r.md"}}}},
		},
		CurrentState: streamview.CurrentState{Provenance: prov(streamview.Available, "", src), Status: "active"},
		Changes:      streamview.Changes{Provenance: prov(streamview.CouldNotCheck, "no history input"), Window: streamview.TrailingWindow(msNow)},
		NeedsYou:     streamview.NeedsYou{Provenance: prov(streamview.CouldNotCheck, "no decision source")},
		Evidence:     streamview.Evidence{Provenance: prov(streamview.NotAssessed, "no producer")},
		Outcomes:     streamview.Outcomes{Provenance: prov(streamview.NotAssessed, "no producer")},
	}
	if section != nil {
		v.Contract = streamview.Version2
		v.Milestones = section
	}
	return v
}

func msWrite(t *testing.T, dir, name string, v *streamview.StreamView) string {
	t.Helper()
	b, err := streamview.Encode(v)
	if err != nil {
		t.Fatalf("fixture view invalid: %v", err)
	}
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, b, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func msSection(repo string) *streamview.Milestones {
	src := streamview.SourceRef{Repo: repo, Path: "docs/streams/shared/README.md", Revision: msRev}
	return &streamview.Milestones{
		Provenance: streamview.Provenance{Availability: streamview.Available, Sources: []streamview.SourceRef{src}, ObservedAt: msNow},
		Coverage:   streamview.CoverageKnown,
		Items: []streamview.MilestoneItem{
			{
				// Zero work remaining with acceptance outstanding: NOT done.
				Ref: "cell:tracker:shared:m-01@1", Title: "Ready for acceptance", Scope: []string{"cell:tracker:shared:01"},
				Work:         streamview.MilestoneWork{State: streamview.WorkKnown},
				Acceptance:   streamview.MilestoneAcceptance{State: streamview.AcceptanceOutstanding},
				Availability: streamview.MilestoneUnknown,
				Holds:        []streamview.MilestoneHold{{Ref: "cell:tracker:shared:m-01@1", Kind: "acceptance", State: "held", Reason: "acceptance outstanding"}},
				Evidence: []streamview.EvidenceRef{
					{Kind: streamview.EvidencePath, Repo: repo, Path: "docs/report.md"},
					{Kind: streamview.EvidencePath, Repo: msHidden, Path: "secret/plan.md"},
					{Kind: streamview.EvidenceForge, Repo: msHidden, Number: 7},
					{Kind: streamview.EvidenceURL, URL: "https://github.com/" + msHidden + "/pull/7"},
				},
			},
			{
				Ref: "cell:tracker:shared:m-02@1", Title: "Work remains \x1b[31mRED\x1b[0m \"quoted\"\n# Heading", Aliases: []string{"legacy-m2"},
				Scope:         []string{"cell:tracker:shared:01", "cell:tracker:shared:02"},
				Work:          streamview.MilestoneWork{State: streamview.WorkKnown, Remaining: 1, Pending: []string{"cell:tracker:shared:02"}},
				Acceptance:    streamview.MilestoneAcceptance{State: streamview.AcceptanceNotDue},
				Availability:  streamview.MilestoneBlocked,
				Prerequisites: []string{"cell:tracker:shared:m-01@1"},
				Holds:         []streamview.MilestoneHold{{Ref: "cell:tracker:shared:02", Kind: "work", State: "held", Reason: "work remaining"}},
			},
			{
				Ref: "cell:tracker:shared:m-03@1", Title: "Unread", Scope: []string{},
				Work:         streamview.MilestoneWork{State: streamview.WorkUnknown, Reason: "plan coverage is unknown"},
				Acceptance:   streamview.MilestoneAcceptance{State: streamview.AcceptanceCouldNotCheck, Reason: "no acceptance observation supplied"},
				Availability: streamview.MilestoneUnknown,
			},
		},
		Membership: []streamview.MilestoneMembership{{Brief: "cell:tracker:shared:01", Milestones: []string{"cell:tracker:shared:m-01@1", "cell:tracker:shared:m-02@1"}}},
	}
}

func msRun(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	installFakeGH(t)
	var out, errb bytes.Buffer
	code := run(args, &out, &errb)
	return code, out.String(), errb.String()
}

// regression: the milestones verb — the board must show what the versioned
// projection says: zero work with acceptance outstanding is NOT done, aliases
// never add a row or change approval, hostile labels are inert, private
// locators are withheld, and a contract it cannot decode is refused.
func TestDeskboardMilestones(t *testing.T) {
	dir := t.TempDir()
	good := msWrite(t, dir, "shared.json", msView(msAllowed, "shared", msSection(msAllowed)))

	t.Run("table shows the facts and no inferred done", func(t *testing.T) {
		code, out, errb := msRun(t, "milestones", "--table", good)
		if code != deskkit.ExitOK {
			t.Fatalf("exit %d: %s", code, errb)
		}
		var m01 string
		for _, blk := range strings.Split(out, "\nMILESTONE ") {
			if strings.HasPrefix(blk, `"cell:tracker:shared:m-01@1"`) {
				m01 = blk
			}
		}
		if !strings.Contains(m01, "work: 0 remaining | acceptance: outstanding | availability: unknown") {
			t.Errorf("zero work with acceptance outstanding not shown truthfully:\n%s", m01)
		}
		for _, banned := range []string{"done", "complete", "availability: supported", "accepted"} {
			if strings.Contains(strings.ToLower(m01), banned) {
				t.Errorf("renderer inferred %q for a milestone whose acceptance is outstanding:\n%s", banned, m01)
			}
		}
		for _, want := range []string{"SOURCE COVERAGE", "coverage=known", "unknown (\"plan coverage is unknown\")",
			"could-not-check", "supported scope:", "prerequisites:", "HOLD acceptance", "MEMBERSHIP", "example-org/tracker:docs/report.md"} {
			if !strings.Contains(out, want) {
				t.Errorf("table lacks %q:\n%s", want, out)
			}
		}
	})

	t.Run("hostile labels are inert", func(t *testing.T) {
		_, out, _ := msRun(t, "milestones", "--table", good)
		if strings.Contains(out, "\x1b") {
			t.Error("an escape sequence reached the terminal")
		}
		if strings.Contains(out, "\n# Heading") {
			t.Errorf("a newline in a title opened a new line of output:\n%s", out)
		}
		if !strings.Contains(out, `\n# Heading`) {
			t.Errorf("hostile title not shown quoted:\n%s", out)
		}
	})

	t.Run("private locators are withheld", func(t *testing.T) {
		code, out, _ := msRun(t, "milestones", "--table", good)
		if code != deskkit.ExitOK {
			t.Fatal(code)
		}
		if strings.Contains(out, msHidden) || strings.Contains(out, "secret/plan.md") {
			t.Errorf("an inaccessible locator leaked:\n%s", out)
		}
		if !strings.Contains(out, "[withheld]") || !strings.Contains(out, "WITHHELD: 0 view(s), 3 locator(s)") {
			t.Errorf("withheld locators not accounted for:\n%s", out)
		}
		hidden := msWrite(t, dir, "hidden.json", msView(msHidden, "shared", msSection(msHidden)))
		_, out, _ = msRun(t, "milestones", "--table", hidden)
		if strings.Contains(out, msHidden) || strings.Contains(out, "m-01") {
			t.Errorf("a view from an inaccessible repository was rendered:\n%s", out)
		}
	})

	t.Run("aliases never duplicate rows or change approval", func(t *testing.T) {
		// A second view spells the same milestone by its legacy alias and claims
		// it accepted. It must not add a row nor upgrade the canonical reading.
		sec := &streamview.Milestones{
			Provenance: streamview.Provenance{Availability: streamview.Available, ObservedAt: msNow,
				Sources: []streamview.SourceRef{{Repo: msAllowed, Path: "docs/streams/other/README.md", Revision: msRev}}},
			Coverage: streamview.CoverageKnown,
			Items: []streamview.MilestoneItem{{
				Ref: "legacy-m2", Title: "Work remains (legacy spelling)", Scope: []string{},
				Work:         streamview.MilestoneWork{State: streamview.WorkKnown},
				Acceptance:   streamview.MilestoneAcceptance{State: streamview.AcceptanceAccepted},
				Availability: streamview.MilestoneSupported,
			}},
		}
		other := msWrite(t, dir, "other.json", msView(msAllowed, "other", sec))
		for _, order := range [][]string{{other, good}, {good, other}} {
			code, out, errb := msRun(t, append([]string{"milestones"}, order...)...)
			if code != deskkit.ExitOK {
				t.Fatalf("exit %d: %s", code, errb)
			}
			var rep milestonesReport
			if err := json.Unmarshal([]byte(out), &rep); err != nil {
				t.Fatal(err)
			}
			if len(rep.Milestones) != 3 {
				t.Fatalf("alias created or lost a row: %d rows", len(rep.Milestones))
			}
			for _, r := range rep.Milestones {
				if r.Ref == "legacy-m2" {
					t.Error("the alias became a row of its own")
				}
				if strings.HasSuffix(r.Ref, "m-02@1") {
					if r.Acceptance.State != streamview.AcceptanceNotDue || r.Availability != streamview.MilestoneBlocked || r.Work.Remaining != 1 {
						t.Errorf("alias changed approval state: %+v", r)
					}
					if len(r.Streams) != 2 || len(r.Conflicts) == 0 {
						t.Errorf("alias view not folded in and flagged: streams=%v conflicts=%v", r.Streams, r.Conflicts)
					}
				}
			}
		}
	})

	t.Run("v1 view is not assessed, and unknown coverage is explicit", func(t *testing.T) {
		v1 := msWrite(t, dir, "v1.json", msView(msAllowed, "old", nil))
		unk := msSection(msAllowed)
		unk.Provenance.Availability = streamview.Partial
		unk.Provenance.Reason = "one record unreadable"
		unk.Coverage = streamview.CoverageUnknown
		unk.Unknowns = []string{"records/m-09.md: unreadable"}
		partial := msWrite(t, dir, "partial.json", msView(msAllowed, "partial", unk))
		code, out, errb := msRun(t, "milestones", "--table", v1, partial)
		if code != deskkit.ExitOK {
			t.Fatalf("exit %d: %s", code, errb)
		}
		for _, want := range []string{"availability=not-assessed", "view predates milestones", "availability=partial", "coverage=unknown", "UNKNOWN: \"records/m-09.md: unreadable\""} {
			if !strings.Contains(out, want) {
				t.Errorf("missing %q:\n%s", want, out)
			}
		}
	})

	t.Run("an undecodable contract is refused, never partially rendered", func(t *testing.T) {
		b, _ := os.ReadFile(good)
		bad := filepath.Join(dir, "v9.json")
		os.WriteFile(bad, []byte(strings.Replace(string(b), `"stream-view/v2"`, `"stream-view/v9"`, 1)), 0o644)
		code, out, errb := msRun(t, "milestones", good, bad)
		if code != deskkit.ExitRefused {
			t.Fatalf("exit %d, want %d", code, deskkit.ExitRefused)
		}
		if strings.TrimSpace(out) != "" || !strings.Contains(errb, "unsupported contract version") {
			t.Errorf("stdout=%q stderr=%q", out, errb)
		}
		// A v2 document that lost its mandatory unknown field is also refused.
		broken := strings.Replace(string(b), `"coverage": "known"`, `"coverage": "unknown"`, 1)
		bp := filepath.Join(dir, "broken.json")
		os.WriteFile(bp, []byte(broken), 0o644)
		if code, _, _ := msRun(t, "milestones", bp); code != deskkit.ExitRefused {
			t.Errorf("unknown coverage with no unknown listed must be refused, exit %d", code)
		}
		if code, _, _ := msRun(t, "milestones"); code == deskkit.ExitOK {
			t.Error("no input must not be a clean result")
		}
	})
}
