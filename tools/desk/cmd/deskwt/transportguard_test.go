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
			if fn.Recv == nil && fn.Name.Name == "wireRoleTransport" {
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
	want := []string{
		"planted.go:4:2", "planted.go:7:21",
		"planted.go:18:2", "planted.go:19:12",
		"planted.go:25:2", "planted.go:26:12",
	}
	if err != nil || strings.Join(found, "\n") != strings.Join(want, "\n") {
		t.Fatalf("guard must detect ordinary and receiver calls/aliases only: got %v, want %v; error=%v", found, want, err)
	}
}
