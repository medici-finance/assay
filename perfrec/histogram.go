package perfrec

// histogram.go — reading request-latency percentiles back out of the merged perf-v3
// duration histogram. RequestStats.DurationBins is the ONLY place request durations
// survive aggregation, and v3.go's durationBin is the one definition of the buckets —
// so the estimator that maps bucket counts back to milliseconds lives beside it in this
// package. A second definition of the edges anywhere else would drift from the writer's.

import "math"

// DurationBinBounds maps a DurationBins index back to its edges in milliseconds: bin i
// holds every duration d with durationBin(d) == i, i.e. lo <= d < hi. Bins 0 and 1 are
// exact (0 ms and 1 ms). Bin 31 is open-ended — every duration of 2^30 ms or more — and
// reports hi == 0, meaning "no upper edge".
func DurationBinBounds(i int) (lo, hi int64) {
	switch {
	case i <= 0:
		return 0, 1
	case i == 1:
		return 1, 2
	case i >= 31:
		return 1 << 30, 0
	default:
		return 1 << (i - 1), 1 << i
	}
}

// EstimateDurationPercentile estimates the k-th percentile (0 < k <= 100; the report asks
// for 50 and 90) of the measured durations behind a merged DurationBins histogram, and
// reports ok=false when k is out of range, a count is negative, or the histogram holds no
// measured sample. The rank is NEAREST-RANK over the samples — the same rank rule the exact
// invocation-wall percentiles use — and the found bin's samples are then spread evenly over
// the bin's edges (DurationBinBounds), so the estimate is the midpoint of the sub-interval
// the ranked sample falls in. The exact bins (0 and 1) return their exact value; a
// percentile landing in the open-ended bin 31 estimates as its lower edge, there being no
// upper edge to spread over. The result is an ESTIMATE read from bucket counts: it names no
// observed duration, and a caller that prints it labels it as one.
func EstimateDurationPercentile(bins [32]int64, k float64) (ms int64, ok bool) {
	if k <= 0 || k > 100 {
		return 0, false
	}
	var total int64
	for _, n := range bins {
		if n < 0 {
			return 0, false
		}
		total += n
	}
	if total == 0 {
		return 0, false
	}
	rank := int64(math.Ceil(k / 100 * float64(total)))
	if rank < 1 {
		rank = 1
	}
	var cum int64
	for i, n := range bins {
		if n == 0 {
			continue
		}
		if cum+n < rank {
			cum += n
			continue
		}
		lo, hi := DurationBinBounds(i)
		if i <= 1 || hi == 0 {
			return lo, true
		}
		// The (rank-cum)-th of the bin's n samples (j, 0-based), spread evenly over
		// [lo, hi): sample j sits at the midpoint of its sub-interval.
		j := rank - cum - 1
		ms = int64(math.Round(float64(lo) + float64(hi-lo)*(float64(j)+0.5)/float64(n)))
		if ms < lo {
			ms = lo
		}
		if ms >= hi {
			ms = hi - 1
		}
		return ms, true
	}
	return 0, false
}
