package dorajoin

import (
	"strings"
	"testing"
)

// TestDurableVolumeFixture pins the exact denominator arithmetic (spec §8,
// Verify item 3): landed=1200, churn=200, copied=150 must yield EXACTLY 850
// (1200-200-150) — a specific asserted value, not a presence check.
func TestDurableVolumeFixture(t *testing.T) {
	in := DurableVolumeInput{
		Window:        "2026-08",
		Stream:        "quality",
		IdentityClass: "human",
		LandedLines:   Measured(1200.0),
		Churn:         Measured(200.0),
		Copied:        Measured(150.0),
	}
	got := ComputeDurableVolume(in)
	if got.Value.State != StateMeasured {
		t.Fatalf("expected StateMeasured, got %q (reason %q)", got.Value.State, got.Value.Reason)
	}
	if got.Value.Value != 850 {
		t.Fatalf("durable-change volume = %v, want exactly 850 (1200-200-150)", got.Value.Value)
	}
	if got.Window != "2026-08" || got.Stream != "quality" || got.IdentityClass != "human" {
		t.Fatalf("grain keys not preserved: %+v", got)
	}
}

func TestDurableVolumeMeasuredZero(t *testing.T) {
	in := DurableVolumeInput{
		LandedLines: Measured(100.0),
		Churn:       Measured(60.0),
		Copied:      Measured(40.0),
	}
	got := ComputeDurableVolume(in)
	if got.Value.State != StateMeasuredZero {
		t.Fatalf("expected StateMeasuredZero for an exact wash, got %q", got.Value.State)
	}
}

// TestDurableVolumeCouldNotMeasure asserts the guard this brief requires:
// any could-not-measure component makes the WHOLE denominator
// could-not-measure, naming the missing component, never a subtraction
// against an assumed zero.
func TestDurableVolumeCouldNotMeasure(t *testing.T) {
	cases := []struct {
		name string
		in   DurableVolumeInput
		want string
	}{
		{
			name: "landed unmeasured",
			in: DurableVolumeInput{
				LandedLines: CouldNotMeasure[float64]("miner could not read"),
				Churn:       Measured(10.0),
				Copied:      Measured(5.0),
			},
			want: "landed_lines",
		},
		{
			name: "churn unmeasured",
			in: DurableVolumeInput{
				LandedLines: Measured(10.0),
				Churn:       CouldNotMeasure[float64]("churn window unresolved"),
				Copied:      Measured(5.0),
			},
			want: "churn_14d",
		},
		{
			name: "copied unmeasured",
			in: DurableVolumeInput{
				LandedLines: Measured(10.0),
				Churn:       Measured(5.0),
				Copied:      CouldNotMeasure[float64]("taxonomy unresolved"),
			},
			want: "copied",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ComputeDurableVolume(tc.in)
			if got.Value.State != StateCouldNotMeasure {
				t.Fatalf("expected StateCouldNotMeasure, got %q", got.Value.State)
			}
			if !strings.Contains(got.Value.Reason, tc.want) {
				t.Fatalf("reason %q does not name missing component %q", got.Value.Reason, tc.want)
			}
		})
	}
}

func TestComputeDurableVolumesPreservesOrder(t *testing.T) {
	inputs := []DurableVolumeInput{
		{Window: "w1", LandedLines: Measured(10.0), Churn: Measured(1.0), Copied: Measured(1.0)},
		{Window: "w2", LandedLines: Measured(20.0), Churn: Measured(2.0), Copied: Measured(2.0)},
	}
	got := ComputeDurableVolumes(inputs)
	if len(got) != 2 || got[0].Window != "w1" || got[1].Window != "w2" {
		t.Fatalf("order not preserved: %+v", got)
	}
}
