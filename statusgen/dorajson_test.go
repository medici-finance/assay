package main

import (
	"encoding/json"
	"regexp"
	"strings"
	"testing"
	"time"
)

// mustTime is shared with alarms_test.go (handles date-only + RFC3339).

func djWindow(t *testing.T) (since, until, now time.Time) {
	t.Helper()
	return mustTime(t, "2026-08-01T00:00:00Z"),
		mustTime(t, "2026-09-01T00:00:00Z"),
		mustTime(t, "2026-09-01T12:00:00Z")
}

func leadRec(mergedAt string, secs int64) doraTimingRecord {
	return doraTimingRecord{Type: "pr_lead_time", MergedAt: mergedAt, LeadSeconds: secs}
}

func restoreRec(restoredAt string, secs int64) doraTimingRecord {
	return doraTimingRecord{Type: "restore_episode", RestoredAt: restoredAt, RestoreSeconds: secs}
}

// doneEntry / revertEntry build the historian rows the instability proxy reads.
func doneEntry(brief, ts string) HistoryEntry {
	return HistoryEntry{Ts: ts, Brief: brief, From: "verified", To: "done"}
}

func revertEntry(brief, ts string) HistoryEntry {
	return HistoryEntry{Ts: ts, Brief: brief, From: "implemented", To: "in-progress"}
}

// --- the load-bearing honesty invariant ------------------------------------

// An absent/empty timing substrate must render could-not-check with a NULL value.
// This is the whole point of the emitter's three-state: a fabricated 0 here would
// publish "changes ship instantly, main never breaks" as a measured fact.
func TestDoraJSONBlankSubstrateIsCouldNotCheckNeverZero(t *testing.T) {
	since, until, now := djWindow(t)
	feed := computeDoraJSONFeed(nil, nil, nil, since, until, now, defaultDoraJSONLimit)

	for _, key := range []string{doraLeadTime, doraTimeToRestore} {
		m, ok := feed.Metrics[key]
		if !ok {
			t.Fatalf("%s missing from the feed", key)
		}
		if m.State != "could-not-check" {
			t.Errorf("%s state = %q, want could-not-check", key, m.State)
		}
		if m.Computed {
			t.Errorf("%s computed = true with no records", key)
		}
		if m.Value != nil {
			t.Errorf("%s value = %v, want null — a fabricated number for an unrecorded metric", key, *m.Value)
		}
		if len(m.Needs) == 0 {
			t.Errorf("%s carries no needs; a blank metric must name what is missing", key)
		}
	}

	// And it must survive serialization as a literal null, not a 0 or an omission:
	// the published renderer reads the JSON, not the Go value.
	b, err := json.Marshal(feed.Metrics[doraLeadTime])
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(b), `"value":null`) {
		t.Errorf("serialized blank metric = %s, want an explicit \"value\":null", b)
	}
}

// --- the source-collision guard --------------------------------------------

// deployment_frequency is sourced by the external platform. If this emitter ever
// starts emitting it, two sources write the same key and the frozen shape cannot
// say which won.
func TestDoraJSONNeverEmitsDeploymentFrequency(t *testing.T) {
	since, until, now := djWindow(t)
	feed := computeDoraJSONFeed(
		[]doraTimingRecord{leadRec("2026-08-10T00:00:00Z", 3600)},
		[]HistoryEntry{doneEntry("s/01", "2026-08-05T00:00:00Z")},
		nil, since, until, now, defaultDoraJSONLimit)

	if _, found := feed.Metrics[doraDeployFreq]; found {
		t.Fatalf("feed emits %q — that metric belongs to the external source; two emitters writing one key is a collision", doraDeployFreq)
	}
	want := map[string]bool{doraLeadTime: true, doraTimeToRestore: true, doraChangeFail: true}
	for k := range feed.Metrics {
		if !want[k] {
			t.Errorf("feed carries unexpected metric key %q", k)
		}
	}
	if len(feed.Metrics) != len(want) {
		t.Errorf("feed carries %d metrics, want exactly %d", len(feed.Metrics), len(want))
	}
}

// --- recorded substrate present: the metric actually computes ---------------

