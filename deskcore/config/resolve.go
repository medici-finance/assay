package config

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/medici-finance/assay/deskcore/domain"
)

// Layer is where a value came from.
type Layer string

// Layers, highest precedence first.
const (
	LayerEnv     Layer = "env"
	LayerConfig  Layer = "config"
	LayerDefault Layer = "default"
)

// KnobFile is the knobs section of the operator's config file, already decoded by the caller.
// Values are the strings the operator wrote; Resolve parses and bounds-checks them.
type KnobFile struct {
	// Source names the file for provenance, for example "cell.yaml"; empty reads as "file".
	Source string
	// Knobs sets a knob for every role: knob name -> value.
	Knobs map[string]string
	// Roles sets a per-role knob for one role: role -> knob name -> value.
	Roles map[string]map[string]string
}

// BoundsError refuses a value outside its knob's bounds.
type BoundsError struct {
	Knob, Role, Value string
	Layer             Layer
	Source            string
	Min, Max          string
}

func (e *BoundsError) Error() string {
	return fmt.Sprintf("knob %s%s: value %s from %s %s is outside its bounds [%s, %s]",
		e.Knob, roleSuffix(e.Role), e.Value, e.Layer, e.Source, e.Min, e.Max)
}

// ParseError refuses a value that is not a well-formed value for its knob.
type ParseError struct {
	Knob, Role, Value string
	Layer             Layer
	Source            string
	Reason            string
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("knob %s%s: value %q from %s %s: %s", e.Knob, roleSuffix(e.Role), e.Value, e.Layer, e.Source, e.Reason)
}

// UnknownKnobError refuses a setting that names no knob, or names a per-role setting for a knob
// that is not per-role. An unknown setting is refused rather than ignored, so a misspelt limit
// cannot silently leave the default in force.
type UnknownKnobError struct {
	Name   string
	Layer  Layer
	Source string
	Reason string
}

func (e *UnknownKnobError) Error() string {
	return fmt.Sprintf("%s %s: %s", e.Layer, e.Source, e.Reason)
}

func roleSuffix(role string) string {
	if role == "" {
		return ""
	}
	return "[" + role + "]"
}

// setting is one parsed value from one layer.
type setting struct {
	layer  Layer
	source string
	raw    string
	value  int64
}

type knobState struct {
	knob      Knob
	envGlobal *setting
	envRole   map[string]*setting
	cfgGlobal *setting
	cfgRole   map[string]*setting
}

// Resolved holds every knob's layers after resolution. It is immutable.
type Resolved struct {
	order []string
	knobs map[string]*knobState
}

// Value is one knob's resolved value with its provenance.
type Value struct {
	Knob   Knob
	Role   string
	Value  int64
	Layer  Layer
	Source string
	// chain is every layer in precedence order, set or not, for String.
	chain []chainEntry
}

type chainEntry struct {
	layer  Layer
	role   string // set for a per-role entry
	source string
	set    bool
	value  int64
}

func (c chainEntry) label() string { return string(c.layer) + roleSuffix(c.role) }

// Duration returns the value of a Duration knob.
func (v Value) Duration() time.Duration { return time.Duration(v.Value) }

// Off reports whether v is a budget or rate set to 0, which means no limit is enforced.
func (v Value) Off() bool { return v.Knob.limitClass() && v.Value == 0 }

// String renders the value with its provenance, for example
// "issue.file.rate = 10 (env ASSAY_DESK_ISSUE_FILE_RATE; config 0; default 0)".
// The winning layer is named with its source; every other layer shows its value or "unset".
// A per-role layer appears only when it is set, labelled with its role.
func (v Value) String() string {
	parts := make([]string, 0, len(v.chain))
	won := false
	for _, c := range v.chain {
		switch {
		case !won && c.set:
			p := c.label()
			if c.source != "" {
				p += " " + c.source
			}
			parts = append(parts, p)
			won = true
		case c.set:
			parts = append(parts, c.label()+" "+v.Knob.format(c.value))
		case c.role == "":
			parts = append(parts, c.label()+" unset")
		}
	}
	return fmt.Sprintf("%s%s = %s (%s)", v.Knob.Name, roleSuffix(v.Role), v.Knob.format(v.Value), strings.Join(parts, "; "))
}

// EnvFromList turns a list of KEY=VALUE strings (the shape a process environment comes in)
// into the map Resolve takes. Only ASSAY_DESK_ variables are kept.
func EnvFromList(list []string) map[string]string {
	out := map[string]string{}
	for _, kv := range list {
		k, v, ok := strings.Cut(kv, "=")
		if ok && strings.HasPrefix(k, envPrefix) {
			out[k] = v
		}
	}
	return out
}

