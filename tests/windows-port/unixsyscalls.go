// The Windows build split is enforced on syntax, so comments and fixture
// strings that describe a syscall do not count as a use of that syscall.
package main

import (
	"fmt"
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
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
				target := build.Default
				target.GOOS, target.GOARCH, target.CgoEnabled = "windows", arch, true
				included, err := target.MatchFile(filepath.Dir(path), name)
				if err != nil {
					return err
				}
				if included {
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
