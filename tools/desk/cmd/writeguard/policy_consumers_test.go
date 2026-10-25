package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/medici-finance/assay/gitquiet/goquiet"
	"github.com/medici-finance/assay/tools/desk/internal/arch"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"
)

const policyModule = "github.com/medici-finance/assay/deskcore"

// consumerInventory inspects production declarations/imports, not prose hits.
// It is a source-bound candidate check, not a proof that arbitrary differently
// named code cannot duplicate policy. Independent review owns that wider claim.
func consumerInventory(tree fs.FS) ([]string, error) {
	var consumers []string
	err := fs.WalkDir(tree, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if p != "." && (strings.HasPrefix(d.Name(), ".") || d.Name() == "vendor" || d.Name() == "testdata") {
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
			return nil
		}
		b, err := fs.ReadFile(tree, p)
		if err != nil {
			return err
		}
		f, err := parser.ParseFile(token.NewFileSet(), p, b, 0)
		if err != nil {
			return err
		}
		for _, imp := range f.Imports {
			v, _ := strconv.Unquote(imp.Path.Value)
			if v == policyModule+"/writepolicy" {
				consumers = append(consumers, p)
			}
		}
		if !strings.HasPrefix(p, "deskcore/writepolicy/") {
			for _, decl := range f.Decls {
				switch d := decl.(type) {
				case *ast.FuncDecl:
					switch strings.ToLower(d.Name.Name) {
					case "checkbash", "checkfilepath", "checkclaimsentinel", "scanwriteindicators", "sharedhomed":
						return fmt.Errorf("unregistered policy decision %s in %s", d.Name.Name, p)
					}
				case *ast.GenDecl:
					for _, spec := range d.Specs {
						if v, ok := spec.(*ast.ValueSpec); ok {
							for _, name := range v.Names {
								if name.Name == "indicatorSpecs" {
									return fmt.Errorf("unregistered indicator owner in %s", p)
								}
							}
						}
					}
				}
			}
		}
		return nil
	})
	sort.Strings(consumers)
	return consumers, err
}

// regression: #3752
func TestWritePolicyConsumers(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", "..", "..", ".."))
	want := []string{"tools/desk/cmd/writeguard/guard.go", "tools/desk/cmd/writeguard/observe.go"}
	got, err := consumerInventory(os.DirFS(root))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("consumer inventory changed: got %v want %v", got, want)
	}
	doc, err := os.ReadFile(filepath.Join(root, "docs", "write-policy-consumers.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range want {
		if !bytes.Contains(doc, []byte(p)) {
			t.Errorf("consumer %s absent from source census", p)
		}
	}
	for _, p := range []string{"Dockerfile", "images/assay-harness/Dockerfile"} {
		b, err := os.ReadFile(filepath.Join(root, p))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Contains(b, []byte("COPY deskcore")) {
			t.Errorf("%s does not include producer module", p)
		}
	}
	t.Run("unregistered-production-decision", func(t *testing.T) {
		planted := fstest.MapFS{"other/guard.go": {Data: []byte("package other\nfunc CheckBash(command string) bool { return true }\n")}}
		if _, err := consumerInventory(planted); err == nil || !strings.Contains(err.Error(), "unregistered") {
			t.Fatalf("planted owner escaped: %v", err)
		}
	})
	t.Run("missing-and-extra-import", func(t *testing.T) {
		for _, tree := range []fstest.MapFS{{}, {"other/use.go": {Data: []byte("package other\nimport _ \"" + policyModule + "/writepolicy\"\n")}}} {
			got, err := consumerInventory(tree)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Join(got, "\n") == strings.Join(want, "\n") {
				t.Fatal("stale inventory passed")
			}
		}
	})
	t.Run("semantic-owner", func(t *testing.T) {
		b, err := os.ReadFile(filepath.Join(root, "docs", "contracts.md"))
		if err != nil {
			t.Fatal(err)
		}
		index, err := arch.ParseIndex(bytes.NewReader(b))
		if err != nil {
			t.Fatal(err)
		}
		var rows []arch.SRow
		for _, row := range index {
			if row.ID == "S-write-policy" {
				rows = append(rows, row)
			}
		}
		if len(rows) != 1 || len(rows[0].Owners) != 1 || rows[0].Owners[0] != "deskcore/writepolicy/policy.go" {
			t.Fatalf("owner drifted: %+v", rows)
		}
		b, err = os.ReadFile(filepath.Join(root, "tools", "desk", "internal", "arch", "markers.txt"))
		if err != nil {
			t.Fatal(err)
		}
		ceilings, err := arch.ParseCeilings(bytes.NewReader(b))
		if err != nil {
			t.Fatal(err)
		}
		if ceilings["S-write-policy"] != 1 {
			t.Fatal("one owner ceiling changed")
		}
		for _, dir := range []string{"deskcore", "tools/desk"} {
			markers, violations, err := arch.Markers(os.DirFS(root), dir, index)
			if err != nil {
				t.Fatal(err)
			}
			var ours []arch.Marker
			for _, m := range markers {
				if m.ID == "S-write-policy" {
					ours = append(ours, m)
				}
			}
			more, _ := arch.Ratchet(ours, rows, map[string]int{"S-write-policy": 1}, dir)
			violations = append(violations, more...)
			for _, v := range violations {
				t.Error(v)
			}
			want := 0
			if dir == "deskcore" {
				want = 1
			}
			if len(ours) != want {
				t.Errorf("%s has %d owners, want %d", dir, len(ours), want)
			}
		}
	})
	t.Run("external-offline-module", func(t *testing.T) { externalPolicyConsumer(t, filepath.Join(root, "deskcore")) })
}

