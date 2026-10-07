package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

// admittedFixture is a home whose one tracked file holds attested bytes, a
// second tree whose copy of that file is changed, and a deskdispatch on PATH
// that admits any run. It returns the home and the second tree.
func admittedFixture(t *testing.T, admit bool) (string, string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("POSIX-only: the fake checker is a #!/bin/sh script on PATH")
	}
	root := t.TempDir()
	gitInit(t, root, "fixture", "fixture@example.invalid")
	if err := os.WriteFile(filepath.Join(root, "source.txt"), []byte("attested source\n"), 0600); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "add", "source.txt")
	runGit(t, root, "commit", "-m", "fixture")
	other := t.TempDir()
	if err := os.WriteFile(filepath.Join(other, "source.txt"), []byte("TAMPERED source\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if admit {
		bin := t.TempDir()
		receipt := `{"Issue":7,"Binding":{"Run":"r","Repo":"example-org/one","Source":"s","Brief":"brief.md","Model":"m","Tier":"strong"}}`
		if err := os.WriteFile(filepath.Join(bin, "deskdispatch"), []byte("#!/bin/sh\necho '"+receipt+"'\n"), 0700); err != nil {
			t.Fatal(err)
		}
		t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
		t.Setenv("DESK_LOOP", "verify-desk")
	} else {
		t.Setenv("DESK_LOOP", "")
	}
	return root, other
}

// verifyrunRow runs one Verify row through verifyrun and returns its exit.
func verifyrunRow(t *testing.T, root, row string) int {
	t.Helper()
	tick := string(rune(96))
	brief := filepath.Join(t.TempDir(), "brief.md")
	body := "# Fixture\n\n## Verify\n\n| # | Command | Expect |\n|---|---|---|\n| 1 | " + tick + row + tick + " | exit 0 |\n\n## Evidence\n\n"
	if err := os.WriteFile(brief, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	got, stderr := captureVerifyrun(t, []string{"--brief", brief, "--root", root, "--dry-run"})
	if got.code == verifyrunExitCouldNot {
		t.Fatalf("row could not run: %s", stderr)
	}
	return got.code
}

// TestAdmittedRowsStripGitEnv: an admitted run's rows read the home admission
// compared, whatever GIT_* variables the caller's environment carries. The
// unadmitted control proves the plant lands: the same row in the same
// environment reads the changed tree there.
func TestAdmittedRowsStripGitEnv(t *testing.T) {
	const row = "git grep -q TAMPERED; test $? -eq 1"
	for _, admit := range []bool{true, false} {
		t.Run(map[bool]string{true: "admitted", false: "control"}[admit], func(t *testing.T) {
			root, other := admittedFixture(t, admit)
			t.Setenv("GIT_DIR", filepath.Join(root, ".git"))
			t.Setenv("GIT_WORK_TREE", other)
			code := verifyrunRow(t, root, row)
			if admit && code != verifyrunExitPass {
				t.Fatalf("admitted row read the caller's GIT_WORK_TREE, not the home (exit %d)", code)
			}
			if !admit && code != verifyrunExitFail {
				t.Fatalf("plant did not land: unadmitted row did not read GIT_WORK_TREE (exit %d)", code)
			}
		})
	}
}

// TestAdmittedRowsPinGrep: grep's pattern settings in the home's config cannot
// change what an admitted row's git grep matches. The unadmitted control
// proves the setting changes the match there.
func TestAdmittedRowsPinGrep(t *testing.T) {
	const row = "git grep -q 'attested.source'"
	for _, admit := range []bool{true, false} {
		t.Run(map[bool]string{true: "admitted", false: "control"}[admit], func(t *testing.T) {
			root, _ := admittedFixture(t, admit)
			runGit(t, root, "config", "grep.patternType", "fixed")
			code := verifyrunRow(t, root, row)
			if admit && code != verifyrunExitPass {
				t.Fatalf("admitted row's git grep followed the home's grep.patternType (exit %d)", code)
			}
			if !admit && code != verifyrunExitFail {
				t.Fatalf("plant did not land: grep.patternType=fixed did not change the match (exit %d)", code)
			}
		})
	}
}

// TestAdmittedRowEnvShape pins the admitted environment one variable at a
// time: no inherited GIT_* variable survives, whatever its case, and only the
// settings admission reads with plus the grep pins are present.
func TestAdmittedRowEnvShape(t *testing.T) {
	in := []string{"PATH=/bin", "GIT_DIR=x", "GIT_WORK_TREE=x", "GIT_INDEX_FILE=x", "GIT_CONFIG_COUNT=1", "GIT_CONFIG_KEY_0=core.worktree", "GIT_CONFIG_VALUE_0=x", "GIT_CONFIG_PARAMETERS='core.worktree'='x'", "Git_Dir=x", "GIT_CEILING_DIRECTORIES=x"}
	want := []string{"PATH=/bin", "GIT_NO_REPLACE_OBJECTS=1", "GIT_ATTR_NOSYSTEM=1", "GIT_CONFIG_COUNT=2", "GIT_CONFIG_KEY_0=grep.patternType", "GIT_CONFIG_VALUE_0=default", "GIT_CONFIG_KEY_1=grep.extendedRegexp", "GIT_CONFIG_VALUE_1=false"}
	if got := admittedRowEnv(in); !slices.Equal(got, want) {
		t.Fatalf("admitted row environment:\n got %q\nwant %q", got, want)
	}
}

// rowEnvAllowed names every function that may read the caller's environment
// with os.Environ: the row environment's single source (shellPlan.rowEnv), the
// run that chooses it (runVerifyrun), the sandbox helper re-exec (whose env is
// the row's own, inherited) and the container hand-off (which runs no row on
// the host).
var rowEnvAllowed = []string{"rowEnv", "runVerifyrun", "runNetnsHelper", "runInContainer"}

// environCallers returns "file: func" for every os.Environ call outside the
// allowed functions in the given sources.
func environCallers(t *testing.T, files map[string]string) []string {
	t.Helper()
	var stray []string
	fset := token.NewFileSet()
	for name, src := range files {
		f, err := parser.ParseFile(fset, name, src, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, d := range f.Decls {
			fn, ok := d.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				sel, ok := n.(*ast.SelectorExpr)
				if ok && sel.Sel.Name == "Environ" {
					if id, ok := sel.X.(*ast.Ident); ok && id.Name == "os" && !slices.Contains(rowEnvAllowed, fn.Name.Name) {
						stray = append(stray, name+": "+fn.Name.Name)
					}
				}
				return true
			})
		}
	}
	return stray
}

// TestRowEnvSingleSource is the class guard: a Verify row runs in the run's
// row environment (shellPlan.rowEnv), so no other function may hand a process
// the caller's environment, which would let a row escape the admitted one. The
// planted second runner proves the guard sees such a site.
func TestRowEnvSingleSource(t *testing.T) {
	names, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]string{}
	for _, name := range names {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		b, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		files[name] = string(b)
	}
	if stray := environCallers(t, files); len(stray) > 0 {
		t.Fatalf("os.Environ outside the row environment's source: %v", stray)
	}
	planted := map[string]string{"planted.go": "package main\nimport (\"os\"; \"os/exec\")\nfunc runRowAgain(c string) { cmd := exec.Command(\"sh\", \"-c\", c); cmd.Env = os.Environ(); _ = cmd.Run() }\n"}
	if stray := environCallers(t, planted); !slices.Equal(stray, []string{"planted.go: runRowAgain"}) {
		t.Fatalf("guard blind to a planted second row runner: %v", stray)
	}
}
