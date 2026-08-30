package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// --- Window computation (statusgen/13) ---

func TestCadenceWindowWeekly(t *testing.T) {
	// A Wednesday mid-week: 2026-08-26 is a Wednesday. The prior complete ISO
	// week is Mon 2026-08-17 → Mon 2026-08-24, labelled 2026-W34.
	now := time.Date(2026, 8, 26, 13, 0, 0, 0, time.UTC)
	start, end, label, err := cadenceWindow(now, "weekly")
	if err != nil {
		t.Fatalf("weekly: unexpected error: %v", err)
	}
	wantStart := time.Date(2026, 8, 17, 0, 0, 0, 0, time.UTC)
	wantEnd := time.Date(2026, 8, 24, 0, 0, 0, 0, time.UTC)
	if !start.Equal(wantStart) || !end.Equal(wantEnd) {
		t.Fatalf("weekly window = [%s, %s), want [%s, %s)", start, end, wantStart, wantEnd)
	}
	if label != "2026-W34" {
		t.Errorf("weekly label = %q, want 2026-W34", label)
	}
	// The window must be exactly 7 days and end must be a Monday.
	if end.Sub(start) != 7*24*time.Hour {
		t.Errorf("weekly window length = %v, want 168h", end.Sub(start))
	}
	if start.Weekday() != time.Monday || end.Weekday() != time.Monday {
		t.Errorf("weekly window edges must be Mondays: start=%s end=%s", start.Weekday(), end.Weekday())
	}
}

func TestCadenceWindowWeeklyOnMonday(t *testing.T) {
	// On a Monday, the prior complete week is the immediately preceding Mon→Mon,
	// NOT the week that starts today (today's week is still open).
	now := time.Date(2026, 8, 24, 0, 0, 0, 0, time.UTC) // Monday
	start, end, label, err := cadenceWindow(now, "weekly")
	if err != nil {
		t.Fatalf("weekly monday: %v", err)
	}
	if !start.Equal(time.Date(2026, 8, 17, 0, 0, 0, 0, time.UTC)) ||
		!end.Equal(time.Date(2026, 8, 24, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("weekly on Monday window = [%s, %s), want [2026-08-17, 2026-08-24)", start, end)
	}
	if label != "2026-W34" {
		t.Errorf("label = %q, want 2026-W34", label)
	}
}

func TestCadenceWindowWeeklyYearBoundary(t *testing.T) {
	// Early January 2027: 2027-01-06 is a Wednesday. The prior complete ISO week
	// is Mon 2026-12-28 → Mon 2027-01-04, which ISO-labels as 2026-W53 (the week
	// begins in the prior Gregorian year and ISOWeek carries the ISO year).
	now := time.Date(2027, 1, 6, 9, 0, 0, 0, time.UTC)
	start, end, label, err := cadenceWindow(now, "weekly")
	if err != nil {
		t.Fatalf("weekly year-boundary: %v", err)
	}
	if !start.Equal(time.Date(2026, 12, 28, 0, 0, 0, 0, time.UTC)) ||
		!end.Equal(time.Date(2027, 1, 4, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("year-boundary window = [%s, %s), want [2026-12-28, 2027-01-04)", start, end)
	}
	gotY, gotW := start.ISOWeek()
	if label != "2026-W53" {
		t.Errorf("year-boundary label = %q, want 2026-W53 (ISOWeek reports %d-W%02d)", label, gotY, gotW)
	}
}

func TestCadenceWindowMonthly(t *testing.T) {
	// Mid-August → the prior complete calendar month is July.
	now := time.Date(2026, 8, 15, 12, 0, 0, 0, time.UTC)
	start, end, label, err := cadenceWindow(now, "monthly")
	if err != nil {
		t.Fatalf("monthly: %v", err)
	}
	if !start.Equal(time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)) ||
		!end.Equal(time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("monthly window = [%s, %s), want [2026-07-01, 2026-08-01)", start, end)
	}
	if label != "2026-07" {
		t.Errorf("monthly label = %q, want 2026-07", label)
	}
}

func TestCadenceWindowMonthlyDecemberToJanuary(t *testing.T) {
	// Mid-January → the prior complete month is the previous December; the year
	// rolls back.
	now := time.Date(2027, 1, 12, 0, 0, 0, 0, time.UTC)
	start, end, label, err := cadenceWindow(now, "monthly")
	if err != nil {
		t.Fatalf("monthly dec→jan: %v", err)
	}
	if !start.Equal(time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)) ||
		!end.Equal(time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("dec→jan window = [%s, %s), want [2026-12-01, 2027-01-01)", start, end)
	}
	if label != "2026-12" {
		t.Errorf("dec→jan label = %q, want 2026-12", label)
	}
}

