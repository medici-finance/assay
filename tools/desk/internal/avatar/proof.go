package avatar

import (
	"fmt"
	"image"
	"strings"
)

// Proof parameters (brief facts, "Proof metric at 20 px").
const (
	proofSize      = 20   // render each tile to 20×20, GitHub's timeline size
	proofDeltaE    = 25.0 // a pair is colour-indistinct below this CIE76 ΔE
	proofIoU       = 0.6  // …and silhouette-indistinct above this IoU
	proofLumThresh = 0.60 // a glyph pixel counts only when its mark is bright
)

// proofTile is the reduced representation of one tile at 20 px: its mean colour
// in CIELAB and its binary silhouette.
type proofTile struct {
	app  string
	mean lab
	sil  []bool // proofSize*proofSize, true where luminance > proofLumThresh
}

func proofTileFor(sp tileSpec) (proofTile, error) {
	// Mean colour is read from the whole composed tile (brief: "mean tile
	// colour").
	img, err := renderStructural(sp, proofSize)
	if err != nil {
		return proofTile{}, err
	}
	var sr, sg, sb float64
	n := float64(proofSize * proofSize)
	for i := 0; i+3 < len(img.Pix); i += 4 {
		sr += float64(img.Pix[i])
		sg += float64(img.Pix[i+1])
		sb += float64(img.Pix[i+2])
	}
	mean := rgb{clamp8(sr / n), clamp8(sg / n), clamp8(sb / n)}

	// Silhouette is the GLYPH's pixels (brief: "binary silhouette (glyph pixels
	// above a luminance threshold)") — rendered from the glyph layer alone so the
	// shared octagon frame does not swamp it. A glyph pixel is one the glyph layer
	// actually painted (alpha) whose colour clears the luminance threshold.
	gimg := image.NewRGBA(image.Rect(0, 0, proofSize, proofSize))
	if err := rasterizeSVGOnto(gimg, wrap(sp.svgGlyphLayer())); err != nil {
		return proofTile{}, err
	}
	sil := make([]bool, proofSize*proofSize)
	for p := 0; p < proofSize*proofSize; p++ {
		i := p * 4
		// A silhouette pixel is a glyph pixel (the glyph layer painted it at least
		// half-opaque) whose mark is BRIGHT (above the luminance threshold). This
		// is the brief's "glyph pixels above a luminance threshold" read exactly:
		// a light glyph on a dark body reads as a solid mark; a dark glyph punched
		// into a bright body (the act tile's inversion) reads as no mark, so the
		// read/act pair — same star, opposite figure/ground — never collides.
		if gimg.Pix[i+3] >= 128 && relLuminance(rgb{gimg.Pix[i], gimg.Pix[i+1], gimg.Pix[i+2]}) > proofLumThresh {
			sil[p] = true
		}
	}
	return proofTile{app: sp.App, mean: toLab(mean), sil: sil}, nil
}

// silhouetteIoU is the intersection-over-union of two binary silhouettes.
func silhouetteIoU(a, b []bool) float64 {
	inter, union := 0, 0
	for i := range a {
		if a[i] || b[i] {
			union++
			if a[i] && b[i] {
				inter++
			}
		}
	}
	if union == 0 {
		return 0
	}
	return float64(inter) / float64(union)
}

// collides reports whether two tiles are indistinguishable at 20 px: too close
// in mean colour AND too similar in silhouette (brief: ΔE < 25 AND IoU > 0.6).
func collides(a, b proofTile) (bool, float64, float64) {
	de := deltaE(a.mean, b.mean)
	iou := silhouetteIoU(a.sil, b.sil)
	return de < proofDeltaE && iou > proofIoU, de, iou
}

// ProofError is returned by runProof when a set contains an indistinguishable
// pair. It names both Apps of every colliding pair; the CLI exits 5 on it.
type ProofError struct {
	Collisions []string
}

func (e *ProofError) Error() string {
	return "avatar 20px proof failed — indistinguishable pair(s): " + strings.Join(e.Collisions, "; ")
}

// runProof renders every tile to 20 px and checks all pairs. Deterministic and
// allocation-bounded; it is the single control against an indistinguishable set
// (brief single-point-of-failure), run before any file is written.
func runProof(specs []tileSpec) error {
	tiles := make([]proofTile, len(specs))
	for i, sp := range specs {
		t, err := proofTileFor(sp)
		if err != nil {
			return err
		}
		tiles[i] = t
	}
	var collisions []string
	for i := 0; i < len(tiles); i++ {
		for j := i + 1; j < len(tiles); j++ {
			if hit, de, iou := collides(tiles[i], tiles[j]); hit {
				collisions = append(collisions,
					fmt.Sprintf("%s / %s (ΔE=%.1f, IoU=%.2f)", tiles[i].app, tiles[j].app, de, iou))
			}
		}
	}
	if len(collisions) > 0 {
		return &ProofError{Collisions: collisions}
	}
	return nil
}
