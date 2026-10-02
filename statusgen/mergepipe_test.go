package main

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

func pipeChild(role, marker string, code int) *exec.Cmd {
	cmd := exec.Command(os.Args[0], "-test.run=^TestPipeChild$")
	cmd.Env = append(os.Environ(), "ASSAY_PIPE_CHILD="+role, "ASSAY_PIPE_MARKER="+marker, fmt.Sprintf("ASSAY_PIPE_EXIT=%d", code))
	return cmd
}

// The test executable is the portable subprocess fixture; no shell or real archive is needed.
func TestPipeChild(t *testing.T) {
	role := os.Getenv("ASSAY_PIPE_CHILD")
	if role == "" {
		return
	}
	for i := 0; i < 2048; i++ {
		fmt.Fprintln(os.Stderr, role+"-note")
	}
	if role == "archive" {
		fmt.Fprint(os.Stdout, "synthetic payload")
	} else {
		b, err := io.ReadAll(os.Stdin)
		if err != nil {
			os.Exit(8)
		}
		// An empty payload is legitimate only for the archive-start failure control.
		if len(b) > 0 && string(b) != "synthetic payload" {
			os.Exit(9)
		}
	}
	if marker := os.Getenv("ASSAY_PIPE_MARKER"); marker != "" {
		if err := os.WriteFile(marker, []byte("finished"), 0600); err != nil {
			os.Exit(10)
		}
	}
	var code int
	fmt.Sscan(os.Getenv("ASSAY_PIPE_EXIT"), &code)
	os.Exit(code)
}

func TestPipeDiagnostics(t *testing.T) {
	for _, tc := range []struct {
		name           string
		archive, untar int
		want           string
	}{
		{"archive_error", 3, 4, "git archive example-tree:"},
		{"untar_error", 0, 4, "tar -x:"},
		{"success", 0, 0, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			aMark, uMark := filepath.Join(dir, "archive-finished"), filepath.Join(dir, "untar-finished")
			a, u := pipeChild("archive", aMark, tc.archive), pipeChild("untar", uMark, tc.untar)
			err := materializeCommands("example-tree", a, u)
			if tc.want == "" {
				if err != nil {
					t.Fatal(err)
				}
			} else {
				if err == nil || !strings.HasPrefix(err.Error(), tc.want) {
					t.Fatalf("lost error precedence: %v", err)
				}
				for _, role := range []string{"archive", "untar"} {
					if n := strings.Count(err.Error(), role+"-note"); n != 2048 {
						t.Errorf("%s diagnostics retained=%d want2048", role, n)
					}
				}
			}
			for _, p := range []string{aMark, uMark} {
				if _, err := os.Stat(p); err != nil {
					t.Errorf("child did not complete: %v", err)
				}
			}
			if a.ProcessState == nil || u.ProcessState == nil {
				t.Fatal("returned before both children were waited")
			}
		})
	}
}

func TestPipeStartCleanup(t *testing.T) {
	t.Run("archive_start", func(t *testing.T) {
		marker := filepath.Join(t.TempDir(), "untar-finished")
		u := pipeChild("untar", marker, 0)
		// Clean up the intentionally exposed old-code failure without leaving a subprocess behind.
		t.Cleanup(func() {
			if u.Process != nil && u.ProcessState == nil {
				u.Process.Kill()
				u.Wait()
			}
		})
		a := exec.Command(filepath.Join(t.TempDir(), "missing-archive"))
		if err := materializeCommands("example-tree", a, u); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("archive start error lost: %v", err)
		}
		if u.ProcessState == nil {
			t.Fatal("archive start failure did not wait for extractor")
		}
		if _, err := os.Stat(marker); err != nil {
			t.Fatalf("extractor did not receive EOF and complete: %v", err)
		}
	})
	t.Run("untar_start", func(t *testing.T) {
		a := pipeChild("archive", "", 0)
		u := exec.Command(filepath.Join(t.TempDir(), "missing-untar"))
		if err := materializeCommands("example-tree", a, u); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("extractor start error lost: %v", err)
		}
		if a.Process != nil {
			t.Fatal("archive was started after extractor failed to start")
		}
	})
}

