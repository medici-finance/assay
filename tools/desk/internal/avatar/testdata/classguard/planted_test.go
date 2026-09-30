// Positive-control fixture for TestCompressedGoldenGuardPositiveControl — never compiled (testdata).
// It repeats the #1952 shape: a committed PNG golden compared byte-for-byte.
package classguard

import (
	"bytes"
	"os"
	"testing"
)

func TestPlanted(t *testing.T) {
	want, _ := os.ReadFile("testdata/golden/strip.png")
	var got []byte
	if !bytes.Equal(got, want) {
		t.Fatal("differs")
	}
}
