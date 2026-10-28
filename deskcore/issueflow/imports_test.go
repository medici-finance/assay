package issueflow

import (
	"archive/zip"
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	return root
}

// goCheck runs a child go command in dir. Workspace resolution is always
// disabled. Where dir sits decides how modules resolve:
//   - inside deskcore, the module under test: lookup is disabled, at no cost,
//     because deskcore requires no modules;
//   - in another module of this repository (a consumer): the job's proxy is
//     inherited and -mod=readonly holds the child to that module's own go.sum.
//     deskcore's cache does not hold a consumer's dependencies: the release
//     leg tests deskcore before anything has fetched statusgen's (#4234);
//   - anywhere else (a temporary module): lookup is disabled unless env
//     names the caller's own hermetic proxy.
func goCheck(t *testing.T, dir string, env []string, names []string, args ...string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	root := repoRoot(t)
	base := []string{"GOWORK=off", "GOPROXY=off", "GOSUMDB=off"}
	if within(root, dir) && !within(filepath.Join(root, "deskcore"), dir) {
		base = base[:1]
		args = append([]string{args[0], "-mod=readonly"}, args[1:]...)
	}
	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), base...)
	cmd.Env = append(cmd.Env, env...)
	raw, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("consumer %s: %v\n%s", filepath.Base(dir), err, raw)
	}
	for _, name := range names {
		if !strings.Contains(string(raw), "--- PASS: "+name+" ") {
			t.Fatalf("missing production PASS %s\n%s", name, raw)
		}
	}
}

// within reports whether path is dir or lies beneath it.
func within(dir, path string) bool {
	rel, err := filepath.Rel(dir, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}

func checkConsumers(t *testing.T) {
	t.Helper()
	root := repoRoot(t)
	for _, c := range []struct {
		dir, pkg, pattern string
		names             []string
	}{
		{"statusgen", ".", "TestHandoffScanner|TestIntakeExitSchema_MatchesDoc", []string{"TestHandoffScanner", "TestIntakeExitSchema_MatchesDoc"}},
		{"tools/desk", "./cmd/issueboard", "TestProjectionJourney|TestProjectionFailures|TestBoardLabelHistoryHold|TestTrustGate_BlessThenEdit", []string{"TestProjectionJourney", "TestProjectionFailures", "TestBoardLabelHistoryHold", "TestTrustGate_BlessThenEdit"}},
		{"tools/desk", "./cmd/fanoutloop", "TestPlanLeadsWithAddressed|TestUnaddressedRowsUnaffected", []string{"TestPlanLeadsWithAddressed", "TestUnaddressedRowsUnaffected"}},
		{"tools/desk", "./cmd/scanloop", "TestIntakeExitSchema_MatchesDoc", []string{"TestIntakeExitSchema_MatchesDoc"}},
		{"tools/desk", "./cmd/deskinbox", "TestDiscoveryJourney", []string{"TestDiscoveryJourney"}},
	} {
		goCheck(t, filepath.Join(root, c.dir), nil, c.names, "test", "-count=1", "-v", "-timeout=45s", "-run", "^("+c.pattern+")$", c.pkg)
	}
}

// regression: class verification-handoff
func TestHandoffModuleImports(t *testing.T) {
	root := repoRoot(t)
	// Check the actual consumer routes, not an import-only demonstration program.
	for _, c := range []struct{ path, imp, call string }{
		{"statusgen/scanissues.go", "github.com/medici-finance/assay/deskcore/issueflow", "Select"},
		{"tools/desk/internal/issueflow/selection.go", "github.com/medici-finance/assay/deskcore/issueflow", "Select"},
		{"tools/desk/cmd/issueboard/board.go", "github.com/medici-finance/assay/tools/desk/internal/issueflow", "Select"},
		{"tools/desk/cmd/deskinbox/query.go", "github.com/medici-finance/assay/tools/desk/internal/issueflow", "InboxRank"},
		{"tools/desk/cmd/fanoutloop/adapter.go", "github.com/medici-finance/assay/deskcore/issueflow", "Select"},
	} {
		f, err := parser.ParseFile(token.NewFileSet(), filepath.Join(root, c.path), nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		alias := ""
		for _, imp := range f.Imports {
			path, _ := strconv.Unquote(imp.Path.Value)
			if path == c.imp {
				alias = filepath.Base(path)
				if imp.Name != nil {
					alias = imp.Name.Name
				}
			}
		}
		if alias == "" {
			t.Fatalf("consumer route missing: %s", c.path)
		}
		called := false
		ast.Inspect(f, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectorExpr)
			if ok {
				id, ok := sel.X.(*ast.Ident)
				if ok && id.Name == alias && sel.Sel.Name == c.call {
					called = true
				}
			}
			return true
		})
		if !called {
			t.Fatalf("consumer bypassed owner: %s", c.path)
		}
	}
	// Reuse the module's canonical transitive-import, clock, JSON and environment checks.
	goCheck(t, filepath.Join(root, "deskcore"), nil, nil, "test", "-count=1", "-v", "-timeout=45s", "-run", "^Test(PurePackagesHaveNoEffectfulTransitiveImports|PurePackagesFollowTheCallRules|NoKnobReadsOutsideConfig)$", "./archtest")

	// Build both real module consumers with workspace resolution disabled.
	for _, c := range []struct{ dir, pkg string }{{"statusgen", "."}, {"tools/desk", "./cmd/issueboard"}, {"tools/desk", "./cmd/fanoutloop"}} {
		goCheck(t, filepath.Join(root, c.dir), nil, nil, "build", "-o", filepath.Join(t.TempDir(), "consumer"), c.pkg)
	}
	standalone(t, root)
}

