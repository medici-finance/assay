package workref_test

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"go/parser"
	"go/token"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// regression: #4219
func TestWorkrefPublicImportBoundary(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	// Inspect every production source, including a newly planted second file.
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		tree, err := parser.ParseFile(token.NewFileSet(), file, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}
		for _, imp := range tree.Imports {
			name, err := strconv.Unquote(imp.Path.Value)
			if err != nil {
				t.Fatal(err)
			}
			switch name {
			case "fmt", "regexp", "strconv", "strings", "unicode":
			default:
				t.Errorf("pure workref boundary: %s imports %s", file, name)
			}
		}
	}
	// Walk the actual compiled closure as well. Standard fmt has OS implementation
	// details; workref cannot import those effect APIs itself or any external module.
	out, err := runGo(t, filepath.Join(root, "statusgen"), nil, "list", "-deps", "-json", "./workref")
	if err != nil {
		t.Fatalf("dependency walk: %v\n%s", err, out)
	}
	decoder := json.NewDecoder(strings.NewReader(out))
	for {
		var pkg struct {
			ImportPath string
			Standard   bool
			Error      any
		}
		err := decoder.Decode(&pkg)
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if pkg.Error != nil {
			t.Fatalf("unreadable dependency: %+v", pkg)
		}
		if !pkg.Standard && pkg.ImportPath != "github.com/medici-finance/assay/statusgen/workref" {
			t.Errorf("non-stdlib dependency: %s", pkg.ImportPath)
		}
		if pkg.ImportPath == "net/http" || pkg.ImportPath == "os/exec" || strings.Contains(pkg.ImportPath, "credential") || strings.Contains(pkg.ImportPath, "tools/desk/internal") {
			t.Errorf("forbidden dependency: %s", pkg.ImportPath)
		}
	}
	// Both real consumers must compile and exercise their production adapters with
	// workspace assistance disabled and go.sum pinning (consumerGo). Positive
	// named guards reject an absent test.
	for _, consumer := range []struct{ dir, pkg, test string }{
		{"statusgen", "./contexthandoff", "TestLegacyQIDParity"},
		{"tools/desk", "./internal/deskkit", "TestSharedWorkrefTrailerParity"},
	} {
		out, err := consumerGo(t, filepath.Join(root, consumer.dir), "test", "-count=1", "-timeout=45s", "-v", "-run", "^"+consumer.test+"$", consumer.pkg)
		if err != nil || !strings.Contains(out, "--- PASS: "+consumer.test+" ") {
			t.Errorf("production consumer %s: %v\n%s", consumer.dir, err, out)
		}
	}
	// External consumer via a file proxy: actual manifest, no workspace/replace in
	// the consumer, no public release claimed. Only the pure package closure is used.
	proxy := t.TempDir()
	const mod = "github.com/medici-finance/assay/statusgen"
	const version = "v0.0.0-20261023000000-000000000001"
	var archive bytes.Buffer
	zw := zip.NewWriter(&archive)
	add := func(name string) {
		data, err := os.ReadFile(filepath.Join(root, "statusgen", name))
		if err != nil {
			t.Fatal(err)
		}
		w, err := zw.Create(mod + "@" + version + "/" + filepath.ToSlash(name))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	add("go.mod")
	for _, pkg := range []string{"workref", "contexthandoff"} {
		files, err := filepath.Glob(filepath.Join(root, "statusgen", pkg, "*.go"))
		if err != nil {
			t.Fatal(err)
		}
		for _, file := range files {
			if !strings.HasSuffix(file, "_test.go") {
				add(filepath.Join(pkg, filepath.Base(file)))
			}
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, "statusgen", "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(proxy, filepath.FromSlash(mod), "@v")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, data := range map[string][]byte{"list": []byte(version + "\n"), version + ".mod": data, version + ".zip": archive.Bytes(), version + ".info": []byte(fmt.Sprintf(`{"Version":%q,"Time":"2026-10-23T00:00:00Z"}`, version))} {
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	consumer := t.TempDir()
	if err := os.WriteFile(filepath.Join(consumer, "go.mod"), []byte("module example.test/workref-consumer\ngo 1.27.0\nrequire "+mod+" "+version+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(consumer, "main.go"), []byte(externalMain), 0o600); err != nil {
		t.Fatal(err)
	}
	out, err = runGo(t, consumer, []string{"GOPROXY=file://" + filepath.ToSlash(proxy), "GOMODCACHE=" + filepath.Join(consumer, "cache"), "GOFLAGS=-mod=mod -modcacherw"}, "run", ".")
	if err != nil || !strings.HasSuffix(out, "shared identity and exact revision\n") {
		t.Fatalf("external consumer: %v\n%s", err, out)
	}
	after, err := os.ReadFile(filepath.Join(consumer, "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(after), "replace") {
		t.Error("external consumer required a replace")
	}
}

// runGo runs a child go command fully offline. The dependency walk needs no
// module outside the standard library, and the external consumer names its own
// file proxy and module cache in extra.
func runGo(t *testing.T, dir string, extra []string, args ...string) (string, error) {
	t.Helper()
	env := []string{"GOWORK=off", "GOPROXY=off", "GOSUMDB=off", "GOTOOLCHAIN=local", "HTTP_PROXY=http://127.0.0.1:1", "HTTPS_PROXY=http://127.0.0.1:1", "NO_PROXY=", "GOPRIVATE=", "GONOPROXY=none"}
	return goChild(t, dir, append(env, extra...), args...)
}

// consumerGo tests a real consumer module of this repository. Those packages
// import third-party modules (yaml in contexthandoff; go-git and others in
// desk) that a cold module cache does not hold, so lookup cannot be disabled
// (#4238). Workspace resolution stays off; the job's proxy and checksum
// database are inherited, and -mod=readonly holds the child to its own
// module's go.mod and go.sum.
func consumerGo(t *testing.T, dir string, args ...string) (string, error) {
	t.Helper()
	args = append([]string{args[0], "-mod=readonly"}, args[1:]...)
	return goChild(t, dir, []string{"GOWORK=off", "GOTOOLCHAIN=local"}, args...)
}

func goChild(t *testing.T, dir string, env []string, args ...string) (string, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 65*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

const externalMain = `package main
import (
 "fmt"
 "github.com/medici-finance/assay/statusgen/workref"
)
func main() {
 reg,err:=workref.NewRegistry([]workref.Root{{Cell:"example",Repo:"tools"}},nil);if err!=nil {panic(err)}
 resolved,err:=workref.Resolve("build:b-11@2",&workref.Context{Cell:"example",Repo:"tools"},reg,workref.TypedBrief)
 if err!=nil || resolved.Revision!=2 {panic("lost revision")}
 qid,err:=workref.Resolve(resolved.Compatibility,nil,workref.Registry{},workref.LegacyQualified)
 if err!=nil || qid.Compatibility!="example:tools:build:11" {panic("lost identity")}
 if _,err:=workref.Resolve("build:b-11@2",nil,workref.Registry{},workref.LegacyQualified);err==nil {panic("legacy profile broadened")}
 fmt.Println("shared identity and exact revision")
}
`
