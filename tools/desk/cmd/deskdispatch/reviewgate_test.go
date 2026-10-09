package main

// reviewgate_test.go — the pre-dispatch gate (#2444): each hold condition, each read that
// fails (and so dispatches), and what a held dispatch leaves behind — nothing.

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

const gateRatifier = "example-ratifier"

// useRatifier binds the ratifying identity for one test; ok=false is "none configured".
func useRatifier(t *testing.T, ok bool) {
	t.Helper()
	old := reviewGateRatifierFn
	reviewGateRatifierFn = func() (func(string, int64) bool, bool) {
		return func(login string, _ int64) bool { return login == gateRatifier }, ok
	}
	t.Cleanup(func() { reviewGateRatifierFn = old })
}

func at(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return t
}

func run1(name, status, conclusion, when string) deskkit.CheckRun {
	return deskkit.CheckRun{Name: name, Status: status, Conclusion: conclusion, StartedAt: when, CompletedAt: when}
}

func checksOf(runs []deskkit.CheckRun, statuses ...deskkit.StatusContext) *deskkit.ChecksAtHead {
	return &deskkit.ChecksAtHead{CheckRuns: runs, CheckRunsTotalCount: len(runs), Statuses: statuses, StatusTotalCount: len(statuses)}
}

// Red is a completed failure on the latest report under a required name, and nothing else.
func TestRequiredCheckRedIsACompletedFailureOnly(t *testing.T) {
	const t1, t2 = "2026-03-04T05:00:00Z", "2026-03-04T06:00:00Z"
	for _, tc := range []struct {
		name     string
		required []string
		checks   *deskkit.ChecksAtHead
		want     string
	}{
		{"a completed failure", []string{"build"}, checksOf([]deskkit.CheckRun{run1("build", "completed", "failure", t1)}), `"build"`},
		{"a timed-out run", []string{"build"}, checksOf([]deskkit.CheckRun{run1("build", "completed", "timed_out", t1)}), `"build"`},
		{"a run that failed to start", []string{"build"}, checksOf([]deskkit.CheckRun{run1("build", "completed", "startup_failure", t1)}), `"build"`},
		{"a failed commit status", []string{"sweep"}, checksOf(nil, deskkit.StatusContext{Context: "sweep", State: "failure", CreatedAt: t1}), `"sweep"`},
		{"an errored commit status", []string{"sweep"}, checksOf(nil, deskkit.StatusContext{Context: "sweep", State: "error", CreatedAt: t1}), `"sweep"`},
		{"the name is matched without case", []string{"Build"}, checksOf([]deskkit.CheckRun{run1("build", "completed", "failure", t1)}), `"Build"`},
		{"two required, one red", []string{"lint", "build"}, checksOf([]deskkit.CheckRun{
			run1("build", "completed", "failure", t1), run1("lint", "completed", "success", t1)}), `"build"`},
		{"a failed re-run after a green run", []string{"build"}, checksOf([]deskkit.CheckRun{
			run1("build", "completed", "success", t1), run1("build", "completed", "failure", t2)}), `"build"`},
		{"a failed status after a pending one", []string{"sweep"}, checksOf(nil,
			deskkit.StatusContext{Context: "sweep", State: "pending", CreatedAt: t1},
			deskkit.StatusContext{Context: "sweep", State: "failure", CreatedAt: t2}), `"sweep"`},

		{"a run in progress", []string{"build"}, checksOf([]deskkit.CheckRun{{Name: "build", Status: "in_progress", StartedAt: t1}}), ""},
		{"a queued run", []string{"build"}, checksOf([]deskkit.CheckRun{{Name: "build", Status: "queued"}}), ""},
		{"a pending commit status", []string{"sweep"}, checksOf(nil, deskkit.StatusContext{Context: "sweep", State: "pending", CreatedAt: t1}), ""},
		{"nothing reported under the name", []string{"build"}, checksOf(nil), ""},
		{"a cancelled run", []string{"build"}, checksOf([]deskkit.CheckRun{run1("build", "completed", "cancelled", t1)}), ""},
		{"a skipped run", []string{"build"}, checksOf([]deskkit.CheckRun{run1("build", "completed", "skipped", t1)}), ""},
		{"a stale run", []string{"build"}, checksOf([]deskkit.CheckRun{run1("build", "completed", "stale", t1)}), ""},
		{"a pending status after a failed one", []string{"sweep"}, checksOf(nil,
			deskkit.StatusContext{Context: "sweep", State: "failure", CreatedAt: t1},
			deskkit.StatusContext{Context: "sweep", State: "pending", CreatedAt: t2}), ""},
		{"a run awaiting approval", []string{"build"}, checksOf([]deskkit.CheckRun{run1("build", "completed", "action_required", t1)}), ""},
		{"a green run", []string{"build"}, checksOf([]deskkit.CheckRun{run1("build", "completed", "success", t1)}), ""},
		{"a re-run in progress after a failure", []string{"build"}, checksOf([]deskkit.CheckRun{
			run1("build", "completed", "failure", t1), {Name: "build", Status: "in_progress", StartedAt: t2}}), ""},
		{"a green re-run after a failure", []string{"build"}, checksOf([]deskkit.CheckRun{
			run1("build", "completed", "failure", t1), run1("build", "completed", "success", t2)}), ""},
		{"a failed status and a green run under one name", []string{"build"}, checksOf(
			[]deskkit.CheckRun{run1("build", "completed", "success", t1)},
			deskkit.StatusContext{Context: "build", State: "failure", CreatedAt: t1}), ""},
		{"a red check that is not required", []string{"build"}, checksOf([]deskkit.CheckRun{run1("optional", "completed", "failure", t1)}), ""},
		{"no required check", nil, checksOf([]deskkit.CheckRun{run1("build", "completed", "failure", t1)}), ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := strings.Join(redRequiredChecks(tc.required, tc.checks), ","); got != tc.want {
				t.Errorf("red = %q, want %q", got, tc.want)
			}
		})
	}
}

