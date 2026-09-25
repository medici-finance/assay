package main

// flowhtml.go — the self-contained page renderer shared by `deskinbox html` (the Flow
// section beside the decision cards) and `deskinbox flow --html` (the Flow section alone),
// ported line-for-line from the oracle's write_html_program() JQHTML heredoc
// (assay-inbox.sh:594-855): the page skeleton/CSS, the inline-SVG stage diagram
// (stagebox/arrow/flowrow/flowsvg, assay-inbox.sh:608-683), the equivalent `<table>`
// (flowtable, assay-inbox.sh:685-724) and the section wrapper (flowsection,
// assay-inbox.sh:726-748).
//
// SELF-CONTAINED, BY CONSTRUCTION. Every colour is a CSS custom property in this page's own
// `<style>`; the only `href`s anywhere on the page are the issue links html.go's cards carry.
// No `<script>`, no ` src=`, no `url(` — asserted by Verify row 4 (self-contained-page test)
// and, for the diagram specifically, by the arrowhead being drawn as an explicit `<path>`
// rather than a `<marker>` reference (a marker reference is spelled `url(#id)`).
//
// AN SVG A SCREEN READER CANNOT READ IS HALF THE PAGE. The `<table>` that follows the
// diagram carries the SAME numbers, not a summary of them — role="img" plus a <title>/<desc>
// pair only names the picture.

import (
	"strconv"
	"strings"
)

