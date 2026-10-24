package perfrec

// schema.go — the perf-v2 vocabulary and the JSON codec that keeps the two schemas apart.
//
// A day file may hold perf-v1 lines (older binaries, including older statusgen builds, and boot
// and launch steps) and perf-v2 lines (Session, which statusgen now writes too) side by side.
// The codec is the one place that decides which fields a line of each schema carries:
//
//   - Marshal writes EXACTLY the named schema's field set and refuses a record that carries
//     another schema's fields — a v1 record with a v2 field, or a v2 record with a legacy
//     counter, is silent schema reuse and is never written.
//   - Unmarshal decodes strictly (unknown keys refused) into the named schema's shape and
//     validates every v2 enumeration, so a reader never sees a v1 line reinterpreted as v2 or a
//     v2 line whose coverage is incomplete.

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
)

// SchemaV2 is the perf-v2 record version. Its wall and CPU are deltas over one declared
// Boundary, its counters carry explicit coverage, and it never carries the legacy counters.
const SchemaV2 = "perf-v2"

// The boundaries a perf-v2 wall/CPU delta can start at.
const (
	// BoundaryProcess is this package's initialisation: the first Go code of any binary that
	// links perfrec, before main and before its configuration or activation. Excluded:
	// executable load, Go runtime start-up and the package initialisers of the standard
	// library packages perfrec imports.
	BoundaryProcess = "process-init"
	// BoundaryCaller is a mark the caller took itself (Probe.Mark): an embedded consumer or a
	// test. The caller's documentation names where.
	BoundaryCaller = "caller"
)

// The CPU coverage states of a perf-v2 record. Only CPUMeasured carries cpuMs.
const (
	CPUMeasured    = "measured"    // both samples read; cpuMs is end minus start
	CPUUnsupported = "unsupported" // this build has no CPU source (Probe.CPU is nil)
	CPUFailed      = "failed"      // a sample was attempted and failed, or went backwards
	CPUMissing     = "missing"     // no start sample was taken at the boundary
)

// Counter is one member of the fixed perf-v2 counter roster. Counters are low-cardinality
// totals of effects the CALLER observed at its own seams; perfrec starts none of them.
type Counter string

// The counter roster, in record order. Every perf-v2 record names each one exactly once,
// either in counters (measured, possibly zero) or in unavailable (not observed).
const (
	CounterSubprocess   Counter = "subprocess"   // child process attempts, including failed starts
	CounterSubprocessMs Counter = "subprocessMs" // summed wall of those attempts; overlapping, inclusive
	CounterHTTP         Counter = "http"         // direct HTTP attempts, including retries and 304s
	CounterCacheLookup  Counter = "cacheLookup"  // cache lookups
	CounterCacheHit     Counter = "cacheHit"     // lookups answered from the cache
	CounterCacheMiss    Counter = "cacheMiss"    // lookups that fell through to the source
	CounterCacheRefresh Counter = "cacheRefresh" // cache entries rewritten from the source
	CounterWaitShared   Counter = "waitShared"   // coalesced waiters that received the leader's result
	CounterWaitFailed   Counter = "waitFailed"   // coalesced waiters whose leader failed
	CounterWaitAbandon  Counter = "waitAbandon"  // coalesced waiters that stopped waiting (cancel/timeout)
)

// counterRoster is the roster as a slice (counterRosterArr, session.go, is its array twin).
var counterRoster = counterRosterArr[:]

// Counters returns a copy of the counter roster, in record order.
func Counters() []Counter { return append([]Counter(nil), counterRoster...) }

func counterIndex(c Counter) int {
	for i, k := range counterRoster {
		if k == c {
			return i
		}
	}
	return -1
}

// PhaseStat is one timed phase of a perf-v2 record: the roster name (or "other"), how many
// times it ran, and its summed duration. Phase time is INCLUSIVE — phases may nest or overlap
// (PhaseOverlap says so) and include child-process time — so it is never added to wallMs or to
// another phase.
type PhaseStat struct {
	Name string `json:"name"`
	N    int64  `json:"n"`
	Ms   int64  `json:"ms"`
}

// MaxPhases bounds a phase roster: entries past it are dropped, so a record holds at most
// MaxPhases named phases plus "other".
const MaxPhases = 16

// labelPattern is the shape every perf-v2 label (tool, verb, role, phase) must have: a lowercase
// letter then lowercase letters, digits and '-', an optional leading "-" or "--" (a flag spelled as a verb) and at most
// one ":" qualifier, 64 characters at most. It is a last gate behind the callers' rosters: a
// path, a slug, a token, a number or free text cannot pass it.
var labelPattern = regexp.MustCompile(`^-{0,2}[a-z][a-z0-9-]{0,39}(:[a-z][a-z0-9-]{0,39})?$`)

// Label returns s when it has the shape of a roster label and Other otherwise. It returns its
// input or one constant, never a transformation of the input.
func Label(s string) string {
	if len(s) <= 64 && labelPattern.MatchString(s) {
		return s
	}
	return Other
}

