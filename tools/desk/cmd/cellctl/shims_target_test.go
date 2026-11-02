package main

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// regression: #5288 — the verifier's instrument pin check reads the binary a
// generated shim execs through deskkit.ShimExecTarget. This pins that the real
// generator output still carries the marker and the exec-line shape it parses:
// an ordinary shim resolves to its one binary, a hotfix shim (binary chosen at
// run time) is a shim whose binary cannot be known from its text.
func TestShimsParseForPinCheck(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("cellctl shims are bash scripts")
	}
	base := t.TempDir()
	c := &Cell{Name: "fixture", Dir: base, Home: filepath.Join(base, "home")}
	wrap := filepath.Join(base, "shim-gh")
	bin := filepath.Join(base, "pinned", "statusgen")

	got, shim, err := deskkit.ShimExecTarget([]byte(shimBody(c, wrap, bin)))
	if !shim || err != nil || got != bin {
		t.Fatalf("generated shim: ShimExecTarget = %q, shim=%v, err=%v; want %q", got, shim, err, bin)
	}

	ch := deskkit.HotfixShimChoice{
		Hotfix: true,
		Entry: deskkit.HotfixEntry{Tool: "statusgen", BasePin: "desk-tools/v1.0.0",
			SourceSHA: "abc1234", Expires: "2030-01-01T00:00:00Z"},
		Bin:    filepath.Join(base, "overlay", "statusgen"),
		Pinned: bin,
	}
	got, shim, err = deskkit.ShimExecTarget([]byte(hotfixShimBody(c, wrap, ch)))
	if !shim || err == nil {
		t.Fatalf("hotfix shim: ShimExecTarget = %q, shim=%v, err=%v; want a shim with an unresolvable binary", got, shim, err)
	}
}
