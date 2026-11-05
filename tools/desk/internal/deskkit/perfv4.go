package deskkit

import (
	"github.com/medici-finance/assay/perfrec"
	"sort"
	"time"
)

// PerfV4Day is deliberately separate from historical daily cohorts. Selection
// precedes day/tool buckets, so changed copies cannot hide across midnight.
type PerfV4Day struct {
	Day     string            `json:"day"`
	Tool    string            `json:"tool"`
	Summary perfrec.V4Summary `json:"summary"`
}

func SummarisePerfV4Days(records []PerfRecord, permission perfrec.Profile, f perfrec.V3Filter) ([]PerfV4Day, error) {
	kept, _, err := perfrec.SelectV4(records, permission, f)
	if err != nil {
		return nil, err
	}
	type key struct{ day, tool string }
	by := map[key][]PerfRecord{}
	for _, r := range kept {
		ts, err := time.Parse(time.RFC3339, r.TS)
		if err != nil {
			continue
		}
		k := key{ts.UTC().Format("2006-01-02"), r.Tool}
		by[k] = append(by[k], r)
	}
	keys := make([]key, 0, len(by))
	for k := range by {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].day != keys[j].day {
			return keys[i].day < keys[j].day
		}
		return keys[i].tool < keys[j].tool
	})
	var out []PerfV4Day
	for _, k := range keys {
		s, err := perfrec.ReduceV4(by[k], permission, "tool", f)
		if err != nil {
			return nil, err
		}
		out = append(out, PerfV4Day{k.day, k.tool, s})
	}
	return out, nil
}
