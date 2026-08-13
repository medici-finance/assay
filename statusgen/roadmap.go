package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// --- Health rules (ONE Go table — the page legend renders from this) ---

type roadmapHealthRule struct {
	Name   string
	Color  string // "red", "amber", "green"
	Legend string // human-readable description for the page legend
	// Fires returns (true, reason) when this rule matches.
	Fires func(s *Stream, rows []roadmapStreamRow, streams []*Stream, findings []Finding, now time.Time) (bool, string)
}

// briefTouch maps "stream/NN" → last transition time.
// A nil or empty map falls back to stream LastTouch.

var roadmapHealthRules = []roadmapHealthRule{
	{
		Name:   "P0-finding",
		Color:  "red",
		Legend: "P0 stream with an unresolved finding affecting it",
		Fires: func(s *Stream, rows []roadmapStreamRow, streams []*Stream, findings []Finding, now time.Time) (bool, string) {
			if s.Priority != "P0" {
				return false, ""
			}
			for _, f := range findings {
				if f.Resolved {
					continue
				}
				for _, a := range f.Affects {
					if strings.HasPrefix(a, s.Name) {
						return true, fmt.Sprintf("red: P0 stream, unresolved finding %s — %s", f.ID, f.Title)
					}
				}
			}
			return false, ""
		},
	},
	{
		Name:   "Stalled-critical",
		Color:  "red",
		Legend: "Brief on the critical path stalled in-stage > 7 days",
		Fires: func(s *Stream, rows []roadmapStreamRow, streams []*Stream, findings []Finding, now time.Time) (bool, string) {
			// Build a status index over all briefs.
			allStatus := map[string]string{}
			for _, st := range streams {
				for _, b := range st.Briefs {
					allStatus[st.Name+"/"+b.Num] = b.Status
				}
			}
			// Find the critical-path briefs: in-progress or todo whose typed
			// depends are ALL done/verified. Among those, the highest wave.
			type cand struct {
				brief  Brief
				stream *Stream
			}
			var critical []cand
			for _, st := range streams {
				for _, b := range st.Briefs {
					if b.Status != "in-progress" && b.Status != "todo" {
						continue
					}
					if b.Schema != "brief-v1" || len(b.Depends) == 0 {
						continue
					}
					allDone := true
					for _, dep := range b.Depends {
						if st, ok := allStatus[dep]; !ok || (st != "done" && st != "verified") {
							allDone = false
							break
						}
					}
					if allDone {
						critical = append(critical, cand{brief: b, stream: st})
					}
				}
			}
			if len(critical) == 0 {
				return false, ""
			}
			// Among critical, pick highest wave.
			maxWave := 0
			for _, c := range critical {
				if c.brief.Wave > maxWave {
					maxWave = c.brief.Wave
				}
			}
			// Check if THIS stream has a critical-path brief at maxWave
			// that is stalled > 7d.
			for _, c := range critical {
				if c.stream.Name != s.Name || c.brief.Wave != maxWave {
					continue
				}
				id := s.Name + "/" + c.brief.Num
				days := daysSinceLastTouch(s, c.brief, now)
				if days > 7 {
					return true, fmt.Sprintf("red: %s stalled %dd in %s on critical path", id, days, c.brief.Status)
				}
			}
			return false, ""
		},
	},
	{
		Name:   "Awaiting-backlog",
		Color:  "amber",
		Legend: ">= 2 briefs implemented-unverified > 3 days",
		Fires: func(s *Stream, rows []roadmapStreamRow, streams []*Stream, findings []Finding, now time.Time) (bool, string) {
			count := 0
			for _, b := range s.Briefs {
				if b.Status != "implemented" && b.Status != "verified" {
					continue
				}
				days := daysSinceLastTouch(s, b, now)
				if days > 3 {
					count++
				}
			}
			if count >= 2 {
				return true, fmt.Sprintf("amber: %d briefs awaiting verification >3d", count)
			}
			return false, ""
		},
	},
	{
		Name:   "Nextup-stale",
		Color:  "amber",
		Legend: "Next-up entry from this stream unpicked > 3 days",
		Fires: func(s *Stream, rows []roadmapStreamRow, streams []*Stream, findings []Finding, now time.Time) (bool, string) {
			// Check the stream's briefs: find the one that has been in
			// "todo" longest (if any), and flag if > 3d.
			var oldest int
			var oldestID string
			for _, b := range s.Briefs {
				if b.Status != "todo" {
					continue
				}
				days := daysSinceLastTouch(s, b, now)
				if days > oldest {
					oldest = days
					oldestID = s.Name + "/" + b.Num
				}
			}
			if oldest > 3 {
				return true, fmt.Sprintf("amber: Next-up entry unpicked %dd", oldest)
			}
			_ = oldestID
			return false, ""
		},
	},
	{
		Name:   "Green",
		Color:  "green",
		Legend: "No issues detected — green is the default fallback",
		Fires: func(s *Stream, rows []roadmapStreamRow, streams []*Stream, findings []Finding, now time.Time) (bool, string) {
			return true, ""
		},
	},
}