// The stale-description test: a head declared in the weight section that is not this head.
func TestStaleDescriptionIsADeclaredHeadThatIsNotThisHead(t *testing.T) {
	for _, tc := range []struct {
		name  string
		body  string
		stale bool
	}{
		{"declares this head, abbreviated", "## Weight\n\n- head `2222222`\n", false},
		{"declares this head, in full", "## Weight\n\nhead " + rrHead2 + "\n", false},
		{"declares this head in a table row", "## Weight\n\n| head 2222222222 | 12 |\n", false},
		{"declares another head", "## Weight\n\n- head `1111111`\n", true},
		{"declares another head, upper case", "## Weight\n\n- Head `1111111ABC`\n", true},
		{"declares another head in a quote", "### Weight (measured)\n\n> head 1111111\n", true},
		{"declares two heads, one of them this one", "## Weight\n\n- head `1111111`\n- head `2222222`\n", false},
		{"declares two heads, neither this one", "## Weight\n\n- head `1111111`\n- head `3333333`\n", true},

		{"no weight section", "Adds the widget.\n\n- head `1111111`\n", false},
		{"the head line is after the weight section ends", "## Weight\n\nnone\n\n## Notes\n\n- head `1111111`\n", false},
		{"a commit id in prose is not a declaration", "## Weight\n\nMeasured after the head 1111111 moved.\n", false},
		{"a word of letters is not a commit id", "## Weight\n\n- head deadbeef\n", false},
		{"six digits is not a commit id", "## Weight\n\n- head `111111`\n", false},
		{"an empty description", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			hold := staleDescriptionHold(&deskkit.PullRequest{HeadSHA: rrHead2, Body: tc.body})
			if (hold != nil) != tc.stale {
				t.Errorf("held = %v, want %v (declared heads: %v)", hold != nil, tc.stale, declaredHeads(tc.body))
			}
			if hold != nil && hold.reason != holdStaleDescription {
				t.Errorf("reason = %q", hold.reason)
			}
		})
	}
	// A head the forge did not serve as a commit id is never called stale.
	if hold := staleDescriptionHold(&deskkit.PullRequest{HeadSHA: "", Body: "## Weight\n\n- head `1111111`\n"}); hold != nil {
		t.Error("held on a head that could not be read")
	}
}

