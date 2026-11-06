//go:build !hotfixseam

package main

// sgHotfixSeamGet is the hotfix seam lookup. Without the test-only hotfixseam tag it reads
// nothing, so every shipped statusgen checks root ownership only.
func sgHotfixSeamGet(string) string { return "" }