// daysSinceLastTouch returns the number of days since the brief's last
// recorded transition (from briefTouch if available, else stream LastTouch).
// This is a package-level function so tests can override the "touch" source
// indirectly via the fixtures they construct.
var roadmapBriefTouch func() map[string]time.Time

func daysSinceLastTouch(s *Stream, b Brief, now time.Time) int {
	ref := s.LastTouch
	if roadmapBriefTouch != nil {
		if bt := roadmapBriefTouch(); bt != nil {
			if t, ok := bt[s.Name+"/"+b.Num]; ok {
				ref = t
			}
		}
	}
	days := int(now.Sub(ref).Hours() / 24)
	if days < 0 {
		days = 0
	}
	return days
}

// --- Data structures ---

type roadmapStreamRow struct {
	Stream       *Stream
	HealthColor  string
	HealthReason string
	StageCounts  map[string]int // todo, in-progress, implemented, verified, done, blocked
	Deltas       []string       // delta badges e.g. "+2 done", "1→in-progress", "NEW"
	NextWave     int            // next incomplete wave (0 = all done)
	TopBlocker   string         // formatted blocker text
	// BlockerFindingLink is the finding entry filename (e.g.
	// "2026-07-17-ready-flip-is-convention-only.md") when TopBlocker references
	// a finding; "" otherwise. Used for hyperlinks in the roadmap deck.
	BlockerFindingLink string
}

type roadmapException struct {
	Stream string
	Text   string
	Color  string // "red" or "amber"
}

type goalMix struct {
	Goal      string // "example-app", "example-service", "assay", "platform", "untagged"
	Label     string // "Example App", "Example Service", "Assay", "Platform", "Untagged"
	Active    int
	NextUp    int
	Merged24h int
}

// --- Fixed ordering ---

var servesOrder = []string{"example-app", "example-service", "assay", "platform", ""}

func servesIndex(serves string) int {
	for i, v := range servesOrder {
		if v == serves {
			return i
		}
	}
	return len(servesOrder) - 1 // "" (untagged) is last
}

func servesLabel(serves string) string {
	switch serves {
	case "example-app":
		return "Example App"
	case "example-service":
		return "Example Service"
	case "assay":
		return "Assay"
	case "platform":
		return "Platform"
	default:
		return "Untagged"
	}
}

func servesColor(serves string) string {
	switch serves {
	case "example-app":
		return "#3366FF"
	case "example-service":
		return "#8B5CF6"
	case "assay":
		return "#10B981"
	case "platform":
		return "#6366F1"
	default:
		return "#64748B"
	}
}

func roadmapSortKey(s *Stream) string {
	si := servesIndex(s.Serves)
	pi := 2
	switch s.Priority {
	case "P0":
		pi = 0
	case "P1":
		pi = 1
	}
	return fmt.Sprintf("%d-%d-%s", si, pi, s.Name)
}

// --- Stage helpers ---

var roadmapStages = []string{"todo", "in-progress", "implemented", "verified", "done", "blocked"}

func stageCounts(s *Stream) map[string]int {
	m := map[string]int{}
	for _, b := range s.Briefs {
		m[b.Status]++
	}
	return m
}

func nextWave(s *Stream) int {
	next := 0
	for _, b := range s.Briefs {
		if b.Status == "done" || b.Status == "verified" {
			continue
		}
		if next == 0 || b.Wave < next {
			next = b.Wave
		}
	}
	return next
}

