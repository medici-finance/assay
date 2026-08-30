package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Cadenced roadmap artifacts (statusgen/13).
//
// The `--roadmap` renderer answers "what is the portfolio's state right now."
// `--cadence weekly|monthly` re-renders the SAME roadmap skeleton over a CLOSED
// time window — the prior complete ISO week or calendar month — so a weekly or
// monthly reviewer gets a point-in-time deck scoped to the window that just
// closed. This file holds the pure window computation (no I/O, unit-testable),
// the adopter-config readers for the priority order and brand (the tool bakes in
// NEITHER — an unconfigured repo degrades to declaration order and a neutral
// palette), and the cadence view the roadmap renderer consumes. The renderer and
// its one health-rule table are reused verbatim; this file adds no second
// renderer and no new health rules.

// cadenceWindow returns the closed reporting window [start, end) and its label
// for a cadence, computed purely from `now`. A pure function so the boundary
// math (ISO-week and calendar-month edges, incl. year boundaries and leap
// February) is unit-testable with no I/O.
//
//   - weekly:  the prior complete ISO week — Monday 00:00 UTC → the following
//     Monday 00:00 UTC — labelled "%G-W%V" (ISO year + zero-padded ISO week).
//   - monthly: the prior complete calendar month — first-of-month 00:00 UTC →
//     first of the following month — labelled "%Y-%m".
func cadenceWindow(now time.Time, cadence string) (start, end time.Time, label string, err error) {
	now = now.UTC()
	switch cadence {
	case "weekly":
		// Days since Monday: Go's Weekday has Sunday=0..Saturday=6; ISO weeks
		// start on Monday, so Monday→0 … Sunday→6.
		daysSinceMon := (int(now.Weekday()) + 6) % 7
		curMon := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC).AddDate(0, 0, -daysSinceMon)
		start = curMon.AddDate(0, 0, -7) // the PRIOR week's Monday
		end = curMon
		// ISOWeek carries the ISO year, so a week straddling the Gregorian
		// year boundary (e.g. 2026-W01 beginning in December) labels correctly.
		y, w := start.ISOWeek()
		label = fmt.Sprintf("%d-W%02d", y, w)
		return start, end, label, nil
	case "monthly":
		firstThis := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
		start = firstThis.AddDate(0, -1, 0) // first of the PRIOR month (rolls the year in December)
		end = firstThis
		label = start.Format("2006-01")
		return start, end, label, nil
	default:
		return time.Time{}, time.Time{}, "", fmt.Errorf("unknown cadence %q (want weekly|monthly)", cadence)
	}
}

// countWindowChurn counts briefs that BOTH opened (a from:"" transition) and
// closed (a transition to done/verified) within the window [start, end). It is a
// within-window churn signal — created and completed inside the same reporting
// window — and is exactly why cadence accounting differs from point-in-time: a
// brief that opened and closed inside the window would be invisible to a
// still-open-at-end count. The window is half-open: start inclusive, end
// exclusive.
func countWindowChurn(history []HistoryEntry, start, end time.Time) int {
	opened := map[string]bool{}
	closed := map[string]bool{}
	for _, e := range history {
		ts, err := time.Parse(time.RFC3339, e.Ts)
		if err != nil {
			continue
		}
		if ts.Before(start) || !ts.Before(end) { // [start, end)
			continue
		}
		if e.From == "" {
			opened[e.Brief] = true
		}
		if e.To == "done" || e.To == "verified" {
			closed[e.Brief] = true
		}
	}
	n := 0
	for b := range opened {
		if closed[b] {
			n++
		}
	}
	return n
}

