package main

// Tests for the findings-register bounded-shelving state machine (statusgen/06):
// the ALARM half (a live well-formed park is a snooze — suppressed and excluded
// from flood; an expired park re-annunciates louder and counts again; a malformed
// park does not shelve and raises a hard --lint PROBLEM) and the GUARD half
// (adding or extending a parked-until on a landed finding is a guarded mutation
// exactly like a resolve/affects gut, authorized only by a mapped human under
// parked-by — never authorized-by — and never past the 90-day horizon).

import (
	"os"
	"strings"
	"testing"
	"time"
)

// pinParkNow pins the park-horizon clock to day (YYYY-MM-DD) for one test.
func pinParkNow(t *testing.T, day string) {
	t.Helper()
	at := mustTime(t, day)
	old := parkHorizonNow
	parkHorizonNow = func() time.Time { return at }
	t.Cleanup(func() { parkHorizonNow = old })
}

// parkedFinding constructs a finding carrying a bounded park.
func parkedFinding(id, date, title, until, by, reason string) Finding {
	return Finding{ID: id, Date: date, Title: title,
		ParkedUntil: until, ParkedBy: by, ParkedReason: reason}
}

// ----- ALARM half: shelving suppression, re-annunciation, flood accounting -----

// A live, well-formed park suppresses the standing NOTICE, is excluded from the
// active/flood count, and shows up in AlarmReport.Parked — a consciously-shelved
// finding no longer alarms identically to a neglected one.
func TestParkActiveSuppressesStandingAlarm(t *testing.T) {
	now := mustTime(t, "2026-07-30")
	findings := []Finding{
		// Opened long ago (would be a standing alarm past the 7-day threshold),
		// but parked until a FUTURE date with all three fields.
		parkedFinding("F-park", "2026-06-01", "accepted-deferred", "2026-09-01", "human:alex", "waiting on upstream fix"),
	}
	cfg := AlarmConfig{StandingAgeDays: defaultStandingAgeDays, FloodThreshold: defaultFloodThreshold}
	rep := computeAlarms(findings, cfg, now)

	if rep.ActiveCount != 0 {
		t.Errorf("a live park must be excluded from ActiveCount; got %d", rep.ActiveCount)
	}
	if len(rep.Standing) != 0 {
		t.Errorf("a live park must not appear in Standing; got %v", rep.Standing)
	}
	if len(rep.Parked) != 1 || rep.Parked[0].ID != "F-park" {
		t.Errorf("a live park must appear in Parked; got %v", rep.Parked)
	}
	notices := standingAlarmNotices(findings, cfg, now)
	for _, n := range notices {
		if strings.Contains(n, "standing alarm: F-park") {
			t.Errorf("a live park must suppress its standing NOTICE; got %q", n)
		}
	}
}

// An expired park (now >= parked-until) re-annunciates with a distinct, LOUDER
// NOTICE and counts toward the active/flood total again — a park never silently
// becomes permanent.
func TestExpiredParkReAnnunciatesLouder(t *testing.T) {
	now := mustTime(t, "2026-07-30")
	findings := []Finding{
		parkedFinding("F-exp", "2026-06-01", "window closed", "2026-07-15", "human:alex", "deferred one cycle"),
	}
	cfg := AlarmConfig{StandingAgeDays: defaultStandingAgeDays, FloodThreshold: defaultFloodThreshold}
	rep := computeAlarms(findings, cfg, now)

	if rep.ActiveCount != 1 {
		t.Errorf("an expired park must count toward ActiveCount again; got %d", rep.ActiveCount)
	}
	if len(rep.ExpiredParks) != 1 || rep.ExpiredParks[0].ID != "F-exp" {
		t.Fatalf("an expired park must appear in ExpiredParks; got %v", rep.ExpiredParks)
	}
	// It must NOT be double-reported as a plain standing alarm.
	for _, s := range rep.Standing {
		if s.ID == "F-exp" {
			t.Errorf("an expired park must not also appear in Standing; got %v", rep.Standing)
		}
	}
	notices := standingAlarmNotices(findings, cfg, now)
	var sawLoud, sawPlain bool
	for _, n := range notices {
		if strings.Contains(n, "park EXPIRED") && strings.Contains(n, "F-exp") {
			sawLoud = true
		}
		if strings.Contains(n, "standing alarm: F-exp") {
			sawPlain = true
		}
	}
	if !sawLoud {
		t.Errorf("expected a louder 'park EXPIRED' re-annunciation NOTICE for F-exp; got %v", notices)
	}
	if sawPlain {
		t.Errorf("expired park must not emit the plain standing-alarm NOTICE (indistinguishable from fresh); got %v", notices)
	}
}

