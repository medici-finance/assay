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

type signalKind int

const (
	sigTerm signalKind = 15
	sigKill signalKind = 9
)

func signalMarked(process, Marker, signalKind) error {
	return fmt.Errorf("cell process termination is unsupported on this platform")
}

func inventoryAll() (map[int]process, error) { return inventory() }
