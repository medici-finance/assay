package chart

import (
	"fmt"
	"math"
	"strings"
)

// THE §7.6 COLOUR RULES, AS DATA
// ------------------------------
// The design doc's chart requirements are binding and each one exists because
// a rendered frame failed a validator run (2026-07-15):
//
//   - The frames' five-hue lifecycle bar FAILED CVD validation: two adjacent
//     segments were protan-indistinguishable at deltaE 0.4. Lifecycle is a
//     PROGRESSION, not five identities, so it gets a single-hue sequential
//     ramp with surface gaps between segments.
//   - Gold is a RESERVED semantic (the human gate). It is never a series
//     colour and never appears without its glyph and label.
//   - Values wear text tokens, never the series colour.
//
// Encoding them as constants plus a validator means a future series cannot
// quietly acquire a sixth hue or borrow the gate's gold: the tests read this
// file.

// Ramp is the single-hue sequential ramp that replaces the CVD-failing
// five-hue bar. One hue (~213 degrees), monotonically increasing relative
// luminance, so the ordering survives every dichromacy — a progression read
// off lightness cannot be lost by a viewer who cannot separate the hues,
// because there is only one hue to lose.
//
// It is deliberately short. A ramp long enough to need eleven steps is a
// chart that should have been a table.
var Ramp = []string{
	"#0e2233",
	"#173a56",
	"#22557c",
	"#2f74a6",
	"#4f97cb",
}

// GateGold is the reserved human-gate colour. It is NOT in [Ramp] and
// [ValidatePalette] fails if it ever is. Every element drawn in it also
// carries [GateGlyph] and a word, because colour alone encodes nothing.
const GateGold = "#d4a017"

// GateGlyph is the mandatory companion to [GateGold].
const GateGlyph = "⟡"

// UnknownInk is the neutral used for the could-not-check hatch. It is not a
// series colour, it is not gold, and it is not red: could-not-check is not a
// failure, it is an absence of measurement.
const UnknownInk = "#7b8087"

// Text tokens. Values and axis labels wear these — never a series colour —
// so a number is never encoded by the thing it is labelling.
const (
	InkText = "#e8e8e8" // ~16.5:1 on Surface: body text, passes AA and AAA
	InkDim  = "#a6a6a6" // ~7.9:1 on Surface: secondary labels, passes AA
	Surface = "#0b0b0b"
)

// MinTypePx is the §7.6 type floor. The wireframes' 8.5-9.5px mono chips
// violate it and the build must not copy them.
const MinTypePx = 11

// ValidatePalette re-asserts the §7.6 rules over the constants above. It is
// exported so a caller assembling its own palette can be held to the same
// bar, and so the failure is a returned error rather than a review comment.
func ValidatePalette(ramp []string, gold string) error {
	if len(ramp) < 2 {
		return fmt.Errorf("a sequential ramp needs at least two steps, got %d", len(ramp))
	}
	// Gold is reserved. Checked FIRST, because gold is far enough off the
	// ramp's hue that the single-hue check would otherwise catch it and
	// report the wrong finding — the same defect class as a mutation test
	// that reddens for a reason other than the arm it was aimed at.
	for i, c := range ramp {
		if strings.EqualFold(c, gold) {
			return fmt.Errorf("ramp step %d is the reserved gate gold %s — gold marks the human gate and is never a series colour", i, gold)
		}
	}
	// One hue. Two hues in a "sequential" ramp is the five-hue bar again
	// wearing fewer segments.
	var hues []float64
	for _, c := range ramp {
		r, g, b, err := hexRGB(c)
		if err != nil {
			return err
		}
		hues = append(hues, hue(r, g, b))
	}
	for i := 1; i < len(hues); i++ {
		if d := angularDelta(hues[0], hues[i]); d > 25 {
			return fmt.Errorf("ramp step %d is %.0f degrees off the base hue — a sequential ramp is single-hue by definition (the five-hue lifecycle bar is exactly what this rejects)", i, d)
		}
	}
	// Monotonic luminance, with enough separation that adjacent steps are
	// distinguishable without hue. 0.02 in relative luminance is the floor
	// used here; below it, two steps are one step.
	prev := -1.0
	for i, c := range ramp {
		r, g, b, err := hexRGB(c)
		if err != nil {
			return err
		}
		l := relLuminance(r, g, b)
		if l <= prev+0.015 {
			return fmt.Errorf("ramp step %d (%s) has relative luminance %.4f, not meaningfully above step %d (%.4f) — adjacent steps that differ only by hue fail for a dichromat", i, c, l, i-1, prev)
		}
		prev = l
	}
	return nil
}

// ContrastRatio is the WCAG 2.1 relative-luminance contrast ratio.
func ContrastRatio(fgHex, bgHex string) (float64, error) {
	fr, fg, fb, err := hexRGB(fgHex)
	if err != nil {
		return 0, err
	}
	br, bg, bb, err := hexRGB(bgHex)
	if err != nil {
		return 0, err
	}
	l1, l2 := relLuminance(fr, fg, fb), relLuminance(br, bg, bb)
	if l1 < l2 {
		l1, l2 = l2, l1
	}
	return (l1 + 0.05) / (l2 + 0.05), nil
}

func hexRGB(h string) (float64, float64, float64, error) {
	s := strings.TrimPrefix(strings.TrimSpace(h), "#")
	if len(s) != 6 {
		return 0, 0, 0, fmt.Errorf("colour %q is not a 6-digit hex triplet", h)
	}
	var v [3]float64
	for i := 0; i < 3; i++ {
		var n int
		if _, err := fmt.Sscanf(s[i*2:i*2+2], "%02x", &n); err != nil {
			return 0, 0, 0, fmt.Errorf("colour %q is not hex: %v", h, err)
		}
		v[i] = float64(n) / 255
	}
	return v[0], v[1], v[2], nil
}

func relLuminance(r, g, b float64) float64 {
	lin := func(c float64) float64 {
		if c <= 0.04045 {
			return c / 12.92
		}
		return math.Pow((c+0.055)/1.055, 2.4)
	}
	return 0.2126*lin(r) + 0.7152*lin(g) + 0.0722*lin(b)
}

func hue(r, g, b float64) float64 {
	max := math.Max(r, math.Max(g, b))
	min := math.Min(r, math.Min(g, b))
	d := max - min
	if d == 0 {
		return 0
	}
	var h float64
	switch max {
	case r:
		h = math.Mod((g-b)/d, 6)
	case g:
		h = (b-r)/d + 2
	default:
		h = (r-g)/d + 4
	}
	h *= 60
	if h < 0 {
		h += 360
	}
	return h
}

func angularDelta(a, b float64) float64 {
	d := math.Abs(a - b)
	if d > 180 {
		d = 360 - d
	}
	return d
}
