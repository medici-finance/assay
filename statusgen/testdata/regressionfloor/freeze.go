package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"gopkg.in/yaml.v3"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

func main() {
	p := "testdata/regressionfloor/inventory.yaml"
	b, e := os.ReadFile(p)
	must(e)
	var inv map[string]any
	must(yaml.Unmarshal(b, &inv))
	for _, v := range inv["entries"].([]any) {
		row := v.(map[string]any)
		object, e := pinnedObject(row["source"].(string), row["file"].(string))
		must(e)
		b, e := exec.Command("git", "--no-replace-objects", "show", object).Output()
		must(e)
		f, e := parser.ParseFile(token.NewFileSet(), "baseline.go", b, 0)
		must(e)
		digest, e := pinnedBody(f, row["test"].(string))
		must(e)
		row["body_sha256"] = digest
	}
	b, e = json.MarshalIndent(inv, "", "  ")
	must(e)
	must(os.WriteFile(p, append(b, '\n'), 0644))
}
func must(e error) {
	if e != nil {
		panic(e)
	}
}

func pinnedObject(source, file string) (string, error) {
	if !regexp.MustCompile(`^[0-9a-f]{40}$`).MatchString(source) {
		return "", fmt.Errorf("invalid pinned source")
	}
	if file == "" || filepath.IsAbs(file) || strings.Contains(file, "..") {
		return "", fmt.Errorf("invalid pinned path")
	}
	return source + ":" + file, nil
}
func pinnedBody(file *ast.File, name string) (string, error) {
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Recv == nil && fn.Name.Name == name && fn.Type.Params.NumFields() == 1 {
			var out bytes.Buffer
			if err := format.Node(&out, token.NewFileSet(), fn.Body); err != nil {
				return "", err
			}
			return fmt.Sprintf("%x", sha256.Sum256(out.Bytes())), nil
		}
	}
	return "", fmt.Errorf("missing pinned source test")
}