// A ruling is recorded when the ratifying identity wrote after the label's standing
// application; missing only when the complete history holds none.
func TestRulingRecordedSince(t *testing.T) {
	isRatifier := func(login string, _ int64) bool { return login == gateRatifier }
	labeled := func(when string) deskkit.LabelEvent { return deskkit.LabelEvent{Name: decisionLabel, CreatedAt: when} }
	removed := func(when string) deskkit.LabelEvent {
		return deskkit.LabelEvent{Name: decisionLabel, Removed: true, CreatedAt: when}
	}
	wrote := func(who, when string) deskkit.ContentEvent {
		return deskkit.ContentEvent{Author: who, CreatedAt: at(when)}
	}
	full := func(evs ...deskkit.ContentEvent) *deskkit.TrustPayload {
		return &deskkit.TrustPayload{Events: evs, Complete: true}
	}
	const d1, d2, d3, d4 = "2026-03-01T00:00:00Z", "2026-03-02T00:00:00Z", "2026-03-03T00:00:00Z", "2026-03-04T00:00:00Z"
	for _, tc := range []struct {
		name   string
		events []deskkit.LabelEvent
		trust  *deskkit.TrustPayload
		want   rulingState
	}{
		{"the ratifier wrote after the label", []deskkit.LabelEvent{labeled(d1)}, full(wrote(gateRatifier, d2)), rulingRecorded},
		{"nobody wrote", []deskkit.LabelEvent{labeled(d1)}, full(), rulingMissing},
		{"only someone else wrote", []deskkit.LabelEvent{labeled(d1)}, full(wrote("someone", d2)), rulingMissing},
		{"the ratifier wrote only before the label", []deskkit.LabelEvent{labeled(d2)}, full(wrote(gateRatifier, d1)), rulingMissing},
		{"the ratifier wrote at the label's instant", []deskkit.LabelEvent{labeled(d2)}, full(wrote(gateRatifier, d2)), rulingMissing},
		{"the label was re-applied after the ruling", []deskkit.LabelEvent{labeled(d1), removed(d3), labeled(d4)},
			full(wrote(gateRatifier, d2)), rulingMissing},
		{"a ruling after the re-application", []deskkit.LabelEvent{labeled(d1), removed(d2), labeled(d3)},
			full(wrote(gateRatifier, d4)), rulingRecorded},
		{"another label's events are not this label's", []deskkit.LabelEvent{labeled(d1), {Name: "bug", CreatedAt: d3}},
			full(wrote(gateRatifier, d2)), rulingRecorded},

		{"the history is incomplete and holds no ruling", []deskkit.LabelEvent{labeled(d1)},
			&deskkit.TrustPayload{Events: []deskkit.ContentEvent{wrote("someone", d2)}}, rulingUnknown},
		{"the history is incomplete but holds a ruling", []deskkit.LabelEvent{labeled(d1)},
			&deskkit.TrustPayload{Events: []deskkit.ContentEvent{wrote(gateRatifier, d2)}}, rulingRecorded},
		{"the label's application cannot be dated", []deskkit.LabelEvent{{Name: decisionLabel, CreatedAt: "yesterday"}}, full(), rulingUnknown},
		{"the label history holds no application", nil, full(), rulingUnknown},
		{"the label's last event is a removal", []deskkit.LabelEvent{labeled(d1), removed(d2)}, full(), rulingUnknown},
		{"no comment history", []deskkit.LabelEvent{labeled(d1)}, nil, rulingUnknown},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := rulingRecordedSince(tc.events, tc.trust, isRatifier); got != tc.want {
				t.Errorf("state = %d, want %d", got, tc.want)
			}
		})
	}
}

// --- the gate over a forge ---

const gateLabeled = "2026-03-01T00:00:00Z"

// prereqVerdict is a blocking correctness verdict at head whose typed block declares only
// external prerequisites, one per object.
func prereqVerdict(id int64, head string, objects ...string) deskkit.Review {
	var fs []deskkit.Finding
	for i, o := range objects {
		fs = append(fs, deskkit.Finding{ID: "P-" + string(rune('1'+i)), Class: "ruling", Severity: deskkit.SeverityBlocking,
			Blocker: deskkit.BlockerExternalPrereq, State: deskkit.StateOpen, Explanation: "waits on a ruling",
			Resolution: "the ruling is recorded", SharedRepair: o})
	}
	return verdict(id, laneCorrectness, "request-changes", head,
		"External-Prereq-Only: waits on a ruling", "", deskkit.RenderFindingBlock(deskkit.FindingBlockV1{Findings: fs}))
}

// Each hold fixture makes exactly one condition true on top of roundFixture.
var gateHoldFixtures = []struct {
	reason gateReason
	detail string // a fragment the refusal's detail line carries
	edit   func(*fakeRoundForge)
}{
	{holdRequiredCheckRed, `1 required check(s) failed on their latest completed run: "build"`, func(f *fakeRoundForge) {
		f.required = []string{"build", "lint"}
		f.checks = checksOf([]deskkit.CheckRun{
			run1("build", "completed", "failure", "2026-03-04T05:00:00Z"),
			{Name: "lint", Status: "in_progress", StartedAt: "2026-03-04T05:00:00Z"}})
	}},
	{holdStaleDescription, "declares head 1111111", func(f *fakeRoundForge) {
		f.pr.Body = "Adds the widget.\n\n## Weight\n\n- head `1111111`\n- verbs 3\n"
	}},
	{holdDecisionNotRuled, "the ratifying identity has written nothing on it since that label was applied", func(f *fakeRoundForge) {
		f.pr.Labels = []string{"bug", decisionLabel}
		f.labelEvents[77] = []deskkit.LabelEvent{{Name: decisionLabel, CreatedAt: gateLabeled}}
		f.trust[77] = &deskkit.TrustPayload{Complete: true, Events: []deskkit.ContentEvent{
			{Author: "someone", CreatedAt: at("2026-03-02T00:00:00Z")}}}
	}},
	{holdLaneAwaitsRuling, "review 502 at this head blocked only on rulings, and no ruling is recorded yet on #900", func(f *fakeRoundForge) {
		f.reviews = append(f.reviews, prereqVerdict(502, rrHead2, "#900"))
		f.issues[900] = &deskkit.Issue{Number: 900, State: "open", Labels: []string{decisionLabel}}
		f.labelEvents[900] = []deskkit.LabelEvent{{Name: decisionLabel, CreatedAt: gateLabeled}}
		f.trust[900] = &deskkit.TrustPayload{Complete: true}
	}},
}

