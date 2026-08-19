// Package chart renders the Ask Assay charts (design §7.5 and §7.6) from
// [askassay.Answer] values and from NOTHING ELSE.
//
// WHAT THIS PACKAGE IS FOR
// ------------------------
// The numbers rule was made structural for a single figure: an
// answer holds its value in an unexported field, could-not-check cannot carry
// one, and Render reads the state rather than the field. A chart is the same
// figure repeated across a domain, and it is where that guarantee is most
// easily thrown away — because the natural chart primitive for "no value" is
// a bar of height zero, and a bar of height zero is a drawn assertion that
// the answer is 0.
//
// THE COULD-NOT-CHECK RENDER, STATED
// ----------------------------------
// A point whose answer is [askassay.CouldNotCheck] is drawn as:
//
//   - a HATCHED BAND spanning the full plot height in the neutral
//     [UnknownInk] — not a short bar, not a zero-height bar, not a gap;
//   - carrying the literal word "could-not-check" and the "?" glyph, so the
//     signal survives a greyscale print and a colour-blind reader (§7.6: no
//     colour-only encoding);
//   - EXCLUDED from the value domain, so an unmeasurable point cannot
//     compress or stretch the axis the measured points are read against;
//   - with its reason in the hover tooltip and in the table view.
//
// The trade-off is stated rather than hidden: a full-height band has a top
// edge at the plot ceiling, and a reader who ignores both the hatch and the
// word could misread it as a maximum. That is the lesser error. A gap and a
// zero-height bar are the SAME PIXELS as a measured zero, and the eye reads
// both as "none" with no cue at all that a measurement is missing; a
// full-height hatch is not a magnitude any reader can take a number off, and
// it is the only treatment here that is impossible to confuse with the value
// 0. A genuinely measured 0 still draws a zero-height bar with the numeral
// "0" printed above it — the two states are visually distinct by
// construction, which [TestZeroAndCouldNotCheckDrawDifferently] asserts.
//
// If NO point in a series carries a value, the chart does not render an empty
// axis. An empty axis with a baseline is a drawn zero line. It renders a
// could-not-check panel instead.
//
// DETERMINISM
// -----------
// Render is a pure function of its input: no clock, no map iteration, no
// randomness, no locale. Two calls on the same [Chart] return identical
// bytes, which [TestRenderIsByteIdenticalAcrossCalls] asserts. The as-of
// stamp is an input field, never time.Now().
package chart

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/askassay"
)

// CouldNotCheckToken is the literal string drawn in the plot, printed in the
// table view, and asserted on by the tests. Downstream consumers key on this
// constant rather than pattern-matching prose.
const CouldNotCheckToken = string(askassay.CouldNotCheck)

// Point is one slot on the category axis. Its value — if it has one — comes
// from the answer and only from the answer; there is no exported numeric
// field on this type, so a caller cannot supply a figure the numbers rule did
// not produce.
type Point struct {
	// Label is the category-axis label for this slot.
	Label string
	// Answer is the ONE source of this slot's value and state.
	Answer askassay.Answer
}

// Series is one chart's data. A chart draws exactly one series: the lifecycle
// progression is a single-hue ramp across the series' own slots (§7.6), not
// five identities overlaid.
type Series struct {
	// Name titles the value axis.
	Name string
	// Unit is the value axis unit word ("PRs", "days"). Mandatory: an
	// unlabelled axis is a figure with no units, which is not a figure.
	Unit string
	// Points are the slots, in the order they are drawn. Order is the
	// caller's; this package never sorts, because a progression's order is
	// meaning.
	Points []Point
}

// GateMarker is the ONLY thing on a chart that may be gold. It marks a human
// gate (§7.6: gold is a reserved semantic, never a series colour, always with
// its glyph and label).
type GateMarker struct {
	// AtLabel is the point label the marker sits over.
	AtLabel string
	// Label is the word that ships with the glyph, e.g. "decide".
	Label string
}

