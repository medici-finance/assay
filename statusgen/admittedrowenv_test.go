package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
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
// time: no inherited GIT_* variable outside its narrowing form survives,
// whatever its case, no shell startup variable survives, a caller's widening
// value of a pinned setting gives way to the pin, and only the settings
// admission reads with plus the grep pins are added.
func TestAdmittedRowEnvShape(t *testing.T) {
	in := []string{"PATH=/bin", "GIT_DIR=x", "GIT_WORK_TREE=x", "GIT_INDEX_FILE=x", "GIT_CONFIG_COUNT=1", "GIT_CONFIG_KEY_0=core.worktree", "GIT_CONFIG_VALUE_0=x", "GIT_CONFIG_PARAMETERS='core.worktree'='x'", "Git_Dir=x", "GIT_CEILING_DIRECTORIES=x", "BASH_ENV=x", "ENV=x", "BASH_FUNC_git%%=x", "Bash_Func_ls%%=x", "SHELLOPTS=x", "BASHOPTS=x", "PS4=x", "GIT_CONFIG_NOSYSTEM=0", "GIT_TERMINAL_PROMPT=1", "GIT_ASKPASS=helper"}
	want := []string{"PATH=/bin", "GIT_NO_REPLACE_OBJECTS=1", "GIT_ATTR_NOSYSTEM=1", "GIT_CONFIG_NOSYSTEM=1", "GIT_TERMINAL_PROMPT=0", "GIT_ASKPASS=", "GIT_CONFIG_COUNT=2", "GIT_CONFIG_KEY_0=grep.patternType", "GIT_CONFIG_VALUE_0=default", "GIT_CONFIG_KEY_1=grep.extendedRegexp", "GIT_CONFIG_VALUE_1=false"}
	if got := admittedRowEnv(in); !slices.Equal(got, want) {
		t.Fatalf("admitted row environment:\n got %q\nwant %q", got, want)
	}
}

// rowEnvAllowed names every function that may read the caller's environment
// with os.Environ: the row environment's single source (shellPlan.rowEnv), the
// run that chooses it (runVerifyrun), the sandbox helper re-exec (whose env is
// the row's own, inherited), the container hand-off (which runs no row on
// the host) and the provenance git reads' environment (historyGitEnv: it only
// wraps historyGit's literal `git` reads, which start no Verify row, and adds
// GIT_GRAFT_FILE to what a fixed-tool launch would inherit anyway).
var rowEnvAllowed = []string{"rowEnv", "runVerifyrun", "runNetnsHelper", "runInContainer", "historyGitEnv"}

// rowLaunchShells are the programs a Verify row's command text runs under.
var rowLaunchShells = []string{"sh", "bash", "pwsh", "powershell", "cmd"}

// implicitEnvAllowed names every function that may start a shell or a program
// it does not name literally without setting the process environment, so the
// process inherits the caller's: the statusgen self re-runs of the lint and
// diff-lint audits, the forge reader binary, the merge check (which runs the
// operator's own command line, not a Verify row) and the probes that run only
// `exit 0` or `true` to learn what the host supports.
var implicitEnvAllowed = []string{"productionDiffLintRunner", "productionLintRunner", "OpenIssues", "readItem", "execOverTree", "probeShell", "networkOffWrapper", "netnsFacility"}

// environCallers returns "file: func" for every os.Environ call outside the
// allowed functions in the given sources, and for every function outside
// implicitEnvAllowed that starts a shell, or a program it does not name
// literally, without assigning a process environment (an unset Env inherits
// the caller's environment as surely as os.Environ does). The second check is
// per function: it asks whether the function sets an Env at all, not which
// command the setting belongs to.
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
			launches, setsEnv := false, false
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				switch n := n.(type) {
				case *ast.SelectorExpr:
					if id, ok := n.X.(*ast.Ident); ok && id.Name == "os" && n.Sel.Name == "Environ" && !slices.Contains(rowEnvAllowed, fn.Name.Name) {
						stray = append(stray, name+": "+fn.Name.Name)
					}
				case *ast.AssignStmt:
					for _, lhs := range n.Lhs {
						if sel, ok := lhs.(*ast.SelectorExpr); ok && sel.Sel.Name == "Env" {
							setsEnv = true
						}
					}
				case *ast.CallExpr:
					launches = launches || shellOrUnnamedLaunch(n)
				}
				return true
			})
			if launches && !setsEnv && !slices.Contains(implicitEnvAllowed, fn.Name.Name) {
				stray = append(stray, name+": "+fn.Name.Name+" (inherits)")
			}
		}
	}
	slices.Sort(stray)
	return stray
}

