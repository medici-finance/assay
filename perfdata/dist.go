package perfdata

// dist.go — the versioned mergeable millisecond distribution. Its 32 log2 bins are the
// same edges as the perf-v3 request-duration histogram (perfrec's DurationBinBounds):
// bin 0 holds 0 ms, bin 1 holds 1 ms, bin i (2..30) holds [2^(i-1), 2^i) ms and bin 31
// holds every value of 2^30 ms or more. The definition is mirrored rather than imported
// so this contract package stays standard-library only; dist_test.go pins the edges.
//
// Count and sum give a pooled mean; exact retained samples give an exact nearest-rank
// percentile; without them the percentile is a labelled bin estimate. A percentile is
// never averaged: pooled distributions are merged first and the percentile is read once.

import (
	"math"
	"sort"
)

// DistVersion identifies the bin layout and merge rule.
const DistVersion = "log2ms-v1"

// DistBins is the number of bins.
const DistBins = 32

// MaxEventSamples bounds the exact samples one event may carry.
const MaxEventSamples = 64

// Distribution is a mergeable millisecond distribution. Samples, when present, hold
// every observation exactly (len == Count); when absent only the bins are known.
type Distribution struct {
	Version string          `json:"v"`
	Count   int64           `json:"count"`
	SumMs   int64           `json:"sumMs"`
	MaxMs   int64           `json:"maxMs"`
	Bins    [DistBins]int64 `json:"bins"`
	Samples []int64         `json:"samples,omitempty"`
}

// NewDistribution returns an empty distribution of the current version.
func NewDistribution() Distribution { return Distribution{Version: DistVersion} }

// DistributionOf builds a distribution holding every value exactly.
func DistributionOf(ms ...int64) Distribution {
	d := NewDistribution()
	for _, v := range ms {
		d.Add(v)
	}
	return d
}

// Bin returns the bin index of a non-negative millisecond value.
func Bin(ms int64) int {
	if ms <= 0 {
		return 0
	}
	i := 1
	for ms > 1 && i < DistBins-1 {
		ms >>= 1
		i++
	}
	return i
}

// BinBounds returns bin i's edges, lo <= v < hi; hi == 0 means open-ended.
func BinBounds(i int) (lo, hi int64) {
	switch {
	case i <= 0:
		return 0, 1
	case i == 1:
		return 1, 2
	case i >= DistBins-1:
		return 1 << 30, 0
	default:
		return 1 << (i - 1), 1 << i
	}
}

// Add records one exact observation.
func (d *Distribution) Add(ms int64) {
	if ms < 0 {
		ms = 0
	}
	if d.Version == "" {
		d.Version = DistVersion
	}
	exact := len(d.Samples) == int(d.Count)
	d.Count++
	d.SumMs += ms
	if ms > d.MaxMs {
		d.MaxMs = ms
	}
	d.Bins[Bin(ms)]++
	if exact {
		d.Samples = append(d.Samples, ms)
	}
}

// Exact reports whether every observation is retained exactly.
func (d Distribution) Exact() bool { return d.Count > 0 && int64(len(d.Samples)) == d.Count }

// Merge returns the pooled distribution of a and b. Exact samples survive only while
// both sides are exact and the total stays within sampleCap; otherwise the result
// keeps bins, count, sum and max and drops samples (its percentile becomes an
// estimate). Merging different versions is refused.
func Merge(a, b Distribution, sampleCap int) (Distribution, error) {
	if a.Version == "" {
		a.Version = DistVersion
	}
	if b.Version == "" {
		b.Version = DistVersion
	}
	if a.Version != DistVersion || b.Version != DistVersion {
		return Distribution{}, refuse(RefuseUnsupported, "v", "distribution versions differ or are unsupported")
	}
	out := Distribution{Version: DistVersion, Count: a.Count + b.Count, SumMs: a.SumMs + b.SumMs, MaxMs: max(a.MaxMs, b.MaxMs)}
	for i := range out.Bins {
		out.Bins[i] = a.Bins[i] + b.Bins[i]
	}
	aExact := a.Count == 0 || a.Exact()
	bExact := b.Count == 0 || b.Exact()
	if aExact && bExact && out.Count <= int64(sampleCap) && out.Count > 0 {
		out.Samples = make([]int64, 0, out.Count)
		out.Samples = append(out.Samples, a.Samples...)
		out.Samples = append(out.Samples, b.Samples...)
	}
	return out, nil
}

