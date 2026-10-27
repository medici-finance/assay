package perfrec

// The v4 codec owns an additive, bounded envelope, not a raw JSON relay.
// Historical schema validators remain the owners of every observation value.
import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"io"
	"reflect"
	"regexp"
	"sort"
	"strings"
)

const SchemaV4 = "perf-v4"
const MaxV4LineBytes = 1 << 20 // including the JSONL newline; strictly smaller
const MaxV4Extensions = 8
const MaxV4Depth = 32
const MaxV4ExtensionVersion = 2147483647

var extensionName = regexp.MustCompile(`^[a-z][a-z0-9-]{0,31}$`)
var errV4 = errors.New("perfrec: invalid v4 record")

// V4Attribution reuses the v3 field vocabulary without its invocation ID.
// The identity belongs exclusively to the v4 core.
type V4Attribution struct {
	SessionID          string                    `json:"sessionId,omitempty"`
	AgentID            string                    `json:"agentId,omitempty"`
	ParentAgentID      string                    `json:"parentAgentId,omitempty"`
	ParentInvocationID string                    `json:"parentInvocationId,omitempty"`
	DeskRole           string                    `json:"deskRole,omitempty"`
	Role               string                    `json:"role,omitempty"`
	Work               []WorkRef                 `json:"work,omitempty"`
	Coverage           map[ContextField]Coverage `json:"coverage"`
}
type V4Requests struct {
	Requests        []RequestStats `json:"requests"`
	RequestDrops    int64          `json:"requestDrops"`
	RequestCoverage Coverage       `json:"requestCoverage"`
}
type v4Process struct {
	Phases       []PhaseStat       `json:"phases"`
	PhaseOverlap bool              `json:"phaseOverlap"`
	Counters     map[Counter]int64 `json:"counters"`
	Unavailable  []Counter         `json:"unavailable"`
}
type v4Query struct {
	Query *QueryObservation `json:"query,omitempty"`
}

// V4Data exposes only registered typed facets. Missing process/request/query
// facets remain unavailable. Core role alone never grants attribution coverage.
type V4Data struct {
	Profile          Profile
	Kind             ObservationKind
	InvocationID     string
	ProcessAvailable bool
	Attribution      *V4Attribution
	Requests         *V4Requests
	QueryAvailable   bool
	Query            *QueryObservation
	unsupported      int
	// Canonical input fingerprint preserves conflict detection for discarded
	// unknown facets, without retaining their names or payloads.
	fingerprint [32]byte
}

func (v *V4Data) UnsupportedExtensions() int {
	if v == nil {
		return 0
	}
	return v.unsupported
}

type v4Core struct {
	Schema       string                 `json:"schema"`
	TS           string                 `json:"ts"`
	Tool         string                 `json:"tool"`
	Verb         string                 `json:"verb"`
	Role         string                 `json:"role"`
	Boundary     string                 `json:"boundary"`
	WallMs       int64                  `json:"wallMs"`
	CPU          string                 `json:"cpu"`
	CPUMs        *int64                 `json:"cpuMs,omitempty"`
	Exit         int                    `json:"exit"`
	Result       string                 `json:"result"`
	SourceSHA    string                 `json:"sourceSHA"`
	Install      string                 `json:"install"`
	Profile      Profile                `json:"profile"`
	Kind         ObservationKind        `json:"observationKind"`
	InvocationID string                 `json:"invocationId"`
	Extensions   map[string]v4Extension `json:"extensions"`
}
type v4Extension struct {
	Version int             `json:"version"`
	Data    json.RawMessage `json:"data"`
}

// ToV4 converts an already measured v3 record. It performs no measurement,
// context resolution, consent check or append, and cannot introduce raw facets.
func ToV4(r Record) (Record, error) {
	if r.Schema != SchemaV3 || r.V4 != nil {
		return Record{}, ErrSchemaReuse
	}
	if err := validV3(r); err != nil {
		return Record{}, err
	}
	if err := validV2(intervalWire(r)); err != nil {
		return Record{}, err
	}
	v := cloneV3(r.V3)
	if r.Phases == nil {
		r.Phases = []PhaseStat{}
	}
	if r.Counters == nil {
		r.Counters = map[Counter]int64{}
	}
	if r.Unavailable == nil {
		r.Unavailable = []Counter{}
	}
	if v.Requests == nil {
		v.Requests = []RequestStats{}
	}
	r.V3 = nil
	r.Schema = SchemaV4
	r.V4 = &V4Data{Profile: v.Profile, Kind: v.Kind, InvocationID: v.Attribution.InvocationID,
		ProcessAvailable: true, Requests: &V4Requests{v.Requests, v.RequestDrops, v.RequestCoverage}, QueryAvailable: true, Query: v.Query}
	if v.Profile == ProfileWorkflow {
		a := v.Attribution
		r.V4.Attribution = &V4Attribution{a.SessionID, a.AgentID, a.ParentAgentID, a.ParentInvocationID, a.DeskRole, a.Role, a.Work, a.Coverage}
	}
	return r, nil
}

