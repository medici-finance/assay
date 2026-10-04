package deskkit

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// hostAssetsCallers is the allow-list of production files that may call
// HostPlatformAssets. Both are READ-side fallbacks: they look for this host's
// per-platform pin line when the bare line is absent, and never write a digest.
//
// The defect class this guards (assay#2201): a writer of a bare pin line
// (`statusgen <tag> <sha256>`) that fills it from the host platform's asset
// instead of the `<artifact>-linux-amd64` asset by name. upgrade-assay used to
// reach the host pick through this function. A new caller is a new site that can
// repeat the defect, so it fails here until it is reviewed and listed.
var hostAssetsCallers = []string{
	"cmd/deskboard/nextup.go",
	"internal/deskkit/pins.go",
}

// TestHostAssetsCallerAllowList walks every non-test Go file under tools/desk and
// fails on a HostPlatformAssets caller outside the allow-list. It also fails when
// an allow-listed caller is no longer found, so a matcher that silently stops
// matching cannot report clean (the listed files are its positive control).
func TestHostAssetsCallerAllowList(t *testing.T) {
	root := filepath.Join("..", "..") // tools/desk
	found := map[string]bool{}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == "testdata" || strings.HasPrefix(d.Name(), ".") && p != root {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
			return nil
		}
		raw, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		for _, line := range strings.Split(string(raw), "\n") {
			if strings.Contains(line, "HostPlatformAssets(") && !strings.HasPrefix(line, "func HostPlatformAssets(") {
				rel, _ := filepath.Rel(root, p)
				found[filepath.ToSlash(rel)] = true
				break
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	allowed := map[string]bool{}
	for _, f := range hostAssetsCallers {
		allowed[f] = true
		if !found[f] {
			t.Errorf("allow-listed caller %s no longer calls HostPlatformAssets — update the list (or the matcher is broken)", f)
		}
	}
	var extra []string
	for f := range found {
		if !allowed[f] {
			extra = append(extra, f)
		}
	}
	sort.Strings(extra)
	for _, f := range extra {
		t.Errorf("%s calls HostPlatformAssets, which is not on the allow-list: a bare pin line must take the <artifact>-linux-amd64 digest by name, never the host's (assay#2201)", f)
	}
}