// loadPriorityOrder reads the adopter-configured product priority order — an
// ordered list of serves: tags, one per line (blank lines and #-comments
// ignored) — from docs/brand/priority-order. The tool ships NO baked-in order,
// so an absent or unreadable file returns nil and the effort-mix section ranks
// in declaration (first-encountered) order. The tags use the same serves:
// vocabulary `--scope` validates; no product name is compiled into statusgen.
func loadPriorityOrder(root string) []string {
	raw, err := os.ReadFile(filepath.Join(root, "docs", "brand", "priority-order"))
	if err != nil {
		return nil
	}
	var order []string
	for _, ln := range strings.Split(string(raw), "\n") {
		ln = strings.TrimSpace(ln)
		if ln == "" || strings.HasPrefix(ln, "#") {
			continue
		}
		order = append(order, ln)
	}
	return order
}

// brandConfig is the deck's visual surface, read from the running repo's brand
// config. statusgen hard-codes no brand: an absent or malformed config degrades
// to neutralBrand(), which reproduces the existing point-in-time deck palette so
// an unconfigured repo renders unchanged.
type brandConfig struct {
	Wordmark string `json:"wordmark"`
	Bg       string `json:"bg"`
	Surface  string `json:"surface"`
	Accent   string `json:"accent"`
}

// neutralBrand is the built-in fallback — no product identity, matching the
// existing roadmap deck palette.
func neutralBrand() brandConfig {
	return brandConfig{Wordmark: "", Bg: "#1a1a2e", Surface: "#16213e", Accent: "#3366FF"}
}

// loadBrandConfig reads docs/brand/brand.json. Absent or malformed → the neutral
// fallback, never a panic and never a partial half-styled deck. Present fields
// override the corresponding neutral default; empty fields keep it.
func loadBrandConfig(root string) brandConfig {
	b := neutralBrand()
	raw, err := os.ReadFile(filepath.Join(root, "docs", "brand", "brand.json"))
	if err != nil {
		return b
	}
	var got brandConfig
	if err := json.Unmarshal(raw, &got); err != nil {
		return b // malformed config → neutral fallback, no panic
	}
	if got.Wordmark != "" {
		b.Wordmark = got.Wordmark
	}
	if got.Bg != "" {
		b.Bg = got.Bg
	}
	if got.Surface != "" {
		b.Surface = got.Surface
	}
	if got.Accent != "" {
		b.Accent = got.Accent
	}
	return b
}

// mappedThemes are the render-style selectors a stream README's optional
// `theme:` key may name. The set is methodology-neutral (no product identity),
// the same kind of fixed vocabulary as the deck's stage colors. A `theme:` value
// outside this set is NOT dropped — unmappedThemes surfaces it as a visible
// marker.
var mappedThemes = map[string]bool{
	"default":   true,
	"highlight": true,
	"muted":     true,
}

// unmappedThemes returns, for every stream carrying a `theme:` value with no
// mapped render style, a "<stream>: unmapped theme: <value>" marker line. An
// unmapped theme is itself the signal — the same untagged-renders-visibly
// philosophy as the roadmap's serves handling — rather than being silently
// dropped. Absent theme, or a mapped one, contributes no marker.
func unmappedThemes(streams []*Stream) []string {
	var out []string
	for _, s := range streams {
		t := strings.TrimSpace(s.Theme)
		if t == "" || mappedThemes[t] {
			continue
		}
		out = append(out, fmt.Sprintf("%s: unmapped theme: %s", s.Name, t))
	}
	sort.Strings(out)
	return out
}

// effortMixEntry is one per-serves in-progress-work row of the monthly's
// effort-mix section, tagged with the tier its serves position resolves to.
type effortMixEntry struct {
	Serves string
	Label  string
	Active int
	Tier   string // "revenue" | "supporting" | "" (no order configured)
}

