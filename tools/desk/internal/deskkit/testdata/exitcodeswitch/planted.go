// Package planted is the positive control for TestExitSwitchGuardFlagsPlant. It is
// parsed, never compiled: a testdata directory is outside every build.
package planted

import "example.invalid/deskkit"

func plantedSwitch(code int) string {
	// The guard must flag this switch: two exit codes as constant cases.
	switch code {
	case deskkit.ExitRefused:
		return "refused"
	case deskkit.ExitRateLimited:
		return "rate-limited"
	}
	return ""
}

var plantedMap = map[int]string{deskkit.ExitDisabled: "disabled", deskkit.ExitUnverifiable: "unverifiable"}