// A live park is excluded from the flood count; the same set with the park
// EXPIRED tips into flood — proof the exclusion is bounded by the window.
func TestActiveParkExcludedFromFloodButExpiredCounts(t *testing.T) {
	now := mustTime(t, "2026-07-30")
	cfg := AlarmConfig{StandingAgeDays: defaultStandingAgeDays, FloodThreshold: 1}

	// One ordinary active finding + one park. With the park LIVE, active count is
	// 1 (≤ threshold) → no flood.
	live := []Finding{
		finding("F-open", "2026-07-29", "ordinary open", false),
		parkedFinding("F-park", "2026-06-01", "shelved", "2026-09-01", "human:alex", "reason"),
	}
	if rep := computeAlarms(live, cfg, now); rep.Flood {
		t.Errorf("a live park must not push the register into flood; ActiveCount=%d", rep.ActiveCount)
	}

	// With the park EXPIRED, active count is 2 (> threshold) → flood.
	expired := []Finding{
		finding("F-open", "2026-07-29", "ordinary open", false),
		parkedFinding("F-park", "2026-06-01", "shelved", "2026-07-01", "human:alex", "reason"),
	}
	if rep := computeAlarms(expired, cfg, now); !rep.Flood {
		t.Errorf("an expired park must count toward flood again; ActiveCount=%d, want flood", rep.ActiveCount)
	}
}

// A malformed park (a required field missing) does NOT shelve — the finding still
// alarms as an ordinary open finding, and parkFieldProblems raises a hard PROBLEM.
func TestMalformedParkDoesNotShelveAndProblems(t *testing.T) {
	now := mustTime(t, "2026-07-30")
	findings := []Finding{
		// parked-until in the future, parked-by present, but parked-reason MISSING.
		parkedFinding("F-bad", "2026-06-01", "half-parked", "2026-09-01", "human:alex", ""),
	}
	cfg := AlarmConfig{StandingAgeDays: defaultStandingAgeDays, FloodThreshold: defaultFloodThreshold}
	rep := computeAlarms(findings, cfg, now)
	if rep.ActiveCount != 1 {
		t.Errorf("a malformed park must NOT shelve — it still counts as active; got %d", rep.ActiveCount)
	}
	if len(rep.Parked) != 0 {
		t.Errorf("a malformed park must not appear in Parked; got %v", rep.Parked)
	}
	if p := parkFieldProblems(findings); !containsSubstr(p, "malformed park") || !containsSubstr(p, "parked-reason") {
		t.Errorf("a park missing parked-reason must raise a PROBLEM naming it; got %v", p)
	}
}

