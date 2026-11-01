//go:build windows

package main

import "fmt"

// sgHotfixOwnedBy on Windows refuses every path: the hotfix overlay is not supported there, so
// no Windows statusgen ever treats an overlay file as trusted.
func sgHotfixOwnedBy(uid uint32) func(string) error {
	return func(path string) error {
		return fmt.Errorf("%s: the hotfix overlay is not supported on Windows", path)
	}
}
