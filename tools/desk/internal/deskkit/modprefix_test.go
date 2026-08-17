package deskkit

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// requiredModulePrefix is the sole module-path prefix a go.mod registered in
// go.work's `use` block may declare.
//
// PR #485 renamed all nine Go module paths in this repo
// from github.com/medici-finance/assay/... to
// github.com/medici-finance/assay/.... Nothing in CI held that invariant for
// a module landing AFTER the rename (#491). The gap was not
// theoretical: PR #336 added a tenth module, tools/assess, whose go.mod still
// declared the OLD prefix, and every check that could have caught it stayed
// green — `lint` passed, `test (tools/assess)` passed, and `go build ./...`
// succeeded in every module, because nothing imports assess so the mismatch
// was invisible to the compiler. The only surface where it showed was the
// test binary's own `ok github.com/medici-finance/assay/tools/assess`
// line.
//
// An earlier one-time check enumerated the nine go.mod files BY NAME — a check
// a human ran at verification time, which cannot see a module added later.
const requiredModulePrefix = "github.com/medici-finance/assay/"

// TestGoModModulesUseAssayPrefix walks go.work's `use` block — the set of
// modules this repo actually builds and tests, per go.work itself rather
// than a hand-maintained list — and fails on any go.mod whose `module` line
// is not prefixed requiredModulePrefix.
//
// Deriving the set from go.work is the point of #491's fix: a
// module is covered the moment it is REGISTERED in go.work, with no
// checklist row to remember to add. Registered in
// ciCrossModuleRegistry (citrigger_test.go) so that an edit to go.work
// itself — which names no path under tools/** or statusgen/** — still runs
// this job; see that file's registry entry for this test.
func TestGoModModulesUseAssayPrefix(t *testing.T) {
	const repoRoot = "../../../.."

	goWorkPath := filepath.Join(repoRoot, "go.work")
	// go.work is do-not-copy for the public assay repo — it ships its own subset
	// go.work (publication/06), absent from this de-housed copy. Skip when it is
	// legitimately absent; the source repo always carries it, so the module-prefix
	// invariant still runs there in full.
	skipIfDehoused(t, goWorkPath,
		"go.work is do-not-copy for the public assay repo (it ships its own subset at publication/06)")


	raw, err := os.ReadFile(goWorkPath)
	if err != nil {
		t.Fatalf("cannot read %s: %v", goWorkPath, err)
	}

	dirs, err := parseGoWorkUseDirs(string(raw))
	if err != nil {
		t.Fatalf("cannot parse %s: %v", goWorkPath, err)
	}
	if len(dirs) == 0 {
		t.Fatalf("%s declares no `use` directories — this check proved nothing; "+
			"re-point parseGoWorkUseDirs at go.work's real shape", goWorkPath)
	}

	for _, dir := range dirs {
		dir := dir
		t.Run(dir, func(t *testing.T) {
			modPath := filepath.Join(repoRoot, filepath.FromSlash(dir), "go.mod")
			raw, err := os.ReadFile(modPath)
			if err != nil {
				t.Fatalf("go.work's use block names %s but its go.mod cannot be read: %v", dir, err)
			}
			module, err := goModModulePath(string(raw))
			if err != nil {
				t.Fatalf("%s: %v", modPath, err)
			}
			if !strings.HasPrefix(module, requiredModulePrefix) {
				t.Errorf("%s declares `module %s`, which is not prefixed %q.\n"+
					"Every module go.work registers must live under the %s namespace "+
					"(#491, PR #485 renamed the other nine). Fix go.mod's "+
					"module line; do not add an exception here.",
					modPath, module, requiredModulePrefix, requiredModulePrefix)
			}
		})
	}
}

// goWorkUseBlockRe matches the block form of a `use` directive:
//
//	use (
//		./foo
//		./bar
//	)
var goWorkUseBlockRe = regexp.MustCompile(`(?s)use\s*\(\s*(.*?)\)`)

// goWorkUseLineRe matches the single-line form of a `use` directive
// (`use ./foo`), which go.work's own syntax permits alongside the block
// form and which this parser must not silently miss.
var goWorkUseLineRe = regexp.MustCompile(`(?m)^use\s+(\S+)\s*$`)

// parseGoWorkUseDirs extracts the directories a go.work file's `use`
// directive(s) name, in either form, de-duplicated and in the order first
// seen. It does not validate that the directories exist — the caller does
// that by trying to read each one's go.mod, so a stale entry surfaces as a
// clear "cannot be read" failure rather than a silent skip here.
func parseGoWorkUseDirs(content string) ([]string, error) {
	var dirs []string
	seen := map[string]bool{}

	add := func(dir string) {
		if dir == "" || seen[dir] {
			return
		}
		seen[dir] = true
		dirs = append(dirs, dir)
	}

	if m := goWorkUseBlockRe.FindStringSubmatch(content); m != nil {
		for _, line := range strings.Split(m[1], "\n") {
			line = strings.TrimSpace(line)
			if i := strings.Index(line, "//"); i >= 0 {
				line = strings.TrimSpace(line[:i])
			}
			add(line)
		}
	}

	// Strip the block (if any) before scanning for single-line `use`
	// directives, so nothing inside the block can be double-matched by the
	// line-anchored regexp below.
	rest := goWorkUseBlockRe.ReplaceAllString(content, "")
	for _, m := range goWorkUseLineRe.FindAllStringSubmatch(rest, -1) {
		add(strings.TrimSpace(m[1]))
	}

	return dirs, nil
}