// parkFieldProblems fires on each individual required-field defect, and stays
// silent on a fully well-formed park.
func TestParkFieldProblemsMatrix(t *testing.T) {
	cases := []struct {
		name    string
		f       Finding
		wantSub string // "" = no problem expected
	}{
		{"well-formed", parkedFinding("F-ok", "2026-06-01", "t", "2026-09-01", "human:alex", "why"), ""},
		{"missing until", parkedFinding("F-a", "2026-06-01", "t", "", "human:alex", "why"), "parked-until"},
		{"unparseable until", parkedFinding("F-b", "2026-06-01", "t", "next week", "human:alex", "why"), "parseable parked-until"},
		{"missing by", parkedFinding("F-c", "2026-06-01", "t", "2026-09-01", "", "why"), "parked-by"},
		{"non-human by", parkedFinding("F-d", "2026-06-01", "t", "2026-09-01", "the desk", "why"), "human:<name> form"},
		{"missing reason", parkedFinding("F-e", "2026-06-01", "t", "2026-09-01", "human:alex", ""), "parked-reason"},
		{"not a park", finding("F-f", "2026-06-01", "t", false), ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := parkFieldProblems([]Finding{tc.f})
			if tc.wantSub == "" {
				if len(p) != 0 {
					t.Fatalf("expected no PROBLEM; got %v", p)
				}
				return
			}
			if !containsSubstr(p, tc.wantSub) {
				t.Fatalf("expected a PROBLEM containing %q; got %v", tc.wantSub, p)
			}
		})
	}
}

// A stale park on a RESOLVED finding never overrides the resolve.
func TestResolvedFindingIgnoresStalePark(t *testing.T) {
	now := mustTime(t, "2026-07-30")
	f := parkedFinding("F-res", "2026-06-01", "resolved with stale park", "2026-07-01", "human:alex", "why")
	f.Resolved = true
	rep := computeAlarms([]Finding{f}, AlarmConfig{StandingAgeDays: 7, FloodThreshold: 7}, now)
	if rep.ActiveCount != 0 || len(rep.ExpiredParks) != 0 || len(rep.Parked) != 0 {
		t.Errorf("a resolved finding must not be treated as parked/active; rep=%+v", rep)
	}
}

// ----- GUARD half: park add/extend is a guarded merge-base mutation -----

const landedParkedFinding = "---\n" +
	"id: F-gut\n" +
	"date: \"2026-07-17\"\n" +
	"title: Register guard gap\n" +
	"affects: [\"stream-y\"]\n" +
	"resolved: false\n" +
	"parked-until: \"2026-08-01\"\n" +
	"parked-by: human:bot\n" +
	"parked-reason: deferred\n" +
	"---\n\nBody.\n"

// Adding a parked-until to a landed finding with no verified-human authorization
// is a HARD lint problem — an uncorroborated self-park mutes the standing alarm.
func TestGuttedParkAddedUnauthorized(t *testing.T) {
	root, path := gutFixture(t, landedOpenFinding)
	gutted := strings.Replace(landedOpenFinding, "resolved: false",
		"resolved: false\nparked-until: \"2099-01-01\"", 1)
	if err := os.WriteFile(path, []byte(gutted), 0o644); err != nil {
		t.Fatal(err)
	}
	p := guttedRegisterFields(root)
	if !containsSubstr(p, "field-gutting (unauthorized)") || !containsSubstr(p, "parked-until added") {
		t.Fatalf("adding parked-until without human auth must fire; got %v", p)
	}
}

// The same park add WITH a mapped human under parked-by (its authorizing party)
// passes the offline guard — parked-by is the park's authorized-by.
func TestGuttedParkAddedAuthorizedByParkedBy(t *testing.T) {
	pinParkNow(t, "2026-07-20")
	root, path := gutFixture(t, landedOpenFinding)
	gutted := strings.Replace(landedOpenFinding, "resolved: false",
		"resolved: false\nparked-until: \"2026-09-01\"\nparked-by: human:alex\nparked-reason: waiting", 1)
	if err := os.WriteFile(path, []byte(gutted), 0o644); err != nil {
		t.Fatal(err)
	}
	if p := guttedRegisterFields(root); len(p) != 0 {
		t.Fatalf("park add WITH parked-by: human:alex must pass; got %v", p)
	}
}

