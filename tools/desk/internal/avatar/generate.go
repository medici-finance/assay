package avatar

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// orgLoginRe is the GitHub org/user login shape: 1–39 chars, ASCII
// alphanumeric or hyphen, not starting with a hyphen. Validating the org here is
// defense-in-depth: the org flows into output file stems (`<org>-<role>.svg`), so
// pinning it to the login grammar keeps a stray `/` or `..` out of a path the
// caller later joins, whatever the caller's own trust boundary.
var orgLoginRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9-]{0,38}$`)

// MaxRenderSize caps a requested pixel size. Each size allocates an
// image.NewRGBA(size×size) (4 bytes/px), so an unbounded size is an unbounded
// allocation; 4096 px is far above GitHub's ~512 px master and keeps the worst
// case at ~64 MiB per tile.
const MaxRenderSize = 4096

// Tier selects which set of avatars Generate produces.
type Tier string

const (
	// TierTeam is the two-App set an adopter installs first: a read tile and an
	// act tile, both hued from the org login and inverted against each other.
	TierTeam Tier = "team"
	// TierFamily is the full role suite, one hue per role.
	TierFamily Tier = "family"
)

// Options tunes a Generate call. The zero value renders one 512 px PNG per tile
// with no uploaded avatar and no hue override — the deterministic default.
type Options struct {
	// Sizes are the pixel sizes to rasterise. Empty means [512].
	Sizes []int
	// Avatar is an optional uploaded org avatar PNG used as the identity field.
	Avatar []byte
	// forceAccent, when non-nil, overrides every family role accent with one
	// colour. It exists for the proof's collapse test (Verify row 6): forcing
	// every hue to #3366FF must make the proof name a colliding pair.
	forceAccent *rgb
}

// Avatar is one generated App tile: its SVG source plus a PNG per requested size.
type Avatar struct {
	App  string
	Role string
	SVG  []byte
	PNGs map[int][]byte
}

// roleDef is one entry in the family suite.
type roleDef struct {
	slug   string
	glyph  string
	accent string // canonical hex accent (brief sources §5)
}

// familyRoles binds each role to its hue and glyph. The glyph assignment is not
// arbitrary: desk (#5C85FF) and issue-loop (#D46BFF) are close enough in hue
// that their 20 px mean colours sit under the ΔE floor, so the proof can only
// tell them apart by silhouette — desk takes the solid disc and issue-loop the
// thin check, whose silhouettes are the most distinct pair in the set.
// familyRoles binds each role to its hue and glyph. The glyph assignment is
// load-bearing, not decorative:
//
//   - The palette has hues that sit under the 20 px ΔE floor as mean tiles:
//     desk (#5C85FF) vs issue-loop (#D46BFF), and intake-loop (#2DD4BF) vs both
//     reviewer (#00CC66) and verifier (#E6E9F2). Every one of those color-close
//     pairs is routed through a silhouette-DISTINCT glyph — intake-loop takes the
//     ring and desk the check (both near-zero IoU against anything) — so the
//     proof separates them by shape, not the colour it cannot rely on.
//   - reviewer (disc) and worker (ticket) are color-DISTANT but their glyph
//     silhouettes are two big central blobs that overlap (IoU > 0.6): colour
//     alone tells them apart. That is exactly the pair the collapse test (Verify
//     row 6) targets — force every hue to one blue and the two collide, which is
//     the regression the proof must catch.
//   - board-writer is the seventh bound role (a GitLab adopter roster binds all
//     seven). It holds the only red hue in the palette (#E5484D), far in colour
//     from every other role, and takes the pen glyph — a narrow vertical
//     silhouette distinct from the central blobs — so it clears the proof on
//     both axes rather than leaning on one.
var familyRoles = []roleDef{
	{"reviewer", GlyphDisc, colReviewer},
	{"worker", GlyphTicket, colWorker},
	{"verifier", GlyphHammer, colVerifier},
	{"desk", GlyphCheck, colDesk},
	{"issue-loop", GlyphFunnel, colIssueLoop},
	{"intake-loop", GlyphRing, colIntakeLoop},
	{"board-writer", GlyphPen, colBoardWriter},
}

// Generate builds the avatar set for org at the given tier. It is a pure
// function of its inputs: no time, no randomness, so `go test -count=2` — and a
// re-run months later — produces identical bytes (brief Task / Verify row 4).
//
// The hue seed is derived from the org login via FNV-1a (deterministic); an
// adopter needs to pass only the login. The installer imports this in-process.
func Generate(org string, tier Tier, opts Options) ([]Avatar, error) {
	if strings.TrimSpace(org) == "" {
		return nil, fmt.Errorf("avatar: org login is required")
	}
	if !orgLoginRe.MatchString(org) {
		return nil, fmt.Errorf("avatar: org %q is not a valid login (want %s — ASCII alphanumeric and hyphen, ≤39 chars, no leading hyphen)", org, orgLoginRe)
	}
	sizes := opts.Sizes
	if len(sizes) == 0 {
		sizes = []int{512}
	}
	for _, sz := range sizes {
		if sz < 1 || sz > MaxRenderSize {
			return nil, fmt.Errorf("avatar: size %d out of range (want 1..%d)", sz, MaxRenderSize)
		}
	}
	sort.Ints(sizes)

	specs, err := specsFor(org, tier, opts)
	if err != nil {
		return nil, err
	}

	// The 20 px legibility proof is the single control this design depends on
	// (brief single-point-of-failure). It runs BEFORE any file is produced, so a
	// set that would be indistinguishable in a PR timeline is never written.
	if err := runProof(specs); err != nil {
		return nil, err
	}

	out := make([]Avatar, 0, len(specs))
	for _, sp := range specs {
		av := Avatar{App: sp.App, Role: roleOf(sp.App, org), SVG: []byte(sp.svgSource()), PNGs: map[int][]byte{}}
		for _, sz := range sizes {
			png, err := renderPNG(sp, sz)
			if err != nil {
				return nil, fmt.Errorf("avatar %s @%dpx: %w", sp.App, sz, err)
			}
			av.PNGs[sz] = png
		}
		out = append(out, av)
	}
	return out, nil
}

func roleOf(app, org string) string {
	return strings.TrimPrefix(app, org+"-")
}

// specsFor builds the tile specs for a tier.
func specsFor(org string, tier Tier, opts Options) ([]tileSpec, error) {
	initials := initialsOf(org)
	switch tier {
	case TierTeam:
		h := seedHue(org)
		accent := hslToRGB(h, 0.72, 0.58)     // bright, reads on #0A0A0A
		accentDark := hslToRGB(h, 0.72, 0.40) // frame on the act tile
		dark := hexColor(colBodyDark)
		read := tileSpec{
			App: org + "-read", Glyph: GlyphStar,
			Body: dark, Frame: accent, GlyphColor: accent,
			Initials: initials, IdentColor: identColor(dark), Avatar: opts.Avatar,
		}
		act := tileSpec{
			App: org + "-act", Glyph: GlyphStar,
			Body: accent, Frame: accentDark, GlyphColor: hexColor(colInkDark),
			Initials: initials, IdentColor: identColor(accent), Avatar: opts.Avatar,
		}
		return []tileSpec{read, act}, nil
	case TierFamily:
		specs := make([]tileSpec, 0, len(familyRoles))
		for _, r := range familyRoles {
			accent := hexColor(r.accent)
			if opts.forceAccent != nil {
				accent = *opts.forceAccent
			}
			body := mix(accent, hexColor(colBodyDark), 0.55) // muted, hue-bearing
			glyph := hexColor(colGlyphLite)
			if relLuminance(body) >= 0.5 {
				glyph = hexColor(colInkDark)
			}
			specs = append(specs, tileSpec{
				App: org + "-" + r.slug, Glyph: r.glyph,
				Body: body, Frame: accent, GlyphColor: glyph,
				Initials: initials, IdentColor: identColor(body), Avatar: opts.Avatar,
			})
		}
		return specs, nil
	default:
		return nil, fmt.Errorf("avatar: unknown tier %q (want %q or %q)", tier, TierTeam, TierFamily)
	}
}

// identColor derives a low-contrast identity colour for a body: a small step
// toward the opposite ink, so the monogram sits just at the edge of legibility
// (brief: "low contrast on the dark body").
func identColor(body rgb) rgb {
	ink := hexColor(colGlyphLite)
	if relLuminance(body) >= 0.5 {
		ink = hexColor(colInkDark)
	}
	return mix(body, ink, 0.16)
}

// initialsOf returns the identity monogram: the upper-cased first letters of up
// to two hyphen- or space-separated words of the org login.
func initialsOf(org string) string {
	fields := strings.FieldsFunc(org, func(r rune) bool { return r == '-' || r == ' ' || r == '_' })
	var b strings.Builder
	for _, f := range fields {
		if f == "" {
			continue
		}
		b.WriteString(strings.ToUpper(f[:1]))
		if b.Len() >= 2 {
			break
		}
	}
	return b.String()
}