// v4Observation adapts supported facets to the ONE historical validation and
// reduction model. It is internal only: missing attribution is never persisted
// as a fabricated observation or allowed through a workflow filter.
func v4Observation(r Record) Record {
	v := r.V4
	n := r
	n.Schema, n.V4 = SchemaV3, nil
	a := Attribution{InvocationID: v.InvocationID, Coverage: map[ContextField]Coverage{}}
	for _, k := range contextFields {
		if v.Profile == ProfileWorkflow {
			a.Coverage[k] = CoverageMissing
		} else {
			a.Coverage[k] = CoverageUnsupported
		}
	}
	if x := v.Attribution; x != nil {
		a = Attribution{v.InvocationID, x.SessionID, x.AgentID, x.ParentAgentID, x.ParentInvocationID, x.DeskRole, x.Role, x.Work, x.Coverage}
	}
	n.Role = a.Role
	if n.Role == "" {
		n.Role = RoleNone
	}
	n.V3 = &V3Data{Profile: v.Profile, Attribution: a, Kind: v.Kind, Requests: []RequestStats{}, RequestCoverage: CoverageMissing, Query: v.Query}
	if q := v.Requests; q != nil {
		n.V3.Requests = q.Requests
		n.V3.RequestDrops = q.RequestDrops
		n.V3.RequestCoverage = q.RequestCoverage
	}
	if !v.ProcessAvailable {
		n.Phases = []PhaseStat{}
		n.PhaseOverlap = false
		n.Counters = map[Counter]int64{}
		n.Unavailable = Counters()
	}
	return n
}
func validV4(r Record) error {
	return validateV4(r, false)
}
func validateV4(r Record, unboundInstall bool) error {
	v := r.V4
	if r.Schema != SchemaV4 || v == nil || r.V3 != nil || !IDValid(v.InvocationID) || (!unboundInstall && !IDValid(r.Install)) || r.hasLegacyCounters() {
		return errV4
	}
	if v.Profile != ProfileTool && v.Profile != ProfileWorkflow || r.Role != RoleNone && !roleValid(r.Role) {
		return errV4
	}
	if v.Profile == ProfileTool && (r.Role != RoleNone || v.Attribution != nil) {
		return errV4
	}
	if a := v.Attribution; a != nil && r.Role != a.Role && !(r.Role == RoleNone && a.Role == "") {
		return errV4
	}
	if !v.ProcessAvailable && (r.Phases != nil || r.Counters != nil || r.Unavailable != nil || r.PhaseOverlap) {
		return errV4
	}
	if v.ProcessAvailable && (r.Phases == nil || r.Counters == nil || r.Unavailable == nil) {
		return errV4
	}
	if v.Requests != nil && v.Requests.Requests == nil {
		return errV4
	}
	if !v.QueryAvailable && v.Query != nil {
		return errV4
	}
	n := v4Observation(r)
	if err := validV3(n); err != nil {
		return errV4
	}
	if err := validV2(intervalWire(n)); err != nil {
		return errV4
	}
	return nil
}
func marshalV4(r Record) ([]byte, error) {
	if err := validV4(r); err != nil {
		return nil, err
	}
	v := r.V4
	w := v4Core{r.Schema, r.TS, r.Tool, r.Verb, r.Role, r.Boundary, r.WallMs, r.CPU, r.CPUMs, r.Exit, r.Result, r.SourceSHA, r.Install, v.Profile, v.Kind, v.InvocationID, map[string]v4Extension{}}
	put := func(k string, x any) error {
		b, e := json.Marshal(x)
		if e == nil {
			w.Extensions[k] = v4Extension{1, b}
		}
		return e
	}
	if v.ProcessAvailable {
		if e := put("process", v4Process{r.Phases, r.PhaseOverlap, r.Counters, r.Unavailable}); e != nil {
			return nil, errV4
		}
	}
	if v.Attribution != nil {
		if e := put("attribution", v.Attribution); e != nil {
			return nil, errV4
		}
	}
	if v.Requests != nil {
		if e := put("requests", v.Requests); e != nil {
			return nil, errV4
		}
	}
	if v.QueryAvailable {
		if e := put("query", v4Query{v.Query}); e != nil {
			return nil, errV4
		}
	}
	b, e := json.Marshal(w)
	if e != nil || len(b)+1 >= MaxV4LineBytes {
		return nil, errV4
	}
	return b, nil
}