func TestDoraJSONTimingMeasuredWhenRecordsExist(t *testing.T) {
	since, until, now := djWindow(t)
	recs := []doraTimingRecord{
		leadRec("2026-08-10T00:00:00Z", 3600),  // 1h
		leadRec("2026-08-11T00:00:00Z", 7200),  // 2h
		leadRec("2026-08-12T00:00:00Z", 10800), // 3h
		restoreRec("2026-08-13T00:00:00Z", 1800),
		// out of window — must not be counted
		leadRec("2026-07-01T00:00:00Z", 999999),
	}
	feed := computeDoraJSONFeed(recs, nil, nil, since, until, now, defaultDoraJSONLimit)

	lt := feed.Metrics[doraLeadTime]
	if lt.State != "measured" || !lt.Computed || lt.Value == nil {
		t.Fatalf("change_lead_time = %+v, want a measured value", lt)
	}
	if got := lt.Sample["n"]; got != 3 {
		t.Errorf("change_lead_time n = %v, want 3 (the out-of-window record must be excluded)", got)
	}
	if got := *lt.Value; got != 2.0 {
		t.Errorf("change_lead_time p50 = %v hours, want 2.0", got)
	}

	ttr := feed.Metrics[doraTimeToRestore]
	if ttr.State != "measured" || ttr.Value == nil || *ttr.Value != 0.5 {
		t.Fatalf("time_to_restore = %+v, want a measured 0.5h", ttr)
	}
}

// --- the cap is real, and it says so ---------------------------------------

// A capped aggregate must (a) keep the MOST RECENT records and (b) declare itself
// partial. A cap that silently truncated to a clean "measured" would publish an
// undercount as a complete measurement — the #174/#182 defect class.
func TestDoraJSONCapKeepsRecentAndMarksPartial(t *testing.T) {
	since, until, now := djWindow(t)
	recs := []doraTimingRecord{
		leadRec("2026-08-01T00:00:00Z", 36000), // oldest, 10h — must be dropped
		leadRec("2026-08-20T00:00:00Z", 3600),  // 1h
		leadRec("2026-08-21T00:00:00Z", 3600),  // 1h
	}
	feed := computeDoraJSONFeed(recs, nil, nil, since, until, now, 2)

	lt := feed.Metrics[doraLeadTime]
	if lt.State != "partial" {
		t.Errorf("capped change_lead_time state = %q, want partial — a capped number is not a complete measurement", lt.State)
	}
	if !lt.Computed {
		t.Errorf("capped change_lead_time should still be computed")
	}
	if got := lt.Sample["n"]; got != 2 {
		t.Errorf("capped n = %v, want 2", got)
	}
	if lt.Value == nil || *lt.Value != 1.0 {
		t.Errorf("capped p50 = %v, want 1.0 — the cap must drop the OLDEST record, not the newest", lt.Value)
	}
	if len(lt.Needs) == 0 {
		t.Errorf("a capped metric must name the cap in needs")
	}
}

// The declared caps must cover every metric the feed emits, or a published number
// carries no statement of what bounded it.
func TestDoraJSONCapsCoverEveryMetric(t *testing.T) {
	since, until, now := djWindow(t)
	feed := computeDoraJSONFeed(nil, nil, nil, since, until, now, defaultDoraJSONLimit)

	covered := map[string]bool{}
	for _, c := range feed.Caps {
		for _, k := range c.AppliesTo {
			covered[k] = true
		}
	}
	for k := range feed.Metrics {
		if !covered[k] {
			t.Errorf("metric %q is emitted but no declared cap covers it", k)
		}
	}
}

// --- change_failure_rate: the aggregation correctness core ------------------

// A finding affecting THREE streams is ONE failure signal. findingsPerGroup
// increments once per Affects entry (correct per-group, wrong in an aggregate), so
// the aggregate must not be built by summing that helper's groups.
func TestChangeFailureRateCountsAMultiStreamFindingOnce(t *testing.T) {
	since, until, now := djWindow(t)
	history := []HistoryEntry{
		doneEntry("a/01", "2026-08-05T00:00:00Z"),
		doneEntry("b/01", "2026-08-06T00:00:00Z"),
		doneEntry("c/01", "2026-08-07T00:00:00Z"),
		doneEntry("d/01", "2026-08-08T00:00:00Z"),
	}
	findings := []Finding{
		{ID: "F-01", Affects: []string{"a", "b", "c"}, Resolved: false},
	}
	feed := computeDoraJSONFeed(nil, history, findings, since, until, now, defaultDoraJSONLimit)

	cf := feed.Metrics[doraChangeFail]
	if got := cf.Sample["open_defect_notes"]; got != 1 {
		t.Fatalf("open_defect_notes = %v, want 1 — one finding is one signal however many streams it affects", got)
	}
	if cf.Value == nil || *cf.Value != 0.25 {
		t.Fatalf("change_failure_rate = %v, want 0.25 (1 finding / 4 completed)", cf.Value)
	}
}

