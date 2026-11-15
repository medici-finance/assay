package perfdata_test

import (
	"testing"

	"github.com/medici-finance/assay/perfdata"
)

// TestDistributionEdges pins log2ms-v1 to the perf-v3 request-duration histogram edges
// (bin 0 = 0ms, bin 1 = 1ms, bin i = [2^(i-1), 2^i), bin 31 = 2^30ms and above), so a
// distribution recorded here and a perf-v3 histogram are read with the same buckets.
// The edges are written out, not imported: this module stays standard-library only.
func TestDistributionEdges(t *testing.T) {
	if perfdata.DistBins != 32 {
		t.Fatalf("log2ms-v1 has 32 bins, got %d", perfdata.DistBins)
	}
	for i := 0; i < perfdata.DistBins; i++ {
		lo, hi := perfdata.BinBounds(i)
		var wlo, whi int64
		switch {
		case i == 0:
			wlo, whi = 0, 1
		case i == 31:
			wlo, whi = 1<<30, 0
		default:
			wlo, whi = 1<<(i-1), 1<<i
		}
		if lo != wlo || hi != whi {
			t.Fatalf("bin %d edges [%d,%d), want [%d,%d)", i, lo, hi, wlo, whi)
		}
		if perfdata.Bin(lo) != i || (hi > 0 && perfdata.Bin(hi-1) != i) || (hi > 0 && perfdata.Bin(hi) != i+1) {
			t.Fatalf("bin %d: Bin(lo)=%d Bin(hi-1)=%d Bin(hi)=%d", i, perfdata.Bin(lo), perfdata.Bin(hi-1), perfdata.Bin(hi))
		}
	}
	if perfdata.Bin(1<<40) != 31 {
		t.Fatal("durations past 2^30ms land in the open-ended bin")
	}
}
