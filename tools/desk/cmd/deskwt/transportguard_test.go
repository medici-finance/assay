package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Credential-only provisioning bypasses the URL reset and transport read-back.
// References (including aliases) to the helper may occur only in the transport
// provisioner; inspect all production files in this package, where it is private.
func bareCredentialRefs(name string, src any) ([]string, error) {
	fs := token.NewFileSet()
	file, err := parser.ParseFile(fs, name, src, 0)
	if err != nil {
		return nil, err
	}
	var found []string
	for _, decl := range file.Decls {
		var node ast.Node = decl
		if fn, ok := decl.(*ast.FuncDecl); ok {
			if fn.Name.Name == "wireRoleTransport" {
				continue
			}
			node = fn.Body // the helper declaration itself is not a reference
		}
		if node == nil {
			continue
		}
		ast.Inspect(node, func(n ast.Node) bool {
			id, ok := n.(*ast.Ident)
			if ok && id.Name == "wireRoleCredential" {
				found = append(found, fmt.Sprint(fs.Position(id.Pos())))
			}
			return true
		})
	}
	return found, nil
}

func TestTransportClassGuard(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil || len(files) == 0 {
		t.Fatalf("production file inventory: %v", err)
	}
	for _, path := range files {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		found, err := bareCredentialRefs(path, nil)
		if err != nil {
			t.Fatal(err)
		}
		for _, at := range found {
			t.Errorf("credential-only provisioning at %s; use wireRoleTransport", at)
		}
	}
}

func TestTransportGuardControl(t *testing.T) {
	plant, err := os.ReadFile("testdata/transport-bypass.go.txt")
	if err != nil {
		t.Fatal(err)
	}
	found, err := bareCredentialRefs("planted.go", plant)
	if err != nil || len(found) != 2 {
		t.Fatalf("guard must detect planted call and alias: %v %v", found, err)
	}
}
