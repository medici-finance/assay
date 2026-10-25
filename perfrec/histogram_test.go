package perfrec

import "testing"

// TestDurationBinBoundsInvertDurationBin pins DurationBinBounds as durationBin's inverse
// over every edge the writer can produce: every duration lands in the bounds of the bin
// durationBin puts it in. This is the anti-drift check — the estimator reads the writer's
// bucket definition, never a second copy of it.
func TestDurationBinBoundsInvertDurationBin(t *testing.T) {
	values := []int64{0, 1, 2, 3, 4, 5, 7, 8, 15, 16, 100, 1000,
		1<<29 - 1, 1 << 29, 1<<30 - 1, 1 << 30, 1<<30 + 1, 1 << 40, 1<<62 - 1}
	for _, ms := range values {
		lo, hi := DurationBinBounds(durationBin(ms))
		if ms < lo {
			t.Errorf("%d ms: bin %d lower edge %d exceeds it", ms, durationBin(ms), lo)
		}
		if hi != 0 && ms >= hi {
			t.Errorf("%d ms: bin %d upper edge %d does not exceed it", ms, durationBin(ms), hi)
		}
	}
	// Every bin's bounds re-bin every interior duration to itself.
	for i := 0; i < 32; i++ {
		lo, hi := DurationBinBounds(i)
		if got := durationBin(lo); got != i {
			t.Errorf("bin %d lower edge %d re-bins to %d", i, lo, got)
		}
		if hi != 0 {
			if got := durationBin(hi - 1); got != i {
				t.Errorf("bin %d top duration %d re-bins to %d", i, hi-1, got)
			}
		}
	}
}

// TestEstimateDurationPercentile hand-checks the estimator: refusal shapes first (no
// samples, an out-of-range k, a negative count), then the exact bins, nearest-rank across
// bins, the even spread inside a bin, and the open-ended top bin estimating as its own
// lower edge.
func TestEstimateDurationPercentile(t *testing.T) {
	var empty [32]int64
	if _, ok := EstimateDurationPercentile(empty, 50); ok {
		t.Error("an empty histogram has no p50")
	}
	var one [32]int64
	one[3] = 1
	for _, k := range []float64{0, -1, 100.1} {
		if _, ok := EstimateDurationPercentile(one, k); ok {
			t.Errorf("k %v is out of range and was not refused", k)
		}
	}
	bad := one
	bad[4] = -1
	if _, ok := EstimateDurationPercentile(bad, 50); ok {
		t.Error("a negative bucket count is not a histogram")
	}

	bins := func(entries map[int]int64) (b [32]int64) {
		for i, n := range entries {
			b[i] = n
		}
		return b
	}
	for name, tc := range map[string]struct {
		bins [32]int64
		k    float64
		want int64
	}{
		// Exact bins: bin 0 is exactly 0 ms, bin 1 exactly 1 ms, however many samples.
		"zeros":     {bins(map[int]int64{0: 3}), 50, 0},
		"zeros p90": {bins(map[int]int64{0: 3}), 90, 0},
		"one ms":    {bins(map[int]int64{1: 2}), 90, 1},
		// One sample in bin 3 ([4, 8)): the sub-interval midpoint 4 + 4*0.5 = 6.
		"single sample":     {one, 50, 6},
		"single sample p90": {one, 90, 6},
		// Nearest-rank across bins: 2 samples total. p50 -> rank 1 -> bin 2 ([2,4)):
		// 2 + 2*0.5 = 3. p90 -> rank 2 -> bin 5 ([16,32)): 16 + 16*0.5 = 24.
		"across bins p50": {bins(map[int]int64{2: 1, 5: 1}), 50, 3},
		"across bins p90": {bins(map[int]int64{2: 1, 5: 1}), 90, 24},
		// Three samples in bin 7 ([64,128)): ranks 2 and 3 spread evenly —
		// 64 + 64*1.5/3 = 96, 64 + 64*2.5/3 = 117.33 -> 117.
		"spread p50": {bins(map[int]int64{7: 3}), 50, 96},
		"spread p90": {bins(map[int]int64{7: 3}), 90, 117},
		// The open-ended bin 31 has no upper edge to spread over: its lower edge.
		"open top bin": {bins(map[int]int64{31: 2}), 50, 1 << 30},
		// A p100 lands on the last sample of the last non-empty bin.
		"p100": {bins(map[int]int64{2: 1, 7: 3}), 100, 117},
	} {
		got, ok := EstimateDurationPercentile(tc.bins, tc.k)
		if !ok || got != tc.want {
			t.Errorf("%s: p%v = %d, %v; want %d, true", name, tc.k, got, ok, tc.want)
		}
	}
}