// Each hold condition holds, alone, with its own reason and a refusal naming the change and
// the head; and the fixture it is built on does not hold.
func TestGateHoldsOnEachCondition(t *testing.T) {
	useRatifier(t, true)
	if _, err := roundOf(t, roundFixture(), rrKeyC); err != nil {
		t.Fatalf("the base fixture is held: %v", err)
	}
	for _, tc := range gateHoldFixtures {
		t.Run(string(tc.reason), func(t *testing.T) {
			f := roundFixture()
			tc.edit(f)
			_, err := roundOf(t, f, rrKeyC)
			if err == nil {
				t.Fatal("not held")
			}
			if deskkit.ExitCodeOf(err) != deskkit.ExitRefused {
				t.Errorf("exit = %d, want the refusal code %d", deskkit.ExitCodeOf(err), deskkit.ExitRefused)
			}
			first := firstLine(err.Error())
			want := heldMarker + " — " + rrRepo + "#77 at head " + rrHead2 + ": " + string(tc.reason) + ". " +
				"No reviewer was dispatched, no claim was taken and nothing is recorded as reviewed; this is a delay, not a verdict."
			if first != want {
				t.Errorf("first line = %q\n               want %q", first, want)
			}
			if !strings.Contains(err.Error(), "\n  - "+string(tc.reason)+": ") || !strings.Contains(err.Error(), tc.detail) {
				t.Errorf("refusal lacks the detail %q:\n%s", tc.detail, err)
			}
		})
	}
}

// Two conditions at once are both named, in the gate's order.
func TestGateNamesEveryReason(t *testing.T) {
	useRatifier(t, true)
	f := roundFixture()
	for _, tc := range gateHoldFixtures {
		tc.edit(f)
	}
	_, err := roundOf(t, f, rrKeyC)
	if err == nil {
		t.Fatal("not held")
	}
	if !strings.Contains(firstLine(err.Error()), ": required-check-red, description-stale, decision-not-ruled, lane-awaits-ruling. ") {
		t.Errorf("first line does not name all four reasons in order: %q", firstLine(err.Error()))
	}
}

// The gate holds whatever the lane: a dispatch whose key names no single lane, or with the
// reviewer unbound or the reviews unreadable, is still held on the three conditions that do
// not depend on the lane's verdicts.
func TestGateHoldsWithoutALane(t *testing.T) {
	useRatifier(t, true)
	for _, tc := range gateHoldFixtures[:3] {
		for name, degrade := range map[string]func(*testing.T, *fakeRoundForge) string{
			"lane-less key":      func(*testing.T, *fakeRoundForge) string { return "tracker--pr-77--fact-check" },
			"reviews unreadable": func(_ *testing.T, f *fakeRoundForge) string { f.reviewsErr = errors.New("503"); return rrKeyC },
			"security lane":      func(*testing.T, *fakeRoundForge) string { return rrKeyS },
		} {
			t.Run(string(tc.reason)+"/"+name, func(t *testing.T) {
				f := roundFixture()
				tc.edit(f)
				key := degrade(t, f)
				if _, err := roundOf(t, f, key); err == nil || !strings.Contains(err.Error(), string(tc.reason)) {
					t.Errorf("not held on %s: %v", tc.reason, err)
				}
			})
		}
	}
}

