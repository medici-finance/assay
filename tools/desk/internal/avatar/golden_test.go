package avatar

import (
	"bytes"
	"flag"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

var update = flag.Bool("update", false, "regenerate the golden 20 px strips")

// buildStrip lays every tile of a tier out as 20 px structural squares in a
// horizontal strip — the committed artifact a reviewer eyeballs and the test
// compares byte-for-byte. It is the INDEPENDENT layer behind the proof metric
// (brief single-point-of-failure): a palette or geometry regression the metric
// happens to accept is still a visible golden diff a reviewer must approve.
func buildStrip(t *testing.T, tier Tier) []byte {
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
	var buf bytes.Buffer
	enc := png.Encoder{CompressionLevel: png.DefaultCompression}
	if err := enc.Encode(&buf, strip); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// TestGolden20px is Verify row 5: the generated strips match the committed
// goldens byte-for-byte. Run with -update to regenerate them.
func TestGolden20px(t *testing.T) {
	cases := map[Tier]string{
		TierTeam:   "team-20px.png",
		TierFamily: "family-20px.png",
	}
	for tier, name := range cases {
		got := buildStrip(t, tier)
		path := filepath.Join("testdata", "golden", name)
		if *update {
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, got, 0o644); err != nil {
				t.Fatal(err)
			}
			t.Logf("updated golden %s (%d bytes)", path, len(got))
			continue
		}
		want, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read golden %s: %v (run `go test ./internal/avatar/ -run TestGolden20px -update`)", path, err)
		}
		if !bytes.Equal(got, want) {
			t.Errorf("%s: generated strip differs from the golden — a palette/geometry regression, or a deliberate change that needs `-update` and a reviewer's eye", name)
		}
	}
}
