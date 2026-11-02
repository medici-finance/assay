package drainloop

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
)

// walkCheckpoint visits every typed position of the independent persisted fixture.
// Replacing a position is temporary; each case starts with the same valid record.
func walkCheckpoint(node any, typ reflect.Type, path string, set func(any), visit func(any, reflect.Type, string, func(any))) {
	visit(node, typ, path, set)
	if typ.Kind() == reflect.Pointer {
		return
	}
	switch n := node.(type) {
	case map[string]any:
		for i := 0; i < typ.NumField(); i++ {
			f := typ.Field(i)
			name, _, _ := strings.Cut(f.Tag.Get("json"), ",")
			v := n[name]
			walkCheckpoint(v, f.Type, path+"."+name, func(x any) { n[name] = x }, visit)
		}
	case []any:
		for i, v := range n {
			walkCheckpoint(v, typ.Elem(), fmt.Sprintf("%s[%d]", path, i), func(x any) { n[i] = x }, visit)
		}
	}
}

func checkpointTree(t *testing.T) any {
	t.Helper()
	body, err := os.ReadFile("testdata/cell-checkpoint.json")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = DecodeCellCheckpoint(body); err != nil {
		t.Fatalf("valid control: %v", err)
	}
	var tree any
	if err = json.Unmarshal(body, &tree); err != nil {
		t.Fatal(err)
	}
	return tree
}

// regression: F-checkpoint-hold-decode F-checkpoint-null-hold
func TestCheckpointNullPositionsHeld(t *testing.T) {
	tree := checkpointTree(t)
	checked := 0
	walkCheckpoint(tree, reflect.TypeOf(CellCheckpoint{}), "checkpoint", func(v any) { tree = v }, func(node any, typ reflect.Type, path string, set func(any)) {
		if typ.Kind() == reflect.Pointer {
			return
		} // Explicit null is the contract's unknown count.
		checked++
		set(nil)
		body, err := json.Marshal(tree)
		set(node)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = DecodeCellCheckpoint(body); err == nil {
			t.Errorf("null %s accepted", path)
		}
	})
	if checked < 100 {
		t.Fatalf("only %d positions checked", checked)
	}
}

// regression: F-checkpoint-hold-decode F-checkpoint-null-hold
func TestCheckpointObjectKeysHeld(t *testing.T) {
	for _, mode := range []string{"duplicate", "case-variant"} {
		t.Run(mode, func(t *testing.T) {
			tree := checkpointTree(t)
			checked := 0
			walkCheckpoint(tree, reflect.TypeOf(CellCheckpoint{}), "checkpoint", func(v any) { tree = v }, func(node any, typ reflect.Type, path string, set func(any)) {
				obj, ok := node.(map[string]any)
				if !ok {
					return
				}
				for key, value := range obj {
					raw, err := json.Marshal(obj)
					if err != nil {
						t.Fatal(err)
					}
					other := key
					if mode == "case-variant" {
						other = strings.ToUpper(key)
					}
					// A repeated true hold followed by false demonstrates the unsafe last-wins shape.
					replacement := value
					if _, ok := value.(bool); ok {
						replacement = false
					}
					k, _ := json.Marshal(other)
					v, _ := json.Marshal(replacement)
					raw = append(raw[:len(raw)-1], append(append(append([]byte(","), k...), ':'), v...)...)
					raw = append(raw, '}')
					set(json.RawMessage(raw))
					body, err := json.Marshal(tree)
					set(node)
					if err != nil {
						t.Fatal(err)
					}
					checked++
					if _, err = DecodeCellCheckpoint(body); err == nil {
						t.Errorf("%s key %s.%s accepted", mode, path, key)
					}
				}
			})
			if checked < 100 {
				t.Fatalf("only %d keys checked", checked)
			}
		})
	}
}

// regression: F-checkpoint-hold-decode
func TestCheckpointPausedDecodeBoundary(t *testing.T) {
	p := cellFenceFixture()
	state := readCellFixture(t)
	state.Ownership = p.owned
	state.Activation = p.active
	state.Activation.Paused = true
	body, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	for _, plant := range []string{`"paused":null`, `"paused":true,"paused":false`, `"paused":true,"PAUSED":false`} {
		edited := bytes.Replace(body, []byte(`"paused":true`), []byte(plant), 1)
		decoded, err := DecodeCellCheckpoint(edited)
		if err == nil {
			p.active = decoded.Activation
			t.Errorf("malformed pause decoded; boundary result=%v", p.apply(p.owned))
		}
	}
	p = cellFenceFixture()
	if err = p.apply(p.owned); err != nil || p.effects != 1 {
		t.Fatalf("valid boundary control: %v", err)
	}
}
