package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// writePinFile writes a `.assay-versions` carrying exactly the given lines, in a fresh temp
// dir, and points the pin walk at it by making that dir the working directory.
func writePinFile(t *testing.T, lines ...string) string {
	t.Helper()
	dir := t.TempDir()
	body := strings.Join(lines, "\n") + "\n"
	if err := os.WriteFile(filepath.Join(dir, deskkit.AssayVersionsFile), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })
	// t.TempDir on macOS hands back a /var symlink to /private/var; the pin walk resolves
	// the working directory, so return what the process now believes it is standing in.
	got, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	return got
}

// TestStalePinResolvesPerPlatformArtifact is the defect and its fix.
//
// `docs/distribution.md` tells an adopter to pin a `<artifact>-<platform>` line; the drift
// self-check only ever looked up the bare `desk-tools` name. A consumer carrying ONLY the
// per-platform line therefore fell through every arm of staleState and landed on
// could-not-check, which reports stale=true BY DESIGN — a correct verdict about a wrong
// observation, reported to the operator as "your install is stale" on every single run.
func TestStalePinResolvesPerPlatformArtifact(t *testing.T) {
	const tag = "v0.28.0"
	const sha = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

	oldPinned, oldTree, oldTag, oldPlat := isPinned, gitTree, deskkit.ReleaseTag, deskPlatform
	oldSrc := deskToolsSourcePin
	t.Cleanup(func() {
		isPinned, gitTree, deskkit.ReleaseTag, deskPlatform = oldPinned, oldTree, oldTag, oldPlat
		deskToolsSourcePin = oldSrc
	})
	isPinned = func() bool { return true }
	// A consumer checkout: the in-tree tools/desk ref does not resolve there, which is what
	// made the fall-through terminal rather than merely slower.
	gitTree = func(string) (string, error) { return "", os.ErrNotExist }
	// Keep the channel-D source pin OFF so a stray `.assay-versions` cannot answer for this.
	deskToolsSourcePin = func() (string, string, bool) { return "", "", false }
	deskPlatform = "darwin-arm64"
	deskkit.ReleaseTag = tag

	t.Run("per-platform line ONLY — the defect", func(t *testing.T) {
		writePinFile(t, "desk-tools-darwin-arm64 "+tag+" "+sha)
		state, stale, detail := staleState()
		if state != staleStateInSync || stale {
			t.Fatalf("a pin file carrying only the per-platform line reports %q (stale=%v): %s\n"+
				"want in-sync — the pin IS present and the running tag matches it",
				state, stale, detail)
		}
		if !strings.Contains(detail, "desk-tools-darwin-arm64") {
			t.Errorf("the detail does not name the artifact the verdict came from: %q", detail)
		}
	})

	t.Run("per-platform line at a DIFFERENT tag still reports drift, not could-not-check", func(t *testing.T) {
		writePinFile(t, "desk-tools-darwin-arm64 v0.27.0 "+sha)
		state, stale, detail := staleState()
		if state != staleStateDrift || !stale {
			t.Fatalf("a per-platform pin at a different tag reports %q (stale=%v), want drift: %s", state, stale, detail)
		}
		if !strings.Contains(detail, "desk-tools-darwin-arm64") {
			t.Errorf("the drift detail does not name the artifact: %q", detail)
		}
	})

	t.Run("the BARE line still wins when both are present", func(t *testing.T) {
		// The bare line is tried first, so an existing consumer's verdict cannot change
		// under this. Here the two lines disagree, and the bare one must decide.
		writePinFile(t,
			"desk-tools "+tag+" "+sha,
			"desk-tools-darwin-arm64 v0.27.0 "+sha)
		state, stale, detail := staleState()
		if state != staleStateInSync || stale {
			t.Fatalf("with both lines present the verdict is %q (stale=%v), want the BARE line's in-sync: %s",
				state, stale, detail)
		}
		if strings.Contains(detail, "desk-tools-darwin-arm64") {
			t.Errorf("the per-platform line decided although the bare line is present: %q", detail)
		}
	})

	t.Run("ANOTHER platform's line does not answer for this host", func(t *testing.T) {
		// The trailing-space prefix match is a control: `desk-tools ` must never match a
		// per-platform line, and `desk-tools-darwin-arm64 ` must never match another
		// platform's. A file carrying only a foreign platform's pin is still could-not-check.
		writePinFile(t, "desk-tools-linux-amd64 "+tag+" "+sha)
		state, stale, _ := staleState()
		if state != staleStateUnknown || !stale {
			t.Fatalf("a foreign platform's pin answered for this host: state=%q stale=%v — "+
				"the exact-name lookup must not have become a prefix match", state, stale)
		}
	})

	t.Run("no usable line at all is still could-not-check", func(t *testing.T) {
		writePinFile(t, "statusgen "+tag+" "+sha)
		state, stale, _ := staleState()
		if state != staleStateUnknown || !stale {
			t.Fatalf("a pin file with no desk-tools line of either shape: state=%q stale=%v, want could-not-check", state, stale)
		}
	})
}

// The per-platform artifact name must be built from the platform token, not guessed, and an
// empty platform must yield NO name — asking ArtifactPin for `desk-tools-` would match any
// line that merely starts that way, which is exactly the prefix collapse pins.go forbids.
func TestDeskToolsPlatformArtifactName(t *testing.T) {
	old := deskPlatform
	t.Cleanup(func() { deskPlatform = old })

	deskPlatform = "linux-amd64"
	if got, want := deskToolsPlatformArtifact(), "desk-tools-linux-amd64"; got != want {
		t.Errorf("deskToolsPlatformArtifact() = %q, want %q", got, want)
	}
	deskPlatform = ""
	if got := deskToolsPlatformArtifact(); got != "" {
		t.Errorf("an empty platform produced the artifact name %q — it must produce none", got)
	}
	deskPlatform = "   "
	if got := deskToolsPlatformArtifact(); got != "" {
		t.Errorf("a blank platform produced the artifact name %q — it must produce none", got)
	}
}
