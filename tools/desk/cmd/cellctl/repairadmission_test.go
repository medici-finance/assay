package main

import "testing"

// TestRepairAdmissionKnownCellEnvKey: the opt-in is a recognised cell.env key, so `cellctl set`
// accepts it without --force (the whole point of the rollout wiring — it was refused before).
func TestRepairAdmissionKnownCellEnvKey(t *testing.T) {
	if !knownCellEnvKey("ASSAY_REPAIR_ADMISSION") {
		t.Error("knownCellEnvKey(ASSAY_REPAIR_ADMISSION) = false, want true")
	}
}

// TestRepairAdmissionValueValidation: only the literal on/off pass, and the value rule is
// NON-bypassable — --force widens the unknown-KEY allowlist, never the value rule for a known
// key (the CELL_HARNESS/CELL_KIND/CELL_COCKPIT precedent).
func TestRepairAdmissionValueValidation(t *testing.T) {
	validateEnvKey("ASSAY_REPAIR_ADMISSION", "on", false)
	validateEnvKey("ASSAY_REPAIR_ADMISSION", "off", false)
	assertDies(t, "invalid value", func() { validateEnvKey("ASSAY_REPAIR_ADMISSION", "maybe", false) })
	// --force does not lift the value rule for a known key.
	assertDies(t, "invalid value even with --force", func() { validateEnvKey("ASSAY_REPAIR_ADMISSION", "maybe", true) })
	assertDies(t, "empty value", func() { validateEnvKey("ASSAY_REPAIR_ADMISSION", "", true) })
}

// TestRepairAdmissionValue: the composition helper turns cell.env into what deskLaunch composes —
// "on" only for the literal on-token, and identical ABSENCE ("") for off, unset and anything
// else, matching how deskkit.RepairAdmissionEnabled reads the same variable in the consumer.
func TestRepairAdmissionValue(t *testing.T) {
	cases := []struct {
		name string
		env  map[string]string
		want string
	}{
		{"on", map[string]string{"ASSAY_REPAIR_ADMISSION": "on"}, "on"},
		{"on with surrounding space", map[string]string{"ASSAY_REPAIR_ADMISSION": " on "}, "on"},
		{"off", map[string]string{"ASSAY_REPAIR_ADMISSION": "off"}, ""},
		{"unset", map[string]string{}, ""},
		{"garbage", map[string]string{"ASSAY_REPAIR_ADMISSION": "true"}, ""},
	}
	for _, c := range cases {
		cell := &Cell{Env: envWith(c.env)}
		if got := cell.repairAdmissionValue(); got != c.want {
			t.Errorf("%s: repairAdmissionValue() = %q, want %q", c.name, got, c.want)
		}
	}
}