// computeEffortMix aggregates in-progress work per serves: tag and ranks it by
// the configured priority order. When order is nil (unconfigured) the entries
// come back in declaration (first-encountered) order with an empty Tier — the
// tool ranks nothing on its own. When an order is configured, revenue vs
// supporting tiers are derived from POSITION: the top half (ceil) of the ordered
// tags is the revenue tier, the remainder is supporting; a tag absent from the
// order sorts last and counts as supporting.
func computeEffortMix(streams []*Stream, order []string) []effortMixEntry {
	active := map[string]int{}
	var seen []string
	for _, s := range streams {
		g := s.Serves
		if _, ok := active[g]; !ok {
			active[g] = 0 // seed at first encounter so a serves tag with zero in-progress work is still registered exactly once
			seen = append(seen, g)
		}
		for _, b := range s.Briefs {
			if b.Status == "in-progress" {
				active[g]++
			}
		}
	}

	split := (len(order) + 1) / 2 // ceil(n/2)
	tierOf := func(serves string) string {
		if len(order) == 0 {
			return ""
		}
		idx := -1
		for i, t := range order {
			if t == serves {
				idx = i
				break
			}
		}
		if idx < 0 || idx >= split {
			return "supporting"
		}
		return "revenue"
	}

	var keys []string
	if len(order) > 0 {
		inOrder := map[string]bool{}
		for _, t := range order {
			if _, ok := active[t]; ok {
				keys = append(keys, t)
				inOrder[t] = true
			}
		}
		for _, g := range seen { // unranked serves keep declaration order, after ranked ones
			if !inOrder[g] {
				keys = append(keys, g)
			}
		}
	} else {
		keys = seen
	}

	out := make([]effortMixEntry, 0, len(keys))
	for _, g := range keys {
		out = append(out, effortMixEntry{Serves: g, Label: servesLabel(g), Active: active[g], Tier: tierOf(g)})
	}
	return out
}

// revenueVsSupporting returns the monthly's generic callout when supporting-tier
// active work outweighed revenue-tier active work for the window. It is a
// generic reporting concept — the tiers are purely the ordered positions the
// adopter configured, no project identity attached. Empty string when no order
// is configured (tiers cannot be derived) or revenue >= supporting.
func revenueVsSupporting(mix []effortMixEntry) string {
	rev, sup := 0, 0
	haveTiers := false
	for _, e := range mix {
		switch e.Tier {
		case "revenue":
			rev += e.Active
			haveTiers = true
		case "supporting":
			sup += e.Active
			haveTiers = true
		}
	}
	if !haveTiers || sup <= rev {
		return ""
	}
	return fmt.Sprintf("supporting-tier work (%d in-progress) outweighed revenue-tier work (%d) this window", sup, rev)
}

// cadenceView is everything the roadmap renderer needs to overlay a cadence
// artifact on the reused skeleton. nil when rendering the point-in-time deck.
type cadenceView struct {
	Cadence   string
	Label     string
	Start     time.Time
	End       time.Time
	Brand     brandConfig
	EffortMix []effortMixEntry // monthly only; nil for weekly
	RevSup    string           // monthly only
	Themes    []string         // unmapped-theme markers
	Churn     int              // briefs opened AND closed within the window
}

// activeCadence is set for the duration of a cadence render so renderRoadmap can
// overlay the window sections without a signature change to the point-in-time
// callers (the same saved/restored package-var pattern as activeDriveSet /
// roadmapBriefTouch). nil = point-in-time deck, byte-identical to before.
var activeCadence *cadenceView

// buildCadenceView computes the cadence overlay for a run, anchored on the given
// wall-clock time. Returns the view and the closed window [start, end); the
// window doubles as the accounting scope the caller threads into the renderer.
func buildCadenceView(root, cadence string, streams []*Stream, history []HistoryEntry, anchor time.Time) (*cadenceView, time.Time, time.Time, error) {
	start, end, label, err := cadenceWindow(anchor, cadence)
	if err != nil {
		return nil, time.Time{}, time.Time{}, err
	}
	cv := &cadenceView{
		Cadence: cadence,
		Label:   label,
		Start:   start,
		End:     end,
		Brand:   loadBrandConfig(root),
		Themes:  unmappedThemes(streams),
		Churn:   countWindowChurn(history, start, end),
	}
	if cadence == "monthly" {
		cv.EffortMix = computeEffortMix(streams, loadPriorityOrder(root))
		cv.RevSup = revenueVsSupporting(cv.EffortMix)
	}
	return cv, start, end, nil
}