func standalone(t *testing.T, root string) {
	t.Helper()
	dir := t.TempDir()
	proxy := filepath.Join(dir, "proxy")
	module := "github.com/medici-finance/assay/deskcore"
	version := "v0.0.1"
	vdir := filepath.Join(proxy, filepath.FromSlash(module), "@v")
	if err := os.MkdirAll(vdir, 0700); err != nil {
		t.Fatal(err)
	}
	mod, err := os.ReadFile(filepath.Join(root, "deskcore/go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string][]byte{version + ".mod": mod, version + ".info": []byte(`{"Version":"v0.0.1","Time":"2026-10-23T00:00:00Z"}`), "list": []byte(version + "\n")} {
		if err = os.WriteFile(filepath.Join(vdir, name), body, 0600); err != nil {
			t.Fatal(err)
		}
	}
	archive, err := os.Create(filepath.Join(vdir, version+".zip"))
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(archive)
	err = filepath.WalkDir(filepath.Join(root, "deskcore"), func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(filepath.Join(root, "deskcore"), path)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		w, err := zw.Create(module + "@" + version + "/" + filepath.ToSlash(rel))
		if err != nil {
			return err
		}
		_, err = w.Write(data)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if err = zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err = archive.Close(); err != nil {
		t.Fatal(err)
	}
	consumer := filepath.Join(dir, "external")
	if err = os.Mkdir(consumer, 0700); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{"go.mod": "module example.org/handoff-consumer\n\ngo 1.27.0\n\nrequire " + module + " " + version + "\n", "main.go": `package main
import p "github.com/medici-finance/assay/deskcore/issueflow"
func main() { if p.Select(p.Scanner,p.CandidateForLabels(p.Candidate{},[]string{"to:worker"}),"") { panic("addressed work leaked") }; if p.AssessHandoff(p.Handoff{},p.HandoffFacts{}).State!="unknown" {panic("legacy accepted")} }
`}
	for name, body := range files {
		if err = os.WriteFile(filepath.Join(consumer, name), []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	// Isolated cache and a file proxy: no workspace or relative replace rescues it.
	env := []string{"GOPROXY=file://" + filepath.ToSlash(proxy), "GOSUMDB=off", "GOMODCACHE=" + filepath.Join(dir, "modules"), "GONOPROXY=none", "GOPRIVATE=", "GONOSUMDB=*", "GOFLAGS=-mod=mod -modcacherw"}
	goCheck(t, consumer, env, nil, "run", ".")
	// A consumer expecting an API this version lacks must fail to compile.
	if err = os.WriteFile(filepath.Join(consumer, "main.go"), []byte("package main\nimport p \""+module+"/issueflow\"\nfunc main(){p.AcceptRepair()}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "build", ".")
	cmd.Dir = consumer
	cmd.Env = append(os.Environ(), env...)
	cmd.Env = append(cmd.Env, "GOWORK=off")
	raw, err := cmd.CombinedOutput()
	if err == nil || !strings.Contains(string(raw), "undefined: p.AcceptRepair") {
		t.Fatalf("unsupported API did not fail: %v %s", err, raw)
	}
}
