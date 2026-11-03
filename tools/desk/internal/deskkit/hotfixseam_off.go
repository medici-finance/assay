//go:build !hotfixseam

package deskkit

// HotfixSeamBuild reports whether this binary was built with the test-only hotfixseam tag.
// The release build never passes the tag, so in every shipped binary it is false and every
// hotfix check reads only its compiled values.
const HotfixSeamBuild = false

// hotfixSeamGet is the seam lookup. Without the hotfixseam tag it reads nothing.
func hotfixSeamGet(string) string { return "" }
