package avatar

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"sync"

	"github.com/srwiley/oksvg"
	"github.com/srwiley/rasterx"
	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

// rasterizeSVGOnto parses an SVG document and draws it onto dst, compositing
// (source-over) over whatever dst already holds. oksvg is pure Go — no cgo, no
// system library — so the desk-tools release matrix is unchanged (Verify row 8).
func rasterizeSVGOnto(dst *image.RGBA, svg string) error {
	icon, err := oksvg.ReadIconStream(bytes.NewReader([]byte(svg)))
	if err != nil {
		return fmt.Errorf("parse svg: %w", err)
	}
	w, h := dst.Bounds().Dx(), dst.Bounds().Dy()
	icon.SetTarget(0, 0, float64(w), float64(h))
	icon.Draw(rasterx.NewDasher(w, h, rasterx.NewScannerGV(w, h, dst, dst.Bounds())), 1)
	return nil
}

// renderStructural renders base + glyph (no identity field) at size×size. This
// is the render the proof metric and the golden strips operate on: it is font-
// free, so it is byte-identical across platforms.
func renderStructural(sp tileSpec, size int) (*image.RGBA, error) {
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	if err := rasterizeSVGOnto(img, sp.svgStructural()); err != nil {
		return nil, err
	}
	return img, nil
}

// renderFull renders the uploaded avatar at size×size: base, then the identity
// field (behind the glyph), then the glyph on top.
func renderFull(sp tileSpec, size int) (*image.RGBA, error) {
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	if err := rasterizeSVGOnto(img, wrap(sp.svgBase())); err != nil {
		return nil, err
	}
	if err := drawIdentity(img, sp); err != nil {
		return nil, err
	}
	if err := rasterizeSVGOnto(img, wrap(sp.svgGlyphLayer())); err != nil {
		return nil, err
	}
	return img, nil
}

// renderPNG renders the full tile and encodes it as PNG. image/png encoding is
// deterministic, so two runs produce identical bytes (Verify row 4).
func renderPNG(sp tileSpec, size int) ([]byte, error) {
	img, err := renderFull(sp, size)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	enc := png.Encoder{CompressionLevel: png.DefaultCompression}
	if err := enc.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

var (
	faceOnce sync.Once
	faceFont *opentype.Font
	faceErr  error
)

func identityFont() (*opentype.Font, error) {
	faceOnce.Do(func() {
		faceFont, faceErr = opentype.Parse(goregular.TTF)
	})
	return faceFont, faceErr
}

// drawIdentity draws the identity field onto dst, behind the glyph. When an
// uploaded avatar is supplied it is used as a two-tone copy at 30 % opacity;
// otherwise the monogram initials are drawn with a font in the low-contrast
// identity colour. Font rasterisation is deterministic (no hinting, fixed size),
// so a regenerated PNG is byte-identical.
func drawIdentity(dst *image.RGBA, sp tileSpec) error {
	if len(sp.Avatar) > 0 {
		return drawUploadedAvatar(dst, sp.Avatar)
	}
	if sp.Initials == "" {
		return nil
	}
	ft, err := identityFont()
	if err != nil {
		return fmt.Errorf("parse identity font: %w", err)
	}
	size := dst.Bounds().Dx()
	// 176 units of the 512 design grid.
	px := 176.0 * float64(size) / 512.0
	face, err := opentype.NewFace(ft, &opentype.FaceOptions{Size: px, DPI: 72, Hinting: font.HintingNone})
	if err != nil {
		return fmt.Errorf("identity face: %w", err)
	}
	defer face.Close()

	c := sp.IdentColor
	d := &font.Drawer{
		Dst:  dst,
		Src:  image.NewUniform(color.RGBA{c.R, c.G, c.B, 0xFF}),
		Face: face,
	}
	adv := d.MeasureString(sp.Initials)
	m := face.Metrics()
	x := (fixed.I(size) - adv) / 2
	// Centre the cap height roughly on the tile centre.
	y := fixed.I(size)/2 + (m.Ascent-m.Descent)/2
	d.Dot = fixed.Point26_6{X: x, Y: y}
	d.DrawString(sp.Initials)
	return nil
}

// drawUploadedAvatar composites a two-tone quantized copy of an uploaded PNG at
// 30 % opacity, centred and scaled to ~62 % of the tile. Deterministic:
// nearest-neighbour scaling, a fixed luminance split, no dithering.
func drawUploadedAvatar(dst *image.RGBA, pngBytes []byte) error {
	src, err := png.Decode(bytes.NewReader(pngBytes))
	if err != nil {
		return fmt.Errorf("decode uploaded avatar: %w", err)
	}
	size := dst.Bounds().Dx()
	target := int(0.62 * float64(size))
	if target < 1 {
		target = 1
	}
	sb := src.Bounds()
	if sb.Dx() == 0 || sb.Dy() == 0 {
		return nil
	}
	lo := hexColor(colBodyDark)
	hi := hexColor(colGlyphLite)
	off := (size - target) / 2
	const alpha = 0.30
	for ty := 0; ty < target; ty++ {
		sy := sb.Min.Y + ty*sb.Dy()/target
		for tx := 0; tx < target; tx++ {
			sx := sb.Min.X + tx*sb.Dx()/target
			r16, g16, b16, a16 := src.At(sx, sy).RGBA()
			if a16 == 0 {
				continue
			}
			l := (0.299*float64(r16) + 0.587*float64(g16) + 0.114*float64(b16)) / 65535
			tone := lo
			if l >= 0.5 {
				tone = hi
			}
			dx, dy := off+tx, off+ty
			base := dst.RGBAAt(dx, dy)
			dst.SetRGBA(dx, dy, color.RGBA{
				R: clamp8(float64(base.R)*(1-alpha) + float64(tone.R)*alpha),
				G: clamp8(float64(base.G)*(1-alpha) + float64(tone.G)*alpha),
				B: clamp8(float64(base.B)*(1-alpha) + float64(tone.B)*alpha),
				A: 0xFF,
			})
		}
	}
	return nil
}
