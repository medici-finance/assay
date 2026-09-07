package avatar

import (
	"fmt"
	"hash/fnv"
	"math"
)

// rgb is a straight-alpha 8-bit colour used while composing a tile.
type rgb struct{ R, G, B uint8 }

// hexColor parses a #RRGGBB string. It panics on a malformed literal because
// every caller passes a compile-time constant from this package's palette — a
// bad literal is a programming error, not runtime data.
func hexColor(s string) rgb {
	var r, g, b uint8
	if len(s) != 7 || s[0] != '#' {
		panic("avatar: bad hex colour " + s)
	}
	if _, err := fmt.Sscanf(s, "#%02x%02x%02x", &r, &g, &b); err != nil {
		panic("avatar: bad hex colour " + s + ": " + err.Error())
	}
	return rgb{r, g, b}
}

// hex renders a colour back to #RRGGBB for embedding in SVG.
func (c rgb) hex() string { return fmt.Sprintf("#%02X%02X%02X", c.R, c.G, c.B) }

// hslToRGB converts an HSL triple (h in [0,360), s and l in [0,1]) to RGB. The
// conversion is the standard closed form; it is deterministic and allocation
// free.
func hslToRGB(h, s, l float64) rgb {
	h = math.Mod(math.Mod(h, 360)+360, 360)
	c := (1 - math.Abs(2*l-1)) * s
	hp := h / 60
	x := c * (1 - math.Abs(math.Mod(hp, 2)-1))
	var r1, g1, b1 float64
	switch {
	case hp < 1:
		r1, g1, b1 = c, x, 0
	case hp < 2:
		r1, g1, b1 = x, c, 0
	case hp < 3:
		r1, g1, b1 = 0, c, x
	case hp < 4:
		r1, g1, b1 = 0, x, c
	case hp < 5:
		r1, g1, b1 = x, 0, c
	default:
		r1, g1, b1 = c, 0, x
	}
	m := l - c/2
	return rgb{
		R: clamp8((r1 + m) * 255),
		G: clamp8((g1 + m) * 255),
		B: clamp8((b1 + m) * 255),
	}
}

// seedHue maps an org login to a stable hue in [0,360) via FNV-1a. No time, no
// randomness: the same login always yields the same hue, which is the
// determinism the brief requires.
func seedHue(org string) float64 {
	h := fnv.New32a()
	_, _ = h.Write([]byte(org))
	return float64(h.Sum32() % 360)
}

// mix blends a toward b by t in [0,1] in straight RGB space. Used to derive a
// muted body fill from a bright accent.
func mix(a, b rgb, t float64) rgb {
	return rgb{
		R: clamp8(float64(a.R)*(1-t) + float64(b.R)*t),
		G: clamp8(float64(a.G)*(1-t) + float64(b.G)*t),
		B: clamp8(float64(a.B)*(1-t) + float64(b.B)*t),
	}
}

func clamp8(v float64) uint8 {
	if v <= 0 {
		return 0
	}
	if v >= 255 {
		return 255
	}
	return uint8(v + 0.5)
}

// relLuminance returns the perceptual luminance of a colour in [0,1], used for
// the silhouette threshold in the proof metric.
func relLuminance(c rgb) float64 {
	// Rec. 601 luma is adequate for a threshold; it needs no gamma round-trip
	// and is monotone in brightness, which is all the silhouette mask asks for.
	return (0.299*float64(c.R) + 0.587*float64(c.G) + 0.114*float64(c.B)) / 255
}

// lab is a CIELAB colour.
type lab struct{ L, A, B float64 }

// toLab converts sRGB (0..255) to CIELAB under the D65 white point.
func toLab(c rgb) lab {
	// sRGB -> linear
	lin := func(v uint8) float64 {
		f := float64(v) / 255
		if f <= 0.04045 {
			return f / 12.92
		}
		return math.Pow((f+0.055)/1.055, 2.4)
	}
	r, g, b := lin(c.R), lin(c.G), lin(c.B)
	// linear sRGB -> XYZ (D65)
	x := r*0.4124564 + g*0.3575761 + b*0.1804375
	y := r*0.2126729 + g*0.7151522 + b*0.0721750
	z := r*0.0193339 + g*0.1191920 + b*0.9503041
	// normalise by D65 reference white
	x /= 0.95047
	z /= 1.08883
	f := func(t float64) float64 {
		if t > 216.0/24389.0 {
			return math.Cbrt(t)
		}
		return (24389.0/27.0*t + 16) / 116
	}
	fx, fy, fz := f(x), f(y), f(z)
	return lab{
		L: 116*fy - 16,
		A: 500 * (fx - fy),
		B: 200 * (fy - fz),
	}
}

// deltaE is the CIE76 colour difference between two CIELAB values. The brief's
// proof metric compares mean tile colours with a ΔE threshold; CIE76 is the
// closed-form Euclidean distance that threshold is expressed against.
func deltaE(a, b lab) float64 {
	dl := a.L - b.L
	da := a.A - b.A
	db := a.B - b.B
	return math.Sqrt(dl*dl + da*da + db*db)
}
