package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Overrides is a cell's operator override table: the five knobs a human turns
// from the cockpit's Drive tab to slow a cell down without editing config and
// waiting for a deploy (the service contract). Every field is a POINTER, so an
// ABSENT key (nil) is "no override" and a PRESENT key is a set value — a plain
// value could not distinguish "leave the loop's own default" from "force this
// value", which is the whole distinction an override table exists to carry.
//
// # One meaning, three readers
//
// This is the SINGLE definition of the shape, deliberately in internal/domain
// (the pure leaf) rather than in any one consumer, because the same value is
// read by three components that must agree on it byte-for-byte: the HTTP write
// (internal/serve), the MCP read the loops call each tick (internal/mcp), and
// the record PR's YAML block (internal/mcp/github_filer.go, config/schema).
// internal/contract can import internal/domain and nothing heavier, so a wire
// type living here keeps the contract a leaf while every reader shares one
// struct — a rename in one is a compile error in the others, not a silent drift
// (the exec-tier-(b) hazard this brief was raised on).
//
// The json tags carry the HTTP/MCP wire; the yaml tags carry the cells.yaml
// `overrides:` block and the record PR. Both tag sets name the same key, so the
// value round-trips across the seam and the file with no per-reader mapping.
type Overrides struct {
	// DispatchWidth caps concurrent workers a cell dispatches (0–16; 0 = hold).
	DispatchWidth *int `json:"dispatch_width,omitempty" yaml:"dispatch_width,omitempty"`
	// HoldMerges pauses merges while true.
	HoldMerges *bool `json:"hold_merges,omitempty" yaml:"hold_merges,omitempty"`
	// PauseVerify pauses the verify loop while true.
	PauseVerify *bool `json:"pause_verify,omitempty" yaml:"pause_verify,omitempty"`
	// ModelFloor raises the cheapest model a loop may use to this ladder id. It
	// must be one of the ids the cell's own loops[].models already name.
	ModelFloor *string `json:"model_floor,omitempty" yaml:"model_floor,omitempty"`
	// QuietHours silences a cell over a daily window, `HH:MM-HH:MM <IANA zone>`.
	QuietHours *string `json:"quiet_hours,omitempty" yaml:"quiet_hours,omitempty"`
}

// overrideKeys is the closed key vocabulary, in a stable order. It is the ONE
// place the wire key names live; SetKeys, the record-PR title and the
// per-key table storage all read it, so a key added to the struct without a
// key here is invisible to them (and vice versa) — a mismatch a reviewer sees.
var overrideKeys = []string{"dispatch_width", "hold_merges", "pause_verify", "model_floor", "quiet_hours"}

// DispatchWidthMax is the inclusive ceiling on dispatch_width. Zero is a valid,
// meaningful value (hold all dispatch), so the range is [0, DispatchWidthMax].
const DispatchWidthMax = 16

// SetKeys returns the override keys that are PRESENT (non-nil), in the stable
// vocabulary order. It is what the record-PR title enumerates and what the
// idempotency hash and the per-key upsert iterate, so all three agree on which
// keys a table carries.
func (o Overrides) SetKeys() []string {
	var out []string
	if o.DispatchWidth != nil {
		out = append(out, "dispatch_width")
	}
	if o.HoldMerges != nil {
		out = append(out, "hold_merges")
	}
	if o.PauseVerify != nil {
		out = append(out, "pause_verify")
	}
	if o.ModelFloor != nil {
		out = append(out, "model_floor")
	}
	if o.QuietHours != nil {
		out = append(out, "quiet_hours")
	}
	return out
}

// IsEmpty reports whether no override is set.
func (o Overrides) IsEmpty() bool { return len(o.SetKeys()) == 0 }

// StringValue renders one key's set value as the string the per-key table row
// stores, or ("", false) when the key is not set. The table stores one row per
// (cell, key); this is the projection onto that row's value column.
func (o Overrides) StringValue(key string) (string, bool) {
	switch key {
	case "dispatch_width":
		if o.DispatchWidth != nil {
			return strconv.Itoa(*o.DispatchWidth), true
		}
	case "hold_merges":
		if o.HoldMerges != nil {
			return strconv.FormatBool(*o.HoldMerges), true
		}
	case "pause_verify":
		if o.PauseVerify != nil {
			return strconv.FormatBool(*o.PauseVerify), true
		}
	case "model_floor":
		if o.ModelFloor != nil {
			return *o.ModelFloor, true
		}
	case "quiet_hours":
		if o.QuietHours != nil {
			return *o.QuietHours, true
		}
	}
	return "", false
}