func TestCadenceWindowMonthlyLeapFebruary(t *testing.T) {
	// 2028 is a leap year. In mid-March the prior complete month is February,
	// whose window is [Feb 1, Mar 1) — 29 days — and must anchor on month firsts
	// regardless of February's length.
	now := time.Date(2028, 3, 10, 0, 0, 0, 0, time.UTC)
	start, end, label, err := cadenceWindow(now, "monthly")
	if err != nil {
		t.Fatalf("monthly leap-feb: %v", err)
	}
	if !start.Equal(time.Date(2028, 2, 1, 0, 0, 0, 0, time.UTC)) ||
		!end.Equal(time.Date(2028, 3, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("leap-feb window = [%s, %s), want [2028-02-01, 2028-03-01)", start, end)
	}
	if end.Sub(start) != 29*24*time.Hour {
		t.Errorf("leap-feb window length = %v, want 29 days", end.Sub(start))
	}
	if label != "2028-02" {
		t.Errorf("leap-feb label = %q, want 2028-02", label)
	}
}

func TestCadenceWindowUnknownCadence(t *testing.T) {
	if _, _, _, err := cadenceWindow(time.Now(), "quarterly"); err == nil {
		t.Fatal("unknown cadence must error, got nil")
	}
	if _, _, _, err := cadenceWindow(time.Now(), ""); err == nil {
		t.Fatal("empty cadence must error at the window boundary, got nil")
	}
}

// --- Opened-AND-closed exception accounting (statusgen/13) ---

func TestCadenceWindowChurn(t *testing.T) {
	start := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	at := func(d int) string {
		return time.Date(2026, 7, d, 12, 0, 0, 0, time.UTC).Format(time.RFC3339)
	}
	history := []HistoryEntry{
		// churned: opened AND closed inside the window → counts.
		{Ts: at(3), Brief: "alpha/01", From: "", To: "todo"},
		{Ts: at(20), Brief: "alpha/01", From: "implemented", To: "done"},
		// opened in-window but never closed → does NOT count.
		{Ts: at(5), Brief: "beta/01", From: "", To: "todo"},
		{Ts: at(10), Brief: "beta/01", From: "todo", To: "in-progress"},
		// closed in-window but opened BEFORE the window → does NOT count as churn.
		{Ts: time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC).Format(time.RFC3339), Brief: "gamma/01", From: "", To: "todo"},
		{Ts: at(22), Brief: "gamma/01", From: "verified", To: "done"},
		// churned via a verified close → counts.
		{Ts: at(2), Brief: "delta/01", From: "", To: "todo"},
		{Ts: at(28), Brief: "delta/01", From: "implemented", To: "verified"},
	}
	got := countWindowChurn(history, start, end)
	if got != 2 {
		t.Fatalf("countWindowChurn = %d, want 2 (alpha/01 + delta/01)", got)
	}
}

func TestCadenceWindowChurnHalfOpen(t *testing.T) {
	start := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	// A brief opened exactly at start (inclusive) and closed exactly at end
	// (exclusive → NOT counted). The end-exact close falls outside [start, end).
	history := []HistoryEntry{
		{Ts: start.Format(time.RFC3339), Brief: "edge/01", From: "", To: "todo"},
		{Ts: end.Format(time.RFC3339), Brief: "edge/01", From: "implemented", To: "done"},
	}
	if got := countWindowChurn(history, start, end); got != 0 {
		t.Fatalf("half-open: close at end (exclusive) must not count; churn = %d, want 0", got)
	}
	// Move the close one second before end → now inside the window.
	history[1].Ts = end.Add(-time.Second).Format(time.RFC3339)
	if got := countWindowChurn(history, start, end); got != 1 {
		t.Fatalf("half-open: open at start (inclusive) + close before end must count; churn = %d, want 1", got)
	}
}

// --- Effort-mix tiers + config fallbacks (statusgen/13) ---

func cadStream(name, serves string, inProgress int) *Stream {
	s := mkStream(name, "active", "P1")
	s.Serves = serves
	for i := 0; i < inProgress; i++ {
		s.Briefs = append(s.Briefs, Brief{Num: "0", Status: "in-progress"})
	}
	return s
}

func TestCadenceEffortMixDeclarationOrderWhenUnconfigured(t *testing.T) {
	streams := []*Stream{
		cadStream("s1", "example-service", 1),
		cadStream("s2", "example-app", 2),
	}
	mix := computeEffortMix(streams, nil) // no configured order
	if len(mix) != 2 {
		t.Fatalf("mix len = %d, want 2", len(mix))
	}
	// Declaration (first-encountered) order: example-service, then example-app.
	if mix[0].Serves != "example-service" || mix[1].Serves != "example-app" {
		t.Fatalf("unconfigured order = [%s, %s], want declaration order [example-service, example-app]", mix[0].Serves, mix[1].Serves)
	}
	for _, e := range mix {
		if e.Tier != "" {
			t.Errorf("unconfigured tier for %s = %q, want empty (tool ranks nothing on its own)", e.Serves, e.Tier)
		}
	}
}

