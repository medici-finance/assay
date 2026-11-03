package drainloop

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"sort"
	"strings"
)

// CellCheckpointSchema identifies the passive Cell checkpoint record contract.
const CellCheckpointSchema = "cell-checkpoint/v1"

// CellOwnership is a generation inside one operator incarnation. Generation is
// opaque, not an increasing integer whose value can survive rollback as authority.
type CellOwnership struct {
	CellID        string `json:"cell_id"`
	IncarnationID string `json:"incarnation_id"`
	OwnerID       string `json:"owner_id"`
	Generation    string `json:"generation"`
	Backend       string `json:"backend"`
}

// CellCheckpoint is a passive persistence contract, not a second lease store.
// Outstanding effects, cancellation, stop and overrides must survive ordinary
// restart and restore. Missing history is retained as unknown, never new allowance.
// A consumer must not use Activation to select the current operator incarnation.
// Decode it with DecodeCellCheckpoint: a plain json.Unmarshal reads an absent,
// null, repeated or case-variant hold flag as not held.
type CellCheckpoint struct {
	Schema          string         `json:"schema"`
	Ownership       CellOwnership  `json:"ownership"`
	Activation      CellActivation `json:"activation"`
	Budget          CellBudget     `json:"budget"`
	CancelRequested bool           `json:"cancel_requested"`
	Stopped         bool           `json:"stopped"`
	OverrideRefs    []string       `json:"override_refs"`
	PendingEffects  []string       `json:"pending_effects"`
}

// DecodeCellCheckpoint decodes one checkpoint record. It refuses an absent field
// at any depth, an explicit null where the field is not nullable, a repeated key,
// a key that is not exactly a field name, trailing data and an unsupported
// schema. A bool or a zero count cannot say "absent", and encoding/json reads a
// null bool as false and keeps the last of two keys that match a field without
// regard to case, so each of those shapes is refused here instead of decoding as
// not held or as zero. An explicit null nullable count stays unknown. A decoded
// record grants no authority and selects no current incarnation.
func DecodeCellCheckpoint(body []byte) (CellCheckpoint, error) {
	// Preserve encoding/json's syntax and nesting bound before recursive token reading.
	if !json.Valid(body) {
		return CellCheckpoint{}, errors.New("checkpoint: invalid JSON")
	}
	tree := json.NewDecoder(bytes.NewReader(body))
	tree.UseNumber()
	raw, err := readStrict(tree)
	if err != nil {
		return CellCheckpoint{}, err
	}
	if _, err := tree.Token(); !errors.Is(err, io.EOF) {
		return CellCheckpoint{}, errors.New("checkpoint: trailing data")
	}
	if err := requireFields(raw, reflect.TypeOf(CellCheckpoint{}), "checkpoint"); err != nil {
		return CellCheckpoint{}, err
	}
	// The first pass has refused trailing data, repeated keys and inexact keys,
	// so this decode sees one value whose keys each name exactly one field.
	var state CellCheckpoint
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&state); err != nil {
		return CellCheckpoint{}, err
	}
	if state.Schema != CellCheckpointSchema {
		return CellCheckpoint{}, fmt.Errorf("checkpoint: unsupported schema %q", state.Schema)
	}
	return state, nil
}

// readStrict reads one JSON value into a generic tree and refuses a key that
// appears twice in one object. A map decode would keep the last one silently.
func readStrict(dec *json.Decoder) (any, error) {
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	switch tok {
	case json.Delim('{'):
		obj := map[string]any{}
		for dec.More() {
			keyTok, err := dec.Token()
			if err != nil {
				return nil, err
			}
			key, ok := keyTok.(string)
			if !ok {
				return nil, errors.New("checkpoint: object key is not a string")
			}
			if _, seen := obj[key]; seen {
				return nil, fmt.Errorf("checkpoint: repeated key %q", key)
			}
			if obj[key], err = readStrict(dec); err != nil {
				return nil, err
			}
		}
		_, err := dec.Token()
		return obj, err
	case json.Delim('['):
		items := []any{}
		for dec.More() {
			item, err := readStrict(dec)
			if err != nil {
				return nil, err
			}
			items = append(items, item)
		}
		_, err := dec.Token()
		return items, err
	}
	return tok, nil
}

// requireFields reports the first position of v that does not fit t: a null
// where the Go field is not a pointer (a pointer is how this contract spells
// unknown), an object or array position holding something else, an absent
// field, or a key that is not exactly one of t's JSON field names.
func requireFields(v any, t reflect.Type, path string) error {
	if v == nil {
		if t.Kind() == reflect.Pointer {
			return nil
		}
		return fmt.Errorf("%s: null where the field is not nullable", path)
	}
	switch t.Kind() {
	case reflect.Struct:
		obj, ok := v.(map[string]any)
		if !ok {
			return fmt.Errorf("%s: want an object", path)
		}
		names := map[string]bool{}
		for i := 0; i < t.NumField(); i++ {
			name, _, _ := strings.Cut(t.Field(i).Tag.Get("json"), ",")
			names[name] = true
			child, ok := obj[name]
			if !ok {
				return fmt.Errorf("%s.%s: field absent", path, name)
			}
			if err := requireFields(child, t.Field(i).Type, path+"."+name); err != nil {
				return err
			}
		}
		keys := make([]string, 0, len(obj))
		for key := range obj {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			if !names[key] {
				return fmt.Errorf("%s.%s: not exactly a field name", path, key)
			}
		}
	case reflect.Slice:
		items, ok := v.([]any)
		if !ok {
			return fmt.Errorf("%s: want an array", path)
		}
		for i, item := range items {
			if err := requireFields(item, t.Elem(), fmt.Sprintf("%s[%d]", path, i)); err != nil {
				return err
			}
		}
	}
	return nil
}