// findingFileByID maps a finding ID to its entry filename relative to
// docs/streams/findings/ (e.g. "F-ws-token-expiry" →
// "2026-07-17-ws-token-expiry.md"). The filename cannot be derived from the ID:
// register IDs are letter slugs (or frozen legacy F-NN numerics) while the
// filename is date-prefixed and slugged from the title. Returns nil when the
// register is absent or unreadable — callers then degrade to plain text.
func findingFileByID(root string) map[string]string {
	dir := filepath.Join(root, "docs", "streams", "findings")
	files, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	m := map[string]string{}
	for _, f := range files {
		if f.IsDir() || !strings.HasSuffix(f.Name(), ".md") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, f.Name()))
		if err != nil {
			continue
		}
		e, err := parseFindingFile(raw)
		if err != nil || e.ID == "" {
			continue
		}
		m[e.ID] = f.Name()
	}
	return m
}

// topBlocker returns the formatted blocker text for a stream and, when that
// blocker is a finding present in findingFiles, the finding's entry filename so
// the deck can hyperlink it. findingFiles may be nil; the link is then "" and
// the cell renders as plain text.
func topBlocker(s *Stream, findings []Finding, findingFiles map[string]string) (text, findingLink string) {
	// Check for unresolved findings affecting this stream.
	for _, f := range findings {
		if f.Resolved {
			continue
		}
		for _, a := range f.Affects {
			if strings.HasPrefix(a, s.Name) {
				return fmt.Sprintf("⚠ %s — %s", f.ID, f.Title), findingFiles[f.ID]
			}
		}
	}
	// Check for blocked briefs.
	for _, b := range s.Briefs {
		if b.Status == "blocked" {
			return fmt.Sprintf("Blocked: %s — %s", b.Num, b.Title), ""
		}
	}
	return "—", ""
}

// --- Delta computation ---

func computeRoadmapDeltas(streamName string, history []HistoryEntry, now time.Time) []string {
	cutoff := now.Add(-24 * time.Hour)
	type badge struct {
		kind  string // "done", "in-progress", "implemented", "new"
		count int
		label string
	}
	badges := map[string]*badge{}
	for _, e := range history {
		ts, err := time.Parse(time.RFC3339, e.Ts)
		if err != nil {
			continue
		}
		if !ts.After(cutoff) {
			continue
		}
		parts := strings.SplitN(e.Brief, "/", 2)
		if len(parts) != 2 || parts[0] != streamName {
			continue
		}
		switch {
		case e.From == "":
			key := "new"
			if b, ok := badges[key]; ok {
				b.count++
			} else {
				badges[key] = &badge{kind: "new", count: 1, label: "NEW"}
			}
		case e.To == "done" || e.To == "verified":
			key := "done"
			if b, ok := badges[key]; ok {
				b.count++
			} else {
				badges[key] = &badge{kind: "done", count: 1, label: fmt.Sprintf("+%d done", 1)}
			}
		case e.To == "in-progress" || e.To == "implemented":
			key := e.To
			if b, ok := badges[key]; ok {
				b.count++
			} else {
				badges[key] = &badge{kind: e.To, count: 1, label: fmt.Sprintf("%d→%s", 1, e.To)}
			}
		}
	}
	// Rebuild labels with correct counts.
	var deltas []string
	for _, b := range badges {
		switch b.kind {
		case "done":
			b.label = fmt.Sprintf("+%d done", b.count)
		case "in-progress":
			if b.count == 1 {
				b.label = "1→in-progress"
			} else {
				b.label = fmt.Sprintf("%d in-progress", b.count)
			}
		case "implemented":
			if b.count == 1 {
				b.label = "1→implemented"
			} else {
				b.label = fmt.Sprintf("%d implemented", b.count)
			}
		case "new":
			b.label = "NEW"
		}
		deltas = append(deltas, b.label)
	}
	sort.Strings(deltas)
	return deltas
}

// --- Health computation ---

func computeRoadmapHealth(s *Stream, streams []*Stream, findings []Finding, now time.Time) (color, reason string) {
	for _, rule := range roadmapHealthRules {
		if rule.Color == "green" {
			continue // green is the fallback, handled below
		}
		hit, msg := rule.Fires(s, nil, streams, findings, now)
		if hit {
			return rule.Color, msg
		}
	}
	return "green", ""
}

// --- Exception computation ---