// ErrSchemaReuse is the codec's refusal of a record whose fields belong to another schema.
var ErrSchemaReuse = errors.New("perfrec: record fields do not belong to its schema")

// legacyRecord is the perf-v1 wire shape, unchanged.
type legacyRecord struct {
	Schema      string `json:"schema"`
	TS          string `json:"ts"`
	Tool        string `json:"tool"`
	Verb        string `json:"verb"`
	Role        string `json:"role"`
	WallMs      int64  `json:"wallMs"`
	CPUMs       *int64 `json:"cpuMs,omitempty"`
	Exit        int    `json:"exit"`
	Result      string `json:"result"`
	ChildProcs  int64  `json:"childProcs"`
	ChildWallMs int64  `json:"childWallMs"`
	ForgeCalls  int64  `json:"forgeCalls"`
	SourceSHA   string `json:"sourceSHA"`
	Install     string `json:"install"`
}

// v2Record is the perf-v2 wire shape: every key always present except cpuMs.
type v2Record struct {
	Schema       string            `json:"schema"`
	TS           string            `json:"ts"`
	Tool         string            `json:"tool"`
	Verb         string            `json:"verb"`
	Role         string            `json:"role"`
	Boundary     string            `json:"boundary"`
	WallMs       int64             `json:"wallMs"`
	CPU          string            `json:"cpu"`
	CPUMs        *int64            `json:"cpuMs,omitempty"`
	Exit         int               `json:"exit"`
	Result       string            `json:"result"`
	Phases       []PhaseStat       `json:"phases"`
	PhaseOverlap bool              `json:"phaseOverlap"`
	Counters     map[Counter]int64 `json:"counters"`
	Unavailable  []Counter         `json:"unavailable"`
	SourceSHA    string            `json:"sourceSHA"`
	Install      string            `json:"install"`
}

func (r Record) hasV2Fields() bool {
	return r.Boundary != "" || r.CPU != "" || r.Phases != nil || r.PhaseOverlap || r.Counters != nil || r.Unavailable != nil
}

func (r Record) hasLegacyCounters() bool {
	return r.ChildProcs != 0 || r.ChildWallMs != 0 || r.ForgeCalls != 0
}

// MarshalJSON writes the named schema's field set exactly. A record carrying another schema's
// fields, an unknown schema, or an invalid perf-v2 coverage is refused, never written.
func (r Record) MarshalJSON() ([]byte, error) {
	if r.Schema != SchemaV4 && r.V4 != nil {
		return nil, ErrSchemaReuse
	}
	switch r.Schema {
	case SchemaV4:
		return marshalV4(r)
	case SchemaV3:
		if err := validV3(r); err != nil {
			return nil, err
		}
		old := r
		old.Schema = SchemaV2
		old.V3 = nil
		b, err := old.MarshalJSON()
		if err != nil {
			return nil, err
		}
		var fields map[string]json.RawMessage
		if err = json.Unmarshal(b, &fields); err != nil {
			return nil, err
		}
		fields["schema"], _ = json.Marshal(SchemaV3)
		fields["v3"], err = json.Marshal(r.V3)
		if err != nil {
			return nil, err
		}
		return json.Marshal(fields)
	case Schema:
		if r.hasV2Fields() || r.V3 != nil {
			return nil, ErrSchemaReuse
		}
		return json.Marshal(legacyRecord{r.Schema, r.TS, r.Tool, r.Verb, r.Role, r.WallMs, r.CPUMs, r.Exit,
			r.Result, r.ChildProcs, r.ChildWallMs, r.ForgeCalls, r.SourceSHA, r.Install})
	case SchemaV2:
		if r.hasLegacyCounters() || r.V3 != nil {
			return nil, ErrSchemaReuse
		}
		w := intervalWire(r)
		if err := validV2(w); err != nil {
			return nil, err
		}
		return json.Marshal(w)
	default:
		return nil, fmt.Errorf("perfrec: unknown schema %q", r.Schema)
	}
}

// intervalWire is the single reference mapping for interval validation and the
// historical v2 encoder. New formats validate it directly, without serializing
// an intermediate historical JSON record merely to invoke its checks.
func intervalWire(r Record) v2Record {
	w := v2Record{r.Schema, r.TS, r.Tool, r.Verb, r.Role, r.Boundary, r.WallMs, r.CPU, r.CPUMs, r.Exit, r.Result, r.Phases, r.PhaseOverlap, r.Counters, r.Unavailable, r.SourceSHA, r.Install}
	if w.Phases == nil {
		w.Phases = []PhaseStat{}
	}
	if w.Counters == nil {
		w.Counters = map[Counter]int64{}
	}
	if w.Unavailable == nil {
		w.Unavailable = []Counter{}
	}
	return w
}

