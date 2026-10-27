//go:build hotfixseam

package deskkit

import "os"

// HotfixSeamBuild reports whether this binary was built with the test-only hotfixseam tag.
// Only the end-to-end test script builds with it; the release build never passes the tag.
const HotfixSeamBuild = true

// hotfixSeamGet reads a seam from the environment. It exists only in a hotfixseam build.
func hotfixSeamGet(k string) string { return os.Getenv(k) }
