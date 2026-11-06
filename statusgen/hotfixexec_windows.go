//go:build windows

package main

import (
	"fmt"
	"os"
)

// sgExecFallback on Windows runs nothing and refuses: the hotfix overlay is not supported
// there (Windows has no exec, and sgHotfixOwnedBy refuses every overlay path), so a refused
// hotfix stops with exit 6 instead of starting the pinned statusgen as a child. Starting no
// process here also adds no launch site to statusgen's Git-launch inventory.
func sgExecFallback(path string, _, _ []string) int {
	fmt.Fprintf(os.Stderr, "statusgen: refusing to run: the hotfix overlay is not supported on Windows, so the pinned %s is not started in its place\n", path)
	return sgExitUnverifiable
}
