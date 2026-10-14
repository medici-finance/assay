//go:build !darwin && !linux

package cellstop

import "fmt"

func inventory() (map[int]process, error) {
	return nil, fmt.Errorf("cell reset process inspection requires macOS or Linux")
}
func ancestors(map[int]process) map[int]bool { return nil }
func terminate(process, bool) error {
	return fmt.Errorf("cell reset process termination is unsupported")
}

type signal int

const (
	sigTerm signal = 15
	sigKill signal = 9
)

func signalMarked(process, Marker, signal) error {
	return fmt.Errorf("cell process termination is unsupported on this platform")
}