// A read that fails dispatches: for each hold fixture, each read its condition depends on is
// failed in turn, and the dispatch goes ahead with a note that names the condition's read.
func TestGateReadFailureDispatches(t *testing.T) {
	boom := errors.New("503")
	for _, tc := range []struct {
		name   string
		base   gateReason
		break_ func(t *testing.T, f *fakeRoundForge)
		note   string // "" = no note expected (the condition is simply not evaluable)
	}{
		{"required checks unreadable", holdRequiredCheckRed, func(_ *testing.T, f *fakeRoundForge) { f.requiredErr = boom },
			"could not read the base branch's required checks"},
		{"checks at head unreadable", holdRequiredCheckRed, func(_ *testing.T, f *fakeRoundForge) { f.checksErr = boom },
			"could not read the checks at the head"},
		{"checks at head missing", holdRequiredCheckRed, func(_ *testing.T, f *fakeRoundForge) { f.checks = nil },
			"could not read the checks at the head"},
		{"check runs served in part", holdRequiredCheckRed, func(_ *testing.T, f *fakeRoundForge) { f.checks.CheckRunsTotalCount = 9 },
			"the forge served only part of the checks at the head"},
		{"statuses served in part", holdRequiredCheckRed, func(_ *testing.T, f *fakeRoundForge) { f.checks.StatusTotalCount = 9 },
			"the forge served only part of the checks at the head"},
		{"base branch unknown", holdRequiredCheckRed, func(_ *testing.T, f *fakeRoundForge) { f.pr.BaseRef = "" },
			"the change's base branch or head could not be read"},

		{"label history unreadable", holdDecisionNotRuled, func(_ *testing.T, f *fakeRoundForge) { f.labelErr = boom },
			"whether a ruling is recorded could not be read"},
		{"comment history unreadable", holdDecisionNotRuled, func(_ *testing.T, f *fakeRoundForge) { f.trustErr = boom },
			"whether a ruling is recorded could not be read"},
		{"comment history incomplete", holdDecisionNotRuled, func(_ *testing.T, f *fakeRoundForge) { f.trust[77].Complete = false },
			"whether a ruling is recorded could not be read"},
		{"label application undatable", holdDecisionNotRuled, func(_ *testing.T, f *fakeRoundForge) { f.labelEvents[77][0].CreatedAt = "" },
			"whether a ruling is recorded could not be read"},
		{"no ratifying identity configured", holdDecisionNotRuled, func(t *testing.T, _ *fakeRoundForge) { useRatifier(t, false) },
			"whether a ruling is recorded could not be read"},

		{"prerequisite item unreadable", holdLaneAwaitsRuling, func(_ *testing.T, f *fakeRoundForge) { f.issueErr = boom },
			"blocks on #900, which could not be read"},
		{"prerequisite item missing", holdLaneAwaitsRuling, func(_ *testing.T, f *fakeRoundForge) { delete(f.issues, 900) },
			"blocks on #900, which could not be read"},
		{"prerequisite's comment history incomplete", holdLaneAwaitsRuling, func(_ *testing.T, f *fakeRoundForge) { f.trust[900].Complete = false },
			"blocks on #900, whose ruling state could not be read"},
		{"prerequisite's label history unreadable", holdLaneAwaitsRuling, func(_ *testing.T, f *fakeRoundForge) { f.labelErr = boom },
			"blocks on #900, whose ruling state could not be read"},
		{"the lane's reviews unreadable", holdLaneAwaitsRuling, func(_ *testing.T, f *fakeRoundForge) { f.reviewsErr = boom },
			"could not read the change's reviews"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			useRatifier(t, true)
			f := roundFixture()
			for _, h := range gateHoldFixtures {
				if h.reason == tc.base {
					h.edit(f)
				}
			}
			tc.break_(t, f)
			var err error
			stderr := captureStderr(t, func() { _, err = roundOf(t, f, rrKeyC) })
			if err != nil {
				t.Fatalf("a failed read held the dispatch: %v", err)
			}
			if !strings.Contains(stderr, "deskdispatch: review round: ") || !strings.Contains(stderr, tc.note) {
				t.Errorf("stderr does not report the failed read (%q):\n%s", tc.note, stderr)
			}
		})
	}
}

// What does NOT hold: the conditions' near misses, each of which dispatches silently.
func TestGateNearMissesDispatch(t *testing.T) {
	for _, tc := range []struct {
		name string
		base gateReason
		edit func(f *fakeRoundForge)
	}{
		{"the required check is still running", holdRequiredCheckRed, func(f *fakeRoundForge) {
			f.checks = checksOf([]deskkit.CheckRun{{Name: "build", Status: "in_progress", StartedAt: "2026-03-04T05:00:00Z"}})
		}},
		{"the required check has not reported", holdRequiredCheckRed, func(f *fakeRoundForge) { f.checks = checksOf(nil) }},
		{"the red check is not required", holdRequiredCheckRed, func(f *fakeRoundForge) { f.required = []string{"lint"} }},
		{"the branch requires no check", holdRequiredCheckRed, func(f *fakeRoundForge) { f.required = nil }},

		{"the description declares no head", holdStaleDescription, func(f *fakeRoundForge) { f.pr.Body = "## Weight\n\n- verbs 3\n" }},
		{"the description declares this head", holdStaleDescription, func(f *fakeRoundForge) { f.pr.Body = "## Weight\n\n- head `2222222`\n" }},

		{"the change carries no decision label", holdDecisionNotRuled, func(f *fakeRoundForge) { f.pr.Labels = []string{"bug"} }},
		{"the ruling is recorded", holdDecisionNotRuled, func(f *fakeRoundForge) {
			f.trust[77].Events = append(f.trust[77].Events, deskkit.ContentEvent{Author: gateRatifier, CreatedAt: at("2026-03-03T00:00:00Z")})
		}},

		{"the ruling landed on the prerequisite", holdLaneAwaitsRuling, func(f *fakeRoundForge) {
			f.trust[900].Events = []deskkit.ContentEvent{{Author: gateRatifier, CreatedAt: at("2026-03-03T00:00:00Z")}}
		}},
		{"the prerequisite is closed", holdLaneAwaitsRuling, func(f *fakeRoundForge) { f.issues[900].State = "closed" }},
		{"the prerequisite is not a decision", holdLaneAwaitsRuling, func(f *fakeRoundForge) { f.issues[900].Labels = []string{"bug"} }},
		{"the blocking verdict is at an earlier head", holdLaneAwaitsRuling, func(f *fakeRoundForge) { f.reviews[1].CommitID = rrHead1 }},
		{"the lane's latest verdict approves", holdLaneAwaitsRuling, func(f *fakeRoundForge) {
			f.reviews = append(f.reviews, verdict(503, laneCorrectness, "approve", rrHead2))
		}},
		{"an approving verdict that carries the declaration", holdLaneAwaitsRuling, func(f *fakeRoundForge) {
			f.reviews[1].Body = strings.Replace(f.reviews[1].Body, "Verdict: request-changes", "Verdict: approve", 1)
		}},
		{"an approval after the blocked verdict, at the same head", holdLaneAwaitsRuling, func(f *fakeRoundForge) {
			f.reviews = []deskkit.Review{f.reviews[1], verdict(503, laneCorrectness, "approve", rrHead2)}
		}},
		{"the verdict also carries a content blocker", holdLaneAwaitsRuling, func(f *fakeRoundForge) {
			f.reviews[1] = verdict(502, laneCorrectness, "request-changes", rrHead2, "External-Prereq-Only: waits on a ruling", "",
				deskkit.RenderFindingBlock(deskkit.FindingBlockV1{Findings: []deskkit.Finding{
					{ID: "P-1", Class: "ruling", Severity: deskkit.SeverityBlocking, Blocker: deskkit.BlockerExternalPrereq,
						State: deskkit.StateOpen, Explanation: "waits", SharedRepair: "#900"},
					{ID: "F-9", Class: "nil-deref", Severity: deskkit.SeverityBlocking, Blocker: deskkit.BlockerCodeContent,
						State: deskkit.StateOpen, Failure: "it panics"}}}))
		}},
		{"the verdict does not declare prerequisites only", holdLaneAwaitsRuling, func(f *fakeRoundForge) {
			f.reviews[1].Body = strings.Replace(f.reviews[1].Body, "External-Prereq-Only: waits on a ruling", "", 1)
		}},
		{"a prerequisite is in another repository", holdLaneAwaitsRuling, func(f *fakeRoundForge) {
			f.reviews[1] = prereqVerdict(502, rrHead2, "#900", "other-org/other#4")
			// This repository's own #4 is an unruled decision; the reference is not to it.
			f.issues[4] = &deskkit.Issue{Number: 4, State: "open", Labels: []string{decisionLabel}}
			f.labelEvents[4] = []deskkit.LabelEvent{{Name: decisionLabel, CreatedAt: gateLabeled}}
			f.trust[4] = &deskkit.TrustPayload{Complete: true}
		}},
		{"a prerequisite is not an item reference", holdLaneAwaitsRuling, func(f *fakeRoundForge) {
			f.reviews[1] = prereqVerdict(502, rrHead2, "#900", "the shared CI repair")
		}},
		{"the blocking verdict is the other lane's", holdLaneAwaitsRuling, func(f *fakeRoundForge) {
			f.reviews[1].Body = strings.Replace(f.reviews[1].Body, "Verdict: request-changes", "Security-Review: fail", 1)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			useRatifier(t, true)
			f := roundFixture()
			for _, h := range gateHoldFixtures {
				if h.reason == tc.base {
					h.edit(f)
				}
			}
			tc.edit(f)
			if _, err := roundOf(t, f, rrKeyC); err != nil {
				t.Errorf("held: %v", firstLine(err.Error()))
			}
		})
	}
}