func computeRoadmapExceptions(rows []roadmapStreamRow) []roadmapException {
	var ex []roadmapException
	for _, r := range rows {
		if r.HealthColor == "red" || r.HealthColor == "amber" {
			ex = append(ex, roadmapException{
				Stream: r.Stream.Name,
				Text:   r.HealthReason,
				Color:  r.HealthColor,
			})
		}
	}
	return ex
}

// --- Goal-mix computation ---

func computeGoalMix(streams []*Stream, nu NextUp, history []HistoryEntry) (mix []goalMix, inverted bool, invertMsg string) {
	// Goal definitions in priority order.
	goals := []goalMix{
		{Goal: "example-app", Label: "Example App"},
		{Goal: "example-service", Label: "Example Service"},
		{Goal: "assay", Label: "Assay"},
		{Goal: "platform", Label: "Platform"},
		{Goal: "untagged", Label: "Untagged"},
	}
	goalIdx := map[string]int{}
	for i, g := range goals {
		goalIdx[g.Goal] = i
	}

	// Active WIP: briefs in-progress per goal.
	for _, s := range streams {
		g := s.Serves
		if g == "" {
			g = "untagged"
		}
		idx, ok := goalIdx[g]
		if !ok {
			idx = goalIdx["untagged"]
		}
		for _, b := range s.Briefs {
			if b.Status == "in-progress" {
				goals[idx].Active++
			}
		}
	}

	// Next-up per goal.
	for _, p := range nu.Picks {
		g := p.Stream.Serves
		if g == "" {
			g = "untagged"
		}
		idx, ok := goalIdx[g]
		if !ok {
			idx = goalIdx["untagged"]
		}
		goals[idx].NextUp++
	}

	// Merged last 24h per goal: transitions to done/verified.
	if len(history) > 0 {
		// Find the latest history entry time for the 24h window.
		var latest time.Time
		for _, e := range history {
			ts, err := time.Parse(time.RFC3339, e.Ts)
			if err != nil {
				continue
			}
			if ts.After(latest) {
				latest = ts
			}
		}
		if !latest.IsZero() {
			cutoff := latest.Add(-24 * time.Hour)
			for _, e := range history {
				ts, err := time.Parse(time.RFC3339, e.Ts)
				if err != nil || !ts.After(cutoff) {
					continue
				}
				if e.To != "done" && e.To != "verified" {
					continue
				}
				parts := strings.SplitN(e.Brief, "/", 2)
				if len(parts) != 2 {
					continue
				}
				// Find the stream this brief belongs to.
				streamName := parts[0]
				var serves string
				for _, s := range streams {
					if s.Name == streamName {
						serves = s.Serves
						break
					}
				}
				if serves == "" {
					serves = "untagged"
				}
				idx, ok := goalIdx[serves]
				if !ok {
					idx = goalIdx["untagged"]
				}
				goals[idx].Merged24h++
			}
		}
	}

	// Inversion detection: if non-example-app goals dominate merges.
	total := 0
	primaryGoal := 0
	primaryLabel := ""
	for _, g := range goals {
		total += g.Merged24h
		if g.Goal == "example-app" {
			primaryGoal = g.Merged24h
			primaryLabel = g.Label
		}
	}
	if total > 0 {
		primaryPct := float64(primaryGoal) / float64(total) * 100
		if primaryPct < 30 {
			// Find which goal dominates.
			maxG := ""
			maxC := 0
			for _, g := range goals {
				if g.Merged24h > maxC {
					maxC = g.Merged24h
					maxG = g.Label
				}
			}
			inverted = true
			invertMsg = fmt.Sprintf("merges today: %.0f%% %s, %.0f%% %s — inverted vs stated priorities", float64(maxC)/float64(total)*100, maxG, primaryPct, primaryLabel)
		}
	}

	return goals, inverted, invertMsg
}

// --- HTML rendering ---

func htmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	return s
}

// roadmapTitle is the heading stamped into the rendered roadmap's <title> and
// <h1>. The owning org/deck label is derived from configured roster state (the
// HomeRepo owner) rather than a compiled-in company name; unset degrades to a
// bare "Portfolio Roadmap".
func roadmapTitle() string {
	const base = "Portfolio Roadmap"
	home := scanEffectiveConfig().HomeRepo
	if i := strings.Index(home, "/"); i > 0 {
		return home[:i] + " — " + base
	}
	return base
}