// UnmarshalJSON decodes one line strictly as the schema it names: an unknown key, an unknown
// schema or an invalid perf-v2 enumeration/coverage is an error, so a reader skips the line
// instead of reinterpreting it.
func (r *Record) UnmarshalJSON(b []byte) error {
	var head struct {
		Schema schemaDiscriminator `json:"schema"`
	}
	if err := json.Unmarshal(b, &head); err != nil {
		return err
	}
	// encoding/json accepts case aliases and repeated struct members. Remember
	// EVERY decoded discriminator, so a later legacy alias cannot downgrade a
	// v4 envelope before its exact-key/duplicate-aware validator sees it.
	if head.Schema.sawV4 {
		return unmarshalV4(b, r)
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	switch head.Schema.value {
	case SchemaV4:
		return unmarshalV4(b, r)
	case SchemaV3:
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(b, &fields); err != nil {
			return err
		}
		var v V3Data
		if !strictJSON(fields["v3"], &v) {
			return ErrSchemaReuse
		}
		delete(fields, "v3")
		fields["schema"], _ = json.Marshal(SchemaV2)
		old, err := json.Marshal(fields)
		if err != nil {
			return err
		}
		var base Record
		if err = json.Unmarshal(old, &base); err != nil {
			return err
		}
		base.Schema = SchemaV3
		base.V3 = &v
		if err = validV3(base); err != nil {
			return err
		}
		*r = base
		return nil
	case Schema:
		var l legacyRecord
		if err := dec.Decode(&l); err != nil {
			return err
		}
		*r = Record{Schema: l.Schema, TS: l.TS, Tool: l.Tool, Verb: l.Verb, Role: l.Role, WallMs: l.WallMs,
			CPUMs: l.CPUMs, Exit: l.Exit, Result: l.Result, ChildProcs: l.ChildProcs, ChildWallMs: l.ChildWallMs,
			ForgeCalls: l.ForgeCalls, SourceSHA: l.SourceSHA, Install: l.Install}
		return nil
	case SchemaV2:
		var w v2Record
		if err := dec.Decode(&w); err != nil {
			return err
		}
		if w.Phases == nil || w.Counters == nil || w.Unavailable == nil {
			return ErrSchemaReuse
		}
		if err := validV2(w); err != nil {
			return err
		}
		*r = Record{Schema: w.Schema, TS: w.TS, Tool: w.Tool, Verb: w.Verb, Role: w.Role, WallMs: w.WallMs,
			CPUMs: w.CPUMs, Exit: w.Exit, Result: w.Result, SourceSHA: w.SourceSHA, Install: w.Install,
			Boundary: w.Boundary, CPU: w.CPU, Phases: w.Phases, PhaseOverlap: w.PhaseOverlap,
			Counters: w.Counters, Unavailable: w.Unavailable}
		return nil
	default:
		return fmt.Errorf("perfrec: unknown schema %q", head.Schema.value)
	}
}

type schemaDiscriminator struct {
	value string
	sawV4 bool
}

func (s *schemaDiscriminator) UnmarshalJSON(b []byte) error {
	var value string
	if err := json.Unmarshal(b, &value); err != nil {
		return err
	}
	s.value = value
	s.sawV4 = s.sawV4 || value == SchemaV4
	return nil
}

// Decode parses one day-file line as either schema (Record.UnmarshalJSON).
func Decode(line []byte) (Record, error) {
	var r Record
	err := json.Unmarshal(line, &r)
	return r, err
}

// validV2 checks every perf-v2 enumeration and the coverage partition: each roster counter
// appears exactly once, in counters or in unavailable; cpuMs is present iff CPU is measured;
// phases are bounded, named by label shape and non-negative.
func validV2(w v2Record) error {
	switch w.Boundary {
	case BoundaryProcess, BoundaryCaller:
	default:
		return fmt.Errorf("perfrec: unknown boundary %q", w.Boundary)
	}
	switch w.CPU {
	case CPUMeasured:
		if w.CPUMs == nil || *w.CPUMs < 0 {
			return errors.New("perfrec: measured CPU without a non-negative cpuMs")
		}
	case CPUUnsupported, CPUFailed, CPUMissing:
		if w.CPUMs != nil {
			return errors.New("perfrec: cpuMs present but CPU not measured")
		}
	default:
		return fmt.Errorf("perfrec: unknown cpu state %q", w.CPU)
	}
	if w.WallMs < 0 {
		return errors.New("perfrec: negative wallMs")
	}
	seen := make([]bool, len(counterRoster))
	for c, n := range w.Counters {
		i := counterIndex(c)
		if i < 0 || n < 0 || seen[i] {
			return fmt.Errorf("perfrec: invalid counter %q", c)
		}
		seen[i] = true
	}
	for _, c := range w.Unavailable {
		i := counterIndex(c)
		if i < 0 || seen[i] {
			return fmt.Errorf("perfrec: invalid unavailable counter %q", c)
		}
		seen[i] = true
	}
	for i, ok := range seen {
		if !ok {
			return fmt.Errorf("perfrec: counter %q has no coverage", counterRoster[i])
		}
	}
	if len(w.Phases) > MaxPhases+1 {
		return errors.New("perfrec: too many phases")
	}
	names := map[string]bool{}
	for _, p := range w.Phases {
		if Label(p.Name) != p.Name || names[p.Name] || p.N < 1 || p.Ms < 0 {
			return fmt.Errorf("perfrec: invalid phase %q", p.Name)
		}
		names[p.Name] = true
	}
	return nil
}