// A prerequisite named in full as this repository's item is read like a bare #N; a lane
// waiting on two rulings is held until both land.
func TestLaneAwaitsEveryRuling(t *testing.T) {
	useRatifier(t, true)
	build := func() *fakeRoundForge {
		f := roundFixture()
		f.reviews = append(f.reviews, prereqVerdict(502, rrHead2, rrRepo+"#900", "#901"))
		for _, n := range []int{900, 901} {
			f.issues[n] = &deskkit.Issue{Number: n, State: "open", Labels: []string{decisionLabel}}
			f.labelEvents[n] = []deskkit.LabelEvent{{Name: decisionLabel, CreatedAt: gateLabeled}}
			f.trust[n] = &deskkit.TrustPayload{Complete: true}
		}
		return f
	}
	ruled := deskkit.ContentEvent{Author: gateRatifier, CreatedAt: at("2026-03-03T00:00:00Z")}

	f := build()
	if _, err := roundOf(t, f, rrKeyC); err == nil || !strings.Contains(err.Error(), "no ruling is recorded yet on #900, #901") {
		t.Errorf("two unruled prerequisites: %v", err)
	}
	f = build()
	f.trust[900].Events = []deskkit.ContentEvent{ruled}
	if _, err := roundOf(t, f, rrKeyC); err == nil || !strings.Contains(err.Error(), "no ruling is recorded yet on #901") ||
		strings.Contains(err.Error(), "#900") {
		t.Errorf("one of two ruled: %v", err)
	}
	f = build()
	f.trust[900].Events, f.trust[901].Events = []deskkit.ContentEvent{ruled}, []deskkit.ContentEvent{ruled}
	if _, err := roundOf(t, f, rrKeyC); err != nil {
		t.Errorf("both ruled, still held: %v", err)
	}
	// A prerequisite that is itself a change is read through the change's own history.
	f = build()
	f.issues[900].IsPullRequest = true
	_, _ = roundOf(t, f, rrKeyC)
	if f.called("ListLabelEvents 900") != 1 || f.called("PRTrustEvents 900") != 1 || f.called("ListIssueLabelEvents 900") != 0 {
		t.Errorf("a prerequisite that is a change was not read as one: %v", f.calls)
	}
}

// --- end to end, through run() ---

