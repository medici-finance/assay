// render.go — rollup.md, the human-readable companion to rollup.json.
//
// This file is where #1001 defects 1 and 2 actually bit: the numbers were
// right, and the TABLE was the lie. An unreadable tree rendered
// `| assay | 0 (0/0) | 0 | 0 | 0 |`; an empty-config grouping and an
// all-unreadable grouping rendered byte-identically; the only trace was a
// `## Gaps` section far below the figure a reader actually reads.
//
// So the rule here is: THE STATE IS RENDERED IN THE CELL. A figure that was
// not measured never renders as a numeral — it renders the word for what
// happened to it, in the same cell where the number would have been.
package main

import (
	"fmt"
	"sort"
	"strings"
)

// truncationMark is appended to a measured figure whose sources declared
// nothing about upstream truncation (#1001 defect 8). It is not decoration:
// it is the difference between "12" and "at least 12, and nobody checked".
const truncationMark = " ~"

// cell renders one measure into a table cell. Never returns a bare numeral
// for anything but a complete measurement.
func cell(m measure) string {
	switch m.State {
	case stateMeasured:
		s := fmt.Sprintf("%d", *m.Value)
		if len(m.Cov.TruncationUndeclared) > 0 {
			s += truncationMark
		}
		return s
	case stateWithheld:
		return fmt.Sprintf("withheld (%d/%d sources)", len(m.Cov.Measured), len(m.Cov.Configured))
	case stateCouldNotCheck:
		return fmt.Sprintf("could-not-check (0/%d sources)", len(m.Cov.Configured))
	case stateNotConfigured:
		return "not-configured"
	}
	return "unknown-state"
}

func deltaCell(d deltaMeasure) string {
	if d.State == deltaComputed {
		return fmt.Sprintf("%+d", *d.Value)
	}
	return "withheld"
}

func renderRollupMarkdown(top *topRollup) string {
	var b strings.Builder

	fmt.Fprintf(&b, "# Cross-domain roll-up — %s\n\n", top.Date)
	if top.RunID != "" {
		fmt.Fprintf(&b, "Reduced from the same run's four `metrics-snapshot-<grouping>-%s` "+
			"artifacts. ", top.RunID)
	} else {
		fmt.Fprintf(&b, "Reduced from one run's four `metrics-snapshot-<grouping>-<run-id>` "+
			"artifacts. ")
	}
	b.WriteString("No raw snapshot is committed — this roll-up is the durable record.\n\n")

	// --- headline table ---------------------------------------------------
	b.WriteString("| Grouping | Open PRs | draft | ready | Open issues, unlabelled |\n")
	b.WriteString("|---|---|---|---|---|\n")
	row := func(label string, f *figures) {
		fmt.Fprintf(&b, "| %s | %s | %s | %s | %s |\n", label,
			cell(f.PRsOpenByState.Total), cell(f.PRsOpenByState.Draft),
			cell(f.PRsOpenByState.Ready), cell(f.IssuesOpenUnlabeled))
	}
	for _, g := range productGroupings {
		row(g, &top.Products[g].figures)
	}
	row("**all products**", &top.AllProducts)
	if top.Org != nil {
		row("org *(org-wide, never summed into the product total)*", &top.Org.figures)
	}

	b.WriteString("\nEvery cell above is one of four states, and only the first is a number:\n\n")
	b.WriteString("- `<n>` — **measured**: every configured source for that figure contributed.\n")
	b.WriteString("- `withheld (m/n sources)` — some sources were unreadable, failed, or " +
		"truncated. A count is **not** published from a partial source set: a smaller number " +
		"would be indistinguishable from a real fall.\n")
	b.WriteString("- `could-not-check (0/n sources)` — sources are configured and **none** of " +
		"them could be read. This is not a zero.\n")
	b.WriteString("- `not-configured` — no repos are configured for that grouping at all, so " +
		"nothing was measured. Also not a zero.\n")
	fmt.Fprintf(&b, "- `%s` — the source did not declare whether its upstream listing hit the "+
		"API `--limit` cap, so the count may be a floor. The reducer could not check "+
		"(collector half: #1002 lane).\n", strings.TrimSpace(truncationMark))

	// --- review decision --------------------------------------------------
	b.WriteString("\n## Open PRs by review decision\n\n")
	if top.AllProducts.PRsOpenByReviewDecision == nil {
		b.WriteString("**Not collected.** " + top.AllProducts.PRsOpenByReviewDecisionNote + "\n")
	} else {
		renderLabelSection(&b, "all products", top.AllProducts.PRsOpenByReviewDecision)
		for _, g := range productGroupings {
			renderLabelSection(&b, g, top.Products[g].PRsOpenByReviewDecision)
		}
		if top.Org != nil {
			renderLabelSection(&b, "org", top.Org.PRsOpenByReviewDecision)
		}
	}

	// --- issue labels -----------------------------------------------------
	b.WriteString("\n## Open issues by label\n\n")
	b.WriteString("A label count is an **occurrence** count — an issue carrying two labels " +
		"contributes to two entries — so it is never summed into an issue count.\n")
	lm := top.AllProducts.IssuesOpenByLabel
	renderLabelSection(&b, "all products", &lm)
	for _, g := range productGroupings {
		x := top.Products[g].IssuesOpenByLabel
		renderLabelSection(&b, g, &x)
	}
	if top.Org != nil {
		x := top.Org.IssuesOpenByLabel
		renderLabelSection(&b, "org", &x)
	}

	// --- trend ------------------------------------------------------------
	renderTrend(&b, top)

	// --- coverage ---------------------------------------------------------
	renderCoverage(&b, top)

	return b.String()
}