// Resolve resolves table against env and file. It reports every refusal at once (joined), and
// returns no Resolved when there is any. Precedence: env, then config file, then default; within
// each layer a per-role setting beats the setting for every role.
func Resolve(env map[string]string, file KnobFile, table []Knob) (*Resolved, error) {
	if err := validateTable(table); err != nil {
		return nil, err
	}
	r := &Resolved{knobs: map[string]*knobState{}}
	byEnv := map[string]*knobState{}
	for _, k := range table {
		st := &knobState{knob: k, envRole: map[string]*setting{}, cfgRole: map[string]*setting{}}
		r.knobs[k.Name] = st
		r.order = append(r.order, k.Name)
		byEnv[k.EnvName()] = st
	}
	var errs []error
	fileSource := file.Source
	if fileSource == "" {
		fileSource = "file"
	}

	for name, raw := range env {
		if !strings.HasPrefix(name, envPrefix) {
			continue
		}
		base, roleRaw, perRole := strings.Cut(name, roleSep)
		st, ok := byEnv[base]
		if !ok {
			errs = append(errs, &UnknownKnobError{Name: name, Layer: LayerEnv, Source: name, Reason: "names no knob"})
			continue
		}
		if !perRole {
			st.envGlobal = parse(st.knob, "", raw, LayerEnv, name, &errs)
			continue
		}
		role, err := envRole(roleRaw)
		if err != nil || !st.knob.PerRole {
			reason := fmt.Sprintf("knob %s is not set per role", st.knob.Name)
			if err != nil {
				reason = err.Error()
			}
			errs = append(errs, &UnknownKnobError{Name: name, Layer: LayerEnv, Source: name, Reason: reason})
			continue
		}
		st.envRole[role] = parse(st.knob, role, raw, LayerEnv, name, &errs)
	}

	for name, raw := range file.Knobs {
		st, ok := r.knobs[name]
		src := fileSource + " knobs." + name
		if !ok {
			errs = append(errs, &UnknownKnobError{Name: name, Layer: LayerConfig, Source: src, Reason: "names no knob"})
			continue
		}
		st.cfgGlobal = parse(st.knob, "", raw, LayerConfig, src, &errs)
	}
	for role, set := range file.Roles {
		if err := domain.Role(role).Validate(); err != nil {
			errs = append(errs, &UnknownKnobError{Name: role, Layer: LayerConfig, Source: fileSource + " roles", Reason: err.Error()})
			continue
		}
		for name, raw := range set {
			st, ok := r.knobs[name]
			src := fileSource + " roles." + role + "." + name
			switch {
			case !ok:
				errs = append(errs, &UnknownKnobError{Name: name, Layer: LayerConfig, Source: src, Reason: "names no knob"})
			case !st.knob.PerRole:
				errs = append(errs, &UnknownKnobError{Name: name, Layer: LayerConfig, Source: src, Reason: fmt.Sprintf("knob %s is not set per role", name)})
			default:
				st.cfgRole[role] = parse(st.knob, role, raw, LayerConfig, src, &errs)
			}
		}
	}
	if len(errs) > 0 {
		return nil, errors.Join(sortErrs(errs)...)
	}
	if err := r.crossCheck(); err != nil {
		return nil, err
	}
	return r, nil
}

// crossCheck refuses combinations that are each in bounds but unusable together.
func (r *Resolved) crossCheck() error {
	ttl, okT := r.knobs["lease.ttl"]
	renew, okR := r.knobs["lease.renew_every"]
	if !okT || !okR {
		return nil
	}
	t, rn := r.value(ttl, ""), r.value(renew, "")
	if rn.Value >= t.Value {
		return fmt.Errorf("knob lease.renew_every (%s) must be shorter than lease.ttl (%s)", rn, t)
	}
	return nil
}

func envRole(raw string) (string, error) {
	role := strings.ReplaceAll(strings.ToLower(raw), "_", "-")
	if err := domain.Role(role).Validate(); err != nil {
		return "", fmt.Errorf("role suffix %q: %v", raw, err)
	}
	return role, nil
}

func parse(k Knob, role, raw string, layer Layer, source string, errs *[]error) *setting {
	v, err := parseValue(k, strings.TrimSpace(raw))
	if err != nil {
		*errs = append(*errs, &ParseError{Knob: k.Name, Role: role, Value: raw, Layer: layer, Source: source, Reason: err.Error()})
		return nil
	}
	if v < k.Min || v > k.Max {
		*errs = append(*errs, &BoundsError{Knob: k.Name, Role: role, Value: raw, Layer: layer, Source: source, Min: k.format(k.Min), Max: k.format(k.Max)})
		return nil
	}
	return &setting{layer: layer, source: source, raw: raw, value: v}
}

