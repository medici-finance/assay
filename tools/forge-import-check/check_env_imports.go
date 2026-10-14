// check_env_imports is the source-only check shared by brief 37 row 18 and
// brief 38 row 13. Run this file explicitly; it is not a shipped runtime.
// It never builds or executes the inspected files and does not use the network.
package main

import (
	"fmt"
	"go/parser"
	"go/token"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func main() {
	os.Exit(check(os.Args[1:], os.Stdout, os.Stderr))
}

func check(args []string, out, errs io.Writer) int {
	if len(args) != 1 || (args[0] != "forgeread/mirror" && args[0] != "forgeread/events") {
		fmt.Fprintln(errs, "could-not-check: expected forgeread/mirror or forgeread/events")
		return 2
	}
	// Reject links in the target and its ancestors before walking. The walker
	// also rejects links anywhere below it instead of silently omitting them.
	for dir := args[0]; dir != "."; dir = filepath.Dir(dir) {
		info, err := os.Lstat(dir)
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			fmt.Fprintf(errs, "could-not-check: target component %s is not a readable directory\n", dir)
			return 2
		}
	}
	// Rooted reads prevent a source path from escaping the selected subtree,
	// including a concurrent link replacement after the checks above.
	repo, err := os.OpenRoot(".")
	if err != nil {
		fmt.Fprintf(errs, "could-not-check: %v\n", err)
		return 2
	}
	defer repo.Close()
	source, err := repo.OpenRoot(args[0])
	if err != nil {
		fmt.Fprintf(errs, "could-not-check: %v\n", err)
		return 2
	}
	defer source.Close()
	files, forbidden := 0, 0
	fset := token.NewFileSet()
	err = fs.WalkDir(source.FS(), ".", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("%s: symbolic link in target", path)
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		if !entry.Type().IsRegular() {
			return fmt.Errorf("%s: not a regular source file", path)
		}
		// Parse source independently of build tags/platform selection. Full-file
		// parsing also refuses malformed source after the import declarations.
		data, err := fs.ReadFile(source.FS(), path)
		if err != nil {
			return err
		}
		file, err := parser.ParseFile(fset, filepath.Join(args[0], path), data, parser.SkipObjectResolution)
		if err != nil {
			return err
		}
		files++
		for _, spec := range file.Imports {
			importPath, err := strconv.Unquote(spec.Path.Value)
			if err != nil {
				return fmt.Errorf("%s: decode import: %w", path, err)
			}
			if importPath == "os" || importPath == "syscall" {
				forbidden++
				fmt.Fprintf(errs, "%s: forbidden import %s\n", fset.Position(spec.Pos()), importPath)
			}
		}
		return nil
	})
	if err != nil {
		fmt.Fprintf(errs, "could-not-check: %v\n", err)
		return 2
	}
	if files == 0 {
		fmt.Fprintln(errs, "could-not-check: no non-test Go sources")
		return 2
	}
	fmt.Fprintln(out, forbidden)
	if forbidden != 0 {
		return 1
	}
	return 0
}
