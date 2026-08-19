package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// --- Per-stream roadmap pages (methodology-metrics/24) ---
//
// One identical-skeleton page per stream, sibling to the overview index.html
// under docs/reports/roadmap/. The overview's stream set IS the page set:
// renderAllStreamPages emits exactly one page per grid row, so the row<->page
// invariant holds by construction (no row without a page, no page without a
// row). The skeleton, in fixed order, matches the page-1 grid order:
//   1. header band (stream/owner/health + printed rule + one-line outcome + x/y)
//   2. "since yesterday" delta panel FIRST (explicit "no changes" empty state)
//   3. blockers & asks (computed rows, issue->effect->action; asserted asks distinct)
//   4. next wave gate (from the depends graph)
//   5. brief table grouped by wave (fully-done waves collapse to one line)
//   6. per-stream DORA tile (mm/26 --by stream), n= annotated
//   7. footer legend (from the same mm/23 rule table page 1 renders)

// streamPageFilename is the sibling page filename for a stream. Stream names are
// directory names under docs/streams/ and so are already filesystem-safe.
func streamPageFilename(streamName string) string { return streamName + ".html" }

// --- Header: one-line outcome ---

// streamOutcome returns a one-line outcome for a stream: the first sentence of
// the README's opening. The stream README's H1 carries the outcome as its
// em-dash tagline ("# X Stream — <outcome>"); we take that. Absent an H1
// tagline we fall back to the first prose sentence. "" when unreadable.
func streamOutcome(dir string) string {
	raw, err := os.ReadFile(filepath.Join(dir, "README.md"))
	if err != nil {
		return ""
	}
	lines := strings.Split(string(raw), "\n")
	// Prefer the H1 em-dash tagline.
	for _, ln := range lines {
		t := strings.TrimSpace(ln)
		if strings.HasPrefix(t, "# ") {
			head := strings.TrimSpace(strings.TrimPrefix(t, "# "))
			for _, sep := range []string{" — ", " - ", "—", " – "} {
				if i := strings.Index(head, sep); i >= 0 {
					return firstSentence(stripInlineMarkdown(head[i+len(sep):]))
				}
			}
			break
		}
	}
	// Fall back to the first prose paragraph (skip frontmatter, headings, bold
	// metadata lines starting with "**").
	inFront := false
	for i, ln := range lines {
		t := strings.TrimSpace(ln)
		if i == 0 && t == "---" {
			inFront = true
			continue
		}
		if inFront {
			if t == "---" {
				inFront = false
			}
			continue
		}
		if t == "" || strings.HasPrefix(t, "#") || strings.HasPrefix(t, "**") {
			continue
		}
		return firstSentence(stripInlineMarkdown(t))
	}
	return ""
}

// firstSentence returns text up to and including the first sentence terminator,
// trimmed. Avoids splitting on the "." inside a version or path by requiring the
// period to be followed by a space or end-of-string.
func firstSentence(s string) string {
	s = strings.TrimSpace(s)
	for i := 0; i < len(s); i++ {
		if s[i] == '.' || s[i] == '!' || s[i] == '?' {
			if i+1 >= len(s) || s[i+1] == ' ' {
				return strings.TrimSpace(s[:i+1])
			}
		}
	}
	return s
}

// stripInlineMarkdown removes the inline markdown that would otherwise leak into
// the plain-text outcome (links keep their text, emphasis/backticks dropped).
func stripInlineMarkdown(s string) string {
	// Markdown links [text](url) -> text.
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '[' {
			if close := strings.IndexByte(s[i:], ']'); close > 0 {
				text := s[i+1 : i+close]
				rest := s[i+close+1:]
				if strings.HasPrefix(rest, "(") {
					if paren := strings.IndexByte(rest, ')'); paren > 0 {
						b.WriteString(text)
						i = i + close + 1 + paren
						continue
					}
				}
			}
		}
		b.WriteByte(s[i])
	}
	out := b.String()
	for _, tok := range []string{"**", "`", "*", "_"} {
		out = strings.ReplaceAll(out, tok, "")
	}
	return strings.TrimSpace(out)
}

// --- Brief filename resolution (for resolvable links) ---