func parseValue(k Knob, s string) (int64, error) {
	if k.Kind == Count {
		v, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return 0, errors.New("want a whole number")
		}
		return v, nil
	}
	if s == "" {
		return 0, errors.New("want a duration with a unit, for example 90s, 15m, 2h or 90d")
	}
	if n, ok := strings.CutSuffix(s, "d"); ok {
		days, err := strconv.ParseInt(n, 10, 64)
		if err != nil || days < 0 || days > 100000 {
			return 0, errors.New("want a whole number of days, for example 90d")
		}
		return int64(time.Duration(days) * day), nil
	}
	if _, err := strconv.ParseInt(s, 10, 64); err == nil {
		return 0, errors.New("a bare number is ambiguous for a duration; add a unit (s, m, h or d)")
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, errors.New("want a duration with a unit, for example 90s, 15m, 2h or 90d")
	}
	return int64(d), nil
}

func sortErrs(errs []error) []error {
	out := append([]error(nil), errs...)
	// Insertion sort by message: map iteration order must not change the report.
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j].Error() < out[j-1].Error(); j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}

// value builds a knob's Value for role ("" for a knob that is not per-role).
func (r *Resolved) value(st *knobState, role string) Value {
	k := st.knob
	var layers []*setting
	var chain []chainEntry
	add := func(layer Layer, scope string, s *setting) {
		e := chainEntry{layer: layer, role: scope}
		if s != nil {
			e.set, e.value, e.source = true, s.value, s.source
		}
		chain = append(chain, e)
		layers = append(layers, s)
	}
	if role != "" {
		add(LayerEnv, role, st.envRole[role])
	}
	add(LayerEnv, "", st.envGlobal)
	if role != "" {
		add(LayerConfig, role, st.cfgRole[role])
	}
	add(LayerConfig, "", st.cfgGlobal)
	add(LayerDefault, "", &setting{layer: LayerDefault, value: k.Default})
	v := Value{Knob: k, Role: role, chain: chain}
	for _, s := range layers {
		if s != nil {
			v.Value, v.Layer, v.Source = s.value, s.layer, s.source
			break
		}
	}
	return v
}

// Get returns knob name's value. A per-role knob must be read for one role; a knob that is not
// per-role must be read with role "".
func (r *Resolved) Get(name, role string) (Value, error) {
	st, ok := r.knobs[name]
	if !ok {
		return Value{}, fmt.Errorf("knob %q does not exist", name)
	}
	if st.knob.PerRole {
		if err := domain.Role(role).Validate(); err != nil {
			return Value{}, fmt.Errorf("knob %s is read per role: %v", name, err)
		}
	} else if role != "" {
		return Value{}, fmt.Errorf("knob %s is not per role; read it with an empty role", name)
	}
	return r.value(st, role), nil
}

// Values returns every knob's value for display, in table order. Per-role knobs are included
// only when role is non-empty.
func (r *Resolved) Values(role string) []Value {
	var out []Value
	for _, name := range r.order {
		st := r.knobs[name]
		switch {
		case st.knob.PerRole && role != "":
			out = append(out, r.value(st, role))
		case !st.knob.PerRole:
			out = append(out, r.value(st, ""))
		}
	}
	return out
}

// CounterKey is the key a per-role budget or rate is counted under. Every role has its own
// counter: one role exhausting its budget never spends another's.
func CounterKey(knob, role string) (string, error) {
	if err := domain.Role(role).Validate(); err != nil {
		return "", fmt.Errorf("counter for %s: %v", knob, err)
	}
	return "role/" + role + "/" + knob, nil
}

// Narrow applies a request's own, tighter value for a budget, rate or capacity knob. A request
// may only narrow: the result is the smaller of the resolved value and requested, and when the
// resolved budget or rate is off (0) the requested value applies. A request can never widen a
// limit or switch one off.
func (r *Resolved) Narrow(name, role string, requested int64) (int64, error) {
	v, err := r.Get(name, role)
	if err != nil {
		return 0, err
	}
	if !v.Knob.narrowable() {
		return 0, fmt.Errorf("knob %s (%s) cannot be narrowed by a request", name, v.Knob.Class)
	}
	if v.Knob.limitClass() && requested == 0 && !v.Off() {
		return 0, fmt.Errorf("knob %s: a request cannot switch off a limit of %s", name, v.Knob.format(v.Value))
	}
	if requested < v.Knob.Min || requested > v.Knob.Max {
		return 0, fmt.Errorf("knob %s: requested %s is outside its bounds [%s, %s]", name, v.Knob.format(requested), v.Knob.format(v.Knob.Min), v.Knob.format(v.Knob.Max))
	}
	if v.Off() || requested < v.Value {
		return requested, nil
	}
	return v.Value, nil
}