type heldRun struct {
	rc         int
	stderr     string
	home       string
	promptFile string
	stub       *stub
	gh         *ghStampServer
	queue      *[]queueLabelCall
	forge      *fakeRoundForge
}

// dispatchReview runs one real (stubbed) review dispatch of change 77 over f.
func dispatchReview(t *testing.T, f *fakeRoundForge, extra ...string) heldRun {
	t.Helper()
	s := &stub{}
	home, root := s.install(t)
	pinFixtureForge(t, home, allowedRepo)
	plantScripts(t, root)
	s.replies = happyReplies(filepath.Join(t.TempDir(), "review-home"))
	t.Setenv("DESK_LOOP", "pr-review-desk")
	gh := installGHStamp(t)
	queue := stubQueueLabel(t, nil)
	useRoundForge(t, f)
	useRatifier(t, true)
	args := stampArgs(t, root, append([]string{"--kit", "review", "--quiet"}, extra...)...)
	out := heldRun{home: home, promptFile: promptFileOf(t, args), stub: s, gh: gh, queue: queue, forge: f}
	out.rc, out.stderr = runCapturingStderr(t, args)
	return out
}

func auditRows(t *testing.T, home string) []deskkit.Entry {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(home, ".config", "assay", "audit.jsonl"))
	if err != nil {
		t.Fatalf("no audit log: %v", err)
	}
	var rows []deskkit.Entry
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		var row deskkit.Entry
		if err := json.Unmarshal([]byte(line), &row); err != nil {
			t.Fatalf("audit line %q: %v", line, err)
		}
		rows = append(rows, row)
	}
	return rows
}

// A held head never appears as reviewed, claimed or dispatched: for each hold condition the
// run exits with the refusal code, names the head and the reason, and leaves no claim, no
// worktree, no prompt, no forge request and no credential — only its own audit line.
func TestHeldDispatchLeavesNothingBehind(t *testing.T) {
	for _, tc := range gateHoldFixtures {
		t.Run(string(tc.reason), func(t *testing.T) {
			f := roundFixture()
			tc.edit(f)
			h := dispatchReview(t, f)
			if h.rc != deskkit.ExitRefused {
				t.Fatalf("rc = %d, want the refusal code %d\n%s", h.rc, deskkit.ExitRefused, h.stderr)
			}
			line := heldMarker + " — " + allowedRepo + "#77 at head " + rrHead2 + ": " + string(tc.reason) + ". "
			if !strings.Contains(h.stderr, line) {
				t.Errorf("stderr does not name the held head and why (%q):\n%s", line, h.stderr)
			}
			if _, err := os.Stat(h.promptFile); !os.IsNotExist(err) {
				t.Error("a held dispatch wrote a reviewer prompt")
			}
			for _, c := range h.stub.calls {
				joined := strings.Join(c, " ")
				if strings.Contains(joined, "dispatch-claim") || strings.Contains(joined, "worktree") {
					t.Errorf("a held dispatch ran %q", joined)
				}
			}
			if h.gh.wrote() || len(h.gh.requests) != 0 || len(h.gh.minted) != 0 {
				t.Errorf("a held dispatch reached the forge outside the gate's own reads: wrote=%v, %d request(s), %d credential(s)",
					h.gh.wrote(), len(h.gh.requests), len(h.gh.minted))
			}
			if len(*h.queue) != 0 {
				t.Errorf("a held dispatch applied the review-queue label: %v", *h.queue)
			}
			rows := auditRows(t, h.home)
			if len(rows) != 1 {
				t.Fatalf("%d audit rows, want exactly 1: %+v", len(rows), rows)
			}
			row := rows[0]
			if row.Verb != "dispatch" || row.Result != deskkit.ResultRefused || row.Title != "assay--pr-77" ||
				!strings.HasPrefix(row.Detail, line) || strings.Contains(row.Detail, "\n") {
				t.Errorf("audit row does not record the hold: %+v", row)
			}
		})
	}
}

// The same four fixtures under --dry-run: nothing is read, nothing is held.
func TestDryRunIsNeverHeld(t *testing.T) {
	for _, tc := range gateHoldFixtures {
		f := roundFixture()
		tc.edit(f)
		h := dispatchReview(t, f, "--dry-run")
		if h.rc != deskkit.ExitOK || len(f.calls) != 0 {
			t.Errorf("%s: --dry-run rc = %d with %d gate read(s) — want 0 and none", tc.reason, h.rc, len(f.calls))
		}
	}
}

// A failed read dispatches, end to end: the change cannot be read at all, and the reviewer
// is dispatched with a full pass.
func TestFailedGateReadStillDispatchesTheReviewer(t *testing.T) {
	f := roundFixture()
	gateHoldFixtures[0].edit(f)
	f.pr, f.prErr = nil, errors.New("503")
	h := dispatchReview(t, f)
	if h.rc != deskkit.ExitOK {
		t.Fatalf("rc = %d, want 0\n%s", h.rc, h.stderr)
	}
	if !strings.Contains(h.stderr, "deskdispatch: review round: could not read the change — dispatching") {
		t.Errorf("stderr does not report the failed read:\n%s", h.stderr)
	}
	raw, err := os.ReadFile(h.promptFile)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "- Scope: FULL PASS — reason `could-not-determine`: could not read the change.") {
		t.Error("the reviewer's prompt does not state a full pass")
	}
	if !h.stub.ran("dispatch-claim.sh acquire") {
		t.Error("the dispatch did not claim")
	}
}

