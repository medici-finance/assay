package regression

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

var starters = []int{322, 643, 656, 687, 697, 708, 719, 727, 757, 772, 773, 786, 999, 1007, 1033, 1034, 1056, 1067, 1086, 1145, 1146, 1223}
var commitRE = regexp.MustCompile(`^[0-9a-fA-F]{7,40}$`)
var newTestRE = regexp.MustCompile(`^TestReg[0-9]+[A-Z][A-Za-z0-9]*$`)

// validateManifest checks the register against parsed Go declarations in BOTH
// modules. It accumulates errors, so a missing seed cannot mask a missing test.
func validateManifest(body, root string) []string {
	var faults []string
	seen := map[int]string{}
	dropped := false
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(line, "## Dropped") {
			dropped = true
		}
		cells := strings.Split(line, "|")
		if len(cells) < 4 || !strings.HasPrefix(strings.TrimSpace(cells[1]), "#") {
			continue
		}
		id, err := strconv.Atoi(strings.TrimPrefix(strings.TrimSpace(cells[1]), "#"))
		if err != nil {
			faults = append(faults, "invalid issue: "+cells[1])
			continue
		}
		kind := "seed"
		if dropped {
			kind = "drop"
		}
		if old, ok := seen[id]; ok {
			faults = append(faults, fmt.Sprintf("#%d appears twice (%s/%s)", id, old, kind))
		}
		seen[id] = kind
		if dropped {
			if strings.TrimSpace(cells[2]) == "" {
				faults = append(faults, fmt.Sprintf("#%d empty drop reason", id))
			}
			continue
		}
		if len(cells) != 7 {
			faults = append(faults, fmt.Sprintf("#%d malformed seed", id))
			continue
		}
		behavior, pkg, name, sha := strings.TrimSpace(cells[2]), strings.TrimSpace(cells[3]), strings.TrimSpace(cells[4]), strings.TrimSpace(cells[5])
		if behavior == "" {
			faults = append(faults, fmt.Sprintf("#%d empty behavior", id))
		}
		if !commitRE.MatchString(sha) {
			faults = append(faults, fmt.Sprintf("#%d invalid fixing commit", id))
		}
		if strings.HasPrefix(name, "TestReg") && (!newTestRE.MatchString(name) || len(name) > 31) {
			faults = append(faults, fmt.Sprintf("#%d invalid new test %s", id, name))
		}
		clean := filepath.ToSlash(filepath.Clean(pkg))
		if (clean != strings.TrimSuffix(pkg, "/")) || !(strings.HasPrefix(pkg, "tools/desk/") || strings.HasPrefix(pkg, "statusgen/")) {
			faults = append(faults, fmt.Sprintf("#%d invalid package %s", id, pkg))
			continue
		}
		dir := filepath.Join(root, filepath.FromSlash(pkg))
		files, err := filepath.Glob(filepath.Join(dir, "*_test.go"))
		if err != nil {
			faults = append(faults, err.Error())
			continue
		}
		found := false
		for _, file := range files {
			f, err := parser.ParseFile(token.NewFileSet(), file, nil, 0)
			if err != nil {
				faults = append(faults, fmt.Sprintf("cannot parse %s: %v", pkg, err))
				continue
			}
			for _, decl := range f.Decls {
				if fn, ok := decl.(*ast.FuncDecl); ok && fn.Recv == nil && fn.Name.Name == name {
					found = true
				}
			}
		}
		if !found {
			faults = append(faults, fmt.Sprintf("#%d missing test %s in %s", id, name, pkg))
		}
	}
	for _, id := range starters {
		if _, ok := seen[id]; !ok {
			faults = append(faults, fmt.Sprintf("missing starter #%d", id))
		}
	}
	return faults
}

func TestRegressionManifest(t *testing.T) {
	root, err := filepath.Abs("../../../..")
	if err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile("MANIFEST.md")
	if err != nil {
		t.Fatal(err)
	}
	if faults := validateManifest(string(body), root); len(faults) > 0 {
		t.Fatal(strings.Join(faults, "\n"))
	}
	bad, err := os.ReadFile("testdata/incomplete.md")
	if err != nil {
		t.Fatal(err)
	}
	faults := strings.Join(validateManifest(string(bad), root), "\n")
	for _, want := range []string{"missing starter #643", "missing test TestReg656Absent"} {
		if !strings.Contains(faults, want) {
			t.Fatalf("positive control missed %q: %s", want, faults)
		}
	}
}

func TestManifestRules(t *testing.T) {
	root, err := filepath.Abs("../../../..")
	if err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile("MANIFEST.md")
	if err != nil {
		t.Fatal(err)
	}
	good := string(body)
	cases := []struct{ name, body, want string }{
		{"missing", regexp.MustCompile(`(?m)^\| #643 .*\n`).ReplaceAllString(good, ""), "missing starter #643"},
		{"duplicate", good + "\n| #643 | duplicate |\n", "#643 appears twice"},
		{"sha", strings.Replace(good, "0276ce0a5", "not-a-sha", 1), "#643 invalid fixing commit"},
		{"missing-function", strings.Replace(good, "TestReg727WorktreeOrigin", "TestReg727Absent", 1), "missing test TestReg727Absent"},
		{"new-name", strings.Replace(good, "TestReg727WorktreeOrigin", "TestReg727bad", 1), "invalid new test TestReg727bad"},
		{"traversal", strings.Replace(good, "tools/desk/cmd/deskclaim-ref", "tools/desk/../desk/cmd/deskclaim-ref", 1), "invalid package"},
		{"empty-drop", strings.Replace(good, "| #719 | Documentation-only close; no code fix. |", "| #719 | |", 1), "#719 empty drop reason"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			faults := strings.Join(validateManifest(c.body, root), "\n")
			if !strings.Contains(faults, c.want) {
				t.Fatalf("want %q, got %q", c.want, faults)
			}
		})
	}
}
