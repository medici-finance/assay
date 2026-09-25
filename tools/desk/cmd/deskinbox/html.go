package main

// html.go — `deskinbox html OUT.html [repo ...]`, ported from the oracle's `--html` case
// (assay-inbox.sh:1356-1383): the whole decision queue as self-contained HTML cards in the
// SAME five-part format walk.go renders (buildRendered, format.go — reused here exactly as
// the oracle's own write_format_program is shared between --walk and --html, per
// windows-port/13's testdata/spec.md), PLUS the Flow section (flowhtml.go), built over the
// current directory ('.') as a single cell.
//
// WHY THE FLOW SECTION READS '.', NOT THE REPO LIST. The decision page's repo list is a repo
// axis (which forges to query for issues); the Flow section's cell list is a statusgen-root
// axis. Inventing a cell list from the repo args would claim a coverage no reader was given
// (assay-inbox.sh:1370-1372) — `deskinbox flow --root ...` is the multi-cell form.

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// cardData is one rendered item's card input: the source item (repo/number/url/title/index/
// total) plus its five-part rendering.
type cardData struct {
	Item     item
	Index    int // 1-based
	Total    int
	Rendered rendered
}

// cardLines is the oracle's per-item `<article class="card">` block (assay-inbox.sh:826-848).
func cardLines(c cardData) []string {
	lines := []string{
		"<article class=\"card\">",
		"<h2><a href=\"" + htmlEsc(c.Item.URL) + "\">" + htmlEsc(c.Item.Repo) + "#" + htmlEscInt(c.Item.Number) +
			"</a> — question " + htmlEscInt(c.Index) + " of " + htmlEscInt(c.Total) + "</h2>",
		"<p class=\"title\">" + htmlEsc(demd(cleanText(c.Item.Title))) + "</p>",
	}
	if c.Rendered.Unread {
		lines = append(lines, "<p class=\"unread\">could-not-check: this item was not read</p>")
	}
	lines = append(lines, "<h3>Context</h3>", "<ul>")
	for _, ctx := range c.Rendered.Context {
		lines = append(lines, "<li>"+htmlEsc(ctx)+"</li>")
	}
	lines = append(lines, "</ul>", "<h3>Options</h3>", "<ul class=\"opts\">")
	for _, o := range c.Rendered.Options {
		rec := ""
		if o.Recommended {
			rec = " <span class=\"rec\">recommended</span>"
		}
		lines = append(lines, "<li><b>"+htmlEsc(o.Letter)+".</b> "+htmlEsc(o.Text)+rec+"</li>")
	}
	lines = append(lines, "</ul>", "<h3>Reply shape</h3>", "<p>"+htmlEsc(c.Rendered.Reply)+"</p>",
		"<h3>Verification</h3>", "<p>"+htmlEsc(c.Rendered.Verification)+"</p>", "</article>")
	return lines
}

// buildDecisionPage is the oracle's `html` MODE case's page assembly (assay-inbox.sh:756-853
// with $flowonly=false): the doctype/head/style, the "Decisions waiting" header, one card per
// queued item (or the empty-queue paragraph), then the Flow section.
func buildDecisionPage(summary string, cards []cardData, flow flowModel) string {
	var lines []string
	lines = append(lines, pageHeadLines(false)...)
	lines = append(lines, "<body>", "<main>", "<h1>Decisions waiting on the driver</h1>",
		"<p class=\"meta\">"+htmlEsc(summary)+"</p>")
	if len(cards) == 0 {
		lines = append(lines, "<p class=\"empty\">Nothing is waiting on the driver right now.</p>")
	}
	for _, c := range cards {
		lines = append(lines, cardLines(c)...)
	}
	lines = append(lines, flowSectionLines(flow)...)
	lines = append(lines, "</main>", "</body>", "</html>")
	return strings.Join(lines, "\n") + "\n"
}

// htmlSummary is finish()'s CLI-stdout summary for the html MODE (assay-inbox.sh:1290,
// 1296-1297): the ordinary item-count summary, PLUS a note when the Flow section is
// incomplete — which never reddens this mode's own exit code (the decision queue's exit
// code is a statement about the decisions, not the sidebar).
func htmlSummary(itemCount, repoCount int, failures []queryFailure, flowFailures int) string {
	s := summaryText(itemCount, repoCount, failures)
	if flowFailures > 0 {
		s += fmt.Sprintf("; the Flow section is INCOMPLETE (%d reader(s) could not be read — see stderr)", flowFailures)
	}
	return s
}