func TestPipeTreeCleanup(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TMPDIR", dir)
	t.Setenv("TMP", dir)
	t.Setenv("TEMP", dir)
	r := newFixtureRepo(t)
	r.write("payload.txt", "synthetic payload")
	r.commit("fixture")
	for _, tc := range []struct {
		name, tree, command string
		want                int
		materializeError    bool
	}{
		{"success", "HEAD", "cat payload.txt", 0, false},
		{"command_error", "HEAD", "exit 7", 7, false},
		{"archive_error", "missing-tree", "echo unexpected", 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			code, out, err := execOverTree(r.dir, tc.tree, tc.command)
			if (err != nil) != tc.materializeError || code != tc.want {
				t.Fatalf("code=%d error=%v output=%s", code, err, out)
			}
			if tc.name == "success" && out != "synthetic payload" {
				t.Fatalf("pipeline did not extract: %q", out)
			}
			paths, err := filepath.Glob(filepath.Join(dir, "statusgen-mergecheck-*"))
			if err != nil {
				t.Fatal(err)
			}
			if len(paths) != 0 {
				t.Fatalf("extraction directories leaked: %v", paths)
			}
		})
	}
}

// sharedCmdBuffers checks the direct shape: a local bytes.Buffer assigned by address
// to output fields on distinct named exec.Cmds in one function. It deliberately does
// not claim alias/global/interprocedural analysis, or that every such pair overlaps.
func sharedCmdBuffers(path string, src []byte) ([]string, error) {
	f, err := parser.ParseFile(token.NewFileSet(), path, src, 0)
	if err != nil {
		return nil, err
	}
	var findings []string
	imports := map[string]string{}
	for _, imp := range f.Imports {
		path, err := strconv.Unquote(imp.Path.Value)
		if err != nil {
			return nil, err
		}
		name := filepath.Base(path)
		if imp.Name != nil {
			name = imp.Name.Name
		}
		imports[name] = path
	}
	for _, d := range f.Decls {
		fn, ok := d.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		buffers, commands := map[string]bool{}, map[string]bool{}
		isType := func(e ast.Expr, pkg, name string) bool {
			s, ok := e.(*ast.SelectorExpr)
			if !ok {
				return false
			}
			x, ok := s.X.(*ast.Ident)
			if pkg == "exec" {
				pkg = "os/exec"
			}
			return ok && imports[x.Name] == pkg && s.Sel.Name == name
		}
		if fn.Type.Params != nil {
			for _, p := range fn.Type.Params.List {
				if ptr, ok := p.Type.(*ast.StarExpr); ok && isType(ptr.X, "exec", "Cmd") {
					for _, n := range p.Names {
						commands[n.Name] = true
					}
				}
			}
		}
		// Direct initializers occur in both var declarations and assignments.
		// No identifier-to-identifier propagation is attempted: aliases stay out
		// of this guard's explicitly bounded contract.
		initializer := func(name string, value ast.Expr) {
			if literal, ok := value.(*ast.CompositeLit); ok && isType(literal.Type, "bytes", "Buffer") {
				buffers[name] = true
			}
			if call, ok := value.(*ast.CallExpr); ok && (isType(call.Fun, "exec", "Command") || isType(call.Fun, "exec", "CommandContext")) {
				commands[name] = true
			}
		}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			switch v := n.(type) {
			case *ast.ValueSpec:
				for i, name := range v.Names {
					if isType(v.Type, "bytes", "Buffer") {
						buffers[name.Name] = true
					}
					commandType := v.Type
					if ptr, ok := commandType.(*ast.StarExpr); ok {
						commandType = ptr.X
					}
					if isType(commandType, "exec", "Cmd") {
						commands[name.Name] = true
					}
					if i < len(v.Values) {
						initializer(name.Name, v.Values[i])
					}
				}
			case *ast.AssignStmt:
				for i, value := range v.Rhs {
					if i >= len(v.Lhs) {
						continue
					}
					if name, ok := v.Lhs[i].(*ast.Ident); ok {
						initializer(name.Name, value)
					}
				}
			}
			return true
		})
		users := map[string]map[string]bool{}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			a, ok := n.(*ast.AssignStmt)
			if !ok {
				return true
			}
			for i, left := range a.Lhs {
				if i >= len(a.Rhs) {
					continue
				}
				field, ok := left.(*ast.SelectorExpr)
				if !ok || (field.Sel.Name != "Stderr" && field.Sel.Name != "Stdout") {
					continue
				}
				cmd, ok := field.X.(*ast.Ident)
				if !ok || !commands[cmd.Name] {
					continue
				}
				addr, ok := a.Rhs[i].(*ast.UnaryExpr)
				if !ok || addr.Op != token.AND {
					continue
				}
				buf, ok := addr.X.(*ast.Ident)
				if !ok || !buffers[buf.Name] {
					continue
				}
				if users[buf.Name] == nil {
					users[buf.Name] = map[string]bool{}
				}
				users[buf.Name][cmd.Name] = true
			}
			return true
		})
		for b, u := range users {
			if len(u) > 1 {
				findings = append(findings, fmt.Sprintf("%s:%s shares bytes.Buffer %s across commands", path, fn.Name.Name, b))
			}
		}
	}
	sort.Strings(findings)
	return findings, nil
}

