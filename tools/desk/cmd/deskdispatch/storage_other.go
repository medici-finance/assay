//go:build !darwin && !linux

package main

import "fmt"

func availableDiskBytes(string) (uint64, error) {
	return 0, fmt.Errorf("dispatch free-space measurement requires macOS or Linux")
}
