//go:build untrustscan_broken_unicode

package deskkit

import (
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit/untrustcorpus"
)

// TestUntrustscanMutation is the NEGATIVE-PATH control for Verify row 5. It compiles ONLY
// under the `untrustscan_broken_unicode` build tag, which swaps in an EMPTIED
// injectionUnicodeRanges table (untrustscan_unicode_broken.go). With the detector's Unicode
// range table disarmed, the injection-unicode samples — the ones that FLAG on the live
// build (Verify row 3) — must scan CLEAN on the injection family.
//
// This proves the row-3 control is a live lamp, not one wired to nothing: a scanner whose
// Verify table cannot go red when a detector is disarmed has verified nothing.
func TestUntrustscanMutation(t *testing.T) {
	samples, err := untrustcorpus.Load("untrustcorpus/testdata")
	if err != nil {
		t.Fatalf("load corpus: %v", err)
	}
	var checked int
	for _, s := range samples {
		if s.Class != "injection-unicode" {
			continue
		}
		checked++
		v := UntrustScan(s.Bytes, UntrustScanOptions{DisableSemgrep: true})
		if v.Families[FamilyInjection].State != UntrustStateClean {
			t.Fatalf("%s: injection family = %q with the Unicode table disarmed, want %q — "+
				"the detector did NOT go silent, so row 3 is not testing a live control",
				s.ID, v.Families[FamilyInjection].State, UntrustStateClean)
		}
	}
	if checked == 0 {
		t.Fatalf("no injection-unicode sample in corpus — cannot run the mutation control")
	}
}
