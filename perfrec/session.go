package perfrec

// session.go — the perf-v2 measurement API: Probe, Mark, Begin, Phase, Add/Observe, Finish.
//
// One invocation is measured as
//
//	s := perfrec.Begin(perfrec.SystemProbe(), perfrec.ProcessEntry(), phaseRoster)
//	stop := s.Phase("fetch") // optional, fixed tool-owned roster
//	...
//	stop()
//	s.Add(perfrec.CounterHTTP, n) // or s.Observe(...) for a measured zero
//	rec := s.Finish(tool, verb, role, exit, sha)
//	perfrec.Append(rec) // best effort, AFTER Finish: writer cost is outside the measurement
//
// Wall time and CPU both start at the SAME Mark and both end inside Finish, before any
// record is built or written, so they describe one interval. What lies outside it is stated:
// with ProcessEntry, executable load, Go runtime start-up and the standard-library package
// initialisers perfrec imports precede the start; record encoding, the install-id read, the
// prune and the append follow the end. Executable elapsed time (the whole process as an
// external observer sees it) is a separate benchmark, never this field.
//
// A nil *Session is valid and does nothing, so a disabled caller pays one nil check per call.

import (
	"sync"
	"time"
)

// Probe is the clock and CPU source a Session reads. Tests inject both; SystemProbe is the
// real one.
type Probe struct {
	// Now is the duration clock (monotonic in production). nil = time.Now.
	Now func() time.Time
	// Stamp is the wall clock for the record's ts. nil = Now.
	Stamp func() time.Time
	// CPU returns this process's cumulative user+system CPU in ms; ok=false is a failed read.
	// nil means the build has no CPU source: coverage CPUUnsupported, never zero.
	CPU func() (ms int64, ok bool)
}

// SystemProbe is the real clock and, where the platform has one, the getrusage CPU source.
func SystemProbe() Probe { return Probe{Now: time.Now, CPU: cpuSampler} }

func (p Probe) now() time.Time {
	if p.Now != nil {
		return p.Now()
	}
	return time.Now()
}

func (p Probe) stamp() time.Time {
	if p.Stamp != nil {
		return p.Stamp()
	}
	return p.now()
}

// Mark is a measurement start: one wall reading and one CPU reading taken together at a
// declared boundary.
type Mark struct {
	boundary string
	at       time.Time
	cpuMs    int64
	cpu      string
}

// Boundary names where the mark was taken (BoundaryProcess or BoundaryCaller).
func (m Mark) Boundary() string { return m.boundary }

// At is the mark's wall reading.
func (m Mark) At() time.Time { return m.at }

// CPU is the mark's CPU coverage state: CPUMeasured when a start sample was read.
func (m Mark) CPU() string { return m.cpu }

func (p Probe) mark(boundary string) Mark {
	m := Mark{boundary: boundary, at: p.now()}
	switch {
	case p.CPU == nil:
		m.cpu = CPUUnsupported
	default:
		if ms, ok := p.CPU(); ok {
			m.cpuMs, m.cpu = ms, CPUMeasured
		} else {
			m.cpu = CPUFailed
		}
	}
	return m
}

// Mark takes a caller-boundary start: wall and CPU read back to back.
func (p Probe) Mark() Mark { return p.mark(BoundaryCaller) }

// entry is taken while this package initialises — before any importer's initialisers and
// before main — so every binary linking perfrec has a start that precedes its configuration
// and activation. With the switch off at that moment only the wall clock is read (no CPU
// syscall); a later Begin from that mark reports CPU coverage CPUMissing.
var entry = func() Mark {
	if !Enabled() {
		return Mark{boundary: BoundaryProcess, at: time.Now(), cpu: CPUMissing}
	}
	return SystemProbe().mark(BoundaryProcess)
}()

// ProcessEntry returns the process-init mark.
func ProcessEntry() Mark { return entry }

// Session measures one invocation from a Mark to Finish.
type Session struct {
	probe  Probe
	start  Mark
	phases []string // bounded, label-shaped, de-duplicated roster

	mu       sync.Mutex
	phaseN   []int64 // len(phases)+1; the last slot is "other"
	phaseNs  []int64
	open     int
	overlap  bool
	counts   [len(counterRosterArr)]int64
	measured [len(counterRosterArr)]bool
}

// counterRosterArr sizes the per-session counter arrays at compile time.
var counterRosterArr = [...]Counter{
	CounterSubprocess, CounterSubprocessMs, CounterHTTP,
	CounterCacheLookup, CounterCacheHit, CounterCacheMiss, CounterCacheRefresh,
	CounterWaitShared, CounterWaitFailed, CounterWaitAbandon,
}

