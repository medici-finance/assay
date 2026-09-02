package main

import (
	"fmt"
	"strings"
)

// renderMarkdown renders the human-facing pack: the honest header, the scope
// boundary, the completeness line and any omissions, the declared-set drift in
// both directions, then one section per gate carrying its four-column
// instrument view and one row per injected mutation. It renders from the same
// *pack the JSON does, so the two formats cannot disagree.
func (p *pack) renderMarkdown() string {
	var b strings.Builder

	fmt.Fprintf(&b, "# Tool-validation evidence pack — %s\n\n", p.Tag)
	fmt.Fprintf(&b, "Generated %s from the release's captured `muhar` mutation reports.\n\n", p.Generated)

	b.WriteString(packHeader)
	b.WriteString("\n")

	// Completeness — the exit-code contract, stated on the page.
	if p.Complete {
		fmt.Fprintf(&b, "**Status: COMPLETE.** All %d declared controls had a trustworthy report (exit 0).\n\n", p.Declared)
	} else {
		fmt.Fprintf(&b, "**Status: INCOMPLETE (exit 3).** %d of %d declared controls could not be evidenced this run; each is named under *Omitted* below. A silently incomplete pack is a worse outcome than a failed export, so this is deliberately non-zero and fails the release step.\n\n", len(p.Omitted), p.Declared)
	}

	// Scope boundary (D6).
	b.WriteString("## Scope\n\n")
	b.WriteString(packScope)
	b.WriteString("\n")

	// Omitted — the pack's own statement of what it is missing.
	if len(p.Omitted) > 0 {
		b.WriteString("## Omitted\n\n")
		b.WriteString("| Spec | Reason |\n|------|--------|\n")
		for _, o := range p.Omitted {
			fmt.Fprintf(&b, "| `%s` | %s |\n", o.Spec, mdCell(o.Reason))
		}
		b.WriteString("\n")
	}

	// Declared-set drift, both directions.
	b.WriteString("## Declared-set drift\n\n")
	if len(p.Drift.DeclaredSpecsWithNoFile) == 0 && len(p.Drift.SpecFilesNotDeclared) == 0 {
		b.WriteString("None: every declared control has a spec file on disk, and every `*mutations*.json` under `tools/desk` is either declared here or would appear below.\n\n")
	} else {
		if len(p.Drift.DeclaredSpecsWithNoFile) > 0 {
			b.WriteString("**Declared here but no file on disk** (a control's spec vanished — investigate before trusting this release's coverage):\n\n")
			for _, s := range p.Drift.DeclaredSpecsWithNoFile {
				fmt.Fprintf(&b, "- `%s`\n", s)
			}
			b.WriteString("\n")
		}
		if len(p.Drift.SpecFilesNotDeclared) > 0 {
			b.WriteString("**On disk but not declared** (mutation specs this pack does not claim as release controls — exercised elsewhere, or a new gate awaiting declaration):\n\n")
			for _, s := range p.Drift.SpecFilesNotDeclared {
				fmt.Fprintf(&b, "- `%s`\n", s)
			}
			b.WriteString("\n")
		}
	}

	// Per-gate instrument summary (the four-column instrument-rule view).
	b.WriteString("## Instrument view\n\n")
	b.WriteString("One row per gate (`docs/three-state-instrument-rule.md`): the instrument, the literal string it prints when it cannot see, how many states it reports, and this release's disposition.\n\n")
	b.WriteString("| Instrument | What it prints when it cannot see | States | Disposition |\n")
	b.WriteString("|------------|-----------------------------------|--------|-------------|\n")
	for _, g := range p.Gates {
		fmt.Fprintf(&b, "| `%s` | `%s` | %d | %s |\n",
			g.Instrument.Instrument, g.Instrument.PrintsWhenBlind, g.Instrument.States, mdCell(g.Instrument.Disposition))
	}
	b.WriteString("\n")

	// Per-gate mutation demonstrations.
	b.WriteString("## Demonstrations\n\n")
	for _, g := range p.Gates {
		fmt.Fprintf(&b, "### %s\n\n", g.Gate)
		fmt.Fprintf(&b, "- Spec: `%s`\n", g.Spec)
		fmt.Fprintf(&b, "- Why it is a release control: %s\n", g.Rationale)
		if g.PositiveControl != "" {
			fmt.Fprintf(&b, "- Harness self-check (positive control the run must catch): %s\n", mdCell(g.PositiveControl))
		}
		fmt.Fprintf(&b, "- Harness: **%s**", g.Harness)
		if g.HarnessNote != "" {
			fmt.Fprintf(&b, " — %s", mdCell(g.HarnessNote))
		}
		b.WriteString("\n\n")

		if len(g.Mutations) == 0 {
			b.WriteString("_No mutations enumerated (spec unavailable)._\n\n")
			continue
		}
		b.WriteString("| Injected error (verbatim from the spec) | Verdict | Date | Tag |\n")
		b.WriteString("|-----------------------------------------|---------|------|-----|\n")
		for _, m := range g.Mutations {
			fmt.Fprintf(&b, "| %s | %s | %s | %s |\n", mdCell(m.Name), m.Verdict, m.Date, m.Tag)
		}
		b.WriteString("\n")
	}

	return b.String()
}

// mdCell makes a string safe to drop into a Markdown table cell: pipes would
// otherwise split the cell, and newlines would break the row. The mutation
// names are muhar's verbatim (they may contain neither, but a future spec might).
func mdCell(s string) string {
	s = strings.ReplaceAll(s, "|", "\\|")
	s = strings.ReplaceAll(s, "\n", " ")
	return s
}
