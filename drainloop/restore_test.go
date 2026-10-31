package drainloop

import (
	"bytes"
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func readCellFixture(t *testing.T) CellCheckpoint {
	t.Helper()
	body, err := os.ReadFile("testdata/cell-checkpoint.json")
	if err != nil {
		t.Fatal(err)
	}
	var state CellCheckpoint
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&state); err != nil {
		t.Fatal(err)
	}
	return state
}

// regression: F-ge16-schema
func TestCellCheckpointRoundTrip(t *testing.T) {
	state := readCellFixture(t)
	body, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	var restored CellCheckpoint
	if err := json.Unmarshal(body, &restored); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(state, restored) {
		t.Fatal("checkpoint fields lost")
	}
	if !restored.Stopped || !restored.CancelRequested || !restored.Activation.Paused ||
		len(restored.OverrideRefs) != 1 || len(restored.PendingEffects) != 1 {
		t.Fatal("stop/cancellation/override/unknown effect lost")
	}
	if len(restored.Budget.Balances) != 2 ||
		restored.Budget.Balances[0].Unit == restored.Budget.Balances[1].Unit ||
		restored.Budget.Balances[1].Consumed != nil {
		t.Fatal("quota units merged or unknown usage became zero")
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