func renderRoadmap(streams []*Stream, rows []roadmapStreamRow, exceptions []roadmapException, goals []goalMix, inverted bool, invertMsg string, nu NextUp, findings []Finding, sha string, now time.Time) string {
	var b strings.Builder
	w := func(format string, a ...any) { fmt.Fprintf(&b, format+"\n", a...) }
	title := roadmapTitle()

	// --- CSS ---
	w("<!DOCTYPE html>")
	w("<html lang=\"en\">")
	w("<head>")
	w("<meta charset=\"UTF-8\">")
	w("<meta name=\"viewport\" content=\"width=device-width, initial-scale=1.0\">")
	w("<title>%s</title>", htmlEscape(title))
	w("<style>")
	w("  :root {")
	w("    --bg: #1a1a2e; --surface: #16213e; --blue: #3366FF; --green: #00CC66;")
	w("    --amber: #F59E0B; --red: #EF4444; --text: #E2E8F0; --text2: #94A3B8;")
	w("    --border: #1E293B;")
	w("  }")
	w("  * { box-sizing: border-box; margin: 0; padding: 0; }")
	w("  body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; background: var(--bg); color: var(--text); line-height: 1.5; padding: 24px; }")
	w("  h1 { font-size: 24px; font-weight: 700; color: var(--text); margin-bottom: 4px; }")
	w("  .subtitle { color: var(--text2); font-size: 13px; margin-bottom: 24px; }")
	w("  .subtitle code { background: var(--surface); padding: 1px 6px; border-radius: 4px; }")
	w("  .section { margin-bottom: 24px; }")
	w("  .section h2 { font-size: 16px; font-weight: 600; margin-bottom: 12px; color: var(--text); }")
	w("  .bar-row { display: flex; align-items: center; gap: 12px; margin-bottom: 8px; }")
	w("  .bar-seg { height: 24px; border-radius: 4px; min-width: 20px; display: flex; align-items: center; justify-content: center; font-size: 11px; font-weight: 600; color: #fff; }")
	w("  .bar-seg.todo { background: #475569; }")
	w("  .bar-seg.in-progress { background: var(--blue); }")
	w("  .bar-seg.implemented { background: var(--amber); }")
	w("  .bar-seg.verified { background: #06B6D4; }")
	w("  .bar-seg.done { background: var(--green); }")
	w("  .bar-seg.blocked { background: var(--red); }")
	w("  .bar-label { font-size: 12px; color: var(--text2); }")
	w("  .goal-mix { display: flex; gap: 16px; flex-wrap: wrap; }")
	w("  .goal-card { background: var(--surface); border-radius: 8px; padding: 12px 16px; min-width: 140px; }")
	w("  .goal-card .label { font-size: 12px; color: var(--text2); text-transform: uppercase; letter-spacing: 0.5px; }")
	w("  .goal-card .counts { font-size: 18px; font-weight: 700; }")
	w("  .goal-card .counts span { font-size: 12px; font-weight: 400; color: var(--text2); }")
	w("  .inversion { background: var(--surface); border-left: 3px solid var(--amber); border-radius: 0 8px 8px 0; padding: 10px 16px; font-size: 13px; color: var(--amber); margin-bottom: 12px; }")
	w("  .nextup-rows { font-size: 13px; }")
	w("  .nextup-rows .item { padding: 4px 0; }")
	w("  .nextup-rows a { color: var(--blue); text-decoration: none; }")
	w("  .nextup-rows a:hover { text-decoration: underline; }")
	w("  .exceptions-strip { font-size: 13px; }")
	w("  .exceptions-strip .exc { padding: 6px 12px; border-radius: 6px; margin-bottom: 6px; }")
	w("  .exc.red { background: rgba(239,68,68,0.15); border-left: 3px solid var(--red); }")
	w("  .exc.amber { background: rgba(245,158,11,0.15); border-left: 3px solid var(--amber); }")
	w("  .no-exceptions { color: var(--green); font-size: 13px; }")
	w("  table { width: 100%; border-collapse: collapse; font-size: 13px; }")
	w("  th { text-align: left; padding: 8px 12px; border-bottom: 1px solid var(--border); color: var(--text2); font-weight: 600; font-size: 11px; text-transform: uppercase; letter-spacing: 0.5px; }")
	w("  td { padding: 10px 12px; border-bottom: 1px solid var(--border); vertical-align: middle; }")
	w("  td a { color: var(--blue); text-decoration: none; }")
	w("  td a:hover { text-decoration: underline; }")
	w("  .health-pill { display: inline-flex; align-items: center; gap: 6px; }")
	w("  .health-dot { width: 10px; height: 10px; border-radius: 50%; flex-shrink: 0; }")
	w("  .health-dot.green { background: var(--green); }")
	w("  .health-dot.amber { background: var(--amber); }")
	w("  .health-dot.red { background: var(--red); }")
	w("  .health-reason { font-size: 11px; color: var(--text2); }")
	w("  .mini-bar { display: flex; gap: 2px; height: 14px; min-width: 80px; }")
	w("  .mini-seg { border-radius: 2px; }")
	w("  .mini-seg.todo { background: #475569; }")
	w("  .mini-seg.in-progress { background: var(--blue); }")
	w("  .mini-seg.implemented { background: var(--amber); }")
	w("  .mini-seg.verified { background: #06B6D4; }")
	w("  .mini-seg.done { background: var(--green); }")
	w("  .mini-seg.blocked { background: var(--red); }")
	w("  .delta { display: inline-block; background: var(--surface); border-radius: 4px; padding: 1px 6px; font-size: 11px; margin: 1px 2px; }")
	w("  .serves-tag { display: inline-block; border-radius: 4px; padding: 2px 8px; font-size: 11px; font-weight: 600; }")
	w("  .legend { background: var(--surface); border-radius: 8px; padding: 16px; font-size: 12px; }")
	w("  .legend .row { display: flex; align-items: center; gap: 8px; padding: 4px 0; }")
	w("  .legend-dot { width: 10px; height: 10px; border-radius: 50%; flex-shrink: 0; }")
	w("  .legend-dot.green { background: var(--green); }")
	w("  .legend-dot.amber { background: var(--amber); }")
	w("  .legend-dot.red { background: var(--red); }")
	w("  footer { color: var(--text2); font-size: 11px; margin-top: 32px; padding-top: 16px; border-top: 1px solid var(--border); }")
	w("  @media print { body { background: #fff; color: #000; padding: 0; } }")
	w("</style>")
	w("</head>")
	w("<body>")

	// --- Header ---
	w("<h1>%s</h1>", htmlEscape(title))
	shortSHA := sha
	if len(shortSHA) > 7 {
		shortSHA = shortSHA[:7]
	}
	w("<div class=\"subtitle\">Generated %s UTC · commit <code>%s</code></div>", now.Format("2006-01-02 15:04"), shortSHA)

	// --- Portfolio stage-stacked bar ---
	w("<div class=\"section\">")
	w("<h2>Portfolio Overview</h2>")
	totalCounts := map[string]int{}
	totalBriefs := 0
	for _, s := range streams {
		for _, b := range s.Briefs {
			totalCounts[b.Status]++
			totalBriefs++
		}
	}
	var barParts []struct {
		stage string
		count int
	}
	for _, st := range roadmapStages {
		if c := totalCounts[st]; c > 0 {
			barParts = append(barParts, struct {
				stage string
				count int
			}{st, c})
		}
	}
	w("<div class=\"bar-row\">")
	for _, bp := range barParts {
		pct := float64(bp.count) / float64(totalBriefs) * 100
		w("<div class=\"bar-seg %s\" style=\"flex: %d;\">%d</div>", bp.stage, bp.count, bp.count)
		_ = pct
	}
	w("</div>")
	w("<div class=\"bar-label\">%d total briefs across %d streams</div>", totalBriefs, len(streams))
	// Delta callouts for the bar: count transitions in last 24h from history.
	w("</div>")

	// --- Goal-mix strip ---
	w("<div class=\"section\">")
	w("<h2>Goal Mix</h2>")
	if inverted && invertMsg != "" {
		w("<div class=\"inversion\">⚠ %s</div>", htmlEscape(invertMsg))
	}
	w("<div class=\"goal-mix\">")
	for _, g := range goals {
		w("<div class=\"goal-card\">")
		w("<div class=\"label\">%s</div>", g.Label)
		w("<div class=\"counts\">%d<span> WIP</span> &middot; %d<span> Next-up</span> &middot; %d<span> merged 24h</span></div>", g.Active, g.NextUp, g.Merged24h)
		w("</div>")
	}
	w("</div>")
	w("<div style=\"font-size:12px;color:var(--text2);margin-top:8px;\">DORA rollup: pending</div>")
	w("</div>")

	// --- Next-up block ---
	w("<div class=\"section\">")
	w("<h2>Next Up (%d picks)</h2>", len(nu.Picks))
	if nu.Overflow() {
		w("<div style=\"font-size:12px;color:var(--amber);margin-bottom:8px;\">%d of %d eligible — %d held back (span cap %d)</div>", len(nu.Picks), nu.Eligible, nu.HeldBack(), nu.Span)
	}
	if len(nu.Picks) == 0 {
		w("<div class=\"no-exceptions\">Nothing eligible.</div>")
	} else {
		w("<div class=\"nextup-rows\">")
		for _, p := range nu.Picks {
			briefPath := fmt.Sprintf("docs/streams/%s/brief-%s", p.Stream.Name, p.Brief.Num)
			// Check if the brief file exists; link to the file if so.
			w("<div class=\"item\"><strong>%s</strong> &mdash; <a href=\"%s\">%s</a> (wave %d, score %d)</div>",
				p.Stream.Name, briefPath+".md", htmlEscape(p.Brief.Title), p.Brief.Wave, p.Score)
		}
		w("</div>")
	}
	w("</div>")

	// --- Exceptions strip ---
	w("<div class=\"section\">")
	w("<h2>Exceptions</h2>")
	if len(exceptions) == 0 {
		w("<div class=\"no-exceptions\">no exceptions</div>")
	} else {
		w("<div class=\"exceptions-strip\">")
		for _, e := range exceptions {
			w("<div class=\"exc %s\">%s: %s</div>", e.Color, htmlEscape(e.Stream), htmlEscape(e.Text))
		}
		w("</div>")
	}
	w("</div>")

	// --- Stream grid ---
	w("<div class=\"section\">")
	w("<h2>Streams</h2>")
	w("<table>")
	w("<thead><tr>")
	w("<th>Stream</th><th>Serves</th><th>Health</th><th>Stage</th><th>Deltas</th><th>Next Wave</th><th>Top Blocker</th><th>Owner</th>")
	w("</tr></thead>")
	w("<tbody>")
	for _, row := range rows {
		s := row.Stream
		streamURL := fmt.Sprintf("docs/streams/%s/README.md", s.Name)
		// Health pill.
		healthLabel := row.HealthReason
		if row.HealthColor == "green" {
			healthLabel = "green"
		}
		// Mini-bar.
		var mbParts []struct {
			stage string
			count int
		}
		for _, st := range roadmapStages {
			if c := row.StageCounts[st]; c > 0 {
				mbParts = append(mbParts, struct {
					stage string
					count int
				}{st, c})
			}
		}
		// Deltas.
		deltaHTML := ""
		for _, d := range row.Deltas {
			deltaHTML += fmt.Sprintf("<span class=\"delta\">%s</span> ", htmlEscape(d))
		}
		// Next wave.
		waveCell := "—"
		if row.NextWave > 0 {
			waveCell = fmt.Sprintf("Wave %d", row.NextWave)
		}
		// Owner.
		owner := row.Stream.Owner
		if owner == "" {
			owner = "—"
		}
		serves := row.Stream.Serves
		servesDisplay := serves
		if serves == "" {
			servesDisplay = "untagged"
		}
		w("<tr>")
		w("<td><a href=\"%s\">%s</a></td>", streamURL, s.Name)
		w("<td><span class=\"serves-tag\" style=\"background:%s20;color:%s;border:1px solid %s40\">%s</span></td>", servesColor(serves), servesColor(serves), servesColor(serves), servesDisplay)
		w("<td><div class=\"health-pill\"><span class=\"health-dot %s\"></span><span class=\"health-reason\">%s</span></div></td>", row.HealthColor, htmlEscape(healthLabel))
		w("<td><div class=\"mini-bar\">")
		for _, mp := range mbParts {
			w("<div class=\"mini-seg %s\" style=\"flex:%d;\"></div>", mp.stage, mp.count)
		}
		w("</div></td>")
		w("<td>%s</td>", deltaHTML)
		w("<td>%s</td>", waveCell)
		if row.BlockerFindingLink != "" {
			w("<td style=\"font-size:12px;\"><a href=\"../../streams/findings/%s\">%s</a></td>", htmlEscape(row.BlockerFindingLink), htmlEscape(row.TopBlocker))
		} else {
			w("<td style=\"font-size:12px;\">%s</td>", htmlEscape(row.TopBlocker))
		}
		w("<td>%s</td>", htmlEscape(owner))
		w("</tr>")
	}
	w("</tbody></table>")
	w("</div>")

	// --- Legend ---
	w("<div class=\"section\">")
	w("<h2>Legend</h2>")
	w("<div class=\"legend\">")
	for _, rule := range roadmapHealthRules {
		w("<div class=\"row\"><span class=\"legend-dot %s\"></span> <strong>%s</strong> &mdash; %s</div>", rule.Color, rule.Name, rule.Legend)
	}
	w("<div class=\"row\" style=\"margin-top:8px;color:var(--text2);\">Stage colors: <span style=\"color:#475569;\">todo</span> &middot; <span style=\"color:var(--blue);\">in-progress</span> &middot; <span style=\"color:var(--amber);\">implemented</span> &middot; <span style=\"color:#06B6D4;\">verified</span> &middot; <span style=\"color:var(--green);\">done</span> &middot; <span style=\"color:var(--red);\">blocked</span></div>")
	w("</div>")
	w("</div>")

	// --- Footer ---
	w("<footer>Generated daily &mdash; computed, never hand-asserted.</footer>")
	w("</body>")
	w("</html>")

	return b.String()
}

