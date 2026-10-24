package container

import (
	"bufio"
	"go/ast"
	"go/parser"
	"go/token"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// startHUPTarget returns only after the shell installs its trap. Start alone
// proves that the process exists, not that sending HUP is safe for the fixture.
func startHUPTarget(t *testing.T, mark string, ignore bool) (*exec.Cmd, <-chan struct{}) {
	t.Helper()
	action := `echo hup >> "$0"`
	if ignore {
		action = ""
	}
	cmd := exec.Command("sh", "-c", `trap "$1" HUP; printf 'ready\n'; while :; do sleep 0.05; done`, mark, action)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	gone := make(chan struct{})
	go func() { _ = cmd.Wait(); close(gone) }()
	t.Cleanup(func() { _ = cmd.Process.Kill(); <-gone; _ = stdout.Close() })
	ready := make(chan string, 1)
	go func() { line, _ := bufio.NewReader(stdout).ReadString('\n'); ready <- line }()
	select {
	case line := <-ready:
		if line != "ready\n" {
			t.Fatalf("HUP target exited before trap readiness: %q", line)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("HUP target did not report trap readiness within 5s")
	}
	return cmd, gone
}

// Keep trap-dependent fixtures behind the handshake instead of letting a
// sibling fixture rediscover the Start-versus-shell-initialization race.
func hupFixtureSites(t *testing.T, filename string, src any) []string {
	t.Helper()
	f, err := parser.ParseFile(token.NewFileSet(), filename, src, 0)
	if err != nil {
		t.Fatal(err)
	}
	var sites []string
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil || (fn.Name.Name == "startHUPTarget" || fn.Name.Name == "TestHUPFixtureClass") {
			continue
		}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			lit, ok := n.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			s, err := strconv.Unquote(lit.Value)
			if err == nil && strings.Contains(s, "trap ") && strings.Contains(s, " HUP") {
				sites = append(sites, fn.Name.Name)
			}
			return true
		})
	}
	return sites
}

func TestHUPFixtureClass(t *testing.T) {
	files, err := filepath.Glob("*_test.go")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("no fixtures scanned")
	}
	for _, file := range files {
		if sites := hupFixtureSites(t, file, nil); len(sites) > 0 {
			t.Errorf("%s: unsynchronized HUP fixture(s): %v; use startHUPTarget", file, sites)
		}
	}
	// A second site must be found even when every current caller uses the helper.
	planted := `package container
 func second(){ exec.Command("sh","-c","trap '' HUP; sleep 30") }`
	sites := hupFixtureSites(t, "planted.go", planted)
	if len(sites) != 1 || sites[0] != "second" {
		t.Fatalf("class matcher missed planted fixture: %v", sites)
	}
	// Comments and a shell without a trap are not members of this class.
	if sites := hupFixtureSites(t, "clean.go", `package container
 // trap '' HUP
 func clean(){ exec.Command("sh","-c","echo ready") }`); len(sites) != 0 {
		t.Fatalf("unexpected class members: %v", sites)
	}
}