// Begin starts a session from start. phaseRoster is the tool's own fixed phase list: entries
// that are not label-shaped or repeat an earlier entry are dropped, and only the first
// MaxPhases are kept. Any other phase name is recorded as "other".
func Begin(p Probe, start Mark, phaseRoster []string) *Session {
	s := &Session{probe: p, start: start}
	for _, name := range phaseRoster {
		if len(s.phases) == MaxPhases {
			break
		}
		if Label(name) != name || name == Other || contains(s.phases, name) {
			continue
		}
		s.phases = append(s.phases, name)
	}
	s.phaseN = make([]int64, len(s.phases)+1)
	s.phaseNs = make([]int64, len(s.phases)+1)
	return s
}

func contains(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}

func (s *Session) phaseIndex(name string) int {
	for i, v := range s.phases {
		if v == name {
			return i
		}
	}
	return len(s.phases)
}

func noop() {}

// Phase starts a timed phase and returns its stop function (idempotent). name is matched
// exactly against the roster; anything else counts as "other". A phase started while another
// is open marks the record PhaseOverlap. A phase still open at Finish is not recorded.
func (s *Session) Phase(name string) (stop func()) {
	if s == nil {
		return noop
	}
	i := s.phaseIndex(name)
	began := s.probe.now()
	s.mu.Lock()
	if s.open > 0 {
		s.overlap = true
	}
	s.open++
	s.mu.Unlock()
	var once sync.Once
	return func() {
		once.Do(func() {
			d := s.probe.now().Sub(began)
			if d < 0 {
				d = 0
			}
			s.mu.Lock()
			s.open--
			s.phaseN[i]++
			s.phaseNs[i] += int64(d)
			s.mu.Unlock()
		})
	}
}

// Add adds n (≥ 0) to counter c and marks c measured. An unknown counter or negative n is
// ignored.
func (s *Session) Add(c Counter, n int64) {
	if s == nil || n < 0 {
		return
	}
	i := counterIndex(c)
	if i < 0 {
		return
	}
	s.mu.Lock()
	s.counts[i] += n
	s.measured[i] = true
	s.mu.Unlock()
}

// Observe marks counters measured without adding to them: the caller's seam for each was in
// place, so a zero is a measured zero. Counters never Added or Observed are recorded as
// unavailable.
func (s *Session) Observe(cs ...Counter) {
	if s == nil {
		return
	}
	s.mu.Lock()
	for _, c := range cs {
		if i := counterIndex(c); i >= 0 {
			s.measured[i] = true
		}
	}
	s.mu.Unlock()
}

// Finish ends the measurement — wall reading, then CPU reading, back to back — and builds the
// perf-v2 record. tool, verb and role should already be roster members; each still passes
// through Label, so a label that is not roster-shaped is recorded as "other". Writing is the
// caller's next, separate step.
func (s *Session) Finish(tool, verb, role string, exit int, sha string) Record {
	if s == nil {
		return Record{}
	}
	end := s.probe.now()
	cpu, cpuMs := s.start.cpu, (*int64)(nil)
	if s.start.cpu == CPUMeasured {
		switch {
		case s.probe.CPU == nil:
			cpu = CPUUnsupported
		default:
			if v, ok := s.probe.CPU(); !ok || v < s.start.cpuMs {
				cpu = CPUFailed
			} else {
				d := v - s.start.cpuMs
				cpuMs = &d
			}
		}
	}
	wall := end.Sub(s.start.at)
	if wall < 0 {
		wall = 0
	}
	rec := Record{
		Schema:    SchemaV2,
		TS:        s.probe.stamp().UTC().Truncate(time.Second).Format(time.RFC3339),
		Tool:      Label(tool),
		Verb:      Label(verb),
		Role:      Label(role),
		Boundary:  s.start.boundary,
		WallMs:    wall.Milliseconds(),
		CPU:       cpu,
		CPUMs:     cpuMs,
		Exit:      exit,
		Result:    ResultForExit(exit),
		SourceSHA: SourceSHA(sha),
		Phases:    []PhaseStat{},
		Counters:  map[Counter]int64{},
	}
	if rec.Boundary != BoundaryProcess {
		rec.Boundary = BoundaryCaller
	}
	if role == "" {
		rec.Role = RoleNone
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.phaseN {
		if s.phaseN[i] == 0 {
			continue
		}
		name := Other
		if i < len(s.phases) {
			name = s.phases[i]
		}
		rec.Phases = append(rec.Phases, PhaseStat{Name: name, N: s.phaseN[i], Ms: time.Duration(s.phaseNs[i]).Milliseconds()})
	}
	rec.PhaseOverlap = s.overlap
	rec.Unavailable = []Counter{}
	for i, c := range counterRosterArr {
		if s.measured[i] {
			rec.Counters[c] = s.counts[i]
		} else {
			rec.Unavailable = append(rec.Unavailable, c)
		}
	}
	return rec
}
