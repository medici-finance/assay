package avatar

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"
)

// TestUploadedOmitsFinenessMark is Verify row 7: the generated SVG carries the
// identity field but NO fineness mark (`A·999`) — the uploaded avatar omits it
// by design. Checked for both the plain monogram and the uploaded-avatar path.
func TestUploadedOmitsFinenessMark(t *testing.T) {
	set, err := Generate("example-org", TierTeam, Options{})
	if err != nil {
		t.Fatal(err)
	}
	for _, av := range set {
		svg := string(av.SVG)
		if strings.Contains(svg, "A·999") {
			t.Errorf("%s: SVG contains the fineness mark A·999, which must be omitted", av.App)
		}
		if strings.Contains(strings.ToLower(svg), "fineness") {
			t.Errorf("%s: SVG references a fineness mark", av.App)
		}
		// The identity field itself IS present as a <text> node (behind the glyph).
		if !strings.Contains(svg, "<text") {
			t.Errorf("%s: expected an identity <text> node in the source SVG", av.App)
		}
	}

	// The uploaded-avatar path (--avatar) must also omit the fineness mark.
	up, err := Generate("example-org", TierTeam, Options{Avatar: tinyPNG(t)})
	if err != nil {
		t.Fatal(err)
	}
	for _, av := range up {
		if strings.Contains(string(av.SVG), "A·999") {
			t.Errorf("%s: uploaded-avatar SVG contains the fineness mark A·999", av.App)
		}
	}
}

// tinyPNG returns a 4×4 two-tone PNG used to exercise the --avatar path.
func tinyPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			c := color.RGBA{20, 20, 20, 255}
			if (x+y)%2 == 0 {
				c = color.RGBA{230, 230, 230, 255}
			}
			img.SetRGBA(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}