// A park add whose parked-by names an UNMAPPED human (an agent-written token) is
// not authorized — still fires. Fail-closed.
func TestGuttedParkAddedUnknownParkedByNotAuthorized(t *testing.T) {
	root, path := gutFixture(t, landedOpenFinding)
	gutted := strings.Replace(landedOpenFinding, "resolved: false",
		"resolved: false\nparked-until: \"2099-01-01\"\nparked-by: human:bot\nparked-reason: waiting", 1)
	if err := os.WriteFile(path, []byte(gutted), 0o644); err != nil {
		t.Fatal(err)
	}
	if p := guttedRegisterFields(root); !containsSubstr(p, "parked-until added") {
		t.Fatalf("unmapped parked-by must NOT authorize a park add; got %v", p)
	}
}

// Extending a landed park to a LATER date is a guarded mutation (it mutes the
// alarm longer); without a mapped human it is a HARD problem.
func TestGuttedParkExtendedUnauthorized(t *testing.T) {
	root, path := gutFixture(t, landedParkedFinding) // base parked-until 2026-08-01, parked-by human:bot (unmapped)
	gutted := strings.Replace(landedParkedFinding, "parked-until: \"2026-08-01\"",
		"parked-until: \"2099-01-01\"", 1)
	if err := os.WriteFile(path, []byte(gutted), 0o644); err != nil {
		t.Fatal(err)
	}
	p := guttedRegisterFields(root)
	if !containsSubstr(p, "field-gutting (unauthorized)") || !containsSubstr(p, "parked-until extended") {
		t.Fatalf("extending parked-until without human auth must fire; got %v", p)
	}
}

// Extending WITH a mapped human under parked-by passes.
func TestGuttedParkExtendedAuthorized(t *testing.T) {
	pinParkNow(t, "2026-07-20")
	root, path := gutFixture(t, landedParkedFinding)
	gutted := strings.Replace(landedParkedFinding, "parked-until: \"2026-08-01\"\nparked-by: human:bot",
		"parked-until: \"2026-09-01\"\nparked-by: human:alex", 1)
	if err := os.WriteFile(path, []byte(gutted), 0o644); err != nil {
		t.Fatal(err)
	}
	if p := guttedRegisterFields(root); len(p) != 0 {
		t.Fatalf("park extend WITH parked-by: human:alex must pass; got %v", p)
	}
}

// ----- #2012 ruling, item 2: only parked-by authorizes a park -----

// An `authorized-by` anchor naming a mapped human does NOT authorize a park add:
// the park's authority is its own parked-by, and here that names an unmapped
// account. Before the ruling the guard took either key, so this passed.
func TestParkAuthorizedByNotPark(t *testing.T) {
	pinParkNow(t, "2026-07-20")
	root, path := gutFixture(t, landedOpenFinding)
	gutted := strings.Replace(landedOpenFinding, "resolved: false",
		"resolved: false\nauthorized-by: human:alex\nparked-until: \"2026-09-01\"\nparked-by: human:bot\nparked-reason: waiting", 1)
	if err := os.WriteFile(path, []byte(gutted), 0o644); err != nil {
		t.Fatal(err)
	}
	p := guttedRegisterFields(root)
	if !containsSubstr(p, "field-gutting (unauthorized)") || !containsSubstr(p, "parked-until added") {
		t.Fatalf("authorized-by must not authorize a park add; got %v", p)
	}
}

// The second instance: an EXTENSION carried by authorized-by alone (parked-by
// still the landed, unmapped name) is unauthorized too.
func TestParkExtendAuthByNotPark(t *testing.T) {
	pinParkNow(t, "2026-07-20")
	root, path := gutFixture(t, landedParkedFinding)
	gutted := strings.Replace(landedParkedFinding, "parked-until: \"2026-08-01\"",
		"authorized-by: human:alex\nparked-until: \"2026-09-01\"", 1)
	if err := os.WriteFile(path, []byte(gutted), 0o644); err != nil {
		t.Fatal(err)
	}
	p := guttedRegisterFields(root)
	if !containsSubstr(p, "parked-until extended") {
		t.Fatalf("authorized-by must not authorize a park extend; got %v", p)
	}
}