func externalPolicyConsumer(t *testing.T, source string) {
	t.Helper()
	// This is an intentionally synthetic LOCAL PROXY fixture version. It is
	// neither a proposed public release nor proof a release exists.
	const version = "v0.0.0-fixture"
	base := t.TempDir()
	proxy := filepath.Join(base, "proxy")
	versions := filepath.Join(proxy, filepath.FromSlash(policyModule), "@v")
	if err := os.MkdirAll(versions, 0o755); err != nil {
		t.Fatal(err)
	}
	mod, err := os.ReadFile(filepath.Join(source, "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(versions, version+".mod"), mod, 0o600); err != nil {
		t.Fatal(err)
	}
	info, _ := json.Marshal(map[string]string{"Version": version, "Time": "2026-10-20T00:00:00Z"})
	if err := os.WriteFile(filepath.Join(versions, version+".info"), info, 0o600); err != nil {
		t.Fatal(err)
	}
	var zipped bytes.Buffer
	zw := zip.NewWriter(&zipped)
	err = filepath.WalkDir(source, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(source, p)
		if err != nil {
			return err
		}
		w, err := zw.Create(policyModule + "@" + version + "/" + filepath.ToSlash(rel))
		if err != nil {
			return err
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		_, err = w.Write(b)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(versions, version+".zip"), zipped.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	consumer := filepath.Join(base, "consumer")
	if err := os.Mkdir(consumer, 0o755); err != nil {
		t.Fatal(err)
	}
	consumerMod := "module example.invalid/writepolicy-consumer\n\ngo 1.27.0\n\nrequire " + policyModule + " " + version + "\n"
	if err := os.WriteFile(filepath.Join(consumer, "go.mod"), []byte(consumerMod), 0o600); err != nil {
		t.Fatal(err)
	}
	program := `package main
import ("fmt"; p "github.com/medici-finance/assay/deskcore/writepolicy")
func main() {
 c:=p.Classify(p.Request{Tool:"Write",Target:"/shared/new"},p.LexicalContext{Platform:p.POSIX,Cwd:"/owned"})
 in:=p.Evaluation{Classified:c,ClassificationID:c.ID(),Claim:p.ClaimObservation{Source:p.NoClaim},Policy:p.Policy{Profile:p.LegacyHook,SharedRoot:"/shared",ProjectDir:"/owned"},Callout:p.CalloutObservation{State:p.CalloutUnconfigured}}
 for n:=0;n<20;n++ {
  out:=p.Evaluate(in)
  if len(out.Need.Paths)==0 {
   if out.Decision!=p.Deny || out.Reason!=p.SharedDenied { panic("shared write was not refused") }
   in.Policy.Profile=p.Managed
   if p.Evaluate(in).Decision!=p.Unknown { panic("managed inherited legacy admission") }
   fmt.Println(p.APIVersion+" shared=deny managed-missing=unknown");return
  }
  for _,q:=range out.Need.Paths { in.Paths=append(in.Paths,p.PathObservation{Input:q.Input,State:p.PathResolved,Resolved:q.Input,Ancestor:q.Input,ResolvedAncestor:q.Input}) }
 }
 panic("observation plan did not settle")
}
`
	if err := os.WriteFile(filepath.Join(consumer, "main.go"), []byte(program), 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "run", "-modcacherw", "-mod=mod", ".")
	cmd.Dir = consumer
	cmd.Env = append(os.Environ(), "GOWORK=off", "GOENV=off", "GOFLAGS=", "GOVCS=off", "GOTOOLCHAIN=local", "GOPROXY=file://"+filepath.ToSlash(proxy), "GOSUMDB=off", "GOPRIVATE=", "GONOPROXY=", "GONOSUMDB=", "GOMODCACHE="+filepath.Join(base, "modcache"))
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := goquiet.Cmd(cmd).Output()
	if err != nil {
		t.Fatalf("external consumer: %v\n%s", err, stderr.String())
	}
	if string(out) != "writepolicy/v1 shared=deny managed-missing=unknown\n" {
		t.Fatalf("consumer output %q", out)
	}
	resolved, err := os.ReadFile(filepath.Join(consumer, "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(resolved, []byte("replace")) {
		t.Fatal("external qualification used a source replacement")
	}
	t.Log("external GOWORK=off consumer resolved supplied file-proxy zip into an empty module cache; no replace; synthetic fixture only")
}
