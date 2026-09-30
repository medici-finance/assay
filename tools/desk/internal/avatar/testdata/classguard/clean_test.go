// Negative-control fixture for TestCompressedGoldenGuardPositiveControl — never compiled (testdata).
// It reads the same kind of golden but compares decoded content, so it must NOT be flagged.
package classguard

import (
	"bytes"
	"image/png"
	"os"
	"testing"
)

func TestClean(t *testing.T) {
	want, _ := os.ReadFile("testdata/golden/strip.png")
	if _, err := png.Decode(bytes.NewReader(want)); err != nil {
		t.Fatal(err)
	}
}
