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

// shimText is a generated shim exec'ing bin (CellctlShimTemplate, the one copy
// cmd/cellctl renders). other != bin hand-edits the first exec line to name a
// second binary.
func shimText(home, bin, other string) string {
	s := RenderCellctlShim("fixture", home, home+"/shim-gh", bin)
	if other != bin {
		s = strings.Replace(s, "\""+bin+"\" \"$@\"", "\""+other+"\" \"$@\"", 1)
	}
	return s
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

// shimCase is one hand-edited shim body and the name its failure reports.
type shimCase struct{ name, body string }

// admitEach writes each body as the statusgen shim and requires could-not-check.
func (w *shimWorld) admitEach(t *testing.T, cases []shimCase) {
	t.Helper()
	for _, tc := range cases {
		w.write(t, w.shimDir, "statusgen", []byte(tc.body))
		if value, _, c := w.admit(); c.State != CouldNotCheck || value != "" {
			t.Errorf("%s: hand-edited shim = %v %q value %q, want could-not-check", tc.name, c.State, c.Detail, value)
		}
	}
}

// regression: #5288 SEC-1 — a marked script that keeps a well-formed exec line
// naming the pinned binary but runs, preloads or resolves something else must
// never be admitted on the pinned binary's digest.
func TestShimBypassUnchecked(t *testing.T) {
	w := newShimWorld(t, pinnedBytes)
	bin := w.write(t, w.binDir, "statusgen", pinnedBytes)
	evil := w.write(t, w.binDir, "evil", driftedBytes)
	gen := shimText(w.home, bin, bin)
	argv := `"` + bin + `" "$@"`
	cases := []shimCase{
		{"command before exec", strings.Replace(gen, "gh_token=", `"`+evil+`" "$@"`+"\ngh_token=", 1)},
		{"compound exec line", strings.Replace(gen, "  exec env ", `  "`+evil+`" || exec env `, 1)},
		{"extra program to env", strings.ReplaceAll(gen, argv, `"`+evil+`" `+argv)},
		{"LD_PRELOAD", strings.ReplaceAll(gen, "exec env ", `exec env LD_PRELOAD="`+evil+`" `)},
		{"DYLD_INSERT_LIBRARIES", strings.ReplaceAll(gen, "exec env ", `exec env DYLD_INSERT_LIBRARIES="`+evil+`" `)},
		{"subst in HOME", strings.ReplaceAll(gen, `HOME="`+w.home+`"`, `HOME="$(`+evil+`)"`)},
		{"PATH change", strings.Replace(gen, "gh_token=", `export PATH="`+w.binDir+`:$PATH"`+"\ngh_token=", 1)},
		{"env -S", strings.ReplaceAll(gen, "exec env ", `exec env -S "`+evil+`" `)},
	}
	for _, tc := range cases {
		if tc.body == gen {
			t.Fatalf("%s: the fixture edit did not apply", tc.name)
		}
	}
	w.admitEach(t, cases)
}

// regression: #5288 SEC-1 (class) — shapes the review did not list: a body
// that IS the template rendered, but for a field value that changes what runs.
func TestShimFieldInjectUnchecked(t *testing.T) {
	w := newShimWorld(t, pinnedBytes)
	bin := w.write(t, w.binDir, "statusgen", pinnedBytes)
	evil := w.write(t, w.binDir, "evil", driftedBytes)
	// env(1) reads a NAME=VALUE word as an assignment, then runs "$1".
	eq := w.write(t, w.binDir, "x=y", pinnedBytes)
	wrap := w.home + "/shim-gh"
	w.admitEach(t, []shimCase{
		{"quote in home", RenderCellctlShim("fixture", w.home+`" LD_PRELOAD="`+evil, wrap, bin)},
		{"newline in cell", RenderCellctlShim("fixture\n\""+evil+"\" \"$@\"\n#", w.home, wrap, bin)},
		{"= in binary", RenderCellctlShim("fixture", w.home, wrap, eq)},
	})
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