// Chart is a renderable bar chart over one series.
type Chart struct {
	// Title is the chart's heading. Mandatory.
	Title string
	// Series is the data. Mandatory, non-empty.
	Series Series
	// AsOf is the stamp every figure on this chart was computed at. It is an
	// INPUT, not a clock read, so that the render is deterministic and so
	// that a chart cannot claim to be fresher than its data.
	AsOf askassay.Stamp
	// Gate optionally marks one slot as a human gate.
	Gate *GateMarker

	// Width and Height are the SVG viewBox extents. Zero means the defaults.
	Width, Height int
}

const (
	defaultWidth  = 720
	defaultHeight = 320
	padLeft       = 64
	padRight      = 24
	padTop        = 56
	padBottom     = 64
	slotGap       = 2 // §7.6: 2px surface gaps between segments
)

// ErrNoRender is returned when the chart cannot be drawn at all. It is a
// refusal, not a blank canvas: a blank chart is indistinguishable from a
// chart of zeroes.
var ErrNoRender = errors.New("chart refused")

func (c Chart) validate() error {
	if strings.TrimSpace(c.Title) == "" {
		return fmt.Errorf("%w: the chart declares no title", ErrNoRender)
	}
	if strings.TrimSpace(c.Series.Name) == "" {
		return fmt.Errorf("%w: the series declares no name, so the value axis is unlabelled", ErrNoRender)
	}
	if strings.TrimSpace(c.Series.Unit) == "" {
		return fmt.Errorf("%w: the series declares no unit, so every value on the axis is a bare number", ErrNoRender)
	}
	if len(c.Series.Points) == 0 {
		return fmt.Errorf("%w: the series has no points — an empty plot area draws a baseline, and a baseline is a zero line", ErrNoRender)
	}
	if c.AsOf.Zero() {
		return fmt.Errorf("%w: the chart carries no as-of stamp, and every count it plots moves", ErrNoRender)
	}
	for i, p := range c.Series.Points {
		if strings.TrimSpace(p.Label) == "" {
			return fmt.Errorf("%w: point %d has no label", ErrNoRender, i)
		}
	}
	if c.Gate != nil {
		if strings.TrimSpace(c.Gate.Label) == "" {
			return fmt.Errorf("%w: the gate marker carries no label — gold never ships without its word", ErrNoRender)
		}
		found := false
		for _, p := range c.Series.Points {
			if p.Label == c.Gate.AtLabel {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("%w: the gate marker names slot %q, which is not on this chart", ErrNoRender, c.Gate.AtLabel)
		}
	}
	return ValidatePalette(Ramp, GateGold)
}

// domain returns the axis maximum over the points that ACTUALLY CARRY A
// VALUE, and how many those are. Could-not-check points are excluded: an
// unmeasurable slot must not move the scale the measured slots are read
// against.
func (c Chart) domain() (max int64, measured int) {
	for _, p := range c.Series.Points {
		v, ok := p.Answer.Value()
		if !ok {
			continue
		}
		measured++
		if v > max {
			max = v
		}
	}
	return max, measured
}

// Render returns the chart as a self-contained SVG string.
//
// It reads each point's STATE, never its value field, exactly as
// [askassay.Answer.Render] does. There is no code path in this function that
// converts a could-not-check into a coordinate.
func (c Chart) Render() (string, error) {
	if err := c.validate(); err != nil {
		return "", err
	}
	w, h := c.Width, c.Height
	if w <= 0 {
		w = defaultWidth
	}
	if h <= 0 {
		h = defaultHeight
	}

	max, measured := c.domain()
	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %d %d" width="%d" height="%d" role="img" aria-label="%s">`, w, h, w, h, esc(c.Title))
	b.WriteString("\n")
	fmt.Fprintf(&b, `<rect x="0" y="0" width="%d" height="%d" fill="%s"/>`+"\n", w, h, Surface)
	b.WriteString(hatchDef())
	fmt.Fprintf(&b, `<text x="%d" y="26" fill="%s" font-family="system-ui,sans-serif" font-size="15" font-weight="600">%s</text>`+"\n",
		padLeft, InkText, esc(c.Title))
	fmt.Fprintf(&b, `<text x="%d" y="44" fill="%s" font-family="system-ui,sans-serif" font-size="%d">%s (%s) · as-of %s</text>`+"\n",
		padLeft, InkDim, MinTypePx, esc(c.Series.Name), esc(c.Series.Unit), esc(c.AsOf.String()))

	if measured == 0 {
		// No axis, no baseline, no slots. A plot frame with nothing in it is
		// a drawn set of zeroes.
		b.WriteString(c.unmeasurablePanel(w, h))
		b.WriteString("</svg>\n")
		return b.String(), nil
	}

	plotX, plotY := padLeft, padTop
	plotW, plotH := w-padLeft-padRight, h-padTop-padBottom

	// Value axis: the maximum and zero, in text tokens (§7.6 — values wear
	// text tokens, never the series colour). The axis is drawn ONLY on this
	// branch, i.e. only when at least one slot carries a measured value: a
	// baseline under an empty plot is a drawn row of zeroes.
	b.WriteString(`<g class="axis">` + "\n")
	fmt.Fprintf(&b, `<text x="%d" y="%d" fill="%s" font-family="system-ui,sans-serif" font-size="%d" text-anchor="end">%d</text>`+"\n",
		plotX-8, plotY+10, InkDim, MinTypePx, max)
	fmt.Fprintf(&b, `<text x="%d" y="%d" fill="%s" font-family="system-ui,sans-serif" font-size="%d" text-anchor="end">0</text>`+"\n",
		plotX-8, plotY+plotH, InkDim, MinTypePx)
	fmt.Fprintf(&b, `<line x1="%d" y1="%d" x2="%d" y2="%d" stroke="%s" stroke-width="1"/>`+"\n",
		plotX, plotY+plotH, plotX+plotW, plotY+plotH, InkDim)
	b.WriteString("</g>\n")

	n := len(c.Series.Points)
	slotW := (plotW - slotGap*(n-1)) / n
	if slotW < 4 {
		slotW = 4
	}
	for i, p := range c.Series.Points {
		x := plotX + i*(slotW+slotGap)
		b.WriteString(c.renderSlot(p, i, n, x, plotY, plotW, plotH, slotW, max))
	}

	if c.Gate != nil {
		b.WriteString(c.renderGate(plotX, plotY, plotH, slotW, n))
	}
	b.WriteString("</svg>\n")
	return b.String(), nil
}

// renderSlot draws one point. The switch is on the answer's STATE.
func (c Chart) renderSlot(p Point, i, n, x, plotY, plotW, plotH, slotW int, max int64) string {
	var b strings.Builder
	labelY := plotY + plotH + 18

	v, ok := p.Answer.Value()
	if !ok {
		// COULD-NOT-CHECK. Full-height hatch, neutral ink, the literal token,
		// the "?" glyph, and the reason in the tooltip. No height is derived
		// from any number, because there is no number.
		fmt.Fprintf(&b, `<g class="slot could-not-check">`+"\n")
		fmt.Fprintf(&b, `<title>%s — %s: %s</title>`+"\n",
			esc(p.Label), CouldNotCheckToken, esc(reasonOf(p.Answer)))
		fmt.Fprintf(&b, `<rect x="%d" y="%d" width="%d" height="%d" fill="url(#cnc-hatch)" stroke="%s" stroke-width="1" stroke-dasharray="3 3"/>`+"\n",
			x, plotY, slotW, plotH, UnknownInk)
		fmt.Fprintf(&b, `<text x="%d" y="%d" fill="%s" font-family="system-ui,sans-serif" font-size="%d" text-anchor="middle">?</text>`+"\n",
			x+slotW/2, plotY+plotH/2-6, InkText, MinTypePx+3)
		fmt.Fprintf(&b, `<text x="%d" y="%d" fill="%s" font-family="system-ui,sans-serif" font-size="%d" text-anchor="middle">%s</text>`+"\n",
			x+slotW/2, plotY+plotH/2+10, InkText, MinTypePx, CouldNotCheckToken)
		fmt.Fprintf(&b, `<text x="%d" y="%d" fill="%s" font-family="system-ui,sans-serif" font-size="%d" text-anchor="middle">%s</text>`+"\n",
			x+slotW/2, labelY, InkDim, MinTypePx, esc(p.Label))
		b.WriteString("</g>\n")
		return b.String()
	}

	// A measured value — including a measured 0, which draws a zero-height
	// bar with the numeral printed above it.
	barH := 0
	if max > 0 {
		barH = int(int64(plotH) * v / max)
	}
	y := plotY + plotH - barH
	fill := Ramp[rampIndex(i, n)]

	state := string(p.Answer.State())
	glyph := ""
	if p.Answer.State() == askassay.CheckedFailed {
		// §7.6: no colour-only encoding. A failed measurement is marked by a
		// glyph and a word, not by turning the bar red.
		glyph = "!"
	}
	fmt.Fprintf(&b, `<g class="slot %s">`+"\n", esc(state))
	fmt.Fprintf(&b, `<title>%s — %s %s · %s</title>`+"\n",
		esc(p.Label), formatInt(v), esc(c.Series.Unit), esc(sourceLine(p.Answer)))
	fmt.Fprintf(&b, `<rect x="%d" y="%d" width="%d" height="%d" fill="%s"/>`+"\n", x, y, slotW, barH, fill)
	// Direct-labelled count, in a TEXT token. Never the series colour.
	label := formatInt(v)
	if glyph != "" {
		label = glyph + " " + label + " failed"
	}
	fmt.Fprintf(&b, `<text x="%d" y="%d" fill="%s" font-family="system-ui,sans-serif" font-size="%d" text-anchor="middle">%s</text>`+"\n",
		x+slotW/2, y-6, InkText, MinTypePx+1, esc(label))
	fmt.Fprintf(&b, `<text x="%d" y="%d" fill="%s" font-family="system-ui,sans-serif" font-size="%d" text-anchor="middle">%s</text>`+"\n",
		x+slotW/2, labelY, InkDim, MinTypePx, esc(p.Label))
	b.WriteString("</g>\n")
	return b.String()
}

func (c Chart) renderGate(plotX, plotY, plotH, slotW, n int) string {
	idx := 0
	for i, p := range c.Series.Points {
		if p.Label == c.Gate.AtLabel {
			idx = i
			break
		}
	}
	x := plotX + idx*(slotW+slotGap) + slotW/2
	var b strings.Builder
	b.WriteString(`<g class="gate">` + "\n")
	fmt.Fprintf(&b, `<title>%s %s — human gate</title>`+"\n", GateGlyph, esc(c.Gate.Label))
	fmt.Fprintf(&b, `<line x1="%d" y1="%d" x2="%d" y2="%d" stroke="%s" stroke-width="2"/>`+"\n",
		x, plotY-6, x, plotY+plotH, GateGold)
	// Gold NEVER appears without its glyph and a word.
	fmt.Fprintf(&b, `<text x="%d" y="%d" fill="%s" font-family="system-ui,sans-serif" font-size="%d" text-anchor="middle">%s %s</text>`+"\n",
		x, plotY-12, InkText, MinTypePx, GateGlyph, esc(c.Gate.Label))
	b.WriteString("</g>\n")
	return b.String()
}

// unmeasurablePanel is what a series with no measurable point renders. It
// deliberately draws no axis and no baseline.
func (c Chart) unmeasurablePanel(w, h int) string {
	var b strings.Builder
	b.WriteString(`<g class="chart-could-not-check">` + "\n")
	fmt.Fprintf(&b, `<title>%s — %s for every slot</title>`+"\n", esc(c.Title), CouldNotCheckToken)
	fmt.Fprintf(&b, `<rect x="%d" y="%d" width="%d" height="%d" fill="url(#cnc-hatch)" stroke="%s" stroke-width="1" stroke-dasharray="3 3"/>`+"\n",
		padLeft, padTop, w-padLeft-padRight, h-padTop-padBottom, UnknownInk)
	fmt.Fprintf(&b, `<text x="%d" y="%d" fill="%s" font-family="system-ui,sans-serif" font-size="%d" text-anchor="middle">? %s — no slot on this chart carries a measured value</text>`+"\n",
		w/2, h/2, InkText, MinTypePx+1, CouldNotCheckToken)
	fmt.Fprintf(&b, `<text x="%d" y="%d" fill="%s" font-family="system-ui,sans-serif" font-size="%d" text-anchor="middle">%s</text>`+"\n",
		w/2, h/2+18, InkDim, MinTypePx, esc(firstReason(c.Series.Points)))
	b.WriteString("</g>\n")
	return b.String()
}

func hatchDef() string {
	return fmt.Sprintf(`<defs><pattern id="cnc-hatch" patternUnits="userSpaceOnUse" width="8" height="8" patternTransform="rotate(45)">`+
		`<rect width="8" height="8" fill="%s" fill-opacity="0.10"/>`+
		`<line x1="0" y1="0" x2="0" y2="8" stroke="%s" stroke-width="2" stroke-opacity="0.55"/></pattern></defs>`+"\n",
		UnknownInk, UnknownInk)
}

// rampIndex maps slot i of n onto the ramp. The ramp is a PROGRESSION across
// the axis; with fewer slots than steps it still walks the ramp in order, so
// the lightness gradient always reads left to right.
func rampIndex(i, n int) int {
	if n <= 1 {
		return len(Ramp) - 1
	}
	idx := i * (len(Ramp) - 1) / (n - 1)
	if idx >= len(Ramp) {
		idx = len(Ramp) - 1
	}
	return idx
}

// Table is the mandatory table view (§7.6: every chart ships a hover tooltip
// AND a table view). It is markdown, and it states each slot's source, probe,
// window and limit — a chart that hides its caps in a tooltip has hidden them.
//
// A could-not-check slot's value cell reads the literal token and its reason.
// There is no formatting path here that emits "0" for a stateless answer,
// because the value cell is derived from Value()'s ok flag.
func (c Chart) Table() string {
	var b strings.Builder
	fmt.Fprintf(&b, "### %s\n\n", c.Title)
	fmt.Fprintf(&b, "%s (%s) · as-of %s\n\n", c.Series.Name, c.Series.Unit, c.AsOf.String())
	b.WriteString("| Slot | Value | State | Source | Probe | Window | Limit | Note |\n")
	b.WriteString("|---|---|---|---|---|---|---|---|\n")
	for _, p := range c.Series.Points {
		value := CouldNotCheckToken
		if v, ok := p.Answer.Value(); ok {
			value = formatInt(v)
		}
		s := p.Answer.Source()
		note := reasonOf(p.Answer)
		if note == "" {
			note = "—"
		}
		fmt.Fprintf(&b, "| %s | %s | %s | %s | %s | %s | %s | %s |\n",
			cell(p.Label), cell(value), cell(string(p.Answer.State())),
			cell(orDash(s.Cmd)), cell(orDash(s.Probe)), cell(orDash(s.Window)), cell(orDash(s.Limit)),
			cell(note))
	}
	if c.Gate != nil {
		fmt.Fprintf(&b, "\n%s %s — human gate at slot %s.\n", GateGlyph, c.Gate.Label, c.Gate.AtLabel)
	}
	return b.String()
}

func reasonOf(a askassay.Answer) string {
	r := a.Reason()
	if r == "" && a.State() == askassay.CouldNotCheck {
		return "unstated"
	}
	return r
}

func firstReason(points []Point) string {
	for _, p := range points {
		if r := reasonOf(p.Answer); r != "" {
			return r
		}
	}
	return "no reason recorded"
}

func sourceLine(a askassay.Answer) string {
	s := a.Source()
	return fmt.Sprintf("source=%s · probe=%s · window=%s · limit=%s",
		orDash(s.Cmd), orDash(s.Probe), orDash(s.Window), orDash(s.Limit))
}

func orDash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "UNDECLARED"
	}
	return s
}

// cell makes a string safe for a markdown table cell. A raw pipe shreds the
// table it lands in, and a shredded cell is how an Evidence row came to
// assert a result for a command that could not execute.
func cell(s string) string {
	s = strings.ReplaceAll(s, "|", "/")
	s = strings.ReplaceAll(s, "\n", " ")
	return strings.TrimSpace(s)
}

func formatInt(v int64) string { return strconv.FormatInt(v, 10) }

func esc(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;")
	return r.Replace(s)
}
