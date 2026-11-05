package perfrec

import (
	"encoding/json"
	"errors"
	"time"
)

const SchemaV3 = "perf-v3"

type ObservationKind string

const (
	ObservationProcess  ObservationKind = "process-summary"
	ObservationClient   ObservationKind = "service-client"
	ObservationHandler  ObservationKind = "service-handler"
	ObservationProvider ObservationKind = "provider-work"
)

type RequestClass string
type Origin string
type PointUnit string

const (
	RequestProvider       RequestClass = "provider"
	RequestAuthentication RequestClass = "authentication"
	RequestService        RequestClass = "service"
	OriginDirect          Origin       = "direct"
	OriginPassthrough     Origin       = "service-passthrough"
	OriginRefresh         Origin       = "refresh"
	OriginPoll            Origin       = "poll"
	OriginRepair          Origin       = "repair"
	OriginUnknown         Origin       = "unknown"
	UnitGitHubPoints      PointUnit    = "github-points"
	UnitGitLabComplexity  PointUnit    = "gitlab-complexity"
	UnitNone              PointUnit    = "none"
)

// Points never infers charges from shared quota gauges. Estimates retain method/version.
type Points struct {
	Coverage Coverage  `json:"coverage"`
	Unit     PointUnit `json:"unit"`
	Value    *int64    `json:"value,omitempty"`
	Method   string    `json:"method,omitempty"`
	Version  string    `json:"version,omitempty"`
}

// RequestObservation is one actual OWN attempt, timed by its adapter through body EOF/close
// or terminal error. It has no body, URL, query or quota-gauge field.
type RequestObservation struct {
	Class            RequestClass `json:"class"`
	Origin           Origin       `json:"origin"`
	Protocol         string       `json:"protocol"`
	Resource         string       `json:"resource"`
	Outcome          string       `json:"outcome"`
	DurationMs       *int64       `json:"durationMs,omitempty"`
	DurationCoverage Coverage     `json:"durationCoverage"`
	Bytes            *int64       `json:"bytes,omitempty"`
	Points           Points       `json:"points"`
}

// RequestStats merges fixed log2-ms histogram bins: 0ms, 1ms, 2..3ms, ... >=2^30ms.
type RequestStats struct {
	Class            RequestClass       `json:"class"`
	Origin           Origin             `json:"origin"`
	Protocol         string             `json:"protocol"`
	Resource         string             `json:"resource"`
	Outcome          string             `json:"outcome"`
	Attempts         int64              `json:"attempts"`
	DurationCoverage map[Coverage]int64 `json:"durationCoverage"`
	DurationBins     [32]int64          `json:"durationBins"`
	DurationSumMs    int64              `json:"durationSumMs"`
	DurationMaxMs    int64              `json:"durationMaxMs"`
	Bytes            int64              `json:"bytes"`
	BytesSamples     int64              `json:"bytesSamples"`
	Points           Points             `json:"points"`
	PointSamples     int64              `json:"pointSamples"`
}

const MaxRequestGroups = 64

// QueryObservation separates client logical calls from server handlers and upstream sends.
// ProviderAttempts requires fully observed own seams; client success cannot establish it.
type QueryObservation struct {
	Operation          string   `json:"operation"`
	LogicalCalls       int64    `json:"logicalCalls"`
	TransportAttempts  int64    `json:"transportAttempts"`
	ProviderAttempts   *int64   `json:"providerAttempts,omitempty"`
	ProviderCoverage   Coverage `json:"providerCoverage"`
	Source             string   `json:"source"`
	Coalescing         string   `json:"coalescing"`
	Result             string   `json:"result"`
	DiagnosticID       string   `json:"diagnosticId,omitempty"`
	ParentDiagnosticID string   `json:"parentDiagnosticId,omitempty"`
}

// V3Data adds selected disclosure and bounded observations to the existing interval.
type V3Data struct {
	Profile         Profile           `json:"profile"`
	Attribution     Attribution       `json:"attribution"`
	Kind            ObservationKind   `json:"observationKind"`
	Requests        []RequestStats    `json:"requests"`
	RequestDrops    int64             `json:"requestDrops"`
	RequestCoverage Coverage          `json:"requestCoverage"`
	Query           *QueryObservation `json:"query,omitempty"`
}