// --- Entry point ---

func runRoadmap(root string) int {
	streams, findings, err := loadStreams(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "statusgen:", err)
		return 1
	}
	attachPlaceholders(streams)

	// Load history.
	var history []HistoryEntry
	historyPath := filepath.Join(root, filepath.FromSlash(historyRelPath))
	if h, err := LoadHistory(historyPath); err == nil {
		history = h
	}

	// Compute briefTouch.
	briefTouch := map[string]time.Time{}
	if len(history) > 0 {
		briefTouch = LastTransitionTime(history)
	}

	// Set up the briefTouch provider for daysSinceLastTouch.
	roadmapBriefTouch = func() map[string]time.Time { return briefTouch }
	defer func() { roadmapBriefTouch = nil }()

	// Compute "now" from latest history entry for deterministic deltas.
	// Fall back to nowFunc() if no history.
	var now time.Time
	for _, e := range history {
		ts, err := time.Parse(time.RFC3339, e.Ts)
		if err != nil {
			continue
		}
		if ts.After(now) {
			now = ts
		}
	}
	if now.IsZero() {
		now = nowFunc()
	}
	now = now.UTC()

	// Build sorted stream list.
	sorted := make([]*Stream, len(streams))
	copy(sorted, streams)
	sort.Slice(sorted, func(i, j int) bool {
		return roadmapSortKey(sorted[i]) < roadmapSortKey(sorted[j])
	})

	// Compute stream rows.
	findingFiles := findingFileByID(root)
	var rows []roadmapStreamRow
	for _, s := range sorted {
		color, reason := computeRoadmapHealth(s, streams, findings, now)
		blockerText, blockerLink := topBlocker(s, findings, findingFiles)
		row := roadmapStreamRow{
			Stream:             s,
			HealthColor:        color,
			HealthReason:       reason,
			StageCounts:        stageCounts(s),
			Deltas:             computeRoadmapDeltas(s.Name, history, now),
			NextWave:           nextWave(s),
			TopBlocker:         blockerText,
			BlockerFindingLink: blockerLink,
		}
		rows = append(rows, row)
	}

	// Compute Next-up (reuse nextUp from nextup.go without claim filtering).
	nu := nextUp(streams, nil, briefTouch)

	// Compute exceptions.
	exceptions := computeRoadmapExceptions(rows)

	// Compute goal mix.
	goals, inverted, invertMsg := computeGoalMix(streams, nu, history)

	// Render.
	sha := gitCurrentSHA(root)
	headerNow := nowFunc().UTC()
	html := renderRoadmap(streams, rows, exceptions, goals, inverted, invertMsg, nu, findings, sha, headerNow)

	// Write output.
	outDir := filepath.Join(root, "docs", "reports", "roadmap")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "statusgen:", err)
		return 1
	}
	outPath := filepath.Join(outDir, "index.html")
	if err := os.WriteFile(outPath, []byte(html), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "statusgen:", err)
		return 1
	}
	fmt.Println("wrote", outPath)
	return 0
}