func renderLabelSection(b *strings.Builder, label string, lm *labelMeasure) {
	if lm == nil {
		fmt.Fprintf(b, "\n### %s\n\nNot collected — the section is absent, never zero.\n", label)
		return
	}
	fmt.Fprintf(b, "\n### %s\n\n", label)
	if lm.State != stateMeasured {
		fmt.Fprintf(b, "**%s** — %s\n", lm.State, lm.Note)
		return
	}
	if len(lm.Labels) == 0 {
		fmt.Fprintf(b, "measured over %d source(s): no labelled open issues.\n", len(lm.Cov.Measured))
		return
	}
	names := make([]string, 0, len(lm.Labels))
	for k := range lm.Labels {
		names = append(names, k)
	}
	sort.Slice(names, func(i, j int) bool {
		if lm.Labels[names[i]] != lm.Labels[names[j]] {
			return lm.Labels[names[i]] > lm.Labels[names[j]]
		}
		return names[i] < names[j]
	})
	b.WriteString("| Label | Open issues carrying it |\n|---|---|\n")
	for _, n := range names {
		fmt.Fprintf(b, "| `%s` | %d |\n", n, lm.Labels[n])
	}
	if len(lm.Cov.TruncationUndeclared) > 0 {
		fmt.Fprintf(b, "\n%s: %s\n", strings.TrimSpace(truncationMark), lm.Note)
	}
}

func renderTrend(b *strings.Builder, top *topRollup) {
	t := top.Trend
	if t == nil {
		b.WriteString("\n## Trend\n\nNo trend was computed for this roll-up.\n")
		return
	}
	switch t.State {
	case trendNoPriorDay:
		fmt.Fprintf(b, "\n## Trend (vs %s)\n\n**No prior day's roll-up.** %s\n", t.PreviousDate, t.Note)
		return
	case trendPriorUnreadable:
		fmt.Fprintf(b, "\n## Trend (vs %s)\n\n**The prior day's roll-up could not be read.** %s\n\n"+
			"This is stated as a read failure, not as an absence: claiming \"no prior day\" "+
			"about a file that is on disk and failed to parse would launder a checked-failed "+
			"into a could-not-check (#1001 defect 5).\n", t.PreviousDate, t.Note)
		return
	}

	fmt.Fprintf(b, "\n## Trend (vs %s)\n\n", t.PreviousDate)
	b.WriteString("A delta is computed only when **both days measured that figure over the " +
		"same source set**. Otherwise it is `withheld`: a difference across changing coverage " +
		"is movement in the instrument, not in the world (#1001 defect 4).\n\n")
	b.WriteString("| Section | Δ open PRs | Δ draft | Δ ready | Δ unlabelled issues |\n")
	b.WriteString("|---|---|---|---|---|\n")
	drow := func(label string, d *deltaSet) {
		if d == nil {
			fmt.Fprintf(b, "| %s | withheld | withheld | withheld | withheld |\n", label)
			return
		}
		fmt.Fprintf(b, "| %s | %s | %s | %s | %s |\n", label,
			deltaCell(d.PRsOpenTotal), deltaCell(d.PRsOpenDraft),
			deltaCell(d.PRsOpenReady), deltaCell(d.IssuesOpenUnlabeled))
	}
	for _, g := range productGroupings {
		drow(g, t.Products[g])
	}
	drow("**all products**", t.AllProducts)
	drow("org", t.Org)
}

func renderCoverage(b *strings.Builder, top *topRollup) {
	b.WriteString("\n## Coverage\n\n")
	b.WriteString("Per figure — not per repo. One repo can be excluded from one figure and " +
		"included in three, and a single per-repo gap flag could not say so (#1001 defect 3).\n")

	scopes := []struct {
		name string
		gr   *groupingRollup
	}{}
	for _, g := range productGroupings {
		scopes = append(scopes, struct {
			name string
			gr   *groupingRollup
		}{g, top.Products[g]})
	}
	if top.Org != nil {
		scopes = append(scopes, struct {
			name string
			gr   *groupingRollup
		}{"org", top.Org})
	}

	for _, s := range scopes {
		fmt.Fprintf(b, "\n### %s\n\n", s.name)
		fmt.Fprintf(b, "Snapshot: `%s`", s.gr.Source)
		if s.gr.SourceNote != "" {
			fmt.Fprintf(b, " — %s", s.gr.SourceNote)
		}
		b.WriteString("\n\n")
		fmt.Fprintf(b, "Configured sources (%d): %s\n\n", len(s.gr.ReposConfigured),
			specList(s.gr.ReposConfigured))
		b.WriteString("| Figure | State | Measured | Failed | Missing | Truncated | Truncation undeclared |\n")
		b.WriteString("|---|---|---|---|---|---|---|\n")
		covRow := func(name string, st string, c coverage) {
			fmt.Fprintf(b, "| `%s` | %s | %s | %s | %s | %s | %s |\n", name, st,
				specList(c.Measured), specList(c.Failed), specList(c.Missing),
				specList(c.Truncated), specList(c.TruncationUndeclared))
		}
		f := &s.gr.figures
		for _, fg := range f.all() {
			covRow(fg.Name, fg.M.State, fg.M.Cov)
		}
		covRow("issuesOpenByLabel", f.IssuesOpenByLabel.State, f.IssuesOpenByLabel.Cov)
		if u := f.PRsOpenByState.Total.Cov.Unexpected; len(u) > 0 {
			fmt.Fprintf(b, "\n**Unconfigured sources in the snapshot** (never summed, always "+
				"reported): %s\n", specList(u))
		}
	}
}

func specList(specs []string) string {
	if len(specs) == 0 {
		return "—"
	}
	out := make([]string, 0, len(specs))
	for _, s := range specs {
		out = append(out, "`"+s+"`")
	}
	return strings.Join(out, ", ")
}