// SetFromString sets one key from its stored string form, the inverse of
// StringValue — how the per-key table rows are reassembled into a value. A
// malformed stored value (only reachable by hand-editing the DB) is an error.
func (o *Overrides) SetFromString(key, value string) error {
	switch key {
	case "dispatch_width":
		n, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("dispatch_width value %q is not an integer: %w", value, err)
		}
		o.DispatchWidth = &n
	case "hold_merges":
		b, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("hold_merges value %q is not a boolean: %w", value, err)
		}
		o.HoldMerges = &b
	case "pause_verify":
		b, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("pause_verify value %q is not a boolean: %w", value, err)
		}
		o.PauseVerify = &b
	case "model_floor":
		v := value
		o.ModelFloor = &v
	case "quiet_hours":
		v := value
		o.QuietHours = &v
	default:
		return fmt.Errorf("unknown override key %q", key)
	}
	return nil
}

// Merge returns o with every key PRESENT in other overriding o's value; keys
// absent in other are carried through unchanged. It is the semantics of a POST
// that sets a subset of keys: the provided keys win, the rest stay.
func (o Overrides) Merge(other Overrides) Overrides {
	out := o
	if other.DispatchWidth != nil {
		out.DispatchWidth = other.DispatchWidth
	}
	if other.HoldMerges != nil {
		out.HoldMerges = other.HoldMerges
	}
	if other.PauseVerify != nil {
		out.PauseVerify = other.PauseVerify
	}
	if other.ModelFloor != nil {
		out.ModelFloor = other.ModelFloor
	}
	if other.QuietHours != nil {
		out.QuietHours = other.QuietHours
	}
	return out
}

// Hash is a stable content hash over the SET keys and their values. Two tables
// with the same set keys and values hash identically regardless of pointer
// identity, which is what makes the record PR idempotent per (cell, table hash)
// — refiling the same table produces the same hash and no second PR.
func (o Overrides) Hash() string {
	// Canonical form: the set keys in vocabulary order, each with its string
	// value. Pointers and struct layout never enter the hash.
	pairs := make([][2]string, 0, len(overrideKeys))
	for _, k := range overrideKeys {
		if v, ok := o.StringValue(k); ok {
			pairs = append(pairs, [2]string{k, v})
		}
	}
	raw, _ := json.Marshal(pairs)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

// ValidateOverrides is the CLOSED-FORM validator shared by the HTTP POST and
// the cells.yaml schema — one function, two callers, so the file can never
// accept a value the API refuses (spec: one validator). allowedModels is the
// union of the cell's loops[].models ladder ids; model_floor must name one of
// them. An empty allowedModels means the cell names no models, so any
// model_floor is refused (there is nothing it could legitimately be).
//
// It validates ONLY the keys that are set — an absent key is not an error, it
// is the no-override state. Booleans carry no constraint.
func ValidateOverrides(o Overrides, allowedModels []string) error {
	if o.DispatchWidth != nil {
		if *o.DispatchWidth < 0 || *o.DispatchWidth > DispatchWidthMax {
			return fmt.Errorf("dispatch_width %d is out of range: it must be between 0 and %d inclusive", *o.DispatchWidth, DispatchWidthMax)
		}
	}
	if o.ModelFloor != nil {
		if !containsString(allowedModels, *o.ModelFloor) {
			return fmt.Errorf("model_floor %q is not one of the cell's declared model ladder ids %v — the floor may only raise the cheapest model to one the cell's loops already name",
				*o.ModelFloor, allowedModels)
		}
	}
	if o.QuietHours != nil {
		if err := validateQuietHours(*o.QuietHours); err != nil {
			return fmt.Errorf("quiet_hours %q is invalid: %w", *o.QuietHours, err)
		}
	}
	return nil
}

// validateQuietHours enforces `HH:MM-HH:MM <IANA zone>`: a start and end time
// in 24-hour form and an IANA location the runtime can load. The zone is
// REQUIRED — a quiet window with no zone is ambiguous across a fleet that spans
// time zones, and "quiet overnight" means nothing without saying whose night.
func validateQuietHours(s string) error {
	fields := strings.Fields(s)
	if len(fields) != 2 {
		return fmt.Errorf("expected `HH:MM-HH:MM <IANA zone>` (a time range and an IANA zone separated by a space), got %d field(s)", len(fields))
	}
	rangePart, zonePart := fields[0], fields[1]
	start, end, ok := strings.Cut(rangePart, "-")
	if !ok {
		return fmt.Errorf("time range %q is not HH:MM-HH:MM", rangePart)
	}
	for _, hm := range []string{start, end} {
		if _, err := time.Parse("15:04", hm); err != nil {
			return fmt.Errorf("time %q is not a 24-hour HH:MM: %w", hm, err)
		}
	}
	if _, err := time.LoadLocation(zonePart); err != nil {
		return fmt.Errorf("zone %q is not a loadable IANA time zone: %w", zonePart, err)
	}
	return nil
}

func containsString(xs []string, want string) bool {
	for _, x := range xs {
		if x == want {
			return true
		}
	}
	return false
}

// SortedStrings returns a sorted copy — a small helper the record-PR title and
// error messages use so their key lists are stable across runs.
func SortedStrings(xs []string) []string {
	out := append([]string(nil), xs...)
	sort.Strings(out)
	return out
}
