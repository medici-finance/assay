package main

// binary-pin (layer 3 of the local hotfix overlay). These tests run against temp dirs only:
// a throwaway pinned dir, overlay dir and cells root, an httptest forge, and the owner check
// seam bound to the test user's uid. They never read /opt/desk-tools or a real overlay.

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"github.com/medici-finance/assay/tools/desk/internal/hotfixreaders"
)

const (
	pinBase  = "v1.0.41"
	pinNow   = "v1.0.42"
	srcSHA   = "0123456789abcdef0123456789abcdef01234567"
	pinStamp = "2026-10-25T10:00:00Z"
)

type pinFixture struct {
	root, cells, pinned, overlay, shimDir string
	forgeStatus                           *string
}

// newPinFixture builds the cell, the pinned dir and the repo root carrying a desk-tools pin
// line at pin, binds the environment to them and the seams to the test user.
func newPinFixture(t *testing.T, pin string) *pinFixture {
	t.Helper()
	base := t.TempDir()
	f := &pinFixture{
		root:    filepath.Join(base, "repo"),
		cells:   filepath.Join(base, "cells"),
		pinned:  filepath.Join(base, "pinned"),
		overlay: filepath.Join(base, "overlay"),
	}
	f.shimDir = filepath.Join(f.cells, "c1", "shim")
	for _, d := range []string{f.root, f.shimDir, f.pinned, filepath.Join(f.overlay, "bin")} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writePins(t, f.root, "desk-tools "+pin+" 0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef\n")
	t.Setenv(deskkit.CellMarkerEnv, "c1")
	t.Setenv("CELLS_ROOT", f.cells)
	t.Setenv("PATH", f.shimDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv(deskkit.DeskToolsBinEnv, f.pinned)
	t.Setenv(deskkit.HotfixOverlayEnv, f.overlay)

	status := "behind"
	f.forgeStatus = &status
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"status":%q,"files":[]}`, *f.forgeStatus)
	}))
	t.Cleanup(srv.Close)

	oo, of, oc, os_ := binaryPinOwner, binaryPinForge, binaryPinClock, binaryPinOS
	t.Cleanup(func() { binaryPinOwner, binaryPinForge, binaryPinClock, binaryPinOS = oo, of, oc, os_ })
	binaryPinOwner = func() deskkit.HotfixOwnerCheck { return deskkit.HotfixOwnedBy(uint32(os.Getuid())) }
	binaryPinForge = func() deskkit.HotfixForge {
		return deskkit.HotfixForge{BaseURL: srv.URL, Repo: deskkit.HotfixRepoSlug, HTTP: srv.Client()}
	}
	binaryPinClock = func() time.Time { return mustTime(t, "2026-10-26T10:00:00Z") }
	binaryPinOS = "linux"
	return f
}

func mustTime(t *testing.T, s string) time.Time {
	t.Helper()
	v, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

// pinnedShim writes a generated-shape shim execing the pinned binary.
func (f *pinFixture) pinnedShim(t *testing.T, tool string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(f.pinned, tool), []byte("pinned\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	body := fmt.Sprintf("#!/bin/sh\nexec \"%s\" \"$@\"\n", filepath.Join(f.pinned, tool))
	if err := os.WriteFile(filepath.Join(f.shimDir, tool), []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
}

// hotfixShim writes the overlay binary, a manifest entry for it and a hotfix-shaped shim.
func (f *pinFixture) hotfixShim(t *testing.T, tool string, edit func(*deskkit.HotfixEntry)) deskkit.HotfixEntry {
	t.Helper()
	f.pinnedShim(t, tool)
	bin := deskkit.HotfixBinaryPath(f.overlay, tool)
	content := []byte("hotfix build of " + tool + "\n")
	if err := os.WriteFile(bin, content, 0o755); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(content)
	e := deskkit.HotfixEntry{
		Tool: tool, SHA256: hex.EncodeToString(sum[:]), SourceSHA: srcSHA, BasePin: pinBase,
		BuiltAt: pinStamp, Expires: "2026-10-28T10:00:00Z", SourceRule: deskkit.HotfixSourceRule,
		Ack: "per the driver's ruling", InstalledBy: "operator", GoVersion: "go1.25.0", GoSumSHA256: "00",
	}
	if edit != nil {
		edit(&e)
	}
	m, err := deskkit.ReadHotfixManifest(f.overlay)
	if err != nil {
		m = &deskkit.HotfixManifest{Schema: deskkit.HotfixManifestSchema}
	}
	m.Entries = append(m.Entries, e)
	if err := deskkit.WriteHotfixManifest(f.overlay, m); err != nil {
		t.Fatal(err)
	}
	body := fmt.Sprintf("#!/bin/sh\nhotfix_bin=\"%s\"\npinned_bin=\"%s\"\nexec \"$hotfix_bin\" \"$@\"\n", bin, filepath.Join(f.pinned, tool))
	if err := os.WriteFile(filepath.Join(f.shimDir, tool), []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	return e
}

func (f *pinFixture) check() ([]string, bool, error) {
	return binaryPinCheck(bootOpts{role: "worker-desk", root: f.root})
}

func wantPinRefusal(t *testing.T, err error, phrases ...string) {
	t.Helper()
	if err == nil {
		t.Fatalf("binary-pin passed, want an exit-6 refusal naming %q", phrases)
	}
	if got := deskkit.ExitCodeOf(err); got != deskkit.ExitUnverifiable {
		t.Fatalf("exit = %d, want %d: %v", got, deskkit.ExitUnverifiable, err)
	}
	for _, p := range append(phrases, "binary-pin") {
		if !strings.Contains(err.Error(), p) {
			t.Errorf("refusal does not name %q:\n%v", p, err)
		}
	}
}

// Row 3: every shim pinned prints the pin; one hotfix prints the marker line, even quiet.
func TestBinaryPinStep(t *testing.T) {
	f := newPinFixture(t, pinNow)
	f.pinnedShim(t, "deskboard")
	lines, hot, err := f.check()
	if err != nil || hot || len(lines) != 1 || lines[0] != "all pinned (v1.0.42)" {
		t.Fatalf("all-pinned = %q hot=%v err=%v", lines, hot, err)
	}

}

// One hotfix among pinned shims prints the marker line naming tool, tag, base, source, expiry.
func TestBinaryPinStepHotfixLine(t *testing.T) {
	g := newPinFixture(t, pinBase)
	g.hotfixShim(t, "deskboard", nil)
	g.pinnedShim(t, "deskstatus")
	lines, hot, err := g.check()
	if err != nil || !hot || len(lines) != 1 {
		t.Fatalf("hotfix = %q hot=%v err=%v", lines, hot, err)
	}
	want := "running hotfix deskboard v1.0.41+hotfix-0123456789ab over pin v1.0.41 (source 0123456789ab, expires 2026-10-28T10:00:00Z)"
	if lines[0] != want {
		t.Errorf("marker line\n got %q\nwant %q", lines[0], want)
	}
}

// The step is wired into boot as the tenth, printed even under --quiet when a hotfix runs.
func TestBinaryPinIsBootStepTen(t *testing.T) {
	if n := len(bootSteps); n != 10 || bootSteps[n-1] != stepBinaryPin {
		t.Fatalf("bootSteps = %v, want binary-pin last of 10", bootSteps)
	}
}

// Row 6, layer 3 ALONE: layers 1 and 2 are bypassed (the shim still points at the overlay and
// no hotfix binary ever runs); the boot step refuses on the pin moving past the base.
func TestBinaryPinOutlived(t *testing.T) {
	f := newPinFixture(t, pinNow)
	f.hotfixShim(t, "deskboard", func(e *deskkit.HotfixEntry) { e.BasePin = pinBase })
	_, _, err := f.check()
	wantPinRefusal(t, err, deskkit.HotfixReasonOutlived, "deskboard")
}

// Row 21: the owner seam is NOT set — the real uid-0 check runs. A user-owned overlay with a
// consistently rewritten manifest and binary is refused, and a regenerated shim selects the
// pinned binary.
func TestBinaryPinOverlayOwner(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("the test user is root, so a user-owned overlay cannot be built")
	}
	f := newPinFixture(t, pinBase)
	binaryPinOwner = deskkit.HotfixRootOwned
	f.hotfixShim(t, "deskboard", nil)
	_, _, err := f.check()
	wantPinRefusal(t, err, deskkit.HotfixReasonNotRootOwned)

	c := deskkit.ChooseHotfixShim(f.overlay, f.pinned, "deskboard", pinBase, binaryPinClock(), deskkit.HotfixRootOwned())
	if c.Hotfix || c.Bin != filepath.Join(f.pinned, "deskboard") {
		t.Errorf("a regenerated shim chose %+v, want the pinned binary", c)
	}
}

// Row 22: with the upper layers bypassed, each bypass is refused by this layer on its own.
func TestBinaryPinBypass(t *testing.T) {
	t.Run("over the ceiling", func(t *testing.T) {
		f := newPinFixture(t, pinBase)
		f.hotfixShim(t, "deskboard", func(e *deskkit.HotfixEntry) { e.Expires = "2026-11-20T10:00:00Z" })
		_, _, err := f.check()
		wantPinRefusal(t, err, deskkit.HotfixReasonCeiling)
	})
	t.Run("another source rule", func(t *testing.T) {
		f := newPinFixture(t, pinBase)
		f.hotfixShim(t, "deskboard", func(e *deskkit.HotfixEntry) { e.SourceRule = "any-branch" })
		_, _, err := f.check()
		wantPinRefusal(t, err, deskkit.HotfixReasonSourceRule)
	})
	t.Run("expired", func(t *testing.T) {
		f := newPinFixture(t, pinBase)
		f.hotfixShim(t, "deskboard", func(e *deskkit.HotfixEntry) { e.Expires = "2026-10-25T20:00:00Z" })
		_, _, err := f.check()
		wantPinRefusal(t, err, deskkit.HotfixReasonExpired)
	})
	t.Run("overlay binary stamped with the bare pin tag is still a hotfix", func(t *testing.T) {
		// The classification is by the path the shim execs, never by what the binary prints.
		f := newPinFixture(t, pinBase)
		f.hotfixShim(t, "deskboard", nil)
		lines, hot, err := f.check()
		if err != nil || !hot || strings.Contains(strings.Join(lines, "\n"), "all pinned") {
			t.Fatalf("lines=%q hot=%v err=%v: an overlay-resident shim must be reported as a hotfix", lines, hot, err)
		}
	})
	t.Run("shim aimed outside both dirs", func(t *testing.T) {
		f := newPinFixture(t, pinBase)
		f.pinnedShim(t, "deskboard")
		stray := filepath.Join(t.TempDir(), "deskboard")
		if err := os.WriteFile(stray, []byte("x"), 0o755); err != nil {
			t.Fatal(err)
		}
		body := fmt.Sprintf("#!/bin/sh\nexec \"%s\" \"$@\"\n", stray)
		if err := os.WriteFile(filepath.Join(f.shimDir, "deskboard"), []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
		_, _, err := f.check()
		wantPinRefusal(t, err, "shim target outside the pinned and hotfix dirs")
	})
	t.Run("tampered binary", func(t *testing.T) {
		f := newPinFixture(t, pinBase)
		f.hotfixShim(t, "deskboard", nil)
		if err := os.WriteFile(deskkit.HotfixBinaryPath(f.overlay, "deskboard"), []byte("changed\n"), 0o755); err != nil {
			t.Fatal(err)
		}
		_, _, err := f.check()
		wantPinRefusal(t, err, deskkit.HotfixReasonTampered, "sha256")
	})
	t.Run("source no longer on main", func(t *testing.T) {
		f := newPinFixture(t, pinBase)
		f.hotfixShim(t, "deskboard", nil)
		*f.forgeStatus = "diverged"
		_, _, err := f.check()
		wantPinRefusal(t, err, deskkit.HotfixReasonNotOnMain)
	})
	t.Run("never a hotfix target", func(t *testing.T) {
		f := newPinFixture(t, pinBase)
		f.hotfixShim(t, "deskboot", nil)
		_, _, err := f.check()
		wantPinRefusal(t, err, "never a hotfix target")
	})
	t.Run("overlay shim with no manifest entry", func(t *testing.T) {
		f := newPinFixture(t, pinBase)
		f.hotfixShim(t, "deskboard", nil)
		m := &deskkit.HotfixManifest{Schema: deskkit.HotfixManifestSchema}
		if err := deskkit.WriteHotfixManifest(f.overlay, m); err != nil {
			t.Fatal(err)
		}
		_, _, err := f.check()
		wantPinRefusal(t, err, deskkit.HotfixReasonTampered)
	})
}

// A forge that cannot be reached is could-not-check (exit 6), never a pass.
func TestBinaryPinForgeUnreachable(t *testing.T) {
	f := newPinFixture(t, pinBase)
	f.hotfixShim(t, "deskboard", nil)
	binaryPinForge = func() deskkit.HotfixForge {
		return deskkit.HotfixForge{BaseURL: "http://127.0.0.1:1", Repo: deskkit.HotfixRepoSlug}
	}
	_, _, err := f.check()
	wantPinRefusal(t, err, "could-not-check")
}

// No cell marker, no shim directory: nothing shims a desk tool, so there is nothing to
// classify and no hotfix can be running.
func TestBinaryPinNoCell(t *testing.T) {
	f := newPinFixture(t, pinNow)
	t.Setenv(deskkit.CellMarkerEnv, "")
	t.Setenv("PATH", "/usr/bin")
	lines, hot, err := f.check()
	if err != nil || hot || len(lines) != 1 || !strings.HasPrefix(lines[0], "all pinned") {
		t.Fatalf("lines=%q hot=%v err=%v", lines, hot, err)
	}
}

// regression: #4405
// TestDeskbootHotfixReaders: deskboot's own-tag check reaches the shared matrix's verdict on
// every tag, and a hotfix-stamped deskboot is refused by the step itself (not only classified).
func TestDeskbootHotfixReaders(t *testing.T) {
	m, err := hotfixreaders.Load("deskboot")
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range m.Cases {
		t.Run(c.Tag, func(t *testing.T) {
			want := c.Verdicts["deskboot"]
			if got := ownTagVerdict(c.Tag, m.Pin); got != want {
				t.Errorf("deskboot reads its own tag %q as %s, matrix says %s", c.Tag, got, want)
			}
			f := newPinFixture(t, m.Pin)
			f.pinnedShim(t, "deskboard")
			old := deskkit.ReleaseTag
			deskkit.ReleaseTag = c.Tag
			defer func() { deskkit.ReleaseTag = old }()
			_, _, cerr := f.check()
			if (cerr != nil) != (want == "refuse") {
				t.Errorf("deskboot step with own tag %q: err=%v, matrix says %s", c.Tag, cerr, want)
			}
			if cerr != nil && !strings.Contains(cerr.Error(), "never a hotfix target") {
				t.Errorf("deskboot refusal for %q does not say why:\n%v", c.Tag, cerr)
			}
		})
	}
}
