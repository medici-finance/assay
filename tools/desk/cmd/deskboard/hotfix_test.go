package main

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"github.com/medici-finance/assay/tools/desk/internal/hotfixreaders"
)

// withHotfixSeams drives staleState's primary branch: pin v1.0.41 (or pinTag), running tag
// `running`, a hotfix expiry `expires`, and the clock at now.
func withHotfixSeams(t *testing.T, pinTag, running, expires string, now time.Time) {
	t.Helper()
	withPinSeams(t, true, pinTag, running)
	oldExp, oldNow, oldOrigin := deskkit.HotfixExpires, hotfixNow, originPin
	t.Cleanup(func() { deskkit.HotfixExpires, hotfixNow, originPin = oldExp, oldNow, oldOrigin })
	deskkit.HotfixExpires = expires
	hotfixNow = func() time.Time { return now }
	originPin = func(string, string) (string, string, string, bool) { return "", "", "", false }
}

// regression: #4405
// TestStaleStateHotfix: a valid hotfix over the pin is its own state — `hotfix`, stale=false,
// a `HOTFIX:` header — never drift, never in-sync; expired or pin-moved, it is drift.
func TestStaleStateHotfix(t *testing.T) {
	now := time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)
	live := now.Add(48 * time.Hour).Format(time.RFC3339)
	tag := "v1.0.41+hotfix-0123456789ab"

	if got := normalizeTag(tag); got != tag {
		t.Fatalf("normalizeTag(%q) = %q: the hotfix suffix must survive normalization, or the in-sync compare would read a hotfix as the pin", tag, got)
	}

	withHotfixSeams(t, "v1.0.41", tag, live, now)
	state, stale, detail := staleState()
	if state != staleStateHotfix || stale {
		t.Fatalf("valid hotfix: state %q stale %v (detail %q), want hotfix/false", state, stale, detail)
	}
	var out bytes.Buffer
	printBanners(&out, Header{StaleState: state, Stale: stale, StaleDetail: detail})
	first := strings.SplitN(out.String(), "\n", 2)[0]
	if !strings.HasPrefix(first, "HOTFIX:") || !strings.Contains(first, tag) || !strings.Contains(first, "over pin v1.0.41") {
		t.Fatalf("banner %q, want a HOTFIX: line naming %s over pin v1.0.41", first, tag)
	}

	withHotfixSeams(t, "v1.0.41", tag, now.Add(-time.Minute).Format(time.RFC3339), now)
	if state, stale, detail := staleState(); state != staleStateDrift || !stale || !strings.Contains(detail, "expired") {
		t.Fatalf("expired hotfix: %q %v %q, want drift naming expired", state, stale, detail)
	}

	withHotfixSeams(t, "v1.0.42", tag, live, now)
	if state, stale, detail := staleState(); state != staleStateDrift || !stale || !strings.Contains(detail, "outlived its pin") {
		t.Fatalf("pin moved: %q %v %q, want drift naming outlived", state, stale, detail)
	}

	withHotfixSeams(t, "v1.0.41", tag, "", now)
	if state, stale, _ := staleState(); state != staleStateDrift || !stale {
		t.Fatalf("hotfix with no expiry: %q %v, want drift", state, stale)
	}
}

// regression: #4405
// TestDeskboardHotfixReaders: deskboard's staleState reaches the shared matrix's verdict on
// every tag, read against the matrix pin with an unexpired hotfix clock.
func TestDeskboardHotfixReaders(t *testing.T) {
	m, err := hotfixreaders.Load("deskboard")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)
	for _, c := range m.Cases {
		withHotfixSeams(t, m.Pin, c.Tag, now.Add(48*time.Hour).Format(time.RFC3339), now)
		state, _, detail := staleState()
		if want := c.Verdicts["deskboard"]; state != want {
			t.Errorf("deskboard reads %q as %s (%s), matrix says %s", c.Tag, state, detail, want)
		}
	}
}