func validPoints(p Points) bool {
	switch p.Unit {
	case UnitNone, UnitGitHubPoints, UnitGitLabComplexity:
	default:
		return false
	}
	switch p.Coverage {
	case CoverageMeasured:
		return p.Unit != UnitNone && p.Value != nil && *p.Value >= 0 && p.Method == "" && p.Version == ""
	case CoverageEstimated:
		return p.Unit != UnitNone && p.Value != nil && *p.Value >= 0 && Label(p.Method) == p.Method && Label(p.Version) == p.Version
	case CoverageUnknown, CoverageUnsupported, CoverageFailed, CoverageMissing:
		return p.Value == nil && p.Method == "" && p.Version == ""
	}
	return false
}
func validRequest(r RequestObservation) bool {
	switch r.Class {
	case RequestProvider, RequestAuthentication, RequestService:
	default:
		return false
	}
	switch r.Origin {
	case OriginDirect, OriginPassthrough, OriginRefresh, OriginPoll, OriginRepair, OriginUnknown:
	default:
		return false
	}
	switch r.Protocol {
	case "rest", "graphql", "http", "local":
	default:
		return false
	}
	switch r.Resource {
	case "core", "search", "graphql", "complexity", "authentication", "service", "unknown":
	default:
		return false
	}
	switch r.Outcome {
	case "2xx", "3xx", "304", "4xx", "5xx", "error", "cancelled", "timeout":
	default:
		return false
	}
	if r.DurationCoverage == CoverageMeasured {
		if r.DurationMs == nil || *r.DurationMs < 0 {
			return false
		}
	} else {
		switch r.DurationCoverage {
		case CoverageMissing, CoverageFailed, CoverageUnsupported:
		default:
			return false
		}
		if r.DurationMs != nil {
			return false
		}
	}
	if r.Bytes != nil && *r.Bytes < 0 {
		return false
	}
	if !validPoints(r.Points) {
		return false
	}
	if r.Class != RequestProvider && (r.Points.Value != nil || r.Points.Unit != UnitNone) {
		return false
	}
	if r.Points.Unit == UnitGitHubPoints && (r.Protocol != "graphql" || r.Resource != "graphql") {
		return false
	}
	if r.Points.Unit == UnitGitLabComplexity && (r.Protocol != "graphql" || r.Resource != "complexity") {
		return false
	}
	return true
}
func durationBin(ms int64) int {
	if ms == 0 {
		return 0
	}
	i := 1
	for ms > 1 && i < 31 {
		ms >>= 1
		i++
	}
	return i
}
func sameRequest(a RequestStats, b RequestObservation) bool {
	return a.Class == b.Class && a.Origin == b.Origin && a.Protocol == b.Protocol && a.Resource == b.Resource && a.Outcome == b.Outcome && a.Points.Coverage == b.Points.Coverage && a.Points.Unit == b.Points.Unit && a.Points.Method == b.Points.Method && a.Points.Version == b.Points.Version
}
func cloneV3(v *V3Data) *V3Data {
	if v == nil {
		return nil
	}
	b, _ := json.Marshal(v)
	var d V3Data
	_ = json.Unmarshal(b, &d)
	return &d
}
func validQuery(p Profile, k ObservationKind, q *QueryObservation) bool {
	if q == nil {
		return true
	}
	if k != ObservationClient && k != ObservationHandler {
		return false
	}
	switch q.Operation {
	case "query", "source-query", "mirror-read", "refresh", "poll", "repair", "other":
	default:
		return false
	}
	if q.LogicalCalls < 0 || q.TransportAttempts < 0 {
		return false
	}
	if k == ObservationHandler && (q.LogicalCalls != 0 || q.TransportAttempts != 0) {
		return false
	}
	if k == ObservationClient && (q.ProviderAttempts != nil || q.ProviderCoverage == CoverageMeasured) {
		return false
	}
	switch q.ProviderCoverage {
	case CoverageMeasured:
		if q.ProviderAttempts == nil || *q.ProviderAttempts < 0 {
			return false
		}
	case CoverageMissing, CoverageUnknown, CoverageUnsupported, CoverageFailed:
		if q.ProviderAttempts != nil {
			return false
		}
	default:
		return false
	}
	switch q.Source {
	case "hit", "miss", "stale", "bypass", "unavailable":
	default:
		return false
	}
	switch q.Coalescing {
	case "none", "leader", "waiter", "failed", "abandoned", "unavailable":
	default:
		return false
	}
	switch q.Result {
	case "ok", "partial", "stale", "denied", "error", "cancelled", "timeout", "unknown":
	default:
		return false
	}
	if (q.DiagnosticID != "" || q.ParentDiagnosticID != "") && p != ProfileWorkflow {
		return false
	}
	return (q.DiagnosticID == "" || IDValid(q.DiagnosticID)) && (q.ParentDiagnosticID == "" || IDValid(q.ParentDiagnosticID))
}
func validV3(r Record) error {
	if Label(r.Tool) != r.Tool || Label(r.Verb) != r.Verb || SourceSHA(r.SourceSHA) != r.SourceSHA || r.Result != ResultForExit(r.Exit) {
		return errors.New("perfrec: invalid v3 fixed metadata")
	}
	if _, err := time.Parse(time.RFC3339, r.TS); err != nil {
		return errors.New("perfrec: invalid timestamp")
	}
	v := r.V3
	if v == nil || !validAttribution(v.Profile, v.Attribution) || r.hasLegacyCounters() {
		return ErrSchemaReuse
	}
	if (v.Profile == ProfileTool && r.Role != RoleNone) || (v.Profile == ProfileWorkflow && r.Role != v.Attribution.Role && !(r.Role == RoleNone && v.Attribution.Role == "")) {
		return errors.New("perfrec: invalid v3 role")
	}
	switch v.Kind {
	case ObservationProcess, ObservationClient, ObservationHandler, ObservationProvider:
	default:
		return errors.New("perfrec: invalid observation kind")
	}
	if v.Kind != ObservationProcess && (r.CPU != CPUUnsupported || r.CPUMs != nil || r.Boundary != BoundaryCaller) {
		return errors.New("perfrec: unqualified per-request CPU")
	}
	switch v.RequestCoverage {
	case CoverageMeasured, CoverageMissing, CoverageUnsupported, CoverageFailed:
	default:
		return errors.New("perfrec: invalid request coverage")
	}
	if len(v.Requests) > MaxRequestGroups || v.RequestDrops < 0 || !validQuery(v.Profile, v.Kind, v.Query) {
		return errors.New("perfrec: invalid observations")
	}
	if v.RequestDrops > 0 && v.RequestCoverage != CoverageFailed {
		return errors.New("perfrec: dropped request coverage must stay failed")
	}
	seenGroups := map[string]bool{}
	for _, s := range v.Requests {
		groupKey, _ := json.Marshal(struct {
			Class                       RequestClass
			Origin                      Origin
			Protocol, Resource, Outcome string
			Coverage                    Coverage
			Unit                        PointUnit
			Method, Version             string
		}{s.Class, s.Origin, s.Protocol, s.Resource, s.Outcome, s.Points.Coverage, s.Points.Unit, s.Points.Method, s.Points.Version})
		if seenGroups[string(groupKey)] {
			return errors.New("perfrec: duplicate request group")
		}
		seenGroups[string(groupKey)] = true
		zero := int64(0)
		o := RequestObservation{Class: s.Class, Origin: s.Origin, Protocol: s.Protocol, Resource: s.Resource, Outcome: s.Outcome, DurationCoverage: CoverageMeasured, DurationMs: &zero, Points: s.Points}
		if !validRequest(o) || s.Attempts < 1 || s.PointSamples != s.Attempts || s.DurationSumMs < 0 || s.DurationMaxMs < 0 || s.Bytes < 0 || s.BytesSamples < 0 || s.BytesSamples > s.Attempts {
			return errors.New("perfrec: invalid request aggregate")
		}
		var n, bins int64
		for c, x := range s.DurationCoverage {
			switch c {
			case CoverageMeasured, CoverageMissing, CoverageUnsupported, CoverageFailed:
			default:
				return errors.New("perfrec: invalid duration coverage")
			}
			if x < 0 {
				return ErrSchemaReuse
			}
			if !addNonnegative(&n, x) {
				return errAggregateOverflow
			}
		}
		for _, x := range s.DurationBins {
			if x < 0 {
				return ErrSchemaReuse
			}
			if !addNonnegative(&bins, x) {
				return errAggregateOverflow
			}
		}
		measured := s.DurationCoverage[CoverageMeasured]
		if s.DurationMaxMs > s.DurationSumMs || (measured == 0 || s.DurationMaxMs == 0) && s.DurationSumMs != 0 {
			return errors.New("perfrec: inconsistent duration aggregate")
		}
		// Compare sum <= samples*max without overflowing the product.
		if s.DurationMaxMs > 0 {
			q, rem := s.DurationSumMs/s.DurationMaxMs, s.DurationSumMs%s.DurationMaxMs
			if q > measured || q == measured && rem > 0 {
				return errors.New("perfrec: inconsistent duration aggregate")
			}
		}
		if n != s.Attempts || bins != s.DurationCoverage[CoverageMeasured] {
			return errors.New("perfrec: incomplete duration coverage")
		}
	}
	return nil
}
func BeginSelected(p Probe, start Mark, roster []string, c Context) *Session {
	s := Begin(p, start, roster)
	s.SelectContext(c)
	return s
}