// boundedJSON rejects duplicate keys before maps can erase them. The same
// structural pass covers unknown facets, including their depth, without echo.
func boundedJSON(b []byte) (map[string]any, error) {
	if len(b)+1 >= MaxV4LineBytes {
		return nil, errV4
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.UseNumber()
	var read func(int) (any, error)
	read = func(depth int) (any, error) {
		t, e := d.Token()
		if e != nil {
			return nil, errV4
		}
		delim, ok := t.(json.Delim)
		if !ok {
			return t, nil
		}
		if depth >= MaxV4Depth {
			return nil, errV4
		}
		switch delim {
		case '{':
			m := map[string]any{}
			for d.More() {
				k, e := d.Token()
				if e != nil {
					return nil, errV4
				}
				s, ok := k.(string)
				if !ok {
					return nil, errV4
				}
				if _, ok = m[s]; ok {
					return nil, errV4
				}
				x, e := read(depth + 1)
				if e != nil {
					return nil, e
				}
				m[s] = x
			}
			if t, e = d.Token(); e != nil || t != json.Delim('}') {
				return nil, errV4
			}
			return m, nil
		case '[':
			a := []any{}
			for d.More() {
				x, e := read(depth + 1)
				if e != nil {
					return nil, e
				}
				a = append(a, x)
			}
			if t, e = d.Token(); e != nil || t != json.Delim(']') {
				return nil, errV4
			}
			return a, nil
		}
		return nil, errV4
	}
	x, e := read(0)
	if e != nil {
		return nil, e
	}
	if _, e = d.Token(); e != io.EOF {
		return nil, errV4
	}
	m, ok := x.(map[string]any)
	if !ok {
		return nil, errV4
	}
	return m, nil
}

// exactShape derives exact member spellings from the canonical typed structs,
// never from a second hand-maintained observation grammar. Optional members may
// be absent, but explicit null is never silently treated as missing coverage.
func exactShape(x any, t reflect.Type) bool {
	if x == nil {
		return false
	}
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	switch t.Kind() {
	case reflect.Struct:
		m, ok := x.(map[string]any)
		if !ok {
			return false
		}
		allowed := map[string]reflect.StructField{}
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			tag := strings.Split(f.Tag.Get("json"), ",")[0]
			if tag == "-" {
				continue
			}
			if tag == "" {
				tag = f.Name
			}
			allowed[tag] = f
			if !strings.Contains(f.Tag.Get("json"), ",omitempty") {
				if _, ok := m[tag]; !ok {
					return false
				}
			}
		}
		for k, v := range m {
			f, ok := allowed[k]
			if !ok || !exactShape(v, f.Type) {
				return false
			}
		}
	case reflect.Map:
		m, ok := x.(map[string]any)
		if !ok {
			return false
		}
		for _, v := range m {
			if !exactShape(v, t.Elem()) {
				return false
			}
		}
	case reflect.Slice, reflect.Array:
		a, ok := x.([]any)
		if !ok {
			return false
		}
		if t.Kind() == reflect.Array && len(a) != t.Len() {
			return false
		}
		for _, v := range a {
			if !exactShape(v, t.Elem()) {
				return false
			}
		}
	}
	return true
}
func typedV4(x any, out any) bool {
	if !exactShape(x, reflect.TypeOf(out)) {
		return false
	}
	b, e := json.Marshal(x)
	return e == nil && strictJSON(b, out)
}
func unmarshalV4(b []byte, out *Record) error {
	m, e := boundedJSON(b)
	if e != nil {
		return errV4
	}
	// Extensions are structurally checked separately; reflection otherwise sees
	// RawMessage as an array. No raw map survives this function.
	em, ok := m["extensions"].(map[string]any)
	if !ok || len(em) > MaxV4Extensions {
		return errV4
	}
	core := map[string]any{}
	for k, v := range m {
		core[k] = v
	}
	core["extensions"] = map[string]any{}
	var w v4Core
	if !typedV4(core, &w) {
		return errV4
	}
	r := Record{Schema: w.Schema, TS: w.TS, Tool: w.Tool, Verb: w.Verb, Role: w.Role, Boundary: w.Boundary, WallMs: w.WallMs, CPU: w.CPU, CPUMs: w.CPUMs, Exit: w.Exit, Result: w.Result, SourceSHA: w.SourceSHA, Install: w.Install}
	v := &V4Data{Profile: w.Profile, Kind: w.Kind, InvocationID: w.InvocationID}
	r.V4 = v
	for name, x := range em {
		if !extensionName.MatchString(name) {
			return errV4
		}
		ext, ok := x.(map[string]any)
		if !ok || len(ext) != 2 {
			return errV4
		}
		num, ok := ext["version"].(json.Number)
		if !ok {
			return errV4
		}
		version, e := num.Int64()
		if e != nil || version < 1 || version > MaxV4ExtensionVersion {
			return errV4
		}
		data, ok := ext["data"].(map[string]any)
		if !ok {
			return errV4
		}
		if version != 1 {
			v.unsupported++
			continue
		}
		switch name {
		case "process":
			var p v4Process
			if !typedV4(data, &p) {
				return errV4
			}
			r.Phases = p.Phases
			r.PhaseOverlap = p.PhaseOverlap
			r.Counters = p.Counters
			r.Unavailable = p.Unavailable
			v.ProcessAvailable = true
		case "attribution":
			var a V4Attribution
			if !typedV4(data, &a) {
				return errV4
			}
			v.Attribution = &a
		case "requests":
			var q V4Requests
			if !typedV4(data, &q) {
				return errV4
			}
			v.Requests = &q
		case "query":
			var q v4Query
			if !typedV4(data, &q) {
				return errV4
			}
			v.Query = q.Query
			v.QueryAvailable = true
		default:
			v.unsupported++
		}
	}
	if validV4(r) != nil {
		return errV4
	}
	// Fingerprint canonical complete input for identity conflicts, without making
	// untrusted optional payloads observable or retaining them in memory.
	if v.unsupported > 0 {
		canonical, _ := json.Marshal(m)
		v.fingerprint = sha256.Sum256(canonical)
	}
	*out = r
	return nil
}

