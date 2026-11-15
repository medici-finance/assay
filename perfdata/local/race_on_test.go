//go:build race

package local

// raceEnabled: timing gates are reported but not enforced under the race detector.
const raceEnabled = true