// BeginObservation excludes whole-process CPU, which cannot be assigned to concurrent handlers.
func BeginObservation(p Probe, roster []string, c Context, k ObservationKind) *Session {
	p.CPU = nil
	s := BeginSelected(p, p.Mark(), roster, c)
	if s.v3 != nil {
		s.v3.Kind = k
	}
	return s
}

// SelectContext freezes parsed context once; timings retain the original measurement mark.
func (s *Session) SelectContext(c Context) bool {
	if s == nil {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.contextSet {
		return false
	}
	s.contextSet = true
	if c.Profile() == "" {
		return true
	}
	s.v3 = &V3Data{Profile: c.Profile(), Attribution: c.Data(), Kind: ObservationProcess, Requests: []RequestStats{}, RequestCoverage: CoverageMissing}
	return true
}

// ObserveRequest observes only completed effects; invalid/excess input increments drops.
func (s *Session) ObserveRequest(r RequestObservation) bool {
	if s == nil {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.v3 == nil {
		return false
	}
	if !validRequest(r) {
		s.v3.RequestDrops++
		s.v3.RequestCoverage = CoverageFailed
		return false
	}
	i := 0
	for ; i < len(s.v3.Requests); i++ {
		if sameRequest(s.v3.Requests[i], r) {
			break
		}
	}
	if i == len(s.v3.Requests) {
		if i == MaxRequestGroups {
			s.v3.RequestDrops++
			s.v3.RequestCoverage = CoverageFailed
			return false
		}
		p := r.Points
		if p.Value != nil {
			zero := int64(0)
			p.Value = &zero
		}
		s.v3.Requests = append(s.v3.Requests, RequestStats{Class: r.Class, Origin: r.Origin, Protocol: r.Protocol, Resource: r.Resource, Outcome: r.Outcome, Points: p, DurationCoverage: map[Coverage]int64{}})
	}
	a := &s.v3.Requests[i]
	a.Attempts++
	a.PointSamples++
	a.DurationCoverage[r.DurationCoverage]++
	if r.DurationMs != nil {
		a.DurationBins[durationBin(*r.DurationMs)]++
		a.DurationSumMs += *r.DurationMs
		if *r.DurationMs > a.DurationMaxMs {
			a.DurationMaxMs = *r.DurationMs
		}
	}
	if r.Bytes != nil {
		a.Bytes += *r.Bytes
		a.BytesSamples++
	}
	if r.Points.Value != nil {
		*a.Points.Value += *r.Points.Value
	}
	return true
}
func (s *Session) SetQuery(q QueryObservation) bool {
	if s == nil {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.v3 == nil || !validQuery(s.v3.Profile, s.v3.Kind, &q) {
		return false
	}
	s.v3.Query = cloneV3(&V3Data{Query: &q}).Query
	return true
}

// RequestSink is the credential-free provider adapter seam.
type RequestSink interface{ ObserveRequest(RequestObservation) bool }

// ObserveRequests marks the own transport seams covered, including measured zero sends.
// Call only when every relevant own seam is observed; partial callers leave coverage missing.
func (s *Session) ObserveRequests() {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.v3 != nil && s.v3.RequestDrops == 0 {
		s.v3.RequestCoverage = CoverageMeasured
	}
}