func TestCmdBufferClass(t *testing.T) {
	const common = `package fixture
import("bytes";"os/exec")
func example(){ var aBuf,bBuf bytes.Buffer; a:=exec.Command("example"); b:=exec.Command("example"); _=b; _=bBuf; a.Stderr=&aBuf; `
	for _, tc := range []struct {
		name, tail string
		want       int
	}{
		{"shared_second_command", "b.Stderr=&aBuf }", 1},
		{"separate_buffers", "b.Stderr=&bBuf }", 0},
		{"one_command_two_streams", "a.Stdout=&aBuf }", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := sharedCmdBuffers("synthetic.go", []byte(common+tc.tail))
			if err != nil {
				t.Fatal(err)
			}
			if len(got) != tc.want {
				t.Fatalf("matcher findings=%v want%d", got, tc.want)
			}
		})
	}
	// Cover declaration forms as a cross-product, so a constructor or buffer form
	// cannot silently lose coverage when used with another ordinary declaration.
	for _, buffer := range []string{"var x,y bytes.Buffer", "var x,y = bytes.Buffer{},bytes.Buffer{}", "x,y := bytes.Buffer{},bytes.Buffer{}"} {
		for _, command := range []struct{ params, body string }{
			{"", `a,b := exec.Command("example"),exec.Command("example")`},
			{"", `var a,b = exec.Command("example"),exec.Command("example")`},
			{"", `var a,b = exec.CommandContext(context.Background(),"example"),exec.CommandContext(context.Background(),"example")`},
			{"a,b *exec.Cmd", ""},
			{"", `var a,b exec.Cmd`},
			{"", `var a,b *exec.Cmd = nil,nil`},
			{"", `var a,b *exec.Cmd; a=exec.Command("example"); b=exec.Command("example")`},
		} {
			for _, control := range []struct {
				name, tail string
				want       int
			}{
				{"shared", "b.Stderr=&x", 1},
				{"separate", "b.Stderr=&y", 0},
				{"same_command", "a.Stdout=&x", 0},
			} {
				src := `package fixture
import("bytes";"os/exec";"context")
func example(` + command.params + `){ ` + buffer + `; ` + command.body + `; _=a; _=b; _=y; a.Stderr=&x; ` + control.tail + ` }`
				got, err := sharedCmdBuffers("declarations.go", []byte(src))
				if err != nil {
					t.Fatal(err)
				}
				if len(got) != control.want {
					t.Errorf("buffer=%s commands=%s params=%s control=%s findings=%v want%d", buffer, command.body, command.params, control.name, got, control.want)
				}
			}
		}
	}

	// Only this module's shipped Go sources are covered. Cross-module siblings are
	// inventoried separately; CI execution depends on the recorded statusgen wiring.
	err := filepath.WalkDir(".", func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			if d.Name() == "testdata" || d.Name() == "vendor" || (path != "." && strings.HasPrefix(d.Name(), ".")) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		findings, err := sharedCmdBuffers(path, b)
		if err != nil {
			return err
		}
		for _, f := range findings {
			t.Error(f)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
