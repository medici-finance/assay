package cursorbundle

import (
	"fmt"
	"os"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// Acquire consumes the upgrade resolver's selected composition. A legacy release
// without this asset refuses; no source checkout or adjacent tag is substituted.
func Acquire(src deskkit.CompositionSource, comp deskkit.Composition) (string, func(), error) {
	if !versionPattern.MatchString(comp.Umbrella) {
		return "", nil, fmt.Errorf("selected Cursor composition must name strict vX.Y.Z")
	}
	expected := comp.AssetSHA256[AssetName]
	if !shaPattern.MatchString(expected) {
		return "", nil, fmt.Errorf("release %s has no verifiable Cursor bundle digest; choose a release publishing %s and checksums.txt (legacy releases are not supplemented from floating source)", comp.Umbrella, AssetName)
	}
	raw, err := src.CursorBundle(comp.Umbrella)
	if err != nil {
		return "", nil, err
	}
	bundle, err := VerifyArchive(raw, comp.Umbrella, expected)
	if err != nil {
		return "", nil, err
	}
	dir, err := os.MkdirTemp("", "assay-cursor-verified-")
	if err != nil {
		return "", nil, err
	}
	cleanup := func() { _ = os.RemoveAll(dir) }
	if err = bundle.Materialize(dir); err != nil {
		cleanup()
		return "", nil, err
	}
	return dir, cleanup, nil
}
