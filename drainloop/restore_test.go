package drainloop

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"sort"
	"testing"
)

func readCellFixture(t *testing.T) CellCheckpoint {
	t.Helper()
	body, err := os.ReadFile("testdata/cell-checkpoint.json")
	if err != nil {
		t.Fatal(err)
	}
	state, err := DecodeCellCheckpoint(body)
	if err != nil {
		t.Fatal(err)
	}
	return state
}

// regression: F-ge16-schema
func TestCellCheckpointRoundTrip(t *testing.T) {
	state := readCellFixture(t)
	if state.Schema != CellCheckpointSchema {
		t.Fatalf("fixture schema %q, want %q", state.Schema, CellCheckpointSchema)
	}
	body, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	restored, err := DecodeCellCheckpoint(body)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(state, restored) {
		t.Fatal("checkpoint fields lost")
	}
	if !restored.Stopped || !restored.CancelRequested || !restored.Activation.Paused ||
		len(restored.OverrideRefs) != 1 || len(restored.PendingEffects) != 1 {
		t.Fatal("stop/cancellation/override/unknown effect lost")
	}
	balances := restored.Budget.Balances
	if len(balances) != 2 || balances[0].Unit == balances[1].Unit ||
		balances[1].Consumed != nil {
		t.Fatal("quota units merged or unknown usage became zero")
	}
	if balances[0].WindowStart == balances[1].WindowStart &&
		balances[0].WindowEnd == balances[1].WindowEnd {
		t.Fatal("fixture windows are not distinct")
	}
	if len(restored.Budget.Reservations) != 1 ||
		!restored.Budget.Reservations[0].Unknown ||
		restored.Budget.Reservations[0].Amount != 30 {
		t.Fatal("unknown reservation lost")
	}
	usage := restored.Budget.Usage
	if len(usage) != 5 {
		t.Fatal("not every attempt phase retained")
	}
	phases := []string{"preparation", "summary", "execution", "fallback", "verification"}
	for i, u := range usage {
		if u.WorkID != "example-work" || u.Phase != phases[i] ||
			u.AttemptID == "" || u.RequestID == "" || u.SourceRef == "" {
			t.Fatal("attempt attribution lost")
		}
	}
	if usage[2].Outcome != "failed" || usage[2].Input == nil || *usage[2].Input != 17 ||
		usage[4].Output != nil {
		t.Fatal("failed attempt counts or missing telemetry lost")
	}
	// The restored store cannot promote its historical activation to current.
	p := cellFenceFixture()
	p.active = restored.Activation
	p.owned = restored.Ownership
	if err := p.apply(restored.Ownership); err == nil || p.effects != 0 {
		t.Fatal("restored paused state admitted an effect")
	}
}

// eachKey calls visit once per object key at every depth of node, with that key
// removed from the shared tree; the key is restored before the next visit.
func eachKey(node any, path string, visit func(string)) {
	switch n := node.(type) {
	case map[string]any:
		keys := make([]string, 0, len(n))
		for key := range n {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			child := n[key]
			delete(n, key)
			visit(path + "." + key)
			n[key] = child
			eachKey(child, path+"."+key, visit)
		}
	case []any:
		for i, child := range n {
			eachKey(child, fmt.Sprintf("%s[%d]", path, i), visit)
		}
	}
}

// regression: F-ge16-schema
func TestCheckpointAbsentFieldsHeld(t *testing.T) {
	body, err := os.ReadFile("testdata/cell-checkpoint.json")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeCellCheckpoint(body); err != nil {
		t.Fatalf("positive control: %v", err)
	}
	var tree any
	if err := json.Unmarshal(body, &tree); err != nil {
		t.Fatal(err)
	}
	removed := 0
	eachKey(tree, "checkpoint", func(path string) {
		removed++
		edited, err := json.Marshal(tree)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := DecodeCellCheckpoint(edited); err == nil {
			t.Errorf("absent %s decoded without error", path)
		}
	})
	// Every field of every record in the fixture, nested ones included.
	if removed < 100 {
		t.Fatalf("walked %d keys; the fixture walk did not reach every record", removed)
	}
	refused := map[string]func(map[string]any){
		"unsupported schema": func(m map[string]any) { m["schema"] = "cell-checkpoint/v0" },
		"null activation":    func(m map[string]any) { m["activation"] = nil },
		"null effect list":   func(m map[string]any) { m["pending_effects"] = nil },
		"unknown field":      func(m map[string]any) { m["paused_hint"] = false },
	}
	for name, edit := range refused {
		var m map[string]any
		if err := json.Unmarshal(body, &m); err != nil {
			t.Fatal(err)
		}
		edit(m)
		edited, err := json.Marshal(m)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := DecodeCellCheckpoint(edited); err == nil {
			t.Errorf("%s decoded without error", name)
		}
	}
	if _, err := DecodeCellCheckpoint(append(body, []byte(" {}")...)); err == nil {
		t.Error("trailing data decoded without error")
	}
}
