package main

import (
	"reflect"
	"testing"
)

func TestScanHeldRefusalsKeepsSourceOrderAndOrdinaryFirstReason(t *testing.T) {
	const evidence = "First check: **HELD**\n\nSecond check: **COULD-NOT-CHECK**"
	want := []heldRefusalSignal{
		{line: 1, reason: "First check: **HELD**"},
		{line: 3, reason: "Second check: **COULD-NOT-CHECK**"},
	}
	if got := scanHeldRefusals(evidence, false); !reflect.DeepEqual(got, want) {
		t.Fatalf("all refusals = %#v, want %#v", got, want)
	}
	if held, reason := unroutedHeldLine(evidence, false); !held || reason != want[0].reason {
		t.Fatalf("ordinary refusal = %v, %q, want true, %q", held, reason, want[0].reason)
	}
}
