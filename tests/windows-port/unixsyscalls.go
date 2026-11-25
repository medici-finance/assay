// The Windows build split is enforced on syntax, so comments and fixture
// strings that describe a syscall do not count as a use of that syscall.
package main

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
)

func main() {
	count, err := checkTrees("statusgen", "tools/desk")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if count == 0 {
		fmt.Fprintln(os.Stderr, "no Unix-only syscall uses found")
		os.Exit(1)
	}
	fmt.Printf("OK %d\n", count)
}

func checkTrees(roots ...string) (int, error) {
	type targetPackage struct{ dir, arch string }
	ignoredByTarget := make(map[targetPackage][]string)
	count := 0
	for _, root := range roots {
		err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			name := entry.Name()
			if entry.IsDir() {
				if path != root && (name == "testdata" || name == "vendor" || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_")) {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(name, ".go") || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") {
				return nil
			}
			file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
			if err != nil {
				return err
			}
			if !usesUnixSyscall(file) {
				return nil
			}
			for _, arch := range []string{"amd64", "arm64"} {
				dir := filepath.Dir(path)
				key := targetPackage{dir, arch}
				ignored, ok := ignoredByTarget[key]
				if !ok {
					var err error
					ignored, err = windowsIgnoredFiles(dir, arch)
					if err != nil {
						return err
					}
					ignoredByTarget[key] = ignored
				}
				if !slices.Contains(ignored, name) {
					return fmt.Errorf("COMPILED ON windows/%s: %s", arch, path)
				}
			}
			count++
			return nil
		})
		if err != nil {
			return count, err
		}
	}
	return count, nil
}

func windowsIgnoredFiles(dir, arch string) ([]string, error) {
	cmd := exec.Command("go", "list", "-e", "-json", ".")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOOS=windows", "GOARCH="+arch, "CGO_ENABLED=1")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("could not check %s on windows/%s: %w", dir, arch, err)
	}
	var pkg struct{ IgnoredGoFiles []string }
	if err := json.Unmarshal(out, &pkg); err != nil {
		return nil, fmt.Errorf("invalid go list output for %s on windows/%s: %w", dir, arch, err)
	}
	return pkg.IgnoredGoFiles, nil
}

func usesUnixSyscall(file *ast.File) bool {
	var qualifier string
	var importsSyscall bool
	for _, imp := range file.Imports {
		if imp.Path.Value == `"syscall"` || imp.Path.Value == "`syscall`" {
			importsSyscall = true
			qualifier = "syscall"
			if imp.Name != nil {
				qualifier = imp.Name.Name
			}
			break
		}
	}
	if !importsSyscall || qualifier == "_" {
		return false
	}

	syscallName := func(expr ast.Expr) string {
		switch n := expr.(type) {
		case *ast.SelectorExpr:
			if pkg, ok := n.X.(*ast.Ident); ok && pkg.Name == qualifier && pkg.Obj == nil {
				return n.Sel.Name
			}
		case *ast.Ident:
			if qualifier == "." && n.Obj == nil {
				return n.Name
			}
		}
		return ""
	}

	found := false
	ast.Inspect(file, func(node ast.Node) bool {
		if found {
			return false
		}
		switch n := node.(type) {
		case *ast.CompositeLit:
			if syscallName(n.Type) == "SysProcAttr" {
				for _, elt := range n.Elts {
					if pair, ok := elt.(*ast.KeyValueExpr); ok {
						key, ok := pair.Key.(*ast.Ident)
						if ok && key.Name == "Setpgid" {
							found = true
						}
					}
				}
			}
		case *ast.SelectorExpr, *ast.Ident:
			switch syscallName(n.(ast.Expr)) {
			case "Flock", "Kill", "Stat_t":
				found = true
			}
		}
		return !found
	})
	return found
}