func TestChangeFailureRateExcludesResolvedFindings(t *testing.T) {
	since, until, now := djWindow(t)
	history := []HistoryEntry{doneEntry("a/01", "2026-08-05T00:00:00Z")}
	findings := []Finding{
		{ID: "F-01", Affects: []string{"a"}, Resolved: true},
		{ID: "F-02", Affects: []string{"a"}, Resolved: false},
	}
	feed := computeDoraJSONFeed(nil, history, findings, since, until, now, defaultDoraJSONLimit)
	if got := feed.Metrics[doraChangeFail].Sample["open_defect_notes"]; got != 1 {
		t.Errorf("open_defect_notes = %v, want 1 — a resolved finding is not an open defect", got)
	}
}

func TestChangeFailureRateCountsReversions(t *testing.T) {
	since, until, now := djWindow(t)
	history := []HistoryEntry{
		doneEntry("a/01", "2026-08-05T00:00:00Z"),
		doneEntry("b/01", "2026-08-06T00:00:00Z"),
		revertEntry("c/01", "2026-08-07T00:00:00Z"),
	}
	feed := computeDoraJSONFeed(nil, history, nil, since, until, now, defaultDoraJSONLimit)
	cf := feed.Metrics[doraChangeFail]
	if got := cf.Sample["reversions"]; got != 1 {
		t.Errorf("reversions = %v, want 1", got)
	}
	if cf.Value == nil || *cf.Value != 0.5 {
		t.Errorf("change_failure_rate = %v, want 0.5 (1 reversion / 2 completed)", cf.Value)
	}
}

// No completed work in the window = no denominator. That is could-not-check, NOT
// 0% — "nothing was completed" must never publish as "nothing ever failed".
func TestChangeFailureRateWithNoDenominatorIsCouldNotCheck(t *testing.T) {
	since, until, now := djWindow(t)
	findings := []Finding{{ID: "F-01", Affects: []string{"a"}, Resolved: false}}
	feed := computeDoraJSONFeed(nil, nil, findings, since, until, now, defaultDoraJSONLimit)

	cf := feed.Metrics[doraChangeFail]
	if cf.State != "could-not-check" {
		t.Errorf("state = %q, want could-not-check with no completed work", cf.State)
	}
	if cf.Value != nil {
		t.Errorf("value = %v, want null — an empty window is unread, not a 0%% failure rate", *cf.Value)
	}
}

// The proxy is never `measured`: it lacks the change-to-incident linkage the
// canonical metric is defined over, and a proxy published as measured is exactly
// the over-claim the three-state exists to prevent.
func TestChangeFailureRateIsNeverMeasured(t *testing.T) {
	since, until, now := djWindow(t)
	history := []HistoryEntry{doneEntry("a/01", "2026-08-05T00:00:00Z")}
	feed := computeDoraJSONFeed(nil, history, nil, since, until, now, defaultDoraJSONLimit)

	cf := feed.Metrics[doraChangeFail]
	if cf.State != "partial" {
		t.Errorf("state = %q, want partial — an unlinked proxy is never a measured DORA number", cf.State)
	}
	if len(cf.Needs) == 0 {
		t.Errorf("a partial metric must state what it needs to become measured")
	}
}

// --- the state/computed/needs triple cannot drift ---------------------------

// The consumer derives its own tally from computed+needs while a renderer reads
// `state`. If the two disagree, the page and the count tell different stories.
func TestDoraJSONStateAlwaysAgreesWithComputedAndNeeds(t *testing.T) {
	since, until, now := djWindow(t)
	cases := [][]doraTimingRecord{
		nil,
		{leadRec("2026-08-10T00:00:00Z", 3600)},
		{leadRec("2026-08-10T00:00:00Z", 3600), leadRec("2026-08-11T00:00:00Z", 7200)},
	}
	history := []HistoryEntry{doneEntry("a/01", "2026-08-05T00:00:00Z")}
	for i, recs := range cases {
		feed := computeDoraJSONFeed(recs, history, nil, since, until, now, 1)
		for key, m := range feed.Metrics {
			var want string
			switch {
			case !m.Computed:
				want = "could-not-check"
			case len(m.Needs) > 0:
				want = "partial"
			default:
				want = "measured"
			}
			if m.State != want {
				t.Errorf("case %d metric %s: state=%q but computed=%v needs=%d imply %q",
					i, key, m.State, m.Computed, len(m.Needs), want)
			}
			if !m.Computed && m.Value != nil {
				t.Errorf("case %d metric %s: uncomputed but carries a value", i, key)
			}
			if m.Computed && m.Value == nil {
				t.Errorf("case %d metric %s: computed but carries no value", i, key)
			}
		}
	}
}

