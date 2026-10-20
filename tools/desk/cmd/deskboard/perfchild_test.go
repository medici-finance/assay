package main

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

func TestPerfStatusgenChildren(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell fixture")
	}
	dir := t.TempDir()
	bin := filepath.Join(dir, "statusgen")
	script := "#!/bin/sh\ncase \"$1\" in\n--version) printf 'fixture-version\\n';;\n--gate-scores) printf '[]\\n';;\n--next-up) printf '{\"rows\":[]}\\n';;\nesac\n"
	if err := os.WriteFile(bin, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ASSAY_TELEMETRY", "1")
	deskkit.PerfRun("deskboard", perfVerbs, []string{"nextup"}, func([]string) int {
		if got := statusgenVersionOf(bin); got != "fixture-version" {
			t.Fatal(got)
		}
		if _, err := gateScoresForRoot(bin, dir, "example-org/tracker"); err != nil {
			t.Fatal(err)
		}
		if _, err := nextUpForRoot(bin, dir, "example-org/tracker"); err != nil {
			t.Fatal(err)
		}
		count, _, _ := deskkit.PerfCounters()
		if count != 3 {
			t.Fatalf("actual statusgen children=%d; want 3", count)
		}
		return 0
	})
}
