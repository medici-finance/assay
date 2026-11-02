package deskkit

// Tests for #5288: the instrument pin check hashed a cellctl shim (a wrapper
// script first on PATH) instead of the binary the shim execs, so a matching
// install read as pin drift.

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

const shimHost = "linux-amd64"

// shimText reproduces the shape cmd/cellctl generates for one desk verb: the
// marker comment, then two exec branches. cmd/cellctl's own test pins that the
// real generator output parses the same way.
func shimText(home, bin, other string) string {
	return "#!/usr/bin/env bash\n" +
		CellctlShimMarker + "fixture): run this desk verb with the CELL config-home.\n" +
		"gh_token=\"${CELLCTL_GH_AMBIENT:-}\"\n" +
		"if [[ -n \"$gh_token\" ]]; then\n" +
		"  export CELLCTL_GH_AMBIENT=\"$gh_token\"\n" +
		"  exec env HOME=\"" + home + "\" PATH=\"" + home + "/shim-gh:$PATH\" \"" + bin + "\" \"$@\"\n" +
		"else\n" +
		"  exec env HOME=\"" + home + "\" \"" + other + "\" \"$@\"\n" +
		"fi\n"
}

// hotfixText reproduces a hotfix shim: a prelude choosing $bin at run time.
func hotfixText(home, hot, pinned string) string {
	body := shimText(home, "$bin", "$bin")
	nl := strings.IndexByte(body, '\n')
	return body[:nl+1] + "hotfix_bin=\"" + hot + "\"\npinned_bin=\"" + pinned + "\"\nbin=\"$hotfix_bin\"\n" + body[nl+1:]
}

type shimWorld struct {
	pinRoot, shimDir, binDir, home string
	pinSHA                         string
}

// newShimWorld writes a pin file whose statusgen line pins pinned's bytes and
// puts only an empty shim directory on PATH.
func newShimWorld(t *testing.T, pinned []byte) *shimWorld {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("cellctl shims are bash scripts; PATH lookup on Windows never resolves one")
	}
	base := t.TempDir()
	w := &shimWorld{
		pinRoot: filepath.Join(base, "repo"),
		shimDir: filepath.Join(base, "shim"),
		binDir:  filepath.Join(base, "bin"),
		home:    filepath.Join(base, "home"),
		pinSHA:  verifierDigest(pinned),
	}
	for _, d := range []string{w.pinRoot, w.shimDir, w.binDir, w.home} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	pins := "statusgen-" + shimHost + " statusgen/v0.9.0 " + w.pinSHA + "\n"
	if err := os.WriteFile(filepath.Join(w.pinRoot, AssayVersionsFile), []byte(pins), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", w.shimDir)
	return w
}

func (w *shimWorld) write(t *testing.T, dir, name string, body []byte) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, body, 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

func (w *shimWorld) admit() (string, string, Check) {
	return admitInstrument(PreflightProbes{Instrument: instrumentProbe}, w.pinRoot, "statusgen", shimHost)
}

var (
	pinnedBytes  = []byte("\x7fELF pinned statusgen build")
	driftedBytes = []byte("\x7fELF some other statusgen build")
)

// regression: #5288
func TestShimToPinnedBinaryIsClean(t *testing.T) {
	w := newShimWorld(t, pinnedBytes)
	bin := w.write(t, w.binDir, "statusgen", pinnedBytes)
	w.write(t, w.shimDir, "statusgen", []byte(shimText(w.home, bin, bin)))
	value, path, c := w.admit()
	if c.State != CheckedClean {
		t.Fatalf("shim exec'ing the pinned binary = %v %q, want clean", c.State, c.Detail)
	}
	if path != bin || value != "sha256:"+w.pinSHA {
		t.Fatalf("admitted %q %q, want the exec'd binary %q and its digest", path, value, bin)
	}
}

// regression: #5288
func TestShimToDriftedBinaryFails(t *testing.T) {
	w := newShimWorld(t, pinnedBytes)
	bin := w.write(t, w.binDir, "statusgen", driftedBytes)
	shim := w.write(t, w.shimDir, "statusgen", []byte(shimText(w.home, bin, bin)))
	_, _, c := w.admit()
	if c.State != CheckedFailed || !strings.Contains(c.Detail, "pin drift: "+bin+" ") {
		t.Fatalf("shim exec'ing a drifted binary = %v %q, want pin drift naming %s", c.State, c.Detail, bin)
	}
	if strings.Contains(c.Detail, shim) {
		t.Fatalf("drift names the shim %s, not the binary it runs: %q", shim, c.Detail)
	}
}

// regression: #5288
func TestHotfixShimIsUnchecked(t *testing.T) {
	w := newShimWorld(t, pinnedBytes)
	bin := w.write(t, w.binDir, "statusgen", pinnedBytes)
	w.write(t, w.shimDir, "statusgen", []byte(hotfixText(w.home, bin, bin)))
	value, _, c := w.admit()
	if c.State != CouldNotCheck || value != "" {
		t.Fatalf("run-time-selected shim = %v %q value %q, want could-not-check", c.State, c.Detail, value)
	}
}

