package avatar

import (
	"fmt"
	"strings"
)

// Palette — the constant geometry colours shared by every tile (brief sources
// §5 / the stamp-family geometry).
const (
	colGround    = "#0A0A0A" // rounded-corner near-black ground
	colBodyDark  = "#10141B" // octagon body on a read/dark tile
	colFrameBlue = "#3366FF" // Electric-Blue, the team default frame hue
	colInnerEdge = "#05070A" // the thin punched inner edge
	colGlyphLite = "#EAF0FF" // a light glyph on a hue-filled body
	colInkDark   = "#0A0A0A" // a dark glyph on a bright (act) body
)

// Role hues for the full role suite (brief sources §5). Held as their
// canonical hex accents; the muted body fill is derived from each.
const (
	colReviewer    = "#00CC66"
	colWorker      = "#F08A2D"
	colVerifier    = "#E6E9F2"
	colDesk        = "#5C85FF"
	colIssueLoop   = "#D46BFF"
	colIntakeLoop  = "#2DD4BF"
	colBoardWriter = "#E5484D" // crimson — no other role holds a red hue
)

// tileSpec fully determines one avatar tile. Generate builds these; every
// downstream artifact (SVG source, structural raster, proof render) is a pure
// function of the spec, which is what makes regeneration byte-identical.
type tileSpec struct {
	App        string // file-name stem, e.g. "example-org-read", "example-org-reviewer"
	Glyph      string // glyph key
	Body       rgb    // octagon body fill
	Frame      rgb    // octagon frame stroke
	GlyphColor rgb    // glyph colour
	Initials   string // identity monogram (source + full-size raster)
	IdentColor rgb    // low-contrast identity colour
	Avatar     []byte // optional uploaded org avatar PNG (identity field)
}

// octagon point strings in the 512-unit design space. Kept as vars (not consts)
// so the geometry is patchable in one place; the outer octagon is large so the
// body colour dominates the 20 px mean the proof metric reads.
var (
	octagonPoints = "136,16 376,16 496,136 496,376 376,496 136,496 16,376 16,136"
	octagonInner  = "140,30 372,30 482,140 482,372 372,482 140,482 30,372 30,140"
)

// svgBase is the structural ground+octagon+frame+inner-edge, without glyph or
// identity field. Returned as an inner fragment so callers can layer over it.
func (s tileSpec) svgBase() string {
	var b strings.Builder
	fmt.Fprintf(&b, `<rect x="0" y="0" width="512" height="512" rx="96" fill="%s"/>`, colGround)
	fmt.Fprintf(&b, `<polygon points="%s" fill="%s" stroke="%s" stroke-width="20" stroke-linejoin="round"/>`,
		octagonPoints, s.Body.hex(), s.Frame.hex())
	fmt.Fprintf(&b, `<polygon points="%s" fill="none" stroke="%s" stroke-width="4" stroke-linejoin="round"/>`,
		octagonInner, colInnerEdge)
	return b.String()
}

// svgIdentityText is the identity monogram as an SVG <text> node — the source-
// of-truth layer that sits behind the glyph. The rasteriser (oksvg) does not
// draw <text>; the full-size PNG draws the same monogram with a font (raster.go)
// so the uploaded file carries it too. It is deliberately low-contrast.
//
// Crucially there is NO fineness mark (`A·999`) in this node — the uploaded
// avatar omits it by design (brief Task 5).
func (s tileSpec) svgIdentityText() string {
	if s.Initials == "" || len(s.Avatar) > 0 {
		return ""
	}
	return fmt.Sprintf(
		`<text x="256" y="322" font-family="Georgia, 'Times New Roman', serif" font-size="176" font-weight="600" text-anchor="middle" fill="%s">%s</text>`,
		s.IdentColor.hex(), s.Initials)
}

// svgGlyphLayer is the glyph fragment alone.
func (s tileSpec) svgGlyphLayer() string { return glyphSVG(s.Glyph, s.GlyphColor) }

// wrap wraps inner fragments in a complete SVG document.
func wrap(inner ...string) string {
	return `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 512 512" width="512" height="512">` +
		strings.Join(inner, "") + `</svg>`
}

// svgStructural is base + glyph, the render the proof metric and the golden
// strips operate on. It is font-free, so it is byte-identical across platforms.
func (s tileSpec) svgStructural() string {
	return wrap(s.svgBase(), s.svgGlyphLayer())
}

// svgSource is the source-of-truth SVG written to <app>.svg: base + identity
// field + glyph, the identity field behind the glyph.
func (s tileSpec) svgSource() string {
	return wrap(s.svgBase(), s.svgIdentityText(), s.svgGlyphLayer())
}