// pageSummaryText is the oracle's summary_text() (assay-inbox.sh:1309-1315) — the string
// embedded on the PAGE's own `<p class="meta">`, recomputed rather than reusing the CLI
// summary so an incomplete run says so ON THE PAGE, not only on the terminal. Its wording is
// deliberately its OWN (shorter "FAILED — INCOMPLETE", "the Flow section BELOW is
// INCOMPLETE") rather than finish()'s CLI phrasing — two call sites in the oracle that were
// never merged into one string, so this port keeps them apart too.
func pageSummaryText(itemCount, repoCount int, failures []queryFailure, flowFailures int) string {
	s := fmt.Sprintf("deskinbox: %d item(s) across %d repo(s)", itemCount, repoCount)
	if len(failures) > 0 {
		s += fmt.Sprintf("; %d query(s) FAILED — INCOMPLETE", len(failures))
	}
	if flowFailures > 0 {
		s += fmt.Sprintf("; the Flow section below is INCOMPLETE (%d reader(s) could not be read)", flowFailures)
	}
	return s
}

// runHTML implements `deskinbox html OUT.html [repo ...]`.
func runHTML(stdout, stderr io.Writer, outPath string, repos []string, now time.Time) int {
	if err := validateRepos(repos); err != nil {
		fmt.Fprintln(stderr, err)
		return deskkit.ExitCodeOf(err)
	}
	items, failures := fetchQueue(repos)
	for _, f := range failures {
		fmt.Fprintf(stderr, "deskinbox: QUERY FAILED for %s: %v\n", f.Repo, f.Err)
	}

	cards := make([]cardData, 0, len(items))
	for i, it := range items {
		fr := deskkit.ForgeRepo{}
		if owner, name, ok := splitRepo(it.Repo); ok {
			fr = deskkit.ForgeRepo{Owner: owner, Name: name}
		}
		d := fetchDetail(fr, it.Number)
		r := buildRendered(it, d, i, len(items), now)
		cards = append(cards, cardData{Item: it, Index: i + 1, Total: len(items), Rendered: r})
	}

	// The Flow section is built over the current directory as a single cell, always — see
	// the file header for why this is a repo axis vs. cell axis distinction, not an
	// oversight.
	cwd, cerr := os.Getwd()
	if cerr != nil {
		fmt.Fprintln(stderr, deskkit.Unverifiable("cannot resolve the current directory", cerr))
		return deskkit.ExitUnverifiable
	}
	sgBin, sgErr := resolveFlowBin(statusgenBinEnv, "statusgen")
	if sgErr != nil {
		fmt.Fprintln(stderr, sgErr)
		return deskkit.ExitUnverifiable
	}
	dbBin, dbErr := resolveFlowBin(deskboardBinEnv, "deskboard")
	if dbErr != nil {
		fmt.Fprintln(stderr, dbErr)
		return deskkit.ExitUnverifiable
	}
	cell := cellSpec{Name: cellNameFor(cwd), Path: "."}
	raw := collectFlow([]cellSpec{cell}, "", sgBin, dbBin, now.UTC().Format("2006-01-02T15:04:05Z"))
	flow := interpretFlow(raw)
	flowFailures := countFlowFailures(raw)

	// summary_text() (assay-inbox.sh:1309-1315) is recomputed with the Flow section's own
	// failure count AFTER build_flow runs, so an incomplete Flow section says so ON THE PAGE
	// too, not only on the terminal.
	page := buildDecisionPage(pageSummaryText(len(items), len(repos), failures, flowFailures), cards, flow)
	if err := os.WriteFile(outPath, []byte(page), 0o644); err != nil {
		fmt.Fprintf(stderr, "deskinbox: failed to write %s: %v\n", outPath, err)
		return deskkit.ExitRefused
	}
	fmt.Fprintf(stdout, "deskinbox: wrote %d card(s) to %s\n", len(items), outPath)
	fmt.Fprintln(stdout, htmlSummary(len(items), len(repos), failures, flowFailures))
	if len(failures) > 0 {
		return deskkit.ExitUnverifiable
	}
	return deskkit.ExitOK
}
