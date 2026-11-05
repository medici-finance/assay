package deskkit

import (
	"net/http"
	"strconv"
	"strings"
	"time"
)

// The hotfix test seams. A seam is read ONLY in a binary built with the test-only hotfixseam
// build tag (hotfixseam_on.go); in every other build hotfixSeamGet returns "" and each
// accessor below returns the compiled, unseamed value. Go tests in the consuming packages set
// their own unexported package variables from _test.go files instead.
const (
	// HotfixSeamRootUID names the uid the owner check treats as the hotfix owner.
	HotfixSeamRootUID = "ASSAY_HOTFIXSEAM_ROOT_UID"
	// HotfixSeamForge names the forge API base URL the source rule is decided against.
	HotfixSeamForge = "ASSAY_HOTFIXSEAM_FORGE"
	// HotfixSeamClone names the repository URL the install verb clones from.
	HotfixSeamClone = "ASSAY_HOTFIXSEAM_CLONE"
	// HotfixSeamGoProxy names the module proxy the install verb's build reads.
	HotfixSeamGoProxy = "ASSAY_HOTFIXSEAM_GOPROXY"
)

// HotfixSeamValue is seam k's value in a hotfixseam build, else "".
func HotfixSeamValue(k string) string { return strings.TrimSpace(hotfixSeamGet(k)) }

// ActiveHotfixOwner is the owner check every layer uses: root, unless a hotfixseam build
// names another uid.
func ActiveHotfixOwner() HotfixOwnerCheck {
	if v := HotfixSeamValue(HotfixSeamRootUID); v != "" {
		if uid, err := strconv.ParseUint(v, 10, 32); err == nil {
			return HotfixOwnedBy(uint32(uid))
		}
	}
	return HotfixRootOwned()
}

// ActiveHotfixForge is the forge every layer decides the source rule against: the compiled
// forge, unless a hotfixseam build names another base URL.
func ActiveHotfixForge() HotfixForge {
	f := DefaultHotfixForge()
	if v := HotfixSeamValue(HotfixSeamForge); v != "" {
		f.BaseURL = v
		f.HTTP = &http.Client{Timeout: 30 * time.Second}
	}
	return f
}