// The block-level state must never read could-not-check while a metric under it
// carries a real number — that would blank a published number that exists.
func TestDoraJSONOverallStateNotBlankWhenAMetricComputed(t *testing.T) {
	since, until, now := djWindow(t)
	history := []HistoryEntry{doneEntry("a/01", "2026-08-05T00:00:00Z")}
	feed := computeDoraJSONFeed(nil, history, nil, since, until, now, defaultDoraJSONLimit)
	if feed.State == "could-not-check" {
		t.Fatalf("block state = could-not-check while change_failure_rate computed %v", feed.Metrics[doraChangeFail].Value)
	}
	if feed.State != "partial" {
		t.Errorf("block state = %q, want partial", feed.State)
	}
}

// --- byte stability ---------------------------------------------------------

// The feed is committed/published downstream; unstable ordering would churn it on
// every run and bury a real change in noise.
func TestDoraJSONIsByteStable(t *testing.T) {
	since, until, now := djWindow(t)
	recs := []doraTimingRecord{
		leadRec("2026-08-10T00:00:00Z", 3600),
		restoreRec("2026-08-11T00:00:00Z", 1800),
	}
	history := []HistoryEntry{doneEntry("a/01", "2026-08-05T00:00:00Z")}
	findings := []Finding{{ID: "F-01", Affects: []string{"a"}}}

	var first string
	for i := 0; i < 5; i++ {
		b, err := json.MarshalIndent(
			computeDoraJSONFeed(recs, history, findings, since, until, now, defaultDoraJSONLimit),
			"", "  ")
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		if i == 0 {
			first = string(b)
			continue
		}
		if string(b) != first {
			t.Fatalf("feed is not byte-stable across runs")
		}
	}
	// Metric keys must serialize in sorted order (the encoder's map ordering is
	// what makes the feed diffable). Scope the search to the `metrics` object —
	// the same three names also appear in the `caps` array above it, in the
	// caps' own declaration order.
	mStart := strings.Index(first, `"metrics"`)
	if mStart < 0 {
		t.Fatalf("no metrics object in the feed")
	}
	metricsText := first[mStart:]
	iFail := strings.Index(metricsText, `"change_failure_rate"`)
	iLead := strings.Index(metricsText, `"change_lead_time"`)
	iRest := strings.Index(metricsText, `"time_to_restore"`)
	if !(iFail >= 0 && iFail < iLead && iLead < iRest) {
		t.Errorf("metric keys are not in sorted order (%d,%d,%d)", iFail, iLead, iRest)
	}
}

// --- public-safety of the strings the feed publishes ------------------------

// Every string in this feed is serialized into a public artifact. A leaked issue
// ref, slug, or on-disk path is a disclosure, and the leak gate is the backstop
// rather than the design.
func TestDoraJSONStringsCarryNoInternalIdentifiers(t *testing.T) {
	since, until, now := djWindow(t)
	recs := []doraTimingRecord{
		leadRec("2026-08-10T00:00:00Z", 3600), leadRec("2026-08-11T00:00:00Z", 7200),
	}
	history := []HistoryEntry{doneEntry("a/01", "2026-08-05T00:00:00Z")}
	// limit 1 so the capped-partial needs string is exercised too.
	b, err := json.Marshal(computeDoraJSONFeed(recs, history, nil, since, until, now, 1))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	text := string(b)
	// Checked STRUCTURALLY, by the shape an internal identifier takes, rather than
	// against a list of the real ones: this file ships in a public tree, so naming
	// the private repositories and streams here would itself be the disclosure the
	// test exists to prevent (the leak gate flags exactly that).
	for _, bad := range []string{
		"docs/",  // an on-disk path into the source tree
		"/Users", // an absolute path off a developer machine
		".jsonl", // a substrate filename
		".go",    // a source filename
	} {
		if strings.Contains(text, bad) {
			t.Errorf("feed leaks an internal path-shaped identifier %q into a published artifact", bad)
		}
	}
	// Issue/PR cross-references (`#123`) resolve only inside the originating
	// project and are meaningless — or misleading — in a published artifact.
	if m := regexp.MustCompile(`#\d+`).FindString(text); m != "" {
		t.Errorf("feed leaks the issue reference %q into a published artifact", m)
	}
	// A slug of the `word-word/NN` shape is an internal work-item id.
	if m := regexp.MustCompile(`[a-z]+-[a-z]+/\d+`).FindString(text); m != "" {
		t.Errorf("feed leaks the work-item id %q into a published artifact", m)
	}
}
