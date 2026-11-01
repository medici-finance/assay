//go:build hotfixseam

package main

import "os"

// sgHotfixSeamGet reads a hotfix seam from the environment. It exists only in a binary built
// with the test-only hotfixseam tag; the release build never passes that tag.
func sgHotfixSeamGet(k string) string { return os.Getenv(k) }