// briefFileByNum maps a brief number ("01", "12a") to its actual filename in the
// stream directory (brief-<num>-<slug>.md or issue-<num>-<slug>.md). The
// filename cannot be derived from the number alone — it carries a title slug —
// so we read the directory once per page. nil/empty on an unreadable dir; the
// caller then degrades to a README link.
func briefFileByNum(dir string) map[string]string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	m := map[string]string{}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".md") {
			continue
		}
		var prefix string
		switch {
		case strings.HasPrefix(name, "brief-"):
			prefix = "brief-"
		case strings.HasPrefix(name, "issue-"):
			prefix = "issue-"
		default:
			continue
		}
		rest := strings.TrimPrefix(name, prefix)
		num := rest
		if i := strings.IndexByte(rest, '-'); i >= 0 {
			num = rest[:i]
		} else {
			num = strings.TrimSuffix(rest, ".md")
		}
		if _, exists := m[num]; !exists {
			m[num] = name
		}
	}
	return m
}

// --- Delta panel (since yesterday) ---

type streamDelta struct {
	Transitions    []string // "01: todo → in-progress (a1b2c3d)"
	NewBriefs      []string // "07"
	FindingsOpened []string // "F-xyz — title"
}

func (d streamDelta) empty() bool {
	return len(d.Transitions) == 0 && len(d.NewBriefs) == 0 && len(d.FindingsOpened) == 0
}

// computeStreamDelta derives the last-24h changes for one stream from the
// historian and the findings register. Stage transitions (incl. the merge-commit
// SHA that carried a done/verified move), briefs newly created, and findings
// opened in the window. Deterministic: history is an ordered slice and the
// findings list is stable; nothing is map-ordered.
func computeStreamDelta(streamName string, history []HistoryEntry, findings []Finding, now time.Time) streamDelta {
	cutoff := now.Add(-24 * time.Hour)
	var d streamDelta
	for _, e := range history {
		ts, err := time.Parse(time.RFC3339, e.Ts)
		if err != nil || !ts.After(cutoff) || ts.After(now) {
			continue
		}
		parts := strings.SplitN(e.Brief, "/", 2)
		if len(parts) != 2 || parts[0] != streamName {
			continue
		}
		num := parts[1]
		if e.From == "" {
			d.NewBriefs = append(d.NewBriefs, num)
			continue
		}
		line := fmt.Sprintf("%s: %s → %s", num, e.From, e.To)
		if (e.To == "done" || e.To == "verified") && e.SHA != "" {
			sha := e.SHA
			if len(sha) > 7 {
				sha = sha[:7]
			}
			line += fmt.Sprintf(" (%s)", sha)
		}
		d.Transitions = append(d.Transitions, line)
	}
	for _, f := range findings {
		if !findingAffectsStream(f, streamName) {
			continue
		}
		if ft, err := time.Parse("2006-01-02", f.Date); err == nil {
			// Day-granular open date; count as opened when its day is within the
			// 24h window's day span.
			if ft.After(cutoff.Add(-24*time.Hour)) && !ft.After(now) {
				d.FindingsOpened = append(d.FindingsOpened, fmt.Sprintf("%s — %s", f.ID, f.Title))
			}
		}
	}
	return d
}

// findingAffectsStream reports whether any Affects entry names this stream (bare
// "stream" or "stream/brief-NN"/"stream/NN").
func findingAffectsStream(f Finding, streamName string) bool {
	for _, a := range f.Affects {
		if a == streamName || strings.HasPrefix(a, streamName+"/") {
			return true
		}
	}
	return false
}

// --- Blockers & asks (computed rows, issue -> effect -> action) ---

type blockerRow struct {
	Issue    string
	Effect   string
	Action   string
	Finding  string // finding entry filename when this row references a finding; "" otherwise
	Asserted bool   // true for a hand-authored README ask (rendered visually distinct)
}

