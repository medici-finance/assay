package perfrec

import (
	"encoding/json"
	"errors"
	"sort"
)

// V3Filter never resolves work references through a provider. Empty values match all.
type V3Filter struct {
	SessionID, AgentID, DeskRole, Role string
	Work                               *WorkRef
}
type V3Group struct {
	Key                 string `json:"key"`
	Records             int    `json:"records"`
	SumOwnProcessWallMs int64  `json:"sumOwnProcessWallMs"`
	CPURecords          int    `json:"cpuRecords"`
	SumOwnCPUMs         int64  `json:"sumOwnCpuMs"`
}

// V3RequestStats keeps each request aggregate in its originating observation kind.
type V3RequestStats struct {
	Kind ObservationKind `json:"observationKind"`
	RequestStats
}

// V3Summary keeps observation kinds, point units, estimates and coverage separate.
// Summed intervals are work, never elapsed time or savings. Conflicting IDs are excluded.
type V3Summary struct {
	Records                 int                                 `json:"records"`
	Duplicates              int                                 `json:"duplicates"`
	Conflicts               int                                 `json:"conflicts"`
	Invalid                 int                                 `json:"invalid"`
	Kinds                   map[ObservationKind]int             `json:"observationKinds"`
	Groups                  []V3Group                           `json:"groups"`
	Requests                []V3RequestStats                    `json:"requests"`
	RequestDrops            int64                               `json:"requestDrops"`
	RequestCoverage         map[Coverage]int64                  `json:"requestCoverage"`
	LogicalQueries          int64                               `json:"logicalQueries"`
	ServiceTransports       int64                               `json:"serviceTransportAttempts"`
	Handlers                int64                               `json:"handlers"`
	HandlerUpstreamCoverage map[Coverage]int64                  `json:"handlerUpstreamCoverage"`
	HandlerCoveredAttempts  int64                               `json:"handlerCoveredAttempts"`
	CacheSources            map[string]int64                    `json:"cacheSources"`
	Coalescing              map[string]int64                    `json:"coalescing"`
	ContextCoverage         map[ContextField]map[Coverage]int64 `json:"contextCoverage"`
}

func workflowDimension(s string) bool {
	switch s {
	case "session", "agent", "work", "deskRole", "role":
		return true
	}
	return false
}
func filterUsed(f V3Filter) bool {
	return f.SessionID != "" || f.AgentID != "" || f.DeskRole != "" || f.Role != "" || f.Work != nil
}
func workEqual(a, b WorkRef) bool { a.Primary = false; b.Primary = false; return a == b }

// Used reports whether the filter narrows anything: a filter in use needs workflow
// permission and keeps only workflow-profile records.
func (f V3Filter) Used() bool { return filterUsed(f) }

// V3Selection counts the perf-v3 records SelectV3 left out before any filter: Duplicates are
// exact replays of a kept identity (counted once), Conflicts are identities seen with
// different contents (every copy excluded), Invalid are records with no usable identity.
type V3Selection struct {
	Duplicates int
	Conflicts  int
	Invalid    int
}

