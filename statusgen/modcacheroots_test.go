package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// The module roots manifest against the tree it provisions (#4683). The open
// step downloads modules only for the roots images/verify-runner/
// go-module-roots.txt lists, and the image bakes the same set, so a Go module
// in the tree that has requirements of its own (a non-empty go.sum) but is not
// listed gets no modules: a row that builds it offline stops on module
// resolution, and the pre-row gap check, which reads only the listed roots,
// cannot name the cause. qualgen was such a module.

// notProvisionedPrefix marks a manifest comment naming a module root that is
// deliberately not listed, with the reason: `# not provisioned: <root> — <why>`.
const notProvisionedPrefix = "# not provisioned: "

// manifestExclusions reads the manifest's `# not provisioned:` lines: each
// root it names, mapped to its reason.
func manifestExclusions(manifest string) map[string]string {
	out := map[string]string{}
	for _, l := range strings.Split(manifest, "\n") {
		rest, ok := strings.CutPrefix(strings.TrimSpace(l), notProvisionedPrefix)
		if !ok {
			continue
		}
		root, why, _ := strings.Cut(rest, " ")
		out[filepath.ToSlash(filepath.Clean(root))] = strings.TrimSpace(why)
	}
	return out
}

// moduleRootsCoverage lists why the manifest does not cover the tree at home:
// each module root (a directory of one of gomods, home-relative go.mod paths)
// whose go.sum names any module but that is neither listed nor declared not
// provisioned with a reason; each listed root with no go.mod; and each root
// both listed and declared not provisioned.
func moduleRootsCoverage(home string, gomods []string, manifest string) []string {
	listed := map[string]bool{}
	for _, l := range strings.Split(manifest, "\n") {
		if l = strings.TrimSpace(l); l != "" && !strings.HasPrefix(l, "#") {
			listed[filepath.ToSlash(filepath.Clean(l))] = true
		}
	}
	excluded := manifestExclusions(manifest)
	var out []string
	for _, m := range gomods {
		root := filepath.ToSlash(filepath.Dir(filepath.FromSlash(m)))
		sum, err := os.ReadFile(filepath.Join(home, filepath.FromSlash(root), "go.sum"))
		if err != nil || strings.TrimSpace(string(sum)) == "" {
			continue // no requirement to resolve from a module cache
		}
		why, isExcluded := excluded[root]
		switch {
		case listed[root] && isExcluded:
			out = append(out, root+" is both listed and declared not provisioned")
		case listed[root]:
		case isExcluded && why == "":
			out = append(out, root+" is declared not provisioned with no reason")
		case !isExcluded:
			out = append(out, root+" has a go.sum but is not listed, so no module cache holds its requirements")
		}
	}
	for r := range listed {
		if _, err := os.Stat(filepath.Join(home, filepath.FromSlash(r), "go.mod")); err != nil {
			out = append(out, r+" is listed but holds no go.mod")
		}
	}
	sort.Strings(out)
	return out
}

// regression: #4683
// TestModuleRootsCoverTree: every tracked Go module whose go.sum names a
// module is listed in the manifest, or declared not provisioned with a reason.
// A planted module with a go.sum that the manifest does not list goes red.
func TestModuleRootsCoverTree(t *testing.T) {
	out, err := exec.Command("git", "-C", "..", "ls-files", "--", "go.mod", "*/go.mod").Output()
	if err != nil {
		t.Skipf("git ls-files: %v", err)
	}
	gomods := strings.Fields(string(out))
	if len(gomods) == 0 {
		t.Fatal("no tracked go.mod found")
	}
	raw, err := os.ReadFile(filepath.Join("..", filepath.FromSlash(moduleRootsManifest)))
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range moduleRootsCoverage("..", gomods, string(raw)) {
		t.Errorf("%s: %s", moduleRootsManifest, p)
	}

	// Positive control: a tree with one listed root and a second module the
	// manifest does not name.
	home := t.TempDir()
	writeGapFile(t, home, "a/go.mod", "module example.com/a\n")
	writeGapFile(t, home, "a/go.sum", "golang.org/x/text v0.43.0 h1:x=\n")
	writeGapFile(t, home, "b/go.mod", "module example.com/b\n")
	writeGapFile(t, home, "b/go.sum", "golang.org/x/sys v0.49.0 h1:x=\n")
	writeGapFile(t, home, "c/go.mod", "module example.com/c\n") // no go.sum: nothing to resolve
	planted := []string{"a/go.mod", "b/go.mod", "c/go.mod"}
	got := moduleRootsCoverage(home, planted, "a\n")
	if len(got) != 1 || !strings.HasPrefix(got[0], "b has a go.sum but is not listed") {
		t.Errorf("unlisted module b: got %q", got)
	}
	for _, c := range []struct{ manifest, want string }{
		{"a\n# not provisioned: b — fetched from elsewhere\n", ""},
		{"a\n# not provisioned: b\n", "b is declared not provisioned with no reason"},
		{"a\nb\n# not provisioned: b — x\n", "b is both listed and declared not provisioned"},
		{"a\nb\ngone\n", "gone is listed but holds no go.mod"},
	} {
		got := strings.Join(moduleRootsCoverage(home, planted, c.manifest), "; ")
		if got != c.want {
			t.Errorf("manifest %q: got %q, want %q", c.manifest, got, c.want)
		}
	}
}

// regression: #4683
// TestRunnerCopiesReplaceTargets: every directory a listed root replaces a
// module with has its go.mod copied into the image's module source, or the
// image's `go mod download` for that root fails at release time.
func TestRunnerCopiesReplaceTargets(t *testing.T) {
	raw, err := os.ReadFile("../images/verify-runner/Dockerfile")
	if err != nil {
		t.Fatal(err)
	}
	df := string(raw)
	roots, ok := bakedModuleRoots(filepath.Join("..", filepath.FromSlash(moduleRootsManifest)))
	if !ok {
		t.Fatalf("%s lists no module roots", moduleRootsManifest)
	}
	for _, r := range roots {
		for _, dir := range dirReplacements(t, filepath.Join("..", filepath.FromSlash(r), "go.mod")) {
			target := filepath.ToSlash(filepath.Clean(filepath.Join(r, dir)))
			if !strings.Contains(df, "COPY "+target+"/go.mod ") {
				t.Errorf("%s replaces a module with %s, whose go.mod the Dockerfile does not copy", r, target)
			}
		}
	}
}

// dirReplacements is the directory targets of a go.mod's replace directives,
// read with Go's own parser.
func dirReplacements(t *testing.T, gomod string) []string {
	t.Helper()
	raw, err := goModEditJSON(gomod, append(os.Environ(), "GOENV=off", "GOWORK=off"))
	if err != nil {
		t.Fatalf("go mod edit -json %s: %v", gomod, err)
	}
	var m struct {
		Replace []struct {
			New struct{ Path, Version string }
		}
	}
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, rp := range m.Replace {
		if rp.New.Version == "" {
			out = append(out, rp.New.Path)
		}
	}
	return out
}