// streamBlockers computes the typed-graph blocker rows for a stream: a brief
// blocked by a dependency not yet done, an env-blocked brief, and unresolved
// findings affecting the stream or one of its briefs. allStatus maps
// "stream/NN" -> status across the whole portfolio.
func streamBlockers(s *Stream, allStatus map[string]string, findings []Finding, findingFiles map[string]string) []blockerRow {
	var rows []blockerRow
	done := func(st string) bool { return st == "done" || st == "verified" }
	for _, b := range s.Briefs {
		if done(b.Status) {
			continue
		}
		for _, dep := range b.Depends {
			if st, ok := allStatus[dep]; !ok || !done(st) {
				depState := "not yet started"
				if ok {
					depState = st
				}
				rows = append(rows, blockerRow{
					Issue:  fmt.Sprintf("%s depends on %s", b.Num, dep),
					Effect: fmt.Sprintf("%s is %s — brief %s cannot reach verified", dep, depState, b.Num),
					Action: fmt.Sprintf("advance %s to verified", dep),
				})
			}
		}
		if b.BlockedBy == "env" {
			rows = append(rows, blockerRow{
				Issue:  fmt.Sprintf("%s blocked on environment", b.Num),
				Effect: "waiting on infrastructure/environment outside this repo",
				Action: "resolve the env dependency, then unblock",
			})
		}
	}
	for _, f := range findings {
		if f.Resolved || !findingAffectsStream(f, s.Name) {
			continue
		}
		// Prefer a brief-scoped affects entry for a precise issue line.
		scope := "the stream"
		for _, a := range f.Affects {
			if strings.HasPrefix(a, s.Name+"/") {
				scope = "brief " + strings.TrimPrefix(strings.TrimPrefix(a, s.Name+"/"), "brief-")
				break
			}
		}
		rows = append(rows, blockerRow{
			Issue:   fmt.Sprintf("finding %s affects %s", f.ID, scope),
			Effect:  f.Title,
			Action:  "resolve the finding, then re-verify",
			Finding: findingFiles[f.ID],
		})
	}
	return rows
}

// streamAsserted parses hand-authored asks from the stream README's "## Asks"
// (or "## Blockers") section — the list items following the heading, up to the
// next heading. These render visually distinct from computed rows (asserted !=
// computed). Empty when the section is absent.
func streamAsserted(dir string) []blockerRow {
	raw, err := os.ReadFile(filepath.Join(dir, "README.md"))
	if err != nil {
		return nil
	}
	lines := strings.Split(string(raw), "\n")
	var rows []blockerRow
	in := false
	for _, ln := range lines {
		t := strings.TrimSpace(ln)
		if strings.HasPrefix(t, "#") {
			if in {
				break // next heading ends the section
			}
			lower := strings.ToLower(t)
			if strings.HasPrefix(lower, "## ") && (strings.Contains(lower, "asks") || strings.Contains(lower, "blockers")) {
				in = true
			}
			continue
		}
		if !in {
			continue
		}
		if strings.HasPrefix(t, "- ") || strings.HasPrefix(t, "* ") {
			rows = append(rows, blockerRow{
				Issue:    stripInlineMarkdown(strings.TrimSpace(t[2:])),
				Asserted: true,
			})
		}
	}
	return rows
}

// --- Next wave gate ---