// SelectV3 is the ONE definition of which perf-v3 records a v3 figure may count — the set
// ReduceV3 aggregates. It deduplicates the complete input BEFORE filters, so a conflicting
// duplicate cannot disappear by choosing a filter; the source identity is install + fresh
// invocation ID. It then applies the workflow filter. Records of any other schema are
// ignored. The kept records come back in identity order. A reader that buckets perf-v3
// records (by day, by tool) selects over the WHOLE input first and buckets the result, so a
// conflict whose copies would land in different buckets is still seen.
func SelectV3(records []Record, permission Profile, f V3Filter) ([]Record, V3Selection, error) {
	var sel V3Selection
	if filterUsed(f) && permission != ProfileWorkflow {
		return nil, sel, errors.New("perfrec: workflow permission required")
	}
	if f.Work != nil && !validWork([]WorkRef{*f.Work}) {
		return nil, sel, errors.New("perfrec: invalid work filter")
	}
	unique := map[string]Record{}
	enc := map[string]string{}
	conflict := map[string]bool{}
	for _, r := range records {
		if r.Schema != SchemaV3 {
			continue
		}
		b, e := json.Marshal(r)
		if e != nil || !InstallIDPattern.MatchString(r.Install) {
			sel.Invalid++
			continue
		}
		key := r.Install + ":" + r.V3.Attribution.InvocationID
		if old, ok := enc[key]; ok {
			if old == string(b) {
				sel.Duplicates++
			} else if !conflict[key] {
				sel.Conflicts++
				conflict[key] = true
			}
			continue
		}
		unique[key] = r
		enc[key] = string(b)
	}
	var keys []string
	for key := range unique {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var kept []Record
	for _, key := range keys {
		if conflict[key] {
			continue
		}
		r := unique[key]
		v := r.V3
		d := v.Attribution
		if filterUsed(f) && v.Profile != ProfileWorkflow {
			continue
		}
		if f.SessionID != "" && d.SessionID != f.SessionID || f.AgentID != "" && d.AgentID != f.AgentID || f.DeskRole != "" && d.DeskRole != f.DeskRole || f.Role != "" && d.Role != f.Role {
			continue
		}
		if f.Work != nil {
			found := false
			for _, w := range d.Work {
				if workEqual(w, *f.Work) {
					found = true
				}
			}
			if !found {
				continue
			}
		}
		kept = append(kept, r)
	}
	return kept, sel, nil
}

// ReduceV3 aggregates the records SelectV3 keeps (deduplicated before filters, conflicting
// identities excluded, the workflow filter applied) under one grouping dimension.
func ReduceV3(records []Record, permission Profile, dimension string, f V3Filter) (V3Summary, error) {
	s := V3Summary{Kinds: map[ObservationKind]int{}, RequestCoverage: map[Coverage]int64{}, HandlerUpstreamCoverage: map[Coverage]int64{}, CacheSources: map[string]int64{}, Coalescing: map[string]int64{}, ContextCoverage: map[ContextField]map[Coverage]int64{}}
	if dimension == "" {
		dimension = "tool"
	}
	if dimension != "tool" && !workflowDimension(dimension) {
		return s, errors.New("perfrec: unknown grouping dimension")
	}
	if workflowDimension(dimension) && permission != ProfileWorkflow {
		return s, errors.New("perfrec: workflow permission required")
	}
	kept, sel, err := SelectV3(records, permission, f)
	if err != nil {
		return s, err
	}
	s.Duplicates, s.Conflicts, s.Invalid = sel.Duplicates, sel.Conflicts, sel.Invalid
	groups := map[string]*V3Group{}
	for _, r := range kept {
		v := r.V3
		d := v.Attribution
		gkey := r.Tool
		switch dimension {
		case "session":
			gkey = d.SessionID
		case "agent":
			gkey = d.AgentID
		case "deskRole":
			gkey = d.DeskRole
		case "role":
			gkey = d.Role
		case "work":
			if len(d.Work) > 0 {
				b, _ := json.Marshal(d.Work)
				gkey = string(b)
			} else {
				gkey = ""
			}
		}
		if gkey == "" {
			gkey = "unavailable"
		}
		g := groups[gkey]
		if g == nil {
			g = &V3Group{Key: gkey}
			groups[gkey] = g
		}
		g.Records++
		s.Records++
		s.Kinds[v.Kind]++
		if v.Kind == ObservationProcess {
			g.SumOwnProcessWallMs += r.WallMs
			if r.CPU == CPUMeasured && r.CPUMs != nil {
				g.CPURecords++
				g.SumOwnCPUMs += *r.CPUMs
			}
		}
		for field, cov := range d.Coverage {
			if s.ContextCoverage[field] == nil {
				s.ContextCoverage[field] = map[Coverage]int64{}
			}
			s.ContextCoverage[field][cov]++
		}
		s.RequestDrops += v.RequestDrops
		s.RequestCoverage[v.RequestCoverage]++
		for _, x := range v.Requests {
			merged := false
			for i := range s.Requests {
				a := &s.Requests[i]
				if a.Kind == v.Kind && sameRequest(a.RequestStats, RequestObservation{Class: x.Class, Origin: x.Origin, Protocol: x.Protocol, Resource: x.Resource, Outcome: x.Outcome, Points: x.Points}) {
					a.Attempts += x.Attempts
					a.PointSamples += x.PointSamples
					a.DurationSumMs += x.DurationSumMs
					if x.DurationMaxMs > a.DurationMaxMs {
						a.DurationMaxMs = x.DurationMaxMs
					}
					for j, n := range x.DurationBins {
						a.DurationBins[j] += n
					}
					for c, n := range x.DurationCoverage {
						a.DurationCoverage[c] += n
					}
					a.Bytes += x.Bytes
					a.BytesSamples += x.BytesSamples
					if x.Points.Value != nil {
						*a.Points.Value += *x.Points.Value
					}
					merged = true
					break
				}
			}
			if !merged {
				copy := cloneV3(&V3Data{Requests: []RequestStats{x}})
				s.Requests = append(s.Requests, V3RequestStats{Kind: v.Kind, RequestStats: copy.Requests[0]})
			}
		}
		if q := v.Query; q != nil {
			switch v.Kind {
			case ObservationClient:
				s.LogicalQueries += q.LogicalCalls
				s.ServiceTransports += q.TransportAttempts
			case ObservationHandler:
				s.Handlers++
				s.HandlerUpstreamCoverage[q.ProviderCoverage]++
				if q.ProviderCoverage == CoverageMeasured {
					s.HandlerCoveredAttempts += *q.ProviderAttempts
				}
				s.CacheSources[q.Source]++
				s.Coalescing[q.Coalescing]++
			}
		}
	}
	for _, g := range groups {
		s.Groups = append(s.Groups, *g)
	}
	sort.Slice(s.Groups, func(i, j int) bool { return s.Groups[i].Key < s.Groups[j].Key })
	sort.Slice(s.Requests, func(i, j int) bool {
		a, _ := json.Marshal(s.Requests[i])
		b, _ := json.Marshal(s.Requests[j])
		return string(a) < string(b)
	})
	return s, nil
}
