package deskkit

import (
	"os"
	"strings"
	"testing"
)

// TestRepairAdmissionEnabledIsOptIn pins the opt-in contract: only the exact on-token enables
// enforcement, and the policy version is reported either way. Anything else is OFF, so the shipped
// default (unset) leaves the reservation advisory exactly as example-stream/05 shipped it.
func TestRepairAdmissionEnabledIsOptIn(t *testing.T) {
	cases := []struct {
		val  string
		want bool
	}{
		{"", false},
		{"on", true},
		{" on ", true}, // trimmed
		{"off", false},
		{"0", false},
		{"true", false}, // not the on-token — unrecognised is OFF, never a silent enforce
		{"ON", false},   // case-sensitive: only the exact token
	}
	for _, c := range cases {
		if c.val == "" {
			os.Unsetenv(EnvRepairAdmission)
		} else {
			t.Setenv(EnvRepairAdmission, c.val)
		}
		on, version := RepairAdmissionEnabled()
		if on != c.want {
			t.Errorf("RepairAdmissionEnabled(%q) = %v, want %v", c.val, on, c.want)
		}
		if version != RepairAdmissionPolicyVersion {
			t.Errorf("policy version = %q, want %q", version, RepairAdmissionPolicyVersion)
		}
	}
}

// TestEvaluateAdmissionFloorHoldsFreshAdmitsRepair is the evaluator's core: a fresh candidate is
// HELD when admitting it would drop the free slots to or below the reserved floor of a class with
// runnable demand, while the reserved (repair) candidate is admitted on the same state. This is the
// example-stream/18 reservation as an EFFECT — the assertion whose removal reopens the defect.
func TestEvaluateAdmissionFloorHoldsFreshAdmitsRepair(t *testing.T) {
	// width 3, one repair reserved and runnable, 2 slots already taken => 1 free slot == the floor.
	base := AdmissionInputs{
		Width:          3,
		Reserve:        map[string]int{"resume": 0, "rework": 1},
		Occupancy:      2,
		RunnableDemand: map[string]int{"rework": 1},
		WaitingItem:    "repair example-stream/17",
	}

	fresh := base
	fresh.Class = AdmissionFresh
	if v, reason := EvaluateAdmission(fresh); v != AdmissionHold {
		t.Fatalf("fresh at/below the floor: got %v (%s), want HOLD", v, reason)
	} else if !strings.Contains(reason, "repair example-stream/17") {
		t.Errorf("HOLD reason must name the waiting repair, got %q", reason)
	}

	repair := base
	repair.Class = AdmissionRework
	if v, _ := EvaluateAdmission(repair); v != AdmissionAdmit {
		t.Fatalf("the reserved repair itself must be ADMITTED into its own reservation, got %v", v)
	}
}

// TestEvaluateAdmissionExternalWaitDoesNotIdle proves a reserved class with NO runnable demand
// (its only obligation is waiting-external, so RunnableDemand is 0) contributes no floor, and fresh
// is admitted into the otherwise-reserved slot — an external hold never idles a usable slot.
func TestEvaluateAdmissionExternalWaitDoesNotIdle(t *testing.T) {
	in := AdmissionInputs{
		Class:          AdmissionFresh,
		Width:          3,
		Reserve:        map[string]int{"rework": 1},
		Occupancy:      2,
		RunnableDemand: map[string]int{"rework": 0}, // the repair is waiting-external, not runnable
	}
	if v, reason := EvaluateAdmission(in); v != AdmissionAdmit {
		t.Fatalf("fresh must be ADMITTED when the reserved class has no runnable demand, got %v (%s)", v, reason)
	}
}

// TestEvaluateAdmissionPoolFullHolds proves a full pool holds every class — a slot has to free
// before anything, including a repair, can be admitted.
func TestEvaluateAdmissionPoolFullHolds(t *testing.T) {
	in := AdmissionInputs{Class: AdmissionRework, Width: 2, Occupancy: 2, Reserve: map[string]int{}}
	if v, _ := EvaluateAdmission(in); v != AdmissionHold {
		t.Fatalf("a full pool must HOLD even a repair, got %v", v)
	}
}