// Two (or more) streams sharing one serves: tag with ZERO in-progress work must
// appear in the mix exactly once — the "first-encountered" guard has to seed the
// aggregation key on encounter, not only when there is in-progress work, else the
// tag is appended once per stream and renders a duplicate goal-card. Covers both
// the unconfigured (keys = seen) and configured (unranked-after-ranked) branches.
func TestCadenceEffortMixDedupsSharedServesWithZeroInProgress(t *testing.T) {
	streams := []*Stream{
		cadStream("s1", "example-app", 0), // shared tag, no in-progress
		cadStream("s2", "example-app", 0), // shared tag, no in-progress
	}
	// Unconfigured: keys come from `seen`.
	mix := computeEffortMix(streams, nil)
	if len(mix) != 1 {
		t.Fatalf("unconfigured: mix len = %d, want 1 (shared serves must dedup); entries=%+v", len(mix), mix)
	}
	if mix[0].Serves != "example-app" || mix[0].Active != 0 {
		t.Errorf("unconfigured entry = {Serves:%q Active:%d}, want {example-app 0}", mix[0].Serves, mix[0].Active)
	}
	// Configured: unranked serves dedup against `seen` in the after-ranked loop.
	mixOrdered := computeEffortMix(streams, []string{"platform"}) // example-app is unranked
	if len(mixOrdered) != 1 {
		t.Fatalf("configured: mix len = %d, want 1 (shared unranked serves must dedup); entries=%+v", len(mixOrdered), mixOrdered)
	}
}

func TestCadenceEffortMixTiersFromConfiguredOrder(t *testing.T) {
	streams := []*Stream{
		cadStream("s1", "example-service", 3), // supporting
		cadStream("s2", "example-app", 1),     // revenue
		cadStream("s3", "platform", 5),        // supporting
	}
	order := []string{"example-app", "example-service", "platform"} // split = ceil(3/2)=2
	mix := computeEffortMix(streams, order)
	tierBy := map[string]string{}
	activeBy := map[string]int{}
	for _, e := range mix {
		tierBy[e.Serves] = e.Tier
		activeBy[e.Serves] = e.Active
	}
	if tierBy["example-app"] != "revenue" || tierBy["example-service"] != "revenue" {
		t.Errorf("top-half tiers: example-app=%s example-service=%s, want both revenue", tierBy["example-app"], tierBy["example-service"])
	}
	if tierBy["platform"] != "supporting" {
		t.Errorf("platform tier = %s, want supporting", tierBy["platform"])
	}
	// Ranked entries come first, in configured order.
	if mix[0].Serves != "example-app" {
		t.Errorf("first entry = %s, want example-app (highest configured priority)", mix[0].Serves)
	}
	// Supporting (8) > revenue (4) → callout fires.
	if got := revenueVsSupporting(mix); got == "" {
		t.Errorf("revenueVsSupporting should fire when supporting (8) > revenue (4); got empty")
	}
}

func TestCadenceRevenueVsSupportingNoCalloutWithoutOrder(t *testing.T) {
	streams := []*Stream{cadStream("s1", "example-app", 5)}
	mix := computeEffortMix(streams, nil) // no order → no tiers → no callout
	if got := revenueVsSupporting(mix); got != "" {
		t.Errorf("no configured order must yield no tier callout; got %q", got)
	}
}

func TestCadencePriorityOrderFallbackAbsent(t *testing.T) {
	root := t.TempDir() // no docs/brand/priority-order
	if got := loadPriorityOrder(root); got != nil {
		t.Errorf("absent priority-order config must return nil (declaration order); got %v", got)
	}
}

func TestCadencePriorityOrderReadFromConfig(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "docs", "brand"), 0o755); err != nil {
		t.Fatal(err)
	}
	body := "# priority order\nexample-app\n\nexample-service\nplatform\n"
	if err := os.WriteFile(filepath.Join(root, "docs", "brand", "priority-order"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	got := loadPriorityOrder(root)
	want := []string{"example-app", "example-service", "platform"}
	if len(got) != len(want) {
		t.Fatalf("priority order = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("priority order[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestCadenceBrandFallbackAbsent(t *testing.T) {
	root := t.TempDir() // no docs/brand/brand.json
	got := loadBrandConfig(root)
	if got != neutralBrand() {
		t.Errorf("absent brand config must degrade to neutralBrand(); got %+v", got)
	}
}

func TestCadenceBrandFallbackMalformedNoPanic(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "docs", "brand"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "docs", "brand", "brand.json"), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := loadBrandConfig(root) // must not panic
	if got != neutralBrand() {
		t.Errorf("malformed brand config must degrade to neutralBrand(); got %+v", got)
	}
}

func TestCadenceBrandReadFromConfig(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "docs", "brand"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "docs", "brand", "brand.json"),
		[]byte(`{"wordmark":"Example Deck","accent":"#123456"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	got := loadBrandConfig(root)
	if got.Wordmark != "Example Deck" {
		t.Errorf("wordmark = %q, want Example Deck", got.Wordmark)
	}
	if got.Accent != "#123456" {
		t.Errorf("accent = %q, want #123456", got.Accent)
	}
	// Unspecified fields keep the neutral defaults.
	if got.Bg != neutralBrand().Bg {
		t.Errorf("bg = %q, want neutral default %q", got.Bg, neutralBrand().Bg)
	}
}