// Pending does not hold, end to end: a required check still running dispatches the reviewer,
// and the round's note-free stderr says nothing about the gate.
func TestPendingRequiredCheckDispatches(t *testing.T) {
	f := roundFixture()
	f.required = []string{"build"}
	f.checks = checksOf([]deskkit.CheckRun{{Name: "build", Status: "in_progress", StartedAt: "2026-03-04T05:00:00Z"}},
		deskkit.StatusContext{Context: "sweep", State: "pending", CreatedAt: "2026-03-04T05:00:00Z"})
	h := dispatchReview(t, f)
	if h.rc != deskkit.ExitOK {
		t.Fatalf("rc = %d, want 0\n%s", h.rc, h.stderr)
	}
	if strings.Contains(h.stderr, heldMarker) || strings.Contains(h.stderr, "review round:") {
		t.Errorf("a pending check produced gate output:\n%s", h.stderr)
	}
	if _, err := os.Stat(h.promptFile); err != nil {
		t.Errorf("no reviewer prompt: %v", err)
	}
}

// The gate's refusal is the ONLY error the pre-claim read returns: every other outcome of
// every fixture in this file is a nil error. Pinned over the cross product of hold fixtures
// and read failures: an error is returned exactly when a hold reason is named in it.
func TestRoundReadErrorsAreHoldsAndNothingElse(t *testing.T) {
	useRatifier(t, true)
	boom := errors.New("503")
	breaks := map[string]func(*fakeRoundForge){
		"none":       func(*fakeRoundForge) {},
		"reviews":    func(f *fakeRoundForge) { f.reviewsErr = boom },
		"files":      func(f *fakeRoundForge) { f.filesErr = boom },
		"checks":     func(f *fakeRoundForge) { f.checksErr = boom },
		"required":   func(f *fakeRoundForge) { f.requiredErr = boom },
		"labels":     func(f *fakeRoundForge) { f.labelErr = boom },
		"trust":      func(f *fakeRoundForge) { f.trustErr = boom },
		"issues":     func(f *fakeRoundForge) { f.issueErr = boom },
		"commits":    func(f *fakeRoundForge) { f.commitErr = boom },
		"comparison": func(f *fakeRoundForge) { f.compareErr[rrHead1+"..."+rrHead2] = boom },
	}
	for name, brk := range breaks {
		for i := -1; i < len(gateHoldFixtures); i++ {
			f := roundFixture()
			if i >= 0 {
				gateHoldFixtures[i].edit(f)
			}
			brk(f)
			_, err := roundOf(t, f, rrKeyC)
			if err == nil {
				continue
			}
			if deskkit.ExitCodeOf(err) != deskkit.ExitRefused || !strings.HasPrefix(err.Error(), heldMarker+" — ") {
				t.Errorf("break %q, fixture %d: an error that is not a hold: %v", name, i, err)
			}
			if i < 0 {
				t.Errorf("break %q with no hold condition true returned an error: %v", name, err)
			}
		}
	}
}

// The lane condition reads "this lane's latest verdict is at this head" off the forge's
// record of that verdict's commit. Where the verdict's own text names a different head, that
// record is disputed, so the condition is not evaluated: the head is dispatched as a full
// pass and never held on a verdict that may not be at it.
func TestLaneAwaitsRulingIsNotReadOffADisputedHead(t *testing.T) {
	useRatifier(t, true)
	build := func(named string) *fakeRoundForge {
		f := roundFixture()
		gateHoldFixtures[3].edit(f)
		f.reviews[1].Body += "\nHead reviewed: `" + named + "`\n"
		return f
	}
	if gateHoldFixtures[3].reason != holdLaneAwaitsRuling {
		t.Fatalf("fixture 3 is %q, want the lane condition", gateHoldFixtures[3].reason)
	}
	if _, err := roundOf(t, build(rrHead2), rrKeyC); err == nil || !strings.Contains(err.Error(), string(holdLaneAwaitsRuling)) {
		t.Fatalf("a verdict whose body names this head is not held on the lane condition: %v", err)
	}
	f := build(rrHead1)
	rr, err := roundOf(t, f, rrKeyC)
	if err != nil {
		t.Fatalf("held on a verdict whose own text names another head: %v", err)
	}
	if reasonsOf(rr.scope) != "could-not-determine" || rr.round.known {
		t.Errorf("scope %q, round %+v — want could-not-determine and not determined", reasonsOf(rr.scope), rr.round)
	}
	if n := f.called("GetIssue"); n != 0 {
		t.Errorf("the lane condition was evaluated (%d prerequisite read(s)) on a disputed head", n)
	}
}
