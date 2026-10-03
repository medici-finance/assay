package runner_test

import (
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

// Defect class: a refusal that formats request or adapter content into its
// error text. The contract's errors may carry only sentinels (%w), numbers
// (%d) and a small set of identifiers whose values the contract itself defines
// (the wrapped err, a defined field path, the version constant). Anything else
// formatted with %s, %v or %q, or a format string that is not a literal, is a
// potential echo of untrusted payload.
var allowedFormatted = map[string]bool{"err": true, "path": true, "sub": true, "name": true, "Version": true}

var verb = regexp.MustCompile(`%[-+# 0]*[0-9]*(?:\.[0-9]+)?([a-zA-Z%])`)

// payloadEchoes reports every fmt.Errorf call in src that could format
// content the contract does not define into an error.
func payloadEchoes(file string, src []byte) ([]string, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, file, src, 0)
	if err != nil {
		return nil, err
	}
	var bad []string
	ast.Inspect(f, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "Errorf" {
			return true
		}
		if pkg, ok := sel.X.(*ast.Ident); !ok || pkg.Name != "fmt" {
			return true
		}
		at := fset.Position(call.Pos()).String()
		lit, ok := call.Args[0].(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			bad = append(bad, at+": format is not a literal")
			return true
		}
		format, err := strconv.Unquote(lit.Value)
		if err != nil {
			bad = append(bad, at+": format does not unquote")
			return true
		}
		arg := 1
		for _, m := range verb.FindAllStringSubmatch(format, -1) {
			v := m[1]
			if v == "%" {
				continue
			}
			if arg >= len(call.Args) {
				bad = append(bad, at+": more verbs than arguments")
				return true
			}
			a := call.Args[arg]
			arg++
			switch v {
			case "w", "d":
			case "s", "v":
				if id, ok := a.(*ast.Ident); !ok || !allowedFormatted[id.Name] {
					bad = append(bad, at+": %"+v+" formats an expression the contract does not define")
				}
			default:
				bad = append(bad, at+": %"+v+" is not allowed in a refusal")
			}
		}
		return true
	})
	return bad, nil
}

// TestNoPayloadInErrors runs the class guard over every non-test source file
// of the contract package, after a positive control shows it flags the shapes
// the review found.
func TestNoPayloadInErrors(t *testing.T) {
	planted := []byte(`package p
import "fmt"
func f(r struct{ Outcome, Name string }, k string) error {
	_ = fmt.Errorf("%w: %q", errX, r.Outcome)
	_ = fmt.Errorf("%w: artifact %s", errX, r.Name)
	_ = fmt.Errorf("%w: extension %v", errX, k)
	return fmt.Errorf("%w: generation %d", errX, 1)
}`)
	bad, err := payloadEchoes("planted.go", planted)
	if err != nil {
		t.Fatal(err)
	}
	if len(bad) != 3 {
		t.Fatalf("positive control: want 3 echoes flagged, got %d: %v", len(bad), bad)
	}

	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	checked := 0
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		src, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		bad, err := payloadEchoes(file, src)
		if err != nil {
			t.Fatal(err)
		}
		for _, b := range bad {
			t.Error(b)
		}
		checked++
	}
	if checked < 5 {
		t.Fatalf("class guard checked only %d source files", checked)
	}
}
