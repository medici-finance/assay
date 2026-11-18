//go:build unix

package main

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// hfReasonCases is one owner-check error of each refusal class and the reason it must carry:
// a namespace-gate refusal (no owner was read) and an owner actually read and found wrong.
func hfReasonCases() []struct {
	name, want, not string
	err             error
} {
	return []struct {
		name, want, not string
		err             error
	}{
		{"gate refusal", deskkit.HotfixReasonOwnerUnverified, deskkit.HotfixReasonNotRootOwned,
			deskkit.HotfixOwnerUnverifiedForTest("uid_map is 0 1000 1")},
		{"owner read", deskkit.HotfixReasonNotRootOwned, deskkit.HotfixReasonOwnerUnverified,
			errors.New("/opt/overlay is owned by uid 1000, not 0")},
	}
}

// regression: #6781
// TestHotfixOverlayOwnerReason: install's overlay check names each owner-check refusal by
// its class: could-not-check for a namespace-gate refusal, not-root-owned only for an owner
// actually read.
func TestHotfixOverlayOwnerReason(t *testing.T) {
	for _, c := range hfReasonCases() {
		err := ensureHotfixOverlay(t.TempDir(), func(string) error { return c.err })
		var he *deskkit.HotfixError
		if !errors.As(err, &he) || he.Reason != c.want || he.Code != hotfixExitRefused {
			t.Errorf("%s: ensureHotfixOverlay = %v, want reason %q and exit %d", c.name, err, c.want, hotfixExitRefused)
		}
	}
}

// regression: #6781
// TestHotfixClearOwnerReason: `hotfix clear` names each owner-check refusal of the overlay by
// its class, as install does.
func TestHotfixClearOwnerReason(t *testing.T) {
	w := newHotfixWorld(t, hfBase)
	if err := os.MkdirAll(w.overlay, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, c := range hfReasonCases() {
		hotfixOwner = func() deskkit.HotfixOwnerCheck { return func(string) error { return c.err } }
		code, _, errs := w.run("clear")
		if code != hotfixExitRefused || !strings.Contains(errs, "hotfix clear: "+c.want+": ") || strings.Contains(errs, c.not) {
			t.Errorf("%s: clear rc=%d stderr %q, want exit %d naming %q only", c.name, code, errs, hotfixExitRefused, c.want)
		}
	}
}