// Mean returns the pooled arithmetic mean (sum/count), ok=false when empty.
func (d Distribution) Mean() (float64, bool) {
	if d.Count == 0 {
		return 0, false
	}
	return float64(d.SumMs) / float64(d.Count), true
}

// Percentile methods.
const (
	MethodExact    = "exact-nearest-rank"
	MethodEstimate = "log2ms-v1-bin-estimate"
)

// Percentile returns the nearest-rank k-th percentile (0 < k <= 100): exact from the
// retained samples, otherwise a bin estimate named by method. ok=false when empty.
func (d Distribution) Percentile(k float64) (ms int64, method string, ok bool) {
	if k <= 0 || k > 100 || d.Count <= 0 {
		return 0, "", false
	}
	rank := int64(math.Ceil(k / 100 * float64(d.Count)))
	if rank < 1 {
		rank = 1
	}
	if d.Exact() {
		s := append([]int64(nil), d.Samples...)
		sort.Slice(s, func(i, j int) bool { return s[i] < s[j] })
		return s[rank-1], MethodExact, true
	}
	var cum int64
	for i, n := range d.Bins {
		if n == 0 {
			continue
		}
		if cum+n < rank {
			cum += n
			continue
		}
		lo, hi := BinBounds(i)
		if i <= 1 || hi == 0 {
			return lo, MethodEstimate, true
		}
		j := rank - cum - 1
		v := int64(math.Round(float64(lo) + float64(hi-lo)*(float64(j)+0.5)/float64(n)))
		v = min(max(v, lo), hi-1)
		return v, MethodEstimate, true
	}
	return 0, "", false
}

func (d Distribution) validate(field string) error {
	return d.validateCap(field, MaxEventSamples)
}

// ValidateStored checks a distribution held in a rollup (larger sample cap).
func (d Distribution) ValidateStored(sampleCap int) error { return d.validateCap("dist", sampleCap) }

func (d Distribution) validateCap(field string, sampleCap int) error {
	if d.Version != DistVersion {
		return refuse(RefuseUnsupported, field, "unsupported distribution version")
	}
	if d.Count < 0 || d.SumMs < 0 || d.MaxMs < 0 || d.Count > 1<<40 {
		return refuse(RefuseMeasurement, field, "negative or out-of-range distribution")
	}
	var total int64
	top := -1
	for i, n := range d.Bins {
		if n < 0 {
			return refuse(RefuseMeasurement, field, "negative bin")
		}
		if n > 0 {
			top = i
		}
		total += n
	}
	if total != d.Count {
		return refuse(RefuseMeasurement, field, "bins must sum to count")
	}
	if d.Count == 0 {
		if d.SumMs != 0 || d.MaxMs != 0 || d.Samples != nil {
			return refuse(RefuseMeasurement, field, "an empty distribution has no sum, max or samples")
		}
		return nil
	}
	if Bin(d.MaxMs) != top || d.SumMs < d.MaxMs {
		return refuse(RefuseMeasurement, field, "max is inconsistent with bins or sum")
	}
	if d.Samples == nil {
		return nil
	}
	if int64(len(d.Samples)) != d.Count || len(d.Samples) > sampleCap {
		return refuse(RefuseBounds, field, "samples must be complete and within the sample cap")
	}
	var bins [DistBins]int64
	var sum, mx int64
	for _, v := range d.Samples {
		if v < 0 {
			return refuse(RefuseMeasurement, field, "negative sample")
		}
		bins[Bin(v)]++
		sum += v
		mx = max(mx, v)
	}
	if bins != d.Bins || sum != d.SumMs || mx != d.MaxMs {
		return refuse(RefuseMeasurement, field, "samples disagree with bins, sum or max")
	}
	return nil
}