// V4Summary adds facet coverage to the SAME observation reducer used for v3.
// It never includes historical records or invents request/query measurements.
type V4Summary struct {
	V3Summary
	UnsupportedExtensions  int `json:"unsupportedExtensions"`
	ProcessUnavailable     int `json:"processUnavailable"`
	AttributionUnavailable int `json:"attributionUnavailable"`
	RequestsUnavailable    int `json:"requestsUnavailable"`
	QueryUnavailable       int `json:"queryUnavailable"`
	QueryNotApplicable     int `json:"queryNotApplicable"`
}

func SelectV4(records []Record, permission Profile, f V3Filter) ([]Record, V3Selection, error) {
	var s V3Selection
	if filterUsed(f) && permission != ProfileWorkflow {
		return nil, s, errors.New("perfrec: workflow permission required")
	}
	if f.Work != nil && !validWork([]WorkRef{*f.Work}) {
		return nil, s, errors.New("perfrec: invalid work filter")
	}
	unique := map[string]Record{}
	encoded := map[string]string{}
	conflicts := map[string]bool{}
	for _, r := range records {
		if r.Schema != SchemaV4 {
			continue
		}
		b, e := json.Marshal(r)
		if e != nil {
			s.Invalid++
			continue
		}
		key := r.Install + ":" + r.V4.InvocationID
		signature := string(b)
		if r.V4.unsupported > 0 {
			signature += string(r.V4.fingerprint[:])
		}
		if old, ok := encoded[key]; ok {
			if old == signature {
				s.Duplicates++
			} else if !conflicts[key] {
				conflicts[key] = true
				s.Conflicts++
			}
			continue
		}
		encoded[key] = signature
		unique[key] = r
	}
	keys := make([]string, 0, len(unique))
	for k := range unique {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var kept []Record
	for _, k := range keys {
		if conflicts[k] {
			continue
		}
		r := unique[k]
		if filterUsed(f) {
			if r.V4.Profile != ProfileWorkflow || r.V4.Attribution == nil {
				continue
			}
			n := v4Observation(r)
			sel, _, e := SelectV3([]Record{n}, permission, f)
			if e != nil {
				return nil, s, e
			}
			if len(sel) == 0 {
				continue
			}
		}
		kept = append(kept, r)
	}
	return kept, s, nil
}
func ReduceV4(records []Record, permission Profile, dimension string, f V3Filter) (V4Summary, error) {
	var s V4Summary
	dimension, e := reductionDimension(permission, dimension)
	if e != nil {
		return s, e
	}
	kept, selection, e := SelectV4(records, permission, f)
	if e != nil {
		return s, e
	}
	var normalized []Record
	for _, r := range kept {
		v := r.V4
		s.UnsupportedExtensions += v.unsupported
		if !v.ProcessAvailable {
			s.ProcessUnavailable++
		}
		if v.Attribution == nil {
			s.AttributionUnavailable++
		}
		if v.Requests == nil {
			s.RequestsUnavailable++
		}
		if v.Kind == ObservationProcess || v.Kind == ObservationProvider {
			s.QueryNotApplicable++
		} else if v.Query == nil {
			s.QueryUnavailable++
		}
		normalized = append(normalized, v4Observation(r))
	}
	s.V3Summary, e = reduceSelectedObservations(normalized, selection, dimension)
	if e != nil {
		return V4Summary{}, e
	}
	s.Duplicates, s.Conflicts, s.Invalid = selection.Duplicates, selection.Conflicts, selection.Invalid
	return s, e
}
