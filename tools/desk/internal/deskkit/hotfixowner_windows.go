//go:build windows

package deskkit

import "fmt"

// HotfixOwnedBy on Windows refuses every path: the hotfix overlay is not supported there, so
// no Windows shim or boot ever treats an overlay file as trusted.
func HotfixOwnedBy(uid uint32) HotfixOwnerCheck {
	return func(path string) error {
		return fmt.Errorf("%s: the hotfix overlay is not supported on Windows", path)
	}
}