// shellOrUnnamedLaunch reports whether call is exec.Command or
// exec.CommandContext starting a shell or a program not named literally.
func shellOrUnnamedLaunch(call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || (sel.Sel.Name != "Command" && sel.Sel.Name != "CommandContext") {
		return false
	}
	if id, ok := sel.X.(*ast.Ident); !ok || id.Name != "exec" {
		return false
	}
	at := 0
	if sel.Sel.Name == "CommandContext" {
		at = 1
	}
	if len(call.Args) <= at {
		return false
	}
	lit, ok := call.Args[at].(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return true
	}
	program, err := strconv.Unquote(lit.Value)
	return err != nil || slices.Contains(rowLaunchShells, program)
}

// TestRowEnvSingleSource is the class guard: a Verify row runs in the run's
// row environment (shellPlan.rowEnv), so no other function may hand a process
// the caller's environment, which would let a row escape the admitted one,
// whether explicitly (os.Environ) or by leaving the process environment unset.
// The planted runners prove the guard sees each kind of site: one passing
// os.Environ, one starting a shell with no Env, one starting an argv-named
// program with no Env, beside a fixed-tool launch it must leave alone.
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
		t.Fatalf("caller's environment reaches a process outside the row environment's source: %v", stray)
	}
	planted := map[string]string{"planted.go": "package main\nimport (\"context\"; \"os\"; \"os/exec\")\n" +
		"func runRowAgain(c string) { cmd := exec.Command(\"sh\", \"-c\", c); cmd.Env = os.Environ(); _ = cmd.Run() }\n" +
		"func runRowInherit(c string) { cmd := exec.Command(\"bash\", \"-c\", c); _ = cmd.Run() }\n" +
		"func runArgvInherit(a []string) { _ = exec.CommandContext(context.Background(), a[0], a[1:]...).Run() }\n" +
		"func readHead() { _ = exec.Command(\"git\", \"rev-parse\", \"HEAD\").Run() }\n"}
	want := []string{"planted.go: runArgvInherit (inherits)", "planted.go: runRowAgain", "planted.go: runRowInherit (inherits)"}
	if stray := environCallers(t, planted); !slices.Equal(stray, want) {
		t.Fatalf("guard blind to a planted second row runner:\n got %q\nwant %q", stray, want)
	}
}

// TestAdmittedRowsShellEnv: what the caller's environment asks the row's shell
// to run at startup (a BASH_ENV file, an exported function) cannot point an
// admitted row's git at another tree. The unadmitted control proves each
// plant lands.
func TestAdmittedRowsShellEnv(t *testing.T) {
	const row = "git grep -q TAMPERED; test $? -eq 1"
	for _, mode := range []string{"bash-env", "bash-func"} {
		for _, admit := range []bool{true, false} {
			t.Run(mode+"/"+map[bool]string{true: "admitted", false: "control"}[admit], func(t *testing.T) {
				root, other := admittedFixture(t, admit)
				redirect := "GIT_DIR='" + filepath.Join(root, ".git") + "' GIT_WORK_TREE='" + other + "'"
				switch mode {
				case "bash-env":
					startup := filepath.Join(t.TempDir(), "startup.sh")
					if err := os.WriteFile(startup, []byte("export "+redirect+"\n"), 0600); err != nil {
						t.Fatal(err)
					}
					t.Setenv("BASH_ENV", startup)
				case "bash-func":
					t.Setenv("BASH_FUNC_git%%", "() { "+redirect+" command git \"$@\"; }")
				}
				code := verifyrunRow(t, root, row)
				if admit && code != verifyrunExitPass {
					t.Fatalf("admitted row's shell ran the caller's %s and read another tree (exit %d)", mode, code)
				}
				if !admit && code != verifyrunExitFail {
					t.Fatalf("plant did not land: unadmitted row did not run the caller's %s (exit %d)", mode, code)
				}
			})
		}
	}
}

