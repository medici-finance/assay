package main

import (
	"path/filepath"
	"testing"
)

// TestConfigPathFromTrimpath: the default config path survives a -trimpath
// build, whose recorded source path is module-relative rather than a directory
// on disk.
func TestConfigPathFromTrimpath(t *testing.T) {
	none := func(string) bool { return false }
	abs := filepath.Join(string(filepath.Separator)+"repo", "tools", "metrics-harvest", "support.go")
	if got, want := configPathFrom(abs, none), filepath.Join(filepath.Dir(abs), "domains.yaml"); got != want {
		t.Errorf("absolute source path: got %q, want %q", got, want)
	}
	trimmed := "github.com/medici-finance/assay/tools/metrics-harvest/support.go"
	rootCfg := filepath.Join("tools", "metrics-harvest", "domains.yaml")
	for name, c := range map[string]struct {
		present map[string]bool
		want    string
	}{
		"go run . (cwd is the tool dir)": {map[string]bool{"domains.yaml": true}, "domains.yaml"},
		"go run from the repo root":      {map[string]bool{rootCfg: true}, rootCfg},
		"neither present":                {map[string]bool{}, "domains.yaml"},
	} {
		got := configPathFrom(trimmed, func(p string) bool { return c.present[p] })
		if got != c.want {
			t.Errorf("%s: got %q, want %q", name, got, c.want)
		}
	}
	if got := configPathFrom("", none); got != "domains.yaml" {
		t.Errorf("no caller info: got %q", got)
	}
}