// goModModuleRe matches go.mod's `module <path>` directive line.
var goModModuleRe = regexp.MustCompile(`(?m)^module\s+(\S+)\s*$`)

// goModModulePath extracts the `module` directive's path from a go.mod
// file's content. It errors rather than returning "" when the directive is
// absent — reading a malformed go.mod as "no prefix violation" would be
// exactly the false green this guard exists to prevent.
func goModModulePath(content string) (string, error) {
	m := goModModuleRe.FindStringSubmatch(content)
	if m == nil {
		return "", fmt.Errorf("no `module` directive found")
	}
	return m[1], nil
}

// TestParseGoWorkUseDirs covers parseGoWorkUseDirs against synthetic
// content, both forms go.work syntax allows, so a parser that silently
// degrades is caught here rather than by the absence of a failure in
// TestGoModModulesUseAssayPrefix above.
func TestParseGoWorkUseDirs(t *testing.T) {
	t.Run("block form", func(t *testing.T) {
		const content = `go 1.25.0

use (
	./statusgen
	./tools/assess // trailing comment
	./tools/desk
)
`
		got, err := parseGoWorkUseDirs(content)
		if err != nil {
			t.Fatalf("parseGoWorkUseDirs: %v", err)
		}
		want := []string{"./statusgen", "./tools/assess", "./tools/desk"}
		if !equalStrings(got, want) {
			t.Errorf("parseGoWorkUseDirs(block form) = %v, want %v", got, want)
		}
	})

	t.Run("single-line form", func(t *testing.T) {
		const content = `go 1.25.0

use ./statusgen
use ./tools/assess
`
		got, err := parseGoWorkUseDirs(content)
		if err != nil {
			t.Fatalf("parseGoWorkUseDirs: %v", err)
		}
		want := []string{"./statusgen", "./tools/assess"}
		if !equalStrings(got, want) {
			t.Errorf("parseGoWorkUseDirs(single-line form) = %v, want %v", got, want)
		}
	})

	t.Run("no use directive", func(t *testing.T) {
		got, err := parseGoWorkUseDirs("go 1.25.0\n")
		if err != nil {
			t.Fatalf("parseGoWorkUseDirs: %v", err)
		}
		if len(got) != 0 {
			t.Errorf("parseGoWorkUseDirs(no use) = %v, want empty", got)
		}
	})
}

// TestGoModModulePath covers goModModulePath against synthetic content,
// including the absent-directive error path that TestGoModModulesUseAssayPrefix
// depends on to fail closed rather than read a malformed go.mod as clean.
func TestGoModModulePath(t *testing.T) {
	got, err := goModModulePath("module github.com/medici-finance/assay/tools/assess\n\ngo 1.25.0\n")
	if err != nil {
		t.Fatalf("goModModulePath: %v", err)
	}
	if got != "github.com/medici-finance/assay/tools/assess" {
		t.Errorf("goModModulePath = %q", got)
	}

	if _, err := goModModulePath("go 1.25.0\n\nrequire foo v1.0.0\n"); err == nil {
		t.Error("goModModulePath on content with no `module` line must error, not return \"\"")
	}
}

// TestModulePrefixViolationIsDetected is the negative control: it feeds a
// go.mod declaring a module path OUTSIDE the required prefix (the same kind of
// non-conforming prefix PR #336's pre-#485 rename fixed) through
// goModModulePath + the same prefix check TestGoModModulesUseAssayPrefix
// runs, and asserts the violation is actually flagged. Without this, a
// prefix check that always passed (e.g. a typo'd constant) would look
// identical to a working one — every real go.mod in this repo is already
// compliant, so the positive path alone proves nothing.
func TestModulePrefixViolationIsDetected(t *testing.T) {
	const staleGoMod = "module github.com/example-org/legacy/tools/assess\n\ngo 1.25.0\n"

	module, err := goModModulePath(staleGoMod)
	if err != nil {
		t.Fatalf("goModModulePath: %v", err)
	}
	if strings.HasPrefix(module, requiredModulePrefix) {
		t.Fatalf("negative control is broken: %q unexpectedly matches %q — "+
			"the fixture must reproduce the pre-#485 prefix PR #336 landed with, "+
			"or this test proves nothing", module, requiredModulePrefix)
	}
}