// ----- #2012 ruling, item 1: the 90-day park horizon -----

// A park added more than 90 days past the day it is set is a PROBLEM even with a
// mapped parked-by: no authorization stretches the horizon. Day 90 itself passes.
func TestParkHorizonAddFails(t *testing.T) {
	pinParkNow(t, "2026-07-20") // + 90 days = 2026-10-18
	for _, tc := range []struct {
		until string
		fires bool
	}{{"2026-10-18", false}, {"2026-10-19", true}, {"2099-01-01", true}} {
		root, path := gutFixture(t, landedOpenFinding)
		gutted := strings.Replace(landedOpenFinding, "resolved: false",
			"resolved: false\nparked-until: \""+tc.until+"\"\nparked-by: human:alex\nparked-reason: waiting", 1)
		if err := os.WriteFile(path, []byte(gutted), 0o644); err != nil {
			t.Fatal(err)
		}
		p := guttedRegisterFields(root)
		if got := containsSubstr(p, "beyond the 90-day horizon"); got != tc.fires {
			t.Errorf("park add until %s: horizon PROBLEM = %v, want %v; got %v", tc.until, got, tc.fires, p)
		}
		if containsSubstr(p, "field-gutting (unauthorized)") {
			t.Errorf("park add until %s is authorized by parked-by; got %v", tc.until, p)
		}
	}
}

// The planted second instance: an EXTENSION of a landed, in-horizon park that
// crosses the 90 days is a PROBLEM too, however it is authorized.
func TestParkHorizonExtendFails(t *testing.T) {
	pinParkNow(t, "2026-07-20")
	root, path := gutFixture(t, landedParkedFinding) // landed parked-until 2026-08-01
	gutted := strings.Replace(landedParkedFinding, "parked-until: \"2026-08-01\"\nparked-by: human:bot",
		"parked-until: \"2026-11-01\"\nparked-by: human:alex", 1)
	if err := os.WriteFile(path, []byte(gutted), 0o644); err != nil {
		t.Fatal(err)
	}
	p := guttedRegisterFields(root)
	if !containsSubstr(p, "beyond the 90-day horizon") || !containsSubstr(p, "latest allowed: 2026-10-18") {
		t.Fatalf("an extension crossing 90 days must fire; got %v", p)
	}
}

// The horizon is measured from the date of the COMMIT that sets the park, not
// from today: a park committed on 2026-05-01 may run to 2026-07-30 at most, even
// though today (2026-07-20) + 90 days would allow it.
func TestParkHorizonFromCommit(t *testing.T) {
	pinParkNow(t, "2026-07-20")
	root, path := gutFixture(t, landedOpenFinding)
	gutted := strings.Replace(landedOpenFinding, "resolved: false",
		"resolved: false\nparked-until: \"2026-08-15\"\nparked-by: human:alex\nparked-reason: waiting", 1)
	if err := os.WriteFile(path, []byte(gutted), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, root, "add", "-A")
	runGitEnv(t, root, []string{"GIT_COMMITTER_DATE=2026-05-01T12:00:00Z"},
		"commit", "-q", "-m", "park it")
	p := guttedRegisterFields(root)
	if !containsSubstr(p, "after 2026-05-01") || !containsSubstr(p, "latest allowed: 2026-07-30") {
		t.Fatalf("the horizon must run from the setting commit's date; got %v", p)
	}
}

// A commit date in the FUTURE counts as today: a committer cannot stretch the
// horizon by post-dating the commit that sets the park.
func TestParkHorizonFutureCommit(t *testing.T) {
	pinParkNow(t, "2026-07-20")
	root, path := gutFixture(t, landedOpenFinding)
	gutted := strings.Replace(landedOpenFinding, "resolved: false",
		"resolved: false\nparked-until: \"2027-02-01\"\nparked-by: human:alex\nparked-reason: waiting", 1)
	if err := os.WriteFile(path, []byte(gutted), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, root, "add", "-A")
	runGitEnv(t, root, []string{"GIT_COMMITTER_DATE=2027-01-01T12:00:00Z"},
		"commit", "-q", "-m", "park it")
	p := guttedRegisterFields(root)
	if !containsSubstr(p, "latest allowed: 2026-10-18") {
		t.Fatalf("a post-dated commit must count as today; got %v", p)
	}
}

