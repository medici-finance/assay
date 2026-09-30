package avatar

import (
	"bytes"
	"flag"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

var update = flag.Bool("update", false, "regenerate the golden 20 px strips")

// goldenCases maps each tier to its committed 20 px strip.
var goldenCases = map[Tier]string{
	TierTeam:   "team-20px.png",
	TierFamily: "family-20px.png",
}

// buildStrip lays every tile of a tier out as 20 px structural squares in a
// horizontal strip — the committed artifact a reviewer eyeballs and the test
// compares pixel-for-pixel. It is the INDEPENDENT layer behind the proof metric
// (brief single-point-of-failure): a palette or geometry regression the metric
// happens to accept is still a visible golden diff a reviewer must approve.
func buildStrip(t *testing.T, tier Tier) *image.RGBA {
	t.Helper()
	specs, err := specsFor("example-org", tier, Options{})
	if err != nil {
		t.Fatal(err)
	}
	strip := image.NewRGBA(image.Rect(0, 0, proofSize*len(specs), proofSize))
	for i, sp := range specs {
		tile, err := renderStructural(sp, proofSize)
		if err != nil {
			t.Fatal(err)
		}
		for y := 0; y < proofSize; y++ {
			for x := 0; x < proofSize; x++ {
				strip.Set(i*proofSize+x, y, tile.At(x, y))
			}
		}
	}
	return strip
}

// encodeStrip is the PNG encoding the -update path writes.
func encodeStrip(img image.Image) ([]byte, error) {
	var buf bytes.Buffer
	enc := png.Encoder{CompressionLevel: png.DefaultCompression}
	if err := enc.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// pixelMismatch compares got against a golden PNG by DECODED PIXELS, exactly
// (zero tolerance). It returns "" when every pixel matches, otherwise a
// description of the first mismatch and the mismatch count.
//
// Why pixels and not file bytes (#1952): the encoded bytes of a PNG depend on
// the Go toolchain's compress/flate, which is not stable across releases — Go
// 1.27 emits a different deflate stream than 1.25/1.26 for the SAME filtered
// scanlines, so a byte-level check fails on any machine whose Go differs from
// the one that wrote the golden, while the image is identical. The rasteriser
// output itself is platform-independent (pure Go, font-free structural render),
// which is why no tolerance is needed: any pixel difference is a real change.
//
// got is passed through a PNG encode/decode round trip first, so both sides
// carry exactly what a PNG file can hold (PNG stores non-premultiplied colour);
// the round trip removes only the encoding, never a pixel difference.
func pixelMismatch(got image.Image, golden []byte) (string, error) {
	enc, err := encodeStrip(got)
	if err != nil {
		return "", fmt.Errorf("encode generated strip: %w", err)
	}
	g, err := png.Decode(bytes.NewReader(enc))
	if err != nil {
		return "", fmt.Errorf("decode generated strip: %w", err)
	}
	w, err := png.Decode(bytes.NewReader(golden))
	if err != nil {
		return "", fmt.Errorf("decode golden: %w", err)
	}
	if g.Bounds() != w.Bounds() {
		return fmt.Sprintf("bounds differ: generated %v, golden %v", g.Bounds(), w.Bounds()), nil
	}
	b := w.Bounds()
	n, first := 0, ""
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			gr, gg, gb, ga := g.At(x, y).RGBA()
			wr, wg, wb, wa := w.At(x, y).RGBA()
			if gr != wr || gg != wg || gb != wb || ga != wa {
				if n == 0 {
					first = fmt.Sprintf("(%d,%d) generated rgba16=%04x%04x%04x%04x golden rgba16=%04x%04x%04x%04x",
						x, y, gr, gg, gb, ga, wr, wg, wb, wa)
				}
				n++
			}
		}
	}
	if n == 0 {
		return "", nil
	}
	return fmt.Sprintf("%d of %d pixels differ, first at %s", n, b.Dx()*b.Dy(), first), nil
}

// TestGolden20px is Verify row 5: the generated strips match the committed
// goldens pixel-for-pixel (exact, zero tolerance). Run with -update to
// regenerate them.
func TestGolden20px(t *testing.T) {
	for tier, name := range goldenCases {
		got := buildStrip(t, tier)
		path := filepath.Join("testdata", "golden", name)
		if *update {
			enc, err := encodeStrip(got)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, enc, 0o644); err != nil {
				t.Fatal(err)
			}
			t.Logf("updated golden %s (%d bytes)", path, len(enc))
			continue
		}
		want, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read golden %s: %v (run `go test ./internal/avatar/ -run TestGolden20px -update`)", path, err)
		}
		diff, err := pixelMismatch(got, want)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if diff != "" {
			t.Errorf("%s: generated strip differs from the golden (%s) — a palette/geometry regression, or a deliberate change that needs `-update` and a reviewer's eye", name, diff)
		}
	}
}

// TestGoldenEncodingInvariant pins the comparison itself, on every platform:
// the same pixels under a DIFFERENT deflate stream must still match (the #1952
// toolchain-drift case, simulated by re-encoding at every compression level),
// and a single channel of a single pixel changed by one must not.
func TestGoldenEncodingInvariant(t *testing.T) {
	levels := []png.CompressionLevel{png.NoCompression, png.BestSpeed, png.BestCompression}
	for _, name := range goldenCases {
		want, err := os.ReadFile(filepath.Join("testdata", "golden", name))
		if err != nil {
			t.Fatal(err)
		}
		img, err := png.Decode(bytes.NewReader(want))
		if err != nil {
			t.Fatal(err)
		}
		rgba := image.NewRGBA(img.Bounds())
		for y := img.Bounds().Min.Y; y < img.Bounds().Max.Y; y++ {
			for x := img.Bounds().Min.X; x < img.Bounds().Max.X; x++ {
				rgba.Set(x, y, img.At(x, y))
			}
		}
		def, err := encodeStrip(rgba)
		if err != nil {
			t.Fatal(err)
		}
		for _, lvl := range levels {
			var buf bytes.Buffer
			if err := (&png.Encoder{CompressionLevel: lvl}).Encode(&buf, rgba); err != nil {
				t.Fatal(err)
			}
			if bytes.Equal(buf.Bytes(), def) {
				t.Fatalf("%s level %d: re-encode produced identical bytes — the case no longer exercises a different deflate stream", name, lvl)
			}
			diff, err := pixelMismatch(rgba, buf.Bytes())
			if err != nil {
				t.Fatal(err)
			}
			if diff != "" {
				t.Errorf("%s level %d: same pixels, different encoding, reported as a mismatch: %s", name, lvl, diff)
			}
		}
		// One channel of one pixel, off by one: must be caught.
		mut := image.NewRGBA(rgba.Bounds())
		copy(mut.Pix, rgba.Pix)
		mid := mut.PixOffset(rgba.Bounds().Dx()/2, rgba.Bounds().Dy()/2)
		mut.Pix[mid] ^= 1
		diff, err := pixelMismatch(mut, want)
		if err != nil {
			t.Fatal(err)
		}
		if diff == "" {
			t.Errorf("%s: a one-pixel, one-channel change was NOT reported — the comparison is not exact", name)
		}
	}
}
