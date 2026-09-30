package gitcore

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// claimref_guard_test.go — the CLASS guard for "a server refusal arrives without the server's
// words".
//
// The defect: a plumbing receive-pack request that never negotiates the sideband gives the
// server no channel to say why it refused, so the refusal reaches the caller as one bare word
// ("failure") and the cause is unrecoverable from any log. The fix is one helper,
// captureRemoteMessages, that every such request must pass through. This guard walks every
// non-test Go file under tools/desk, finds every function that BUILDS a receive-pack request
// (packp.NewReferenceUpdateRequest / NewReferenceUpdateRequestFromCapabilities), and fails
// naming any such function that does not also call captureRemoteMessages — so the next ref
// writer added anywhere in the tree is red here, not discovered at the next unexplained
// refusal. A planted instance in testdata/remotecapture is the positive control: if the matcher
// ever stops matching, the control fails rather than the guard reporting clean.

// receivePackBuilders are the packp constructors that start a receive-pack request.
var receivePackBuilders = map[string]bool{
	"NewReferenceUpdateRequest":                 true,
	"NewReferenceUpdateRequestFromCapabilities": true,
}

// sitesMissingRemoteCapture returns "<file>:<func>" for every function in src that builds a
// receive-pack request without calling captureRemoteMessages.
func sitesMissingRemoteCapture(t *testing.T, fset *token.FileSet, name string, src []byte) []string {
	t.Helper()
	f, err := parser.ParseFile(fset, name, src, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", name, err)
	}
	var out []string
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		builds, captures := false, false
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			switch fun := call.Fun.(type) {
			case *ast.SelectorExpr:
				if x, ok := fun.X.(*ast.Ident); ok && x.Name == "packp" && receivePackBuilders[fun.Sel.Name] {
					builds = true
				}
				if fun.Sel.Name == "captureRemoteMessages" {
					captures = true
				}
			case *ast.Ident:
				if fun.Name == "captureRemoteMessages" {
					captures = true
				}
			}
			return true
		})
		if builds && !captures {
			out = append(out, name+":"+fn.Name.Name)
		}
	}
	return out
}

func TestEveryReceivePackRequestCapturesServerMessages(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	var missing []string
	builders := 0
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, werr error) error {
		if werr != nil {
			return werr
		}
		if d.IsDir() {
			if d.Name() == "testdata" || d.Name() == "vendor" || strings.HasPrefix(d.Name(), ".") && path != root {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		src, rerr := os.ReadFile(path)
		if rerr != nil {
			return rerr
		}
		if !strings.Contains(string(src), "NewReferenceUpdateRequest") {
			return nil
		}
		builders++
		rel, _ := filepath.Rel(root, path)
		missing = append(missing, sitesMissingRemoteCapture(t, fset, rel, src)...)
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	if builders == 0 {
		t.Fatal("found no file that builds a receive-pack request — the walk is not reading the tree it guards")
	}
	if len(missing) > 0 {
		t.Fatalf("receive-pack request built without captureRemoteMessages (a server refusal from here "+
			"would arrive as a bare status word, its reason discarded): %s", strings.Join(missing, ", "))
	}
}

// TestRemoteCaptureGuardFlagsPlantedInstance is the positive control: the planted file builds a
// request without the capture, and the guard's matcher must name it.
func TestRemoteCaptureGuardFlagsPlantedInstance(t *testing.T) {
	src, err := os.ReadFile(filepath.Join("testdata", "remotecapture", "planted.go.txt"))
	if err != nil {
		t.Fatal(err)
	}
	got := sitesMissingRemoteCapture(t, token.NewFileSet(), "planted.go", src)
	if len(got) != 1 || got[0] != "planted.go:plantedRequest" {
		t.Fatalf("guard on the planted instance = %v, want exactly [planted.go:plantedRequest]", got)
	}
}