// A landed park that is NOT moved is not re-judged against the horizon, and a
// narrowed one only shortens the snooze.
func TestParkHorizonUnmovedQuiet(t *testing.T) {
	pinParkNow(t, "2030-01-01") // landed park long past any horizon from here
	root, _ := gutFixture(t, landedParkedFinding)
	if p := guttedRegisterFields(root); containsSubstr(p, "horizon") {
		t.Fatalf("an unmoved landed park must not be re-judged; got %v", p)
	}
}

// NARROWING a park to an EARLIER date re-exposes the alarm sooner — not an attack,
// not guarded.
func TestParkNarrowedNotGuarded(t *testing.T) {
	root, path := gutFixture(t, landedParkedFinding) // base parked-until 2026-08-01
	narrowed := strings.Replace(landedParkedFinding, "parked-until: \"2026-08-01\"",
		"parked-until: \"2026-07-20\"", 1)
	if err := os.WriteFile(path, []byte(narrowed), 0o644); err != nil {
		t.Fatal(err)
	}
	if p := guttedRegisterFields(root); containsSubstr(p, "parked-until") {
		t.Fatalf("narrowing a park (earlier date) must NOT fire; got %v", p)
	}
}

// classifyPark's expiry test precedes well-formedness: an EXPIRED park always
// re-annunciates even if a field is missing (an out-of-window park must never
// stay quiet).
func TestExpiredParkClassifiedEvenIfMalformed(t *testing.T) {
	now := mustTime(t, "2026-07-30")
	f := parkedFinding("F-x", "2026-06-01", "t", "2026-07-01", "", "") // expired, missing by+reason
	if got := classifyPark(f, now); got != parkExpired {
		t.Fatalf("an expired park must classify as parkExpired regardless of completeness; got %v", got)
	}
	// And a future well-formed park is parkActive; a future malformed one is not.
	future := parkedFinding("F-y", "2026-06-01", "t", "2026-09-01", "human:alex", "why")
	if got := classifyPark(future, now); got != parkActive {
		t.Fatalf("a live well-formed park must classify parkActive; got %v", got)
	}
	if got := classifyPark(parkedFinding("F-z", "2026-06-01", "t", "2026-09-01", "human:alex", ""), now); got != parkMalformed {
		t.Fatalf("a live park missing a field must classify parkMalformed; got %v", got)
	}
}

// The ONLINE half ties in automatically: a `parked-by: human:<name>` line ADDED
// in a PR diff is scanned as a human stamp, so `statusgen --corroborate` requires
// that human to have acted on the PR — the same corroboration path the offline
// guard's parked-by anchor depends on. This pins that the park's authority line is
// visible to the online scanner (an agent cannot self-park past CI).
func TestParkedByStampIsCorroboratable(t *testing.T) {
	diff := `diff --git a/docs/streams/findings/2026-07-17-x.md b/docs/streams/findings/2026-07-17-x.md
index abc..def 100644
--- a/docs/streams/findings/2026-07-17-x.md
+++ b/docs/streams/findings/2026-07-17-x.md
@@ -3,6 +3,3 @@
+parked-until: "2099-01-01"
+parked-by: human:alex
+parked-reason: deferred
`
	stamps := stampsInDiff("", diff)
	if len(stamps) != 1 || stamps[0].Name != "alex" {
		t.Fatalf("a parked-by: human:<name> line must be scanned as a corroboratable stamp; got %+v", stamps)
	}
	if stamps[0].File != "docs/streams/findings/2026-07-17-x.md" {
		t.Errorf("stamp file = %q, want the findings entry path", stamps[0].File)
	}
}
