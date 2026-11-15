package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
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

	gen := shimBody(c, wrap, bin)
	got, shim, err := deskkit.ShimExecTarget([]byte(gen))
	if !shim || err != nil || got != bin {
		t.Fatalf("generated shim: ShimExecTarget = %q, shim=%v, err=%v; want %q", got, shim, err, bin)
	}
	// One changed line in the generated text is no longer the generator's output.
	for name, edited := range map[string]string{
		"added command":  strings.Replace(gen, "fi\n", "fi\n/bin/true\n", 1),
		"extra env word": strings.Replace(gen, "exec env ", "exec env -i ", 1),
		"LD_PRELOAD":     strings.Replace(gen, "exec env ", "exec env LD_PRELOAD=/x.so ", 1),
	} {
		if got, shim, err := deskkit.ShimExecTarget([]byte(edited)); !shim || err == nil {
			t.Errorf("%s: ShimExecTarget = %q, shim=%v, err=%v; want a shim refused", name, got, shim, err)
		}
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

// regression: #5288 — the bash oracle generates shims too; its template must stay
// byte-identical to the Go one (printf '%s\n' restores the newline the heredoc's
// command substitution strips), or the pin check refuses every oracle-made shim.
func TestShimOracleParity(t *testing.T) {
	b, err := os.ReadFile("../../../cellctl/testdata/cellctl-shell-oracle.sh")
	if err != nil {
		t.Fatalf("read the bash oracle: %v", err)
	}
	const open, end = "body=\"$(cat <<'SHIM_TEMPLATE'\n", "\nSHIM_TEMPLATE\n"
	s := strings.ReplaceAll(string(b), "\r\n", "\n")
	i := strings.Index(s, open)
	j := strings.Index(s[i+len(open):], end)
	if i < 0 || j < 0 {
		t.Fatalf("the oracle no longer carries a SHIM_TEMPLATE heredoc after %q", open)
	}
	if got := s[i+len(open):i+len(open)+j] + "\n"; got != deskkit.CellctlShimTemplate {
		t.Fatalf("oracle shim template differs from deskkit.CellctlShimTemplate:\n--- oracle\n%s\n--- go\n%s", got, deskkit.CellctlShimTemplate)
	}
}