// TestAdmittedRowsKeepNarrowing: an admitted run's rows read without system
// config, terminal prompt or askpass helper, whether the launch set those
// narrowing settings or the caller's environment widened them, and a launch's
// global config naming the home's own file survives (the row proves it is the
// only one read: an XDG config beside it is not). The unadmitted control
// proves the widening plant lands.
func TestAdmittedRowsKeepNarrowing(t *testing.T) {
	const pinned = `test "$GIT_CONFIG_NOSYSTEM" = 1 && test "$GIT_TERMINAL_PROMPT" = 0 && test "${GIT_ASKPASS-unset}" = ""`
	for _, mode := range []string{"launch", "widened", "control"} {
		t.Run(mode, func(t *testing.T) {
			root, _ := admittedFixture(t, mode != "control")
			home := t.TempDir()
			xdg := filepath.Join(home, "xdg")
			if err := os.MkdirAll(filepath.Join(xdg, "git"), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(xdg, "git", "config"), []byte("[fixture]\n\tprobe = xdg\n"), 0600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(home, ".gitconfig"), []byte("[fixture]\n\tprobe = global\n"), 0600); err != nil {
				t.Fatal(err)
			}
			t.Setenv("HOME", home)
			t.Setenv("XDG_CONFIG_HOME", xdg)
			row := pinned
			if mode == "launch" {
				t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(home, ".gitconfig"))
				t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
				t.Setenv("GIT_TERMINAL_PROMPT", "0")
				t.Setenv("GIT_ASKPASS", "")
				row = `test "$(git config --get-all fixture.probe)" = global && ` + pinned
			} else {
				t.Setenv("GIT_CONFIG_NOSYSTEM", "0")
				t.Setenv("GIT_TERMINAL_PROMPT", "1")
				t.Setenv("GIT_ASKPASS", "helper")
			}
			code := verifyrunRow(t, root, row)
			if mode != "control" && code != verifyrunExitPass {
				t.Fatalf("admitted row read with the caller's git settings, not the narrowing ones (%s, exit %d)", mode, code)
			}
			if mode == "control" && code != verifyrunExitFail {
				t.Fatalf("plant did not land: unadmitted row did not see the caller's git settings (exit %d)", code)
			}
		})
	}
}

// TestAdmittedEnvNarrowOnly: an inherited GIT_CONFIG_GLOBAL survives only in
// its narrowing form, and the pinned settings hold whatever value of the same
// variable the caller carries.
func TestAdmittedEnvNarrowOnly(t *testing.T) {
	home, elsewhere := t.TempDir(), t.TempDir()
	global := filepath.Join(home, ".gitconfig")
	for _, tc := range []struct {
		kv   string
		keep bool
	}{
		{"GIT_CONFIG_GLOBAL=" + global, true},
		{"GIT_CONFIG_GLOBAL=" + os.DevNull, true},
		{"GIT_CONFIG_GLOBAL=" + filepath.Join(elsewhere, ".gitconfig"), false},
		{"GIT_CONFIG_GLOBAL=.gitconfig", false},
		{"Git_Config_Global=" + global, false},
	} {
		got := slices.Contains(admittedRowEnv([]string{"HOME=" + home, tc.kv}), tc.kv)
		if got != tc.keep {
			t.Errorf("%s: kept=%v, want %v", tc.kv, got, tc.keep)
		}
	}
	if slices.Contains(admittedRowEnv([]string{"GIT_CONFIG_GLOBAL=" + global}), "GIT_CONFIG_GLOBAL="+global) {
		t.Error("GIT_CONFIG_GLOBAL kept with no HOME to compare it with")
	}
	for _, kv := range []string{"GIT_CONFIG_NOSYSTEM=bogus", "GIT_TERMINAL_PROMPT=bogus", "GIT_ASKPASS=" + filepath.Join(elsewhere, "helper")} {
		key, _, _ := strings.Cut(kv, "=")
		var vals []string
		for _, got := range admittedRowEnv([]string{"HOME=" + home, kv}) {
			if k, v, _ := strings.Cut(got, "="); k == key {
				vals = append(vals, v)
			}
		}
		if want := map[string]string{"GIT_CONFIG_NOSYSTEM": "1", "GIT_TERMINAL_PROMPT": "0", "GIT_ASKPASS": ""}[key]; !slices.Equal(vals, []string{want}) {
			t.Errorf("%s: row environment carries %s=%q, want only %q", kv, key, vals, want)
		}
	}
}