// regression: #5288
func TestShimTwoTargetsIsUnchecked(t *testing.T) {
	w := newShimWorld(t, pinnedBytes)
	bin := w.write(t, w.binDir, "statusgen", pinnedBytes)
	other := w.write(t, w.binDir, "statusgen-other", pinnedBytes)
	w.write(t, w.shimDir, "statusgen", []byte(shimText(w.home, bin, other)))
	value, _, c := w.admit()
	if c.State != CouldNotCheck || value != "" {
		t.Fatalf("shim naming two binaries = %v %q value %q, want could-not-check", c.State, c.Detail, value)
	}
}

// regression: #5288
func TestShimNoTargetIsUnchecked(t *testing.T) {
	w := newShimWorld(t, pinnedBytes)
	gone := filepath.Join(w.binDir, "statusgen")
	w.write(t, w.shimDir, "statusgen", []byte(shimText(w.home, gone, gone)))
	if _, _, c := w.admit(); c.State != CouldNotCheck {
		t.Fatalf("shim for a missing binary = %v %q, want could-not-check", c.State, c.Detail)
	}
}

// regression: #5288
func TestShimToShimIsUnchecked(t *testing.T) {
	w := newShimWorld(t, pinnedBytes)
	bin := w.write(t, w.binDir, "statusgen", pinnedBytes)
	inner := w.write(t, w.binDir, "statusgen-shim", []byte(shimText(w.home, bin, bin)))
	w.write(t, w.shimDir, "statusgen", []byte(shimText(w.home, inner, inner)))
	if _, _, c := w.admit(); c.State != CouldNotCheck {
		t.Fatalf("shim chain = %v %q, want could-not-check", c.State, c.Detail)
	}
}

// Non-shim files keep the old behaviour: their own bytes are compared.
func TestPlainInstrumentUnchanged(t *testing.T) {
	w := newShimWorld(t, pinnedBytes)
	bin := w.write(t, w.shimDir, "statusgen", pinnedBytes)
	if _, path, c := w.admit(); c.State != CheckedClean || path != bin {
		t.Fatalf("plain pinned binary = %v %q at %q, want clean at %q", c.State, c.Detail, path, bin)
	}
	bin = w.write(t, w.shimDir, "statusgen", driftedBytes)
	if _, _, c := w.admit(); c.State != CheckedFailed || !strings.Contains(c.Detail, "pin drift: "+bin+" ") {
		t.Fatalf("plain drifted binary = %v %q, want pin drift naming %s", c.State, c.Detail, bin)
	}
	script := "#!/bin/sh\nexec \"" + filepath.Join(w.binDir, "x") + "\" \"$@\"\n"
	w.write(t, w.shimDir, "statusgen", []byte(script))
	if _, path, c := w.admit(); c.State != CheckedFailed || path != bin {
		t.Fatalf("unmarked wrapper script = %v %q at %q, want its own bytes compared (drift)", c.State, c.Detail, path)
	}
}

func TestShimExecTarget(t *testing.T) {
	abs := filepath.Join(t.TempDir(), "bin", "deskverb")
	if runtime.GOOS == "windows" {
		t.Skip("cellctl shims are bash scripts")
	}
	cases := []struct {
		name, script, want string
		shim, wantErr      bool
	}{
		{"binary", "\x7fELF\n" + CellctlShimMarker + "x)\n", "", false, false},
		{"unmarked script", "#!/bin/sh\nexec env HOME=\"/h\" \"" + abs + "\" \"$@\"\n", "", false, false},
		{"marker not at line start", "#!/bin/sh\n  " + CellctlShimMarker + "x)\n", "", false, false},
		{"generated shim", shimText("/h", abs, abs), abs, true, false},
		{"hotfix shim", hotfixText("/h", abs, abs), "", true, true},
		{"two targets", shimText("/h", abs, abs+"2"), "", true, true},
		{"relative target", shimText("/h", "bin/deskverb", "bin/deskverb"), "", true, true},
		{"no exec line", "#!/usr/bin/env bash\n" + CellctlShimMarker + "x)\n", "", true, true},
		{"other exec shape", "#!/usr/bin/env bash\n" + CellctlShimMarker + "x)\nexec " + abs + " \"$@\"\n", "", true, true},
		{"extra exec line", shimText("/h", abs, abs) + "exec \"" + abs + "\"\n", "", true, true},
		{"subst in target", shimText("/h", abs+"$(id)", abs+"$(id)"), "", true, true},
	}
	for _, tc := range cases {
		got, shim, err := ShimExecTarget([]byte(tc.script))
		if shim != tc.shim || (err != nil) != tc.wantErr || got != tc.want {
			t.Errorf("%s: ShimExecTarget = %q, shim=%v, err=%v; want %q, shim=%v, err=%v",
				tc.name, got, shim, err, tc.want, tc.shim, tc.wantErr)
		}
	}
}
