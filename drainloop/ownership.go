package drainloop

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
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
// Decode it with DecodeCellCheckpoint: a plain json.Unmarshal reads an absent
// hold flag as not held.
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
// at any depth, an unknown field, trailing data and an unsupported schema. A bool
// or a zero count cannot say "absent", so absence is refused here instead of
// decoding as not held or as zero. An explicit null nullable count stays unknown.
// A decoded record grants no authority and selects no current incarnation.
func DecodeCellCheckpoint(body []byte) (CellCheckpoint, error) {
	var raw any
	if err := json.Unmarshal(body, &raw); err != nil {
		return CellCheckpoint{}, err
	}
	if err := requireFields(raw, reflect.TypeOf(CellCheckpoint{}), "checkpoint"); err != nil {
		return CellCheckpoint{}, err
	}
	var state CellCheckpoint
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&state); err != nil {
		return CellCheckpoint{}, err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return CellCheckpoint{}, errors.New("checkpoint: trailing data")
	}
	if state.Schema != CellCheckpointSchema {
		return CellCheckpoint{}, fmt.Errorf("checkpoint: unsupported schema %q", state.Schema)
	}
	return state, nil
}

// requireFields reports the first JSON field of t that v leaves out. An object
// or array position must hold an object or array; null is accepted only where
// the Go field is a pointer, which is how this contract spells unknown.
func requireFields(v any, t reflect.Type, path string) error {
	switch t.Kind() {
	case reflect.Struct:
		obj, ok := v.(map[string]any)
		if !ok {
			return fmt.Errorf("%s: want an object", path)
		}
		for i := 0; i < t.NumField(); i++ {
			name, _, _ := strings.Cut(t.Field(i).Tag.Get("json"), ",")
			child, ok := obj[name]
			if !ok {
				return fmt.Errorf("%s.%s: field absent", path, name)
			}
			if err := requireFields(child, t.Field(i).Type, path+"."+name); err != nil {
				return err
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
