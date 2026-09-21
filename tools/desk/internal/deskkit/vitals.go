package deskkit

// vitals.go — the self-reported "resource" block behind example-stream/13: a per-tick
// vitals reading (tokens burned, context-% used, session age, subagent count, model) that
// only the desk session itself can know, written onto the SAME roster beacon deskroster
// and deskack already co-own (ackbeacon.go's AppendAck is this write's sibling).
//
// WHY A SEPARATE MERGE FUNCTION, NOT A NEW FIELD ON deskroster's TYPED Beacon.
// deskroster's cmdSet/loadBeacon/saveBeacon round-trip through a typed Go struct; adding a
// field there is one more place that has to remember to preserve it on every future
// rewrite. This write instead follows AppendAck's shape exactly: load the beacon as raw
// JSON keys, touch only `resource` (and `session`, on a first write), and leave every
// other key — acks, open_work, role, updated — byte-for-byte as found. Two independent
// writers (deskack, this) both proven not to clobber deskroster's fields is a stronger
// property than one typed struct trusted to remember five fields forever.
//
// THREE-STATE, NEVER A FABRICATED ZERO (docs/three-state-instrument-rule.md). Each field
// on ResourceVitals is a *VitalField: nil (the Go zero value — a flag simply not passed)
// marshals to JSON null ("not collected this tick"); a CouldNotCheckVital() marshals to
// the string "could-not-check" ("the source read failed"); a Measured*() field marshals
// to its own value, INCLUDING a measured zero — subagents_spawned: 0 is a real reading and
// must never be conflated with "unknown".

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// VitalField is the three-state wrapper one resource field uses.
//
// A nil *VitalField marshals to JSON null without this type's MarshalJSON ever running —
// encoding/json emits null directly for a nil pointer implementing Marshaler, per its own
// documented behaviour — which is exactly the "field not set this tick" case.
type VitalField struct {
	// CouldNotCheck marks a field whose source the caller tried to read and failed —
	// distinct from Value being nil, which means no read was attempted at all.
	CouldNotCheck bool
	// Value holds the measured reading (int64, float64, or string depending on the
	// field). nil here (on a non-nil *VitalField) is not a shape callers of this
	// package construct; the constructors below always set it for a measured field.
	Value interface{}
}

// MarshalJSON implements json.Marshaler. Only ever invoked for a non-nil *VitalField —
// encoding/json special-cases a nil pointer to null before consulting the Marshaler
// interface at all.
func (f *VitalField) MarshalJSON() ([]byte, error) {
	if f.CouldNotCheck {
		return json.Marshal("could-not-check")
	}
	if f.Value == nil {
		return []byte("null"), nil
	}
	return json.Marshal(f.Value)
}

// MeasuredInt, MeasuredFloat and MeasuredString build a MEASURED VitalField — the
// caller's actual reading, which may legitimately be zero or empty; that is still a
// measured value, never "unknown".
func MeasuredInt(v int64) *VitalField     { return &VitalField{Value: v} }
func MeasuredFloat(v float64) *VitalField { return &VitalField{Value: v} }
func MeasuredString(v string) *VitalField { return &VitalField{Value: v} }

// CouldNotCheckVital builds a VitalField whose source read failed — distinct from both a
// measured value and an unset (null) field.
func CouldNotCheckVital() *VitalField { return &VitalField{CouldNotCheck: true} }

// ResourceVitals is the self-report block: five three-state fields. A field left at its
// Go zero value (nil) was not collected this tick and marshals to null; see the
// constructors above for the other two states.
type ResourceVitals struct {
	Tokens            *VitalField `json:"tokens"`
	ContextPctUsed    *VitalField `json:"context_pct_used"`
	SessionAgeSeconds *VitalField `json:"session_age_seconds"`
	SubagentsSpawned  *VitalField `json:"subagents_spawned"`
	Model             *VitalField `json:"model"`
}

// ValidSessionSegment reports whether name is safe to join as a SINGLE path segment
// under the roster directory: non-empty, and containing neither a path separator nor a
// ".." traversal component.
//
// Security review finding S-1 (example-stream/13): nothing in `deskroster set` binds a
// beacon to the session it names — the caller passes an arbitrary string that gets joined
// unvalidated into roster/<session>.json. That was harmless while the beacon was a status
// display; once example-stream/14 lets a GRACE/HARD-RECYCLE reading on this beacon arm
// an involuntary stop, the write becomes a control input, and the join needs a named,
// checked boundary rather than an implicit one. `deskroster set` calls this on every
// resolved session name before it reaches a beacon path and refuses (exit 5) rather than
// silently joining a name that fails the check.
func ValidSessionSegment(name string) bool {
	if name == "" || name == "." || name == ".." {
		return false
	}
	if strings.ContainsAny(name, "/\\") {
		return false
	}
	return !strings.Contains(name, "..")
}

// MergeResourceVitals writes v's resource block onto session's roster beacon
// (<StateDir>/roster/<session>.json — the SAME file AppendAck and deskroster's own writes
// share), preserving every field this binary does not know about. It mirrors AppendAck
// exactly: load the beacon as raw keys, touch only `resource` and `session`, and fail
// closed on a parse error rather than silently blanking another writer's fields.
func MergeResourceVitals(session string, v ResourceVitals) (path string, err error) {
	path, err = AckBeaconPath(session)
	if err != nil {
		return "", err
	}

	obj := map[string]json.RawMessage{}
	if data, rerr := os.ReadFile(path); rerr == nil {
		if len(data) > 0 {
			if uerr := json.Unmarshal(data, &obj); uerr != nil {
				return path, Unverifiable("cannot parse the roster beacon at "+path+
					" — refusing to overwrite it and lose another writer's fields", uerr)
			}
		}
	} else if !os.IsNotExist(rerr) {
		return path, Unverifiable("cannot read the roster beacon at "+path, rerr)
	}

	resRaw, merr := json.Marshal(v)
	if merr != nil {
		return path, Unverifiable("cannot encode the resource vitals block", merr)
	}
	obj["resource"] = resRaw
	// Keep the session field present so a beacon this write creates is well-formed for
	// deskroster's reader (which keys the file by its session name anyway) — the same
	// reasoning AppendAck's own session-stamp uses.
	if _, ok := obj["session"]; !ok {
		sraw, _ := json.Marshal(session)
		obj["session"] = sraw
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return path, Unverifiable("cannot create the roster directory "+dir, err)
	}
	out, merr := json.MarshalIndent(obj, "", "  ")
	if merr != nil {
		return path, Unverifiable("cannot encode the roster beacon", merr)
	}
	if werr := os.WriteFile(path, append(out, '\n'), 0o600); werr != nil {
		return path, Unverifiable("cannot write the roster beacon at "+path, werr)
	}
	return path, nil
}
