package avatar

import "fmt"

// Glyph keys — the seven bold silhouettes the suite draws. Each is centred in
// the 512-unit design space and sized well above the design floor (≥ 7/64 of the
// canvas) so it survives a 20 px render.
const (
	GlyphRing   = "ring"   // reviewer
	GlyphCheck  = "check"  // (team read/act discriminator is colour; check is a role glyph)
	GlyphHammer = "hammer" // worker
	GlyphDisc   = "disc"   // desk
	GlyphStar   = "star"   // team (read + act)
	GlyphTicket = "ticket" // issue-loop
	GlyphFunnel = "funnel" // intake-loop
)

// glyphKeys is the fixed enumeration, used by tests to assert the whole set
// renders.
var glyphKeys = []string{
	GlyphRing, GlyphCheck, GlyphHammer, GlyphDisc,
	GlyphStar, GlyphTicket, GlyphFunnel,
}

// glyphSVG returns the SVG fragment for one glyph, painted in the given colour.
// Outline glyphs (ring, check) use the colour as a stroke; solid glyphs use it
// as a fill. Every fragment is a pure function of (key, colour) — no time, no
// randomness — so a regenerated avatar is byte-identical.
func glyphSVG(key string, fill rgb) string {
	c := fill.hex()
	switch key {
	case GlyphRing:
		return fmt.Sprintf(`<circle cx="256" cy="256" r="112" fill="none" stroke="%s" stroke-width="36"/>`, c)
	case GlyphCheck:
		return fmt.Sprintf(`<path d="M176 262 l56 60 l112 -140" fill="none" stroke="%s" stroke-width="42" stroke-linecap="round" stroke-linejoin="round"/>`, c)
	case GlyphHammer:
		return fmt.Sprintf(`<g fill="%s"><rect x="150" y="150" width="150" height="64" rx="10" transform="rotate(45 256 256)"/><rect x="236" y="150" width="40" height="220" rx="12" transform="rotate(45 256 256)"/></g>`, c)
	case GlyphDisc:
		return fmt.Sprintf(`<circle cx="256" cy="256" r="124" fill="%s"/>`, c)
	case GlyphStar:
		// Regular five-point star, point up, circumradius 118, centred (256,256).
		return fmt.Sprintf(`<polygon points="256,138 293,227 388,234 315,296 338,389 256,338 174,389 197,296 124,234 219,227" fill="%s"/>`, c)
	case GlyphTicket:
		// A ticket: a bold rounded body. It is deliberately a large central blob so
		// its silhouette overlaps the disc — the colour-distant reviewer/worker pair
		// the collapse test (Verify row 6) relies on.
		return fmt.Sprintf(`<rect x="132" y="178" width="248" height="156" rx="28" fill="%s"/>`, c)
	case GlyphFunnel:
		return fmt.Sprintf(`<polygon points="156,172 356,172 300,258 300,344 212,344 212,258" fill="%s"/>`, c)
	default:
		panic("avatar: unknown glyph " + key)
	}
}