// nextWaveGate returns the human sentence describing what unlocks the next
// incomplete wave, computed from the depends graph. "" input streams and
// all-complete streams are handled explicitly.
func nextWaveGate(s *Stream, allStatus map[string]string) string {
	nw := nextWave(s)
	if nw == 0 {
		return "All waves complete."
	}
	done := func(st string) bool { return st == "done" || st == "verified" }
	// Gate briefs: the dependencies of not-yet-done wave-nw briefs that are not
	// themselves verified/done.
	gate := map[string]bool{}
	for _, b := range s.Briefs {
		if b.Wave != nw || done(b.Status) {
			continue
		}
		for _, dep := range b.Depends {
			if st, ok := allStatus[dep]; !ok || !done(st) {
				gate[dep] = true
			}
		}
	}
	if len(gate) == 0 {
		return fmt.Sprintf("Wave %d is unblocked — no outstanding dependencies; its briefs can proceed now.", nw)
	}
	keys := make([]string, 0, len(gate))
	for k := range gate {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return fmt.Sprintf("Wave %d unlocks when %s reach verified.", nw, strings.Join(keys, ", "))
}

// --- Per-brief 24h delta set (for the Δ column) ---

// briefsTouchedIn24h returns the set of brief numbers in a stream that recorded
// any transition in the last 24h.
func briefsTouchedIn24h(streamName string, history []HistoryEntry, now time.Time) map[string]bool {
	cutoff := now.Add(-24 * time.Hour)
	out := map[string]bool{}
	for _, e := range history {
		ts, err := time.Parse(time.RFC3339, e.Ts)
		if err != nil || !ts.After(cutoff) || ts.After(now) {
			continue
		}
		parts := strings.SplitN(e.Brief, "/", 2)
		if len(parts) == 2 && parts[0] == streamName {
			out[parts[1]] = true
		}
	}
	return out
}

// --- Rendering ---

// renderAllStreamPages renders one page per grid row and returns a
// filename->html map. Exactly one entry per row: this IS the row<->page
// invariant (len(result) == len(rows), keyed by the row's stream page filename).
func renderAllStreamPages(
	rows []roadmapStreamRow, streams []*Stream, history []HistoryEntry,
	findings []Finding, findingFiles map[string]string, doraGroups map[string]DoraGroup,
	sha string, now, headerNow time.Time,
) map[string]string {
	// Portfolio-wide status index for depends resolution.
	allStatus := map[string]string{}
	for _, st := range streams {
		for _, b := range st.Briefs {
			allStatus[st.Name+"/"+b.Num] = b.Status
		}
	}
	out := make(map[string]string, len(rows))
	for _, row := range rows {
		grp, ok := doraGroups[row.Stream.Name]
		var grpPtr *DoraGroup
		if ok {
			g := grp
			grpPtr = &g
		}
		html := renderStreamPage(row, allStatus, history, findings, findingFiles, grpPtr, sha, now, headerNow)
		out[streamPageFilename(row.Stream.Name)] = html
	}
	return out
}

func renderStreamPage(
	row roadmapStreamRow, allStatus map[string]string, history []HistoryEntry,
	findings []Finding, findingFiles map[string]string, dora *DoraGroup,
	sha string, now, headerNow time.Time,
) string {
	s := row.Stream
	var b strings.Builder
	w := func(format string, a ...any) { fmt.Fprintf(&b, format+"\n", a...) }

	title := roadmapTitle()
	pageTitle := fmt.Sprintf("%s — %s", s.Name, title)

	w("<!DOCTYPE html>")
	w("<html lang=\"en\">")
	w("<head>")
	w("<meta charset=\"UTF-8\">")
	w("<meta name=\"viewport\" content=\"width=device-width, initial-scale=1.0\">")
	w("<title>%s</title>", htmlEscape(pageTitle))
	w("<style>")
	w("%s", streamPageCSS())
	w("</style>")
	w("</head>")
	w("<body>")

	// Back link.
	w("<div class=\"backlink\"><a href=\"index.html\">&larr; Portfolio Overview</a></div>")

	// --- 1. Header band ---
	readmeURL := fmt.Sprintf("../../streams/%s/README.md", s.Name)
	owner := s.Owner
	if owner == "" {
		owner = "—"
	}
	healthLabel := row.HealthReason
	if row.HealthColor == "green" {
		healthLabel = "green"
	}
	w("<div class=\"header-band\">")
	w("<h1><a href=\"%s\">%s</a></h1>", readmeURL, htmlEscape(s.Name))
	w("<div class=\"header-meta\">")
	w("<span class=\"health-pill\"><span class=\"health-dot %s\"></span>%s</span>", row.HealthColor, htmlEscape(healthLabel))
	w("<span class=\"meta-sep\">·</span> owner %s", htmlEscape(owner))
	servesDisplay := s.Serves
	if servesDisplay == "" {
		servesDisplay = "untagged"
	}
	w("<span class=\"meta-sep\">·</span> <span class=\"serves-tag\" style=\"background:%s20;color:%s;border:1px solid %s40\">%s</span>",
		servesColor(s.Serves), servesColor(s.Serves), servesColor(s.Serves), servesDisplay)
	w("</div>")
	if outcome := streamOutcome(s.Dir); outcome != "" {
		w("<div class=\"outcome\">%s</div>", htmlEscape(outcome))
	}
	// x done / y total + stage bar.
	total := 0
	for _, c := range row.StageCounts {
		total += c
	}
	doneCount := row.StageCounts["done"] + row.StageCounts["verified"]
	w("<div class=\"progress-line\">%d done / %d total briefs</div>", doneCount, total)
	w("<div class=\"stage-bar\">")
	for _, st := range roadmapStages {
		if c := row.StageCounts[st]; c > 0 {
			w("<div class=\"bar-seg %s\" style=\"flex:%d;\" title=\"%s: %d\">%d</div>", st, c, st, c, c)
		}
	}
	w("</div>")
	w("</div>") // header-band

	// --- 2. Delta panel FIRST ---
	delta := computeStreamDelta(s.Name, history, findings, now)
	w("<div class=\"section\">")
	w("<h2>Since yesterday</h2>")
	w("<div class=\"delta-panel\">")
	// Always-present lowercase caption: the panel is the exec's second read
	// (research: what-materially-changed follows status), so it is labelled on
	// every page whether or not there is anything in it.
	w("<div class=\"delta-caption\">changes since yesterday (last 24h)</div>")
	if delta.empty() {
		w("<div class=\"delta-empty\">no changes</div>")
	} else {
		if len(delta.Transitions) > 0 {
			w("<div class=\"delta-group\"><span class=\"delta-kind\">Stage transitions</span><ul>")
			for _, t := range delta.Transitions {
				w("<li>%s</li>", htmlEscape(t))
			}
			w("</ul></div>")
		}
		if len(delta.NewBriefs) > 0 {
			w("<div class=\"delta-group\"><span class=\"delta-kind\">New briefs</span><ul>")
			for _, n := range delta.NewBriefs {
				w("<li>%s</li>", htmlEscape(n))
			}
			w("</ul></div>")
		}
		if len(delta.FindingsOpened) > 0 {
			w("<div class=\"delta-group\"><span class=\"delta-kind\">Findings opened</span><ul>")
			for _, f := range delta.FindingsOpened {
				w("<li>%s</li>", htmlEscape(f))
			}
			w("</ul></div>")
		}
	}
	w("</div>")
	w("</div>")

	// --- 3. Blockers & asks ---
	computed := streamBlockers(s, allStatus, findings, findingFiles)
	asserted := streamAsserted(s.Dir)
	w("<div class=\"section\">")
	w("<h2>Blockers &amp; asks</h2>")
	if len(computed) == 0 && len(asserted) == 0 {
		w("<div class=\"no-exceptions\">no blockers</div>")
	} else {
		if len(computed) > 0 {
			w("<table class=\"blocker-table\">")
			w("<thead><tr><th>Issue</th><th>Effect</th><th>Action</th></tr></thead><tbody>")
			for _, r := range computed {
				issueCell := htmlEscape(r.Issue)
				if r.Finding != "" {
					issueCell = fmt.Sprintf("<a href=\"../../streams/findings/%s\">%s</a>", htmlEscape(r.Finding), htmlEscape(r.Issue))
				}
				w("<tr><td>%s</td><td>%s</td><td>%s</td></tr>", issueCell, htmlEscape(r.Effect), htmlEscape(r.Action))
			}
			w("</tbody></table>")
		}
		if len(asserted) > 0 {
			w("<div class=\"asserted-asks\">")
			w("<div class=\"asserted-label\">Asserted (hand-authored, from the stream README)</div>")
			w("<ul>")
			for _, r := range asserted {
				w("<li>%s</li>", htmlEscape(r.Issue))
			}
			w("</ul>")
			w("</div>")
		}
	}
	w("</div>")

	// --- 4. Next wave gate ---
	w("<div class=\"section\">")
	w("<h2>Next wave gate</h2>")
	w("<div class=\"wave-gate\">%s</div>", htmlEscape(nextWaveGate(s, allStatus)))
	w("</div>")

	// --- 5. Brief table grouped by wave (done waves collapse) ---
	w("<div class=\"section\">")
	w("<h2>Briefs by wave</h2>")
	renderBriefWaves(w, s, allStatus, history, now)
	w("</div>")

	// --- 6. Per-stream DORA tile ---
	w("<div class=\"section\">")
	w("<h2>DORA (per stream)</h2>")
	renderStreamDoraTile(w, dora)
	w("</div>")

	// --- 7. Footer legend (from the mm/23 rule table) ---
	w("<div class=\"section\">")
	w("<h2>Legend</h2>")
	w("<div class=\"legend\">")
	for _, rule := range roadmapHealthRules {
		w("<div class=\"row\"><span class=\"legend-dot %s\"></span> <strong>%s</strong> &mdash; %s</div>", rule.Color, rule.Name, rule.Legend)
	}
	w("<div class=\"row\" style=\"margin-top:8px;color:var(--text2);\">Stage colors: <span style=\"color:#475569;\">todo</span> &middot; <span style=\"color:var(--blue);\">in-progress</span> &middot; <span style=\"color:var(--amber);\">implemented</span> &middot; <span style=\"color:#06B6D4;\">verified</span> &middot; <span style=\"color:var(--green);\">done</span> &middot; <span style=\"color:var(--red);\">blocked</span></div>")
	w("</div>")
	w("</div>")

	shortSHA := sha
	if len(shortSHA) > 7 {
		shortSHA = shortSHA[:7]
	}
	w("<footer>Generated %s UTC &middot; commit <code>%s</code> &mdash; computed, never hand-asserted.</footer>", headerNow.Format("2006-01-02 15:04"), htmlEscape(shortSHA))
	w("</body>")
	w("</html>")
	return b.String()
}

// renderBriefWaves renders the per-wave brief listing. A wave whose briefs are
// all done/verified collapses to one summary line so large streams hold a page;
// any other wave renders its full row set.
func renderBriefWaves(w func(string, ...any), s *Stream, allStatus map[string]string, history []HistoryEntry, now time.Time) {
	files := briefFileByNum(s.Dir)
	touched := briefsTouchedIn24h(s.Name, history, now)
	done := func(st string) bool { return st == "done" || st == "verified" }

	// Group briefs by wave, preserving parse order within a wave, waves ascending.
	waveOrder := []int{}
	byWave := map[int][]Brief{}
	for _, br := range s.Briefs {
		if _, seen := byWave[br.Wave]; !seen {
			waveOrder = append(waveOrder, br.Wave)
		}
		byWave[br.Wave] = append(byWave[br.Wave], br)
	}
	sort.Ints(waveOrder)

	if len(waveOrder) == 0 {
		w("<div class=\"no-exceptions\">no briefs</div>")
		return
	}

	for _, wv := range waveOrder {
		briefs := byWave[wv]
		allDone := true
		for _, br := range briefs {
			if !done(br.Status) {
				allDone = false
				break
			}
		}
		if allDone {
			w("<div class=\"wave-collapsed\"><span class=\"wave-tag\">Wave %d</span> %d briefs — all complete</div>", wv, len(briefs))
			continue
		}
		w("<div class=\"wave-tag-heading\">Wave %d</div>", wv)
		w("<table class=\"brief-table\">")
		w("<thead><tr><th>Brief</th><th>Title</th><th>Effort</th><th>Status</th><th>Days in stage</th><th>Δ</th><th>Blocked by</th></tr></thead><tbody>")
		for _, br := range briefs {
			idCell := htmlEscape(br.Num)
			if fn, ok := files[br.Num]; ok {
				idCell = fmt.Sprintf("<a href=\"../../streams/%s/%s\">%s</a>", htmlEscape(s.Name), htmlEscape(fn), htmlEscape(br.Num))
			}
			effort := br.Effort
			if effort == "" {
				effort = "—"
			}
			days := daysSinceLastTouch(s, br, now)
			deltaBadge := ""
			if touched[br.Num] {
				deltaBadge = "<span class=\"delta-dot\" title=\"changed in last 24h\">Δ</span>"
			}
			// Blocked-by refs: unsatisfied deps + env block.
			var blockedBy []string
			for _, dep := range br.Depends {
				if st, ok := allStatus[dep]; !ok || !done(st) {
					blockedBy = append(blockedBy, dep)
				}
			}
			if br.BlockedBy == "env" {
				blockedBy = append(blockedBy, "env")
			}
			bbCell := "—"
			if len(blockedBy) > 0 {
				bbCell = htmlEscape(strings.Join(blockedBy, ", "))
			}
			w("<tr><td>%s</td><td>%s</td><td>%s</td><td><span class=\"status-token %s\">%s</span></td><td>%d</td><td>%s</td><td>%s</td></tr>",
				idCell, htmlEscape(br.Title), htmlEscape(effort), br.Status, br.Status, days, deltaBadge, bbCell)
		}
		w("</tbody></table>")
	}
}

// renderStreamDoraTile renders the per-stream DORA tile (mm/26 --by stream):
// lead time median/p90, deploy freq, CFR proxy — each n= annotated, small-n
// honesty rendered rather than hidden.
func renderStreamDoraTile(w func(string, ...any), dora *DoraGroup) {
	if dora == nil {
		w("<div class=\"no-exceptions\">no DORA data</div>")
		return
	}
	nNote := fmt.Sprintf("n=%d", dora.N)
	if dora.SmallN {
		nNote += " · small-n: an anecdote, not a metric"
	}
	metric := func(key, label string) (string, string) {
		if m, ok := dora.Metrics[key]; ok {
			return m.Value, m.Detail
		}
		return "unknown", ""
	}
	ltVal, _ := metric(doraLeadTime, "Change lead time")
	dfVal, _ := metric(doraDeployFreq, "Deployment frequency")
	cfVal, _ := metric(doraChangeFail, "Change failure rate")
	w("<div class=\"dora-tile\">")
	w("<div class=\"dora-cell\"><div class=\"dora-label\">Lead time (median / p90)</div><div class=\"dora-value\">%s</div><div class=\"dora-n\">%s</div></div>", htmlEscape(ltVal), htmlEscape(nNote))
	w("<div class=\"dora-cell\"><div class=\"dora-label\">Deploy frequency</div><div class=\"dora-value\">%s</div><div class=\"dora-n\">%s</div></div>", htmlEscape(dfVal), htmlEscape(nNote))
	w("<div class=\"dora-cell\"><div class=\"dora-label\">Change-failure rate (proxy)</div><div class=\"dora-value\">%s</div><div class=\"dora-n\">%s</div></div>", htmlEscape(cfVal), htmlEscape(nNote))
	w("</div>")
}

// streamPageCSS is the self-contained stylesheet for a stream page. Same brand
// tokens as the mm/23 overview; extra classes for the delta panel, blocker
// table, wave gate, brief table and DORA tile.
func streamPageCSS() string {
	return `  :root {
    --bg: #1a1a2e; --surface: #16213e; --blue: #3366FF; --green: #00CC66;
    --amber: #F59E0B; --red: #EF4444; --text: #E2E8F0; --text2: #94A3B8;
    --border: #1E293B;
  }
  * { box-sizing: border-box; margin: 0; padding: 0; }
  body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; background: var(--bg); color: var(--text); line-height: 1.5; padding: 24px; }
  .backlink { margin-bottom: 12px; font-size: 13px; }
  .backlink a { color: var(--blue); text-decoration: none; }
  .backlink a:hover { text-decoration: underline; }
  .header-band { background: var(--surface); border-radius: 10px; padding: 20px 24px; margin-bottom: 24px; }
  h1 { font-size: 22px; font-weight: 700; margin-bottom: 6px; }
  h1 a { color: var(--text); text-decoration: none; }
  h1 a:hover { text-decoration: underline; }
  .header-meta { font-size: 13px; color: var(--text2); display: flex; align-items: center; gap: 8px; flex-wrap: wrap; margin-bottom: 8px; }
  .meta-sep { color: var(--border); }
  .outcome { font-size: 14px; color: var(--text); margin-bottom: 12px; font-style: italic; }
  .progress-line { font-size: 12px; color: var(--text2); margin-bottom: 6px; }
  .stage-bar, .bar-row { display: flex; gap: 3px; height: 22px; }
  .bar-seg { height: 22px; border-radius: 4px; min-width: 18px; display: flex; align-items: center; justify-content: center; font-size: 11px; font-weight: 600; color: #fff; }
  .bar-seg.todo { background: #475569; }
  .bar-seg.in-progress { background: var(--blue); }
  .bar-seg.implemented { background: var(--amber); }
  .bar-seg.verified { background: #06B6D4; }
  .bar-seg.done { background: var(--green); }
  .bar-seg.blocked { background: var(--red); }
  .section { margin-bottom: 24px; }
  .section h2 { font-size: 15px; font-weight: 600; margin-bottom: 12px; }
  .health-pill { display: inline-flex; align-items: center; gap: 6px; }
  .health-dot { width: 10px; height: 10px; border-radius: 50%; flex-shrink: 0; }
  .health-dot.green { background: var(--green); }
  .health-dot.amber { background: var(--amber); }
  .health-dot.red { background: var(--red); }
  .serves-tag { display: inline-block; border-radius: 4px; padding: 2px 8px; font-size: 11px; font-weight: 600; }
  .delta-panel { background: var(--surface); border-radius: 8px; padding: 14px 18px; font-size: 13px; }
  .delta-caption { font-size: 11px; text-transform: uppercase; letter-spacing: 0.5px; color: var(--text2); margin-bottom: 10px; }
  .delta-empty { color: var(--text2); }
  .delta-group { margin-bottom: 10px; }
  .delta-group:last-child { margin-bottom: 0; }
  .delta-kind { display: block; font-size: 11px; text-transform: uppercase; letter-spacing: 0.5px; color: var(--text2); margin-bottom: 4px; }
  .delta-group ul { list-style: none; }
  .delta-group li { padding: 2px 0; }
  table { width: 100%; border-collapse: collapse; font-size: 13px; }
  th { text-align: left; padding: 8px 12px; border-bottom: 1px solid var(--border); color: var(--text2); font-weight: 600; font-size: 11px; text-transform: uppercase; letter-spacing: 0.5px; }
  td { padding: 9px 12px; border-bottom: 1px solid var(--border); vertical-align: middle; }
  td a { color: var(--blue); text-decoration: none; }
  td a:hover { text-decoration: underline; }
  .blocker-table td:first-child { font-weight: 600; }
  .asserted-asks { margin-top: 12px; background: rgba(245,158,11,0.10); border-left: 3px solid var(--amber); border-radius: 0 8px 8px 0; padding: 10px 16px; }
  .asserted-label { font-size: 11px; text-transform: uppercase; letter-spacing: 0.5px; color: var(--amber); margin-bottom: 6px; }
  .asserted-asks ul { list-style: disc; padding-left: 18px; font-size: 13px; }
  .wave-gate { background: var(--surface); border-radius: 8px; padding: 12px 16px; font-size: 13px; }
  .wave-tag-heading { font-size: 12px; font-weight: 700; color: var(--text2); text-transform: uppercase; letter-spacing: 0.5px; margin: 14px 0 6px; }
  .wave-collapsed { background: var(--surface); border-radius: 6px; padding: 8px 14px; font-size: 13px; color: var(--text2); margin-bottom: 6px; }
  .wave-tag { display: inline-block; font-weight: 700; color: var(--green); margin-right: 8px; }
  .status-token { display: inline-block; border-radius: 4px; padding: 1px 8px; font-size: 11px; font-weight: 600; color: #fff; }
  .status-token.todo { background: #475569; }
  .status-token.in-progress { background: var(--blue); }
  .status-token.implemented { background: var(--amber); }
  .status-token.verified { background: #06B6D4; }
  .status-token.done { background: var(--green); }
  .status-token.blocked { background: var(--red); }
  .delta-dot { color: var(--amber); font-weight: 700; }
  .dora-tile { display: flex; gap: 16px; flex-wrap: wrap; }
  .dora-cell { background: var(--surface); border-radius: 8px; padding: 12px 16px; min-width: 200px; flex: 1; }
  .dora-label { font-size: 11px; text-transform: uppercase; letter-spacing: 0.5px; color: var(--text2); margin-bottom: 6px; }
  .dora-value { font-size: 16px; font-weight: 700; margin-bottom: 4px; }
  .dora-n { font-size: 11px; color: var(--text2); }
  .no-exceptions { color: var(--green); font-size: 13px; }
  .legend { background: var(--surface); border-radius: 8px; padding: 16px; font-size: 12px; }
  .legend .row { display: flex; align-items: center; gap: 8px; padding: 4px 0; }
  .legend-dot { width: 10px; height: 10px; border-radius: 50%; flex-shrink: 0; }
  .legend-dot.green { background: var(--green); }
  .legend-dot.amber { background: var(--amber); }
  .legend-dot.red { background: var(--red); }
  footer { color: var(--text2); font-size: 11px; margin-top: 32px; padding-top: 16px; border-top: 1px solid var(--border); }
  @media print { body { background: #fff; color: #000; padding: 0; } }`
}
