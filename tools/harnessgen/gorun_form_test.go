package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The repository has no root go.mod or go.work, so `go run ./tools/<x>` fails
// with "go.mod file not found" before it reaches the tool. Every tool lives in
// its own module and must be run from inside it. This is the class behind the
// remediation text #880 repaired: any place that tells an operator to run one
// of the generator/lint tools by its repo-root-relative path.
//
// The literal is assembled from parts so this file does not trip its own scan.
var rootRelativeGoRun = regexp.MustCompile("go run \\./" + "tools/(harnessgen|skillslint|plugindrift|harnesslint)\\b")

// scanRootRelativeGoRun returns every offending span in src.
func scanRootRelativeGoRun(src string) []string {
	return rootRelativeGoRun.FindAllString(src, -1)
}

// regression: #880
func TestScanFlagsPlantedRootRelativeGoRun(t *testing.T) {
	planted := "run `go run ./" + "tools/skillslint --sync` and commit"
	if got := scanRootRelativeGoRun(planted); len(got) != 1 {
		t.Fatalf("positive control: scan found %v in a planted instance, want exactly one", got)
	}
	clean := "run `cd tools/skillslint && go run . --sync --root ../..` and commit"
	if got := scanRootRelativeGoRun(clean); len(got) != 0 {
		t.Fatalf("scan flagged the module-scoped form: %v", got)
	}
}

// regression: #880
func TestNoRootRelativeGoRunInToolText(t *testing.T) {
	root := filepath.Join("..", "..")
	// Surfaces that print or carry remediation advice for these tools. Fixtures
	// under testdata and Verify-row evidence in docs/streams are historical
	// record, not advice, and are left out.
	dirs := []string{
		"tools/harnessgen", "tools/skillslint", "tools/plugindrift", "tools/harnesslint",
		"statusgen", "plugins/assay", ".cursor/skills",
	}
	scanned := 0
	for _, d := range dirs {
		base := filepath.Join(root, d)
		err := filepath.WalkDir(base, func(p string, e fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if e.IsDir() {
				if e.Name() == "testdata" {
					return fs.SkipDir
				}
				return nil
			}
			if strings.HasSuffix(p, "gorun_form_test.go") {
				return nil
			}
			b, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			scanned++
			for _, hit := range scanRootRelativeGoRun(string(b)) {
				t.Errorf("%s: %q does not run in this multi-module tree (no root go.mod); use `cd tools/<x> && go run . … --root ../..`", p, hit)
			}
			return nil
		})
		if err != nil {
			t.Fatalf("could-not-check: walking %s: %v", base, err)
		}
	}
	if scanned == 0 {
		t.Fatal("could-not-check: scanned zero files")
	}
}