// htmlEsc is the oracle's `esc` def (`tostring | @html`): jq 1.8's own escape table —
// &, <, >, ' and " map to &amp;, &lt;, &gt;, &apos; and &quot; — NOT Go's html.EscapeString,
// which spells the quote differently (&#34;) and would silently drift from the oracle's own
// output.
func htmlEsc(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch r {
		case '&':
			b.WriteString("&amp;")
		case '<':
			b.WriteString("&lt;")
		case '>':
			b.WriteString("&gt;")
		case '\'':
			b.WriteString("&apos;")
		case '"':
			b.WriteString("&quot;")
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func htmlEscInt(n int) string { return htmlEsc(strconv.Itoa(n)) }

// ------------------------------------------------------------------------ geometry --
// BOXW/GAP/PITCH/BOXH/GUTTER/ROWH are the oracle's own def constants (assay-inbox.sh:608).

const (
	flowBoxW   = 116
	flowGap    = 36
	flowPitch  = 152
	flowBoxH   = 66
	flowGutter = 132
	flowRowH   = 96
)

// flowSVGWidth is the oracle's `svgw`.
const flowSVGWidth = flowGutter + 7*flowBoxW + 6*flowGap + 12

// numOrNA / ratOrNA are the oracle's num($v)/rat($v) defs used inside the SVG/table
// builders — n/a for an absent figure, else the plain (rat: 2-decimal-rounded) value.
func numOrNA(v *int) string {
	if v == nil {
		return "n/a"
	}
	return strconv.Itoa(*v)
}

func ratOrNA(v *float64) string {
	return rOf(v)
}

// strokeWidth is the oracle's stroke($w) (assay-inbox.sh:617-621): a weight that could not
// be read draws thin and dashed (never a measurement); otherwise clamp the magnitude to 24
// and scale into [1.4, 6.0].
func strokeWidth(w *int) float64 {
	if w == nil {
		return 1.2
	}
	m := float64(*w)
	if m < 0 {
		m = -m
	}
	if m > 24 {
		m = 24
	}
	return 1.4 + (m/24)*4.6
}

// stageboxLines is the oracle's stagebox($s;$k;$top) (assay-inbox.sh:623-646).
func stageboxLines(s flowStage, k, top int) []string {
	x := flowGutter + k*flowPitch
	cx := x + flowBoxW/2
	cls := "box"
	switch {
	case s.IsBottleneck:
		cls = "box bneck"
	case s.Count == nil && s.NA != "":
		cls = "box na"
	case s.Count == nil:
		cls = "box blind"
	}

	lines := []string{
		"<g>",
		"<rect class=\"" + cls + "\" x=\"" + strconv.Itoa(x) + "\" y=\"" + strconv.Itoa(top) +
			"\" width=\"" + strconv.Itoa(flowBoxW) + "\" height=\"" + strconv.Itoa(flowBoxH) + "\" rx=\"7\"/>",
		"<text class=\"st\" x=\"" + strconv.Itoa(cx) + "\" y=\"" + strconv.Itoa(top+17) + "\">" + htmlEsc(s.Stage) + "</text>",
	}

	switch {
	case s.Count == nil && s.NA != "":
		lines = append(lines, "<text class=\"nat\" x=\""+strconv.Itoa(cx)+"\" y=\""+strconv.Itoa(top+41)+"\">n/a here</text>")
	case s.Count == nil:
		lines = append(lines, "<text class=\"cnc\" x=\""+strconv.Itoa(cx)+"\" y=\""+strconv.Itoa(top+41)+"\">could-not-check</text>")
	default:
		partial := ""
		if s.CountPartial {
			partial = "+"
		}
		lines = append(lines, "<text class=\"ct\" x=\""+strconv.Itoa(cx)+"\" y=\""+strconv.Itoa(top+44)+"\">"+
			htmlEscInt(*s.Count)+partial+"</text>")
	}

	var sub string
	switch {
	case s.Slots == nil:
		sub = "no pool"
	case s.QueueBlind != "":
		sub = "? of " + numOrNA(s.Slots) + " slots"
	default:
		sub = numOrNA(s.Queue) + "/" + numOrNA(s.Slots) + " · r " + ratOrNA(s.Ratio)
	}
	lines = append(lines, "<text class=\"sub\" x=\""+strconv.Itoa(cx)+"\" y=\""+strconv.Itoa(top+58)+"\">"+htmlEsc(sub)+"</text>")

	if s.IsBottleneck {
		lines = append(lines, "<text class=\"tag\" x=\""+strconv.Itoa(cx)+"\" y=\""+strconv.Itoa(top-4)+"\">bottleneck</text>")
	}
	lines = append(lines, "</g>")
	return lines
}

// arrowLines is the oracle's arrow($a;$k;$top) (assay-inbox.sh:653-663). The arrowhead is an
// explicit <path>, never a <marker> reference (which would be spelled url(#id) — forbidden
// on a self-contained page, see file header).
func arrowLines(a flowArrow, k, top int) []string {
	x1 := flowGutter + k*flowPitch + flowBoxW
	x2 := flowGutter + (k+1)*flowPitch
	y := top + flowBoxH/2

	cls := "arr"
	if a.Weight == nil {
		cls = "arr dash"
	}
	lines := []string{
		"<line class=\"" + cls + "\" x1=\"" + strconv.Itoa(x1) + "\" y1=\"" + strconv.Itoa(y) +
			"\" x2=\"" + strconv.Itoa(x2-8) + "\" y2=\"" + strconv.Itoa(y) +
			"\" stroke-width=\"" + jqNum(strokeWidth(a.Weight)) + "\"/>",
		"<path class=\"ahp\" d=\"M " + strconv.Itoa(x2-9) + " " + jqNum(float64(y)-4.5) +
			" L " + strconv.Itoa(x2) + " " + strconv.Itoa(y) +
			" L " + strconv.Itoa(x2-9) + " " + jqNum(float64(y)+4.5) + " z\"/>",
	}
	if a.Weight != nil {
		lines = append(lines, "<text class=\"wt\" x=\""+jqNum(float64(x1+x2)/2)+"\" y=\""+strconv.Itoa(y-6)+"\">"+
			htmlEscInt(*a.Weight)+"</text>")
	}
	return lines
}

// flowRowLines is the oracle's flowrow($row;$i) (assay-inbox.sh:665-670).
func flowRowLines(row flowRow, i int) []string {
	top := 12 + i*flowRowH + 18
	name := row.Cell
	if row.Fleet {
		name = "fleet"
	}
	lines := []string{
		"<text class=\"cell\" x=\"8\" y=\"" + strconv.Itoa(top+flowBoxH/2) + "\">" + htmlEsc(name) + "</text>",
	}
	for k, s := range row.Stages {
		lines = append(lines, stageboxLines(s, k, top)...)
	}
	for k, a := range row.Arrows {
		lines = append(lines, arrowLines(a, k, top)...)
	}
	return lines
}

// flowSVGLines is the oracle's flowsvg($f) (assay-inbox.sh:672-683).
func flowSVGLines(m flowModel) []string {
	n := len(m.Rows)
	h := 12 + n*flowRowH + 14
	lines := []string{
		"<svg class=\"flow\" viewBox=\"0 0 " + strconv.Itoa(flowSVGWidth) + " " + strconv.Itoa(h) +
			"\" role=\"img\" aria-labelledby=\"flowt flowd\" preserveAspectRatio=\"xMinYMin meet\">",
		"<title id=\"flowt\">Pipeline flow by stage</title>",
		"<desc id=\"flowd\">Seven stages left to right — intake, todo, in-progress, review, " +
			"implemented, verified, done — each box carrying the count now, the owning loop's " +
			"queue against its pool slots, and the ratio. The same figures follow in the table " +
			"below this diagram.</desc>",
	}
	for i, row := range m.Rows {
		lines = append(lines, flowRowLines(row, i)...)
	}
	lines = append(lines, "</svg>")
	return lines
}

// flowTableLines is the oracle's flowtable($f) (assay-inbox.sh:685-724): the equivalent
// <table> that follows the SVG with the same numbers.
func flowTableLines(m flowModel) []string {
	var lines []string
	for _, row := range m.Rows {
		heading := "cell " + htmlEsc(row.Cell)
		if row.Fleet {
			heading = "fleet"
		}
		if row.SHA != "" {
			heading += " <span class=\"sha\">board " + htmlEsc(row.SHA) + "</span>"
		}
		lines = append(lines, "<h3>"+heading+"</h3>")
		lines = append(lines, "<div class=\"scroll\"><table>")
		scope := "for this cell"
		if row.Fleet {
			scope = "across every cell read"
		}
		lines = append(lines, "<caption>Stage counts "+scope+" — the same figures the diagram draws.</caption>")
		lines = append(lines, "<thead><tr><th scope=\"col\">Stage</th><th scope=\"col\">What it holds</th>"+
			"<th scope=\"col\">Count</th><th scope=\"col\">Queue</th><th scope=\"col\">Slots</th>"+
			"<th scope=\"col\">Ratio</th><th scope=\"col\">Dwell</th><th scope=\"col\">Source</th></tr></thead>")
		lines = append(lines, "<tbody>")
		for _, s := range row.Stages {
			trCls := ""
			if s.IsBottleneck {
				trCls = " class=\"bn\""
			}
			th := htmlEsc(s.Stage)
			if s.IsBottleneck {
				th += " <span class=\"rec\">bottleneck</span>"
			}
			var countCell string
			switch {
			case s.Count == nil && s.NA != "":
				countCell = "<span class=\"nax\">n/a</span>"
			case s.Count == nil:
				countCell = "<span class=\"cncx\">could-not-check</span>"
			default:
				countCell = htmlEscInt(*s.Count)
				if s.CountPartial {
					countCell += " <abbr title=\"at least: one or more cells were unread\">+</abbr>"
				}
			}
			var queueCell string
			if s.QueueBlind != "" {
				queueCell = "<span class=\"cncx\" title=\"" + htmlEsc(s.QueueBlind) + "\">could-not-check</span>"
			} else {
				queueCell = htmlEsc(numOrNA(s.Queue))
			}
			dwellCell := s.Dwell
			if dwellCell == "" {
				dwellCell = "—"
			}
			var srcCell string
			switch {
			case s.Count == nil && s.NA != "":
				srcCell = s.NA
			case s.Count == nil:
				srcCell = s.Blind
			case s.CountSource == "":
				srcCell = s.CapacityNote
			default:
				srcCell = s.CountSource
			}
			lines = append(lines, "<tr"+trCls+"><th scope=\"row\">"+th+"</th>"+
				"<td>"+htmlEsc(s.Label)+"</td>"+
				"<td class=\"n\">"+countCell+"</td>"+
				"<td class=\"n\">"+queueCell+"</td>"+
				"<td class=\"n\">"+htmlEsc(numOrNA(s.Slots))+"</td>"+
				"<td class=\"n\">"+htmlEsc(ratOrNA(s.Ratio))+"</td>"+
				"<td class=\"n\">"+htmlEsc(dwellCell)+"</td>"+
				"<td class=\"src\">"+htmlEsc(srcCell)+"</td></tr>")
		}
		lines = append(lines, "</tbody></table></div>")
		if row.CapacityNote != "" {
			lines = append(lines, "<p class=\"flowmeta\">"+htmlEsc(row.CapacityNote)+"</p>")
		}
		flowSuffix := ""
		if row.FlowBlind != "" {
			flowSuffix = " — <span class=\"cncx\">" + htmlEsc(row.FlowBlind) + "</span>"
		}
		interior := ""
		for _, a := range row.Arrows {
			if a.From == "todo" {
				interior = a.Blind
				break
			}
		}
		lines = append(lines, "<p class=\"flowmeta\">Flow in the window: arrivals <b>"+htmlEsc(numOrNA(row.Arrivals))+
			"</b> into the pipeline, completions <b>"+htmlEsc(numOrNA(row.Completions))+"</b> out of it"+
			flowSuffix+". Interior steps: <span class=\"cncx\">"+htmlEsc(interior)+"</span></p>")
		constraint := row.Constraint
		if constraint == "" {
			constraint = "n/a"
		}
		lines = append(lines, "<p class=\"flowmeta\">Dwell-weighted constraint (Theory of Constraints, WIP × dwell): <b>"+
			htmlEsc(constraint)+"</b> — "+htmlEsc(row.ConstraintSrc)+"</p>")
	}
	return lines
}

// flowSectionLines is the oracle's flowsection($f) (assay-inbox.sh:726-748) — the whole Flow
// section, embedded on the decision page (html.go) or standing alone (buildFlowOnlyPage
// below).
func flowSectionLines(m flowModel) []string {
	lines := []string{
		"<section class=\"flow-sec\">",
		"<h2>Flow — how the system is performing</h2>",
		"<p class=\"meta\">asOf " + htmlEsc(m.AsOf) + " · window " + htmlEsc(m.Since) +
			" · read " + htmlEsc(strconv.Itoa(m.StagesRead)) + " of " + htmlEsc(strconv.Itoa(m.StagesTotal)) +
			" capacity stages · " + htmlEsc(strconv.Itoa(m.CellCount)) + " cell(s)</p>",
	}
	lines = append(lines, flowSVGLines(m)...)
	lines = append(lines,
		"<p class=\"legend\">Arrow thickness is the flow measured for the window; a dashed arrow "+
			"is a step no reader measures. A box outlined in the alert colour is a stage whose "+
			"reader failed — could-not-check, never zero. A faintly dotted box is a figure that "+
			"does not exist at that granularity, which is not the same thing.</p>")
	bneck := m.Bottleneck
	if bneck == "" {
		bneck = "no bottleneck named"
	}
	lines = append(lines, "<p class=\"advice\"><b>"+htmlEsc(bneck)+"</b> — "+htmlEsc(m.Advice)+"</p>")
	lines = append(lines, flowTableLines(m)...)
	lines = append(lines, "<h3>Where these numbers come from</h3>", "<ul>")
	for _, s := range m.Sources {
		lines = append(lines, "<li>"+htmlEsc(s)+"</li>")
	}
	lines = append(lines, "</ul>")
	if len(m.Blind) > 0 {
		lines = append(lines, "<h3>Could not check</h3>", "<ul class=\"cnclist\">")
		for _, b := range m.Blind {
			lines = append(lines, "<li>"+htmlEsc(b)+"</li>")
		}
		lines = append(lines, "</ul>")
	}
	lines = append(lines, "<p class=\"honest\">"+htmlEsc(m.Note)+"</p>", "</section>")
	return lines
}

// -------------------------------------------------------------------------- the page --
// pageHead/pageStyle are the oracle's own <head>/<style> block (assay-inbox.sh:755-818),
// shared byte-for-byte by the decision page (html.go) and the flow-only page below — a
// second copy here is exactly the drift the reviewer's question 2 asks about.

const pageStyleCSS = `:root{color-scheme:light dark;--bg:#fbfaf7;--fg:#1b1a17;--card:#fff;--line:#e3e0d8;--muted:#6b6860;--accent:#8a5a2b;--recfg:#12603a;--recbg:#e3f3e9;--bnfg:#8c3b12;--bnbg:#fbe9de;--cnc:#8c3b12;--grid:#cfcbc1}
@media (prefers-color-scheme:dark){:root{--bg:#131310;--fg:#eae7e0;--card:#1d1d18;--line:#34332c;--muted:#a09c92;--accent:#d9a066;--recfg:#8ad9ac;--recbg:#1b3327;--bnfg:#f0a97a;--bnbg:#3a2317;--cnc:#f0a97a;--grid:#4a483f}}
*{box-sizing:border-box}
body{margin:0;padding:2rem 1rem;background:var(--bg);color:var(--fg);font:16px/1.55 ui-sans-serif,system-ui,-apple-system,"Segoe UI",Roboto,Helvetica,Arial,sans-serif}
main{max-width:54rem;margin:0 auto}
h1{font-size:1.5rem;margin:0 0 .25rem}
p.meta{margin:0 0 2rem;color:var(--muted);font-size:.85rem}
article.card{background:var(--card);border:1px solid var(--line);border-radius:10px;padding:1.1rem 1.25rem;margin:0 0 1.25rem}
article.card h2{font-size:1.05rem;margin:0 0 .1rem;font-weight:650}
article.card h2 a{color:var(--accent);text-decoration:none}
article.card h2 a:hover{text-decoration:underline}
p.title{margin:0 0 .9rem;color:var(--muted);font-size:.9rem}
h3{font-size:.72rem;letter-spacing:.09em;text-transform:uppercase;color:var(--muted);margin:1rem 0 .35rem;font-weight:700}
ul{margin:0;padding-left:1.15rem}
ul li{margin:.15rem 0}
ul.opts{list-style:none;padding-left:0}
ul.opts li{margin:.3rem 0}
span.rec{color:var(--recfg);background:var(--recbg);border-radius:999px;padding:.05rem .5rem;font-size:.72rem;font-weight:700;white-space:nowrap}
p.unread{color:var(--accent);font-weight:600}
p.empty{color:var(--muted)}
section.flow-sec{background:var(--card);border:1px solid var(--line);border-radius:10px;padding:1.1rem 1.25rem;margin:2rem 0 1.25rem}
section.flow-sec h2{font-size:1.15rem;margin:0 0 .2rem}
svg.flow{display:block;width:100%;height:auto;margin:.9rem 0 .3rem;overflow:visible}
svg.flow .box{fill:var(--bg);stroke:var(--line);stroke-width:1.2}
svg.flow .box.bneck{stroke:var(--bnfg);stroke-width:2.4;fill:var(--bnbg)}
svg.flow .box.blind{stroke:var(--cnc);stroke-dasharray:5 4;fill:none}
svg.flow .box.na{stroke:var(--line);stroke-dasharray:2 4;fill:none}
svg.flow text{font:11px ui-sans-serif,system-ui,sans-serif;fill:var(--fg)}
svg.flow .st{font-size:10.5px;fill:var(--muted);text-anchor:middle;letter-spacing:.03em}
svg.flow .ct{font-size:21px;font-weight:700;text-anchor:middle}
svg.flow .cnc{font-size:9.5px;fill:var(--cnc);text-anchor:middle;font-weight:700}
svg.flow .nat{font-size:9.5px;fill:var(--muted);text-anchor:middle}
svg.flow .sub{font-size:9.5px;fill:var(--muted);text-anchor:middle}
svg.flow .tag{font-size:9px;fill:var(--bnfg);text-anchor:middle;font-weight:700;letter-spacing:.06em}
svg.flow .cell{font-size:11px;fill:var(--muted);font-weight:600}
svg.flow .arr{stroke:var(--grid);fill:none}
svg.flow .arr.dash{stroke-dasharray:4 4}
svg.flow .ahp{fill:var(--grid);stroke:none}
svg.flow .wt{font-size:9px;fill:var(--muted);text-anchor:middle}
div.scroll{overflow-x:auto;margin:.5rem 0 .2rem}
table{border-collapse:collapse;width:100%;font-size:.82rem}
caption{caption-side:top;text-align:left;color:var(--muted);font-size:.76rem;padding:.2rem 0 .4rem}
th,td{border-bottom:1px solid var(--line);padding:.3rem .5rem;text-align:left;vertical-align:top}
thead th{color:var(--muted);font-size:.72rem;text-transform:uppercase;letter-spacing:.06em}
td.n,th.n{text-align:right;font-variant-numeric:tabular-nums;white-space:nowrap}
td.src{color:var(--muted);font-size:.74rem}
tr.bn th,tr.bn td{background:var(--bnbg)}
tr.bn span.rec{color:var(--bnfg);background:transparent;padding:0}
span.cncx{color:var(--cnc);font-weight:600}
span.nax{color:var(--muted)}
span.sha{color:var(--muted);font-weight:400;font-size:.78rem}
p.legend,p.flowmeta{color:var(--muted);font-size:.78rem;margin:.35rem 0}
p.advice{margin:.5rem 0 .2rem;font-size:.88rem}
p.honest{color:var(--muted);font-size:.78rem;font-style:italic;margin:.9rem 0 0;border-top:1px solid var(--line);padding-top:.7rem}
ul.cnclist li{color:var(--cnc)}`

// pageHeadLines is shared by the decision page (html.go's buildDecisionPage) and the
// flow-only page (buildFlowOnlyPage): the doctype/head/style block, differing only in
// <title> (the oracle's `if $flowonly then ... else ... end`, assay-inbox.sh:760).
func pageHeadLines(flowOnly bool) []string {
	title := "Assay inbox — decisions waiting"
	if flowOnly {
		title = "Assay inbox — pipeline flow"
	}
	return []string{
		"<!doctype html>",
		"<html lang=\"en\">",
		"<head>",
		"<meta charset=\"utf-8\">",
		"<meta name=\"viewport\" content=\"width=device-width, initial-scale=1\">",
		"<title>" + title + "</title>",
		"<style>",
		pageStyleCSS,
		"</style>",
		"</head>",
	}
}

// buildFlowOnlyPage is `deskinbox flow --html OUT.html`'s renderer — the oracle's
// `flowhtml` MODE case (assay-inbox.sh:1391-1401): the Flow section alone.
func buildFlowOnlyPage(m flowModel) string {
	var lines []string
	lines = append(lines, pageHeadLines(true)...)
	lines = append(lines, "<body>", "<main>", "<h1>Pipeline flow</h1>")
	lines = append(lines, flowSectionLines(m)...)
	lines = append(lines, "</main>", "</body>", "</html>")
	return strings.Join(lines, "\n") + "\n"
}
